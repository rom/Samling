package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

// ---------- live updates (Server-Sent Events) ----------

// Event tells clients that a resource changed; they re-fetch what they show.
// Events carry no content, and are only delivered to users who can read the
// resource.
type Event struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	// to, when set, addresses the event to one person only.
	to string
}

// publishTo sends an event to the open streams of one person.
func (h *hub) publishTo(uid string, e Event) {
	e.to = uid
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.clients {
		if s.uid == uid {
			select {
			case s.ch <- e:
			default:
			}
		}
	}
}

// Limits on open event streams. The per-user limit keeps one account from
// occupying all slots; when it is exceeded the account's oldest stream is
// closed (its tab reconnects when it becomes active again).
const (
	maxStreams        = 2000
	maxStreamsPerUser = 6
)

type stream struct {
	ch   chan Event
	uid  string
	sess string // session id hash
	seq  uint64
	quit chan struct{}
	once sync.Once
}

func (s *stream) kick() { s.once.Do(func() { close(s.quit) }) }

type hub struct {
	mu      sync.Mutex
	clients map[*stream]struct{}
	seq     uint64
}

func newHub() *hub { return &hub{clients: map[*stream]struct{}{}} }

func (h *hub) publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.clients {
		select {
		case s.ch <- e:
		default: // slow client: drop, it will catch up on the next event
		}
	}
}

// add registers a stream; ok is false when the server-wide limit is reached.
func (h *hub) add(uid, sess string) (st *stream, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var own []*stream
	for s := range h.clients {
		if s.uid == uid {
			own = append(own, s)
		}
	}
	for len(own) >= maxStreamsPerUser {
		oldest := 0
		for i, s := range own {
			if s.seq < own[oldest].seq {
				oldest = i
			}
		}
		own[oldest].kick()
		delete(h.clients, own[oldest])
		own = append(own[:oldest], own[oldest+1:]...)
	}
	if len(h.clients) >= maxStreams {
		return nil, false
	}
	h.seq++
	st = &stream{ch: make(chan Event, 32), uid: uid, sess: sess, seq: h.seq, quit: make(chan struct{})}
	h.clients[st] = struct{}{}
	return st, true
}

func (h *hub) remove(st *stream) {
	h.mu.Lock()
	delete(h.clients, st)
	h.mu.Unlock()
}

// kickUser closes the streams of an account, except those of one session.
func (h *hub) kickUser(uid, keepSess string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.clients {
		if s.uid == uid && s.sess != keepSess {
			s.kick()
			delete(h.clients, s)
		}
	}
}

// kickSession closes the streams that belong to one session.
func (h *hub) kickSession(sess string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.clients {
		if s.sess == sess {
			s.kick()
			delete(h.clients, s)
		}
	}
}

// streamValid re-checks, while a stream is open, that its session still
// exists and its account is still active, and refreshes the account's roles.
func (a *App) streamValid(c *Ctx) bool {
	var expires, lastSeen int64
	if err := a.DB.Row(c.ctx(), `SELECT expires_at, last_seen FROM sessions WHERE id_hash=?`, c.sess.idHash).Scan(&expires, &lastSeen); err != nil {
		return false
	}
	if !a.sessionAlive(expires, lastSeen) {
		return false
	}
	u, err := a.userByID(c.ctx(), c.U.ID)
	if err != nil || u.Disabled || u.deletedAt != 0 {
		return false
	}
	c.U = u
	return true
}

func (a *App) hEvents(c *Ctx) error {
	fl, ok := c.W.(http.Flusher)
	if !ok {
		return errBad("streaming_unsupported")
	}
	st, ok := a.hub.add(c.U.ID, c.sess.idHash)
	if !ok {
		return &apiError{http.StatusServiceUnavailable, "busy"}
	}
	defer a.hub.remove(st)
	c.W.Header().Set("Content-Type", "text/event-stream")
	c.W.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(c.W, ": connected\n\n")
	fl.Flush()
	ping := time.NewTicker(a.streamPing)
	defer ping.Stop()
	for {
		select {
		case <-c.ctx().Done():
			return nil
		case <-st.quit: // signed out, disabled, deleted or replaced by a newer stream
			return nil
		case <-ping.C:
			if !a.streamValid(c) {
				return nil
			}
			fmt.Fprint(c.W, ": ping\n\n")
		case e := <-st.ch:
			if e.to != "" {
				if e.to != c.U.ID {
					continue
				}
			} else if lvl, err := a.level(c.ctx(), c.U, e.Type, e.ID); err != nil || lvl < LevelRead {
				continue
			}
			b, _ := json.Marshal(e)
			fmt.Fprintf(c.W, "data: %s\n\n", b)
		}
		fl.Flush()
	}
}

