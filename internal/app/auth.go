package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"rsc.io/qr"

	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

// Permissions. Everything a signed-in member may do with their own content
// needs no permission; these gate organisation wide capabilities.
const (
	PermUsers     = "users.manage"     // create/rename/delete accounts, passwords, MFA
	PermRoles     = "roles.manage"     // roles, role assignment and global groups
	PermLogs      = "logs.view"        // audit log
	PermBackup    = "backup.manage"    // backup and bulk export
	PermChannels  = "channels.create"  // create channels
	PermModerate  = "content.moderate" // manage any shared content
	PermDecisions = "decisions.create" // record decisions in the decision log
	PermMeetings  = "meetings.create"  // plan meetings and write protocols
)

var AllPerms = []string{PermUsers, PermRoles, PermLogs, PermBackup, PermChannels, PermModerate, PermDecisions, PermMeetings}

const (
	RoleAdmin  = "admin"
	RoleLeader = "leader"
	RoleMember = "member"
)

type User struct {
	ID             string         `json:"id"`
	Username       string         `json:"username"`
	DisplayName    string         `json:"display_name"`
	Email          string         `json:"email"`
	Phone          string         `json:"phone"`
	ContactVisible bool           `json:"contact_visible"`
	Locale         string         `json:"locale"`
	Settings       map[string]any `json:"settings"`
	TOTPEnabled    bool           `json:"totp_enabled"`
	MFARequired    bool           `json:"mfa_required"`
	MustChangePW   bool           `json:"must_change_password"`
	Disabled       bool           `json:"disabled"`
	CreatedAt      int64          `json:"created_at"`
	LastLoginAt    int64          `json:"last_login_at"`
	Roles          []string       `json:"roles"`
	Perms          []string       `json:"perms"`

	passwordHash string
	totpSecret   string
	totpLastStep int64
	failed       int
	lockedUntil  int64
	deletedAt    int64
}

func (u *User) Can(perm string) bool {
	for _, p := range u.Perms {
		if p == perm {
			return true
		}
	}
	return false
}

// pending returns a non-empty code while the account must complete a step
// (password change, MFA enrolment) before it may use the rest of the API.
func (u *User) pending() string {
	if u.MustChangePW {
		return "password_change_required"
	}
	if u.MFARequired && !u.TOTPEnabled {
		return "mfa_setup_required"
	}
	return ""
}

type session struct {
	idHash string
	csrf   string
	kind   string
}

const userCols = `id, username, display_name, email_enc, phone_enc, contact_visible, locale, settings, totp_enabled,
 mfa_required, must_change_password, disabled, created_at, last_login_at, password_hash, totp_secret_enc,
 totp_last_step, failed_logins, locked_until, deleted_at`

type scanner interface{ Scan(dest ...any) error }

func (a *App) scanUser(s scanner) (*User, error) {
	u := &User{}
	var emailEnc, phoneEnc, settings, totpEnc string
	err := s.Scan(&u.ID, &u.Username, &u.DisplayName, &emailEnc, &phoneEnc, &u.ContactVisible, &u.Locale, &settings,
		&u.TOTPEnabled, &u.MFARequired, &u.MustChangePW, &u.Disabled, &u.CreatedAt, &u.LastLoginAt, &u.passwordHash,
		&totpEnc, &u.totpLastStep, &u.failed, &u.lockedUntil, &u.deletedAt)
	if err != nil {
		return nil, err
	}
	u.Email, _ = a.Keys.DecryptField(emailEnc, "users.email")
	u.Phone, _ = a.Keys.DecryptField(phoneEnc, "users.phone")
	u.totpSecret, _ = a.Keys.DecryptField(totpEnc, "users.totp")
	u.Settings = map[string]any{}
	_ = json.Unmarshal([]byte(settings), &u.Settings)
	u.Roles, u.Perms = []string{}, []string{}
	return u, nil
}

