// Package app implements the Samling HTTP API and serves the web client.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rom/samling/internal/blob"
	"github.com/rom/samling/internal/config"
	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

type App struct {
	Cfg   *config.Config
	DB    *db.DB
	Blobs *blob.Store
	Keys  *crypto.Keys
	Log   *slog.Logger
	hub   *hub
	// Limits are per client address. They are sized so that a whole club
	// behind one address (a meeting on shared Wi-Fi) is not locked out;
	// password guessing is additionally stopped by the per-account lockout.
	login *limiter // password checks
	api   *limiter // all API calls
	mux   *http.ServeMux
	now   func() time.Time
	// streamPing is how often an open event stream is re-validated.
	streamPing time.Duration
	ceremonies *ceremonies // passkey registrations / sign-ins in progress
}

func New(cfg *config.Config, d *db.DB, blobs *blob.Store, keys *crypto.Keys, static fs.FS, log *slog.Logger) (*App, error) {
	a := &App{Cfg: cfg, DB: d, Blobs: blobs, Keys: keys, Log: log, hub: newHub(),
		login: newLimiter(30, time.Minute), api: newLimiter(3000, time.Minute), now: time.Now,
		streamPing: 25 * time.Second, ceremonies: newCeremonies()}
	if err := a.seed(context.Background()); err != nil {
		return nil, err
	}
	a.mux = http.NewServeMux()
	a.routes()
	a.mux.Handle("/", a.staticHandler(static))
	return a, nil
}

// ---------- errors ----------

type apiError struct {
	Status int
	Code   string
}

func (e *apiError) Error() string { return e.Code }

func errBad(code string) error { return &apiError{http.StatusBadRequest, code} }

var (
	errNotFound  = &apiError{http.StatusNotFound, "not_found"}
	errForbidden = &apiError{http.StatusForbidden, "forbidden"}
	errUnauth    = &apiError{http.StatusUnauthorized, "unauthorized"}
)

// ---------- request context ----------

type Ctx struct {
	W    http.ResponseWriter
	R    *http.Request
	A    *App
	U    *User
	sess *session
}

func (c *Ctx) ctx() context.Context { return c.R.Context() }

func (c *Ctx) JSON(v any) error {
	c.W.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(c.W).Encode(v)
}

func (c *Ctx) OK() error { return c.JSON(map[string]bool{"ok": true}) }

// Bind decodes a JSON request body (max 1 MiB, unknown fields rejected).
func (c *Ctx) Bind(v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(c.W, c.R.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errBad("invalid_json")
	}
	return nil
}

func (c *Ctx) Param(name string) string { return c.R.PathValue(name) }
func (c *Ctx) Query(name string) string { return c.R.URL.Query().Get(name) }

func (c *Ctx) QueryInt(name string, def int64) int64 {
	if n, err := strconv.ParseInt(c.Query(name), 10, 64); err == nil {
		return n
	}
	return def
}

func (c *Ctx) IP() string { return c.A.clientIP(c.R) }

// limiterKey is the rate limiting key of a client address. IPv6 clients
// control a whole /64, so the limit applies to that prefix.
func limiterKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil || addr.Is4() || addr.Is4In6() {
		return ip
	}
	p, err := addr.Prefix(64)
	if err != nil {
		return ip
	}
	return p.String()
}

func (a *App) clientIP(r *http.Request) string {
	if a.Cfg.Server.TrustedProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *App) unix() int64 { return a.now().Unix() }

// ---------- routing ----------

type handler func(c *Ctx) error

type routeOpt struct {
	public  bool   // no session needed
	perm    string // required permission
	limited bool   // allowed while password change / MFA setup is pending
}

func (a *App) handle(pattern string, opt routeOpt, h handler) {
	a.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		c := &Ctx{W: w, R: r, A: a}
		err := func() error {
			if !a.api.allow(a.clientIP(r)) {
				return &apiError{http.StatusTooManyRequests, "rate_limited"}
			}
			// Defence in depth on top of the CSRF token and SameSite cookies:
			// a browser request that changes state must come from our own
			// origin. Token (native app) requests are not cookie based.
			if r.Method != http.MethodGet && r.Method != http.MethodHead && !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") && !a.sameOrigin(r) {
				return &apiError{http.StatusForbidden, "cross_origin"}
			}
			if !opt.public {
				if err := a.authenticate(c); err != nil {
					return err
				}
				if !opt.limited && c.U.pending() != "" {
					return &apiError{http.StatusForbidden, c.U.pending()}
				}
				if opt.perm != "" && !c.U.Can(opt.perm) {
					return errForbidden
				}
			}
			return h(c)
		}()
		if err != nil {
			a.writeError(c, err)
		}
	})
}

func (a *App) writeError(c *Ctx, err error) {
	var ae *apiError
	var mbe *http.MaxBytesError
	switch {
	case errors.As(err, &ae):
	case errors.As(err, &mbe):
		ae = &apiError{http.StatusRequestEntityTooLarge, "too_large"}
	case errors.Is(err, context.Canceled):
		return
	default:
		// Never leak internals to the client; log with a reference instead.
		ref := crypto.Token(6)
		a.Log.Error("internal error", "ref", ref, "path", c.R.URL.Path, "err", err)
		c.W.Header().Set("Content-Type", "application/json; charset=utf-8")
		c.W.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(c.W).Encode(map[string]string{"error": "internal", "ref": ref})
		return
	}
	c.W.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.W.WriteHeader(ae.Status)
	json.NewEncoder(c.W).Encode(map[string]string{"error": ae.Code})
}

