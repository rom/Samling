package app

import (
	"context"
	"database/sql"
	"strings"

	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

// Person is the directory view of a user. Contact details are only included
// when the user has chosen to show them or the viewer manages accounts.
type Person struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email,omitempty"`
	Phone       string   `json:"phone,omitempty"`
	Roles       []string `json:"roles"`
	Avatar      string   `json:"avatar,omitempty"`
	// Profile holds the optional profile fields the person shares.
	Profile map[string]string `json:"profile,omitempty"`
}

func (a *App) directory(ctx context.Context, viewer *User) ([]Person, error) {
	roles := map[string][]string{}
	err := a.DB.Each(ctx, `SELECT user_id, role_id FROM user_roles ORDER BY role_id`, nil, func(r *sql.Rows) error {
		var u, role string
		if err := r.Scan(&u, &role); err != nil {
			return err
		}
		roles[u] = append(roles[u], role)
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := []Person{}
	err = a.DB.Each(ctx, `SELECT id, username, display_name, email_enc, phone_enc, contact_visible, profile_enc, profile_shared, avatar FROM users WHERE deleted_at=0 AND disabled=0 ORDER BY LOWER(display_name)`,
		nil, func(r *sql.Rows) error {
			var p Person
			var e, ph, profEnc, profShared string
			var vis bool
			if err := r.Scan(&p.ID, &p.Username, &p.DisplayName, &e, &ph, &vis, &profEnc, &profShared, &p.Avatar); err != nil {
				return err
			}
			// Each optional field is shown only when its owner shares it
			// (account managers and the person see all of them).
			profile, shared := a.decodeProfile(p.ID, profEnc, profShared)
			for k, v := range profile {
				if v != "" && (shared[k] || viewer.Can(PermUsers) || viewer.ID == p.ID) {
					if p.Profile == nil {
						p.Profile = map[string]string{}
					}
					p.Profile[k] = v
				}
			}
			if vis || viewer.Can(PermUsers) || viewer.ID == p.ID {
				p.Email, _ = a.Keys.DecryptField(e, "users.email", p.ID)
				p.Phone, _ = a.Keys.DecryptField(ph, "users.phone", p.ID)
			}
			if p.Roles = roles[p.ID]; p.Roles == nil {
				p.Roles = []string{}
			}
			out = append(out, p)
			return nil
		})
	return out, err
}

func (a *App) hUsers(c *Ctx) error {
	out, err := a.directory(c.ctx(), c.U)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// ---------- roles ----------

type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Builtin     bool     `json:"builtin"`
	Perms       []string `json:"perms"`
}

func (a *App) roles(ctx context.Context) ([]Role, error) {
	out := []Role{}
	idx := map[string]int{}
	err := a.DB.Each(ctx, `SELECT id, name, description, builtin FROM roles ORDER BY builtin DESC, name`, nil, func(r *sql.Rows) error {
		ro := Role{Perms: []string{}}
		if err := r.Scan(&ro.ID, &ro.Name, &ro.Description, &ro.Builtin); err != nil {
			return err
		}
		idx[ro.ID] = len(out)
		out = append(out, ro)
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = a.DB.Each(ctx, `SELECT role_id, perm FROM role_perms ORDER BY perm`, nil, func(r *sql.Rows) error {
		var id, p string
		if err := r.Scan(&id, &p); err != nil {
			return err
		}
		if i, ok := idx[id]; ok {
			out[i].Perms = append(out[i].Perms, p)
		}
		return nil
	})
	return out, err
}

func (a *App) hRoles(c *Ctx) error {
	out, err := a.roles(c.ctx())
	if err != nil {
		return err
	}
	return c.JSON(map[string]any{"roles": out, "permissions": AllPerms})
}

type roleIn struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Perms       []string `json:"perms"`
}

func (in *roleIn) check() error {
	if in.Name = clean(in.Name, 60); in.Name == "" {
		return errBad("name_required")
	}
	in.Description = clean(in.Description, 300)
	for _, p := range in.Perms {
		ok := false
		for _, k := range AllPerms {
			ok = ok || k == p
		}
		if !ok {
			return errBad("unknown_permission")
		}
	}
	return nil
}

func setRolePerms(ctx context.Context, tx *db.Tx, id string, perms []string) error {
	if err := tx.Exec(ctx, `DELETE FROM role_perms WHERE role_id=?`, id); err != nil {
		return err
	}
	for _, p := range perms {
		if err := tx.Exec(ctx, `INSERT INTO role_perms (role_id,perm) VALUES (?,?) ON CONFLICT DO NOTHING`, id, p); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) hCreateRole(c *Ctx) error {
	var in roleIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := in.check(); err != nil {
		return err
	}
	var n int
	if err := a.DB.Row(c.ctx(), `SELECT COUNT(*) FROM roles WHERE LOWER(name)=LOWER(?)`, in.Name).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errBad("name_taken")
	}
	id := crypto.NewID()
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `INSERT INTO roles (id,name,description,builtin) VALUES (?,?,?,0)`, id, in.Name, in.Description); err != nil {
			return err
		}
		return setRolePerms(c.ctx(), tx, id, in.Perms)
	})
	if err != nil {
		return err
	}
	a.logAct(c, true, "role.created", "role", id, in.Name)
	return c.JSON(map[string]string{"id": id})
}

