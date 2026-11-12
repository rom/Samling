package app

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

// Meeting flow: 1) plan (time, place, who) 2) agenda 3) protocol: notes per
// agenda item, attendance, decisions 4) finalize, which locks the protocol.
// A meeting automatically has a calendar entry shared with the same people.

type AgendaItem struct {
	ID          string `json:"id"`
	Position    int    `json:"position"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Presenter   string `json:"presenter"`
	DurationMin int    `json:"duration_min"`
	Notes       string `json:"notes"`
	DecisionID  string `json:"decision_id"`
}

type Meeting struct {
	ID          string       `json:"id"`
	OwnerID     string       `json:"owner_id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Location    string       `json:"location"`
	URL         string       `json:"url"`
	StartsAt    int64        `json:"starts_at"`
	EndsAt      int64        `json:"ends_at"`
	Status      string       `json:"status"` // planned | in_progress | finalized
	Attendees   string       `json:"attendees"`
	Chair       string       `json:"chair"`
	Secretary   string       `json:"secretary"`
	Notes       string       `json:"notes"`
	FinalizedAt int64        `json:"finalized_at"`
	FinalizedBy string       `json:"finalized_by"`
	Level       int          `json:"level,omitempty"`
	Agenda      []AgendaItem `json:"agenda"`
}

const meetingCols = `m.id, m.owner_id, m.title, m.description, m.location, m.url, m.starts_at, m.ends_at, m.status, m.attendees, m.chair, m.secretary, m.notes, m.finalized_at, m.finalized_by`

func scanMeeting(r scanner) (*Meeting, error) {
	m := &Meeting{}
	return m, r.Scan(&m.ID, &m.OwnerID, &m.Title, &m.Description, &m.Location, &m.URL, &m.StartsAt, &m.EndsAt, &m.Status, &m.Attendees, &m.Chair,
		&m.Secretary, &m.Notes, &m.FinalizedAt, &m.FinalizedBy)
}

