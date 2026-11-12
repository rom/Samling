# Design decisions

Short records of the choices that shape Samling: what was decided, why, and
what it costs.

## D1 — One Go binary with the client embedded

**Decision.** Server, CLI, migrations and web client ship as one static
binary (`CGO_ENABLED=0`).
**Why.** A club's volunteer administrator should be able to install and
upgrade by copying one file. Static linking makes the systemd sandbox and
Landlock rules simple (no shared libraries, no interpreter).
**Trade-off.** Client changes require a new binary.

## D2 — Pure Go SQLite (modernc.org/sqlite), not SQLCipher

**Decision.** SQLite through a cgo-free driver is the default database.
**Why.** No C toolchain, trivially cross-compiled, keeps D1.
**Trade-off.** SQLCipher (transparent database encryption) needs cgo and is
therefore not used. Instead Samling encrypts what matters at application
level — PII fields, secrets, every file, backups, exports — and delegates
whole-database encryption to the layer below (LUKS/fscrypt, or the
PostgreSQL platform). Message texts and titles are therefore in clear inside
the database file. See [security.md](security.md#encryption-at-rest).

## D3 — A portable SQL subset instead of an ORM

**Decision.** Hand-written SQL restricted to what SQLite and PostgreSQL
share: `?` placeholders rebound for PostgreSQL, TEXT/INTEGER/BIGINT only,
unix timestamps, application generated ids.
**Why.** PostgreSQL support was required from the start; the same test suite
runs on both. No ORM dependency, queries are readable and auditable.
**Trade-off.** No use of database specific features (JSON columns, full text
search, row level security).

## D4 — One ACL table for all shareable things

**Decision.** Channels, documents, lists, polls, events, meetings, decisions
and boards share one sharing mechanism: owner + `acl` rows granting read or
write to everyone, a group, a role or a user.
**Why.** One model to understand, one dialog in the client, one SQL condition
to audit, and search can reuse it exactly.
**Trade-off.** Only two shared levels (read, write) plus owner; no per-field
or per-item permissions.

## D5 — Administrators get no implicit access to content

**Decision.** Being an administrator does not let you read other people's
private lists, events, documents or direct messages. `content.moderate`
gives management rights only on resources that are already shared.
**Why.** Privacy by default; "administrator" in a club is an operational
role, not a licence to read private notes. Shared material can still be
moderated and never becomes orphaned.
**Trade-off.** A lost private resource cannot be recovered by an admin
through the UI. (Someone with database access can still read unencrypted
content — see D2.)

## D6 — Anonymous polls store unlinkable ballots

**Decision.** For anonymous polls a ballot row has no user id, no timestamp
and a fully random id; participation is recorded separately in
`poll_voters`. Three things keep the two from being matched: no "voted"
entry is written to the person's history, the voter list of the poll is
rewritten in a fixed order at every vote (so row order says nothing about
the order of voting), and the running tally is hidden until the poll closes.
Backups list rows in key order.
**Why.** A copy of the database or a backup must not tell who voted for
what, while double voting is still prevented.
**Trade-off.** An anonymous vote cannot be changed afterwards (`allow_change`
is forced off) and results are only shown after closing. Somebody who can
watch the database between every two votes can still link them. With very few voters the result itself can reveal votes —
that is inherent, not a storage issue.

## D7 — Search by scanning with RE2 instead of a full-text index

**Decision.** Search loads the rows the user may read (per type, within the
time range) and matches them in Go. Plain, wildcard and regex queries are all
compiled to Go regular expressions.
**Why.** Wildcards and regular expressions were required; full-text indexes
do not support them. Go's RE2 engine runs in linear time, so hostile patterns
cannot cause catastrophic backtracking. The code is identical for both
databases, and access control is applied in the same SQL as everywhere else.
**Trade-off.** Cost grows linearly with data. Limits are in place (50 hits
per type, 50 000 rows scanned per query). Good for club-sized data; a larger
installation should add an index prefilter (SQLite FTS5 / PostgreSQL
`tsvector` or trigram) in front. Document contents are not searched.

## D8 — Preact + htm without a build step

**Decision.** The client is written as native ES modules using Preact with
htm templates; the single dependency is vendored.
**Why.** A component framework gives a responsive, app-like client, while
"no bundler, no node_modules" keeps the supply chain tiny and lets the server
enforce a strict CSP (htm needs no `eval`, unlike runtime template
compilers).
**Trade-off.** No TypeScript, no tree shaking; more requests on first load
(mitigated by HTTP/2 and small files).

## D9 — Server-Sent Events, not WebSockets

**Decision.** Live updates are content-free change notifications over SSE;
all data still flows through the normal, access-checked REST endpoints.
**Why.** Standard library only, works through proxies, reconnects by itself,
and there is exactly one code path that returns data.
**Trade-off.** One extra request per update; the hub is in-process, so the
server is single-instance.

## D10 — Database independent, self-contained encrypted backups

**Decision.** A backup is a passphrase-encrypted archive with every table as
JSON lines, all file contents, and the master key.
**Why.** One file plus one passphrase is enough to rebuild the system on a
new machine — the realistic disaster for a club. The same format migrates
between SQLite and PostgreSQL.
**Trade-off.** The passphrase is mandatory and protects everything,
including the master key; a weak passphrase is a real risk (Argon2id slows
guessing). Restore requires an empty target; backups of earlier schema
versions are accepted.

## D11 — Deleting an account anonymises it

**Decision.** Deleting a user erases personal data and credentials, removes
their private content, and keeps shared contributions under a "deleted user"
label.
**Why.** Satisfies the right to erasure while keeping the organisation's
records (protocols, decisions, discussions) intact and consistent.
**Trade-off.** Text the person wrote in shared places remains; it must be
removed manually if it contains personal data itself.

## D12 — Decisions are append-only

**Decision.** Decisions cannot be deleted or silently edited. A change
(including revoking) creates a revision with a mandatory comment.
**Why.** A decision log is only trustworthy if its history is visible.
**Trade-off.** Mistakes stay in the history.

## D13 — YAML file, environment, flags

**Decision.** Configuration is a YAML file; `SAMLING_*` environment variables
override it; `--port`, `--host`, `--data-dir` override both. Unknown YAML
keys are errors.
**Why.** A readable, commented file for administrators; environment
variables for secrets and containers; flags for quick overrides such as the
port.
**Trade-off.** Three places to look — `samling check-config` prints the
effective listen address, database and storage.

## D14 — TLS 1.3 only

**Decision.** The server offers TLS 1.3 and nothing older, and refuses to
start without a certificate unless plain HTTP is explicitly enabled.
**Why.** All TLS 1.3 cipher suites are AEAD with forward secrecy; there is
nothing to misconfigure. Every current browser and OS supports it.
**Trade-off.** Very old clients cannot connect.

## D15 — Landlock inside the process

**Decision.** After opening its listener, database and storage, the server
restricts itself with Landlock: only its data directory is writable, a few
system paths are readable, and — in SQLite + local file mode — outbound TCP
is blocked.
**Why.** Defence in depth that travels with the binary and works even when
the server is not started by the provided systemd unit.
**Trade-off.** Needs a recent kernel (file rules: 5.13+, network rules:
6.7+). The default mode is best effort; `security.landlock: strict` refuses
to start without it.

## D16 — Field encryption with a blind index for PII

**Decision.** E-mail, phone and TOTP secrets are encrypted with AES-256-GCM
under a key derived from the master key, with the column name and the row
id bound as additional data. E-mail additionally gets a keyed hash (blind index).
**Why.** A leaked database file or SQL dump does not expose contact details
or MFA secrets. The blind index allows equality lookups without decrypting.
**Trade-off.** Contact search decrypts in the application (fine at this
scale). Losing the master key loses this data.

## D17 — Sessions in the database, tokens hashed

**Decision.** Opaque random session tokens, stored only as SHA-256 hashes,
instead of self-contained tokens (JWT).
**Why.** Immediate revocation (logout, password change, account disable),
nothing to forge, and a database leak does not leak usable sessions.
**Trade-off.** One indexed lookup per request.

## D18 — Time-ordered identifiers

**Decision.** Row ids are 48 bits of millisecond time plus 80 random bits
(as in UUIDv7), written as hex. Lists and paging order by id.
**Why.** Timestamps are stored in whole seconds; two chat messages or two
revisions written within the same second need a stable order, and paging by
"older than this id" never skips or repeats rows. 80 random bits keep ids
unguessable.
**Trade-off.** An id reveals when its row was created (the timestamp is
stored next to it anyway). Anonymous ballots therefore use fully random ids.

## D19 — Same-origin check in addition to the CSRF token

**Decision.** State-changing browser requests must carry the site's own
`Origin`; bearer-token requests are exempt.
**Why.** Defence in depth: it also covers the unauthenticated sign-in
endpoint and does not depend on the client handling the token correctly.
**Trade-off.** A reverse proxy must forward the `Host` header (or
`X-Forwarded-Host`), and other web origins cannot call the API at all — by
design; native apps use tokens.

## D20 — Passkeys as a sign-in method that includes the second factor

**Decision.** Passkeys are discoverable WebAuthn credentials with user
verification required, registered after re-entering the password. Signing in
with one needs nothing else and satisfies an MFA requirement.
**Why.** Phishing-resistant and easier than password plus code. Requiring
user verification makes the passkey two factors (possession and PIN or
biometrics) in one step.
**Trade-off.** Bound to the site's host name: they need https and a stable
`server.public_url`, and are lost if the address changes. The password
remains as a fallback, protected by TOTP where enabled.

## D21 — Re-authentication for changes to sign-in methods

**Decision.** The current password is required to change the password, to
add or remove a second factor and to replace recovery codes; failures count
towards the lockout and a lockout ends all sessions.
**Why.** A stolen session must not be enough to take over the account
permanently or to lock its owner out.
**Trade-off.** One more password prompt in rare flows.

## D22 — Uploads are opaque downloads; the client validates request paths

**Decision.** Every upload is served as `application/octet-stream` with
`attachment`. The client only issues API requests whose path consists of
plain segments and ignores routes that do not.
**Why.** The audit showed that an uploaded `.json` file plus a link with an
encoded `../` in its fragment made an administrator's browser render
attacker-chosen data and send a state-changing request to another endpoint.
Either measure alone breaks that chain; both are kept.
**Trade-off.** Files cannot be previewed in the browser.
