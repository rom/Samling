# Requirements and MVP status

Status values: **Implemented**, **Partial** (works, with the stated gap),
**Planned** (not in the MVP).

## Functional requirements

| # | Requirement | Status | Notes / where |
|---|-------------|--------|---------------|
| F1 | Chat channels on specific topics for leaders and members | Implemented | `internal/app/chat.go`. Channels are shared like any other resource; "read" = may read, "write" = may post. A read-for-everyone / write-for-leaders channel gives an announcement channel. |
| F2 | Direct messages | Implemented | One-to-one and small group conversations (2–20 people). Not accessible to administrators or moderators. |
| F3 | Pin messages in channels | Implemented | Anyone who may post in the channel may pin/unpin. |
| F4 | Attach documents (Excel, PowerPoint, Word, PDF, …) to messages | Implemented | Any file type, up to `storage.max_upload_mb` (default 50 MiB), up to 10 per message. Files are encrypted at rest and always served as downloads. |
| F5 | Document archive: stored, versioned, access controlled, download, upload, copy | Implemented | `internal/app/files.go`. Folders, version history with comments, download of any version, private copy. |
| F6 | Lists: private or shared; to-do, priority, plain | Implemented | `internal/app/lists.go`. Items have done state, priority, due date and order. |
| F7 | Polls and votes with different options | Implemented | `internal/app/polls.go`. Kinds: single choice, multiple choice (optional maximum), yes/no/abstain, ranked (Borda count), score (0–5). Options: anonymous, change of vote, deadline, result visibility (always / after voting / after closing). |
| F8 | Calendar with private events and events shared with persons or groups | Implemented | `internal/app/calendar.go`. Timed and all-day events. |
| F8a | Import and export calendar events | Partial | iCalendar (.ics) import and export. Recurrence rules (RRULE) are **not expanded**: a recurring event is imported as its first occurrence only, and Samling cannot create recurring events. |
| F9 | Groups and roles; global groups (admin) and local groups (private to their creator) | Implemented | `internal/app/people.go`. Local groups are invisible to everyone but their owner, also in sharing dialogs of others. |
| F10 | Search everywhere the user has access | Partial | `internal/app/search.go`. Covers messages (and attachment names), channels, documents, lists, polls, events, meetings and agenda items, decisions, boards and cards, people. Document **contents** are not indexed (names, folders and descriptions are). |
| F10a | Search for user names and contact information | Implemented | Contact details are only searchable when the person shows them, or by account managers. |
| F10b | Limit search to a time period | Implemented | `from` / `to`. |
| F10c | Advanced search: wildcards, regular expressions | Implemented | Modes plain, wildcard (`*`, `?`) and regex (RE2 syntax), optional case sensitivity, type filter. |
| F11 | Simple flow for planning meetings, agendas and protocols | Implemented | `internal/app/meetings.go`. Plan → agenda → protocol notes per item → finalise (locks). The meeting gets a calendar entry automatically. Protocol export as Markdown. |
| F12 | Decisions saved in a decision log visible to groups, roles or everyone | Implemented | Numbered per year (`2026-001`), linked to meeting agenda items or polls, audience through sharing. CSV export. |
| F13a | Personal history | Implemented | `GET /api/me/history`: the user's own actions. |
| F13b | Decision history (decisions, polls, votes) | Implemented | Decisions are never overwritten: every change is a revision with a mandatory comment. Closed polls keep their results. |
| F14 | i18n, Swedish and English, easy switching | Implemented | Client side translations; the choice is stored per user. The server answers with error codes that the client translates; the Markdown protocol export follows the user's language. |
| F15 | Settings: themes (light, dark, spring, autumn, sand, contrast), text size, accessibility | Implemented | Stored per user (`settings` in the profile) and applied by the client. |
| F16 | URLs/links and integrations with external mail and meeting apps | Partial | Links in events and meetings (http, https, mailto only), meeting links for Teams/Zoom/Meet/Jitsi and similar, `mailto:` links for people, .ics import/export for external calendars. **No** outgoing e-mail (SMTP), webhooks or calendar subscription feeds yet. |
| F17 | Kanban boards, private or shared | Implemented | Boards with columns and cards (assignee, due date, ordering, moving between columns). |
| F18 | Administrator: create, rename, delete accounts, change passwords, manage roles, enable MFA, see logs | Implemented | `internal/app/people.go`. Also disable/unlock accounts, require or reset MFA, custom roles, audit log. |
| F19 | Responsive design for phones, tablets and computers | Implemented | Single responsive web client. |
| F20a | Script to set up the system | Implemented | `scripts/setup.sh`. Not executed end-to-end in the development environment (no systemd as PID 1); its building blocks were tested individually, see [testing.md](testing.md). |
| F20b | Script to create self-signed certificates | Implemented | `scripts/gen-selfsigned-cert.sh` (tested). |
| F20c | Script to obtain and renew Let's Encrypt certificates | Implemented | `scripts/letsencrypt.sh` (certbot). Not run against Let's Encrypt in development. |

