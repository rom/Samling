package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rom/samling/internal/blob"
	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
)

func TestSecurityHeadersAndStatic(t *testing.T) {
	e := newEnv(t)
	anon := &client{e: e}
	st, body, h := anon.raw("GET", "/", nil, "")
	if st != 200 || !strings.Contains(string(body), "samling") {
		t.Fatalf("index: %d", st)
	}
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY", "Referrer-Policy": "no-referrer",
	} {
		if h.Get(k) != want {
			t.Errorf("%s = %q", k, h.Get(k))
		}
	}
	csp := h.Get("Content-Security-Policy")
	// 'wasm-unsafe-eval' only permits compiling WebAssembly (the on-device text
	// recognition); inline script and eval() stay forbidden.
	if !strings.Contains(csp, "script-src 'self' 'wasm-unsafe-eval';") || strings.Contains(csp, "'unsafe-inline'") || strings.Contains(csp, "'unsafe-eval'") {
		t.Errorf("weak CSP: %s", csp)
	}
	anon.fail(401, "GET", "/api/me", nil)
	anon.fail(401, "GET", "/api/channels", nil)

	// State changing requests from another web origin are refused, even with
	// a valid session and CSRF token.
	anna := e.user("anna")
	for _, hdr := range [][2]string{{"Origin", "https://evil.example"}, {"Sec-Fetch-Site", "cross-site"}} {
		req, _ := http.NewRequest("PATCH", e.srv.URL+"/api/me", strings.NewReader(`{"display_name":"Mallory"}`))
		req.Header.Set(hdr[0], hdr[1])
		req.Header.Set("X-CSRF-Token", anna.csrf)
		req.AddCookie(anna.cookie)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("cross-origin write with %s: status %d", hdr[0], resp.StatusCode)
		}
	}
	req, _ := http.NewRequest("PATCH", e.srv.URL+"/api/me", strings.NewReader(`{"display_name":"Anna"}`))
	req.Header.Set("Origin", e.srv.URL)
	req.Header.Set("X-CSRF-Token", anna.csrf)
	req.AddCookie(anna.cookie)
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != 200 {
		t.Errorf("same-origin write refused: %v", err)
	}
	anon.fail(404, "GET", "/api/nope", nil)
}

func TestLoginCSRFAndLockout(t *testing.T) {
	e := newEnv(t)
	anna := e.user("anna")
	var me struct {
		User User
		CSRF string
	}
	anna.ok("GET", "/api/me", nil, &me)
	if me.User.Username != "anna" || me.User.Email != "anna@example.org" {
		t.Fatalf("me: %+v", me.User)
	}
	// Writes without the CSRF token are refused.
	noCSRF := &client{e: e, cookie: anna.cookie}
	if code := noCSRF.fail(403, "PATCH", "/api/me", M{"display_name": "X"}); code != "csrf" {
		t.Fatalf("got %q", code)
	}
	wrong := &client{e: e, cookie: anna.cookie, csrf: "nope"}
	wrong.fail(403, "PATCH", "/api/me", M{"display_name": "X"})
	// A cookie session token is not accepted as a bearer token.
	(&client{e: e, token: anna.cookie.Value}).fail(401, "GET", "/api/me", nil)

	// Bearer token login for native apps: no CSRF token needed.
	anon := &client{e: e}
	var tok struct{ Token string }
	anon.ok("POST", "/api/auth/login", M{"username": "anna", "password": testPW, "token": true}, &tok)
	app := &client{e: e, token: tok.Token}
	app.ok("PATCH", "/api/me", M{"display_name": "Anna A"}, nil)

	// Wrong passwords: same error for unknown and known accounts, then lockout.
	if anon.fail(401, "POST", "/api/auth/login", M{"username": "nobody", "password": "x"}) !=
		anon.fail(401, "POST", "/api/auth/login", M{"username": "anna", "password": "x"}) {
		t.Fatal("login errors must not reveal whether an account exists")
	}
	for i := 0; i < 9; i++ {
		anon.fail(401, "POST", "/api/auth/login", M{"username": "anna", "password": "wrong"})
	}
	anon.fail(401, "POST", "/api/auth/login", M{"username": "anna", "password": testPW}) // locked now

	// Logout invalidates the session.
	anna.ok("POST", "/api/auth/logout", nil, nil)
	anna.fail(401, "GET", "/api/me", nil)
}

func TestForcedPasswordChangeAndMFA(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	uid := admin.id("POST", "/api/admin/users", M{"username": "bert", "display_name": "Bert", "password": "temporary-pass-1"})
	bert := e.login("bert", "temporary-pass-1")
	if code := bert.fail(403, "GET", "/api/channels", nil); code != "password_change_required" {
		t.Fatalf("got %q", code)
	}
	bert.fail(400, "POST", "/api/me/password", M{"current": "temporary-pass-1", "new": "short"})
	bert.ok("POST", "/api/me/password", M{"current": "temporary-pass-1", "new": testPW}, nil)
	bert.ok("GET", "/api/channels", nil, nil)

	// The admin requires MFA: the account is gated until it is enrolled.
	admin.ok("PATCH", "/api/admin/users/"+uid, M{"mfa_required": true}, nil)
	if code := bert.fail(403, "GET", "/api/channels", nil); code != "mfa_setup_required" {
		t.Fatalf("got %q", code)
	}
	var setup struct{ Secret, URI, QR string }
	bert.fail(400, "POST", "/api/me/mfa/setup", M{"password": "not the password"}) // a session alone is not enough
	bert.ok("POST", "/api/me/mfa/setup", M{"password": testPW}, &setup)
	if !strings.HasPrefix(setup.URI, "otpauth://totp/") || !strings.HasPrefix(setup.QR, "data:image/png;base64,") {
		t.Fatalf("setup: %+v", setup)
	}
	bert.fail(400, "POST", "/api/me/mfa/enable", M{"code": "000000"})
	code, _ := crypto.TOTPCode(setup.Secret, time.Now())
	bert.ok("POST", "/api/me/mfa/enable", M{"code": code}, nil)
	bert.ok("GET", "/api/channels", nil, nil)
	bert.fail(400, "POST", "/api/me/mfa/disable", M{"password": testPW}) // required by admin

	anon := &client{e: e}
	if c := anon.fail(401, "POST", "/api/auth/login", M{"username": "bert", "password": testPW}); c != "mfa_required" {
		t.Fatalf("got %q", c)
	}
	// The code used for enrolment cannot be replayed.
	anon.fail(401, "POST", "/api/auth/login", M{"username": "bert", "password": testPW, "code": code})
	next, _ := crypto.TOTPCode(setup.Secret, time.Now().Add(30*time.Second))
	anon.ok("POST", "/api/auth/login", M{"username": "bert", "password": testPW, "code": next}, nil)

	// The secret is stored encrypted.
	var stored string
	e.a.DB.Row(context.Background(), `SELECT totp_secret_enc FROM users WHERE id=?`, uid).Scan(&stored)
	if stored == "" || strings.Contains(stored, setup.Secret) {
		t.Fatal("TOTP secret must be encrypted at rest")
	}
	// Admin reset.
	admin.ok("PATCH", "/api/admin/users/"+uid, M{"reset_mfa": true, "mfa_required": false}, nil)
	anon.ok("POST", "/api/auth/login", M{"username": "bert", "password": testPW}, nil)
}