// ---------- channels ----------

type Channel struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"` // channel | dm
	Name     string   `json:"name"`
	Topic    string   `json:"topic"`
	OwnerID  string   `json:"owner_id"`
	Members  []string `json:"members,omitempty"` // dm only
	Unread   int      `json:"unread"`
	LastAt   int64    `json:"last_at"`
	Level    int      `json:"level"`
	Archived bool     `json:"-"`
}

func (a *App) hChannels(c *Ctx) error {
	vis, args := visible(c.U, "channel", "ch")
	q := `SELECT ch.id, ch.kind, ch.name, ch.topic, ch.owner_id,
		COALESCE((SELECT MAX(m.created_at) FROM messages m WHERE m.channel_id=ch.id),0),
		(SELECT COUNT(*) FROM messages m WHERE m.channel_id=ch.id AND m.deleted_at=0 AND m.user_id<>? AND m.created_at >
			COALESCE((SELECT last_read_at FROM channel_reads r WHERE r.user_id=? AND r.channel_id=ch.id),0))
		FROM channels ch WHERE ` + vis + ` ORDER BY ch.kind, LOWER(ch.name)`
	args = append([]any{c.U.ID, c.U.ID}, args...)
	out := []Channel{}
	err := a.DB.Each(c.ctx(), q, args, func(r *sql.Rows) error {
		var ch Channel
		if err := r.Scan(&ch.ID, &ch.Kind, &ch.Name, &ch.Topic, &ch.OwnerID, &ch.LastAt, &ch.Unread); err != nil {
			return err
		}
		out = append(out, ch)
		return nil
	})
	if err != nil {
		return err
	}
	for i := range out {
		if out[i].Level, err = a.level(c.ctx(), c.U, "channel", out[i].ID); err != nil {
			return err
		}
		if out[i].Kind == "dm" {
			err = a.DB.Each(c.ctx(), `SELECT subj_id FROM acl WHERE res_type='channel' AND res_id=? AND subj_type='user'`, []any{out[i].ID}, func(r *sql.Rows) error {
				var id string
				if err := r.Scan(&id); err != nil {
					return err
				}
				out[i].Members = append(out[i].Members, id)
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return c.JSON(out)
}

func (a *App) hCreateChannel(c *Ctx) error {
	var in struct {
		Name   string  `json:"name"`
		Topic  string  `json:"topic"`
		Shares []Share `json:"shares"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if in.Name = clean(in.Name, 60); in.Name == "" {
		return errBad("name_required")
	}
	shares, err := a.createShares(c, in.Shares)
	if err != nil {
		return err
	}
	id, now := crypto.NewID(), a.unix()
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `INSERT INTO channels (id,owner_id,kind,name,topic,created_at,updated_at) VALUES (?,?,'channel',?,?,?,?)`,
			id, c.U.ID, in.Name, clean(in.Topic, 300), now, now); err != nil {
			return err
		}
		return setShares(c.ctx(), tx, "channel", id, shares)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "channel.created", "channel", id, in.Name)
	a.hub.publish(Event{Type: "channel", ID: id})
	return c.JSON(map[string]string{"id": id})
}

func (a *App) hUpdateChannel(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		Name  string `json:"name"`
		Topic string `json:"topic"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "channel", id, LevelManage); err != nil {
		return err
	}
	if a.isDM(c.ctx(), "channel", id) {
		return errForbidden
	}
	if in.Name = clean(in.Name, 60); in.Name == "" {
		return errBad("name_required")
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE channels SET name=?, topic=?, updated_at=? WHERE id=?`, in.Name, clean(in.Topic, 300), a.unix(), id); err != nil {
		return err
	}
	a.hub.publish(Event{Type: "channel", ID: id})
	return c.OK()
}

func (a *App) hDeleteChannel(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "channel", id, LevelManage); err != nil {
		return err
	}
	if a.isDM(c.ctx(), "channel", id) {
		return errForbidden
	}
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `DELETE FROM acl WHERE res_type='channel' AND res_id=?`, id); err != nil {
			return err
		}
		return tx.Exec(c.ctx(), `DELETE FROM channels WHERE id=?`, id)
	})
	if err != nil {
		return err
	}
	a.gcFiles(c.ctx())
	a.logAct(c, true, "channel.deleted", "channel", id, "")
	return c.OK()
}

// hOpenDM returns the direct message conversation between the caller and the
// given users, creating it when needed.
func (a *App) hOpenDM(c *Ctx) error {
	var in struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	set := map[string]bool{c.U.ID: true}
	for _, id := range in.UserIDs {
		set[id] = true
	}
	if len(set) < 2 || len(set) > 20 {
		return errBad("invalid_recipients")
	}
	ids := make([]string, 0, len(set))
	shares := []Share{}
	for id := range set {
		ids = append(ids, id)
		shares = append(shares, Share{Type: "user", ID: id, Level: LevelWrite})
	}
	if err := a.validateShares(c.ctx(), c.U, shares); err != nil {
		return errBad("invalid_recipients")
	}
	// Somebody who has blocked direct messages from the caller cannot be
	// drawn into a conversation by them.
	var others []string
	for _, id := range ids {
		if id != c.U.ID {
			others = append(others, id)
		}
	}
	if blocked, err := a.blockedByAny(c.ctx(), c.U.ID, others); err != nil {
		return err
	} else if blocked {
		return &apiError{http.StatusForbidden, "dm_blocked"}
	}
	sort.Strings(ids)
	key := crypto.HashToken(strings.Join(ids, ","))
	var id string
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		err := tx.Row(c.ctx(), `SELECT id FROM channels WHERE kind='dm' AND dm_key=?`, key).Scan(&id)
		if err == nil {
			return nil
		}
		id = crypto.NewID()
		now := a.unix()
		// DMs have no owner: every participant has the same (write) level.
		if err := tx.Exec(c.ctx(), `INSERT INTO channels (id,owner_id,kind,dm_key,created_at,updated_at) VALUES (?,'','dm',?,?,?)`, id, key, now, now); err != nil {
			return err
		}
		return setShares(c.ctx(), tx, "channel", id, shares)
	})
	if err != nil {
		return err
	}
	a.hub.publish(Event{Type: "channel", ID: id})
	return c.JSON(map[string]string{"id": id})
}

// ---------- messages ----------

type FileInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Mime string `json:"mime"`
	Size int64  `json:"size"`
}

type Message struct {
	ID        string     `json:"id"`
	ChannelID string     `json:"channel_id"`
	UserID    string     `json:"user_id"`
	Body      string     `json:"body"`
	CreatedAt int64      `json:"created_at"`
	EditedAt  int64      `json:"edited_at"`
	Deleted   bool       `json:"deleted"`
	PinnedAt  int64      `json:"pinned_at"`
	PinnedBy  string     `json:"pinned_by"`
	Files     []FileInfo `json:"files"`
	// ParentID is set on a comment (reply): the message it belongs to.
	ParentID  string     `json:"parent_id"`
	Reactions []Reaction `json:"reactions"`
}

func (a *App) messages(c *Ctx, where string, args []any) ([]Message, error) {
	out := []Message{}
	idx := map[string]int{}
	err := a.DB.Each(c.ctx(), `SELECT id, channel_id, user_id, body, created_at, edited_at, deleted_at, pinned_at, pinned_by, parent_id FROM messages WHERE `+where, args,
		func(r *sql.Rows) error {
			m := Message{Files: []FileInfo{}, Reactions: []Reaction{}}
			var del int64
			if err := r.Scan(&m.ID, &m.ChannelID, &m.UserID, &m.Body, &m.CreatedAt, &m.EditedAt, &del, &m.PinnedAt, &m.PinnedBy, &m.ParentID); err != nil {
				return err
			}
			if del != 0 {
				m.Deleted, m.Body = true, ""
			}
			idx[m.ID] = len(out)
			out = append(out, m)
			return nil
		})
	if err != nil || len(out) == 0 {
		return out, err
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(out)), ",")
	ids := make([]any, len(out))
	for i, m := range out {
		ids[i] = m.ID
	}
	err = a.DB.Each(c.ctx(), `SELECT mf.message_id, f.id, f.name, f.mime, f.size FROM message_files mf JOIN files f ON f.id=mf.file_id WHERE mf.message_id IN (`+ph+`) ORDER BY f.name`,
		ids, func(r *sql.Rows) error {
			var mid string
			var f FileInfo
			if err := r.Scan(&mid, &f.ID, &f.Name, &f.Mime, &f.Size); err != nil {
				return err
			}
			if i := idx[mid]; !out[i].Deleted {
				out[i].Files = append(out[i].Files, f)
			}
			return nil
		})
	if err != nil {
		return out, err
	}
	// Reactions, counted per emoji; "mine" marks the caller's own.
	err = a.DB.Each(c.ctx(), `SELECT message_id, emoji, COUNT(*), SUM(CASE WHEN user_id=? THEN 1 ELSE 0 END) FROM reactions
		WHERE message_id IN (`+ph+`) GROUP BY message_id, emoji ORDER BY MIN(emoji)`, append([]any{c.U.ID}, ids...), func(r *sql.Rows) error {
		var mid string
		var re Reaction
		var mine int
		if err := r.Scan(&mid, &re.Emoji, &re.Count, &mine); err != nil {
			return err
		}
		re.Mine = mine > 0
		if i := idx[mid]; !out[i].Deleted {
			out[i].Reactions = append(out[i].Reactions, re)
		}
		return nil
	})
	return out, err
}

func (a *App) hMessages(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "channel", id, LevelRead); err != nil {
		return err
	}
	limit := min(max(c.QueryInt("limit", 50), 1), 200)
	// Ids sort by creation time, so "before" is simply the oldest id shown.
	// No sentinel value is used for "from the newest": how punctuation sorts
	// depends on the database collation.
	before := c.Query("before")
	msgs, err := a.messages(c, `channel_id=? AND (?='' OR id<?) ORDER BY id DESC LIMIT ?`, []any{id, before, before, limit})
	if err != nil {
		return err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 { // oldest first
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return c.JSON(msgs)
}

func (a *App) hPins(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "channel", id, LevelRead); err != nil {
		return err
	}
	msgs, err := a.messages(c, `channel_id=? AND pinned_at>0 AND deleted_at=0 ORDER BY pinned_at DESC, id DESC`, []any{id})
	if err != nil {
		return err
	}
	return c.JSON(msgs)
}

func (a *App) hPostMessage(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		Body    string   `json:"body"`
		FileIDs []string `json:"file_ids"`
		// ParentID makes the message a comment on another message.
		ParentID string `json:"parent_id"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "channel", id, LevelWrite); err != nil {
		return err
	}
	in.Body = clean(in.Body, 10000)
	if in.Body == "" && len(in.FileIDs) == 0 {
		return errBad("empty_message")
	}
	if len(in.FileIDs) > 10 {
		return errBad("too_many_files")
	}
	if blocked, err := a.dmBlocked(c.ctx(), c.U.ID, id); err != nil {
		return err
	} else if blocked {
		return &apiError{http.StatusForbidden, "dm_blocked"}
	}
	if in.ParentID != "" {
		// A comment belongs to a message of the same channel. Comments are
		// one level deep: a comment on a comment joins the original thread.
		var pch, pparent string
		if err := a.DB.Row(c.ctx(), `SELECT channel_id, parent_id FROM messages WHERE id=? AND deleted_at=0`, in.ParentID).Scan(&pch, &pparent); err != nil || pch != id {
			return errBad("unknown_message")
		}
		if pparent != "" {
			in.ParentID = pparent
		}
	}
	mid, now := crypto.NewID(), a.unix()
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `INSERT INTO messages (id,channel_id,user_id,body,created_at,parent_id) VALUES (?,?,?,?,?,?)`, mid, id, c.U.ID, in.Body, now, in.ParentID); err != nil {
			return err
		}
		for _, f := range in.FileIDs {
			// Only files the sender uploaded can be attached.
			var n int
			if err := tx.Row(c.ctx(), `SELECT COUNT(*) FROM files WHERE id=? AND uploader_id=?`, f, c.U.ID).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				return errBad("unknown_file")
			}
			if err := tx.Exec(c.ctx(), `INSERT INTO message_files (message_id,file_id) VALUES (?,?) ON CONFLICT DO NOTHING`, mid, f); err != nil {
				return err
			}
		}
		return tx.Exec(c.ctx(), `INSERT INTO channel_reads (user_id,channel_id,last_read_at) VALUES (?,?,?)
			ON CONFLICT (user_id,channel_id) DO UPDATE SET last_read_at=excluded.last_read_at`, c.U.ID, id, now)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "message.posted", "channel", id, "")
	a.hub.publish(Event{Type: "channel", ID: id})
	a.notifyMentions(c.ctx(), c.U, id, mid, in.Body)
	return c.JSON(map[string]string{"id": mid})
}

// msgAccess loads a message and the caller's level on its channel.
func (a *App) msgAccess(c *Ctx, mid string) (channel, author string, lvl int, err error) {
	if err = a.DB.Row(c.ctx(), `SELECT channel_id, user_id FROM messages WHERE id=? AND deleted_at=0`, mid).Scan(&channel, &author); err != nil {
		return "", "", 0, errNotFound
	}
	lvl, err = a.level(c.ctx(), c.U, "channel", channel)
	if err == nil && lvl < LevelRead {
		err = errNotFound
	}
	return
}

func (a *App) hEditMessage(c *Ctx) error {
	mid := c.Param("id")
	var in struct {
		Body string `json:"body"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	ch, author, lvl, err := a.msgAccess(c, mid)
	if err != nil {
		return err
	}
	if author != c.U.ID || lvl < LevelWrite {
		return errForbidden
	}
	if in.Body = clean(in.Body, 10000); in.Body == "" {
		return errBad("empty_message")
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE messages SET body=?, edited_at=? WHERE id=?`, in.Body, a.unix(), mid); err != nil {
		return err
	}
	a.notifyMentions(c.ctx(), c.U, ch, mid, in.Body) // people added by the edit
	a.hub.publish(Event{Type: "channel", ID: ch})
	return c.OK()
}

func (a *App) hDeleteMessage(c *Ctx) error {
	mid := c.Param("id")
	ch, author, lvl, err := a.msgAccess(c, mid)
	if err != nil {
		return err
	}
	if author != c.U.ID && lvl < LevelManage {
		return errForbidden
	}
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `DELETE FROM message_files WHERE message_id=?`, mid); err != nil {
			return err
		}
		if err := tx.Exec(c.ctx(), `DELETE FROM reactions WHERE message_id=?`, mid); err != nil {
			return err
		}
		if err := tx.Exec(c.ctx(), `DELETE FROM notifications WHERE message_id=?`, mid); err != nil {
			return err
		}
		return tx.Exec(c.ctx(), `UPDATE messages SET body='', deleted_at=?, pinned_at=0 WHERE id=?`, a.unix(), mid)
	})
	if err != nil {
		return err
	}
	a.gcFiles(c.ctx())
	a.logAct(c, author != c.U.ID, "message.deleted", "channel", ch, "")
	a.hub.publish(Event{Type: "channel", ID: ch})
	return c.OK()
}

