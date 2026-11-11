package app

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

// Calendar events. Times are unix seconds (UTC). All-day events store the
// UTC midnight of their first and last day (inclusive).

type CalEvent struct {
	ID          string `json:"id"`
	OwnerID     string `json:"owner_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	URL         string `json:"url"`
	StartsAt    int64  `json:"starts_at"`
	EndsAt      int64  `json:"ends_at"`
	AllDay      bool   `json:"all_day"`
	MeetingID   string `json:"meeting_id"`
	Shared      bool   `json:"shared"`
	UID         string `json:"-"`
	UpdatedAt   int64  `json:"-"`
}

// cleanURL only accepts http(s) and mailto links, so stored links can never
// be javascript: or similar when rendered as anchors.
func cleanURL(s string) (string, error) {
	if s = clean(s, 1000); s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "mailto") {
		return "", errBad("invalid_url")
	}
	return s, nil
}

func (a *App) events(ctx context.Context, u *User, from, to int64) ([]CalEvent, error) {
	vis, args := visible(u, "event", "e")
	out := []CalEvent{}
	// All-day events end at the start of their last day: add a day of slack.
	err := a.DB.Each(ctx, `SELECT e.id, e.owner_id, e.title, e.description, e.location, e.url, e.starts_at, e.ends_at, e.all_day, e.meeting_id, e.uid, e.updated_at,
		(SELECT COUNT(*) FROM acl a WHERE a.res_type='event' AND a.res_id=e.id)
		FROM events e WHERE e.starts_at <= ? AND e.ends_at + 86400 >= ? AND `+vis+` ORDER BY e.starts_at, e.id LIMIT 5000`,
		append([]any{to, from}, args...), func(r *sql.Rows) error {
			var e CalEvent
			var shares int
			if err := r.Scan(&e.ID, &e.OwnerID, &e.Title, &e.Description, &e.Location, &e.URL, &e.StartsAt, &e.EndsAt, &e.AllDay, &e.MeetingID, &e.UID, &e.UpdatedAt, &shares); err != nil {
				return err
			}
			e.Shared = shares > 0
			out = append(out, e)
			return nil
		})
	return out, err
}

func (a *App) hCalendar(c *Ctx) error {
	out, err := a.events(c.ctx(), c.U, c.QueryInt("from", 0), c.QueryInt("to", 1<<40))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

type eventIn struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Location    string  `json:"location"`
	URL         string  `json:"url"`
	StartsAt    int64   `json:"starts_at"`
	EndsAt      int64   `json:"ends_at"`
	AllDay      bool    `json:"all_day"`
	Shares      []Share `json:"shares"`
}

func (in *eventIn) check() (err error) {
	if in.Title = clean(in.Title, 200); in.Title == "" {
		return errBad("title_required")
	}
	in.Description, in.Location = clean(in.Description, 4000), clean(in.Location, 300)
	if in.URL, err = cleanURL(in.URL); err != nil {
		return err
	}
	if in.StartsAt <= 0 || in.EndsAt < in.StartsAt {
		return errBad("invalid_time")
	}
	return nil
}

func (a *App) hCreateEvent(c *Ctx) error {
	var in eventIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := in.check(); err != nil {
		return err
	}
	shares, err := a.createShares(c, in.Shares)
	if err != nil {
		return err
	}
	id, now := crypto.NewID(), a.unix()
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `INSERT INTO events (id,owner_id,title,description,location,url,starts_at,ends_at,all_day,uid,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, id, c.U.ID, in.Title, in.Description, in.Location, in.URL, in.StartsAt, in.EndsAt, b2i(in.AllDay), id+"@samling", now, now); err != nil {
			return err
		}
		return setShares(c.ctx(), tx, "event", id, shares)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "event.created", "event", id, in.Title)
	a.hub.publish(Event{"event", id})
	return c.JSON(map[string]string{"id": id})
}

