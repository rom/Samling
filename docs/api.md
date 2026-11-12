# REST API

The web client and future native apps use the same JSON API under `/api/`.
The authoritative list of routes is `internal/app/routes.go`.

## Conventions

* JSON request and response bodies (`Content-Type: application/json`), UTF-8.
  Request bodies are limited to 1 MiB and unknown fields are rejected.
* Identifiers are opaque strings. Times are unix seconds (UTC).
* Errors: HTTP status plus `{"error": "<code>"}`, for example
  `{"error": "not_found"}`. Internal errors return
  `{"error": "internal", "ref": "<id>"}`; the id appears in the server log.
* Common codes: `unauthorized` (401), `forbidden` (403), `csrf` (403),
  `password_change_required` / `mfa_setup_required` (403, see below),
  `not_found` (404; also returned when you lack read access),
  `too_large` (413), `rate_limited` (429), and validation codes such as
  `title_required`, `invalid_share`, `invalid_time` (400).
* Successful writes return `{"ok": true}` or the new object's `{"id": …}`.

## Authentication

### Browser (cookie)

`POST /api/auth/login` with `{"username", "password", "code"}` (`code` = TOTP
code, only when MFA is enabled; without it the answer is 401
`mfa_required`). The response sets an `HttpOnly`, `SameSite=Strict` session
cookie and returns `{"csrf": "<token>"}`. Every non-GET request must send the
token in the `X-CSRF-Token` header. `GET /api/me` returns it again.

### Native apps (bearer token)

Send `"token": true` in the login request. No cookie is set; the response is
`{"token": "<token>"}`. Send it as `Authorization: Bearer <token>` on every
request. No CSRF header is needed. `POST /api/auth/logout` revokes it. Store
the token in the platform's secure storage (Keychain / Keystore).

### Pending steps

While an account must change its password or enrol MFA, every endpoint except
those marked *limited* below answers 403 with `password_change_required` or
`mfa_setup_required`. `GET /api/me` reports the step in `pending`.

## Sharing objects

Resources that can be shared accept `"shares"` on creation and expose them
through `/api/acl/{type}/{id}`. A share is

```json
{"type": "all | group | role | user", "id": "<subject id, empty for all>", "level": 1}
```

with level 1 = read, 2 = write. Types: `channel`, `document`, `list`, `poll`,
`event`, `meeting`, `decision`, `board`. Responses that include `level`
report the caller's level (1 read, 2 write, 3 manage).

## Endpoints

"Level" is the caller's level on the resource; "perm" is a role permission.