func (a *App) loadRoles(ctx context.Context, u *User) error {
	err := a.DB.Each(ctx, `SELECT role_id FROM user_roles WHERE user_id=? ORDER BY role_id`, []any{u.ID}, func(r *sql.Rows) error {
		var id string
		if err := r.Scan(&id); err != nil {
			return err
		}
		u.Roles = append(u.Roles, id)
		return nil
	})
	if err != nil {
		return err
	}
	return a.DB.Each(ctx, `SELECT DISTINCT perm FROM role_perms WHERE role_id IN (SELECT role_id FROM user_roles WHERE user_id=?) ORDER BY perm`,
		[]any{u.ID}, func(r *sql.Rows) error {
			var p string
			if err := r.Scan(&p); err != nil {
				return err
			}
			u.Perms = append(u.Perms, p)
			return nil
		})
}

func (a *App) userByID(ctx context.Context, id string) (*User, error) {
	u, err := a.scanUser(a.DB.Row(ctx, `SELECT `+userCols+` FROM users WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, a.loadRoles(ctx, u)
}

// seed creates the built-in roles and the default channel.
func (a *App) seed(ctx context.Context) error {
	var n int
	if err := a.DB.Row(ctx, `SELECT COUNT(*) FROM roles`).Scan(&n); err != nil || n > 0 {
		return err
	}
	return a.DB.InTx(ctx, func(tx *db.Tx) error {
		roles := []struct {
			id, name, desc string
			perms          []string
		}{
			{RoleAdmin, "Administrator", "Full access to administration", AllPerms},
			{RoleLeader, "Leader", "Board member / leader", []string{PermChannels, PermModerate, PermDecisions, PermMeetings}},
			{RoleMember, "Member", "Regular member", nil},
		}
		for _, r := range roles {
			if err := tx.Exec(ctx, `INSERT INTO roles (id,name,description,builtin) VALUES (?,?,?,1)`, r.id, r.name, r.desc); err != nil {
				return err
			}
			for _, p := range r.perms {
				if err := tx.Exec(ctx, `INSERT INTO role_perms (role_id,perm) VALUES (?,?)`, r.id, p); err != nil {
					return err
				}
			}
		}
		id, now := crypto.NewID(), a.unix()
		if err := tx.Exec(ctx, `INSERT INTO channels (id,owner_id,kind,name,topic,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
			id, "", "channel", "general", "", now, now); err != nil {
			return err
		}
		return tx.Exec(ctx, `INSERT INTO acl (res_type,res_id,subj_type,subj_id,level) VALUES ('channel',?,'all','',2)`, id)
	})
}

var usernameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,31}$`)

func (a *App) checkPassword(pw string) error {
	if len([]rune(pw)) < a.Cfg.Security.MinPasswordLength {
		return errBad("password_too_short")
	}
	if len(pw) > 256 {
		return errBad("password_too_long")
	}
	return nil
}

// CreateUser inserts a new account. Used by the admin API and the CLI.
func (a *App) CreateUser(ctx context.Context, username, displayName, email, password string, roles []string, mustChange bool) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernameRe.MatchString(username) {
		return "", errBad("invalid_username")
	}
	if err := a.checkPassword(password); err != nil {
		return "", err
	}
	if displayName = clean(displayName, 100); displayName == "" {
		displayName = username
	}
	email = clean(email, 200)
	var n int
	if err := a.DB.Row(ctx, `SELECT COUNT(*) FROM users WHERE username=?`, username).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		return "", errBad("username_taken")
	}
	id := crypto.NewID()
	err := a.DB.InTx(ctx, func(tx *db.Tx) error {
		if err := tx.Exec(ctx, `INSERT INTO users (id,username,display_name,email_enc,email_idx,password_hash,must_change_password,created_at)
			VALUES (?,?,?,?,?,?,?,?)`, id, username, displayName, a.Keys.EncryptField(email, "users.email"), a.Keys.BlindIndex(email),
			crypto.HashPassword(password), b2i(mustChange), a.unix()); err != nil {
			return err
		}
		return setUserRoles(ctx, tx, id, roles)
	})
	return id, err
}

func setUserRoles(ctx context.Context, tx *db.Tx, uid string, roles []string) error {
	if err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id=?`, uid); err != nil {
		return err
	}
	for _, r := range roles {
		var n int
		if err := tx.Row(ctx, `SELECT COUNT(*) FROM roles WHERE id=?`, r).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return errBad("unknown_role")
		}
		if err := tx.Exec(ctx, `INSERT INTO user_roles (user_id,role_id) VALUES (?,?) ON CONFLICT DO NOTHING`, uid, r); err != nil {
			return err
		}
	}
	return nil
}

