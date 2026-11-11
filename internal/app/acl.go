package app

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"

	"github.com/rom/samling/internal/db"
)

func subtleEq(a, b []byte) bool { return subtle.ConstantTimeCompare(a, b) == 1 }

// Access model
//
// Every shareable resource has an owner and a list of ACL entries (shares).
// A share grants a level to a subject: everyone ("all"), a group, a role or a
// single user. A resource without shares is private to its owner.
//
// Administrators get NO implicit access to private content. The
// content.moderate permission gives manage rights on resources that are
// already shared (so shared material never becomes orphaned), but never on
// private resources or direct messages.

const (
	LevelNone   = 0
	LevelRead   = 1
	LevelWrite  = 2
	LevelManage = 3
)

// resTables maps resource types to their table.
var resTables = map[string]string{
	"channel": "channels", "document": "documents", "list": "lists", "poll": "polls",
	"event": "events", "meeting": "meetings", "decision": "decisions", "board": "boards",
}

type Share struct {
	Type  string `json:"type"` // all | group | role | user
	ID    string `json:"id"`
	Level int    `json:"level"`
	Name  string `json:"name,omitempty"`
}

// aclMatch is the SQL condition "ACL row a applies to user ?" (3 args: uid x3).
const aclMatch = `(a.subj_type='all'
 OR (a.subj_type='user' AND a.subj_id=?)
 OR (a.subj_type='group' AND a.subj_id IN (SELECT group_id FROM group_members WHERE user_id=?))
 OR (a.subj_type='role' AND a.subj_id IN (SELECT role_id FROM user_roles WHERE user_id=?)))`

// visible returns a SQL condition (and its args) that is true for rows of
// the given resource type, aliased as alias, which the user may read.
func visible(u *User, resType, alias string) (string, []any) {
	cond := `(` + alias + `.owner_id=? OR EXISTS (SELECT 1 FROM acl a WHERE a.res_type='` + resType + `' AND a.res_id=` + alias + `.id AND ` + aclMatch + `))`
	return cond, []any{u.ID, u.ID, u.ID, u.ID}
}

// level returns the user's access level on a resource (errNotFound when the
// resource does not exist).
func (a *App) level(ctx context.Context, u *User, resType, id string) (int, error) {
	table, ok := resTables[resType]
	if !ok {
		return 0, errNotFound
	}
	var owner string
	err := a.DB.Row(ctx, `SELECT owner_id FROM `+table+` WHERE id=?`, id).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errNotFound
	}
	if err != nil {
		return 0, err
	}
	if owner == u.ID {
		return LevelManage, nil
	}
	var lvl, shares int
	err = a.DB.Row(ctx, `SELECT COALESCE(MAX(CASE WHEN `+aclMatch+` THEN a.level ELSE 0 END),0), COUNT(*) FROM acl a WHERE a.res_type=? AND a.res_id=?`,
		u.ID, u.ID, u.ID, resType, id).Scan(&lvl, &shares)
	if err != nil {
		return 0, err
	}
	if shares > 0 && u.Can(PermModerate) && !a.isDM(ctx, resType, id) {
		return LevelManage, nil
	}
	return lvl, nil
}

func (a *App) isDM(ctx context.Context, resType, id string) bool {
	if resType != "channel" {
		return false
	}
	var kind string
	_ = a.DB.Row(ctx, `SELECT kind FROM channels WHERE id=?`, id).Scan(&kind)
	return kind == "dm"
}

// need checks that the user has at least the wanted level. To avoid leaking
// the existence of resources, missing read access is reported as not found.
func (a *App) need(c *Ctx, resType, id string, want int) error {
	lvl, err := a.level(c.ctx(), c.U, resType, id)
	if err != nil {
		return err
	}
	if lvl < LevelRead {
		return errNotFound
	}
	if lvl < want {
		return errForbidden
	}
	return nil
}

// validateShares checks subjects: a user may only share with global groups
// or their own local groups.
func (a *App) validateShares(ctx context.Context, u *User, shares []Share) error {
	if len(shares) > 200 {
		return errBad("too_many_shares")
	}
	for _, s := range shares {
		if s.Level != LevelRead && s.Level != LevelWrite {
			return errBad("invalid_share")
		}
		var n int
		var err error
		switch s.Type {
		case "all":
			if s.ID != "" {
				return errBad("invalid_share")
			}
			continue
		case "user":
			err = a.DB.Row(ctx, `SELECT COUNT(*) FROM users WHERE id=? AND deleted_at=0`, s.ID).Scan(&n)
		case "role":
			err = a.DB.Row(ctx, `SELECT COUNT(*) FROM roles WHERE id=?`, s.ID).Scan(&n)
		case "group":
			err = a.DB.Row(ctx, `SELECT COUNT(*) FROM groups WHERE id=? AND (scope='global' OR owner_id=?)`, s.ID, u.ID).Scan(&n)
		default:
			return errBad("invalid_share")
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return errBad("invalid_share")
		}
	}
	return nil
}

