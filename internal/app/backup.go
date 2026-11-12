package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rom/samling/internal/blob"
	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

// Backup format
//
//	passphrase container (argon2id + chunked AES-256-GCM)
//	  └ gzip └ tar
//	      manifest.json            format, schema version, master key
//	      db/<table>/<n>.jsonl     rows as JSON objects (database independent)
//	      blobs/<key>              file contents (plaintext inside the container)
//
// Because rows are stored database independently, a backup taken from SQLite
// can be restored into PostgreSQL and vice versa. The master key is included
// so a backup plus its passphrase is sufficient for disaster recovery; the
// passphrase is therefore mandatory.

type Manifest struct {
	Format        int    `json:"format"`
	CreatedAt     int64  `json:"created_at"`
	SchemaVersion int    `json:"schema_version"`
	MasterKey     string `json:"master_key"`
	Org           string `json:"org"`
}

const backupRowsPerPart = 2000

func CheckPassphrase(p string) error {
	if len([]rune(p)) < 12 {
		return errBad("passphrase_too_short")
	}
	return nil
}

func tarBytes(tw *tar.Writer, name string, b []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(b)), ModTime: time.Now()}); err != nil {
		return err
	}
	_, err := tw.Write(b)
	return err
}

// WriteBackup writes an encrypted backup of the database and all files.
func WriteBackup(ctx context.Context, d *db.DB, blobs *blob.Store, keys *crypto.Keys, org string, w io.Writer, passphrase string) error {
	if err := CheckPassphrase(passphrase); err != nil {
		return err
	}
	enc, err := crypto.NewPassphraseWriter(w, passphrase)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(enc)
	tw := tar.NewWriter(gz)
	man, _ := json.Marshal(Manifest{Format: 1, CreatedAt: time.Now().Unix(), SchemaVersion: db.SchemaVersion(),
		MasterKey: base64.StdEncoding.EncodeToString(keys.Master()), Org: org})
	if err := tarBytes(tw, "manifest.json", man); err != nil {
		return err
	}
	type fileRef struct {
		key  string
		size int64
	}
	var files []fileRef
	// One transaction gives a consistent snapshot of all tables.
	err = d.InTx(ctx, func(tx *db.Tx) error {
		for _, table := range db.Tables {
			var buf bytes.Buffer
			part, rows := 0, 0
			flush := func() error {
				if rows == 0 {
					return nil
				}
				part++
				err := tarBytes(tw, fmt.Sprintf("db/%s/%05d.jsonl", table, part), buf.Bytes())
				buf.Reset()
				rows = 0
				return err
			}
			// Rows are written in key order, never in the order they happen to
			// be stored: storage order can reveal the order of events.
			err := tx.Each(ctx, `SELECT * FROM `+table+` ORDER BY 1, 2`, nil, func(r *sql.Rows) error {
				cols, err := r.Columns()
				if err != nil {
					return err
				}
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := r.Scan(ptrs...); err != nil {
					return err
				}
				row := make(map[string]any, len(cols))
				for i, col := range cols {
					if b, ok := vals[i].([]byte); ok {
						row[col] = string(b)
					} else {
						row[col] = vals[i]
					}
				}
				line, err := json.Marshal(row)
				if err != nil {
					return err
				}
				buf.Write(line)
				buf.WriteByte('\n')
				if rows++; rows >= backupRowsPerPart {
					return flush()
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("dump %s: %w", table, err)
			}
			if err := flush(); err != nil {
				return err
			}
		}
		return tx.Each(ctx, `SELECT DISTINCT blob_key, size FROM files`, nil, func(r *sql.Rows) error {
			var f fileRef
			if err := r.Scan(&f.key, &f.size); err != nil {
				return err
			}
			files = append(files, f)
			return nil
		})
	})
	if err != nil {
		return err
	}
	for _, f := range files {
		rc, err := blobs.Get(ctx, f.key)
		if err != nil {
			return fmt.Errorf("blob %s: %w", f.key, err)
		}
		if err := tw.WriteHeader(&tar.Header{Name: "blobs/" + f.key, Mode: 0o600, Size: f.size, ModTime: time.Now()}); err != nil {
			rc.Close()
			return err
		}
		// A size mismatch (corrupt blob) makes the tar writer fail loudly.
		if err := copyClose(tw, rc); err != nil {
			return fmt.Errorf("blob %s: %w", f.key, err)
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return enc.Close()
}

// RestoreTarget is called with the manifest before any data is written and
// returns the (empty) database and blob store to restore into.
type RestoreTarget func(m Manifest) (*db.DB, *blob.Store, error)

// Restore reads a backup created by WriteBackup into an empty installation.
func Restore(ctx context.Context, r io.Reader, passphrase string, target RestoreTarget) (rows, blobs int, err error) {
	dec, err := crypto.NewPassphraseReader(r, passphrase)
	if err != nil {
		return 0, 0, err
	}
	gz, err := gzip.NewReader(dec)
	if err != nil {
		return 0, 0, errors.New("wrong passphrase or damaged backup")
	}
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil || hdr.Name != "manifest.json" {
		return 0, 0, errors.New("backup has no manifest")
	}
	var man Manifest
	if err := json.NewDecoder(io.LimitReader(tr, 1<<20)).Decode(&man); err != nil {
		return 0, 0, err
	}
	if man.Format != 1 {
		return 0, 0, fmt.Errorf("unsupported backup format %d", man.Format)
	}
	// Older backups can be restored: schema changes only add tables and
	// columns with defaults. A backup from a newer release cannot.
	if man.SchemaVersion > db.SchemaVersion() {
		return 0, 0, fmt.Errorf("backup has schema version %d, this binary only knows %d: restore with a newer release",
			man.SchemaVersion, db.SchemaVersion())
	}
	d, store, err := target(man)
	if err != nil {
		return 0, 0, err
	}
	var n int
	if err := d.Row(ctx, `SELECT (SELECT COUNT(*) FROM users) + (SELECT COUNT(*) FROM roles)`).Scan(&n); err != nil {
		return 0, 0, err
	}
	if n > 0 {
		return 0, 0, errors.New("the target database is not empty; restore only into a fresh installation")
	}
	known := map[string]bool{}
	for _, t := range db.Tables {
		known[t] = true
	}
	err = d.InTx(ctx, func(tx *db.Tx) error {
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			parts := strings.Split(hdr.Name, "/")
			switch {
			case len(parts) == 3 && parts[0] == "db" && known[parts[1]]:
				jd := json.NewDecoder(tr)
				jd.UseNumber()
				for {
					row := map[string]any{}
					if err := jd.Decode(&row); err == io.EOF {
						break
					} else if err != nil {
						return fmt.Errorf("%s: %w", hdr.Name, err)
					}
					cols := make([]string, 0, len(row))
					vals := make([]any, 0, len(row))
					for col, v := range row {
						if !identRe.MatchString(col) {
							return fmt.Errorf("%s: bad column name", hdr.Name)
						}
						if num, ok := v.(json.Number); ok {
							i, err := num.Int64()
							if err != nil {
								return fmt.Errorf("%s: %w", hdr.Name, err)
							}
							v = i
						}
						cols = append(cols, col)
						vals = append(vals, v)
					}
					q := `INSERT INTO ` + parts[1] + ` (` + strings.Join(cols, ",") + `) VALUES (` + strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",") + `)`
					if err := tx.Exec(ctx, q, vals...); err != nil {
						return fmt.Errorf("%s: %w", hdr.Name, err)
					}
					rows++
				}
			case len(parts) == 2 && parts[0] == "blobs":
				if err := store.Put(ctx, parts[1], tr); err != nil {
					return fmt.Errorf("%s: %w", hdr.Name, err)
				}
				blobs++
			default:
				return fmt.Errorf("unexpected entry %q in backup", hdr.Name)
			}
		}
	})
	return rows, blobs, err
}

// ---------- HTTP: backup, exports, history ----------

func (a *App) hBackup(c *Ctx) error {
	var in struct {
		Passphrase string `json:"passphrase"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := CheckPassphrase(in.Passphrase); err != nil {
		return err
	}
	a.logAct(c, true, "backup.created", "", "", "")
	c.W.Header().Set("Content-Type", "application/octet-stream")
	c.W.Header().Set("Content-Disposition", `attachment; filename="samling-backup-`+a.now().UTC().Format("20060102-150405")+`.bak"`)
	if err := WriteBackup(c.ctx(), a.DB, a.Blobs, a.Keys, a.Cfg.OrgName, c.W, in.Passphrase); err != nil {
		// Headers are sent: abort the connection so the client sees a failure
		// instead of a truncated file (which would also fail authentication).
		a.Log.Error("backup failed", "err", err)
		panic(httpAbort)
	}
	return nil
}

// hAdminExport exports members or the full decision log as CSV, optionally
// encrypted with a passphrase (decrypt with "samling decrypt").
func (a *App) hAdminExport(c *Ctx) error {
	var in struct {
		Kind       string `json:"kind"`
		Passphrase string `json:"passphrase"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if in.Kind != "members" {
		return errBad("invalid_kind")
	}
	if !c.U.Can(PermUsers) {
		return errForbidden
	}
	if in.Passphrase != "" {
		if err := CheckPassphrase(in.Passphrase); err != nil {
			return err
		}
	}
	people, err := a.directory(c.ctx(), c.U)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"username", "name", "email", "phone", "roles"})
	for _, p := range people {
		_ = w.Write([]string{p.Username, csvSafe(p.DisplayName), csvSafe(p.Email), csvSafe(p.Phone), strings.Join(p.Roles, " ")})
	}
	w.Flush()
	a.logAct(c, true, "export.members", "", "", fmt.Sprintf("%d rows, encrypted=%v", len(people), in.Passphrase != ""))
	name := "members.csv"
	var out io.Writer = c.W
	var closer io.Closer
	if in.Passphrase != "" {
		name += ".enc"
	}
	c.W.Header().Set("Content-Type", "application/octet-stream")
	c.W.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	if in.Passphrase != "" {
		pw, err := crypto.NewPassphraseWriter(c.W, in.Passphrase)
		if err != nil {
			return err
		}
		out, closer = pw, pw
	}
	if _, err := out.Write(buf.Bytes()); err != nil {
		return err
	}
	if closer != nil {
		return closer.Close()
	}
	return nil
}

type historyEntry struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	ResType   string `json:"res_type"`
	ResID     string `json:"res_id"`
	Summary   string `json:"summary"`
	CreatedAt int64  `json:"created_at"`
}

