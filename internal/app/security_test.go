package app

// Regression tests for the findings of the security audit. Each test states
// the weakness it guards against.

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rom/samling/internal/crypto"
)

// A folded calendar file used to take quadratic time to parse.
func TestSecICSImportIsLinear(t *testing.T) {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nSUMMARY:x\r\n")
	for b.Len() < 60<<10 { // one long, legitimately folded line (below the line limit)
		b.WriteString(" a\n")
	}
	b.WriteString("DTSTART:20300101T100000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	big := strings.Repeat(b.String(), 8) // ~0.5 MiB of folded lines
	t0 := time.Now()
	evs, err := ParseICS(strings.NewReader(big))
	if err != nil || len(evs) != 8 {
		t.Fatalf("parse: %v, %d events", err, len(evs))
	}
	if d := time.Since(t0); d > 2*time.Second {
		t.Fatalf("parsing 0.5 MiB of folded lines took %v", d)
	}
	// A single logical line beyond the limit is rejected instead of being built.
	var huge strings.Builder
	huge.WriteString("BEGIN:VCALENDAR\nBEGIN:VEVENT\nSUMMARY:x\n")
	for huge.Len() < 512<<10 {
		huge.WriteString(" a\n")
	}
	t0 = time.Now()
	if _, err := ParseICS(strings.NewReader(huge.String())); err == nil {
		t.Fatal("an over-long folded line must be rejected")
	}
	if d := time.Since(t0); d > time.Second {
		t.Fatalf("rejecting took %v", d)
	}
	e := newEnv(t)
	m := e.user("mallory")
	if st, _, _ := m.raw("POST", "/api/calendar/import", strings.NewReader(huge.String()), "text/calendar"); st != 400 {
		t.Fatalf("import of an over-long line: status %d", st)
	}
}

// Parallel "change my vote" requests must never leave more than one ballot.
func TestSecOneBallotPerVoter(t *testing.T) {
	e := newEnv(t)
	owner, m := e.user("owner"), e.user("mallory")
	poll := owner.id("POST", "/api/polls", M{"title": "Chair", "kind": "single", "options": []string{"Mallory", "Other"}, "allow_change": true, "results_visible": "always", "shares": all()})
	var p Poll
	owner.ok("GET", "/api/polls/"+poll, nil, &p)
	opt := p.Options[0].ID
	for round := 0; round < 5; round++ {
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				m.call("POST", "/api/polls/"+poll+"/vote", M{"choices": M{opt: 1}})
			}()
		}
		wg.Wait()
	}
	owner.ok("GET", "/api/polls/"+poll, nil, &p)
	if p.Voters != 1 || p.Options[0].Votes != 1 {
		t.Fatalf("voters=%d votes=%d, want 1 and 1", p.Voters, p.Options[0].Votes)
	}
}

// Out-of-range times are refused, so date arithmetic in queries cannot
// overflow and break the calendar for everyone.
func TestSecTimesAreBounded(t *testing.T) {
	e := newEnv(t)
	m, victim := e.user("mallory"), e.user("victim")
	huge := int64(9223372036854775807)
	m.fail(400, "POST", "/api/calendar", M{"title": "boom", "starts_at": 1, "ends_at": huge})
	m.fail(400, "POST", "/api/polls", M{"title": "x", "kind": "single", "options": []string{"a", "b"}, "closes_at": huge})
	list := m.id("POST", "/api/lists", M{"title": "l", "kind": "todo"})
	m.fail(400, "POST", "/api/lists/"+list+"/items", M{"text": "x", "due_at": huge})
	victim.ok("GET", "/api/calendar?from=-5&to=9223372036854775807", nil, nil)
	victim.ok("POST", "/api/calendar", M{"title": "ok", "starts_at": 1900000000, "ends_at": 1900003600}, nil)
	var evs []CalEvent
	victim.ok("GET", "/api/calendar", nil, &evs)
	if len(evs) != 1 {
		t.Fatalf("calendar: %d events", len(evs))
	}
}