// ServeHTTP applies the global middleware: panic recovery and security headers.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			a.Log.Error("panic", "err", fmt.Sprint(rec), "stack", string(debug.Stack()))
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		}
	}()
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; "+
		"connect-src 'self'; font-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	if a.secureCookies(r) {
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		h.Set("Cache-Control", "no-store")
	}
	a.mux.ServeHTTP(w, r)
}

// sameOrigin reports whether a request was made by a page of this site.
// Non-browser clients send no Origin header and are let through (they
// cannot be used for CSRF because they carry no ambient cookies).
func (a *App) sameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	// Behind a trusted reverse proxy the public host may arrive in
	// X-Forwarded-Host when the proxy rewrites Host.
	return u.Host == r.Host || (a.Cfg.Server.TrustedProxy && u.Host == r.Header.Get("X-Forwarded-Host"))
}

func (a *App) secureCookies(r *http.Request) bool {
	return r.TLS != nil || (a.Cfg.Server.TrustedProxy && r.Header.Get("X-Forwarded-Proto") == "https")
}

func (a *App) staticHandler(static fs.FS) http.Handler {
	files := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			a.writeError(&Ctx{W: w, R: r, A: a}, errNotFound)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Assets are revalidated on each load so upgrades take effect at once.
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
}

// ---------- rate limiter (fixed window per key) ----------

type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string]*hit
}

type hit struct {
	n     int
	reset time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, hits: map[string]*hit{}}
}

func (l *limiter) allow(key string) bool {
	key = limiterKey(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.hits) > 50000 { // bound memory under address-spraying
		for k, h := range l.hits {
			if now.After(h.reset) {
				delete(l.hits, k)
			}
		}
	}
	h := l.hits[key]
	if h == nil || now.After(h.reset) {
		l.hits[key] = &hit{n: 1, reset: now.Add(l.window)}
		return true
	}
	h.n++
	return h.n <= l.max
}

// ---------- activity / audit log ----------

// logAct records an entry in the user's personal history. Audit entries are
// additionally visible to administrators with the logs.view permission.
func (a *App) logAct(c *Ctx, audit bool, action, resType, resID, summary string) {
	uid := ""
	if c.U != nil {
		uid = c.U.ID
	}
	a.logRaw(c.ctx(), uid, c.IP(), audit, action, resType, resID, summary)
}

func (a *App) logRaw(ctx context.Context, uid, ip string, audit bool, action, resType, resID, summary string) {
	au := 0
	if audit {
		au = 1
	} else {
		ip = "" // IP addresses are personal data: only keep them for security events
	}
	if len(summary) > 300 {
		summary = summary[:300]
	}
	err := a.DB.Exec(context.WithoutCancel(ctx), `INSERT INTO activity (id,user_id,action,res_type,res_id,summary,ip,audit,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		crypto.NewID(), uid, action, resType, resID, summary, ip, au, a.unix())
	if err != nil {
		a.Log.Error("activity log", "err", err)
	}
}

// ---------- small helpers ----------

// clean prepares text input for storage: valid UTF-8, no control characters
// (line breaks and tabs are kept, with line endings normalised), trimmed and
// limited in length. Removing control characters keeps stored text from
// smuggling line breaks or NUL bytes into exports and database drivers.
func clean(s string, max int) string {
	s = strings.ReplaceAll(strings.ToValidUTF8(s, ""), "\r\n", "\n")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r == '\r' {
			return '\n'
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}

// maxTime is the latest timestamp accepted from clients (year 9999). Bounding
// times keeps date arithmetic in SQL from overflowing.
const maxTime = 253402300799

func validTime(t int64) bool { return t >= 0 && t <= maxTime }

// limitCount refuses to create more rows once a per-owner or per-parent
// limit is reached (protection against filling the database).
func limitCount(ctx context.Context, q db.Q, max int, query string, args ...any) error {
	var n int
	if err := q.Row(ctx, query, args...).Scan(&n); err != nil {
		return err
	}
	if n >= max {
		return &apiError{http.StatusConflict, "limit_reached"}
	}
	return nil
}

// Per-owner and per-parent limits.
const (
	maxPerOwner     = 1000 // lists, boards, polls, documents, groups per account
	maxEventsOwner  = 10000
	maxItemsPerList = 2000
	maxCardsBoard   = 2000
)

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func copyClose(w io.Writer, rc io.ReadCloser) error {
	defer rc.Close()
	_, err := io.Copy(w, rc)
	return err
}

// PurgeLoop removes expired sessions and old audit data periodically.
func (a *App) PurgeLoop(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		now := a.unix()
		_ = a.DB.Exec(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now)
		// Data minimisation: activity older than two years is dropped.
		_ = a.DB.Exec(ctx, `DELETE FROM activity WHERE created_at < ?`, now-2*365*86400)
		a.gcFiles(ctx) // uploads that were never attached to anything
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
