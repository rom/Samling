package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

// a 1x1 PNG
var tinyPNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")

func TestProfileFieldsAndAvatar(t *testing.T) {
	e := newEnv(t)
	admin := e.user("admin", RoleAdmin)
	a, b := e.user("anna"), e.user("bert")
	a.ok("PATCH", "/api/me", M{
		"profile":        M{"address": "Björkvägen 3, 123 45 Björkö", "interests": "Orienteering, baking", "specialities": "First aid", "about": "", "unknown": "x"},
		"profile_shared": M{"interests": true, "specialities": true, "unknown": true},
	}, nil)
	find := func(c *client, name string) Person {
		var people []Person
		c.ok("GET", "/api/users", nil, &people)
		for _, p := range people {
			if p.Username == name {
				return p
			}
		}
		t.Fatalf("%s not in directory", name)
		return Person{}
	}
	// Other members see only the shared fields.
	p := find(b, "anna")
	if p.Profile["interests"] != "Orienteering, baking" || p.Profile["specialities"] != "First aid" || p.Profile["address"] != "" || len(p.Profile) != 2 {
		t.Fatalf("bert sees %v", p.Profile)
	}
	// The person and account managers see all of them.
	if find(a, "anna").Profile["address"] == "" || find(admin, "anna").Profile["address"] == "" {
		t.Fatal("owner / account manager must see unshared fields")
	}
	var me struct{ User User }
	a.ok("GET", "/api/me", nil, &me)
	if me.User.Profile["address"] != "Björkvägen 3, 123 45 Björkö" || !me.User.ProfileShared["interests"] || me.User.ProfileShared["address"] || len(me.User.Profile) != 3 {
		t.Fatalf("own profile: %+v %+v", me.User.Profile, me.User.ProfileShared)
	}
	// Stored encrypted.
	var enc string
	e.a.DB.Row(context.Background(), `SELECT profile_enc FROM users WHERE id=?`, a.ID).Scan(&enc)
	if enc == "" || strings.Contains(enc, "Björk") || strings.Contains(enc, "baking") {
		t.Fatal("profile fields must be encrypted at rest")
	}
	// Search finds shared fields only.
	var hits []SearchHit
	b.ok("GET", "/api/search?q=baking&types=user", nil, &hits)
	if len(hits) != 1 {
		t.Fatalf("shared interest not searchable: %v", hits)
	}
	b.ok("GET", "/api/search?q=Bj%C3%B6rkv%C3%A4gen&types=user", nil, &hits)
	if len(hits) != 0 {
		t.Fatal("unshared address found by another member")
	}
	// Unsharing takes effect; other profile updates do not wipe the fields.
	a.ok("PATCH", "/api/me", M{"profile_shared": M{}}, nil)
	a.ok("PATCH", "/api/me", M{"phone": "070-1"}, nil)
	if len(find(b, "anna").Profile) != 0 {
		t.Fatal("unshared fields still visible")
	}
	if find(a, "anna").Profile["interests"] == "" {
		t.Fatal("profile lost by an unrelated update")
	}

	// Avatars: a built-in one, or an uploaded photo.
	a.ok("PATCH", "/api/me", M{"avatar": "preset:7"}, nil)
	a.fail(400, "PATCH", "/api/me", M{"avatar": "preset:999"})
	a.fail(400, "PATCH", "/api/me", M{"avatar": "https://evil.example/x.png"})
	photo := a.upload("me.png", tinyPNG)
	if photo.Mime != "image/png" {
		t.Fatalf("picture type: %q", photo.Mime)
	}
	notMine := b.upload("b.png", tinyPNG)
	a.fail(400, "PATCH", "/api/me", M{"avatar": "file:" + notMine.ID}) // somebody else's upload
	fake := a.upload("fake.png", []byte("<html><script>alert(1)</script>"))
	if fake.Mime != "application/octet-stream" {
		t.Fatalf("a file that only claims to be a picture got type %q", fake.Mime)
	}
	a.fail(400, "PATCH", "/api/me", M{"avatar": "file:" + fake.ID})
	a.ok("PATCH", "/api/me", M{"avatar": "file:" + photo.ID}, nil)
	if find(b, "anna").Avatar != "file:"+photo.ID {
		t.Fatal("avatar not in directory")
	}
	// Every member can see a profile photo, inline and with its real type.
	st, body, h := b.raw("GET", "/api/files/"+photo.ID+"/image", nil, "")
	if st != 200 || !bytes.Equal(body, tinyPNG) || h.Get("Content-Type") != "image/png" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("avatar image: %d %s", st, h.Get("Content-Type"))
	}
	// Something that is not a picture is never shown inline.
	b.fail(404, "GET", "/api/files/"+fake.ID+"/image", nil)
	a.fail(404, "GET", "/api/files/"+fake.ID+"/image", nil)
}

