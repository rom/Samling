package app

import (
	"database/sql"
	"sort"

	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

// Poll kinds:
//   single  one option
//   multi   several options (optionally at most max_choices)
//   yesno   yes / no / abstain (options are created automatically)
//   ranked  rank the options; counted with the Borda method
//   score   give every option 0-5 points
//
// Anonymous polls store ballots without any link to the voter. Participation
// is recorded separately (poll_voters) to prevent double voting, which also
// means an anonymous vote cannot be changed afterwards.

type PollOption struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Votes int    `json:"votes"` // ballots that chose/ranked/scored this option
	Score int    `json:"score"` // sum of points (ranked: Borda points, score: points)
}

type Poll struct {
	ID             string         `json:"id"`
	OwnerID        string         `json:"owner_id"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Kind           string         `json:"kind"`
	Anonymous      bool           `json:"anonymous"`
	ResultsVisible string         `json:"results_visible"` // always | after_vote | after_close
	AllowChange    bool           `json:"allow_change"`
	MaxChoices     int            `json:"max_choices"`
	ClosesAt       int64          `json:"closes_at"`
	ClosedAt       int64          `json:"closed_at"`
	CreatedAt      int64          `json:"created_at"`
	Closed         bool           `json:"closed"`
	Voted          bool           `json:"voted"`
	Voters         int            `json:"voters"`
	Shared         bool           `json:"shared"`
	Level          int            `json:"level"`
	ResultsShown   bool           `json:"results_shown"`
	Options        []PollOption   `json:"options"`
	MyVote         map[string]int `json:"my_vote,omitempty"`
}

var pollKinds = map[string]bool{"single": true, "multi": true, "yesno": true, "ranked": true, "score": true}

const pollCols = `p.id, p.owner_id, p.title, p.description, p.kind, p.anonymous, p.results_visible, p.allow_change, p.max_choices, p.closes_at, p.closed_at, p.created_at,
	(SELECT COUNT(*) FROM poll_voters v WHERE v.poll_id=p.id),
	(SELECT COUNT(*) FROM poll_voters v WHERE v.poll_id=p.id AND v.user_id=?),
	(SELECT COUNT(*) FROM acl a WHERE a.res_type='poll' AND a.res_id=p.id)`

func (a *App) scanPoll(r scanner) (*Poll, error) {
	p := &Poll{Options: []PollOption{}}
	var voted, shares int
	if err := r.Scan(&p.ID, &p.OwnerID, &p.Title, &p.Description, &p.Kind, &p.Anonymous, &p.ResultsVisible, &p.AllowChange, &p.MaxChoices,
		&p.ClosesAt, &p.ClosedAt, &p.CreatedAt, &p.Voters, &voted, &shares); err != nil {
		return nil, err
	}
	p.Voted, p.Shared = voted > 0, shares > 0
	// A poll is closed when closed manually or when its deadline has passed.
	if p.ClosedAt == 0 && p.ClosesAt > 0 && p.ClosesAt <= a.unix() {
		p.ClosedAt = p.ClosesAt
	}
	p.Closed = p.ClosedAt > 0
	return p, nil
}

func (a *App) hPolls(c *Ctx) error {
	vis, args := visible(c.U, "poll", "p")
	out := []*Poll{}
	err := a.DB.Each(c.ctx(), `SELECT `+pollCols+` FROM polls p WHERE `+vis+` ORDER BY p.created_at DESC`, append([]any{c.U.ID}, args...), func(r *sql.Rows) error {
		p, err := a.scanPoll(r)
		out = append(out, p)
		return err
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// loadPoll returns the poll with options and — when the viewer is allowed to
// see them — the results.
func (a *App) loadPoll(c *Ctx, id string) (*Poll, error) {
	lvl, err := a.level(c.ctx(), c.U, "poll", id)
	if err != nil || lvl < LevelRead {
		return nil, errNotFound
	}
	p, err := a.scanPoll(a.DB.Row(c.ctx(), `SELECT `+pollCols+` FROM polls p WHERE p.id=?`, c.U.ID, id))
	if err != nil {
		return nil, err
	}
	p.Level = lvl
	idx := map[string]int{}
	err = a.DB.Each(c.ctx(), `SELECT id, text FROM poll_options WHERE poll_id=? ORDER BY position`, []any{id}, func(r *sql.Rows) error {
		var o PollOption
		if err := r.Scan(&o.ID, &o.Text); err != nil {
			return err
		}
		idx[o.ID] = len(p.Options)
		p.Options = append(p.Options, o)
		return nil
	})
	if err != nil {
		return nil, err
	}
	p.ResultsShown = p.Closed || p.ResultsVisible == "always" || (p.ResultsVisible == "after_vote" && p.Voted)
	if p.ResultsShown {
		n := len(p.Options)
		err = a.DB.Each(c.ctx(), `SELECT bc.option_id, bc.value FROM ballot_choices bc JOIN ballots b ON b.id=bc.ballot_id WHERE b.poll_id=?`, []any{id}, func(r *sql.Rows) error {
			var oid string
			var v int
			if err := r.Scan(&oid, &v); err != nil {
				return err
			}
			i, ok := idx[oid]
			if !ok {
				return nil
			}
			p.Options[i].Votes++
			switch p.Kind {
			case "ranked":
				p.Options[i].Score += n - v // Borda: rank 1 of n gives n-1 points
			case "score":
				p.Options[i].Score += v
			default:
				p.Options[i].Score++
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if !p.Anonymous && p.Voted {
		p.MyVote = map[string]int{}
		err = a.DB.Each(c.ctx(), `SELECT bc.option_id, bc.value FROM ballot_choices bc JOIN ballots b ON b.id=bc.ballot_id WHERE b.poll_id=? AND b.user_id=?`,
			[]any{id, c.U.ID}, func(r *sql.Rows) error {
				var oid string
				var v int
				if err := r.Scan(&oid, &v); err != nil {
					return err
				}
				p.MyVote[oid] = v
				return nil
			})
	}
	return p, err
}

func (a *App) hPoll(c *Ctx) error {
	p, err := a.loadPoll(c, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(p)
}

func (a *App) hCreatePoll(c *Ctx) error {
	var in struct {
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Kind           string   `json:"kind"`
		Options        []string `json:"options"`
		Anonymous      bool     `json:"anonymous"`
		ResultsVisible string   `json:"results_visible"`
		AllowChange    bool     `json:"allow_change"`
		MaxChoices     int      `json:"max_choices"`
		ClosesAt       int64    `json:"closes_at"`
		Shares         []Share  `json:"shares"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if in.Title = clean(in.Title, 200); in.Title == "" {
		return errBad("title_required")
	}
	if !pollKinds[in.Kind] {
		return errBad("invalid_kind")
	}
	if in.ResultsVisible == "" {
		in.ResultsVisible = "after_vote"
	}
	if in.ResultsVisible != "always" && in.ResultsVisible != "after_vote" && in.ResultsVisible != "after_close" {
		return errBad("invalid_results_visible")
	}
	if in.Kind == "yesno" {
		in.Options = []string{"@yes", "@no", "@abstain"} // translated by the client
	}
	var opts []string
	for _, o := range in.Options {
		if o = clean(o, 200); o != "" {
			opts = append(opts, o)
		}
	}
	if len(opts) < 2 || len(opts) > 30 {
		return errBad("invalid_options")
	}
	if in.Kind != "multi" || in.MaxChoices < 0 || in.MaxChoices > len(opts) {
		in.MaxChoices = 0
	}
	if in.Anonymous {
		in.AllowChange = false
	}
	if in.ClosesAt != 0 && in.ClosesAt <= a.unix() {
		return errBad("closes_in_past")
	}
	shares, err := a.createShares(c, in.Shares)
	if err != nil {
		return err
	}
	id := crypto.NewID()
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `INSERT INTO polls (id,owner_id,title,description,kind,anonymous,results_visible,allow_change,max_choices,closes_at,created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`, id, c.U.ID, in.Title, clean(in.Description, 4000), in.Kind, b2i(in.Anonymous), in.ResultsVisible,
			b2i(in.AllowChange), in.MaxChoices, in.ClosesAt, a.unix()); err != nil {
			return err
		}
		for i, o := range opts {
			if err := tx.Exec(c.ctx(), `INSERT INTO poll_options (id,poll_id,text,position) VALUES (?,?,?,?)`, crypto.NewID(), id, o, i+1); err != nil {
				return err
			}
		}
		return setShares(c.ctx(), tx, "poll", id, shares)
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "poll.created", "poll", id, in.Title)
	a.hub.publish(Event{"poll", id})
	return c.JSON(map[string]string{"id": id})
}