func TestAdminAndRBAC(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	leader := e.user("lena", RoleLeader)
	member := e.user("mia")

	// Members cannot reach administration or gated features.
	member.fail(403, "GET", "/api/admin/users", nil)
	member.fail(403, "POST", "/api/admin/users", M{"username": "x1", "password": testPW})
	member.fail(403, "GET", "/api/admin/logs", nil)
	member.fail(403, "POST", "/api/admin/backup", M{"passphrase": "a long passphrase"})
	member.fail(403, "POST", "/api/channels", M{"name": "x"})
	member.fail(403, "POST", "/api/roles", M{"name": "x"})
	member.fail(403, "POST", "/api/decisions", M{"title": "x"})
	member.fail(403, "POST", "/api/meetings", M{"title": "x", "starts_at": 10, "ends_at": 20})
	member.fail(403, "POST", "/api/groups", M{"name": "g", "scope": "global"})
	leader.fail(403, "GET", "/api/admin/users", nil)
	leader.ok("POST", "/api/channels", M{"name": "board", "shares": all()}, nil)

	// Custom role with one permission.
	rid := admin.id("POST", "/api/roles", M{"name": "Secretary", "perms": []string{PermMeetings}})
	admin.fail(400, "POST", "/api/roles", M{"name": "Bad", "perms": []string{"root"}})
	admin.ok("PATCH", "/api/admin/users/"+member.ID, M{"roles": []string{RoleMember, rid}}, nil)
	member.ok("POST", "/api/meetings", M{"title": "AGM", "starts_at": 1000, "ends_at": 2000}, nil)
	admin.ok("DELETE", "/api/roles/"+rid, nil, nil)
	member.fail(403, "POST", "/api/meetings", M{"title": "AGM", "starts_at": 1000, "ends_at": 2000})
	admin.fail(400, "DELETE", "/api/roles/"+RoleAdmin, nil)

	// Rename, disable, password reset.
	admin.ok("PATCH", "/api/admin/users/"+member.ID, M{"username": "maria", "display_name": "Maria"}, nil)
	admin.fail(400, "PATCH", "/api/admin/users/"+member.ID, M{"username": "lena"})
	admin.ok("POST", "/api/admin/users/"+member.ID+"/password", M{"password": "another-temp-pass"}, nil)
	member.fail(401, "GET", "/api/me", nil) // signed out everywhere
	maria := e.login("maria", "another-temp-pass")
	maria.fail(403, "GET", "/api/lists", nil) // must change password first
	admin.ok("PATCH", "/api/admin/users/"+member.ID, M{"disabled": true}, nil)
	maria.fail(401, "GET", "/api/me", nil)

	// The last administrator cannot be removed or demoted.
	if c := admin.fail(400, "PATCH", "/api/admin/users/"+admin.ID, M{"roles": []string{RoleMember}}); c != "last_admin" {
		t.Fatalf("got %q", c)
	}
	admin.fail(400, "DELETE", "/api/admin/users/"+admin.ID, nil)

	// Audit log records the administrative actions, with source address.
	var logs []M
	admin.ok("GET", "/api/admin/logs", nil, &logs)
	seen := map[string]bool{}
	for _, l := range logs {
		seen[l["action"].(string)] = true
	}
	for _, want := range []string{"login", "user.updated", "user.password_reset", "role.created", "role.deleted"} {
		if !seen[want] {
			t.Errorf("audit log lacks %s", want)
		}
	}
}

func TestUserDeletionAnonymises(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	olle := e.user("olle")
	olle.ok("PATCH", "/api/me", M{"phone": "+46 70 123 45 67"}, nil)
	private := olle.id("POST", "/api/lists", M{"title": "My private list", "kind": "todo"})
	shared := olle.id("POST", "/api/lists", M{"title": "Shared list", "kind": "todo", "shares": all()})
	var chans []M
	olle.ok("GET", "/api/channels", nil, &chans)
	general := chans[0]["id"].(string)
	olle.ok("POST", "/api/channels/"+general+"/messages", M{"body": "hello from olle"}, nil)

	admin.ok("DELETE", "/api/admin/users/"+olle.ID, nil, nil)
	olle.fail(401, "GET", "/api/me", nil)
	(&client{e: e}).fail(401, "POST", "/api/auth/login", M{"username": "olle", "password": testPW})

	ctx := context.Background()
	var username, email, phone, hash string
	e.a.DB.Row(ctx, `SELECT username, email_enc, phone_enc, password_hash FROM users WHERE id=?`, olle.ID).Scan(&username, &email, &phone, &hash)
	if strings.Contains(username, "olle") || email != "" || phone != "" || hash != "" {
		t.Fatalf("personal data remains: %q %q %q", username, email, phone)
	}
	admin.fail(404, "GET", "/api/lists/"+private, nil)
	var n int
	e.a.DB.Row(ctx, `SELECT COUNT(*) FROM lists WHERE id=?`, private).Scan(&n)
	if n != 0 {
		t.Fatal("private content of a deleted user must be removed")
	}
	admin.ok("GET", "/api/lists/"+shared, nil, nil) // shared content stays
	var msgs []M
	admin.ok("GET", "/api/channels/"+general+"/messages", nil, &msgs)
	if len(msgs) != 1 {
		t.Fatal("shared messages stay (attributed to a deleted user)")
	}
	var people []M
	admin.ok("GET", "/api/users", nil, &people)
	if ids(people)[olle.ID] {
		t.Fatal("deleted user still in directory")
	}
}