func (a *App) hPin(c *Ctx) error {
	mid := c.Param("id")
	ch, _, lvl, err := a.msgAccess(c, mid)
	if err != nil {
		return err
	}
	if lvl < LevelWrite {
		return errForbidden
	}
	at, by := a.unix(), c.U.ID
	if c.R.Method == http.MethodDelete {
		at, by = 0, ""
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE messages SET pinned_at=?, pinned_by=? WHERE id=?`, at, by, mid); err != nil {
		return err
	}
	a.hub.publish(Event{Type: "channel", ID: ch})
	return c.OK()
}

func (a *App) hMarkRead(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "channel", id, LevelRead); err != nil {
		return err
	}
	// Reading the conversation also takes care of the mentions in it.
	if n, err := a.DB.ExecN(c.ctx(), `UPDATE notifications SET read_at=? WHERE user_id=? AND channel_id=? AND read_at=0`, a.unix(), c.U.ID, id); err != nil {
		return err
	} else if n > 0 {
		a.hub.publishTo(c.U.ID, Event{Type: "mention", ID: id})
	}
	return firstErr(a.DB.Exec(c.ctx(), `INSERT INTO channel_reads (user_id,channel_id,last_read_at) VALUES (?,?,?)
		ON CONFLICT (user_id,channel_id) DO UPDATE SET last_read_at=excluded.last_read_at`, c.U.ID, id, a.unix()), c.OK)
}

func firstErr(err error, then func() error) error {
	if err != nil {
		return err
	}
	return then()
}