func (a *App) hUpdateRole(c *Ctx) error {
	id := c.Param("id")
	var in roleIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := in.check(); err != nil {
		return err
	}
	if id == RoleAdmin { // the admin role always keeps every permission
		in.Perms = AllPerms
	}
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		var builtin bool
		if err := tx.Row(c.ctx(), `SELECT builtin FROM roles WHERE id=?`, id).Scan(&builtin); err != nil {
			return errNotFound
		}
		if !builtin { // built-in roles keep their names (the UI translates them)
			if err := tx.Exec(c.ctx(), `UPDATE roles SET name=?, description=? WHERE id=?`, in.Name, in.Description, id); err != nil {
				return errBad("name_taken")
			}
		}
		if err := setRolePerms(c.ctx(), tx, id, in.Perms); err != nil {
			return err
		}
		return requireAdmins(c.ctx(), tx)
	})
	if err != nil {
		return err
	}
	a.logAct(c, true, "role.updated", "role", id, in.Name+": "+strings.Join(in.Perms, ","))
	return c.OK()
}

func (a *App) hDeleteRole(c *Ctx) error {
	id := c.Param("id")
	var builtin bool
	if err := a.DB.Row(c.ctx(), `SELECT builtin FROM roles WHERE id=?`, id).Scan(&builtin); err != nil {
		return errNotFound
	}
	if builtin {
		return errBad("builtin_role")
	}
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `DELETE FROM acl WHERE subj_type='role' AND subj_id=?`, id); err != nil {
			return err
		}
		if err := tx.Exec(c.ctx(), `DELETE FROM roles WHERE id=?`, id); err != nil {
			return err
		}
		return requireAdmins(c.ctx(), tx)
	})
	if err != nil {
		return err
	}
	a.logAct(c, true, "role.deleted", "role", id, "")
	return c.OK()
}

// ---------- groups ----------

type Group struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scope       string   `json:"scope"` // global | local
	OwnerID     string   `json:"owner_id"`
	Members     []string `json:"members"`
	CanEdit     bool     `json:"can_edit"`
}

// groups returns the global groups plus the viewer's own local groups.
func (a *App) groups(ctx context.Context, u *User) ([]Group, error) {
	out := []Group{}
	idx := map[string]int{}
	err := a.DB.Each(ctx, `SELECT id, name, description, scope, owner_id FROM groups WHERE scope='global' OR owner_id=? ORDER BY scope, LOWER(name)`,
		[]any{u.ID}, func(r *sql.Rows) error {
			g := Group{Members: []string{}}
			if err := r.Scan(&g.ID, &g.Name, &g.Description, &g.Scope, &g.OwnerID); err != nil {
				return err
			}
			g.CanEdit = (g.Scope == "local" && g.OwnerID == u.ID) || (g.Scope == "global" && u.Can(PermRoles))
			idx[g.ID] = len(out)
			out = append(out, g)
			return nil
		})
	if err != nil {
		return nil, err
	}
	err = a.DB.Each(ctx, `SELECT group_id, user_id FROM group_members`, nil, func(r *sql.Rows) error {
		var g, uid string
		if err := r.Scan(&g, &uid); err != nil {
			return err
		}
		if i, ok := idx[g]; ok {
			out[i].Members = append(out[i].Members, uid)
		}
		return nil
	})
	return out, err
}