func TestSharingModel(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	leader := e.user("lena", RoleLeader)
	a, b, c := e.user("anna"), e.user("bert"), e.user("cia")

	visibleTo := func(cl *client, id string) bool {
		var lists []M
		cl.ok("GET", "/api/lists", nil, &lists)
		return ids(lists)[id]
	}
	// Private: only the owner — not even administrators or moderators.
	priv := a.id("POST", "/api/lists", M{"title": "private", "kind": "todo"})
	for _, other := range []*client{b, admin, leader} {
		if visibleTo(other, priv) {
			t.Fatal("private list leaked")
		}
		other.fail(404, "GET", "/api/lists/"+priv, nil)
		other.fail(404, "POST", "/api/lists/"+priv+"/items", M{"text": "x"})
		other.fail(404, "GET", "/api/acl/list/"+priv, nil)
		other.fail(404, "PUT", "/api/acl/list/"+priv, all())
	}
	// Shared with one user, read only.
	a.ok("PUT", "/api/acl/list/"+priv, []M{share("user", b.ID, 1)}, nil)
	b.ok("GET", "/api/lists/"+priv, nil, nil)
	b.fail(403, "POST", "/api/lists/"+priv+"/items", M{"text": "x"})
	b.fail(403, "PUT", "/api/acl/list/"+priv, all())
	b.fail(403, "DELETE", "/api/lists/"+priv, nil)
	c.fail(404, "GET", "/api/lists/"+priv, nil)
	// Upgrade to write.
	a.ok("PUT", "/api/acl/list/"+priv, []M{share("user", b.ID, 2)}, nil)
	b.ok("POST", "/api/lists/"+priv+"/items", M{"text": "from bert"}, nil)
	b.fail(403, "DELETE", "/api/lists/"+priv, nil)
	// Once shared, a moderator (leader) may manage it; a plain member not.
	leader.ok("GET", "/api/lists/"+priv, nil, nil)

	// Global group (admin) and role based sharing.
	gid := admin.id("POST", "/api/groups", M{"name": "Youth section", "scope": "global", "members": []string{c.ID}})
	byGroup := a.id("POST", "/api/lists", M{"title": "group list", "kind": "plain", "shares": []M{share("group", gid, 1)}})
	byRole := a.id("POST", "/api/lists", M{"title": "leaders only", "kind": "plain", "shares": []M{share("role", RoleLeader, 2)}})
	if !visibleTo(c, byGroup) || visibleTo(b, byGroup) {
		t.Fatal("group sharing broken")
	}
	if !visibleTo(leader, byRole) || visibleTo(b, byRole) || visibleTo(c, byRole) {
		t.Fatal("role sharing broken")
	}
	admin.ok("PUT", "/api/groups/"+gid, M{"name": "Youth section", "members": []string{b.ID}}, nil)
	if visibleTo(c, byGroup) || !visibleTo(b, byGroup) {
		t.Fatal("group membership change not effective")
	}
	b.fail(403, "PUT", "/api/groups/"+gid, M{"name": "hijack", "members": []string{b.ID}})

	// Local groups: only visible to and usable by their owner.
	lg := a.id("POST", "/api/groups", M{"name": "My friends", "scope": "local", "members": []string{c.ID}})
	var groups []M
	b.ok("GET", "/api/groups", nil, &groups)
	if ids(groups)[lg] {
		t.Fatal("local group visible to another user")
	}
	b.fail(404, "PUT", "/api/groups/"+lg, M{"name": "x"})
	b.fail(400, "POST", "/api/lists", M{"title": "x", "kind": "todo", "shares": []M{share("group", lg, 1)}})
	byLocal := a.id("POST", "/api/lists", M{"title": "friends list", "kind": "todo", "shares": []M{share("group", lg, 2)}})
	if !visibleTo(c, byLocal) || visibleTo(b, byLocal) {
		t.Fatal("local group sharing broken")
	}
	// A member with read access sees the shares, but not the local group's name.
	var shares []Share
	c.ok("GET", "/api/acl/list/"+byLocal, nil, &shares)
	if len(shares) != 1 || shares[0].Name != "" || shares[0].ID != "" {
		t.Fatalf("local group identity leaked: %+v", shares)
	}
	// Invalid shares are rejected.
	a.fail(400, "PUT", "/api/acl/list/"+priv, []M{share("user", "nope", 1)})
	a.fail(400, "PUT", "/api/acl/list/"+priv, []M{share("all", "", 3)})
	a.fail(404, "PUT", "/api/acl/users/"+a.ID, all())
}

func TestChat(t *testing.T) {
	e := newEnv(t)
	leader := e.user("lena", RoleLeader)
	a, b, c := e.user("anna"), e.user("bert"), e.user("cia")

	var chans []Channel
	a.ok("GET", "/api/channels", nil, &chans)
	if len(chans) != 1 || chans[0].Name != "general" {
		t.Fatalf("default channel missing: %+v", chans)
	}
	general := chans[0].ID
	// Announcement channel: everyone reads, leaders write.
	news := leader.id("POST", "/api/channels", M{"name": "news", "topic": "Announcements", "shares": []M{share("all", "", 1), share("role", RoleLeader, 2)}})
	a.fail(403, "POST", "/api/channels/"+news+"/messages", M{"body": "can I post?"})
	mid := leader.id("POST", "/api/channels/"+news+"/messages", M{"body": "Welcome! See https://example.org"})
	a.fail(403, "POST", "/api/messages/"+mid+"/pin", nil) // read-only members cannot pin
	leader.ok("POST", "/api/messages/"+mid+"/pin", nil, nil)
	var pins []Message
	a.ok("GET", "/api/channels/"+news+"/pins", nil, &pins)
	if len(pins) != 1 || pins[0].ID != mid || pins[0].PinnedBy != leader.ID {
		t.Fatalf("pins: %+v", pins)
	}
	a.ok("GET", "/api/channels", nil, &chans)
	for _, ch := range chans {
		if ch.ID == news && ch.Unread != 1 {
			t.Fatalf("unread = %d", ch.Unread)
		}
	}
	a.ok("POST", "/api/channels/"+news+"/read", nil, nil)
	leader.ok("DELETE", "/api/messages/"+mid+"/pin", nil, nil)
	a.ok("GET", "/api/channels/"+news+"/pins", nil, &pins)
	if len(pins) != 0 {
		t.Fatal("unpin failed")
	}

	// Edit / delete rules.
	m1 := a.id("POST", "/api/channels/"+general+"/messages", M{"body": "first"})
	b.fail(403, "PATCH", "/api/messages/"+m1, M{"body": "hacked"})
	b.fail(403, "DELETE", "/api/messages/"+m1, nil)
	a.ok("PATCH", "/api/messages/"+m1, M{"body": "first (edited)"}, nil)
	leader.ok("DELETE", "/api/messages/"+m1, nil, nil) // moderators may remove
	var msgs []Message
	a.ok("GET", "/api/channels/"+general+"/messages", nil, &msgs)
	if len(msgs) != 1 || !msgs[0].Deleted || msgs[0].Body != "" {
		t.Fatalf("deleted message: %+v", msgs)
	}
	a.fail(400, "POST", "/api/channels/"+general+"/messages", M{"body": "  "})

	// Direct messages with an attachment: only the participants can read.
	dm := a.id("POST", "/api/dm", M{"user_ids": []string{b.ID}})
	if again := b.id("POST", "/api/dm", M{"user_ids": []string{a.ID}}); again != dm {
		t.Fatal("the same DM conversation must be reused")
	}
	content := []byte("%PDF-1.4 budget 2026 – confidential")
	f := a.upload("Budget 2026.pdf", content)
	b.fail(400, "POST", "/api/channels/"+dm+"/messages", M{"body": "steal", "file_ids": []string{f.ID}}) // not b's file
	b.fail(404, "GET", "/api/files/"+f.ID, nil)                                                          // not attached anywhere yet
	a.ok("POST", "/api/channels/"+dm+"/messages", M{"body": "the budget", "file_ids": []string{f.ID}}, nil)
	st, got, h := b.raw("GET", "/api/files/"+f.ID, nil, "")
	if st != 200 || !bytes.Equal(got, content) {
		t.Fatalf("download: %d", st)
	}
	if !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("downloads must be attachments with a sandbox CSP")
	}
	for _, outsider := range []*client{c, leader} { // moderators have no access to DMs
		outsider.fail(404, "GET", "/api/files/"+f.ID, nil)
		outsider.fail(404, "GET", "/api/channels/"+dm+"/messages", nil)
		outsider.fail(404, "POST", "/api/channels/"+dm+"/messages", M{"body": "hi"})
		outsider.fail(404, "PUT", "/api/acl/channel/"+dm, all())
	}
	a.fail(403, "PUT", "/api/acl/channel/"+dm, all()) // DMs cannot be opened up
	a.fail(403, "DELETE", "/api/channels/"+dm, nil)

	// Files are encrypted on disk.
	found := false
	filepath.Walk(filepath.Join(e.dir, "blobs"), func(p string, fi os.FileInfo, _ error) error {
		if fi != nil && !fi.IsDir() {
			raw, _ := os.ReadFile(p)
			found = true
			if bytes.Contains(raw, []byte("budget 2026")) {
				t.Error("blob stored in plaintext")
			}
		}
		return nil
	})
	if !found {
		t.Fatal("no blob written")
	}
	// Upload limit (1 MiB in tests).
	var big bytes.Buffer
	big.Write(make([]byte, 2<<20))
	if st, _ := a.call("POST", "/api/files", nil); st == 200 {
		t.Fatal("upload without multipart accepted")
	}
	// Channel management.
	a.fail(403, "DELETE", "/api/channels/"+news, nil)
	leader.ok("PATCH", "/api/channels/"+news, M{"name": "announcements", "topic": ""}, nil)
	leader.ok("DELETE", "/api/channels/"+news, nil, nil)
}