func TestListOwnersAndBoardColours(t *testing.T) {
	e := newEnv(t)
	a, b := e.user("anna"), e.user("bert")
	list := a.id("POST", "/api/lists", M{"title": "Tasks", "kind": "todo", "shares": []M{share("user", b.ID, 2)}})
	i1 := a.id("POST", "/api/lists/"+list+"/items", M{"text": "Book the bus", "assignee_id": b.ID})
	a.fail(400, "POST", "/api/lists/"+list+"/items", M{"text": "x", "assignee_id": "nobody"})
	i2 := a.id("POST", "/api/lists/"+list+"/items", M{"text": "Buy food"})
	b.ok("PATCH", "/api/lists/"+list+"/items/"+i2, M{"assignee_id": a.ID}, nil)
	b.ok("PATCH", "/api/lists/"+list+"/items/"+i1, M{"assignee_id": ""}, nil)
	var l List
	a.ok("GET", "/api/lists/"+list, nil, &l)
	if l.Items[0].AssigneeID != "" || l.Items[1].AssigneeID != a.ID {
		t.Fatalf("owners: %+v", l.Items)
	}

	board := a.id("POST", "/api/boards", M{"title": "Work", "columns": []string{"To do", "Done"}, "color": "teal"})
	a.fail(400, "POST", "/api/boards", M{"title": "x", "columns": []string{"a"}, "color": "#ff0000"})
	var bd Board
	a.ok("GET", "/api/boards/"+board, nil, &bd)
	card := a.id("POST", "/api/boards/"+board+"/cards", M{"column_id": bd.Columns[0].ID, "title": "Paint", "color": "red"})
	a.ok("PATCH", "/api/boards/"+board+"/cards/"+card, M{"color": "green"}, nil)
	a.fail(400, "PATCH", "/api/boards/"+board+"/cards/"+card, M{"color": "javascript"})
	a.ok("PATCH", "/api/boards/"+board, M{"title": "Work", "color": "purple"}, nil)
	a.ok("GET", "/api/boards/"+board, nil, &bd)
	if bd.Color != "purple" || bd.CardList[0].Color != "green" {
		t.Fatalf("colours: %q %q", bd.Color, bd.CardList[0].Color)
	}
	var boards []Board
	a.ok("GET", "/api/boards", nil, &boards)
	if boards[0].Color != "purple" {
		t.Fatal("colour missing in the board list")
	}
}

