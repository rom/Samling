package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/rom/samling/internal/crypto"
)

// Passkeys (WebAuthn).
//
// A passkey is registered from a signed-in session after re-entering the
// password. Signing in with a passkey needs no user name or password: the
// authenticator proves possession of the key and verifies the person (PIN or
// biometrics, "user verification" is required), which makes it two factors
// in one step. A passkey therefore also satisfies an administrator's
// requirement for two-step verification.

// ceremonies keeps the server side state of registrations and sign-ins that
// are in progress (the challenge), for a few minutes.
type ceremonies struct {
	mu sync.Mutex
	m  map[string]ceremony
}

type ceremony struct {
	data    webauthn.SessionData
	uid     string // registration: the account; sign-in: empty
	expires time.Time
}

func newCeremonies() *ceremonies { return &ceremonies{m: map[string]ceremony{}} }

func (cs *ceremonies) put(data webauthn.SessionData, uid string) string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	now := time.Now()
	if len(cs.m) > 5000 { // bound memory: drop what has expired, then refuse to grow without limit
		for k, v := range cs.m {
			if now.After(v.expires) {
				delete(cs.m, k)
			}
		}
		if len(cs.m) > 5000 {
			cs.m = map[string]ceremony{}
		}
	}
	id := crypto.Token(18)
	cs.m[id] = ceremony{data: data, uid: uid, expires: now.Add(5 * time.Minute)}
	return id
}

// take returns a ceremony once; a challenge can never be used twice.
func (cs *ceremonies) take(id string) (ceremony, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	c, ok := cs.m[id]
	delete(cs.m, id)
	return c, ok && time.Now().Before(c.expires)
}

// relyingParty builds the WebAuthn configuration. Passkeys are bound to the
// site's host name: server.public_url when configured, otherwise the address
// of the request.
func (a *App) relyingParty(r *http.Request) (*webauthn.WebAuthn, error) {
	origin := a.Cfg.Server.PublicURL
	if origin == "" {
		scheme := "http"
		if a.secureCookies(r) {
			scheme = "https"
		}
		host := r.Host
		if a.Cfg.Server.TrustedProxy && r.Header.Get("X-Forwarded-Host") != "" {
			host = r.Header.Get("X-Forwarded-Host")
		}
		origin = scheme + "://" + host
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return nil, errBad("passkeys_unavailable")
	}
	rp, err := webauthn.New(&webauthn.Config{
		RPID:          u.Hostname(),
		RPDisplayName: a.Cfg.OrgName,
		RPOrigins:     []string{origin},
	})
	if err != nil {
		// Passkeys are bound to a host name: they cannot be used when the
		// site is reached by IP address.
		return nil, errBad("passkeys_unavailable")
	}
	return rp, nil
}

// waUser adapts an account to the library's user interface.
type waUser struct {
	u     *User
	creds []webauthn.Credential
}

func (w waUser) WebAuthnID() []byte                         { return []byte(w.u.ID) }
func (w waUser) WebAuthnName() string                       { return w.u.Username }
func (w waUser) WebAuthnDisplayName() string                { return w.u.DisplayName }
func (w waUser) WebAuthnCredentials() []webauthn.Credential { return w.creds }

func (a *App) credentials(ctx context.Context, uid string) ([]webauthn.Credential, error) {
	var out []webauthn.Credential
	err := a.DB.Each(ctx, `SELECT data FROM passkeys WHERE user_id=?`, []any{uid}, func(r *sql.Rows) error {
		var raw string
		if err := r.Scan(&raw); err != nil {
			return err
		}
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	return out, err
}

func credKey(id []byte) string { return base64.RawURLEncoding.EncodeToString(id) }

type Passkey struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"created_at"`
	LastUsedAt int64  `json:"last_used_at"`
}

func (a *App) hPasskeys(c *Ctx) error {
	out := []Passkey{}
	err := a.DB.Each(c.ctx(), `SELECT id, name, created_at, last_used_at FROM passkeys WHERE user_id=? ORDER BY id`, []any{c.U.ID}, func(r *sql.Rows) error {
		var p Passkey
		if err := r.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.LastUsedAt); err != nil {
			return err
		}
		out = append(out, p)
		return nil
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// hPasskeyRegisterBegin starts a registration. The password is required so
// that a stolen session alone cannot add a sign-in method.
func (a *App) hPasskeyRegisterBegin(c *Ctx) error {
	var in struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.reauth(c, in.Password); err != nil {
		return err
	}
	if c.U.Passkeys >= 10 {
		return errBad("too_many_passkeys")
	}
	rp, err := a.relyingParty(c.R)
	if err != nil {
		return err
	}
	creds, err := a.credentials(c.ctx(), c.U.ID)
	if err != nil {
		return err
	}
	var exclude []protocol.CredentialDescriptor
	for _, cr := range creds {
		exclude = append(exclude, cr.Descriptor())
	}
	options, sess, err := rp.BeginRegistration(waUser{c.U, creds},
		webauthn.WithExclusions(exclude),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation))
	if err != nil {
		return err
	}
	return c.JSON(map[string]any{"ceremony": a.ceremonies.put(*sess, c.U.ID), "options": options.Response})
}

func (a *App) hPasskeyRegisterFinish(c *Ctx) error {
	var in struct {
		Ceremony   string          `json:"ceremony"`
		Name       string          `json:"name"`
		Credential json.RawMessage `json:"credential"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	cer, ok := a.ceremonies.take(in.Ceremony)
	if !ok || cer.uid != c.U.ID {
		return errBad("passkey_expired")
	}
	rp, err := a.relyingParty(c.R)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(in.Credential))
	if err != nil {
		return errBad("passkey_failed")
	}
	creds, err := a.credentials(c.ctx(), c.U.ID)
	if err != nil {
		return err
	}
	cred, err := rp.CreateCredential(waUser{c.U, creds}, cer.data, parsed)
	if err != nil {
		a.Log.Info("passkey registration rejected", "user", c.U.ID, "err", err)
		return errBad("passkey_failed")
	}
	if !cred.Flags.UserVerified {
		return errBad("passkey_failed")
	}
	data, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	name := clean(in.Name, 60)
	if name == "" {
		name = "Passkey"
	}
	id := crypto.NewID()
	if err := a.DB.Exec(c.ctx(), `INSERT INTO passkeys (id,user_id,name,cred_id,data,created_at) VALUES (?,?,?,?,?,?)`,
		id, c.U.ID, name, credKey(cred.ID), string(data), a.unix()); err != nil {
		return errBad("passkey_failed") // the credential id is already registered
	}
	a.logAct(c, true, "passkey.added", "user", c.U.ID, name)
	return c.JSON(map[string]string{"id": id})
}