func TestDocuments(t *testing.T) {
	e := newEnv(t)
	a, b := e.user("anna"), e.user("bert")
	v1 := a.upload("stadgar.docx", []byte("version one"))
	doc := a.id("POST", "/api/documents", M{"name": "Stadgar", "folder": "Styrelsen/../Styrelsen//2026", "description": "Föreningens stadgar",
		"file_id": v1.ID, "shares": []M{share("user", b.ID, 1)}})
	var d Document
	b.ok("GET", "/api/documents/"+doc, nil, &d)
	if d.Folder != "Styrelsen/Styrelsen/2026" || d.Version != 1 || d.Level != LevelRead {
		t.Fatalf("doc: %+v", d)
	}
	// Read-only users cannot add versions or edit.
	bv := b.upload("evil.docx", []byte("bert's version"))
	b.fail(403, "POST", "/api/documents/"+doc+"/versions", M{"file_id": bv.ID})
	b.fail(403, "PATCH", "/api/documents/"+doc, M{"name": "x"})
	// Nobody can claim someone else's upload as a version.
	a.fail(400, "POST", "/api/documents/"+doc+"/versions", M{"file_id": bv.ID})

	v2 := a.upload("stadgar-v2.docx", []byte("version two"))
	a.ok("POST", "/api/documents/"+doc+"/versions", M{"file_id": v2.ID, "comment": "Reviderad §3"}, nil)
	b.ok("GET", "/api/documents/"+doc, nil, &d)
	if d.Version != 2 || len(d.Versions) != 2 || d.Versions[0].Comment != "Reviderad §3" {
		t.Fatalf("versions: %+v", d)
	}
	_, cur, h := b.raw("GET", "/api/documents/"+doc+"/download", nil, "")
	_, old, _ := b.raw("GET", "/api/documents/"+doc+"/download?version=1", nil, "")
	if string(cur) != "version two" || string(old) != "version one" {
		t.Fatalf("download: %q %q", cur, old)
	}
	if !strings.Contains(h.Get("Content-Disposition"), "Stadgar.docx") {
		t.Fatalf("filename: %s", h.Get("Content-Disposition"))
	}
	// Copy: private to the copier, independent of the original.
	cp := b.id("POST", "/api/documents/"+doc+"/copy", M{"name": "My copy"})
	a.fail(404, "GET", "/api/documents/"+cp, nil)
	a.ok("DELETE", "/api/documents/"+doc, nil, nil)
	b.fail(404, "GET", "/api/documents/"+doc, nil)
	if _, got, _ := b.raw("GET", "/api/documents/"+cp+"/download", nil, ""); string(got) != "version two" {
		t.Fatalf("copy lost its content: %q", got)
	}
	// Dangerous file names are neutralised.
	evil := a.upload(`../../etc/pass"wd<script>.html`, []byte("<script>alert(1)</script>"))
	if strings.ContainsAny(evil.Name, `/\"<>`) {
		t.Fatalf("unsafe name %q", evil.Name)
	}
}

func TestListsAndBoards(t *testing.T) {
	e := newEnv(t)
	a, b := e.user("anna"), e.user("bert")
	list := a.id("POST", "/api/lists", M{"title": "Inför sommarlägret", "kind": "priority", "shares": []M{share("user", b.ID, 2)}})
	i1 := a.id("POST", "/api/lists/"+list+"/items", M{"text": "Boka buss", "priority": 3})
	i2 := b.id("POST", "/api/lists/"+list+"/items", M{"text": "Köpa mat"})
	i3 := a.id("POST", "/api/lists/"+list+"/items", M{"text": "Tält"})
	b.ok("PATCH", "/api/lists/"+list+"/items/"+i1, M{"done": true}, nil)
	a.ok("PATCH", "/api/lists/"+list+"/items/"+i3, M{"position": 0}, nil)
	var l List
	a.ok("GET", "/api/lists/"+list, nil, &l)
	if len(l.Items) != 3 || l.Items[0].ID != i3 || l.Items[1].ID != i1 || !l.Items[1].Done || l.Items[1].Priority != 3 || l.Items[2].ID != i2 {
		t.Fatalf("items: %+v", l.Items)
	}
	b.ok("DELETE", "/api/lists/"+list+"/items/"+i2, nil, nil)
	a.fail(400, "POST", "/api/lists", M{"title": "x", "kind": "nope"})

	board := a.id("POST", "/api/boards", M{"title": "Styrelsearbete", "columns": []string{"Att göra", "Pågår", "Klart"}, "shares": []M{share("user", b.ID, 2)}})
	var bd Board
	a.ok("GET", "/api/boards/"+board, nil, &bd)
	todo, doing := bd.Columns[0].ID, bd.Columns[1].ID
	k1 := a.id("POST", "/api/boards/"+board+"/cards", M{"column_id": todo, "title": "Ansök om bidrag", "assignee_id": b.ID})
	k2 := a.id("POST", "/api/boards/"+board+"/cards", M{"column_id": todo, "title": "Boka lokal"})
	b.ok("PATCH", "/api/boards/"+board+"/cards/"+k1, M{"column_id": doing, "position": 0}, nil)
	b.ok("GET", "/api/boards/"+board, nil, &bd)
	byID := map[string]Card{}
	for _, k := range bd.CardList {
		byID[k.ID] = k
	}
	if byID[k1].ColumnID != doing || byID[k2].ColumnID != todo {
		t.Fatalf("cards: %+v", bd.CardList)
	}
	b.fail(403, "POST", "/api/boards/"+board+"/columns", M{"title": "x"}) // structure is for managers
	a.fail(400, "DELETE", "/api/boards/"+board+"/columns/"+doing, nil)    // not empty
	a.fail(400, "PATCH", "/api/boards/"+board+"/cards/"+k1, M{"column_id": "nope"})
	other := e.user("cia")
	other.fail(404, "GET", "/api/boards/"+board, nil)
	other.fail(404, "PATCH", "/api/boards/"+board+"/cards/"+k1, M{"title": "x"})
}