func TestBlockingDirectMessages(t *testing.T) {
	e := newEnv(t)
	a, m, c := e.user("anna"), e.user("mallory"), e.user("cia")
	dm := m.id("POST", "/api/dm", M{"user_ids": []string{a.ID}})
	mid := m.id("POST", "/api/channels/"+dm+"/messages", M{"body": "hello?"})
	a.ok("PUT", "/api/me/blocks/"+m.ID, nil, nil)
	a.fail(400, "PUT", "/api/me/blocks/"+a.ID, nil)
	var blocks []string
	a.ok("GET", "/api/me/blocks", nil, &blocks)
	if len(blocks) != 1 || blocks[0] != m.ID {
		t.Fatalf("blocks: %v", blocks)
	}
	// The blocked person can no longer write to, react in or start conversations with the blocker.
	if code := m.fail(403, "POST", "/api/channels/"+dm+"/messages", M{"body": "why no answer?"}); code != "dm_blocked" {
		t.Fatalf("got %q", code)
	}
	m.fail(403, "POST", "/api/messages/"+mid+"/reactions", M{"emoji": "👍"})
	m.fail(403, "POST", "/api/dm", M{"user_ids": []string{a.ID}})
	m.fail(403, "POST", "/api/dm", M{"user_ids": []string{a.ID, c.ID}}) // not through a group conversation either
	// Channels are unaffected, and so are other people.
	var chans []Channel
	m.ok("GET", "/api/channels", nil, &chans)
	for _, ch := range chans {
		if ch.Kind == "channel" {
			m.ok("POST", "/api/channels/"+ch.ID+"/messages", M{"body": "hi all"}, nil)
		}
	}
	m.ok("POST", "/api/dm", M{"user_ids": []string{c.ID}}, nil)
	// The blocker can still write, and can lift the block.
	a.ok("POST", "/api/channels/"+dm+"/messages", M{"body": "please stop"}, nil)
	a.ok("DELETE", "/api/me/blocks/"+m.ID, nil, nil)
	m.ok("POST", "/api/channels/"+dm+"/messages", M{"body": "sorry"}, nil)
	// Nobody can see who has blocked whom except the blocker.
	m.ok("GET", "/api/me/blocks", nil, &blocks)
	if len(blocks) != 0 {
		t.Fatal("blocks of another person visible")
	}
}

func TestCommentsAndReactions(t *testing.T) {
	e := newEnv(t)
	leader := e.user("lena", RoleLeader)
	a, b, outsider := e.user("anna"), e.user("bert"), e.user("olle")
	ch := leader.id("POST", "/api/channels", M{"name": "news", "shares": []M{share("user", a.ID, 1), share("user", b.ID, 2)}})
	root := leader.id("POST", "/api/channels/"+ch+"/messages", M{"body": "The **spring** clean-up is on *Saturday*. __Bring gloves__!"})
	// Comments
	c1 := b.id("POST", "/api/channels/"+ch+"/messages", M{"body": "I will be there", "parent_id": root})
	c2 := leader.id("POST", "/api/channels/"+ch+"/messages", M{"body": "Great!", "parent_id": c1}) // a comment on a comment joins the thread
	a.fail(403, "POST", "/api/channels/"+ch+"/messages", M{"body": "me too", "parent_id": root})   // read-only member
	b.fail(400, "POST", "/api/channels/"+ch+"/messages", M{"body": "x", "parent_id": "nope"})
	var chans []Channel
	b.ok("GET", "/api/channels", nil, &chans)
	other := ""
	for _, x := range chans {
		if x.Name == "general" {
			other = x.ID
		}
	}
	b.fail(400, "POST", "/api/channels/"+other+"/messages", M{"body": "x", "parent_id": root}) // parent from another channel
	// Reactions: reading is enough; a second click removes it.
	a.ok("POST", "/api/messages/"+root+"/reactions", M{"emoji": "👍"}, nil)
	b.ok("POST", "/api/messages/"+root+"/reactions", M{"emoji": "👍"}, nil)
	b.ok("POST", "/api/messages/"+root+"/reactions", M{"emoji": "❤️"}, nil)
	a.ok("POST", "/api/messages/"+c1+"/reactions", M{"emoji": "🎉"}, nil)
	b.ok("POST", "/api/messages/"+root+"/reactions", M{"emoji": "❤️"}, nil) // toggled off
	a.fail(400, "POST", "/api/messages/"+root+"/reactions", M{"emoji": "<script>"})
	a.fail(400, "POST", "/api/messages/"+root+"/reactions", M{"emoji": "💩"}) // not in the offered set
	outsider.fail(404, "POST", "/api/messages/"+root+"/reactions", M{"emoji": "👍"})
	var msgs []Message
	a.ok("GET", "/api/channels/"+ch+"/messages", nil, &msgs)
	by := map[string]Message{}
	for _, m := range msgs {
		by[m.ID] = m
	}
	if by[c1].ParentID != root || by[c2].ParentID != root || by[root].ParentID != "" {
		t.Fatalf("threading: %q %q", by[c1].ParentID, by[c2].ParentID)
	}
	r := by[root].Reactions
	if len(r) != 1 || r[0].Emoji != "👍" || r[0].Count != 2 || !r[0].Mine {
		t.Fatalf("reactions: %+v", r)
	}
	if rc := by[c1].Reactions; len(rc) != 1 || rc[0].Count != 1 {
		t.Fatalf("comment reactions: %+v", rc)
	}
	// Formatting marks are stored as written (the client renders them).
	if !strings.Contains(by[root].Body, "**spring**") {
		t.Fatalf("body: %q", by[root].Body)
	}
	// Deleting a message removes its reactions.
	leader.ok("DELETE", "/api/messages/"+root, nil, nil)
	var n int
	e.a.DB.Row(context.Background(), `SELECT COUNT(*) FROM reactions WHERE message_id=?`, root).Scan(&n)
	if n != 0 {
		t.Fatal("reactions of a deleted message remain")
	}
}

