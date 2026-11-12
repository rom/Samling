package app

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

// Uploaded files are immutable. They are referenced from chat messages
// (attachments) and document versions; access to a file follows from access
// to one of the things that reference it.

func safeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || strings.ContainsRune(`"<>:|?*/`, r) {
			return '_'
		}
		return r
	}, name)
	if name = clean(name, 200); name == "" || name == "." || name == ".." {
		name = "file"
	}
	return name
}

func (a *App) hUpload(c *Ctx) error {
	limit := a.Cfg.Storage.MaxUploadMB << 20
	// Storage quota: the files an account has uploaded and that are still
	// kept. Checked before reading the body and again with the real size.
	var used int64
	if err := a.DB.Row(c.ctx(), `SELECT COALESCE(SUM(size),0) FROM files WHERE uploader_id=?`, c.U.ID).Scan(&used); err != nil {
		return err
	}
	quota := a.Cfg.Storage.UserQuotaMB << 20
	if quota > 0 && used >= quota {
		return &apiError{http.StatusRequestEntityTooLarge, "quota_exceeded"}
	}
	c.R.Body = http.MaxBytesReader(c.W, c.R.Body, limit+1<<20)
	mr, err := c.R.MultipartReader()
	if err != nil {
		return errBad("multipart_required")
	}
	part, err := mr.NextPart()
	if err != nil || part.FormName() != "file" {
		return errBad("file_required")
	}
	name := safeFilename(part.FileName())
	// The stored type is informational: it comes from the extension, never
	// from the client, and files are always served as downloads (see
	// hDownload). Pictures are the exception: they can be shown inline (see
	// hImage), so a file only counts as a picture when its content really
	// starts like one, whatever it is called.
	mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if mt == "" {
		mt = "application/octet-stream"
	}
	head := bufio.NewReaderSize(part, 512)
	first, _ := head.Peek(512)
	if sniffed := http.DetectContentType(first); imageTypes[sniffed] {
		mt = sniffed
	} else if strings.HasPrefix(mt, "image/") {
		mt = "application/octet-stream"
	}
	id, key := crypto.NewID(), crypto.NewID()
	h := sha256.New()
	cr := &countReader{r: io.TeeReader(head, h)}
	if err := a.Blobs.Put(c.ctx(), key, cr); err != nil {
		_ = a.Blobs.Delete(context.WithoutCancel(c.ctx()), key)
		return err
	}
	if cr.n > limit {
		_ = a.Blobs.Delete(c.ctx(), key)
		return &apiError{http.StatusRequestEntityTooLarge, "too_large"}
	}
	if cr.n == 0 {
		_ = a.Blobs.Delete(c.ctx(), key)
		return errBad("empty_file")
	}
	if quota > 0 && used+cr.n > quota {
		_ = a.Blobs.Delete(c.ctx(), key)
		return &apiError{http.StatusRequestEntityTooLarge, "quota_exceeded"}
	}
	err = a.DB.Exec(c.ctx(), `INSERT INTO files (id,blob_key,name,mime,size,sha256,uploader_id,created_at) VALUES (?,?,?,?,?,?,?,?)`,
		id, key, name, mt, cr.n, hex.EncodeToString(h.Sum(nil)), c.U.ID, a.unix())
	if err != nil {
		_ = a.Blobs.Delete(c.ctx(), key)
		return err
	}
	return c.JSON(FileInfo{ID: id, Name: name, Mime: mt, Size: cr.n})
}