func (a *App) hGroups(c *Ctx) error {
	out, err := a.groups(c.ctx(), c.U)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

type groupIn struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scope       string   `json:"scope"`
	Members     []string `json:"members"`
}

func (a *App) saveGroupMembers(ctx context.Context, tx *db.Tx, id string, members []string) error {
	if err := tx.Exec(ctx, `DELETE FROM group_members WHERE group_id=?`, id); err != nil {
		return err
	}
	for _, m := range members {
		var n int
		if err := tx.Row(ctx, `SELECT COUNT(*) FROM users WHERE id=? AND deleted_at=0`, m).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return errBad("unknown_user")
		}
		if err := tx.Exec(ctx, `INSERT INTO group_members (group_id,user_id) VALUES (?,?) ON CONFLICT DO NOTHING`, id, m); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) hCreateGroup(c *Ctx) error {
	var in groupIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	if in.Name = clean(in.Name, 80); in.Name == "" {
		return errBad("name_required")
	}
	if in.Scope != "global" && in.Scope != "local" {
		return errBad("invalid_scope")
	}
	if in.Scope == "global" && !c.U.Can(PermRoles) {
		return errForbidden
	}
	if err := limitCount(c.ctx(), a.DB, maxPerOwner, `SELECT COUNT(*) FROM groups WHERE owner_id=?`, c.U.ID); err != nil {
		return err
	}
	id := crypto.NewID()
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `INSERT INTO groups (id,name,description,scope,owner_id,created_at) VALUES (?,?,?,?,?,?)`,
			id, in.Name, clean(in.Description, 300), in.Scope, c.U.ID, a.unix()); err != nil {
			return err
		}
		return a.saveGroupMembers(c.ctx(), tx, id, in.Members)
	})
	if err != nil {
		return err
	}
	a.logAct(c, in.Scope == "global", "group.created", "group", id, in.Name)
	return c.JSON(map[string]string{"id": id})
}

func (a *App) groupEditable(c *Ctx, id string) (scope string, err error) {
	var owner string
	if err := a.DB.Row(c.ctx(), `SELECT scope, owner_id FROM groups WHERE id=?`, id).Scan(&scope, &owner); err != nil {
		return "", errNotFound
	}
	if scope == "local" && owner != c.U.ID {
		return "", errNotFound // local groups are invisible to others
	}
	if scope == "global" && !c.U.Can(PermRoles) {
		return "", errForbidden
	}
	return scope, nil
}

func (a *App) hUpdateGroup(c *Ctx) error {
	id := c.Param("id")
	var in groupIn
	if err := c.Bind(&in); err != nil {
		return err
	}
	scope, err := a.groupEditable(c, id)
	if err != nil {
		return err
	}
	if in.Name = clean(in.Name, 80); in.Name == "" {
		return errBad("name_required")
	}
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `UPDATE groups SET name=?, description=? WHERE id=?`, in.Name, clean(in.Description, 300), id); err != nil {
			return err
		}
		return a.saveGroupMembers(c.ctx(), tx, id, in.Members)
	})
	if err != nil {
		return err
	}
	a.logAct(c, scope == "global", "group.updated", "group", id, in.Name)
	return c.OK()
}

func (a *App) hDeleteGroup(c *Ctx) error {
	id := c.Param("id")
	scope, err := a.groupEditable(c, id)
	if err != nil {
		return err
	}
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `DELETE FROM acl WHERE subj_type='group' AND subj_id=?`, id); err != nil {
			return err
		}
		return tx.Exec(c.ctx(), `DELETE FROM groups WHERE id=?`, id)
	})
	if err != nil {
		return err
	}
	a.logAct(c, scope == "global", "group.deleted", "group", id, "")
	return c.OK()
}

// ---------- administration of accounts ----------

func (a *App) hAdminUsers(c *Ctx) error {
	out := []*User{}
	var ids []string
	err := a.DB.Each(c.ctx(), `SELECT id FROM users WHERE deleted_at=0 ORDER BY LOWER(username)`, nil, func(r *sql.Rows) error {
		var id string
		if err := r.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
		return nil
	})
	if err != nil {
		return err
	}
	for _, id := range ids {
		u, err := a.userByID(c.ctx(), id)
		if err != nil {
			return err
		}
		u.Settings = nil
		out = append(out, u)
	}
	return c.JSON(out)
}