func (a *App) hMeetings(c *Ctx) error {
	vis, args := visible(c.U, "meeting", "m")
	out := []*Meeting{}
	err := a.DB.Each(c.ctx(), `SELECT `+meetingCols+` FROM meetings m WHERE `+vis+` ORDER BY m.starts_at DESC`, args, func(r *sql.Rows) error {
		m, err := scanMeeting(r)
		if err == nil {
			m.Notes, m.Attendees = "", "" // list view: headers only
			out = append(out, m)
		}
		return err
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (a *App) loadMeeting(c *Ctx, id string) (*Meeting, error) {
	lvl, err := a.level(c.ctx(), c.U, "meeting", id)
	if err != nil || lvl < LevelRead {
		return nil, errNotFound
	}
	m, err := scanMeeting(a.DB.Row(c.ctx(), `SELECT `+meetingCols+` FROM meetings m WHERE m.id=?`, id))
	if err != nil {
		return nil, err
	}
	m.Level, m.Agenda = lvl, []AgendaItem{}
	err = a.DB.Each(c.ctx(), `SELECT id, position, title, description, presenter, duration_min, notes, decision_id FROM agenda_items WHERE meeting_id=? ORDER BY position`,
		[]any{id}, func(r *sql.Rows) error {
			var i AgendaItem
			if err := r.Scan(&i.ID, &i.Position, &i.Title, &i.Description, &i.Presenter, &i.DurationMin, &i.Notes, &i.DecisionID); err != nil {
				return err
			}
			m.Agenda = append(m.Agenda, i)
			return nil
		})
	return m, err
}

func (a *App) hMeeting(c *Ctx) error {
	m, err := a.loadMeeting(c, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(m)
}

type meetingIn struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	URL         string   `json:"url"`
	StartsAt    int64    `json:"starts_at"`
	EndsAt      int64    `json:"ends_at"`
	Status      string   `json:"status"`
	Attendees   string   `json:"attendees"`
	Chair       string   `json:"chair"`
	Secretary   string   `json:"secretary"`
	Notes       string   `json:"notes"`
	Shares      []Share  `json:"shares"`
	Agenda      []string `json:"agenda"`
}

func (in *meetingIn) check() (err error) {
	if in.Title = clean(in.Title, 200); in.Title == "" {
		return errBad("title_required")
	}
	if in.URL, err = cleanURL(in.URL); err != nil {
		return err
	}
	if in.StartsAt <= 0 || in.EndsAt < in.StartsAt || !validTime(in.EndsAt) {
		return errBad("invalid_time")
	}
	in.Description, in.Location = clean(in.Description, 4000), clean(in.Location, 300)
	in.Attendees, in.Notes = clean(in.Attendees, 4000), clean(in.Notes, 20000)
	in.Chair, in.Secretary = clean(in.Chair, 100), clean(in.Secretary, 100)
	return nil
}

func syncMeetingEventACL(ctx context.Context, tx *db.Tx, meetingID string) error {
	var eid string
	if err := tx.Row(ctx, `SELECT id FROM events WHERE meeting_id=?`, meetingID).Scan(&eid); err != nil {
		return nil // no calendar entry
	}
	if err := tx.Exec(ctx, `DELETE FROM acl WHERE res_type='event' AND res_id=?`, eid); err != nil {
		return err
	}
	// Calendar entries of meetings are read-only for everyone but the owner.
	return tx.Exec(ctx, `INSERT INTO acl (res_type,res_id,subj_type,subj_id,level) SELECT 'event', CAST(? AS TEXT), subj_type, subj_id, 1 FROM acl WHERE res_type='meeting' AND res_id=?`, eid, meetingID)
}

func (a *App) hCreateMeeting(c *Ctx) error {
	var in meetingIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := in.check(); err != nil {
		return err
	}
	if len(in.Agenda) > 100 {
		return errBad("too_many_items")
	}
	shares, err := a.createShares(c, in.Shares)
	if err != nil {
		return err
	}
	id, now := crypto.NewID(), a.unix()
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		if err := tx.Exec(ctx, `INSERT INTO meetings (id,owner_id,title,description,location,url,starts_at,ends_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, c.U.ID, in.Title, in.Description, in.Location, in.URL, in.StartsAt, in.EndsAt, now, now); err != nil {
			return err
		}
		pos := 0
		for _, t := range in.Agenda {
			if t = clean(t, 200); t == "" {
				continue
			}
			pos++
			if err := tx.Exec(ctx, `INSERT INTO agenda_items (id,meeting_id,position,title) VALUES (?,?,?,?)`, crypto.NewID(), id, pos, t); err != nil {
				return err
			}
		}
		if err := setShares(ctx, tx, "meeting", id, shares); err != nil {
			return err
		}
		eid := crypto.NewID()
		if err := tx.Exec(ctx, `INSERT INTO events (id,owner_id,title,description,location,url,starts_at,ends_at,all_day,uid,meeting_id,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,0,?,?,?,?)`, eid, c.U.ID, in.Title, in.Description, in.Location, in.URL, in.StartsAt, in.EndsAt, eid+"@samling", id, now, now); err != nil {
			return err
		}
		return syncMeetingEventACL(ctx, tx, id)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "meeting.created", "meeting", id, in.Title)
	a.hub.publish(Event{Type: "meeting", ID: id})
	return c.JSON(map[string]string{"id": id})
}

// editable checks write access and that the protocol is not finalized.
func (a *App) meetingEditable(c *Ctx, id string) error {
	if err := a.need(c, "meeting", id, LevelWrite); err != nil {
		return err
	}
	var status string
	if err := a.DB.Row(c.ctx(), `SELECT status FROM meetings WHERE id=?`, id).Scan(&status); err != nil {
		return err
	}
	if status == "finalized" {
		return errBad("meeting_finalized")
	}
	return nil
}

func (a *App) hUpdateMeeting(c *Ctx) error {
	id := c.Param("id")
	var in meetingIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.meetingEditable(c, id); err != nil {
		return err
	}
	if err := in.check(); err != nil {
		return err
	}
	if in.Status != "planned" && in.Status != "in_progress" {
		in.Status = "planned"
	}
	now := a.unix()
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `UPDATE meetings SET title=?, description=?, location=?, url=?, starts_at=?, ends_at=?, status=?, attendees=?, chair=?, secretary=?, notes=?, updated_at=? WHERE id=?`,
			in.Title, in.Description, in.Location, in.URL, in.StartsAt, in.EndsAt, in.Status, in.Attendees, in.Chair, in.Secretary, in.Notes, now, id); err != nil {
			return err
		}
		return tx.Exec(c.ctx(), `UPDATE events SET title=?, description=?, location=?, url=?, starts_at=?, ends_at=?, updated_at=? WHERE meeting_id=?`,
			in.Title, in.Description, in.Location, in.URL, in.StartsAt, in.EndsAt, now, id)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "meeting.updated", "meeting", id, in.Title)
	a.hub.publish(Event{Type: "meeting", ID: id})
	return c.OK()
}

// hSetAgenda replaces the agenda with the given ordered list. Items that
// carry an id are kept (with their decision link); the others are created.
func (a *App) hSetAgenda(c *Ctx) error {
	id := c.Param("id")
	var items []AgendaItem
	if err := c.Bind(&items); err != nil {
		return err
	}
	if err := a.meetingEditable(c, id); err != nil {
		return err
	}
	if len(items) > 100 {
		return errBad("too_many_items")
	}
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		existing := map[string]bool{}
		if err := tx.Each(ctx, `SELECT id FROM agenda_items WHERE meeting_id=?`, []any{id}, func(r *sql.Rows) error {
			var x string
			err := r.Scan(&x)
			existing[x] = true
			return err
		}); err != nil {
			return err
		}
		for i, it := range items {
			if it.Title = clean(it.Title, 200); it.Title == "" {
				return errBad("title_required")
			}
			desc, pres, notes := clean(it.Description, 4000), clean(it.Presenter, 100), clean(it.Notes, 20000)
			dur := min(max(it.DurationMin, 0), 1440)
			if existing[it.ID] {
				delete(existing, it.ID)
				if err := tx.Exec(ctx, `UPDATE agenda_items SET position=?, title=?, description=?, presenter=?, duration_min=?, notes=? WHERE id=?`,
					i+1, it.Title, desc, pres, dur, notes, it.ID); err != nil {
					return err
				}
			} else if err := tx.Exec(ctx, `INSERT INTO agenda_items (id,meeting_id,position,title,description,presenter,duration_min,notes) VALUES (?,?,?,?,?,?,?,?)`,
				crypto.NewID(), id, i+1, it.Title, desc, pres, dur, notes); err != nil {
				return err
			}
		}
		for gone := range existing {
			if err := tx.Exec(ctx, `DELETE FROM agenda_items WHERE id=?`, gone); err != nil {
				return err
			}
		}
		return tx.Exec(ctx, `UPDATE meetings SET updated_at=? WHERE id=?`, a.unix(), id)
	})
	if err != nil {
		return err
	}
	a.hub.publish(Event{Type: "meeting", ID: id})
	return c.OK()
}

func (a *App) hFinalizeMeeting(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "meeting", id, LevelManage); err != nil {
		return err
	}
	reopen := strings.HasSuffix(c.R.URL.Path, "/reopen")
	var err error
	if reopen {
		err = a.DB.Exec(c.ctx(), `UPDATE meetings SET status='in_progress', finalized_at=0, finalized_by='' WHERE id=?`, id)
	} else {
		err = a.DB.Exec(c.ctx(), `UPDATE meetings SET status='finalized', finalized_at=?, finalized_by=? WHERE id=?`, a.unix(), c.U.ID, id)
	}
	if err != nil {
		return err
	}
	a.logAct(c, true, map[bool]string{true: "meeting.reopened", false: "meeting.finalized"}[reopen], "meeting", id, "")
	a.hub.publish(Event{Type: "meeting", ID: id})
	return c.OK()
}

func (a *App) hDeleteMeeting(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "meeting", id, LevelManage); err != nil {
		return err
	}
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		for _, q := range []string{
			`DELETE FROM acl WHERE res_type='event' AND res_id IN (SELECT id FROM events WHERE meeting_id=?)`,
			`DELETE FROM events WHERE meeting_id=?`,
			`DELETE FROM acl WHERE res_type='meeting' AND res_id=?`,
			`DELETE FROM meetings WHERE id=?`,
		} {
			if err := tx.Exec(ctx, q, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	a.logAct(c, true, "meeting.deleted", "meeting", id, "")
	return c.OK()
}

// hProtocol renders the protocol (minutes) as Markdown.
func (a *App) hProtocol(c *Ctx) error {
	m, err := a.loadMeeting(c, c.Param("id"))
	if err != nil {
		return err
	}
	sv := c.U.Locale == "sv"
	l := func(en, s string) string {
		if sv {
			return s
		}
		return en
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: %s\n\n", l("Minutes", "Protokoll"), m.Title)
	fmt.Fprintf(&b, "- **%s:** %s\n", l("Date", "Datum"), time.Unix(m.StartsAt, 0).UTC().Format("2006-01-02 15:04 UTC"))
	for _, f := range [][2]string{{l("Location", "Plats"), m.Location}, {l("Chair", "Ordförande"), m.Chair}, {l("Secretary", "Sekreterare"), m.Secretary},
		{l("Present", "Närvarande"), strings.ReplaceAll(m.Attendees, "\n", ", ")}} {
		if f[1] != "" {
			fmt.Fprintf(&b, "- **%s:** %s\n", f[0], f[1])
		}
	}
	b.WriteString("\n")
	for _, it := range m.Agenda {
		fmt.Fprintf(&b, "## § %d %s\n\n", it.Position, it.Title)
		if it.Notes != "" {
			b.WriteString(it.Notes + "\n\n")
		}
		if it.DecisionID != "" {
			var num, title, body string
			// Only include the decision when the reader may see it.
			if lvl, err := a.level(c.ctx(), c.U, "decision", it.DecisionID); err == nil && lvl >= LevelRead {
				if a.DB.Row(c.ctx(), `SELECT number, title, body FROM decisions WHERE id=?`, it.DecisionID).Scan(&num, &title, &body) == nil {
					fmt.Fprintf(&b, "**%s %s:** %s\n\n", l("Decision", "Beslut"), num, title)
					if body != "" {
						b.WriteString(body + "\n\n")
					}
				}
			}
		}
	}
	if m.Notes != "" {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", l("Other notes", "Övriga anteckningar"), m.Notes)
	}
	if m.FinalizedAt > 0 {
		fmt.Fprintf(&b, "---\n%s %s\n", l("Finalized", "Justerat"), time.Unix(m.FinalizedAt, 0).UTC().Format("2006-01-02"))
	} else {
		fmt.Fprintf(&b, "---\n*%s*\n", l("Draft – not finalized", "Utkast – ej justerat"))
	}
	c.W.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	c.W.Header().Set("Content-Disposition", `attachment; filename="protocol-`+time.Unix(m.StartsAt, 0).UTC().Format("2006-01-02")+`.md"`)
	_, err = c.W.Write([]byte(b.String()))
	return err
}

// ---------- decision log ----------

type DecisionRev struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	Comment   string `json:"comment"`
	UserID    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
}

type Decision struct {
	ID        string        `json:"id"`
	OwnerID   string        `json:"owner_id"`
	Number    string        `json:"number"`
	Title     string        `json:"title"`
	Body      string        `json:"body"`
	Status    string        `json:"status"` // decided | revoked | superseded
	DecidedAt int64         `json:"decided_at"`
	MeetingID string        `json:"meeting_id"`
	PollID    string        `json:"poll_id"`
	UpdatedAt int64         `json:"updated_at"`
	Level     int           `json:"level,omitempty"`
	Revisions []DecisionRev `json:"revisions"`
}

var decisionStatus = map[string]bool{"decided": true, "revoked": true, "superseded": true}

func (a *App) decisions(ctx context.Context, u *User) ([]Decision, error) {
	vis, args := visible(u, "decision", "d")
	out := []Decision{}
	err := a.DB.Each(ctx, `SELECT d.id, d.owner_id, d.number, d.title, d.body, d.status, d.decided_at, d.meeting_id, d.poll_id, d.updated_at
		FROM decisions d WHERE `+vis+` ORDER BY d.decided_at DESC, d.number DESC`, args, func(r *sql.Rows) error {
		var d Decision
		if err := r.Scan(&d.ID, &d.OwnerID, &d.Number, &d.Title, &d.Body, &d.Status, &d.DecidedAt, &d.MeetingID, &d.PollID, &d.UpdatedAt); err != nil {
			return err
		}
		out = append(out, d)
		return nil
	})
	return out, err
}

func (a *App) hDecisions(c *Ctx) error {
	out, err := a.decisions(c.ctx(), c.U)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (a *App) hCreateDecision(c *Ctx) error {
	var in struct {
		Title        string  `json:"title"`
		Body         string  `json:"body"`
		DecidedAt    int64   `json:"decided_at"`
		MeetingID    string  `json:"meeting_id"`
		AgendaItemID string  `json:"agenda_item_id"`
		PollID       string  `json:"poll_id"`
		Shares       []Share `json:"shares"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if in.Title = clean(in.Title, 200); in.Title == "" {
		return errBad("title_required")
	}
	now := a.unix()
	if in.DecidedAt <= 0 {
		in.DecidedAt = now
	}
	if !validTime(in.DecidedAt) {
		return errBad("invalid_time")
	}
	// Linked meeting/poll must be accessible to the author.
	if in.MeetingID != "" {
		if err := a.meetingEditable(c, in.MeetingID); err != nil {
			return err
		}
	}
	if in.PollID != "" {
		if err := a.need(c, "poll", in.PollID, LevelRead); err != nil {
			return err
		}
	}
	shares, err := a.createShares(c, in.Shares)
	if err != nil {
		return err
	}
	id, body := crypto.NewID(), clean(in.Body, 20000)
	var number string
	create := func(tx *db.Tx) error {
		ctx := c.ctx()
		// Decisions are numbered per year: 2026-001, 2026-002, ... The next
		// number follows the highest one in use (never a count, which would
		// collide if a row were ever missing).
		year := time.Unix(in.DecidedAt, 0).UTC().Format("2006")
		var n int
		if err := tx.Row(ctx, `SELECT COALESCE(MAX(CAST(SUBSTR(number, 6) AS INTEGER)), 0) FROM decisions WHERE number LIKE ?`, year+"-%").Scan(&n); err != nil {
			return err
		}
		number = fmt.Sprintf("%s-%03d", year, n+1)
		if err := tx.Exec(ctx, `INSERT INTO decisions (id,owner_id,number,title,body,status,decided_at,meeting_id,poll_id,created_at,updated_at) VALUES (?,?,?,?,?,'decided',?,?,?,?,?)`,
			id, c.U.ID, number, in.Title, body, in.DecidedAt, in.MeetingID, in.PollID, now, now); err != nil {
			return err
		}
		if err := tx.Exec(ctx, `INSERT INTO decision_revisions (id,decision_id,title,body,status,comment,user_id,created_at) VALUES (?,?,?,?,'decided','',?,?)`,
			crypto.NewID(), id, in.Title, body, c.U.ID, now); err != nil {
			return err
		}
		if in.MeetingID != "" && in.AgendaItemID != "" {
			if err := tx.Exec(ctx, `UPDATE agenda_items SET decision_id=? WHERE id=? AND meeting_id=?`, id, in.AgendaItemID, in.MeetingID); err != nil {
				return err
			}
		}
		return setShares(ctx, tx, "decision", id, shares)
	}
	// Two decisions recorded at the same moment can pick the same number on
	// PostgreSQL; the unique number then rejects one of them, which retries.
	for attempt := 0; attempt < 5; attempt++ {
		if err = a.DB.InTx(c.ctx(), create); err == nil || !isUniqueViolation(err) {
			break
		}
	}
	if err != nil {
		return err
	}
	a.logAct(c, true, "decision.created", "decision", id, number)
	a.hub.publish(Event{Type: "decision", ID: id})
	return c.JSON(map[string]string{"id": id, "number": number})
}

func (a *App) hDecision(c *Ctx) error {
	id := c.Param("id")
	lvl, err := a.level(c.ctx(), c.U, "decision", id)
	if err != nil || lvl < LevelRead {
		return errNotFound
	}
	d := Decision{Level: lvl}
	if err := a.DB.Row(c.ctx(), `SELECT id, owner_id, number, title, body, status, decided_at, meeting_id, poll_id, updated_at FROM decisions WHERE id=?`, id).
		Scan(&d.ID, &d.OwnerID, &d.Number, &d.Title, &d.Body, &d.Status, &d.DecidedAt, &d.MeetingID, &d.PollID, &d.UpdatedAt); err != nil {
		return err
	}
	err = a.DB.Each(c.ctx(), `SELECT title, body, status, comment, user_id, created_at FROM decision_revisions WHERE decision_id=? ORDER BY id DESC`,
		[]any{id}, func(r *sql.Rows) error {
			var v DecisionRev
			if err := r.Scan(&v.Title, &v.Body, &v.Status, &v.Comment, &v.UserID, &v.CreatedAt); err != nil {
				return err
			}
			d.Revisions = append(d.Revisions, v)
			return nil
		})
	if err != nil {
		return err
	}
	return c.JSON(d)
}

// hReviseDecision changes a decision. Decisions are never deleted or
// silently edited: every change is stored as a revision with a comment.
func (a *App) hReviseDecision(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		Status  string `json:"status"`
		Comment string `json:"comment"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "decision", id, LevelManage); err != nil {
		return err
	}
	if !c.U.Can(PermDecisions) {
		return errForbidden
	}
	if in.Title = clean(in.Title, 200); in.Title == "" {
		return errBad("title_required")
	}
	if !decisionStatus[in.Status] {
		return errBad("invalid_status")
	}
	if in.Comment = clean(in.Comment, 1000); in.Comment == "" {
		return errBad("comment_required")
	}
	body, now := clean(in.Body, 20000), a.unix()
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `UPDATE decisions SET title=?, body=?, status=?, updated_at=? WHERE id=?`, in.Title, body, in.Status, now, id); err != nil {
			return err
		}
		return tx.Exec(c.ctx(), `INSERT INTO decision_revisions (id,decision_id,title,body,status,comment,user_id,created_at) VALUES (?,?,?,?,?,?,?,?)`,
			crypto.NewID(), id, in.Title, body, in.Status, in.Comment, c.U.ID, now)
	})
	if err != nil {
		return err
	}
	var number string
	_ = a.DB.Row(c.ctx(), `SELECT number FROM decisions WHERE id=?`, id).Scan(&number)
	a.logAct(c, true, "decision.revised", "decision", id, number+" "+in.Status)
	a.hub.publish(Event{Type: "decision", ID: id})
	return c.OK()
}

// isUniqueViolation recognises a unique constraint error of either database.
func isUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate key")
}

// csvSafe neutralises spreadsheet formula injection.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func (a *App) hExportDecisions(c *Ctx) error {
	ds, err := a.decisions(c.ctx(), c.U)
	if err != nil {
		return err
	}
	c.W.Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.W.Header().Set("Content-Disposition", `attachment; filename="decisions.csv"`)
	w := csv.NewWriter(c.W)
	_ = w.Write([]string{"number", "date", "status", "title", "text"})
	for _, d := range ds {
		_ = w.Write([]string{d.Number, time.Unix(d.DecidedAt, 0).UTC().Format("2006-01-02"), d.Status, csvSafe(d.Title), csvSafe(d.Body)})
	}
	w.Flush()
	a.logAct(c, false, "decisions.exported", "decision", "", itoa(len(ds)))
	return w.Error()
}