func setShares(ctx context.Context, tx db.Q, resType, id string, shares []Share) error {
	if err := tx.Exec(ctx, `DELETE FROM acl WHERE res_type=? AND res_id=?`, resType, id); err != nil {
		return err
	}
	for _, s := range shares {
		if err := tx.Exec(ctx, `INSERT INTO acl (res_type,res_id,subj_type,subj_id,level) VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING`,
			resType, id, s.Type, s.ID, s.Level); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) getShares(ctx context.Context, resType, id string) ([]Share, error) {
	out := []Share{}
	err := a.DB.Each(ctx, `SELECT a.subj_type, a.subj_id, a.level,
		COALESCE(CASE a.subj_type WHEN 'user' THEN (SELECT display_name FROM users WHERE id=a.subj_id)
		 WHEN 'group' THEN (SELECT name FROM groups WHERE id=a.subj_id)
		 WHEN 'role' THEN (SELECT name FROM roles WHERE id=a.subj_id) END, '')
		FROM acl a WHERE a.res_type=? AND a.res_id=? ORDER BY a.subj_type, a.subj_id`, []any{resType, id}, func(r *sql.Rows) error {
		var s Share
		if err := r.Scan(&s.Type, &s.ID, &s.Level, &s.Name); err != nil {
			return err
		}
		out = append(out, s)
		return nil
	})
	return out, err
}

func (a *App) hGetACL(c *Ctx) error {
	t, id := c.Param("type"), c.Param("id")
	if err := a.need(c, t, id, LevelRead); err != nil {
		return err
	}
	shares, err := a.getShares(c.ctx(), t, id)
	if err != nil {
		return err
	}
	// Local groups are private to their owner: hide their names from others.
	for i := range shares {
		if shares[i].Type == "group" {
			var scope, owner string
			_ = a.DB.Row(c.ctx(), `SELECT scope, owner_id FROM groups WHERE id=?`, shares[i].ID).Scan(&scope, &owner)
			if scope == "local" && owner != c.U.ID {
				shares[i].Name, shares[i].ID = "", ""
			}
		}
	}
	return c.JSON(shares)
}

func (a *App) hSetACL(c *Ctx) error {
	t, id := c.Param("type"), c.Param("id")
	var shares []Share
	if err := c.Bind(&shares); err != nil {
		return err
	}
	if err := a.need(c, t, id, LevelManage); err != nil {
		return err
	}
	if a.isDM(c.ctx(), t, id) {
		return errForbidden
	}
	// Entries for other people's local groups come back blanked from GET;
	// keep the stored ones instead of dropping them.
	old, err := a.getShares(c.ctx(), t, id)
	if err != nil {
		return err
	}
	kept := []Share{}
	for _, s := range shares {
		if s.Type == "group" && s.ID == "" {
			continue
		}
		kept = append(kept, Share{Type: s.Type, ID: s.ID, Level: s.Level})
	}
	if err := a.validateShares(c.ctx(), c.U, kept); err != nil {
		return err
	}
	for _, s := range old {
		if s.Type == "group" {
			var scope, owner string
			_ = a.DB.Row(c.ctx(), `SELECT scope, owner_id FROM groups WHERE id=?`, s.ID).Scan(&scope, &owner)
			if scope == "local" && owner != c.U.ID {
				kept = append(kept, Share{Type: s.Type, ID: s.ID, Level: s.Level})
			}
		}
	}
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := setShares(c.ctx(), tx, t, id, kept); err != nil {
			return err
		}
		if t == "meeting" { // the calendar entry of a meeting follows the meeting
			return syncMeetingEventACL(c.ctx(), tx, id)
		}
		return nil
	})
	if err != nil {
		return err
	}
	a.logAct(c, true, "sharing.changed", t, id, "")
	a.hub.publish(Event{Type: t, ID: id})
	return c.OK()
}

// createShares validates the shares given when a resource is created.
func (a *App) createShares(c *Ctx, shares []Share) ([]Share, error) {
	if shares == nil {
		shares = []Share{}
	}
	return shares, a.validateShares(c.ctx(), c.U, shares)
}
