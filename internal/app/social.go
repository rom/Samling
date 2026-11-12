package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/rom/samling/internal/db"
)

// ---------- optional profile fields ----------

// profileFields are the optional fields a person can fill in. They are
// stored together, encrypted like the other personal data, and each one is
// shown to other members only when its owner chooses to share it.
var profileFields = []string{"address", "interests", "specialities", "about"}

func cleanProfile(in map[string]string) map[string]string {
	out := map[string]string{}
	for _, k := range profileFields {
		if v := clean(in[k], 600); v != "" {
			out[k] = v
		}
	}
	return out
}

func cleanProfileShared(in map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, k := range profileFields {
		if in[k] {
			out[k] = true
		}
	}
	return out
}

func (a *App) encodeProfile(uid string, profile map[string]string, shared map[string]bool) (enc, sharedJSON string) {
	if len(profile) > 0 {
		b, _ := json.Marshal(profile)
		enc = a.Keys.EncryptField(string(b), "users.profile", uid)
	}
	b, _ := json.Marshal(cleanProfileShared(shared))
	return enc, string(b)
}

func (a *App) decodeProfile(uid, enc, sharedJSON string) (map[string]string, map[string]bool) {
	profile, shared := map[string]string{}, map[string]bool{}
	if plain, err := a.Keys.DecryptField(enc, "users.profile", uid); err == nil && plain != "" {
		_ = json.Unmarshal([]byte(plain), &profile)
	}
	if sharedJSON != "" {
		_ = json.Unmarshal([]byte(sharedJSON), &shared)
	}
	return profile, shared
}

// ---------- avatars ----------

// avatarPresets is the number of built-in avatars the client offers.
const avatarPresets = 24

// imageTypes are the picture formats that may be shown inline. The type of
// an upload is established from its content (see hUpload), not its name.
var imageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true}

// checkAvatar validates an avatar value: "" (initial letter), "preset:<n>"
// or "file:<id>" for a photo the person uploaded.
func (a *App) checkAvatar(c *Ctx, v string) error {
	switch {
	case v == "":
		return nil
	case strings.HasPrefix(v, "preset:"):
		if n, err := strconv.Atoi(strings.TrimPrefix(v, "preset:")); err == nil && n >= 1 && n <= avatarPresets {
			return nil
		}
	case strings.HasPrefix(v, "file:"):
		var mime string
		err := a.DB.Row(c.ctx(), `SELECT mime FROM files WHERE id=? AND uploader_id=?`, strings.TrimPrefix(v, "file:"), c.U.ID).Scan(&mime)
		if err == nil && imageTypes[mime] {
			return nil
		}
	}
	return errBad("invalid_avatar")
}

// ---------- blocking direct messages ----------