// ---------- sessions ----------

const cookieName = "samling_session"

func (a *App) cookie(r *http.Request) string {
	if a.secureCookies(r) {
		return "__Host-" + cookieName // __Host- pins the cookie to this origin
	}
	return cookieName
}

// authenticate resolves the session from the cookie or a Bearer token (used
// by native apps). Cookie sessions must present the CSRF token on writes.
func (a *App) authenticate(c *Ctx) error {
	tok, bearer := "", false
	if h := c.R.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		tok, bearer = strings.TrimPrefix(h, "Bearer "), true
	} else if ck, err := c.R.Cookie(a.cookie(c.R)); err == nil {
		tok = ck.Value
	}
	if tok == "" {
		return errUnauth
	}
	s := &session{idHash: crypto.HashToken(tok)}
	var uid string
	var expires, lastSeen int64
	err := a.DB.Row(c.ctx(), `SELECT user_id, csrf, kind, expires_at, last_seen FROM sessions WHERE id_hash=?`, s.idHash).
		Scan(&uid, &s.csrf, &s.kind, &expires, &lastSeen)
	now := a.unix()
	if errors.Is(err, sql.ErrNoRows) || (err == nil && expires < now) {
		return errUnauth
	}
	if err != nil {
		return err
	}
	// A cookie-kind session must not be usable as a bearer token and vice
	// versa; this keeps the CSRF rule below meaningful.
	if (s.kind == "token") != bearer {
		return errUnauth
	}
	if !bearer && c.R.Method != http.MethodGet && c.R.Method != http.MethodHead {
		if !constEq(c.R.Header.Get("X-CSRF-Token"), s.csrf) {
			return &apiError{http.StatusForbidden, "csrf"}
		}
	}
	u, err := a.userByID(c.ctx(), uid)
	if err != nil || u.Disabled || u.deletedAt != 0 {
		return errUnauth
	}
	if now-lastSeen > 300 {
		_ = a.DB.Exec(c.ctx(), `UPDATE sessions SET last_seen=? WHERE id_hash=?`, now, s.idHash)
	}
	c.U, c.sess = u, s
	return nil
}

func constEq(a, b string) bool {
	return len(a) == len(b) && len(a) > 0 && subtleEq([]byte(a), []byte(b))
}