func TestPolls(t *testing.T) {
	e := newEnv(t)
	a, b, c := e.user("anna"), e.user("bert"), e.user("cia")
	vote := func(cl *client, poll string, choices M) {
		cl.e.t.Helper()
		cl.ok("POST", "/api/polls/"+poll+"/vote", M{"choices": choices}, nil)
	}
	get := func(cl *client, poll string) Poll {
		var p Poll
		cl.ok("GET", "/api/polls/"+poll, nil, &p)
		return p
	}
	// Single choice, results after voting, vote can be changed.
	single := a.id("POST", "/api/polls", M{"title": "Färg på tröjan?", "kind": "single", "options": []string{"Röd", "Blå"}, "allow_change": true, "shares": all()})
	p := get(b, single)
	if p.ResultsShown || p.Options[0].Votes != 0 {
		t.Fatal("results must be hidden before voting")
	}
	red, blue := p.Options[0].ID, p.Options[1].ID
	b.fail(400, "POST", "/api/polls/"+single+"/vote", M{"choices": M{red: 1, blue: 1}})
	b.fail(400, "POST", "/api/polls/"+single+"/vote", M{"choices": M{"nope": 1}})
	vote(b, single, M{red: 1})
	vote(c, single, M{red: 1})
	vote(b, single, M{blue: 1}) // change
	p = get(b, single)
	if !p.ResultsShown || p.Options[0].Votes != 1 || p.Options[1].Votes != 1 || p.Voters != 2 || p.MyVote[blue] != 1 {
		t.Fatalf("single: %+v", p)
	}
	b.fail(403, "POST", "/api/polls/"+single+"/close", nil)
	a.ok("POST", "/api/polls/"+single+"/close", nil, nil)
	c.fail(400, "POST", "/api/polls/"+single+"/vote", M{"choices": M{blue: 1}})

	// Anonymous yes/no: no link between voter and ballot, no changes.
	yn := a.id("POST", "/api/polls", M{"title": "Höja avgiften?", "kind": "yesno", "anonymous": true, "allow_change": true, "results_visible": "after_close", "shares": all()})
	p = get(b, yn)
	if len(p.Options) != 3 || p.Options[0].Text != "@yes" || p.AllowChange {
		t.Fatalf("yesno: %+v", p)
	}
	vote(b, yn, M{p.Options[0].ID: 1})
	vote(c, yn, M{p.Options[1].ID: 1})
	if code := b.fail(400, "POST", "/api/polls/"+yn+"/vote", M{"choices": M{p.Options[1].ID: 1}}); code != "already_voted" {
		t.Fatalf("got %q", code)
	}
	var linked int
	e.a.DB.Row(context.Background(), `SELECT COUNT(*) FROM ballots WHERE poll_id=? AND (user_id<>'' OR created_at<>0)`, yn).Scan(&linked)
	if linked != 0 {
		t.Fatal("anonymous ballots must not reference the voter or the time")
	}
	if p = get(b, yn); p.ResultsShown || p.MyVote != nil || !p.Voted {
		t.Fatalf("after_close results leaked: %+v", p)
	}
	a.ok("POST", "/api/polls/"+yn+"/close", nil, nil)
	if p = get(b, yn); !p.ResultsShown || p.Options[0].Votes != 1 || p.Options[1].Votes != 1 {
		t.Fatalf("closed: %+v", p)
	}

	// Multiple choice with a maximum.
	multi := a.id("POST", "/api/polls", M{"title": "Aktiviteter", "kind": "multi", "options": []string{"A", "B", "C"}, "max_choices": 2, "results_visible": "always", "shares": all()})
	p = get(b, multi)
	b.fail(400, "POST", "/api/polls/"+multi+"/vote", M{"choices": M{p.Options[0].ID: 1, p.Options[1].ID: 1, p.Options[2].ID: 1}})
	vote(b, multi, M{p.Options[0].ID: 1, p.Options[2].ID: 1})

	// Ranked choice (Borda) and score voting.
	ranked := a.id("POST", "/api/polls", M{"title": "Resmål", "kind": "ranked", "options": []string{"Fjäll", "Hav", "Stad"}, "results_visible": "always", "shares": all()})
	p = get(a, ranked)
	o := p.Options
	b.fail(400, "POST", "/api/polls/"+ranked+"/vote", M{"choices": M{o[0].ID: 1, o[1].ID: 1, o[2].ID: 2}}) // not a permutation
	vote(a, ranked, M{o[0].ID: 1, o[1].ID: 2, o[2].ID: 3})
	vote(b, ranked, M{o[0].ID: 2, o[1].ID: 1, o[2].ID: 3})
	vote(c, ranked, M{o[0].ID: 1, o[1].ID: 3, o[2].ID: 2})
	p = get(a, ranked)
	if p.Options[0].Score != 5 || p.Options[1].Score != 3 || p.Options[2].Score != 1 {
		t.Fatalf("borda: %+v", p.Options)
	}
	score := a.id("POST", "/api/polls", M{"title": "Betyg", "kind": "score", "options": []string{"X", "Y"}, "results_visible": "always", "shares": all()})
	p = get(a, score)
	a.fail(400, "POST", "/api/polls/"+score+"/vote", M{"choices": M{p.Options[0].ID: 6, p.Options[1].ID: 0}})
	vote(a, score, M{p.Options[0].ID: 5, p.Options[1].ID: 2})
	vote(b, score, M{p.Options[0].ID: 3, p.Options[1].ID: 0})
	if p = get(a, score); p.Options[0].Score != 8 || p.Options[1].Score != 2 {
		t.Fatalf("score: %+v", p.Options)
	}

	// Deadline: a poll past closes_at is closed.
	timed := a.id("POST", "/api/polls", M{"title": "Snabb", "kind": "single", "options": []string{"1", "2"}, "closes_at": time.Now().Unix() + 3600, "shares": all()})
	e.a.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if p = get(b, timed); !p.Closed {
		t.Fatal("deadline not enforced")
	}
	b.fail(400, "POST", "/api/polls/"+timed+"/vote", M{"choices": M{p.Options[0].ID: 1}})
	e.a.now = time.Now

	// Private poll is invisible to others.
	priv := a.id("POST", "/api/polls", M{"title": "Hemlig", "kind": "single", "options": []string{"1", "2"}})
	b.fail(404, "GET", "/api/polls/"+priv, nil)
	b.fail(404, "POST", "/api/polls/"+priv+"/vote", M{"choices": M{}})
}