## Security requirements

| # | Requirement | Status | Notes / where |
|---|-------------|--------|---------------|
| S1 | Role based access control | Implemented | Permissions on roles plus per-resource sharing, see [security.md](security.md). |
| S2 | Backup, restore and export with the right authorisation | Implemented | Backup: web (permission `backup.manage`) or CLI. Restore: **CLI only**, into an empty installation. Exports: members CSV (admin), decisions CSV, calendar .ics, meeting protocol, personal data (JSON). |
| S3 | Protect PII | Implemented | E-mail, phone and MFA secrets are encrypted per field; users control contact visibility; IP addresses only on security events; activity older than two years is purged; deleting an account anonymises it; personal data export. |
| S4 | Possible to encrypt databases, backups, exports | Partial | Backups are always encrypted. The members export can be encrypted. Uploaded files are always encrypted. In the database, PII fields and secrets are encrypted, but **the database as a whole is not encrypted by Samling**: use disk/volume encryption (LUKS, fscrypt) or the PostgreSQL platform's encryption. See [security.md](security.md#encryption-at-rest). |
| S5 | Hardened with SELinux, seccomp, Landlock, systemd and other Linux features | Partial | systemd sandboxing and seccomp filter: `deploy/systemd/samling.service` (verified with `systemd-analyze`, exposure score 1.2). Landlock: built into the server (verified). SELinux: module provided in `deploy/selinux/` but **not compiled or tested**. No capabilities are required (one for ports below 1024). |
| S6 | TLS 1.3 | Implemented | TLS 1.3 is the minimum and only version; verified. Certificates are reloaded without restart. |
| S7 | Other good security practices | Implemented | Argon2id, TOTP MFA, lockout and rate limiting, CSRF protection, strict CSP and security headers, hashed session tokens, audit log, forced password change, safe file handling. Also: MFA recovery codes, passkeys (WebAuthn), optional idle timeout, sign out everywhere, storage quota. A security audit was carried out and its findings fixed (see [security.md](security.md) and `internal/app/security_test.go`). Known gap: no automated master key rotation. |

## Technical requirements

| # | Requirement | Status | Notes / where |
|---|-------------|--------|---------------|
| T1 | Written in Go | Implemented | Single static binary, no cgo. |
| T2 | Web application using a framework; user friendly | Implemented | Preact + htm client embedded in the binary, see [architecture.md](architecture.md). |
| T3 | SQLite first, with the ability to use PostgreSQL | Implemented | The full API test suite passes on both. |
| T4 | S3 and other storage back-ends | Partial | File storage has a back-end interface with local file system and S3 implementations. The S3 back-end (minio-go) has **not been tested against a live S3 service**. |
| T5 | Possible to have native iPhone and Android apps | Partial | The REST API, bearer token login and an installable web app (PWA) are in place. Native wrapper projects are **not included**; see [mobile.md](mobile.md). |
| T6 | Documentation of design decisions, requirements, architecture, tests, security, usage in `docs/` | Implemented | This directory. |
| T7 | The port the server runs on is configurable | Implemented | `server.port`, `SAMLING_PORT`, `--port`. |
| U1 | Welcome walkthrough at first use | Implemented | Ten-step tour that points at each part; restartable from Help. |
| U2 | Online help in all languages | Implemented | Help section with about twenty topics in Swedish and English, searchable, linked from every page. |
| U3 | User manual with screenshots | Implemented | [manual.md](manual.md); screenshots regenerated with `make manual-screenshots`. |
| T8 | YAML configuration file | Implemented | `samling.yaml`, see [`samling.example.yaml`](../samling.example.yaml). Environment variables and flags override it. |

## Known limitations of the MVP (summary)

* No whole-database encryption built in (S4).
* Recurring calendar events are not supported (F8a).
* Search does not look inside documents (F10) and scans rather than indexes,
  which is fine for a club-sized installation but not for millions of rows.
* No outgoing e-mail, push notifications or webhooks (F16).
* No native app projects (T5).
* S3 storage and the SELinux module are untested (T4, S5).
* No automated key rotation (S7).
* Restore is a CLI operation (S2).
* Threads/replies, reactions and message search highlighting are not part of
  the chat.