func (a *App) hAdminCreateUser(c *Ctx) error {
	var in struct {
		Username    string   `json:"username"`
		DisplayName string   `json:"display_name"`
		Email       string   `json:"email"`
		Password    string   `json:"password"`
		Roles       []string `json:"roles"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if in.Roles == nil {
		in.Roles = []string{RoleMember}
	}
	if !c.U.Can(PermRoles) { // without roles.manage only plain members can be created
		in.Roles = []string{RoleMember}
	}
	id, err := a.CreateUser(c.ctx(), in.Username, in.DisplayName, in.Email, in.Password, in.Roles, true)
	if err != nil {
		return err
	}
	a.logAct(c, true, "user.created", "user", id, in.Username)
	return c.JSON(map[string]string{"id": id})
}

// adminCount returns the number of active accounts holding a permission.
func adminCount(ctx context.Context, q db.Q, perm string) (int, error) {
	var n int
	err := q.Row(ctx, `SELECT COUNT(DISTINCT u.id) FROM users u JOIN user_roles ur ON ur.user_id=u.id JOIN role_perms rp ON rp.role_id=ur.role_id
		WHERE rp.perm=? AND u.deleted_at=0 AND u.disabled=0`, perm).Scan(&n)
	return n, err
}

// requireAdmins fails when a change would leave the organisation without
// anybody who can manage accounts or roles. It is checked inside the
// transaction of every change that can remove those permissions.
func requireAdmins(ctx context.Context, q db.Q) error {
	for _, p := range []string{PermUsers, PermRoles} {
		if n, err := adminCount(ctx, q, p); err != nil {
			return err
		} else if n == 0 {
			return errBad("last_admin")
		}
	}
	return nil
}

func (a *App) hAdminUpdateUser(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		Username    *string   `json:"username"`
		DisplayName *string   `json:"display_name"`
		Email       *string   `json:"email"`
		Disabled    *bool     `json:"disabled"`
		MFARequired *bool     `json:"mfa_required"`
		ResetMFA    bool      `json:"reset_mfa"`
		Unlock      bool      `json:"unlock"`
		Roles       *[]string `json:"roles"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	t, err := a.userByID(c.ctx(), id)
	if err != nil || t.deletedAt != 0 {
		return errNotFound
	}
	if in.Roles != nil && !c.U.Can(PermRoles) {
		return errForbidden
	}
	// Somebody who only manages accounts must not be able to take over a more
	// privileged account (e.g. by resetting an administrator's MFA).
	if !c.U.Can(PermRoles) && t.ID != c.U.ID && len(t.Perms) > 0 {
		return errForbidden
	}
	var changes []string
	kick := false
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		if in.Username != nil {
			name := strings.ToLower(strings.TrimSpace(*in.Username))
			if name != t.Username {
				if !usernameRe.MatchString(name) {
					return errBad("invalid_username")
				}
				var n int
				if err := tx.Row(ctx, `SELECT COUNT(*) FROM users WHERE username=?`, name).Scan(&n); err != nil {
					return err
				}
				if n > 0 {
					return errBad("username_taken")
				}
				if err := tx.Exec(ctx, `UPDATE users SET username=? WHERE id=?`, name, id); err != nil {
					return err
				}
				changes = append(changes, "renamed "+t.Username+" -> "+name)
			}
		}
		if in.DisplayName != nil && cleanName(*in.DisplayName) != "" && cleanName(*in.DisplayName) != t.DisplayName {
			name := cleanName(*in.DisplayName)
			if taken, err := nameTaken(ctx, tx, name, id); err != nil {
				return err
			} else if taken {
				return errBad("display_name_taken")
			}
			if err := tx.Exec(ctx, `UPDATE users SET display_name=?, display_key=? WHERE id=?`, name, nameKey(name), id); err != nil {
				return err
			}
		}
		if in.Email != nil {
			e, err := cleanEmail(*in.Email)
			if err != nil {
				return err
			}
			if err := tx.Exec(ctx, `UPDATE users SET email_enc=?, email_idx=? WHERE id=?`, a.Keys.EncryptField(e, "users.email", id), a.Keys.BlindIndex(e), id); err != nil {
				return err
			}
		}
		if in.Disabled != nil {
			if *in.Disabled && id == c.U.ID {
				return errBad("cannot_disable_self")
			}
			if err := tx.Exec(ctx, `UPDATE users SET disabled=? WHERE id=?`, b2i(*in.Disabled), id); err != nil {
				return err
			}
			if *in.Disabled {
				if err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=?`, id); err != nil {
					return err
				}
				kick = true
			}
			changes = append(changes, "disabled="+map[bool]string{true: "yes", false: "no"}[*in.Disabled])
		}
		if in.MFARequired != nil {
			if err := tx.Exec(ctx, `UPDATE users SET mfa_required=? WHERE id=?`, b2i(*in.MFARequired), id); err != nil {
				return err
			}
			changes = append(changes, "mfa_required="+map[bool]string{true: "yes", false: "no"}[*in.MFARequired])
		}
		if in.ResetMFA {
			// Removes every second factor: authenticator app, recovery codes
			// and passkeys.
			if err := resetTOTP(ctx, tx, id); err != nil {
				return err
			}
			if err := tx.Exec(ctx, `DELETE FROM passkeys WHERE user_id=?`, id); err != nil {
				return err
			}
			changes = append(changes, "mfa reset")
		}
		if in.Unlock {
			if err := tx.Exec(ctx, `UPDATE users SET failed_logins=0, locked_until=0 WHERE id=?`, id); err != nil {
				return err
			}
			changes = append(changes, "unlocked")
		}
		if in.Roles != nil {
			if err := setUserRoles(ctx, tx, id, *in.Roles); err != nil {
				return err
			}
			changes = append(changes, "roles="+strings.Join(*in.Roles, ","))
		}
		// Never allow the organisation to end up without an administrator.
		return requireAdmins(ctx, tx)
	})
	if err != nil {
		return err
	}
	if kick {
		a.hub.kickUser(id, "")
	}
	a.logAct(c, true, "user.updated", "user", id, t.Username+": "+strings.Join(changes, "; "))
	return c.OK()
}

func (a *App) hAdminSetPassword(c *Ctx) error {
	id := c.Param("id")
	var in struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	t, err := a.userByID(c.ctx(), id)
	if err != nil || t.deletedAt != 0 {
		return errNotFound
	}
	if !c.U.Can(PermRoles) && t.ID != c.U.ID && len(t.Perms) > 0 {
		return errForbidden
	}
	if err := a.SetPassword(c.ctx(), id, in.Password, true); err != nil {
		return err
	}
	a.logAct(c, true, "user.password_reset", "user", id, t.Username)
	return c.OK()
}

// SetPassword sets a password and signs the user out everywhere.
func (a *App) SetPassword(ctx context.Context, id, password string, mustChange bool) error {
	if err := a.checkPassword(password); err != nil {
		return err
	}
	if err := a.DB.Exec(ctx, `UPDATE users SET password_hash=?, must_change_password=?, failed_logins=0, locked_until=0 WHERE id=?`,
		crypto.HashPassword(password), b2i(mustChange), id); err != nil {
		return err
	}
	return a.SignOutUser(ctx, id, "")
}

// hAdminSignOut ends every session of an account ("sign out everywhere"),
// for example when a device was lost.
func (a *App) hAdminSignOut(c *Ctx) error {
	id := c.Param("id")
	t, err := a.userByID(c.ctx(), id)
	if err != nil || t.deletedAt != 0 {
		return errNotFound
	}
	if !c.U.Can(PermRoles) && t.ID != c.U.ID && len(t.Perms) > 0 {
		return errForbidden
	}
	keep := ""
	if id == c.U.ID { // signing yourself out here keeps the session you are using
		keep = c.sess.idHash
	}
	if err := a.SignOutUser(c.ctx(), id, keep); err != nil {
		return err
	}
	a.logAct(c, true, "sessions.ended", "user", id, t.Username+": by an administrator")
	return c.OK()
}

// hAdminDeleteUser anonymises the account: personal data and credentials are
// erased, private content is removed and shared contributions remain under
// a "deleted user" label (GDPR erasure while keeping the club's records).
func (a *App) hAdminDeleteUser(c *Ctx) error {
	id := c.Param("id")
	if id == c.U.ID {
		return errBad("cannot_delete_self")
	}
	t, err := a.userByID(c.ctx(), id)
	if err != nil || t.deletedAt != 0 {
		return errNotFound
	}
	if !c.U.Can(PermRoles) && len(t.Perms) > 0 {
		return errForbidden
	}
	err = a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		ctx := c.ctx()
		stmts := []string{
			`DELETE FROM sessions WHERE user_id=?`,
			`DELETE FROM user_roles WHERE user_id=?`,
			`DELETE FROM group_members WHERE user_id=?`,
			`DELETE FROM acl WHERE subj_type='group' AND subj_id IN (SELECT id FROM groups WHERE scope='local' AND owner_id=?)`,
			`DELETE FROM groups WHERE scope='local' AND owner_id=?`,
			`DELETE FROM acl WHERE subj_type='user' AND subj_id=?`,
			`DELETE FROM channel_reads WHERE user_id=?`,
			`DELETE FROM passkeys WHERE user_id=?`,
			`DELETE FROM recovery_codes WHERE user_id=?`,
			`DELETE FROM notifications WHERE user_id=?`,
			`DELETE FROM notifications WHERE actor_id=?`,
			`DELETE FROM dm_blocks WHERE user_id=?`,
			`DELETE FROM dm_blocks WHERE blocked_id=?`,
			`DELETE FROM reactions WHERE user_id=?`,
			`UPDATE activity SET ip='', summary='' WHERE user_id=?`,
		}
		// Private (unshared) resources would be unreachable: remove them.
		for typ, table := range resTables {
			// Channels have no private form. Decisions are the organisation's
			// record and are never deleted, shared or not (their numbering
			// also depends on that).
			if typ == "channel" || typ == "decision" {
				continue
			}
			stmts = append(stmts, `DELETE FROM `+table+` WHERE owner_id=? AND NOT EXISTS (SELECT 1 FROM acl a WHERE a.res_type='`+typ+`' AND a.res_id=`+table+`.id)`)
		}
		for _, s := range stmts {
			if err := tx.Exec(ctx, s, id); err != nil {
				return err
			}
		}
		if err := tx.Exec(ctx, `UPDATE users SET username=?, display_name='', email_enc='', email_idx='', phone_enc='', password_hash='',
			totp_secret_enc='', totp_enabled=0, settings='{}', display_key='', profile_enc='', profile_shared='', avatar='', disabled=1, deleted_at=? WHERE id=?`, "deleted-"+id[len(id)-16:], a.unix(), id); err != nil {
			return err
		}
		return requireAdmins(ctx, tx)
	})
	if err != nil {
		return err
	}
	a.hub.kickUser(id, "")
	a.gcFiles(c.ctx())
	a.logAct(c, true, "user.deleted", "user", id, t.Username)
	return c.OK()
}

func (a *App) hAdminLogs(c *Ctx) error {
	type entry struct {
		ID        string `json:"id"`
		UserID    string `json:"user_id"`
		Username  string `json:"username"`
		Action    string `json:"action"`
		ResType   string `json:"res_type"`
		ResID     string `json:"res_id"`
		Summary   string `json:"summary"`
		IP        string `json:"ip"`
		CreatedAt int64  `json:"created_at"`
	}
	q := `SELECT a.id, a.user_id, COALESCE(u.username,''), a.action, a.res_type, a.res_id, a.summary, a.ip, a.created_at
		FROM activity a LEFT JOIN users u ON u.id=a.user_id WHERE a.audit=1 AND (?='' OR a.id < ?)`
	before := c.Query("before")
	args := []any{before, before}
	if act := c.Query("action"); act != "" {
		q += ` AND a.action LIKE ?`
		args = append(args, strings.ReplaceAll(act, "%", "")+"%")
	}
	if uid := c.Query("user"); uid != "" {
		q += ` AND a.user_id=?`
		args = append(args, uid)
	}
	q += ` ORDER BY a.id DESC LIMIT 200`
	out := []entry{}
	err := a.DB.Each(c.ctx(), q, args, func(r *sql.Rows) error {
		var e entry
		if err := r.Scan(&e.ID, &e.UserID, &e.Username, &e.Action, &e.ResType, &e.ResID, &e.Summary, &e.IP, &e.CreatedAt); err != nil {
			return err
		}
		out = append(out, e)
		return nil
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}