func (a *App) hVote(c *Ctx) error {
	id := c.Param("id")
	// choices maps option id -> value. single/multi/yesno: value 1,
	// ranked: rank (1 = best, all options), score: 0-5 points (all options).
	var in struct {
		Choices map[string]int `json:"choices"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	p, err := a.loadPoll(c, id)
	if err != nil {
		return err
	}
	if p.Closed {
		return errBad("poll_closed")
	}
	valid := map[string]bool{}
	for _, o := range p.Options {
		valid[o.ID] = true
	}
	for oid := range in.Choices {
		if !valid[oid] {
			return errBad("invalid_vote")
		}
	}
	n := len(in.Choices)
	switch p.Kind {
	case "single", "yesno":
		if n != 1 {
			return errBad("invalid_vote")
		}
	case "multi":
		if n < 1 || (p.MaxChoices > 0 && n > p.MaxChoices) {
			return errBad("invalid_vote")
		}
	case "ranked": // must be a permutation 1..n of all options
		if n != len(p.Options) {
			return errBad("invalid_vote")
		}
		ranks := make([]int, 0, n)
		for _, v := range in.Choices {
			ranks = append(ranks, v)
		}
		sort.Ints(ranks)
		for i, v := range ranks {
			if v != i+1 {
				return errBad("invalid_vote")
			}
		}
	case "score":
		if n != len(p.Options) {
			return errBad("invalid_vote")
		}
		for _, v := range in.Choices {
			if v < 0 || v > 5 {
				return errBad("invalid_vote")
			}
		}
	}
	if p.Kind == "single" || p.Kind == "yesno" || p.Kind == "multi" {
		for k := range in.Choices {
			in.Choices[k] = 1
		}
	}
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		var voted int
		if err := tx.Row(ctx, `SELECT COUNT(*) FROM poll_voters WHERE poll_id=? AND user_id=?`, id, c.U.ID).Scan(&voted); err != nil {
			return err
		}
		if voted > 0 {
			if !p.AllowChange || p.Anonymous {
				return errBad("already_voted")
			}
			if err := tx.Exec(ctx, `DELETE FROM ballots WHERE poll_id=? AND user_id=?`, id, c.U.ID); err != nil {
				return err
			}
		} else if err := tx.Exec(ctx, `INSERT INTO poll_voters (poll_id,user_id) VALUES (?,?)`, id, c.U.ID); err != nil {
			return err
		}
		// Ballot ids are fully random (not time-ordered) and anonymous ballots
		// carry no voter and no timestamp, so they cannot be correlated.
		bid, uid, at := crypto.RandomID(), c.U.ID, a.unix()
		if p.Anonymous {
			uid, at = "", 0
		}
		if err := tx.Exec(ctx, `INSERT INTO ballots (id,poll_id,user_id,created_at) VALUES (?,?,?,?)`, bid, id, uid, at); err != nil {
			return err
		}
		for oid, v := range in.Choices {
			if err := tx.Exec(ctx, `INSERT INTO ballot_choices (ballot_id,option_id,value) VALUES (?,?,?)`, bid, oid, v); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	a.logAct(c, false, "poll.voted", "poll", id, p.Title)
	a.hub.publish(Event{"poll", id})
	return c.OK()
}

func (a *App) hClosePoll(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "poll", id, LevelManage); err != nil {
		return err
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE polls SET closed_at=? WHERE id=? AND closed_at=0`, a.unix(), id); err != nil {
		return err
	}
	a.logAct(c, false, "poll.closed", "poll", id, "")
	a.hub.publish(Event{"poll", id})
	return c.OK()
}

func (a *App) hDeletePoll(c *Ctx) error {
	id := c.Param("id")
	if err := a.need(c, "poll", id, LevelManage); err != nil {
		return err
	}
	if err := a.deleteRes(c.ctx(), "poll", id); err != nil {
		return err
	}
	a.logAct(c, true, "poll.deleted", "poll", id, "")
	return c.OK()
}