func (a *App) newSession(c *Ctx, u *User, kind string) (token, csrf string, err error) {
	token, csrf = crypto.Token(32), crypto.Token(24)
	now := a.unix()
	exp := now + int64(a.Cfg.Security.SessionHours)*3600
	err = a.DB.Exec(c.ctx(), `INSERT INTO sessions (id_hash,user_id,csrf,kind,ip,user_agent,created_at,last_seen,expires_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		crypto.HashToken(token), u.ID, csrf, kind, c.IP(), clean(c.R.UserAgent(), 200), now, now, exp)
	if err != nil {
		return "", "", err
	}
	if kind == "cookie" {
		http.SetCookie(c.W, &http.Cookie{Name: a.cookie(c.R), Value: token, Path: "/", HttpOnly: true,
			Secure: a.secureCookies(c.R), SameSite: http.SameSiteStrictMode, Expires: time.Unix(exp, 0)})
	}
	return token, csrf, nil
}

// dummyHash makes login timing independent of whether the account exists.
var dummyHash = crypto.HashPassword("samling-dummy-password")

func (a *App) hLogin(c *Ctx) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`
		Token    bool   `json:"token"` // native apps: return a bearer token instead of a cookie
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if !a.login.allow(c.IP()) {
		return &apiError{http.StatusTooManyRequests, "rate_limited"}
	}
	if in.Token && c.R.Header.Get("Origin") != "" && !a.sameOrigin(c.R) {
		// A web page on another origin has no business obtaining tokens.
		return &apiError{http.StatusForbidden, "cross_origin"}
	}
	username := strings.ToLower(strings.TrimSpace(in.Username))
	fail := func(uid, reason string) error {
		a.logRaw(c.ctx(), uid, c.IP(), true, "login.failed", "user", uid, reason+": "+clean(username, 40))
		return &apiError{http.StatusUnauthorized, "invalid_credentials"}
	}
	u, err := a.scanUser(a.DB.Row(c.ctx(), `SELECT `+userCols+` FROM users WHERE username=? AND deleted_at=0`, username))
	if errors.Is(err, sql.ErrNoRows) {
		crypto.VerifyPassword(in.Password, dummyHash)
		return fail("", "unknown user")
	}
	if err != nil {
		return err
	}
	now := a.unix()
	if u.lockedUntil > now {
		crypto.VerifyPassword(in.Password, dummyHash)
		return fail(u.ID, "locked")
	}
	bad := func(reason string) error {
		// Lock the account for 15 minutes after 10 consecutive failures.
		u.failed++
		lock := int64(0)
		if u.failed >= 10 {
			lock, u.failed = now+900, 0
		}
		_ = a.DB.Exec(c.ctx(), `UPDATE users SET failed_logins=?, locked_until=? WHERE id=?`, u.failed, lock, u.ID)
		return fail(u.ID, reason)
	}
	if !crypto.VerifyPassword(in.Password, u.passwordHash) || u.Disabled {
		return bad("bad password")
	}
	if u.TOTPEnabled {
		if strings.TrimSpace(in.Code) == "" {
			return &apiError{http.StatusUnauthorized, "mfa_required"}
		}
		step, ok := crypto.VerifyTOTP(u.totpSecret, in.Code, a.now())
		if !ok || step <= u.totpLastStep { // wrong or replayed code
			return bad("bad mfa code")
		}
		u.totpLastStep = step
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE users SET failed_logins=0, locked_until=0, last_login_at=?, totp_last_step=? WHERE id=?`,
		now, u.totpLastStep, u.ID); err != nil {
		return err
	}
	kind := "cookie"
	if in.Token {
		kind = "token"
	}
	token, csrf, err := a.newSession(c, u, kind)
	if err != nil {
		return err
	}
	c.U = u
	a.logAct(c, true, "login", "user", u.ID, kind)
	out := map[string]any{"csrf": csrf}
	if in.Token {
		out = map[string]any{"token": token}
	}
	return c.JSON(out)
}

func (a *App) hLogout(c *Ctx) error {
	if err := a.DB.Exec(c.ctx(), `DELETE FROM sessions WHERE id_hash=?`, c.sess.idHash); err != nil {
		return err
	}
	http.SetCookie(c.W, &http.Cookie{Name: a.cookie(c.R), Value: "", Path: "/", HttpOnly: true,
		Secure: a.secureCookies(c.R), SameSite: http.SameSiteStrictMode, MaxAge: -1})
	a.logAct(c, true, "logout", "user", c.U.ID, "")
	return c.OK()
}

func (a *App) hMe(c *Ctx) error {
	return c.JSON(map[string]any{"user": c.U, "csrf": c.sess.csrf, "pending": c.U.pending(),
		"org_name": a.Cfg.OrgName, "max_upload_mb": a.Cfg.Storage.MaxUploadMB, "min_password": a.Cfg.Security.MinPasswordLength})
}

func (a *App) hUpdateMe(c *Ctx) error {
	var in struct {
		DisplayName    *string         `json:"display_name"`
		Email          *string         `json:"email"`
		Phone          *string         `json:"phone"`
		ContactVisible *bool           `json:"contact_visible"`
		Locale         *string         `json:"locale"`
		Settings       *map[string]any `json:"settings"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	u := c.U
	if in.DisplayName != nil {
		if u.DisplayName = clean(*in.DisplayName, 100); u.DisplayName == "" {
			return errBad("name_required")
		}
	}
	if in.Email != nil {
		u.Email = clean(*in.Email, 200)
		if u.Email != "" && !strings.Contains(u.Email, "@") {
			return errBad("invalid_email")
		}
	}
	if in.Phone != nil {
		u.Phone = clean(*in.Phone, 50)
	}
	if in.ContactVisible != nil {
		u.ContactVisible = *in.ContactVisible
	}
	if in.Locale != nil {
		if *in.Locale != "sv" && *in.Locale != "en" {
			return errBad("invalid_locale")
		}
		u.Locale = *in.Locale
	}
	if in.Settings != nil {
		u.Settings = *in.Settings
	}
	settings, _ := json.Marshal(u.Settings)
	if len(settings) > 4096 {
		return errBad("too_large")
	}
	err := a.DB.Exec(c.ctx(), `UPDATE users SET display_name=?, email_enc=?, email_idx=?, phone_enc=?, contact_visible=?, locale=?, settings=? WHERE id=?`,
		u.DisplayName, a.Keys.EncryptField(u.Email, "users.email"), a.Keys.BlindIndex(u.Email), a.Keys.EncryptField(u.Phone, "users.phone"),
		b2i(u.ContactVisible), u.Locale, string(settings), u.ID)
	if err != nil {
		return err
	}
	return c.JSON(u)
}

func (a *App) hChangePassword(c *Ctx) error {
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if !a.login.allow(c.IP()) {
		return &apiError{http.StatusTooManyRequests, "rate_limited"}
	}
	if !crypto.VerifyPassword(in.Current, c.U.passwordHash) {
		return errBad("wrong_password")
	}
	if err := a.checkPassword(in.New); err != nil {
		return err
	}
	if in.New == in.Current {
		return errBad("password_unchanged")
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE users SET password_hash=?, must_change_password=0 WHERE id=?`, crypto.HashPassword(in.New), c.U.ID); err != nil {
		return err
	}
	// Sign out every other session of this account.
	if err := a.DB.Exec(c.ctx(), `DELETE FROM sessions WHERE user_id=? AND id_hash<>?`, c.U.ID, c.sess.idHash); err != nil {
		return err
	}
	a.logAct(c, true, "password.changed", "user", c.U.ID, "")
	return c.OK()
}

// hMFASetup generates a new (not yet active) TOTP secret.
func (a *App) hMFASetup(c *Ctx) error {
	if c.U.TOTPEnabled {
		return errBad("mfa_already_enabled")
	}
	secret := crypto.NewTOTPSecret()
	if err := a.DB.Exec(c.ctx(), `UPDATE users SET totp_secret_enc=? WHERE id=?`, a.Keys.EncryptField(secret, "users.totp"), c.U.ID); err != nil {
		return err
	}
	uri := crypto.TOTPURI(a.Cfg.OrgName, c.U.Username, secret)
	out := map[string]string{"secret": secret, "uri": uri}
	if code, err := qr.Encode(uri, qr.M); err == nil {
		code.Scale = 5
		out["qr"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG())
	}
	return c.JSON(out)
}

func (a *App) hMFAEnable(c *Ctx) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if c.U.TOTPEnabled || c.U.totpSecret == "" {
		return errBad("mfa_not_pending")
	}
	step, ok := crypto.VerifyTOTP(c.U.totpSecret, in.Code, a.now())
	if !ok {
		return errBad("invalid_code")
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE users SET totp_enabled=1, totp_last_step=? WHERE id=?`, step, c.U.ID); err != nil {
		return err
	}
	a.logAct(c, true, "mfa.enabled", "user", c.U.ID, "")
	return c.OK()
}

func (a *App) hMFADisable(c *Ctx) error {
	var in struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if c.U.MFARequired {
		return errBad("mfa_is_required")
	}
	if !a.login.allow(c.IP()) {
		return &apiError{http.StatusTooManyRequests, "rate_limited"}
	}
	if !crypto.VerifyPassword(in.Password, c.U.passwordHash) {
		return errBad("wrong_password")
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE users SET totp_enabled=0, totp_secret_enc='' WHERE id=?`, c.U.ID); err != nil {
		return err
	}
	a.logAct(c, true, "mfa.disabled", "user", c.U.ID, "")
	return c.OK()
}