type countReader struct {
	r io.Reader
	n int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// canReadFile: the uploader, anyone who can read a channel where the file is
// attached, or anyone who can read a document that has it as a version.
func (a *App) canReadFile(c *Ctx, fileID, uploader string) (bool, error) {
	if uploader == c.U.ID {
		return true, nil
	}
	var refs []Event
	collect := func(typ string) func(*sql.Rows) error {
		return func(r *sql.Rows) error {
			var id string
			if err := r.Scan(&id); err != nil {
				return err
			}
			refs = append(refs, Event{Type: typ, ID: id})
			return nil
		}
	}
	if err := a.DB.Each(c.ctx(), `SELECT DISTINCT m.channel_id FROM message_files mf JOIN messages m ON m.id=mf.message_id WHERE mf.file_id=? AND m.deleted_at=0`,
		[]any{fileID}, collect("channel")); err != nil {
		return false, err
	}
	if err := a.DB.Each(c.ctx(), `SELECT DISTINCT doc_id FROM doc_versions WHERE file_id=?`, []any{fileID}, collect("document")); err != nil {
		return false, err
	}
	// A profile photo can be seen by every member.
	var avatars int
	if err := a.DB.Row(c.ctx(), `SELECT COUNT(*) FROM users WHERE avatar=? AND deleted_at=0`, "file:"+fileID).Scan(&avatars); err != nil {
		return false, err
	}
	if avatars > 0 {
		return true, nil
	}
	// Attachments follow the thing they are attached to.
	type att struct{ typ, id string }
	var atts []att
	if err := a.DB.Each(c.ctx(), `SELECT res_type, res_id FROM attachments WHERE file_id=?`, []any{fileID}, func(r *sql.Rows) error {
		var x att
		err := r.Scan(&x.typ, &x.id)
		atts = append(atts, x)
		return err
	}); err != nil {
		return false, err
	}
	for _, x := range atts {
		if rt, rid, err := a.attachmentACL(c.ctx(), x.typ, x.id); err == nil {
			refs = append(refs, Event{Type: rt, ID: rid})
		}
	}
	for _, ref := range refs {
		if lvl, err := a.level(c.ctx(), c.U, ref.Type, ref.ID); err == nil && lvl >= LevelRead {
			return true, nil
		}
	}
	return false, nil
}

func (a *App) hDownload(c *Ctx) error {
	id := c.Param("id")
	var key, name, mt, uploader string
	var size int64
	if err := a.DB.Row(c.ctx(), `SELECT blob_key, name, mime, size, uploader_id FROM files WHERE id=?`, id).Scan(&key, &name, &mt, &size, &uploader); err != nil {
		return errNotFound
	}
	ok, err := a.canReadFile(c, id, uploader)
	if err != nil {
		return err
	}
	if !ok {
		return errNotFound
	}
	return a.serveFile(c, key, name, mt)
}

// hImage shows a picture inline (photos in chat and attachments, profile
// photos). Only files whose content was recognised as a picture at upload
// are served this way, with their real type; everything else is refused.
func (a *App) hImage(c *Ctx) error {
	id := c.Param("id")
	var key, mt, uploader string
	if err := a.DB.Row(c.ctx(), `SELECT blob_key, mime, uploader_id FROM files WHERE id=?`, id).Scan(&key, &mt, &uploader); err != nil {
		return errNotFound
	}
	if ok, err := a.canReadFile(c, id, uploader); err != nil {
		return err
	} else if !ok || !imageTypes[mt] {
		return errNotFound
	}
	rc, err := a.Blobs.Get(c.ctx(), key)
	if err != nil {
		return err
	}
	h := c.W.Header()
	h.Set("Content-Type", mt)
	h.Set("Content-Disposition", "inline")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Cache-Control", "private, max-age=86400") // files never change
	return copyClose(c.W, rc)
}

func (a *App) serveFile(c *Ctx, key, name, mt string) error {
	rc, err := a.Blobs.Get(c.ctx(), key)
	if err != nil {
		return err
	}
	h := c.W.Header()
	// Uploads are always sent as an opaque download: "attachment", a generic
	// content type and a sandboxing CSP. The stored type (from the file
	// extension) is only informational. A generic type means an uploaded
	// file can never be loaded by a page of this site as a script, a style
	// sheet or JSON data, whatever its name.
	_ = mt
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	return copyClose(c.W, rc)
}

// gcFiles deletes files that are no longer referenced by any message or
// document version (and are older than an hour, so uploads that are about to
// be attached are left alone).
func (a *App) gcFiles(ctx context.Context) {
	ctx = context.WithoutCancel(ctx)
	// Attachments of things that no longer exist go first.
	for typ, table := range map[string]string{"list": "lists", "event": "events", "card": "cards", "meeting": "meetings", "poll": "polls", "decision": "decisions"} {
		if err := a.DB.Exec(ctx, `DELETE FROM attachments WHERE res_type=? AND NOT EXISTS (SELECT 1 FROM `+table+` x WHERE x.id=attachments.res_id)`, typ); err != nil {
			a.Log.Error("attachment gc", "err", err)
		}
	}
	type f struct{ id, key string }
	var dead []f
	err := a.DB.Each(ctx, `SELECT id, blob_key FROM files WHERE created_at < ?
		AND NOT EXISTS (SELECT 1 FROM message_files mf WHERE mf.file_id=files.id)
		AND NOT EXISTS (SELECT 1 FROM doc_versions dv WHERE dv.file_id=files.id)
		AND NOT EXISTS (SELECT 1 FROM attachments t WHERE t.file_id=files.id)
		AND NOT EXISTS (SELECT 1 FROM users u WHERE u.avatar = 'file:' || files.id)`, []any{a.unix() - 3600}, func(r *sql.Rows) error {
		var x f
		if err := r.Scan(&x.id, &x.key); err != nil {
			return err
		}
		dead = append(dead, x)
		return nil
	})
	if err != nil {
		a.Log.Error("file gc", "err", err)
		return
	}
	for _, x := range dead {
		if err := a.DB.Exec(ctx, `DELETE FROM files WHERE id=?`, x.id); err != nil {
			continue
		}
		if err := a.Blobs.Delete(ctx, x.key); err != nil {
			a.Log.Error("blob delete", "key", x.key, "err", err)
		}
	}
}

// ---------- document archive ----------

type DocVersion struct {
	Version   int      `json:"version"`
	File      FileInfo `json:"file"`
	Comment   string   `json:"comment"`
	UserID    string   `json:"user_id"`
	CreatedAt int64    `json:"created_at"`
}

type Document struct {
	ID          string       `json:"id"`
	OwnerID     string       `json:"owner_id"`
	Folder      string       `json:"folder"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Version     int          `json:"version"`
	Size        int64        `json:"size"`
	Mime        string       `json:"mime"`
	CreatedAt   int64        `json:"created_at"`
	UpdatedAt   int64        `json:"updated_at"`
	Shared      bool         `json:"shared"`
	Level       int          `json:"level,omitempty"`
	Versions    []DocVersion `json:"versions"`
	Shares      []Share      `json:"shares,omitempty"`
}

const docSelect = `SELECT d.id, d.owner_id, d.folder, d.name, d.description, d.current_version, f.size, f.mime, d.created_at, d.updated_at,
	(SELECT COUNT(*) FROM acl a WHERE a.res_type='document' AND a.res_id=d.id)
	FROM documents d JOIN doc_versions v ON v.doc_id=d.id AND v.version=d.current_version JOIN files f ON f.id=v.file_id`

func scanDoc(r scanner) (Document, error) {
	var d Document
	var shares int
	err := r.Scan(&d.ID, &d.OwnerID, &d.Folder, &d.Name, &d.Description, &d.Version, &d.Size, &d.Mime, &d.CreatedAt, &d.UpdatedAt, &shares)
	d.Shared = shares > 0
	return d, err
}

func cleanFolder(f string) string {
	var parts []string
	for _, p := range strings.Split(strings.ReplaceAll(f, "\\", "/"), "/") {
		if p = clean(p, 60); p != "" && p != "." && p != ".." {
			parts = append(parts, p)
		}
	}
	if len(parts) > 8 {
		parts = parts[:8]
	}
	return strings.Join(parts, "/")
}

func (a *App) hDocuments(c *Ctx) error {
	vis, args := visible(c.U, "document", "d")
	out := []Document{}
	err := a.DB.Each(c.ctx(), docSelect+` WHERE `+vis+` ORDER BY LOWER(d.folder), LOWER(d.name)`, args, func(r *sql.Rows) error {
		d, err := scanDoc(r)
		out = append(out, d)
		return err
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// ownFile checks that the caller uploaded the file (so file IDs of others
// cannot be claimed into a document the caller controls).
func ownFile(ctx context.Context, q db.Q, u *User, fileID string) error {
	var n int
	if err := q.Row(ctx, `SELECT COUNT(*) FROM files WHERE id=? AND uploader_id=?`, fileID, u.ID).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return errBad("unknown_file")
	}
	return nil
}

func (a *App) hCreateDocument(c *Ctx) error {
	var in struct {
		Name        string  `json:"name"`
		Folder      string  `json:"folder"`
		Description string  `json:"description"`
		FileID      string  `json:"file_id"`
		Shares      []Share `json:"shares"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	shares, err := a.createShares(c, in.Shares)
	if err != nil {
		return err
	}
	if err := limitCount(c.ctx(), a.DB, maxPerOwner, `SELECT COUNT(*) FROM documents WHERE owner_id=?`, c.U.ID); err != nil {
		return err
	}
	id, now := crypto.NewID(), a.unix()
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		if err := ownFile(ctx, tx, c.U, in.FileID); err != nil {
			return err
		}
		if in.Name = clean(in.Name, 200); in.Name == "" {
			if err := tx.Row(ctx, `SELECT name FROM files WHERE id=?`, in.FileID).Scan(&in.Name); err != nil {
				return err
			}
		}
		if err := tx.Exec(ctx, `INSERT INTO documents (id,owner_id,folder,name,description,current_version,created_at,updated_at) VALUES (?,?,?,?,?,1,?,?)`,
			id, c.U.ID, cleanFolder(in.Folder), in.Name, clean(in.Description, 2000), now, now); err != nil {
			return err
		}
		if err := tx.Exec(ctx, `INSERT INTO doc_versions (doc_id,version,file_id,comment,user_id,created_at) VALUES (?,1,?,'',?,?)`, id, in.FileID, c.U.ID, now); err != nil {
			return err
		}
		return setShares(ctx, tx, "document", id, shares)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "document.created", "document", id, in.Name)
	return c.JSON(map[string]string{"id": id})
}

