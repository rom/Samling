package app

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rom/samling/internal/crypto"
)

// ---------- mentions ----------

// mentionRe finds "@username" in a message. The character before the @ must
// not be part of a word, so e-mail addresses are not taken for mentions.
var mentionRe = regexp.MustCompile(`(?i)(?:^|[^a-z0-9._@/-])@([a-z0-9][a-z0-9._-]{1,31})`)

const maxMentions = 20

// mentioned returns the distinct usernames mentioned in a text.
func mentioned(body string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range mentionRe.FindAllStringSubmatch(body, -1) {
		// "@anna." at the end of a sentence means anna.
		name := strings.TrimRight(strings.ToLower(m[1]), "._-")
		if len(name) < 2 || seen[name] {
			continue
		}
		seen[name] = true
		if out = append(out, name); len(out) == maxMentions {
			break
		}
	}
	return out
}

// notifyMentions tells the people mentioned in a message about it. A person
// is only told when they can read the conversation themselves: mentioning
// somebody in a direct message or a channel they have no access to does
// nothing, and reveals nothing to them. Each person is told once per message.
func (a *App) notifyMentions(ctx context.Context, actor *User, channelID, messageID, body string) {
	for _, name := range mentioned(body) {
		var uid string
		if err := a.DB.Row(ctx, `SELECT id FROM users WHERE username=? AND deleted_at=0 AND disabled=0`, name).Scan(&uid); err != nil || uid == actor.ID {
			continue
		}
		target, err := a.userByID(ctx, uid)
		if err != nil {
			continue
		}
		if lvl, err := a.level(ctx, target, "channel", channelID); err != nil || lvl < LevelRead {
			continue
		}
		n, err := a.DB.ExecN(ctx, `INSERT INTO notifications (id,user_id,kind,actor_id,channel_id,message_id,created_at) VALUES (?,?,?,?,?,?,?)
			ON CONFLICT DO NOTHING`, crypto.NewID(), uid, "mention", actor.ID, channelID, messageID, a.unix())
		if err == nil && n > 0 {
			a.hub.publishTo(uid, Event{Type: "mention", ID: channelID})
		}
	}
}

type Notification struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	ActorID   string `json:"actor_id"`
	ChannelID string `json:"channel_id"`
	Channel   string `json:"channel"` // name; empty for a direct conversation
	MessageID string `json:"message_id"`
	Snippet   string `json:"snippet"`
	CreatedAt int64  `json:"created_at"`
	Read      bool   `json:"read"`
}

// hNotifications lists the caller's mentions. Access is checked again here:
// a mention in a conversation the person can no longer read is not shown.
func (a *App) hNotifications(c *Ctx) error {
	vis, va := visible(c.U, "channel", "ch")
	out := struct {
		Unread int            `json:"unread"`
		Items  []Notification `json:"items"`
	}{Items: []Notification{}}
	from := ` FROM notifications n JOIN messages m ON m.id=n.message_id AND m.deleted_at=0 JOIN channels ch ON ch.id=n.channel_id WHERE n.user_id=? AND ` + vis
	args := append([]any{c.U.ID}, va...)
	if err := a.DB.Row(c.ctx(), `SELECT COUNT(*)`+from+` AND n.read_at=0`, args...).Scan(&out.Unread); err != nil {
		return err
	}
	err := a.DB.Each(c.ctx(), `SELECT n.id, n.kind, n.actor_id, n.channel_id, ch.kind, ch.name, n.message_id, m.body, n.created_at, n.read_at`+from+` ORDER BY n.id DESC LIMIT 50`, args,
		func(r *sql.Rows) error {
			var n Notification
			var kind, body string
			var read int64
			if err := r.Scan(&n.ID, &n.Kind, &n.ActorID, &n.ChannelID, &kind, &n.Channel, &n.MessageID, &body, &n.CreatedAt, &read); err != nil {
				return err
			}
			if kind == "dm" {
				n.Channel = ""
			}
			n.Read = read > 0
			n.Snippet = body
			if len(body) > 240 {
				cut := 240
				for cut > 0 && !utf8.RuneStart(body[cut]) {
					cut--
				}
				n.Snippet = body[:cut] + "…"
			}
			out.Items = append(out.Items, n)
			return nil
		})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// hNotificationsRead marks the caller's notifications as read: the given
// ones, or all of them.
func (a *App) hNotificationsRead(c *Ctx) error {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if len(in.IDs) > 200 {
		return errBad("limit_reached")
	}
	now := a.unix()
	if len(in.IDs) == 0 {
		if err := a.DB.Exec(c.ctx(), `UPDATE notifications SET read_at=? WHERE user_id=? AND read_at=0`, now, c.U.ID); err != nil {
			return err
		}
	}
	for _, id := range in.IDs {
		if err := a.DB.Exec(c.ctx(), `UPDATE notifications SET read_at=? WHERE id=? AND user_id=? AND read_at=0`, now, id, c.U.ID); err != nil {
			return err
		}
	}
	a.hub.publishTo(c.U.ID, Event{Type: "mention", ID: ""})
	return c.OK()
}

// ---------- internal links ----------

// linkTitles gives, per linkable kind of thing, the query for its title.
var linkTitles = map[string]string{
	"channel":  `SELECT CASE WHEN kind='dm' THEN '' ELSE name END FROM channels WHERE id=?`,
	"document": `SELECT name FROM documents WHERE id=?`,
	"list":     `SELECT title FROM lists WHERE id=?`,
	"poll":     `SELECT title FROM polls WHERE id=?`,
	"event":    `SELECT title FROM events WHERE id=?`,
	"meeting":  `SELECT title FROM meetings WHERE id=?`,
	"decision": `SELECT number || ' ' || title FROM decisions WHERE id=?`,
	"board":    `SELECT title FROM boards WHERE id=?`,
}

type LinkRef struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

// hResolveLinks returns the titles of the things a text links to — for
// those the caller can read. A link to something the caller has no access
// to is answered exactly like a link to something that does not exist, so a
// link never reveals more than its reader may see.
func (a *App) hResolveLinks(c *Ctx) error {
	var in struct {
		Refs []LinkRef `json:"refs"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if len(in.Refs) > 100 {
		return errBad("limit_reached")
	}
	out := []LinkRef{}
	for _, ref := range in.Refs {
		q, ok := linkTitles[ref.Type]
		if !ok {
			continue
		}
		if lvl, err := a.level(c.ctx(), c.U, ref.Type, ref.ID); err != nil || lvl < LevelRead {
			continue
		}
		var title string
		if err := a.DB.Row(c.ctx(), q, ref.ID).Scan(&title); err != nil {
			continue
		}
		out = append(out, LinkRef{Type: ref.Type, ID: ref.ID, Title: title})
	}
	return c.JSON(out)
}