func TestCalendarAndICS(t *testing.T) {
	e := newEnv(t)
	a, b := e.user("anna"), e.user("bert")
	start := time.Date(2026, 6, 1, 17, 0, 0, 0, time.UTC).Unix()
	priv := a.id("POST", "/api/calendar", M{"title": "Tandläkare", "starts_at": start, "ends_at": start + 3600})
	shared := a.id("POST", "/api/calendar", M{"title": "Sommarfest; med grill, och bad", "description": "Ta med:\nbadkläder", "location": "Klubbstugan",
		"url": "https://teams.microsoft.com/l/meetup-join/abc", "starts_at": start + 86400, "ends_at": start + 90000, "shares": all()})
	day := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC).Unix()
	a.ok("POST", "/api/calendar", M{"title": "Nationaldagen", "starts_at": day, "ends_at": day, "all_day": true, "shares": all()}, nil)
	a.fail(400, "POST", "/api/calendar", M{"title": "bad link", "url": "javascript:alert(1)", "starts_at": start, "ends_at": start})
	a.fail(400, "POST", "/api/calendar", M{"title": "bad time", "starts_at": start, "ends_at": start - 1})

	var evs []CalEvent
	b.ok("GET", "/api/calendar?from=0&to=9999999999", nil, &evs)
	if len(evs) != 2 {
		t.Fatalf("bert sees %d events", len(evs))
	}
	b.fail(404, "PATCH", "/api/calendar/"+priv, M{"title": "x", "starts_at": 1, "ends_at": 2})
	b.fail(403, "PATCH", "/api/calendar/"+shared, M{"title": "x", "starts_at": 1, "ends_at": 2})
	// Time window filter.
	b.ok("GET", "/api/calendar?from="+itoa(int(day))+"&to="+itoa(int(day+86399)), nil, &evs)
	if len(evs) != 1 || evs[0].Title != "Nationaldagen" {
		t.Fatalf("window: %+v", evs)
	}

	// Export what Bert can see and import it as Cia.
	st, ics, h := b.raw("GET", "/api/calendar/export.ics", nil, "")
	if st != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/calendar") {
		t.Fatalf("export: %d", st)
	}
	text := string(ics)
	if strings.Contains(text, "Tandläkare") {
		t.Fatal("private event exported to another user")
	}
	for _, want := range []string{"BEGIN:VCALENDAR", `SUMMARY:Sommarfest\; med grill\, och bad`, `DESCRIPTION:Ta med:\nbadkläder`, "DTSTART;VALUE=DATE:20260606", "DTEND;VALUE=DATE:20260607"} {
		if !strings.Contains(text, want) {
			t.Errorf("ics lacks %q\n%s", want, text)
		}
	}
	for _, line := range strings.Split(text, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line not folded: %q", line)
		}
	}
	c := e.user("cia")
	var res struct{ Imported int }
	st, body, _ := c.raw("POST", "/api/calendar/import", bytes.NewReader(ics), "text/calendar")
	json.Unmarshal(body, &res)
	if st != 200 || res.Imported != 2 {
		t.Fatalf("import: %d %s", st, body)
	}
	c.raw("POST", "/api/calendar/import", bytes.NewReader(ics), "text/calendar") // idempotent (same UIDs)
	var own []CalEvent
	c.ok("GET", "/api/calendar", nil, &own)
	mine := 0
	for _, ev := range own {
		if ev.OwnerID == c.ID {
			mine++
			if ev.Title == "Sommarfest; med grill, och bad" && (ev.Description != "Ta med:\nbadkläder" || ev.StartsAt != start+86400 || ev.Shared) {
				t.Errorf("round trip changed the event: %+v", ev)
			}
			if ev.Title == "Nationaldagen" && (!ev.AllDay || ev.StartsAt != day || ev.EndsAt != day) {
				t.Errorf("all-day round trip: %+v", ev)
			}
		}
	}
	if mine != 2 {
		t.Fatalf("imported events: %d", mine)
	}

	// A calendar file from another application, with time zone and an alarm.
	foreign := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:x1@other\r\nSUMMARY:Årsmöte\r\nDTSTART;TZID=Europe/Stockholm:20260310T180000\r\n" +
		"DTEND;TZID=Europe/Stockholm:20260310T200000\r\nLOCATION:Folkets\r\n  hus\r\nBEGIN:VALARM\r\nSUMMARY:Reminder\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	parsed, err := ParseICS(strings.NewReader(foreign))
	if err != nil || len(parsed) != 1 {
		t.Fatalf("parse: %v %+v", err, parsed)
	}
	want := time.Date(2026, 3, 10, 17, 0, 0, 0, time.UTC).Unix() // 18:00 CET
	if parsed[0].Title != "Årsmöte" || parsed[0].StartsAt != want || parsed[0].EndsAt != want+7200 || parsed[0].Location != "Folkets hus" {
		t.Fatalf("parsed: %+v", parsed[0])
	}
}