func TestAttachments(t *testing.T) {
	e := newEnv(t)
	leader := e.user("lena", RoleLeader)
	a, b, outsider := e.user("anna"), e.user("bert"), e.user("olle")
	list := a.id("POST", "/api/lists", M{"title": "Camp", "kind": "todo", "shares": []M{share("user", b.ID, 1)}})
	photo := a.upload("tent.png", tinyPNG)
	a.ok("POST", "/api/attachments/list/"+list, M{"file_id": photo.ID}, nil)
	// Read access shows attachments and the picture; no access shows nothing.
	var atts []Attachment
	b.ok("GET", "/api/attachments/list/"+list, nil, &atts)
	if len(atts) != 1 || atts[0].Name != "tent.png" || atts[0].Mime != "image/png" {
		t.Fatalf("attachments: %+v", atts)
	}
	if st, body, _ := b.raw("GET", "/api/files/"+photo.ID+"/image", nil, ""); st != 200 || !bytes.Equal(body, tinyPNG) {
		t.Fatalf("picture: %d", st)
	}
	outsider.fail(404, "GET", "/api/attachments/list/"+list, nil)
	outsider.fail(404, "GET", "/api/files/"+photo.ID+"/image", nil)
	outsider.fail(404, "GET", "/api/files/"+photo.ID, nil)
	// Adding needs write access and your own upload.
	bp := b.upload("b.png", tinyPNG)
	b.fail(403, "POST", "/api/attachments/list/"+list, M{"file_id": bp.ID})
	a.fail(400, "POST", "/api/attachments/list/"+list, M{"file_id": bp.ID})
	b.fail(403, "DELETE", "/api/attachments/list/"+list+"/"+photo.ID, nil)
	a.fail(404, "POST", "/api/attachments/user/"+a.ID, M{"file_id": photo.ID})
	// Cards follow their board; events, polls, meetings and decisions work the same way.
	board := a.id("POST", "/api/boards", M{"title": "B", "columns": []string{"To do"}, "shares": []M{share("user", b.ID, 2)}})
	var bd Board
	a.ok("GET", "/api/boards/"+board, nil, &bd)
	card := a.id("POST", "/api/boards/"+board+"/cards", M{"column_id": bd.Columns[0].ID, "title": "Fix"})
	b.ok("POST", "/api/attachments/card/"+card, M{"file_id": bp.ID}, nil)
	outsider.fail(404, "GET", "/api/attachments/card/"+card, nil)
	ev := a.id("POST", "/api/calendar", M{"title": "Party", "starts_at": 1900000000, "ends_at": 1900003600, "shares": all()})
	a.ok("POST", "/api/attachments/event/"+ev, M{"file_id": a.upload("flyer.png", tinyPNG).ID}, nil)
	outsider.ok("GET", "/api/attachments/event/"+ev, nil, &atts)
	if len(atts) != 1 {
		t.Fatal("event attachment")
	}
	poll := a.id("POST", "/api/polls", M{"title": "Colour?", "kind": "single", "options": []string{"a", "b"}, "shares": all()})
	a.ok("POST", "/api/attachments/poll/"+poll, M{"file_id": a.upload("swatch.png", tinyPNG).ID}, nil)
	b.fail(403, "POST", "/api/attachments/poll/"+poll, M{"file_id": bp.ID}) // voters cannot change the poll
	dec := leader.id("POST", "/api/decisions", M{"title": "D", "shares": all()})
	leader.ok("POST", "/api/attachments/decision/"+dec, M{"file_id": leader.upload("signed.png", tinyPNG).ID}, nil)
	// Removing an attachment, or the thing it is attached to, withdraws access.
	a.ok("DELETE", "/api/attachments/list/"+list+"/"+photo.ID, nil, nil)
	b.fail(404, "GET", "/api/files/"+photo.ID+"/image", nil)
	a.ok("DELETE", "/api/boards/"+board+"/cards/"+card, nil, nil)
	a.fail(404, "GET", "/api/files/"+bp.ID+"/image", nil) // anna is not the uploader and the card is gone
	var n int
	e.a.DB.Row(context.Background(), `SELECT COUNT(*) FROM attachments WHERE res_type='card'`).Scan(&n)
	if n != 0 {
		t.Fatal("attachments of a deleted card remain")
	}
}