func (a *App) hDocument(c *Ctx) error {
	id := c.Param("id")
	lvl, err := a.level(c.ctx(), c.U, "document", id)
	if err != nil || lvl < LevelRead {
		return errNotFound
	}
	d, err := scanDoc(a.DB.Row(c.ctx(), docSelect+` WHERE d.id=?`, id))
	if err != nil {
		return err
	}
	d.Level = lvl
	err = a.DB.Each(c.ctx(), `SELECT v.version, v.comment, v.user_id, v.created_at, f.id, f.name, f.mime, f.size FROM doc_versions v JOIN files f ON f.id=v.file_id
		WHERE v.doc_id=? ORDER BY v.version DESC`, []any{id}, func(r *sql.Rows) error {
		var v DocVersion
		if err := r.Scan(&v.Version, &v.Comment, &v.UserID, &v.CreatedAt, &v.File.ID, &v.File.Name, &v.File.Mime, &v.File.Size); err != nil {
			return err
		}
		d.Versions = append(d.Versions, v)
		return nil
	})
	if err != nil {
		return err
	}
	return c.JSON(d)
}

func (a *App) hUpdateDocument(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		Name        string `json:"name"`
		Folder      string `json:"folder"`
		Description string `json:"description"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "document", id, LevelWrite); err != nil {
		return err
	}
	if in.Name = clean(in.Name, 200); in.Name == "" {
		return errBad("name_required")
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE documents SET name=?, folder=?, description=?, updated_at=? WHERE id=?`,
		in.Name, cleanFolder(in.Folder), clean(in.Description, 2000), a.unix(), id); err != nil {
		return err
	}
	a.logAct(c, false, "document.updated", "document", id, in.Name)
	return c.OK()
}