// Parallel wrong passwords must all be counted towards the lockout.
func TestSecLockoutCountsParallelAttempts(t *testing.T) {
	e := newEnv(t)
	u := e.user("target")
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"username":"target","password":"guess-%d-xxxxxxxxxx"}`, i)
			if resp, err := http.Post(e.srv.URL+"/api/auth/login", "application/json", strings.NewReader(body)); err == nil {
				resp.Body.Close()
			}
		}(i)
	}
	wg.Wait()
	var locked int64
	e.a.DB.Row(context.Background(), `SELECT locked_until FROM users WHERE id=?`, u.ID).Scan(&locked)
	if locked == 0 {
		t.Fatal("30 parallel wrong passwords did not lock the account")
	}
	(&client{e: e}).fail(401, "POST", "/api/auth/login", M{"username": "target", "password": testPW})
}

// Deleting an account must neither destroy decisions nor break numbering.
func TestSecDecisionsSurviveUserDeletion(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	l1, l2 := e.user("l1", RoleLeader), e.user("l2", RoleLeader)
	l1.ok("POST", "/api/decisions", M{"title": "private one"}, nil)
	l2.ok("POST", "/api/decisions", M{"title": "shared", "shares": all()}, nil)
	admin.ok("PATCH", "/api/admin/users/"+l1.ID, M{"roles": []string{RoleMember}}, nil)
	admin.ok("DELETE", "/api/admin/users/"+l1.ID, nil, nil)
	var out struct{ Number string }
	l2.ok("POST", "/api/decisions", M{"title": "next", "shares": all()}, &out)
	if !strings.HasSuffix(out.Number, "-003") {
		t.Fatalf("next number %q, want …-003", out.Number)
	}
	var n int
	e.a.DB.Row(context.Background(), `SELECT COUNT(*) FROM decisions`).Scan(&n)
	if n != 3 {
		t.Fatalf("%d decisions kept, want 3", n)
	}
}

func openStream(t *testing.T, c *client) (lines <-chan string, status int, closeFn func()) {
	t.Helper()
	req, _ := http.NewRequest("GET", c.e.srv.URL+"/api/events", nil)
	req.AddCookie(c.cookie)
	tr := &http.Transport{}
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan string, 100)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			ch <- sc.Text()
		}
		close(ch)
	}()
	return ch, resp.StatusCode, func() { resp.Body.Close(); tr.CloseIdleConnections() }
}

// closed reports whether the server ends the stream within a short time.
func closed(lines <-chan string) bool {
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-lines:
			if !ok {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// One account cannot take all event stream slots.
func TestSecStreamLimitPerUser(t *testing.T) {
	e := newEnv(t)
	m, victim := e.user("mallory"), e.user("victim")
	for i := 0; i < 3*maxStreamsPerUser; i++ {
		_, st, cl := openStream(t, m)
		defer cl()
		if st != 200 {
			t.Fatalf("stream %d: %d", i, st)
		}
	}
	e.a.hub.mu.Lock()
	open := len(e.a.hub.clients)
	e.a.hub.mu.Unlock()
	if open > maxStreamsPerUser {
		t.Fatalf("%d streams held by one account, limit %d", open, maxStreamsPerUser)
	}
	_, st, cl := openStream(t, victim)
	defer cl()
	if st != 200 {
		t.Fatalf("another member's stream: %d", st)
	}
}

// An event stream ends when its session or account does.
func TestSecStreamEndsWithSession(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	for name, end := range map[string]func(c *client){
		"logout":  func(c *client) { c.ok("POST", "/api/auth/logout", nil, nil) },
		"disable": func(c *client) { admin.ok("PATCH", "/api/admin/users/"+c.ID, M{"disabled": true}, nil) },
		"delete":  func(c *client) { admin.ok("DELETE", "/api/admin/users/"+c.ID, nil, nil) },
		"password-reset": func(c *client) {
			admin.ok("POST", "/api/admin/users/"+c.ID+"/password", M{"password": "another-temp-pass"}, nil)
		},
		"admin-sign-out":    func(c *client) { admin.ok("POST", "/api/admin/users/"+c.ID+"/sign-out", nil, nil) },
		"session-row-gone":  nil, // found by the periodic re-check
		"account-locked-pw": func(c *client) { e.a.SignOutUser(context.Background(), c.ID, "") },
	} {
		c := e.user("u" + strings.ReplaceAll(name, "-", ""))
		if end == nil {
			e.a.streamPing = 100 * time.Millisecond
		}
		lines, st, cl := openStream(t, c)
		if st != 200 {
			t.Fatalf("%s: stream status %d", name, st)
		}
		<-lines // ": connected"
		if end == nil {
			e.a.DB.Exec(context.Background(), `DELETE FROM sessions WHERE user_id=?`, c.ID)
		} else {
			end(c)
		}
		if !closed(lines) {
			t.Errorf("%s: the stream stayed open", name)
		}
		cl()
		e.a.streamPing = 25 * time.Second
	}
}

// A session alone must not be enough to guess the password or to change the
// account's sign-in methods.
func TestSecReauthentication(t *testing.T) {
	e := newEnv(t)
	a := e.user("anna")
	other := e.login("anna", testPW) // a second session of the same account
	a.fail(400, "POST", "/api/me/mfa/setup", M{})
	a.fail(400, "POST", "/api/me/passkeys/begin", M{"password": "wrong wrong wrong"})
	st := 0
	for i := 0; i < maxFailures+2 && st != 401; i++ {
		st, _ = a.call("POST", "/api/me/password", M{"current": fmt.Sprintf("guess-number-%d", i), "new": "another long password"})
	}
	if st != 401 {
		t.Fatal("guessing the current password was never stopped")
	}
	// The account is locked and every session was ended.
	other.fail(401, "GET", "/api/me", nil)
	(&client{e: e}).fail(401, "POST", "/api/auth/login", M{"username": "anna", "password": testPW})
}

// The audit log must not expose the content of private decisions.
func TestSecAuditLogHasNoDecisionContent(t *testing.T) {
	e := newEnv(t)
	admin, leader := e.user("admin", RoleAdmin), e.user("leader", RoleLeader)
	id := leader.id("POST", "/api/decisions", M{"title": "Dismiss the treasurer (confidential)"})
	leader.ok("PATCH", "/api/decisions/"+id, M{"title": "Dismiss the treasurer (confidential)", "status": "revoked", "comment": "he paid the money back"}, nil)
	_, b := admin.call("GET", "/api/admin/logs?action=decision", nil)
	if !bytes.Contains(b, []byte("decision.revised")) {
		t.Fatalf("revision not logged: %s", b)
	}
	if bytes.Contains(b, []byte("treasurer")) || bytes.Contains(b, []byte("paid the money")) {
		t.Fatalf("private decision content in the audit log: %s", b)
	}
}

// Changing or deleting a role must not leave the organisation without an administrator.
func TestSecRolesKeepAnAdministrator(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	rid := admin.id("POST", "/api/roles", M{"name": "Board", "perms": AllPerms})
	admin.ok("PATCH", "/api/admin/users/"+admin.ID, M{"roles": []string{rid}}, nil)
	if c := admin.fail(400, "DELETE", "/api/roles/"+rid, nil); c != "last_admin" {
		t.Fatalf("got %q", c)
	}
	if c := admin.fail(400, "PUT", "/api/roles/"+rid, M{"name": "Board", "perms": []string{PermLogs}}); c != "last_admin" {
		t.Fatalf("got %q", c)
	}
	admin.ok("GET", "/api/admin/users", nil, nil)
}

// Uploads count against a per-account quota.
func TestSecStorageQuota(t *testing.T) {
	e := newEnv(t)
	e.a.Cfg.Storage.UserQuotaMB = 2
	m, other := e.user("mallory"), e.user("other")
	chunk := bytes.Repeat([]byte("A"), 900<<10)
	m.upload("a.bin", chunk)
	m.upload("b.bin", chunk)
	var buf bytes.Buffer
	if st := uploadStatus(t, m, "c.bin", chunk, &buf); st != 413 || !strings.Contains(buf.String(), "quota_exceeded") {
		t.Fatalf("third upload: %d %s", st, buf.String())
	}
	other.upload("mine.bin", chunk) // the quota is per account
	list := m.id("POST", "/api/lists", M{"title": "l", "kind": "plain"})
	e.a.DB.Exec(context.Background(), `UPDATE lists SET owner_id=owner_id`) // no-op, keeps the helper honest on both databases
	for i := 0; i < 3; i++ {
		m.ok("POST", "/api/lists/"+list+"/items", M{"text": "x"}, nil)
	}
}

func uploadStatus(t *testing.T, c *client, name string, content []byte, out *bytes.Buffer) int {
	t.Helper()
	var body bytes.Buffer
	boundary := "xxBOUNDARYxx"
	fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=%q\r\nContent-Type: application/octet-stream\r\n\r\n", boundary, name)
	body.Write(content)
	fmt.Fprintf(&body, "\r\n--%s--\r\n", boundary)
	st, b, _ := c.raw("POST", "/api/files", &body, "multipart/form-data; boundary="+boundary)
	out.Write(b)
	return st
}

// Imported identifiers and text cannot inject lines into exports.
func TestSecNoInjectionThroughImport(t *testing.T) {
	e := newEnv(t)
	m, victim := e.user("mallory"), e.user("victim")
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:x1\rATTENDEE:mailto:evil@example.org\rX-INJECTED:1\nSUMMARY:Party\rX-ALSO:1\nDTSTART:20300101T100000Z\nEND:VEVENT\nEND:VCALENDAR\n"
	if st, b, _ := m.raw("POST", "/api/calendar/import", strings.NewReader(ics), "text/calendar"); st != 200 {
		t.Fatal(st, string(b))
	}
	var evs []M
	m.ok("GET", "/api/calendar", nil, &evs)
	m.ok("PUT", "/api/acl/event/"+evs[0]["id"].(string), all(), nil)
	_, out, _ := victim.raw("GET", "/api/calendar/export.ics", nil, "")
	for _, line := range strings.Split(string(out), "\r\n") {
		if strings.Contains(line, "\r") || strings.HasPrefix(line, "ATTENDEE") || strings.HasPrefix(line, "X-INJECTED") || strings.HasPrefix(line, "X-ALSO") {
			t.Fatalf("injected line in export: %q", line)
		}
	}
	// Control characters are removed from stored text in general.
	m.ok("PATCH", "/api/me", M{"phone": "070\x00-1\x1b[31m23"}, nil)
	var me struct{ User User }
	m.ok("GET", "/api/me", nil, &me)
	if me.User.Phone != "070-1[31m23" {
		t.Fatalf("phone %q", me.User.Phone)
	}
}

// Display names must be unique and free of invisible characters; e-mail
// addresses must be plain addresses.
func TestSecNamesAndEmail(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	m := e.user("mallory")
	for _, name := range []string{"Admin", "ADMIN", " a d m i n ", "Ａｄｍｉｎ", "Ad​min", "‮Admin"} {
		if c := m.fail(400, "PATCH", "/api/me", M{"display_name": name}); c != "display_name_taken" {
			t.Errorf("%q: got %q", name, c)
		}
	}
	m.ok("PATCH", "/api/me", M{"display_name": "Mal‮lory​  Smith\x07"}, nil)
	var me struct{ User User }
	m.ok("GET", "/api/me", nil, &me)
	if me.User.DisplayName != "Mallory Smith" {
		t.Fatalf("display name %q", me.User.DisplayName)
	}
	admin.fail(400, "PATCH", "/api/admin/users/"+admin.ID, M{"display_name": "mallory smith"})
	admin.fail(400, "POST", "/api/admin/users", M{"username": "eve", "display_name": "Mallory  Smith", "password": testPW})
	for _, bad := range []string{"a@example.org?cc=boss@example.org", "a@example.org&subject=x", "Eve <e@example.org>", "a b@example.org", "a@localhost", "not-an-address", "a@example.org,b@example.org"} {
		m.fail(400, "PATCH", "/api/me", M{"email": bad})
		admin.fail(400, "PATCH", "/api/admin/users/"+m.ID, M{"email": bad})
	}
	m.ok("PATCH", "/api/me", M{"email": "mallory.smith+club@example.org"}, nil)
}

// Uploaded files are served as opaque downloads whatever their name.
func TestSecDownloadsAreOpaque(t *testing.T) {
	e := newEnv(t)
	m := e.user("mallory")
	for _, name := range []string{"evil.js", "data.json", "page.html", "style.css", "image.svg"} {
		f := m.upload(name, []byte(`{"id":"x"}`))
		_, _, h := m.raw("GET", "/api/files/"+f.ID, nil, "")
		if ct := h.Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("%s served as %q", name, ct)
		}
		if !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") || h.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing download headers", name)
		}
	}
}

// Somebody who can write to the database or the file store, but has no key,
// cannot move encrypted values or files between records.
func TestSecCiphertextsAreBoundToTheirRecord(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	victim, m := e.user("victim"), e.user("mallory")
	victim.ok("PATCH", "/api/me", M{"phone": "070-SECRET", "contact_visible": false}, nil)
	// Copy the victim's encrypted fields into the attacker's row.
	if err := e.a.DB.Exec(ctx, `UPDATE users SET email_enc=(SELECT email_enc FROM users WHERE id=?), phone_enc=(SELECT phone_enc FROM users WHERE id=?) WHERE id=?`,
		victim.ID, victim.ID, m.ID); err != nil {
		t.Fatal(err)
	}
	var me struct{ User User }
	m.ok("GET", "/api/me", nil, &me)
	if me.User.Phone != "" || me.User.Email != "" {
		t.Fatalf("moved ciphertext was decrypted: %q %q", me.User.Email, me.User.Phone)
	}
	// Swap two stored files.
	f1 := m.upload("one.txt", []byte("content of file one"))
	f2 := m.upload("two.txt", []byte("content of file two"))
	var k1, k2 string
	e.a.DB.Row(ctx, `SELECT blob_key FROM files WHERE id=?`, f1.ID).Scan(&k1)
	e.a.DB.Row(ctx, `SELECT blob_key FROM files WHERE id=?`, f2.ID).Scan(&k2)
	p1, p2 := filepath.Join(e.dir, "blobs", k1[:2], k1), filepath.Join(e.dir, "blobs", k2[:2], k2)
	b1, _ := os.ReadFile(p1)
	b2, _ := os.ReadFile(p2)
	os.WriteFile(p1, b2, 0o600)
	os.WriteFile(p2, b1, 0o600)
	if st, body, _ := m.raw("GET", "/api/files/"+f1.ID, nil, ""); st == 200 && bytes.Contains(body, []byte("file two")) {
		t.Fatal("a swapped file was served under another file's id")
	}
}

// Data written by the first release (fields bound to the column only, files
// under one shared key) is still readable and is upgraded.
func TestSecLegacyDataIsUpgraded(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.user("anna")
	fieldKey, _ := hkdf.Key(sha256.New, e.a.Keys.Master(), nil, "samling/v1/field", 32)
	block, _ := aes.NewCipher(fieldKey)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	v1 := "v1:" + base64.RawStdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte("anna@old.example.org"), []byte("users.email")))
	e.a.DB.Exec(ctx, `UPDATE users SET email_enc=?, display_key='' WHERE id=?`, v1, a.ID)
	if err := e.a.upgradeData(ctx); err != nil {
		t.Fatal(err)
	}
	var stored, key string
	e.a.DB.Row(ctx, `SELECT email_enc, display_key FROM users WHERE id=?`, a.ID).Scan(&stored, &key)
	if !strings.HasPrefix(stored, "v2:") || key != "anna" {
		t.Fatalf("not upgraded: %q %q", stored[:3], key)
	}
	var me struct{ User User }
	a.ok("GET", "/api/me", nil, &me)
	if me.User.Email != "anna@old.example.org" {
		t.Fatalf("email %q", me.User.Email)
	}
	// A first-release file object: the plain stream under the shared key.
	f := a.upload("old.txt", []byte("placeholder"))
	var blobKey string
	e.a.DB.Row(ctx, `SELECT blob_key FROM files WHERE id=?`, f.ID).Scan(&blobKey)
	var legacy bytes.Buffer
	w, _ := crypto.NewEncryptWriter(&legacy, e.a.Keys.Blob)
	w.Write([]byte("written by the first release"))
	w.Close()
	if err := e.a.Blobs.B.Put(ctx, blobKey, &legacy); err != nil {
		t.Fatal(err)
	}
	if _, body, _ := a.raw("GET", "/api/files/"+f.ID, nil, ""); string(body) != "written by the first release" {
		t.Fatalf("legacy file: %q", body)
	}
}

// Anonymous ballots cannot be matched to voters from the database or a backup.
func TestSecAnonymousBallotsUnlinkable(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	// Voters are created in one order and vote in another.
	voters := []*client{e.user("v1"), e.user("v2"), e.user("v3"), e.user("v4"), e.user("v5")}
	order := []int{3, 0, 4, 1, 2}
	poll := owner.id("POST", "/api/polls", M{"title": "Secret", "kind": "yesno", "anonymous": true, "results_visible": "always", "shares": all()})
	var p Poll
	owner.ok("GET", "/api/polls/"+poll, nil, &p)
	if p.ResultsVisible != "after_close" {
		t.Fatalf("anonymous polls must hide the running tally, got %q", p.ResultsVisible)
	}
	for _, i := range order {
		voters[i].ok("POST", "/api/polls/"+poll+"/vote", M{"choices": M{p.Options[i%3].ID: 1}}, nil)
	}
	ctx := context.Background()
	// No per-person record of having voted, with a time, in the history.
	var logged int
	e.a.DB.Row(ctx, `SELECT COUNT(*) FROM activity WHERE res_id=? AND action='poll.voted'`, poll).Scan(&logged)
	if logged != 0 {
		t.Fatalf("%d history entries for votes in an anonymous poll", logged)
	}
	// In a backup, voters appear in an order unrelated to the order of voting.
	var buf bytes.Buffer
	if err := WriteBackup(ctx, e.a.DB, e.a.Blobs, e.a.Keys, "org", &buf, "a long passphrase"); err != nil {
		t.Fatal(err)
	}
	dec, _ := crypto.NewPassphraseReader(&buf, "a long passphrase")
	gz, _ := gzip.NewReader(dec)
	tr := tar.NewReader(gz)
	var inBackup []string
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		if !strings.HasPrefix(h.Name, "db/poll_voters/") {
			io.Copy(io.Discard, tr)
			continue
		}
		jd := json.NewDecoder(tr)
		for {
			row := map[string]any{}
			if jd.Decode(&row) != nil {
				break
			}
			inBackup = append(inBackup, row["user_id"].(string))
		}
	}
	// The same holds for the table itself in storage order.
	var inTable []string
	e.a.DB.Each(ctx, `SELECT user_id FROM poll_voters WHERE poll_id=?`, []any{poll}, func(r *sql.Rows) error {
		var u string
		r.Scan(&u)
		inTable = append(inTable, u)
		return nil
	})
	voteOrder := make([]string, len(order))
	for n, i := range order {
		voteOrder[n] = voters[i].ID
	}
	for name, got := range map[string][]string{"backup": inBackup, "table": inTable} {
		if len(got) != len(voters) {
			t.Fatalf("%s: %d voters", name, len(got))
		}
		if name == "backup" && !sort.StringsAreSorted(got) {
			t.Errorf("backup rows are not in key order: %v", got)
		}
		if strings.Join(got, ",") == strings.Join(voteOrder, ",") {
			t.Errorf("%s lists voters in the order they voted", name)
		}
	}
	// Still exactly one vote each.
	voters[0].fail(400, "POST", "/api/polls/"+poll+"/vote", M{"choices": M{p.Options[0].ID: 1}})
	owner.ok("POST", "/api/polls/"+poll+"/close", nil, nil)
	owner.ok("GET", "/api/polls/"+poll, nil, &p)
	if p.Voters != 5 || p.Options[0].Votes+p.Options[1].Votes+p.Options[2].Votes != 5 {
		t.Fatalf("tally: %+v", p)
	}
}

// Only the owner of a resource can share it with a private group, so nobody
// else can plant a share the owner cannot see or remove.
func TestSecNoHiddenSharesByModerators(t *testing.T) {
	e := newEnv(t)
	alice, mod, bob := e.user("alice"), e.user("mod", RoleLeader), e.user("bob")
	list := alice.id("POST", "/api/lists", M{"title": "Budget draft", "kind": "todo", "shares": []M{share("user", bob.ID, 1)}})
	lg := mod.id("POST", "/api/groups", M{"name": "backdoor", "scope": "local", "members": []string{mod.ID}})
	mod.fail(400, "PUT", "/api/acl/list/"+list, []M{share("user", bob.ID, 1), share("group", lg, 2)})
	// The owner's own private group survives an edit by a moderator …
	own := alice.id("POST", "/api/groups", M{"name": "friends", "scope": "local", "members": []string{bob.ID}})
	alice.ok("PUT", "/api/acl/list/"+list, []M{share("group", own, 2), share("all", "", 1)}, nil)
	mod.ok("PUT", "/api/acl/list/"+list, []M{share("all", "", 1)}, nil)
	bob.ok("POST", "/api/lists/"+list+"/items", M{"text": "still allowed through alice's group"}, nil)
	// … and the owner can always make the resource private again.
	alice.ok("PUT", "/api/acl/list/"+list, []M{}, nil)
	var shares []M
	alice.ok("GET", "/api/acl/list/"+list, nil, &shares)
	if len(shares) != 0 {
		t.Fatalf("shares left: %v", shares)
	}
	mod.fail(404, "GET", "/api/lists/"+list, nil)
}

// Recovery codes: single use, replaceable, removed with two-step verification.
func TestSecRecoveryCodes(t *testing.T) {
	e := newEnv(t)
	a := e.user("anna")
	var setup struct{ Secret string }
	a.ok("POST", "/api/me/mfa/setup", M{"password": testPW}, &setup)
	code, _ := crypto.TOTPCode(setup.Secret, time.Now())
	var enabled struct {
		Codes []string `json:"recovery_codes"`
	}
	a.ok("POST", "/api/me/mfa/enable", M{"code": code}, &enabled)
	if len(enabled.Codes) != recoveryCodeCount || len(enabled.Codes[0]) != 19 {
		t.Fatalf("codes: %v", enabled.Codes)
	}
	var stored string
	e.a.DB.Row(context.Background(), `SELECT code_hash FROM recovery_codes WHERE user_id=? LIMIT 1`, a.ID).Scan(&stored)
	for _, c := range enabled.Codes {
		if strings.Contains(stored, strings.ReplaceAll(c, "-", "")) {
			t.Fatal("recovery codes must be stored hashed")
		}
	}
	anon := &client{e: e}
	login := func(code string) int {
		st, _ := anon.call("POST", "/api/auth/login", M{"username": "anna", "password": testPW, "code": code})
		return st
	}
	if login("aaaa-bbbb-cccc-dddd") != 401 {
		t.Fatal("unknown recovery code accepted")
	}
	if login(strings.ToUpper(enabled.Codes[0])) != 200 {
		t.Fatal("recovery code rejected")
	}
	if login(enabled.Codes[0]) != 401 {
		t.Fatal("recovery code accepted twice")
	}
	var me struct{ User User }
	a.ok("GET", "/api/me", nil, &me)
	if me.User.RecoveryCodes != recoveryCodeCount-1 {
		t.Fatalf("remaining: %d", me.User.RecoveryCodes)
	}
	// New codes replace the old ones.
	a.fail(400, "POST", "/api/me/mfa/recovery-codes", M{"password": "wrong wrong wrong"})
	var fresh struct {
		Codes []string `json:"recovery_codes"`
	}
	a.ok("POST", "/api/me/mfa/recovery-codes", M{"password": testPW}, &fresh)
	if login(enabled.Codes[1]) != 401 || login(fresh.Codes[0]) != 200 {
		t.Fatal("regeneration did not replace the codes")
	}
	a.ok("POST", "/api/me/mfa/disable", M{"password": testPW}, nil)
	var n int
	e.a.DB.Row(context.Background(), `SELECT COUNT(*) FROM recovery_codes WHERE user_id=?`, a.ID).Scan(&n)
	if n != 0 {
		t.Fatal("recovery codes must go when two-step verification is turned off")
	}
}

// Idle timeout: a session without activity ends; background refreshes of the
// web client do not count as activity.
func TestSecIdleTimeout(t *testing.T) {
	e := newEnv(t)
	e.a.Cfg.Security.IdleTimeoutMinutes = 10
	a, b := e.user("anna"), e.user("bert")
	start := time.Now()
	clock := start
	e.a.now = func() time.Time { return clock }
	active := func(c *client) int {
		req, _ := http.NewRequest("GET", e.srv.URL+"/api/me", nil)
		req.AddCookie(c.cookie)
		req.Header.Set("X-User-Active", "1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for i := 0; i < 4; i++ { // 20 minutes in steps of 5
		clock = clock.Add(5 * time.Minute)
		if st := active(a); st != 200 {
			t.Fatalf("active session ended after %v: %d", clock.Sub(start), st)
		}
		if i < 1 {
			b.ok("GET", "/api/me", nil, nil) // background request: allowed, but does not extend
		}
	}
	b.fail(401, "GET", "/api/me", nil)
	clock = clock.Add(11 * time.Minute)
	if st := active(a); st != 401 {
		t.Fatalf("idle session still valid: %d", st)
	}
}

// "Sign out everywhere", by the person and by an administrator.
func TestSecSignOutEverywhere(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	a := e.user("anna")
	phone, laptop := e.login("anna", testPW), e.login("anna", testPW)
	a.ok("POST", "/api/me/sessions/end-others", nil, nil)
	a.ok("GET", "/api/me", nil, nil)
	phone.fail(401, "GET", "/api/me", nil)
	laptop.fail(401, "GET", "/api/me", nil)

	other := e.login("anna", testPW)
	admin.ok("POST", "/api/admin/users/"+a.ID+"/sign-out", nil, nil)
	a.fail(401, "GET", "/api/me", nil)
	other.fail(401, "GET", "/api/me", nil)
	e.user("mia").fail(403, "POST", "/api/admin/users/"+a.ID+"/sign-out", nil)
	var logs []M
	admin.ok("GET", "/api/admin/logs?action=sessions", nil, &logs)
	if len(logs) != 2 {
		t.Fatalf("audit entries: %d", len(logs))
	}
}

// Passkey endpoints: registration needs the password, challenges are single
// use and garbage is rejected. (A full registration and sign-in with a
// virtual authenticator is part of the browser test.)
func TestSecPasskeyEndpoints(t *testing.T) {
	e := newEnv(t)
	a := e.user("anna")
	a.fail(400, "POST", "/api/me/passkeys/begin", M{"password": "wrong wrong wrong"})
	// Passkeys are bound to a host name; the test server is reached by IP address.
	if c := a.fail(400, "POST", "/api/me/passkeys/begin", M{"password": testPW}); c != "passkeys_unavailable" {
		t.Fatalf("got %q", c)
	}
	e.a.Cfg.Server.PublicURL = "http://localhost" + e.srv.URL[strings.LastIndex(e.srv.URL, ":"):]
	var begin struct {
		Ceremony string
		Options  struct {
			Challenge              string
			RP                     struct{ ID, Name string }
			User                   struct{ Name string }
			AuthenticatorSelection struct{ ResidentKey, UserVerification string }
		}
	}
	a.ok("POST", "/api/me/passkeys/begin", M{"password": testPW}, &begin)
	o := begin.Options
	if begin.Ceremony == "" || len(o.Challenge) < 32 || o.RP.ID != "localhost" || o.User.Name != "anna" ||
		o.AuthenticatorSelection.ResidentKey != "required" || o.AuthenticatorSelection.UserVerification != "required" {
		t.Fatalf("registration options: %+v", begin)
	}
	a.fail(400, "POST", "/api/me/passkeys/finish", M{"ceremony": begin.Ceremony, "name": "x", "credential": M{"id": "AAAA"}})
	if c := a.fail(400, "POST", "/api/me/passkeys/finish", M{"ceremony": begin.Ceremony, "name": "x", "credential": M{}}); c != "passkey_expired" {
		t.Fatalf("a challenge must be single use, got %q", c)
	}
	anon := &client{e: e}
	var lb struct {
		Ceremony string
		Options  struct{ Challenge, UserVerification string }
	}
	anon.ok("POST", "/api/auth/passkey/begin", nil, &lb)
	if lb.Options.UserVerification != "required" {
		t.Fatalf("sign-in options: %+v", lb)
	}
	anon.fail(401, "POST", "/api/auth/passkey/finish", M{"ceremony": lb.Ceremony, "credential": M{"id": "AAAA", "rawId": "AAAA", "type": "public-key"}})
	anon.fail(400, "POST", "/api/auth/passkey/finish", M{"ceremony": lb.Ceremony, "credential": M{}})
	// A registration challenge cannot be used to sign in.
	a.ok("POST", "/api/me/passkeys/begin", M{"password": testPW}, &begin)
	anon.fail(400, "POST", "/api/auth/passkey/finish", M{"ceremony": begin.Ceremony, "credential": M{}})
	var list []Passkey
	a.ok("GET", "/api/me/passkeys", nil, &list)
	if len(list) != 0 {
		t.Fatalf("passkeys: %v", list)
	}
}
