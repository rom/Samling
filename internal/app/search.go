package app

import (
	"database/sql"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Search covers everything the caller can read — and nothing else: every
// query below is restricted with the same visibility condition as the
// normal list endpoints.
//
// Modes: plain (substring), wildcard (* and ?), regex. All three are
// compiled to Go regular expressions (RE2), which run in linear time, so a
// hostile pattern cannot cause catastrophic backtracking (ReDoS).

type SearchHit struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Link    string `json:"link"` // id of the containing resource (channel, list, board, meeting)
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	At      int64  `json:"at"`
}

const (
	searchPerType = 50
	searchScanMax = 50000
)

func compileSearch(q, mode string, caseSensitive bool) (*regexp.Regexp, error) {
	if q == "" || len(q) > 300 {
		return nil, errBad("invalid_query")
	}
	var pat string
	switch mode {
	case "", "plain":
		pat = regexp.QuoteMeta(q)
	case "wildcard":
		pat = regexp.QuoteMeta(q)
		pat = strings.ReplaceAll(pat, `\*`, `.*`)
		pat = strings.ReplaceAll(pat, `\?`, `.`)
	case "regex":
		pat = q
	default:
		return nil, errBad("invalid_mode")
	}
	// Multi-line mode: ^ and $ anchor at the start/end of a title or text.
	if caseSensitive {
		pat = "(?m)" + pat
	} else {
		pat = "(?mi)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, errBad("invalid_regex")
	}
	return re, nil
}

func snippet(text string, loc []int) string {
	start, end := max(loc[0]-60, 0), min(loc[1]+90, len(text))
	for start > 0 && !utf8.RuneStart(text[start]) {
		start--
	}
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end++
	}
	s := strings.Join(strings.Fields(text[start:end]), " ")
	if start > 0 {
		s = "…" + s
	}
	if end < len(text) {
		s += "…"
	}
	return s
}