func (a *App) history(ctx context.Context, uid, before string, limit int) ([]historyEntry, error) {
	out := []historyEntry{}
	err := a.DB.Each(ctx, `SELECT id, action, res_type, res_id, summary, created_at FROM activity WHERE user_id=? AND (?='' OR id<?) ORDER BY id DESC LIMIT ?`,
		[]any{uid, before, before, limit}, func(r *sql.Rows) error {
			var e historyEntry
			if err := r.Scan(&e.ID, &e.Action, &e.ResType, &e.ResID, &e.Summary, &e.CreatedAt); err != nil {
				return err
			}
			out = append(out, e)
			return nil
		})
	return out, err
}

func (a *App) hMyHistory(c *Ctx) error {
	out, err := a.history(c.ctx(), c.U.ID, c.Query("before"), 200)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// hMyExport returns the personal data held about the caller (GDPR access /
// portability): profile, memberships, activity and authored content.
func (a *App) hMyExport(c *Ctx) error {
	ctx := c.ctx()
	out := map[string]any{"exported_at": a.unix(), "profile": c.U}
	hist, err := a.history(ctx, c.U.ID, "", 100000)
	if err != nil {
		return err
	}
	out["activity"] = hist
	var groups []string
	if err := a.DB.Each(ctx, `SELECT g.name FROM groups g JOIN group_members gm ON gm.group_id=g.id WHERE gm.user_id=? AND g.scope='global'`, []any{c.U.ID}, func(r *sql.Rows) error {
		var n string
		err := r.Scan(&n)
		groups = append(groups, n)
		return err
	}); err != nil {
		return err
	}
	out["groups"] = groups
	type msg struct {
		ChannelID string `json:"channel_id"`
		Body      string `json:"body"`
		CreatedAt int64  `json:"created_at"`
	}
	msgs := []msg{}
	if err := a.DB.Each(ctx, `SELECT channel_id, body, created_at FROM messages WHERE user_id=? AND deleted_at=0 ORDER BY created_at`, []any{c.U.ID}, func(r *sql.Rows) error {
		var m msg
		err := r.Scan(&m.ChannelID, &m.Body, &m.CreatedAt)
		msgs = append(msgs, m)
		return err
	}); err != nil {
		return err
	}
	out["messages"] = msgs
	owned := map[string][]string{}
	for typ, table := range resTables {
		col := "title"
		if typ == "channel" || typ == "document" {
			col = "name"
		}
		if err := a.DB.Each(ctx, `SELECT `+col+` FROM `+table+` WHERE owner_id=?`, []any{c.U.ID}, func(r *sql.Rows) error {
			var t string
			err := r.Scan(&t)
			owned[typ] = append(owned[typ], t)
			return err
		}); err != nil {
			return err
		}
	}
	out["owned"] = owned
	a.logAct(c, true, "export.personal", "user", c.U.ID, "")
	c.W.Header().Set("Content-Disposition", `attachment; filename="samling-my-data.json"`)
	return c.JSON(out)
}