func TestMentionsRespectAccess(t *testing.T) {
	e := newEnv(t)
	leader := e.user("lena", RoleLeader)
	a, r, c := e.user("anna"), e.user("robert"), e.user("cia")
	type inbox struct {
		Unread int
		Items  []Notification
	}
	get := func(cl *client) inbox {
		var in inbox
		cl.ok("GET", "/api/notifications", nil, &in)
		return in
	}
	// A channel robert can read: he is told, once, with the text.
	open := leader.id("POST", "/api/channels", M{"name": "open", "shares": []M{share("all", "", 2)}})
	m1 := a.id("POST", "/api/channels/"+open+"/messages", M{"body": "Can you bring the nets, @robert? Mail me at anna@robert.example. @nobody @anna @ROBERT."})
	in := get(r)
	if in.Unread != 1 || len(in.Items) != 1 || in.Items[0].ActorID != a.ID || in.Items[0].ChannelID != open || in.Items[0].MessageID != m1 ||
		in.Items[0].Channel != "open" || !strings.Contains(in.Items[0].Snippet, "bring the nets") || in.Items[0].Read {
		t.Fatalf("robert's mentions: %+v", in)
	}
	if get(a).Unread != 0 {
		t.Fatal("mentioning yourself must not notify")
	}
	// A channel robert cannot read, and a direct conversation he is not part of: nothing.
	closed := leader.id("POST", "/api/channels", M{"name": "board", "shares": []M{share("user", a.ID, 2)}})
	a.ok("POST", "/api/channels/"+closed+"/messages", M{"body": "@robert should not hear about this"}, nil)
	dm := a.id("POST", "/api/dm", M{"user_ids": []string{c.ID}})
	a.ok("POST", "/api/channels/"+dm+"/messages", M{"body": "between us: @robert is getting a prize, @cia"}, nil)
	if in := get(r); in.Unread != 1 || len(in.Items) != 1 {
		t.Fatalf("robert was told about conversations he cannot read: %+v", in)
	}
	// The participant of the direct conversation is told.
	if in := get(c); in.Unread != 1 || in.Items[0].Channel != "" || in.Items[0].ChannelID != dm {
		t.Fatalf("cia's mentions: %+v", in)
	}
	// Nobody can read or mark somebody else's notifications.
	c.ok("POST", "/api/notifications/read", M{"ids": []string{in.Items[0].ID}}, nil)
	if get(r).Unread != 1 {
		t.Fatal("another person marked robert's mention as read")
	}
	// An edit that adds a mention notifies; repeating one does not notify twice.
	m2 := a.id("POST", "/api/channels/"+open+"/messages", M{"body": "The nets are in the shed"})
	a.ok("PATCH", "/api/messages/"+m2, M{"body": "The nets are in the shed, @robert"}, nil)
	a.ok("PATCH", "/api/messages/"+m1, M{"body": "Can you bring the nets and the balls, @robert?"}, nil)
	if in := get(r); in.Unread != 2 || len(in.Items) != 2 {
		t.Fatalf("after edits: %+v", in)
	}
	// Opening the conversation marks its mentions as read.
	r.ok("POST", "/api/channels/"+open+"/read", nil, nil)
	if in := get(r); in.Unread != 0 || len(in.Items) != 2 || !in.Items[0].Read {
		t.Fatalf("after reading: %+v", in)
	}
	// When access is withdrawn the mention disappears, and so does one whose message is deleted.
	limited := leader.id("POST", "/api/channels", M{"name": "team", "shares": []M{share("user", a.ID, 2), share("user", r.ID, 1)}})
	m3 := a.id("POST", "/api/channels/"+limited+"/messages", M{"body": "@robert welcome to the team"})
	if get(r).Unread != 1 {
		t.Fatal("mention in a shared channel")
	}
	leader.ok("PUT", "/api/acl/channel/"+limited, []M{share("user", a.ID, 2)}, nil)
	if in := get(r); in.Unread != 0 || len(in.Items) != 2 {
		t.Fatalf("mention still shown after access was withdrawn: %+v", in)
	}
	leader.ok("PUT", "/api/acl/channel/"+limited, []M{share("user", a.ID, 2), share("user", r.ID, 1)}, nil)
	a.ok("DELETE", "/api/messages/"+m3, nil, nil)
	if in := get(r); in.Unread != 0 || len(in.Items) != 2 {
		t.Fatalf("mention of a deleted message: %+v", in)
	}
	// Comments mention too.
	a.ok("POST", "/api/channels/"+open+"/messages", M{"body": "thanks @robert!", "parent_id": m1}, nil)
	if get(r).Unread != 1 {
		t.Fatal("mention in a comment")
	}
	r.ok("POST", "/api/notifications/read", M{}, nil)
	if get(r).Unread != 0 {
		t.Fatal("mark all as read")
	}
}