### Account and session

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| POST | `/api/auth/login` | public | Sign in |
| POST | `/api/auth/logout` | signed in (limited) | Sign out, revoke the session |
| GET | `/api/me` | signed in (limited) | Profile, roles, permissions, CSRF token, pending step, organisation name, limits |
| PATCH | `/api/me` | signed in (limited) | Update display name, e-mail, phone, contact visibility, locale (`sv`/`en`), settings, `avatar` (`""`, `preset:1`–`preset:24`, or `file:<id>` of an own uploaded picture), `profile` (`address`, `interests`, `specialities`, `about`) and `profile_shared` (which of those fields other members may see) |
| PUT | `/api/me/channel-order` | signed in | The caller's own order of the channels (`ids`). `GET /api/channels` returns the channels in that order; channels not listed follow by name |
| GET | `/api/notifications` | signed in | The caller's mentions: `unread` and the latest 50 `items` (`actor_id`, `channel_id`, `channel` name, `message_id`, `snippet`, `read`). Only mentions in conversations the caller can read at the time of the request |
| POST | `/api/notifications/read` | signed in | Mark as read: `ids`, or all when empty |
| POST | `/api/links` | signed in | Resolve internal links: `refs` (`type`: channel, document, list, poll, event, meeting, decision, board; `id`), at most 100. Also `message` (written `[[message:id]]` or as the address `#/chat/<channel>/<message>`). Returns `type`, `id`, `title` for those the caller can read; unreadable and missing ones are omitted alike. A message also comes with `channel_id`, `user_id`, `at`, `text` (at most 600 bytes) and `files`; a document with `folder`, `version`, `file_id`, `name`, `mime`, `size` |
| GET | `/api/me/blocks` | signed in | Ids of the people whose direct messages the caller has blocked |
| PUT / DELETE | `/api/me/blocks/{id}` | signed in | Block / stop blocking direct messages from a person |
| POST | `/api/me/password` | signed in (limited) | Change password (`current`, `new`) |
| POST | `/api/me/mfa/setup` | signed in (limited) | Start TOTP enrolment (body: `password`): secret, `otpauth://` URI, QR code (PNG data URI) |
| POST | `/api/me/mfa/enable` | signed in (limited) | Confirm enrolment with a code |
| POST | `/api/me/mfa/disable` | signed in | Disable MFA (password required; refused when MFA is required) |
| POST | `/api/me/mfa/recovery-codes` | signed in | Replace the recovery codes (body: `password`); returns the new codes once |
| GET | `/api/me/passkeys` | signed in (limited) | The caller's passkeys |
| POST | `/api/me/passkeys/begin` | signed in (limited) | Start a passkey registration (body: `password`): `ceremony` id and WebAuthn creation `options` |
| POST | `/api/me/passkeys/finish` | signed in (limited) | Complete it: `ceremony`, `name`, `credential` (the browser's attestation response as JSON) |
| POST | `/api/me/passkeys/{id}/delete` | signed in | Remove a passkey (body: `password`) |
| POST | `/api/me/sessions/end-others` | signed in | Sign out everywhere else |
| POST | `/api/auth/passkey/begin` | public | Start a passkey sign-in: `ceremony` id and WebAuthn request `options` |
| POST | `/api/auth/passkey/finish` | public | Complete it: `ceremony`, `credential` (assertion response), optional `token: true`; answers like the password login |
| GET | `/api/me/history` | signed in | Own activity (`?before=<id of the oldest entry shown>` for paging, 200 per page) |
| GET | `/api/me/export` | signed in | Download own personal data (JSON) |
| GET | `/api/events` | signed in | Server-Sent Events stream of change notifications |

### People, groups, roles

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/users` | signed in | Member directory: `avatar`; contact details and `profile` fields only when their owner shares them (always for the person and for `users.manage`) |
| GET | `/api/groups` | signed in | Global groups and own local groups, with members |
| POST | `/api/groups` | signed in; `roles.manage` for scope `global` | Create a group (`scope`: `global` or `local`) |
| PUT | `/api/groups/{id}` | owner (local) / `roles.manage` (global) | Rename, set members |
| DELETE | `/api/groups/{id}` | as above | Delete |
| GET | `/api/roles` | signed in | Roles and the list of permissions |
| POST | `/api/roles` | perm `roles.manage` | Create a role |
| PUT | `/api/roles/{id}` | perm `roles.manage` | Update a role |
| DELETE | `/api/roles/{id}` | perm `roles.manage` | Delete a custom role |

### Administration

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/admin/users` | perm `users.manage` | All accounts with status |
| POST | `/api/admin/users` | perm `users.manage` | Create an account (temporary password; roles need `roles.manage`) |
| PATCH | `/api/admin/users/{id}` | perm `users.manage` | Rename, display name, e-mail, `disabled`, `mfa_required`, `reset_mfa`, `unlock`, `roles` (needs `roles.manage`) |
| POST | `/api/admin/users/{id}/password` | perm `users.manage` | Set a temporary password |
| DELETE | `/api/admin/users/{id}` | perm `users.manage` | Delete (anonymise) an account |
| POST | `/api/admin/users/{id}/sign-out` | perm `users.manage` | End every session of the account |
| GET | `/api/admin/logs` | perm `logs.view` | Audit log (`?before=<entry id>`, `?action=` prefix, `?user=`) |
| POST | `/api/admin/backup` | perm `backup.manage` | Stream an encrypted backup (`{"passphrase"}`) |
| POST | `/api/admin/export` | perm `backup.manage` and `users.manage` | Members CSV (`{"kind": "members", "passphrase": optional}`) |

### Sharing

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/acl/{type}/{id}` | level read | List shares |
| PUT | `/api/acl/{type}/{id}` | level manage | Replace shares (array of share objects) |

### Chat

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/channels` | signed in | Visible channels and direct messages with unread counts |
| POST | `/api/channels` | perm `channels.create` | Create a channel (`name`, `topic`, `shares`) |
| PATCH | `/api/channels/{id}` | level manage (owner, or `content.moderate` for a shared channel) | Set `name`, `topic` and `description`; not for direct conversations |
| DELETE | `/api/channels/{id}` | level manage | Delete channel and messages |
| POST | `/api/dm` | signed in | Open (or find) a direct conversation (`user_ids`) |
| GET | `/api/channels/{id}/messages` | level read | Messages, oldest first (`?before=<id of the oldest message shown>`, `?limit=` ≤ 200) |
| POST | `/api/channels/{id}/messages` | level write | Post (`body`, `file_ids`, optional `parent_id` to comment on a message in the same channel; comments are one level deep). `403 dm_blocked` in a direct conversation where a participant has blocked the caller. The body may contain the marks `**bold**`, `*italic*`, `__underline__`, internal links `[[type:id]]` and mentions `@username`; all are stored as written. Mentioned people who can read the channel get a notification (once per message; an edit notifies people it adds) |
| POST | `/api/messages/{id}/reactions` | level read | Toggle the caller's reaction (`emoji`, one of a fixed set). Messages are returned with `parent_id` and `reactions` (`emoji`, `count`, `mine`) |
| GET | `/api/channels/{id}/pins` | level read | Pinned messages |
| POST | `/api/channels/{id}/read` | level read | Mark as read |
| PATCH | `/api/messages/{id}` | author | Edit |
| DELETE | `/api/messages/{id}` | author or level manage | Delete |
| POST / DELETE | `/api/messages/{id}/pin` | level write | Pin / unpin |

### Files and documents

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| POST | `/api/files` | signed in | Upload (`multipart/form-data`, field `file`); returns id, name, size |
| GET | `/api/files/{id}` | uploader, or read access to a channel/document/attachment that references it | Download |
| GET | `/api/files/{id}/image` | as above; profile pictures: signed in | The file shown inline, only if its content is a JPEG, PNG, GIF or WebP picture (otherwise 404) |
| GET | `/api/attachments/{type}/{id}` | level read | Files attached to a `list`, `event`, `card`, `meeting`, `poll` or `decision` (a card follows its board) |
| POST | `/api/attachments/{type}/{id}` | level write (`poll`, `decision`: manage) | Attach an own upload (`file_id`), at most 20 per item |
| DELETE | `/api/attachments/{type}/{id}/{file}` | as for POST | Remove an attachment |
| GET | `/api/documents` | signed in | Visible documents |
| POST | `/api/documents` | signed in | Create from an uploaded file (`file_id`, `name`, `folder`, `description`, `shares`) |
| GET | `/api/documents/{id}` | level read | Details and version history |
| PATCH | `/api/documents/{id}` | level write | Rename, move, describe |
| DELETE | `/api/documents/{id}` | level manage | Delete |
| POST | `/api/documents/{id}/versions` | level write | Add a version (`file_id`, `comment`) |
| POST | `/api/documents/{id}/copy` | level read | Private copy of the current version |
| GET | `/api/documents/{id}/download` | level read | Download (`?version=N`, default current) |

### Lists

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/lists` | signed in | Visible lists |
| POST | `/api/lists` | signed in | Create (`title`, `kind`: `todo`/`priority`/`plain`, `shares`) |
| GET | `/api/lists/{id}` | level read | List with items |
| PATCH | `/api/lists/{id}` | level manage | Rename, change kind |
| DELETE | `/api/lists/{id}` | level manage | Delete |
| POST | `/api/lists/{id}/items` | level write | Add item (`text`, `priority` 0–3, `due_at`) |
| PATCH | `/api/lists/{id}/items/{item}` | level write | Update (`text`, `done`, `priority`, `due_at`, `position`) |
| DELETE | `/api/lists/{id}/items/{item}` | level write | Remove item |

List items accept and return `assignee_id` (the person who owns the task; empty for none).

### Polls

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/polls` | signed in | Visible polls |
| POST | `/api/polls` | signed in | Create (`title`, `kind`: `single`/`multi`/`yesno`/`ranked`/`score`, `options`, `anonymous`, `allow_change`, `max_choices`, `closes_at`, `results_visible`: `always`/`after_vote`/`after_close`, `shares`) |
| GET | `/api/polls/{id}` | level read | Poll, options, own vote, results when visible |
| POST | `/api/polls/{id}/vote` | level read | Vote: `{"choices": {"<option id>": value}}` — value 1 (single/multi/yesno), rank 1..n (ranked), 0–5 (score) |
| POST | `/api/polls/{id}/close` | level manage | Close |
| DELETE | `/api/polls/{id}` | level manage | Delete |

### Calendar

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/calendar` | signed in | Visible events overlapping `?from=`…`?to=` |
| POST | `/api/calendar` | signed in | Create (`title`, `description`, `location`, `url`, `starts_at`, `ends_at`, `all_day`, `shares`) |
| PATCH | `/api/calendar/{id}` | level write | Update (not for a meeting's event) |
| DELETE | `/api/calendar/{id}` | level manage | Delete (not for a meeting's event) |
| GET | `/api/calendar/export.ics` | signed in | iCalendar export of visible events (`?from=`, `?to=`, `?id=`) |
| POST | `/api/calendar/import` | signed in | Import an .ics body as private events |

### Meetings and decisions

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/meetings` | signed in | Visible meetings |
| POST | `/api/meetings` | perm `meetings.create` | Plan a meeting (`title`, time, `location`, `url`, `agenda` titles, `shares`) |
| GET | `/api/meetings/{id}` | level read | Meeting with agenda and protocol notes |
| PATCH | `/api/meetings/{id}` | level write, not finalised | Update details, attendees, chair, secretary, notes, status |
| DELETE | `/api/meetings/{id}` | level manage | Delete meeting and its calendar entry |
| PUT | `/api/meetings/{id}/agenda` | level write, not finalised | Replace the ordered agenda (items keep their id) |
| POST | `/api/meetings/{id}/finalize` | level manage | Lock the protocol |
| POST | `/api/meetings/{id}/reopen` | level manage | Unlock |
| GET | `/api/meetings/{id}/protocol.md` | level read | Protocol as Markdown |
| GET | `/api/decisions` | signed in | Visible decisions |
| POST | `/api/decisions` | perm `decisions.create` | Record a decision (`title`, `body`, `decided_at`, optional `meeting_id` + `agenda_item_id`, `poll_id`, `shares`) |
| GET | `/api/decisions/export.csv` | signed in | CSV of visible decisions |
| GET | `/api/decisions/{id}` | level read | Decision with revision history |
| PATCH | `/api/decisions/{id}` | level manage and perm `decisions.create` | Revise (`title`, `body`, `status`: `decided`/`revoked`/`superseded`, `comment` required) |

### Boards

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/boards` | signed in | Visible boards |
| POST | `/api/boards` | signed in | Create (`title`, `description`, `columns`, `shares`, optional `color`: `red`, `orange`, `yellow`, `green`, `teal`, `blue`, `purple`, `pink`, `gray` or empty; cards take the same `color` field) |
| GET | `/api/boards/{id}` | level read | Board with columns and cards |
| PATCH | `/api/boards/{id}` | level manage | Rename, describe |
| DELETE | `/api/boards/{id}` | level manage | Delete |
| POST | `/api/boards/{id}/columns` | level manage | Add column |
| PATCH | `/api/boards/{id}/columns/{col}` | level manage | Rename, reorder |
| DELETE | `/api/boards/{id}/columns/{col}` | level manage | Delete an empty column |
| POST | `/api/boards/{id}/cards` | level write | Add card (`column_id`, `title`, `description`, `assignee_id`, `due_at`) |
| PATCH | `/api/boards/{id}/cards/{card}` | level write | Edit or move (`column_id`, `position`) |
| DELETE | `/api/boards/{id}/cards/{card}` | level write | Delete card |

### Search

| Method | Path | Access | Purpose |
|--------|------|--------|---------|
| GET | `/api/search` | signed in | Search everything the caller can read |

Query parameters: `q` (required, ≤ 300 characters), `mode` = `plain`
(default), `wildcard` (`*`, `?`) or `regex` (RE2 syntax; `^`/`$` match at
line boundaries), `case=1` for case sensitive, `types` = comma separated
subset of `message, channel, document, list, poll, event, meeting, decision,
board, card, user`, `from` / `to` (unix seconds). Each hit has `type`, `id`,
`link` (the containing channel/list/board/meeting), `title`, `snippet`, `at`.
At most 50 hits per type.

## Live updates

`GET /api/events` (`text/event-stream`) sends

```
data: {"type":"channel","id":"…"}
```

whenever a resource the caller can read changes (types as in sharing). The
event carries no content — fetch the resource to get the new state. A
comment line (`: ping`) is sent every 25 seconds. Reconnect on disconnect.
Native apps can use the same stream with the bearer header.

Besides the resource events, a stream receives `{"type":"mention","id":"<channel id>"}` when its own account is mentioned or its mentions are marked as read. These are sent to that account's streams only.

`GET /api/calendar?event=<id>` returns just that event (if readable), which is what a link to an event opens.

## Rate limits

3000 requests per minute per client address for the API as a whole, 30 per
minute for password checks (sign-in, password change). Exceeding them returns
429 `rate_limited`.

## Cross-origin requests

State-changing requests from browsers must come from the server's own origin
(`Origin` header equal to the host; not `Sec-Fetch-Site: cross-site`),
otherwise the answer is 403 `cross_origin`. Requests that authenticate with a
bearer token are exempt, as are clients that send no `Origin` header at all.
No CORS headers are sent.

## Sign-in details

* `POST /api/auth/login` takes `username`, `password` and, when two-step
  verification is on, `code`: the 6-digit TOTP code or a recovery code.
  401 `mfa_required` asks for the code; 401 `passkey_required` means the
  account's only second factor is a passkey.
* `POST /api/me/mfa/enable` answers with `recovery_codes` (shown once).
* Changing the password, enrolling or removing a second factor and
  replacing recovery codes require the current password; 429
  `account_locked` is returned while the account is locked.
* When `security.idle_timeout_minutes` is set, web clients mark requests
  that follow real interaction with `X-User-Active: 1`; only those (and
  token sessions) extend the session.
* Limits answer 409 `limit_reached` (too many objects) or 413
  `quota_exceeded` (storage quota).