func TestMeetingsAndDecisions(t *testing.T) {
	e := newEnv(t)
	leader := e.user("lena", RoleLeader)
	a, outsider := e.user("anna"), e.user("olle")
	board := leader.id("POST", "/api/groups", M{"name": "local board", "scope": "local", "members": []string{a.ID}})
	start := time.Date(2026, 9, 1, 17, 0, 0, 0, time.UTC).Unix()
	mt := leader.id("POST", "/api/meetings", M{"title": "Styrelsemöte september", "location": "Klubbstugan", "url": "https://zoom.us/j/123",
		"starts_at": start, "ends_at": start + 7200, "agenda": []string{"Mötets öppnande", "Ekonomi", "Mötets avslutande"},
		"shares": []M{share("group", board, 1)}})
	// The meeting appears in the invited member's calendar, read-only.
	var evs []CalEvent
	a.ok("GET", "/api/calendar", nil, &evs)
	if len(evs) != 1 || evs[0].MeetingID != mt || evs[0].URL != "https://zoom.us/j/123" {
		t.Fatalf("calendar: %+v", evs)
	}
	leader.fail(400, "DELETE", "/api/calendar/"+evs[0].ID, nil)
	outsider.ok("GET", "/api/calendar", nil, &evs)
	if len(evs) != 0 {
		t.Fatal("meeting leaked into an outsider's calendar")
	}
	outsider.fail(404, "GET", "/api/meetings/"+mt, nil)

	var m Meeting
	a.ok("GET", "/api/meetings/"+mt, nil, &m)
	if len(m.Agenda) != 3 || m.Agenda[1].Title != "Ekonomi" {
		t.Fatalf("agenda: %+v", m.Agenda)
	}
	a.fail(403, "PUT", "/api/meetings/"+mt+"/agenda", m.Agenda) // read-only invitee
	// Reorder, add and annotate agenda items.
	ag := m.Agenda
	ag[1].Notes = "Kassören redovisade läget. Kassan: 12 345 kr."
	ag[1].Presenter = "Kassören"
	ag = append(ag[:2], AgendaItem{Title: "Sommarläger"}, ag[2])
	leader.ok("PUT", "/api/meetings/"+mt+"/agenda", ag, nil)
	leader.ok("PATCH", "/api/meetings/"+mt, M{"title": "Styrelsemöte september", "location": "Klubbstugan", "starts_at": start + 3600, "ends_at": start + 9000,
		"status": "in_progress", "attendees": "Lena\nAnna", "chair": "Lena", "secretary": "Anna"}, nil)
	a.ok("GET", "/api/calendar", nil, &evs)
	if evs[0].StartsAt != start+3600 {
		t.Fatal("calendar entry must follow the meeting")
	}
	leader.ok("GET", "/api/meetings/"+mt, nil, &m)
	if len(m.Agenda) != 4 || m.Agenda[2].Title != "Sommarläger" || m.Agenda[1].ID != ag[1].ID {
		t.Fatalf("agenda after edit: %+v", m.Agenda)
	}
	// A decision taken under an agenda item goes into the decision log.
	var dec struct{ ID, Number string }
	leader.ok("POST", "/api/decisions", M{"title": "Budget för sommarläger", "body": "Styrelsen beslutade avsätta 20 000 kr.", "decided_at": start,
		"meeting_id": mt, "agenda_item_id": m.Agenda[2].ID, "shares": all()}, &dec)
	if dec.Number != "2026-001" {
		t.Fatalf("number: %s", dec.Number)
	}
	var dec2 struct{ ID, Number string }
	leader.ok("POST", "/api/decisions", M{"title": "Intern fråga", "decided_at": start, "shares": []M{share("role", RoleLeader, 1)}}, &dec2)
	if dec2.Number != "2026-002" {
		t.Fatalf("number: %s", dec2.Number)
	}
	// Audience: everyone sees the first decision, only leaders the second.
	var ds []Decision
	outsider.ok("GET", "/api/decisions", nil, &ds)
	if len(ds) != 1 || ds[0].ID != dec.ID {
		t.Fatalf("decision audience: %+v", ds)
	}
	outsider.fail(404, "GET", "/api/decisions/"+dec2.ID, nil)

	// Protocol export contains notes and the decision.
	_, md, _ := a.raw("GET", "/api/meetings/"+mt+"/protocol.md", nil, "")
	for _, want := range []string{"# Protokoll: Styrelsemöte september", "§ 2 Ekonomi", "Kassan: 12 345 kr", "**Beslut 2026-001:** Budget för sommarläger", "Utkast"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("protocol lacks %q:\n%s", want, md)
		}
	}
	// Finalising locks the protocol.
	a.fail(403, "POST", "/api/meetings/"+mt+"/finalize", nil)
	leader.ok("POST", "/api/meetings/"+mt+"/finalize", nil, nil)
	if code := leader.fail(400, "PUT", "/api/meetings/"+mt+"/agenda", m.Agenda); code != "meeting_finalized" {
		t.Fatalf("got %q", code)
	}
	leader.fail(400, "PATCH", "/api/meetings/"+mt, M{"title": "x", "starts_at": 1, "ends_at": 2})
	leader.fail(400, "POST", "/api/decisions", M{"title": "late", "meeting_id": mt})

	// Decisions are revised, never overwritten: history is kept.
	outsider.fail(403, "PATCH", "/api/decisions/"+dec.ID, M{"title": "x", "status": "revoked", "comment": "x"})
	leader.fail(400, "PATCH", "/api/decisions/"+dec.ID, M{"title": "Budget", "status": "revoked"}) // comment required
	leader.ok("PATCH", "/api/decisions/"+dec.ID, M{"title": "Budget för sommarläger", "body": "Upphävt.", "status": "revoked", "comment": "Lägret ställdes in"}, nil)
	var d Decision
	outsider.ok("GET", "/api/decisions/"+dec.ID, nil, &d)
	if d.Status != "revoked" || len(d.Revisions) != 2 || d.Revisions[0].Comment != "Lägret ställdes in" || d.Revisions[1].Status != "decided" {
		t.Fatalf("revisions: %+v", d)
	}
	_, csvOut, _ := outsider.raw("GET", "/api/decisions/export.csv", nil, "")
	if !strings.Contains(string(csvOut), "2026-001") || strings.Contains(string(csvOut), "Intern fråga") {
		t.Fatalf("csv export: %s", csvOut)
	}
	// Personal history shows the leader's own actions only.
	var hist []M
	leader.ok("GET", "/api/me/history", nil, &hist)
	acts := map[string]bool{}
	for _, h := range hist {
		acts[h["action"].(string)] = true
	}
	if !acts["meeting.created"] || !acts["decision.created"] || !acts["meeting.finalized"] {
		t.Fatalf("history: %v", acts)
	}
	outsider.ok("GET", "/api/me/history", nil, &hist)
	for _, h := range hist {
		if h["action"] == "meeting.created" {
			t.Fatal("history of another user leaked")
		}
	}
}

func TestSearch(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	a, b := e.user("anna"), e.user("bert")
	var chans []Channel
	a.ok("GET", "/api/channels", nil, &chans)
	general := chans[0].ID
	a.ok("POST", "/api/channels/"+general+"/messages", M{"body": "Glöm inte städdagen på lördag!"}, nil)
	dm := a.id("POST", "/api/dm", M{"user_ids": []string{admin.ID}})
	a.ok("POST", "/api/channels/"+dm+"/messages", M{"body": "hemlig städdag-planering"}, nil)
	a.ok("POST", "/api/lists", M{"title": "Städdag checklista", "kind": "todo"}, nil) // private
	shared := a.id("POST", "/api/lists", M{"title": "Inköp", "kind": "todo", "shares": all()})
	a.ok("POST", "/api/lists/"+shared+"/items", M{"text": "Sopsäckar till städdagen"}, nil)
	f := a.upload("x.pdf", []byte("x"))
	a.ok("POST", "/api/documents", M{"name": "Protokoll 2026-03", "file_id": f.ID, "shares": all()}, nil)
	a.ok("POST", "/api/documents", M{"name": "Protokoll 2025-11", "file_id": f.ID, "shares": all()}, nil)
	a.ok("PATCH", "/api/me", M{"phone": "070-111 22 33", "contact_visible": false}, nil)

	search := func(cl *client, query string) []SearchHit {
		t.Helper()
		var hits []SearchHit
		cl.ok("GET", "/api/search?"+query, nil, &hits)
		return hits
	}
	types := func(hits []SearchHit) map[string]int {
		m := map[string]int{}
		for _, h := range hits {
			m[h.Type]++
		}
		return m
	}
	// Bert finds only what he may read: the public message and the shared list item.
	got := types(search(b, "q=st%C3%A4ddag"))
	if got["message"] != 1 || got["list"] != 1 || len(got) != 2 {
		t.Fatalf("bert: %v", got)
	}
	// Anna additionally finds her DM and her private list. Case-insensitive by default.
	got = types(search(a, "q=ST%C3%84DDAG"))
	if got["message"] != 2 || got["list"] != 2 {
		t.Fatalf("anna: %v", got)
	}
	if len(search(a, "q=ST%C3%84DDAG&case=1")) != 0 {
		t.Fatal("case sensitive search matched")
	}
	// Wildcards and regular expressions.
	if n := len(search(b, "mode=wildcard&q=Protokoll+2026-*")); n != 1 {
		t.Fatalf("wildcard: %d", n)
	}
	if n := len(search(b, "mode=wildcard&q=Protokoll+202?-*")); n != 2 {
		t.Fatalf("wildcard ?: %d", n)
	}
	if n := len(search(b, "mode=regex&q=Protokoll+20(25|26)-%5Cd%7B2%7D%24")); n != 2 {
		t.Fatalf("regex: %d", n)
	}
	if n := len(search(b, "mode=plain&q=202?-*")); n != 0 {
		t.Fatal("plain mode must not interpret wildcards")
	}
	b.fail(400, "GET", "/api/search?mode=regex&q=(", nil)
	b.fail(400, "GET", "/api/search?q=", nil)
	// Type and time filters.
	if got = types(search(a, "q=st%C3%A4ddag&types=list")); got["list"] != 2 || len(got) != 1 {
		t.Fatalf("type filter: %v", got)
	}
	future := itoa(int(time.Now().Unix() + 3600))
	if n := len(search(a, "q=st%C3%A4ddag&from="+future)); n != 0 {
		t.Fatalf("time filter: %d", n)
	}
	if n := len(search(a, "q=st%C3%A4ddag&types=message&to="+future)); n != 2 {
		t.Fatalf("time filter: %d", n)
	}
	// People: by name for everyone; hidden contact details only for account managers.
	if got = types(search(b, "q=anna&types=user")); got["user"] != 1 {
		t.Fatalf("user by name: %v", got)
	}
	if n := len(search(b, "q=070-111")); n != 0 {
		t.Fatal("hidden phone number found by a member")
	}
	if n := len(search(admin, "q=070-111")); n != 1 {
		t.Fatal("account manager must find contact details")
	}
	if n := len(search(b, "q=bert@example.org&types=user")); n != 1 {
		t.Fatal("visible e-mail must be searchable")
	}
}