func (a *App) hAddVersion(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		FileID  string `json:"file_id"`
		Comment string `json:"comment"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "document", id, LevelWrite); err != nil {
		return err
	}
	var ver int
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		if err := ownFile(ctx, tx, c.U, in.FileID); err != nil {
			return err
		}
		if err := tx.Row(ctx, `SELECT MAX(version)+1 FROM doc_versions WHERE doc_id=?`, id).Scan(&ver); err != nil {
			return err
		}
		now := a.unix()
		if err := tx.Exec(ctx, `INSERT INTO doc_versions (doc_id,version,file_id,comment,user_id,created_at) VALUES (?,?,?,?,?,?)`,
			id, ver, in.FileID, clean(in.Comment, 500), c.U.ID, now); err != nil {
			return err
		}
		return tx.Exec(ctx, `UPDATE documents SET current_version=?, updated_at=? WHERE id=?`, ver, now, id)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "document.version_added", "document", id, "v"+itoa(ver))
	return c.JSON(map[string]int{"version": ver})
}

// hCopyDocument creates a private copy of the current version, owned by the
// caller. The (immutable) file content is shared, not duplicated.
func (a *App) hCopyDocument(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "document", id, LevelRead); err != nil {
		return err
	}
	var in struct {
		Name   string `json:"name"`
		Folder string `json:"folder"`
	}
	if c.R.ContentLength != 0 { // the body is optional
		if err := c.Bind(&in); err != nil {
			return err
		}
	}
	nid, now := crypto.NewID(), a.unix()
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		var name, desc, fileID string
		if err := tx.Row(ctx, `SELECT d.name, d.description, v.file_id FROM documents d JOIN doc_versions v ON v.doc_id=d.id AND v.version=d.current_version WHERE d.id=?`, id).
			Scan(&name, &desc, &fileID); err != nil {
			return err
		}
		if n := clean(in.Name, 200); n != "" {
			name = n
		}
		in.Name = name
		if err := tx.Exec(ctx, `INSERT INTO documents (id,owner_id,folder,name,description,current_version,created_at,updated_at) VALUES (?,?,?,?,?,1,?,?)`,
			nid, c.U.ID, cleanFolder(in.Folder), name, desc, now, now); err != nil {
			return err
		}
		return tx.Exec(ctx, `INSERT INTO doc_versions (doc_id,version,file_id,comment,user_id,created_at) VALUES (?,1,?,'',?,?)`, nid, fileID, c.U.ID, now)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "document.copied", "document", nid, in.Name)
	return c.JSON(map[string]string{"id": nid})
}

func (a *App) hDeleteDocument(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "document", id, LevelManage); err != nil {
		return err
	}
	var name string
	_ = a.DB.Row(c.ctx(), `SELECT name FROM documents WHERE id=?`, id).Scan(&name)
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `DELETE FROM acl WHERE res_type='document' AND res_id=?`, id); err != nil {
			return err
		}
		return tx.Exec(c.ctx(), `DELETE FROM documents WHERE id=?`, id)
	})
	if err != nil {
		return err
	}
	a.gcFiles(c.ctx())
	a.logAct(c, false, "document.deleted", "document", id, name)
	return c.OK()
}

func (a *App) hDownloadDocument(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "document", id, LevelRead); err != nil {
		return err
	}
	q := `SELECT f.blob_key, f.name, f.mime, d.name FROM documents d JOIN doc_versions v ON v.doc_id=d.id JOIN files f ON f.id=v.file_id WHERE d.id=? AND v.version=`
	args := []any{id}
	if v := c.QueryInt("version", 0); v > 0 {
		q += `?`
		args = append(args, v)
	} else {
		q += `d.current_version`
	}
	var key, fname, mt, dname string
	if err := a.DB.Row(c.ctx(), q, args...).Scan(&key, &fname, &mt, &dname); err != nil {
		return errNotFound
	}
	// Serve under the document's name, keeping the uploaded file's extension.
	name := safeFilename(dname)
	if ext := filepath.Ext(fname); ext != "" && !strings.EqualFold(filepath.Ext(name), ext) {
		name += ext
	}
	a.logAct(c, false, "document.downloaded", "document", id, dname)
	return a.serveFile(c, key, name, mt)
}
