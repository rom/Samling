package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rom/samling/internal/blob"
	"github.com/rom/samling/internal/config"
	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

// The suite runs against SQLite by default. Set SAMLING_TEST_PG_DSN to run
// the same tests against PostgreSQL (each test gets its own schema).

const testPW = "correct horse battery staple"

type env struct {
	t   *testing.T
	a   *App
	srv *httptest.Server
	dir string
}

var pgSchemaSeq int

func openTestDB(t *testing.T, dir string) *db.DB {
	t.Helper()
	if dsn := os.Getenv("SAMLING_TEST_PG_DSN"); dsn != "" {
		pgSchemaSeq++
		schema := fmt.Sprintf("t%d_%d", time.Now().UnixNano(), pgSchemaSeq)
		admin, err := db.Open("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		if err := admin.Exec(context.Background(), `CREATE SCHEMA `+schema); err != nil {
			t.Fatal(err)
		}
		admin.Close()
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		d, err := db.Open("postgres", dsn+sep+"search_path="+schema)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			d.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
			d.Close()
		})
		return d
	}
	d, err := db.Open("sqlite", filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = dir
	cfg.Storage.MaxUploadMB = 1
	if err := cfg.Finish(); err != nil {
		t.Fatal(err)
	}
	keys, _ := crypto.NewKeys(crypto.GenerateKey())
	fsb, err := blob.NewFS(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	static := fstest.MapFS{"index.html": {Data: []byte("<html>samling</html>")}}
	a, err := New(cfg, openTestDB(t, dir), blob.NewStore(fsb, keys), keys, static, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	a.login = newLimiter(100000, time.Minute)
	a.api = newLimiter(1000000, time.Minute)
	srv := httptest.NewServer(a)
	t.Cleanup(srv.Close)
	return &env{t: t, a: a, srv: srv, dir: dir}
}

type client struct {
	e      *env
	cookie *http.Cookie
	csrf   string
	token  string
	ID     string
}

// user creates an account with the given roles and signs it in.
func (e *env) user(name string, roles ...string) *client {
	e.t.Helper()
	if roles == nil {
		roles = []string{RoleMember}
	}
	id, err := e.a.CreateUser(context.Background(), name, strings.ToUpper(name[:1])+name[1:], name+"@example.org", testPW, roles, false)
	if err != nil {
		e.t.Fatal(err)
	}
	c := e.login(name, testPW)
	c.ID = id
	return c
}

func (e *env) login(name, pw string) *client {
	e.t.Helper()
	c := &client{e: e}
	body, _ := json.Marshal(map[string]string{"username": name, "password": pw})
	resp, err := http.Post(e.srv.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		e.t.Fatalf("login %s: %d %s", name, resp.StatusCode, b)
	}
	var out struct{ CSRF string }
	json.NewDecoder(resp.Body).Decode(&out)
	c.csrf = out.CSRF
	for _, ck := range resp.Cookies() {
		if ck.Name == cookieName {
			c.cookie = ck
			if !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode {
				e.t.Fatal("session cookie must be HttpOnly and SameSite=Strict")
			}
		}
	}
	if c.cookie == nil {
		e.t.Fatal("no session cookie")
	}
	return c
}

func (c *client) raw(method, path string, body io.Reader, ctype string) (int, []byte, http.Header) {
	c.e.t.Helper()
	req, _ := http.NewRequest(method, c.e.srv.URL+path, body)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.cookie != nil {
		req.AddCookie(c.cookie)
		if c.csrf != "" {
			req.Header.Set("X-CSRF-Token", c.csrf)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, resp.Header
}

// call performs a JSON request and returns the status and raw body.
func (c *client) call(method, path string, body any) (int, []byte) {
	c.e.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	st, b, _ := c.raw(method, path, r, "application/json")
	return st, b
}

// ok performs a request that must succeed and decodes the response into out.
func (c *client) ok(method, path string, body, out any) {
	c.e.t.Helper()
	st, b := c.call(method, path, body)
	if st != 200 {
		c.e.t.Fatalf("%s %s: status %d: %s", method, path, st, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			c.e.t.Fatalf("%s %s: %v: %s", method, path, err, b)
		}
	}
}

// fail performs a request that must fail with the given status.
func (c *client) fail(want int, method, path string, body any) string {
	c.e.t.Helper()
	st, b := c.call(method, path, body)
	if st != want {
		c.e.t.Fatalf("%s %s: status %d, want %d: %s", method, path, st, want, b)
	}
	var e struct{ Error string }
	json.Unmarshal(b, &e)
	return e.Error
}

// id performs a creating request and returns the new id.
func (c *client) id(method, path string, body any) string {
	c.e.t.Helper()
	var out struct{ ID string }
	c.ok(method, path, body, &out)
	if out.ID == "" {
		c.e.t.Fatalf("%s %s returned no id", method, path)
	}
	return out.ID
}

func (c *client) upload(name string, content []byte) FileInfo {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write(content)
	mw.Close()
	st, b, _ := c.raw("POST", "/api/files", &buf, mw.FormDataContentType())
	if st != 200 {
		c.e.t.Fatalf("upload: %d %s", st, b)
	}
	var f FileInfo
	json.Unmarshal(b, &f)
	return f
}

type M = map[string]any

func all() []M { return []M{{"type": "all", "id": "", "level": 1}} }

func share(typ, id string, level int) M { return M{"type": typ, "id": id, "level": level} }

func ids(list []M) map[string]bool {
	out := map[string]bool{}
	for _, x := range list {
		out[x["id"].(string)] = true
	}
	return out
}