func TestBackupRestore(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	a := e.user("anna")
	a.ok("PATCH", "/api/me", M{"phone": "070-999"}, nil)
	content := bytes.Repeat([]byte("årsredovisning "), 10000)
	f := a.upload("Årsredovisning.pdf", content)
	doc := a.id("POST", "/api/documents", M{"name": "Årsredovisning", "file_id": f.ID, "shares": all()})
	a.ok("POST", "/api/lists", M{"title": "Min lista", "kind": "todo"}, nil)

	admin.fail(400, "POST", "/api/admin/backup", M{"passphrase": "short"})
	st, bak, _ := admin.raw("POST", "/api/admin/backup", strings.NewReader(`{"passphrase":"a very long passphrase"}`), "application/json")
	if st != 200 || len(bak) < 100 {
		t.Fatalf("backup: %d", st)
	}
	for _, leak := range []string{"anna", "Min lista", "redovisning", "070-999"} {
		if bytes.Contains(bak, []byte(leak)) {
			t.Fatalf("backup is not encrypted (found %q)", leak)
		}
	}
	// Restore into a fresh installation with a different master key.
	dir := t.TempDir()
	target := func(m Manifest) (*db.DB, *blob.Store, error) {
		d := openTestDB(t, dir)
		fsb, _ := blob.NewFS(filepath.Join(dir, "blobs"))
		key := e.a.Keys.Master() // the restore command installs the key from the manifest
		keys, _ := crypto.NewKeys(key)
		return d, blob.NewStore(fsb, keys), nil
	}
	ctx := context.Background()
	if _, _, err := Restore(ctx, bytes.NewReader(bak), "the wrong passphrase", target); err == nil {
		t.Fatal("wrong passphrase accepted")
	}
	tampered := bytes.Clone(bak)
	tampered[len(tampered)/2] ^= 1
	if _, _, err := Restore(ctx, bytes.NewReader(tampered), "a very long passphrase", target); err == nil {
		t.Fatal("tampered backup accepted")
	}
	dir = t.TempDir() // the failed attempt may have created an (empty) database
	var d2 *db.DB
	var s2 *blob.Store
	rows, blobs, err := Restore(ctx, bytes.NewReader(bak), "a very long passphrase", func(m Manifest) (*db.DB, *blob.Store, error) {
		var err error
		d2, s2, err = target(m)
		return d2, s2, err
	})
	if err != nil || rows == 0 || blobs != 1 {
		t.Fatalf("restore: rows=%d blobs=%d err=%v", rows, blobs, err)
	}
	// The restored system has the same accounts, PII and file contents.
	var phoneEnc, key string
	if err := d2.Row(ctx, `SELECT phone_enc FROM users WHERE username='anna'`).Scan(&phoneEnc); err != nil {
		t.Fatal(err)
	}
	if phone, _ := e.a.Keys.DecryptField(phoneEnc, "users.phone", a.ID); phone != "070-999" {
		t.Fatalf("phone: %q", phone)
	}
	d2.Row(ctx, `SELECT f.blob_key FROM doc_versions v JOIN files f ON f.id=v.file_id WHERE v.doc_id=?`, doc).Scan(&key)
	rc, err := s2.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	copyClose(&got, rc)
	if !bytes.Equal(got.Bytes(), content) {
		t.Fatal("restored file differs")
	}
	for _, table := range db.Tables {
		var n1, n2 int
		e.a.DB.Row(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n1)
		d2.Row(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n2)
		if n1 != n2 && table != "activity" { // the backup itself is logged afterwards
			t.Errorf("%s: %d rows restored, want %d", table, n2, n1)
		}
	}
	// Restoring over existing data is refused.
	if _, _, err := Restore(ctx, bytes.NewReader(bak), "a very long passphrase", func(Manifest) (*db.DB, *blob.Store, error) { return d2, s2, nil }); err == nil {
		t.Fatal("restore into a non-empty database must fail")
	}

	// Encrypted member export and personal data export.
	st, enc, _ := admin.raw("POST", "/api/admin/export", strings.NewReader(`{"kind":"members","passphrase":"a very long passphrase"}`), "application/json")
	if st != 200 || bytes.Contains(enc, []byte("anna")) {
		t.Fatalf("export: %d", st)
	}
	r, err := crypto.NewPassphraseReader(bytes.NewReader(enc), "a very long passphrase")
	if err != nil {
		t.Fatal(err)
	}
	var plain bytes.Buffer
	plain.ReadFrom(r)
	if !strings.Contains(plain.String(), "anna,Anna,anna@example.org,070-999,member") {
		t.Fatalf("members csv: %s", plain.String())
	}
	var mine M
	a.ok("GET", "/api/me/export", nil, &mine)
	if mine["profile"].(M)["phone"] != "070-999" || len(mine["owned"].(M)["list"].([]any)) != 1 {
		t.Fatalf("personal export: %v", mine)
	}
}

func TestPIIEncryptedAtRest(t *testing.T) {
	e := newEnv(t)
	a := e.user("anna")
	a.ok("PATCH", "/api/me", M{"phone": "+46 70 555 01 23"}, nil)
	var dump strings.Builder
	e.a.DB.Each(context.Background(), `SELECT email_enc, phone_enc, password_hash FROM users`, nil, func(r *sql.Rows) error {
		var x, y, z string
		r.Scan(&x, &y, &z)
		dump.WriteString(x + y + z)
		return nil
	})
	for _, leak := range []string{"example.org", "555 01 23", testPW} {
		if strings.Contains(dump.String(), leak) {
			t.Fatalf("%q stored in clear", leak)
		}
	}
	// Session tokens are stored hashed.
	var n int
	e.a.DB.Row(context.Background(), `SELECT COUNT(*) FROM sessions WHERE id_hash=?`, a.cookie.Value).Scan(&n)
	if n != 0 {
		t.Fatal("session token stored in clear")
	}
	// Contact visibility in the directory.
	b := e.user("bert")
	a.ok("PATCH", "/api/me", M{"contact_visible": false}, nil)
	var people []Person
	b.ok("GET", "/api/users", nil, &people)
	for _, p := range people {
		if p.Username == "anna" && (p.Email != "" || p.Phone != "") {
			t.Fatal("hidden contact details shown in the directory")
		}
	}
}