func (a *App) hPasskeyDelete(c *Ctx) error {
	var in struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if err := a.reauth(c, in.Password); err != nil {
		return err
	}
	if c.U.MFARequired && !c.U.TOTPEnabled && c.U.Passkeys <= 1 {
		return errBad("mfa_is_required")
	}
	n, err := a.DB.ExecN(c.ctx(), `DELETE FROM passkeys WHERE id=? AND user_id=?`, c.Param("id"), c.U.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return errNotFound
	}
	a.logAct(c, true, "passkey.removed", "user", c.U.ID, "")
	return c.OK()
}

// hPasskeyLoginBegin starts a sign-in. No account is named: the authenticator
// offers the passkeys it holds for this site.
func (a *App) hPasskeyLoginBegin(c *Ctx) error {
	if !a.login.allow(c.IP()) {
		return &apiError{http.StatusTooManyRequests, "rate_limited"}
	}
	rp, err := a.relyingParty(c.R)
	if err != nil {
		return err
	}
	options, sess, err := rp.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return err
	}
	return c.JSON(map[string]any{"ceremony": a.ceremonies.put(*sess, ""), "options": options.Response})
}

func (a *App) hPasskeyLoginFinish(c *Ctx) error {
	var in struct {
		Ceremony   string          `json:"ceremony"`
		Credential json.RawMessage `json:"credential"`
		Token      bool            `json:"token"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	if !a.login.allow(c.IP()) {
		return &apiError{http.StatusTooManyRequests, "rate_limited"}
	}
	if in.Token && c.R.Header.Get("Origin") != "" && !a.sameOrigin(c.R) {
		return &apiError{http.StatusForbidden, "cross_origin"}
	}
	denied := &apiError{http.StatusUnauthorized, "invalid_credentials"}
	cer, ok := a.ceremonies.take(in.Ceremony)
	if !ok || cer.uid != "" {
		return errBad("passkey_expired")
	}
	rp, err := a.relyingParty(c.R)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(in.Credential))
	if err != nil {
		return denied
	}
	var u *User
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		// Only accept a credential that is registered to the account named
		// by the user handle.
		var owner string
		if err := a.DB.Row(c.ctx(), `SELECT user_id FROM passkeys WHERE cred_id=?`, credKey(rawID)).Scan(&owner); err != nil || owner != string(userHandle) {
			return nil, errors.New("unknown credential")
		}
		var err error
		if u, err = a.userByID(c.ctx(), owner); err != nil {
			return nil, err
		}
		creds, err := a.credentials(c.ctx(), owner)
		return waUser{u, creds}, err
	}
	_, cred, err := rp.ValidatePasskeyLogin(handler, cer.data, parsed)
	if err != nil || u == nil {
		a.logRaw(c.ctx(), "", c.IP(), true, "login.failed", "user", "", "passkey rejected")
		return denied
	}
	if u.Disabled || u.deletedAt != 0 || u.lockedUntil > a.unix() || !cred.Flags.UserVerified {
		a.logRaw(c.ctx(), u.ID, c.IP(), true, "login.failed", "user", u.ID, "passkey: account not usable")
		return denied
	}
	if cred.Authenticator.CloneWarning {
		// The signature counter went backwards: the key may have been copied.
		a.logRaw(c.ctx(), u.ID, c.IP(), true, "passkey.clone_warning", "user", u.ID, "")
		return denied
	}
	data, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	if err := a.DB.Exec(c.ctx(), `UPDATE passkeys SET data=?, last_used_at=? WHERE cred_id=?`, string(data), a.unix(), credKey(cred.ID)); err != nil {
		return err
	}
	return a.finishLogin(c, u, in.Token, "passkey")
}