func TestInternalLinksResolveOnlyWhatTheReaderMaySee(t *testing.T) {
	e := newEnv(t)
	leader := e.user("lena", RoleLeader)
	a, b := e.user("anna"), e.user("bert")
	poll := a.id("POST", "/api/polls", M{"title": "Which day?", "kind": "single", "options": []string{"a", "b"}, "shares": all()})
	board := a.id("POST", "/api/boards", M{"title": "Secret plans", "columns": []string{"To do"}})
	dec := leader.id("POST", "/api/decisions", M{"title": "Fee", "shares": all()})
	dm := a.id("POST", "/api/dm", M{"user_ids": []string{leader.ID}})
	refs := M{"refs": []M{{"type": "poll", "id": poll}, {"type": "board", "id": board}, {"type": "decision", "id": dec}, {"type": "channel", "id": dm},
		{"type": "board", "id": "missing"}, {"type": "user", "id": a.ID}, {"type": "poll", "id": "x' OR 1=1 --"}}}
	var got []LinkRef
	b.ok("POST", "/api/links", refs, &got)
	if len(got) != 2 || got[0].Title != "Which day?" || !strings.HasSuffix(got[1].Title, " Fee") {
		t.Fatalf("bert resolves: %+v", got)
	}
	a.ok("POST", "/api/links", refs, &got)
	if len(got) != 4 || got[1].Title != "Secret plans" || got[3].Title != "" {
		t.Fatalf("anna resolves: %+v", got)
	}
	// The link itself is ordinary text in a message, and stays as written.
	ch := leader.id("POST", "/api/channels", M{"name": "links", "shares": all()})
	leader.ok("POST", "/api/channels/"+ch+"/messages", M{"body": "Vote here: [[poll:" + poll + "]]"}, nil)
}