func (a *App) hUpdateEvent(c *Ctx) error {
	id := c.Param("id")
	var in eventIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.need(c, "event", id, LevelWrite); err != nil {
		return err
	}
	if err := in.check(); err != nil {
		return err
	}
	var meeting string
	_ = a.DB.Row(c.ctx(), `SELECT meeting_id FROM events WHERE id=?`, id).Scan(&meeting)
	if meeting != "" {
		return errBad("edit_via_meeting") // the meeting is the source of truth
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE events SET title=?, description=?, location=?, url=?, starts_at=?, ends_at=?, all_day=?, updated_at=? WHERE id=?`,
		in.Title, in.Description, in.Location, in.URL, in.StartsAt, in.EndsAt, b2i(in.AllDay), a.unix(), id); err != nil {
		return err
	}
	a.logAct(c, false, "event.updated", "event", id, in.Title)
	a.hub.publish(Event{"event", id})
	return c.OK()
}

func (a *App) hDeleteEvent(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "event", id, LevelManage); err != nil {
		return err
	}
	var meeting string
	_ = a.DB.Row(c.ctx(), `SELECT meeting_id FROM events WHERE id=?`, id).Scan(&meeting)
	if meeting != "" {
		return errBad("edit_via_meeting")
	}
	if err := a.deleteRes(c.ctx(), "event", id); err != nil {
		return err
	}
	a.logAct(c, false, "event.deleted", "event", id, "")
	return c.OK()
}

// ---------- iCalendar (RFC 5545) export / import ----------

var icsEsc = strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", "")

// icsLine writes a content line folded at 75 octets.
func icsLine(w io.Writer, name, value string) {
	line := name + ":" + value
	for len(line) > 75 {
		cut := 75
		for cut > 0 && line[cut]&0xC0 == 0x80 { // do not split a UTF-8 sequence
			cut--
		}
		fmt.Fprintf(w, "%s\r\n", line[:cut])
		line = " " + line[cut:]
	}
	fmt.Fprintf(w, "%s\r\n", line)
}

func writeICS(w io.Writer, org string, events []CalEvent) {
	const stamp = "20060102T150405Z"
	icsLine(w, "BEGIN", "VCALENDAR")
	icsLine(w, "VERSION", "2.0")
	icsLine(w, "PRODID", "-//Samling//EN")
	icsLine(w, "CALSCALE", "GREGORIAN")
	icsLine(w, "X-WR-CALNAME", icsEsc.Replace(org))
	for _, e := range events {
		icsLine(w, "BEGIN", "VEVENT")
		icsLine(w, "UID", e.UID)
		icsLine(w, "DTSTAMP", time.Unix(e.UpdatedAt, 0).UTC().Format(stamp))
		if e.AllDay {
			icsLine(w, "DTSTART;VALUE=DATE", time.Unix(e.StartsAt, 0).UTC().Format("20060102"))
			icsLine(w, "DTEND;VALUE=DATE", time.Unix(e.EndsAt, 0).UTC().AddDate(0, 0, 1).Format("20060102")) // exclusive
		} else {
			icsLine(w, "DTSTART", time.Unix(e.StartsAt, 0).UTC().Format(stamp))
			icsLine(w, "DTEND", time.Unix(e.EndsAt, 0).UTC().Format(stamp))
		}
		icsLine(w, "SUMMARY", icsEsc.Replace(e.Title))
		if e.Description != "" {
			icsLine(w, "DESCRIPTION", icsEsc.Replace(e.Description))
		}
		if e.Location != "" {
			icsLine(w, "LOCATION", icsEsc.Replace(e.Location))
		}
		if e.URL != "" {
			icsLine(w, "URL", e.URL)
		}
		icsLine(w, "END", "VEVENT")
	}
	icsLine(w, "END", "VCALENDAR")
}

func (a *App) hExportICS(c *Ctx) error {
	evs, err := a.events(c.ctx(), c.U, c.QueryInt("from", 0), c.QueryInt("to", 1<<40))
	if err != nil {
		return err
	}
	if id := c.Query("id"); id != "" { // single event
		one := evs[:0]
		for _, e := range evs {
			if e.ID == id {
				one = append(one, e)
			}
		}
		evs = one
	}
	c.W.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	c.W.Header().Set("Content-Disposition", `attachment; filename="samling.ics"`)
	writeICS(c.W, a.Cfg.OrgName, evs)
	a.logAct(c, false, "calendar.exported", "event", "", itoa(len(evs)))
	return nil
}

func icsUnescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n', 'N':
				b.WriteByte('\n')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseICSTime handles DATE, UTC date-time, TZID date-time and floating time.
func parseICSTime(params, value string) (t time.Time, allDay bool, err error) {
	loc := time.UTC
	for _, p := range strings.Split(params, ";") {
		if k, v, ok := strings.Cut(p, "="); ok && strings.EqualFold(k, "TZID") {
			if l, e := time.LoadLocation(strings.Trim(v, `"`)); e == nil {
				loc = l
			}
		}
	}
	switch {
	case len(value) == 8:
		t, err = time.ParseInLocation("20060102", value, time.UTC)
		return t, true, err
	case strings.HasSuffix(value, "Z"):
		t, err = time.Parse("20060102T150405Z", value)
	default:
		t, err = time.ParseInLocation("20060102T150405", value, loc)
	}
	return t, false, err
}

