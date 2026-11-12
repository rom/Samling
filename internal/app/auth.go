package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
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

// Brute force protection: after maxFailures consecutive failed password or
// code checks the account is locked for lockDuration.
const (
	maxFailures  = 10
	lockDuration = 15 * 60
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
	Passkeys       int            `json:"passkeys"`       // number of registered passkeys
	RecoveryCodes  int            `json:"recovery_codes"` // unused recovery codes
	Sessions       int            `json:"sessions"`       // active sessions
	// Optional profile fields (address, interests, specialities, about) and,
	// per field, whether other members may see it.
	Profile       map[string]string `json:"profile"`
	ProfileShared map[string]bool   `json:"profile_shared"`
	Avatar        string            `json:"avatar"` // "", "preset:<n>" or "file:<id>"

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
// (password change, second factor enrolment) before it may use the rest of
// the API. A passkey counts as a second factor.
func (u *User) pending() string {
	if u.MustChangePW {
		return "password_change_required"
	}
	if u.MFARequired && !u.TOTPEnabled && u.Passkeys == 0 {
		return "mfa_setup_required"
	}
	return ""
}

type session struct {
	idHash string
	csrf   string
	kind   string
	// passkey is true when the session was established with a passkey.
	passkey bool
}

const userCols = `id, username, display_name, email_enc, phone_enc, contact_visible, locale, settings, totp_enabled,
 mfa_required, must_change_password, disabled, created_at, last_login_at, password_hash, totp_secret_enc,
 totp_last_step, failed_logins, locked_until, deleted_at, profile_enc, profile_shared, avatar`

type scanner interface{ Scan(dest ...any) error }

func (a *App) scanUser(s scanner) (*User, error) {
	u := &User{}
	var emailEnc, phoneEnc, settings, totpEnc, profileEnc, profileShared string
	err := s.Scan(&u.ID, &u.Username, &u.DisplayName, &emailEnc, &phoneEnc, &u.ContactVisible, &u.Locale, &settings,
		&u.TOTPEnabled, &u.MFARequired, &u.MustChangePW, &u.Disabled, &u.CreatedAt, &u.LastLoginAt, &u.passwordHash,
		&totpEnc, &u.totpLastStep, &u.failed, &u.lockedUntil, &u.deletedAt, &profileEnc, &profileShared, &u.Avatar)
	if err != nil {
		return nil, err
	}
	u.Profile, u.ProfileShared = a.decodeProfile(u.ID, profileEnc, profileShared)
	u.Email, _ = a.Keys.DecryptField(emailEnc, "users.email", u.ID)
	u.Phone, _ = a.Keys.DecryptField(phoneEnc, "users.phone", u.ID)
	u.totpSecret, _ = a.Keys.DecryptField(totpEnc, "users.totp", u.ID)
	u.Settings = map[string]any{}
	_ = json.Unmarshal([]byte(settings), &u.Settings)
	u.Roles, u.Perms = []string{}, []string{}
	return u, nil
}

// loadExtras fills in roles, permissions and the counters of the account.
func (a *App) loadExtras(ctx context.Context, u *User) error {
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
	err = a.DB.Each(ctx, `SELECT DISTINCT perm FROM role_perms WHERE role_id IN (SELECT role_id FROM user_roles WHERE user_id=?) ORDER BY perm`,
		[]any{u.ID}, func(r *sql.Rows) error {
			var p string
			if err := r.Scan(&p); err != nil {
				return err
			}
			u.Perms = append(u.Perms, p)
			return nil
		})
	if err != nil {
		return err
	}
	return a.DB.Row(ctx, `SELECT (SELECT COUNT(*) FROM passkeys WHERE user_id=?),
		(SELECT COUNT(*) FROM recovery_codes WHERE user_id=? AND used_at=0),
		(SELECT COUNT(*) FROM sessions WHERE user_id=? AND expires_at>?)`, u.ID, u.ID, u.ID, a.unix()).Scan(&u.Passkeys, &u.RecoveryCodes, &u.Sessions)
}

func (a *App) userByID(ctx context.Context, id string) (*User, error) {
	u, err := a.scanUser(a.DB.Row(ctx, `SELECT `+userCols+` FROM users WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, a.loadExtras(ctx, u)
}

// seed creates the built-in roles and the default channel, and upgrades
// stored data of earlier releases.
func (a *App) seed(ctx context.Context) error {
	if err := a.upgradeData(ctx); err != nil {
		return err
	}
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

// upgradeData rewrites values stored by earlier releases: encrypted fields
// that were bound to their column only are re-encrypted bound to their row,
// and the display name key used for the uniqueness check is filled in.
func (a *App) upgradeData(ctx context.Context) error {
	type row struct{ id, name, key, email, phone, totp string }
	var rows []row
	err := a.DB.Each(ctx, `SELECT id, display_name, display_key, email_enc, phone_enc, totp_secret_enc FROM users
		WHERE email_enc LIKE 'v1:%' OR phone_enc LIKE 'v1:%' OR totp_secret_enc LIKE 'v1:%' OR (display_key='' AND deleted_at=0)`, nil,
		func(r *sql.Rows) error {
			var x row
			if err := r.Scan(&x.id, &x.name, &x.key, &x.email, &x.phone, &x.totp); err != nil {
				return err
			}
			rows = append(rows, x)
			return nil
		})
	if err != nil {
		return err
	}
	for _, x := range rows {
		up := func(enc, column string) (string, error) {
			if !crypto.IsLegacyField(enc) {
				return enc, nil
			}
			plain, err := a.Keys.DecryptLegacyField(enc, column)
			if err != nil {
				return "", err
			}
			return a.Keys.EncryptField(plain, column, x.id), nil
		}
		email, err1 := up(x.email, "users.email")
		phone, err2 := up(x.phone, "users.phone")
		totp, err3 := up(x.totp, "users.totp")
		if err := errors.Join(err1, err2, err3); err != nil {
			return errors.New("cannot decrypt stored fields: is this the right master key?")
		}
		if err := a.DB.Exec(ctx, `UPDATE users SET email_enc=?, phone_enc=?, totp_secret_enc=?, display_key=? WHERE id=?`,
			email, phone, totp, nameKey(x.name), x.id); err != nil {
			return err
		}
	}
	return nil
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

// cleanName prepares a display name: besides the general cleaning it removes
// invisible and text-direction characters, which could be used to make one
// name look like another.
func cleanName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Co, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return -1
		}
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, norm.NFC.String(s))
	return clean(strings.Join(strings.Fields(s), " "), 100)
}

// nameKey is the form of a display name used to detect duplicates:
// compatibility-normalised, lower case, without spaces. It does not catch
// look-alike letters from other alphabets; the client therefore also shows
// the unique @username next to names.
func nameKey(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(norm.NFKC.String(name))), "")
}

// nameTaken reports whether another active account already uses the name.
func nameTaken(ctx context.Context, q db.Q, name, exceptID string) (bool, error) {
	var n int
	err := q.Row(ctx, `SELECT COUNT(*) FROM users WHERE display_key=? AND id<>? AND deleted_at=0`, nameKey(name), exceptID).Scan(&n)
	return n > 0, err
}

// cleanEmail validates an e-mail address. Only a plain address is accepted:
// no display name and none of the characters that would let the value add
// parameters when it is used in a mailto: link.
func cleanEmail(s string) (string, error) {
	if s = clean(s, 200); s == "" {
		return "", nil
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || strings.ContainsAny(s, "?&%#<>\"'\\ ,;") || !strings.Contains(s[strings.LastIndex(s, "@")+1:], ".") {
		return "", errBad("invalid_email")
	}
	return s, nil
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
	if displayName = cleanName(displayName); displayName == "" {
		displayName = username
	}
	email, err := cleanEmail(email)
	if err != nil {
		return "", err
	}
	var n int
	if err := a.DB.Row(ctx, `SELECT COUNT(*) FROM users WHERE username=?`, username).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		return "", errBad("username_taken")
	}
	id := crypto.NewID()
	err = a.DB.InTx(ctx, func(tx *db.Tx) error {
		if taken, err := nameTaken(ctx, tx, displayName, id); err != nil {
			return err
		} else if taken {
			return errBad("display_name_taken")
		}
		if err := tx.Exec(ctx, `INSERT INTO users (id,username,display_name,display_key,email_enc,email_idx,password_hash,must_change_password,created_at)
			VALUES (?,?,?,?,?,?,?,?,?)`, id, username, displayName, nameKey(displayName), a.Keys.EncryptField(email, "users.email", id), a.Keys.BlindIndex(email),
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

// idleLimit returns the idle timeout in seconds (0 = none).
func (a *App) idleLimit() int64 { return int64(a.Cfg.Security.IdleTimeoutMinutes) * 60 }

// sessionAlive checks a stored session against expiry and the idle timeout.
func (a *App) sessionAlive(expires, lastSeen int64) bool {
	now := a.unix()
	if expires < now {
		return false
	}
	if idle := a.idleLimit(); idle > 0 && now-lastSeen > idle {
		return false
	}
	return true
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
	if errors.Is(err, sql.ErrNoRows) {
		return errUnauth
	}
	if err != nil {
		return err
	}
	if !a.sessionAlive(expires, lastSeen) {
		_ = a.DB.Exec(c.ctx(), `DELETE FROM sessions WHERE id_hash=?`, s.idHash)
		a.hub.kickSession(s.idHash)
		return errUnauth
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
	// last_seen drives the idle timeout. The web client refreshes data in the
	// background (live updates), which must not count as the person being
	// active: it marks requests that follow real interaction with a header.
	// Token (native app) sessions have no such header and always count.
	now := a.unix()
	active := bearer || c.R.Header.Get("X-User-Active") == "1" || a.idleLimit() == 0
	if active && now-lastSeen >= 60 {
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

// SignOutUser ends every session of an account (optionally keeping one) and
// closes its open event streams.
func (a *App) SignOutUser(ctx context.Context, uid, keepHash string) error {
	if err := a.DB.Exec(ctx, `DELETE FROM sessions WHERE user_id=? AND id_hash<>?`, uid, keepHash); err != nil {
		return err
	}
	a.hub.kickUser(uid, keepHash)
	return nil
}

// registerFailure counts a failed password or code check and locks the
// account when the limit is reached. Both statements are atomic in the
// database, so parallel attempts cannot under-count.
func (a *App) registerFailure(ctx context.Context, uid string) (locked bool) {
	ctx = context.WithoutCancel(ctx)
	if err := a.DB.Exec(ctx, `UPDATE users SET failed_logins=failed_logins+1 WHERE id=?`, uid); err != nil {
		a.Log.Error("count failed login", "err", err)
		return false
	}
	n, err := a.DB.ExecN(ctx, `UPDATE users SET failed_logins=0, locked_until=? WHERE id=? AND failed_logins>=?`, a.unix()+lockDuration, uid, maxFailures)
	return err == nil && n > 0
}

// isLocked reads the lock state fresh from the database.
func (a *App) isLocked(ctx context.Context, uid string) bool {
	var until int64
	_ = a.DB.Row(ctx, `SELECT locked_until FROM users WHERE id=?`, uid).Scan(&until)
	return until > a.unix()
}

// reauth checks the current password before a sensitive change made from an
// existing session. Wrong passwords count towards the account lockout, and
// when the lock triggers every session is ended: a session that is guessing
// its own password is presumed stolen.
func (a *App) reauth(c *Ctx, password string) error {
	if !a.login.allow(c.IP()) {
		return &apiError{http.StatusTooManyRequests, "rate_limited"}
	}
	if a.isLocked(c.ctx(), c.U.ID) {
		return &apiError{http.StatusTooManyRequests, "account_locked"}
	}
	if crypto.VerifyPassword(password, c.U.passwordHash) {
		_ = a.DB.Exec(c.ctx(), `UPDATE users SET failed_logins=0 WHERE id=?`, c.U.ID)
		return nil
	}
	a.logAct(c, true, "reauth.failed", "user", c.U.ID, "")
	if a.registerFailure(c.ctx(), c.U.ID) {
		_ = a.SignOutUser(c.ctx(), c.U.ID, "")
		a.logAct(c, true, "account.locked", "user", c.U.ID, "sessions ended")
		return errUnauth
	}
	return errBad("wrong_password")
}

// dummyHash makes login timing independent of whether the account exists.
var dummyHash = crypto.HashPassword("samling-dummy-password")

func (a *App) hLogin(c *Ctx) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`  // TOTP code or a recovery code
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
	if err := a.loadExtras(c.ctx(), u); err != nil {
		return err
	}
	now := a.unix()
	if u.lockedUntil > now {
		crypto.VerifyPassword(in.Password, dummyHash)
		return fail(u.ID, "locked")
	}
	bad := func(reason string) error {
		a.registerFailure(c.ctx(), u.ID)
		return fail(u.ID, reason)
	}
	if !crypto.VerifyPassword(in.Password, u.passwordHash) || u.Disabled {
		return bad("bad password")
	}
	// The password check takes long enough for parallel attempts to have
	// locked the account in the meantime.
	if a.isLocked(c.ctx(), u.ID) {
		return fail(u.ID, "locked")
	}
	code := strings.TrimSpace(in.Code)
	switch {
	case u.TOTPEnabled && code == "":
		return &apiError{http.StatusUnauthorized, "mfa_required"}
	case u.TOTPEnabled:
		if ok, err := a.checkSecondFactor(c, u, code); err != nil {
			return err
		} else if !ok {
			return bad("bad mfa code")
		}
	case u.MFARequired && u.Passkeys > 0:
		// The only second factor of this account is a passkey: a password
		// alone must not be enough.
		return &apiError{http.StatusUnauthorized, "passkey_required"}
	}
	return a.finishLogin(c, u, in.Token, "password")
}

// checkSecondFactor verifies a TOTP code or, failing that format, a recovery
// code. Both are single use: the TOTP time step and the recovery code are
// consumed with one conditional UPDATE, so a code cannot be replayed even by
// a parallel request.
func (a *App) checkSecondFactor(c *Ctx, u *User, code string) (bool, error) {
	digits := strings.ReplaceAll(code, " ", "")
	if len(digits) == 6 && strings.Trim(digits, "0123456789") == "" {
		step, ok := crypto.VerifyTOTP(u.totpSecret, digits, a.now())
		if !ok {
			return false, nil
		}
		n, err := a.DB.ExecN(c.ctx(), `UPDATE users SET totp_last_step=? WHERE id=? AND totp_last_step<?`, step, u.ID, step)
		return n == 1, err
	}
	hash := crypto.HashToken(crypto.NormalizeRecoveryCode(code))
	n, err := a.DB.ExecN(c.ctx(), `UPDATE recovery_codes SET used_at=? WHERE code_hash=? AND user_id=? AND used_at=0`, a.unix(), hash, u.ID)
	if err != nil || n != 1 {
		return false, err
	}
	a.logRaw(c.ctx(), u.ID, c.IP(), true, "mfa.recovery_used", "user", u.ID, "")
	return true, nil
}

// finishLogin creates the session after successful authentication.
func (a *App) finishLogin(c *Ctx, u *User, token bool, method string) error {
	if err := a.DB.Exec(c.ctx(), `UPDATE users SET failed_logins=0, locked_until=0, last_login_at=? WHERE id=?`, a.unix(), u.ID); err != nil {
		return err
	}
	kind := "cookie"
	if token {
		kind = "token"
	}
	tok, csrf, err := a.newSession(c, u, kind)
	if err != nil {
		return err
	}
	c.U = u
	a.logAct(c, true, "login", "user", u.ID, kind+", "+method)
	if token {
		return c.JSON(map[string]any{"token": tok})
	}
	return c.JSON(map[string]any{"csrf": csrf})
}

func (a *App) hLogout(c *Ctx) error {
	if err := a.DB.Exec(c.ctx(), `DELETE FROM sessions WHERE id_hash=?`, c.sess.idHash); err != nil {
		return err
	}
	a.hub.kickSession(c.sess.idHash)
	http.SetCookie(c.W, &http.Cookie{Name: a.cookie(c.R), Value: "", Path: "/", HttpOnly: true,
		Secure: a.secureCookies(c.R), SameSite: http.SameSiteStrictMode, MaxAge: -1})
	a.logAct(c, true, "logout", "user", c.U.ID, "")
	return c.OK()
}

// hSignOutEverywhere ends all other sessions of the caller.
func (a *App) hSignOutEverywhere(c *Ctx) error {
	if err := a.SignOutUser(c.ctx(), c.U.ID, c.sess.idHash); err != nil {
		return err
	}
	a.logAct(c, true, "sessions.ended", "user", c.U.ID, "by the user, other sessions")
	return c.OK()
}

func (a *App) hMe(c *Ctx) error {
	return c.JSON(map[string]any{"user": c.U, "csrf": c.sess.csrf, "pending": c.U.pending(),
		"org_name": a.Cfg.OrgName, "max_upload_mb": a.Cfg.Storage.MaxUploadMB, "min_password": a.Cfg.Security.MinPasswordLength,
		"idle_timeout_minutes": a.Cfg.Security.IdleTimeoutMinutes, "quota_mb": a.Cfg.Storage.UserQuotaMB})
}

func (a *App) hUpdateMe(c *Ctx) error {
	var in struct {
		DisplayName    *string           `json:"display_name"`
		Email          *string           `json:"email"`
		Phone          *string           `json:"phone"`
		ContactVisible *bool             `json:"contact_visible"`
		Locale         *string           `json:"locale"`
		Settings       *map[string]any   `json:"settings"`
		Profile        map[string]string `json:"profile"`
		ProfileShared  map[string]bool   `json:"profile_shared"`
		Avatar         *string           `json:"avatar"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	u := c.U
	if in.Profile != nil {
		u.Profile = cleanProfile(in.Profile)
	}
	if in.ProfileShared != nil {
		u.ProfileShared = cleanProfileShared(in.ProfileShared)
	}
	if in.Avatar != nil {
		if err := a.checkAvatar(c, *in.Avatar); err != nil {
			return err
		}
		u.Avatar = *in.Avatar
	}
	if in.DisplayName != nil {
		if u.DisplayName = cleanName(*in.DisplayName); u.DisplayName == "" {
			return errBad("name_required")
		}
		if taken, err := nameTaken(c.ctx(), a.DB, u.DisplayName, u.ID); err != nil {
			return err
		} else if taken {
			return errBad("display_name_taken")
		}
	}
	if in.Email != nil {
		e, err := cleanEmail(*in.Email)
		if err != nil {
			return err
		}
		u.Email = e
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
	profileEnc, profileShared := a.encodeProfile(u.ID, u.Profile, u.ProfileShared)
	err := a.DB.Exec(c.ctx(), `UPDATE users SET display_name=?, display_key=?, email_enc=?, email_idx=?, phone_enc=?, contact_visible=?, locale=?, settings=?,
		profile_enc=?, profile_shared=?, avatar=? WHERE id=?`,
		u.DisplayName, nameKey(u.DisplayName), a.Keys.EncryptField(u.Email, "users.email", u.ID), a.Keys.BlindIndex(u.Email),
		a.Keys.EncryptField(u.Phone, "users.phone", u.ID), b2i(u.ContactVisible), u.Locale, string(settings),
		profileEnc, profileShared, u.Avatar, u.ID)
	if err != nil {
		return err
	}
	if in.Avatar != nil {
		a.gcFiles(c.ctx()) // a replaced photo is no longer referenced
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
	if err := a.reauth(c, in.Current); err != nil {
		return err
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
	if err := a.SignOutUser(c.ctx(), c.U.ID, c.sess.idHash); err != nil {
		return err
	}
	a.logAct(c, true, "password.changed", "user", c.U.ID, "")
	return c.OK()
}

// ---------- two-step verification (TOTP) and recovery codes ----------

// hMFASetup generates a new (not yet active) TOTP secret. The current
// password is required, so that a stolen session alone cannot enrol a second
// factor and lock the owner out.
func (a *App) hMFASetup(c *Ctx) error {
	var in struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if c.U.TOTPEnabled {
		return errBad("mfa_already_enabled")
	}
	if err := a.reauth(c, in.Password); err != nil {
		return err
	}
	secret := crypto.NewTOTPSecret()
	if err := a.DB.Exec(c.ctx(), `UPDATE users SET totp_secret_enc=? WHERE id=?`, a.Keys.EncryptField(secret, "users.totp", c.U.ID), c.U.ID); err != nil {
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

const recoveryCodeCount = 10

// newRecoveryCodes replaces the account's recovery codes and returns the new
// ones. Only hashes are stored; the codes are shown to the user once.
func (a *App) newRecoveryCodes(ctx context.Context, q db.Q, uid string) ([]string, error) {
	if err := q.Exec(ctx, `DELETE FROM recovery_codes WHERE user_id=?`, uid); err != nil {
		return nil, err
	}
	codes := make([]string, recoveryCodeCount)
	for i := range codes {
		codes[i] = crypto.NewRecoveryCode()
		if err := q.Exec(ctx, `INSERT INTO recovery_codes (code_hash,user_id) VALUES (?,?)`,
			crypto.HashToken(crypto.NormalizeRecoveryCode(codes[i])), uid); err != nil {
			return nil, err
		}
	}
	return codes, nil
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
	if a.isLocked(c.ctx(), c.U.ID) {
		return &apiError{http.StatusTooManyRequests, "account_locked"}
	}
	step, ok := crypto.VerifyTOTP(c.U.totpSecret, in.Code, a.now())
	if !ok {
		a.registerFailure(c.ctx(), c.U.ID)
		return errBad("invalid_code")
	}
	var codes []string
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		if err := tx.Exec(c.ctx(), `UPDATE users SET totp_enabled=1, totp_last_step=?, failed_logins=0 WHERE id=?`, step, c.U.ID); err != nil {
			return err
		}
		var err error
		codes, err = a.newRecoveryCodes(c.ctx(), tx, c.U.ID)
		return err
	})
	if err != nil {
		return err
	}
	a.logAct(c, true, "mfa.enabled", "user", c.U.ID, "")
	return c.JSON(map[string]any{"recovery_codes": codes})
}

// hRecoveryCodes issues a fresh set of recovery codes (the old ones stop
// working).
func (a *App) hRecoveryCodes(c *Ctx) error {
	var in struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if !c.U.TOTPEnabled {
		return errBad("mfa_not_enabled")
	}
	if err := a.reauth(c, in.Password); err != nil {
		return err
	}
	var codes []string
	err := a.DB.InTx(c.ctx(), func(tx *db.Tx) error {
		var err error
		codes, err = a.newRecoveryCodes(c.ctx(), tx, c.U.ID)
		return err
	})
	if err != nil {
		return err
	}
	a.logAct(c, true, "mfa.recovery_codes_created", "user", c.U.ID, "")
	return c.JSON(map[string]any{"recovery_codes": codes})
}

func (a *App) hMFADisable(c *Ctx) error {
	var in struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if c.U.MFARequired && c.U.Passkeys == 0 {
		return errBad("mfa_is_required")
	}
	if err := a.reauth(c, in.Password); err != nil {
		return err
	}
	if err := resetTOTP(c.ctx(), a.DB, c.U.ID); err != nil {
		return err
	}
	a.logAct(c, true, "mfa.disabled", "user", c.U.ID, "")
	return c.OK()
}

// resetTOTP removes the TOTP secret and the recovery codes of an account.
func resetTOTP(ctx context.Context, q db.Q, uid string) error {
	if err := q.Exec(ctx, `UPDATE users SET totp_enabled=0, totp_secret_enc='', totp_last_step=0 WHERE id=?`, uid); err != nil {
		return err
	}
	return q.Exec(ctx, `DELETE FROM recovery_codes WHERE user_id=?`, uid)
}