func (a *App) hBlocks(c *Ctx) error {
	out := []string{}
	err := a.DB.Each(c.ctx(), `SELECT blocked_id FROM dm_blocks WHERE user_id=? ORDER BY blocked_id`, []any{c.U.ID}, func(r *sql.Rows) error {
		var id string
		err := r.Scan(&id)
		out = append(out, id)
		return err
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// hBlock makes the caller unreachable by direct message for one person
// (PUT), or lifts that (DELETE).
func (a *App) hBlock(c *Ctx) error {
	id := c.Param("id")
	if c.R.Method == http.MethodDelete {
		if err := a.DB.Exec(c.ctx(), `DELETE FROM dm_blocks WHERE user_id=? AND blocked_id=?`, c.U.ID, id); err != nil {
			return err
		}
		return c.OK()
	}
	var n int
	if err := a.DB.Row(c.ctx(), `SELECT COUNT(*) FROM users WHERE id=? AND deleted_at=0`, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 || id == c.U.ID {
		return errBad("unknown_user")
	}
	if err := a.DB.Exec(c.ctx(), `INSERT INTO dm_blocks (user_id,blocked_id,created_at) VALUES (?,?,?) ON CONFLICT DO NOTHING`, c.U.ID, id, a.unix()); err != nil {
		return err
	}
	a.logAct(c, false, "dm.blocked", "user", id, "")
	return c.OK()
}

// blockedByAny reports whether one of the given people has blocked direct
// messages from sender.
func (a *App) blockedByAny(ctx context.Context, sender string, others []string) (bool, error) {
	for _, o := range others {
		var n int
		if err := a.DB.Row(ctx, `SELECT COUNT(*) FROM dm_blocks WHERE user_id=? AND blocked_id=?`, o, sender).Scan(&n); err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

// dmBlocked reports whether sender may not write in a direct message
// conversation because another participant has blocked them.
func (a *App) dmBlocked(ctx context.Context, sender, channelID string) (bool, error) {
	var kind string
	if err := a.DB.Row(ctx, `SELECT kind FROM channels WHERE id=?`, channelID).Scan(&kind); err != nil || kind != "dm" {
		return false, nil
	}
	var others []string
	err := a.DB.Each(ctx, `SELECT subj_id FROM acl WHERE res_type='channel' AND res_id=? AND subj_type='user' AND subj_id<>?`, []any{channelID, sender}, func(r *sql.Rows) error {
		var id string
		err := r.Scan(&id)
		others = append(others, id)
		return err
	})
	if err != nil {
		return false, err
	}
	return a.blockedByAny(ctx, sender, others)
}

// ---------- reactions ----------

// reactionEmoji are the reactions that can be put on a message.
var reactionEmoji = map[string]bool{
	"👍": true, "👎": true, "❤️": true, "😀": true, "😂": true, "😮": true, "😢": true, "😡": true, "🎉": true, "🙏": true,
	"👏": true, "🔥": true, "✅": true, "❓": true, "👀": true, "💪": true,
}

type Reaction struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
	Mine  bool   `json:"mine"`
}

// hReact toggles the caller's reaction on a message. Reading the channel is
// enough, so members can react in announcement channels too.
func (a *App) hReact(c *Ctx) error {
	mid := c.Param("id")
	var in struct {
		Emoji string `json:"emoji"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if !reactionEmoji[in.Emoji] {
		return errBad("invalid_emoji")
	}
	ch, _, _, err := a.msgAccess(c, mid)
	if err != nil {
		return err
	}
	if blocked, err := a.dmBlocked(c.ctx(), c.U.ID, ch); err != nil {
		return err
	} else if blocked {
		return &apiError{http.StatusForbidden, "dm_blocked"}
	}
	n, err := a.DB.ExecN(c.ctx(), `DELETE FROM reactions WHERE message_id=? AND user_id=? AND emoji=?`, mid, c.U.ID, in.Emoji)
	if err != nil {
		return err
	}
	if n == 0 {
		if err := a.DB.Exec(c.ctx(), `INSERT INTO reactions (message_id,user_id,emoji) VALUES (?,?,?) ON CONFLICT DO NOTHING`, mid, c.U.ID, in.Emoji); err != nil {
			return err
		}
	}
	a.hub.publish(Event{"channel", ch})
	return c.OK()
}

// ---------- attachments (photos and files on other things than messages) ----------

// attachTargets maps what can carry attachments to the access level needed
// to add or remove one. Reading the resource is enough to see them.
var attachTargets = map[string]int{
	"list": LevelWrite, "event": LevelWrite, "card": LevelWrite, "meeting": LevelWrite,
	"poll": LevelManage, "decision": LevelManage,
}

const maxAttachments = 20

// attachmentACL resolves the resource whose sharing governs an attachment
// target: a card belongs to its board, everything else is its own resource.
func (a *App) attachmentACL(ctx context.Context, typ, id string) (resType, resID string, err error) {
	if _, ok := attachTargets[typ]; !ok {
		return "", "", errNotFound
	}
	if typ == "card" {
		var board string
		if err := a.DB.Row(ctx, `SELECT board_id FROM cards WHERE id=?`, id).Scan(&board); err != nil {
			return "", "", errNotFound
		}
		return "board", board, nil
	}
	return typ, id, nil
}

type Attachment struct {
	FileInfo
	UserID    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
}

func (a *App) hAttachments(c *Ctx) error {
	typ, id := c.Param("type"), c.Param("id")
	rt, rid, err := a.attachmentACL(c.ctx(), typ, id)
	if err != nil {
		return err
	}
	if err := a.need(c, rt, rid, LevelRead); err != nil {
		return err
	}
	out := []Attachment{}
	err = a.DB.Each(c.ctx(), `SELECT f.id, f.name, f.mime, f.size, t.user_id, t.created_at FROM attachments t JOIN files f ON f.id=t.file_id
		WHERE t.res_type=? AND t.res_id=? ORDER BY t.created_at, f.id`, []any{typ, id}, func(r *sql.Rows) error {
		var x Attachment
		if err := r.Scan(&x.ID, &x.Name, &x.Mime, &x.Size, &x.UserID, &x.CreatedAt); err != nil {
			return err
		}
		out = append(out, x)
		return nil
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (a *App) hAttach(c *Ctx) error {
	typ, id := c.Param("type"), c.Param("id")
	var in struct {
		FileID string `json:"file_id"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	rt, rid, err := a.attachmentACL(c.ctx(), typ, id)
	if err != nil {
		return err
	}
	if err := a.need(c, rt, rid, attachTargets[typ]); err != nil {
		return err
	}
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := ownFile(c.ctx(), tx, c.U, in.FileID); err != nil {
			return err
		}
		if err := limitCount(c.ctx(), tx, maxAttachments, `SELECT COUNT(*) FROM attachments WHERE res_type=? AND res_id=?`, typ, id); err != nil {
			return err
		}
		return tx.Exec(c.ctx(), `INSERT INTO attachments (res_type,res_id,file_id,user_id,created_at) VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING`,
			typ, id, in.FileID, c.U.ID, a.unix())
	})
	if err != nil {
		return err
	}
	a.hub.publish(Event{rt, rid})
	return c.OK()
}

func (a *App) hDetach(c *Ctx) error {
	typ, id := c.Param("type"), c.Param("id")
	rt, rid, err := a.attachmentACL(c.ctx(), typ, id)
	if err != nil {
		return err
	}
	if err := a.need(c, rt, rid, attachTargets[typ]); err != nil {
		return err
	}
	if err := a.DB.Exec(c.ctx(), `DELETE FROM attachments WHERE res_type=? AND res_id=? AND file_id=?`, typ, id, c.Param("file")); err != nil {
		return err
	}
	a.gcFiles(c.ctx())
	a.hub.publish(Event{rt, rid})
	return c.OK()
}

// colours that boards and cards can be marked with.
var colorNames = map[string]bool{"": true, "red": true, "orange": true, "yellow": true, "green": true, "teal": true,
	"blue": true, "purple": true, "pink": true, "gray": true}