func (a *App) hSearch(c *Ctx) error {
	re, err := compileSearch(strings.TrimSpace(c.Query("q")), c.Query("mode"), c.Query("case") == "1")
	if err != nil {
		return err
	}
	from, to := c.QueryInt("from", 0), c.QueryInt("to", 1<<40)
	want := map[string]bool{}
	for _, t := range strings.Split(c.Query("types"), ",") {
		if t != "" {
			want[t] = true
		}
	}
	u := c.U
	hits := []SearchHit{}
	counts := map[string]int{}

	// run executes a query returning (id, link, ts, title, text) rows and
	// collects those matching the pattern.
	run := func(typ, q string, args ...any) error {
		if len(want) > 0 && !want[typ] {
			return nil
		}
		scanned := 0
		return a.DB.Each(c.ctx(), q, args, func(r *sql.Rows) error {
			if scanned++; scanned > searchScanMax || counts[typ] >= searchPerType {
				return nil
			}
			var h SearchHit
			var text string
			if err := r.Scan(&h.ID, &h.Link, &h.At, &h.Title, &text); err != nil {
				return err
			}
			full := h.Title
			if text != "" {
				full += "\n" + text
			}
			if loc := re.FindStringIndex(full); loc != nil {
				h.Type, h.Snippet = typ, snippet(full, loc)
				hits = append(hits, h)
				counts[typ]++
			}
			return nil
		})
	}
	vis := func(typ, alias string) (string, []any) { return visible(u, typ, alias) }
	tr := []any{from, to}

	v, va := vis("channel", "ch")
	if err := run("message", `SELECT m.id, m.channel_id, m.created_at, '', m.body || COALESCE((SELECT ' ' || MIN(f.name) FROM message_files mf JOIN files f ON f.id=mf.file_id WHERE mf.message_id=m.id), '')
		FROM messages m JOIN channels ch ON ch.id=m.channel_id
		WHERE m.deleted_at=0 AND m.created_at BETWEEN ? AND ? AND `+v+` ORDER BY m.created_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	if err := run("channel", `SELECT ch.id, ch.id, ch.created_at, ch.name, ch.topic FROM channels ch WHERE ch.kind='channel' AND ch.created_at BETWEEN ? AND ? AND `+v, append(tr, va...)...); err != nil {
		return err
	}
	v, va = vis("document", "d")
	if err := run("document", `SELECT d.id, d.id, d.updated_at, d.name, d.folder || ' ' || d.description FROM documents d
		WHERE d.updated_at BETWEEN ? AND ? AND `+v+` ORDER BY d.updated_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	v, va = vis("list", "l")
	if err := run("list", `SELECT l.id, l.id, l.updated_at, l.title, '' FROM lists l WHERE l.updated_at BETWEEN ? AND ? AND `+v+` ORDER BY l.updated_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	if err := run("list", `SELECT i.id, l.id, i.created_at, l.title, i.text FROM list_items i JOIN lists l ON l.id=i.list_id
		WHERE i.created_at BETWEEN ? AND ? AND `+v+` ORDER BY i.created_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	v, va = vis("poll", "p")
	if err := run("poll", `SELECT p.id, p.id, p.created_at, p.title, p.description FROM polls p WHERE p.created_at BETWEEN ? AND ? AND `+v+` ORDER BY p.created_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	if err := run("poll", `SELECT p.id, p.id, p.created_at, p.title, o.text FROM poll_options o JOIN polls p ON p.id=o.poll_id
		WHERE p.created_at BETWEEN ? AND ? AND `+v+` ORDER BY p.created_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	v, va = vis("event", "e")
	if err := run("event", `SELECT e.id, e.id, e.starts_at, e.title, e.location || ' ' || e.description FROM events e
		WHERE e.starts_at BETWEEN ? AND ? AND `+v+` ORDER BY e.starts_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	v, va = vis("meeting", "m")
	if err := run("meeting", `SELECT m.id, m.id, m.starts_at, m.title, m.description || ' ' || m.location || ' ' || m.notes FROM meetings m
		WHERE m.starts_at BETWEEN ? AND ? AND `+v+` ORDER BY m.starts_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	if err := run("meeting", `SELECT ai.id, m.id, m.starts_at, m.title, ai.title || ' ' || ai.description || ' ' || ai.notes FROM agenda_items ai JOIN meetings m ON m.id=ai.meeting_id
		WHERE m.starts_at BETWEEN ? AND ? AND `+v+` ORDER BY m.starts_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	v, va = vis("decision", "d")
	if err := run("decision", `SELECT d.id, d.id, d.decided_at, d.number || ' ' || d.title, d.body FROM decisions d
		WHERE d.decided_at BETWEEN ? AND ? AND `+v+` ORDER BY d.decided_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	v, va = vis("board", "b")
	if err := run("board", `SELECT b.id, b.id, b.updated_at, b.title, b.description FROM boards b WHERE b.updated_at BETWEEN ? AND ? AND `+v, append(tr, va...)...); err != nil {
		return err
	}
	if err := run("card", `SELECT k.id, b.id, k.updated_at, k.title, k.description FROM cards k JOIN boards b ON b.id=k.board_id
		WHERE k.updated_at BETWEEN ? AND ? AND `+v+` ORDER BY k.updated_at DESC`, append(tr, va...)...); err != nil {
		return err
	}
	// People: names always, contact details only when visible to the caller.
	if len(want) == 0 || want["user"] {
		people, err := a.directory(c.ctx(), u)
		if err != nil {
			return err
		}
		for _, p := range people {
			full := p.DisplayName + "\n" + p.Username + " " + p.Email + " " + p.Phone
			if loc := re.FindStringIndex(full); loc != nil && counts["user"] < searchPerType {
				hits = append(hits, SearchHit{Type: "user", ID: p.ID, Link: p.ID, Title: p.DisplayName,
					Snippet: strings.TrimSpace(strings.Join([]string{"@" + p.Username, p.Email, p.Phone}, " "))})
				counts["user"]++
			}
		}
	}
	// Dedupe (e.g. a list matching through several items keeps each item,
	// but identical type+id pairs are collapsed).
	seen := map[string]bool{}
	out := hits[:0]
	for _, h := range hits {
		if k := h.Type + "/" + h.ID; !seen[k] {
			seen[k] = true
			out = append(out, h)
		}
	}
	return c.JSON(out)
}