// ParseICS extracts the VEVENTs of an iCalendar stream. Recurrence rules are
// not expanded: a recurring event is imported as its first occurrence.
func ParseICS(r io.Reader) ([]CalEvent, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	var lines []string
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) && len(lines) > 0 {
			lines[len(lines)-1] += l[1:] // unfold
		} else {
			lines = append(lines, l)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	var out []CalEvent
	var cur *CalEvent
	var hasEnd bool
	depth := 0 // nested components inside VEVENT (VALARM) are skipped
	for _, l := range lines {
		head, value, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		name, params, _ := strings.Cut(head, ";")
		name = strings.ToUpper(name)
		switch {
		case name == "BEGIN" && strings.EqualFold(value, "VEVENT"):
			cur, hasEnd, depth = &CalEvent{}, false, 0
		case name == "BEGIN" && cur != nil:
			depth++
		case name == "END" && cur != nil && depth > 0:
			depth--
		case name == "END" && strings.EqualFold(value, "VEVENT") && cur != nil:
			if cur.StartsAt > 0 && cur.Title != "" {
				if !hasEnd {
					cur.EndsAt = cur.StartsAt
					if !cur.AllDay {
						cur.EndsAt += 3600
					}
				}
				if cur.EndsAt < cur.StartsAt {
					cur.EndsAt = cur.StartsAt
				}
				out = append(out, *cur)
			}
			cur = nil
		case cur == nil || depth > 0:
		case name == "SUMMARY":
			cur.Title = clean(icsUnescape(value), 200)
		case name == "DESCRIPTION":
			cur.Description = clean(icsUnescape(value), 4000)
		case name == "LOCATION":
			cur.Location = clean(icsUnescape(value), 300)
		case name == "URL":
			cur.URL, _ = cleanURL(value)
		case name == "UID":
			cur.UID = clean(value, 300)
		case name == "DTSTART":
			if t, allDay, err := parseICSTime(params, value); err == nil {
				cur.StartsAt, cur.AllDay = t.Unix(), allDay
			}
		case name == "DTEND":
			if t, allDay, err := parseICSTime(params, value); err == nil {
				cur.EndsAt, hasEnd = t.Unix(), true
				if allDay { // DTEND is exclusive for dates
					cur.EndsAt -= 86400
				}
			}
		}
		if len(out) >= 2000 {
			return nil, errBad("too_many_events")
		}
	}
	return out, nil
}

// hImportICS imports an .ics file as private events of the caller. Events
// that were imported before (same UID) are updated instead of duplicated.
func (a *App) hImportICS(c *Ctx) error {
	evs, err := ParseICS(http.MaxBytesReader(c.W, c.R.Body, 5<<20))
	if err != nil {
		return errBad("invalid_ics")
	}
	now := a.unix()
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		for _, e := range evs {
			var id string
			if e.UID != "" {
				_ = tx.Row(c.ctx(), `SELECT id FROM events WHERE uid=? AND owner_id=? AND meeting_id=''`, e.UID, c.U.ID).Scan(&id)
			}
			if id != "" {
				if err := tx.Exec(c.ctx(), `UPDATE events SET title=?, description=?, location=?, url=?, starts_at=?, ends_at=?, all_day=?, updated_at=? WHERE id=?`,
					e.Title, e.Description, e.Location, e.URL, e.StartsAt, e.EndsAt, b2i(e.AllDay), now, id); err != nil {
					return err
				}
				continue
			}
			id = crypto.NewID()
			if e.UID == "" {
				e.UID = id + "@samling"
			}
			if err := tx.Exec(c.ctx(), `INSERT INTO events (id,owner_id,title,description,location,url,starts_at,ends_at,all_day,uid,created_at,updated_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, id, c.U.ID, e.Title, e.Description, e.Location, e.URL, e.StartsAt, e.EndsAt, b2i(e.AllDay), e.UID, now, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "calendar.imported", "event", "", itoa(len(evs)))
	return c.JSON(map[string]int{"imported": len(evs)})
}
