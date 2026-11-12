# Testing

## Running the tests

```bash
make test
```

runs all Go tests on SQLite (a temporary database and blob directory per
test). To run the API suite on PostgreSQL, point it at any server where it
may create schemas (each test uses its own schema and drops it afterwards):

```bash
SAMLING_TEST_PG_DSN='postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable' make test-postgres
```

```bash
make vet
```

runs `go vet` and a `gofmt` check.

## Continuous integration

`.github/workflows/ci.yml` runs on every pull request and on pushes to the
default branch, in three jobs:

| Job | What it runs |
|-----|--------------|
| Build, vet and test (SQLite) | `go mod tidy` check, `make vet`, `make test` (Go tests + translation check), `make build`, `check-config` on the example configuration, `make shellcheck` on the shell scripts |
| API tests (PostgreSQL) | `make test-postgres` against a PostgreSQL 16 service container |
| Browser smoke test | `tests/e2e/run.sh` with Playwright/Chromium; screenshots and the server log are uploaded as an artifact when it fails |

## How the API tests work

`internal/app/app_test.go` starts the real HTTP handler (`httptest`) with a
real database, the encrypting blob store and a real master key. Test clients
sign in through the API and carry the session cookie and CSRF token exactly
like a browser, so authentication, CSRF, permissions and sharing are
exercised end to end. Nothing is mocked.

## What is covered

### `internal/crypto/crypto_test.go`

| Test | Covers |
|------|--------|
| `TestStreamRoundTrip` | Streaming encryption for sizes around the chunk boundaries (0 bytes … several chunks); ciphertext does not contain plaintext |
| `TestStreamDetectsTampering` | Bit flips, truncation at a chunk boundary, reordered chunks and a wrong key are all rejected |
| `TestPassphraseContainer` | Passphrase based encryption round trip; wrong passphrase rejected |
| `TestFieldEncryption` | Field encryption is randomised, bound to its column context, and the blind index is normalised and key dependent |
| `TestPassword` | Argon2id hashing: correct/incorrect passwords, salting, malformed hashes |
| `TestIDs` | Identifiers sort by creation time; random ids carry no time prefix |
| `TestTOTP` | RFC 6238 test vectors, clock skew window, stale and wrong codes |

### `internal/app/features_test.go`

| Test | Covers |
|------|--------|
| `TestSecurityHeadersAndStatic` | Client is served; CSP and security headers; API requires a session |
| `TestLoginCSRFAndLockout` | Cookie flags, CSRF enforcement, cookie token not usable as bearer token, bearer login for apps, identical errors for unknown/known accounts, lockout after repeated failures, logout |
| `TestForcedPasswordChangeAndMFA` | Temporary password gate, password policy, admin-required MFA gate, TOTP enrolment with QR code, replay protection, secret encrypted at rest, admin MFA reset |
| `TestAdminAndRBAC` | Permission checks on gated endpoints, custom roles, rename/disable/password reset, sessions revoked, last administrator protected, audit log content |
| `TestUserDeletionAnonymises` | Personal data and credentials erased, private content removed, shared content kept, directory updated |
| `TestSharingModel` | Private by default (also towards admins and moderators), read vs write vs manage, user/group/role shares, group membership changes, local groups invisible to others and their identity not leaked, invalid shares rejected |
| `TestChat` | Default channel, announcement channel, pinning rules, unread counts, edit/delete rules, moderation, direct messages (reuse, isolation from moderators, cannot be opened up), attachments (ownership, access through the conversation, forced download + sandbox CSP), files encrypted on disk |
| `TestDocuments` | Folder normalisation, versions and comments, download of current and old versions, read-only users cannot add versions, foreign uploads cannot be claimed, private copies survive deletion of the original, dangerous file names neutralised |
| `TestListsAndBoards` | List items (priority, done, reorder, delete), kanban columns and cards, moving cards, column rules, access for outsiders |
| `TestPolls` | All five kinds incl. validation, vote change, anonymous ballots stored without voter and time, result visibility modes, Borda and score tallies, deadline, private polls |
| `TestCalendarAndICS` | Private vs shared events, URL scheme allow-list, time window, .ics export (escaping, folding, all-day), import round trip and idempotency, parsing a foreign file with time zone and alarm |
| `TestMeetingsAndDecisions` | Meeting → calendar entry with the same audience, agenda editing, protocol export (Swedish), decision numbering and audience, finalise locks, revisions with mandatory comment, CSV export, personal history isolation |
| `TestSearch` | Results limited to readable content, case handling, wildcard, regex, plain mode not interpreting wildcards, invalid patterns, type and time filters, contact details searchable only when visible or by account managers |
| `TestBackupRestore` | Backup is encrypted, wrong passphrase and tampering rejected, restore into a fresh installation reproduces rows, PII and file contents, restore into a non-empty database refused, encrypted member export, personal data export |
| `TestPIIEncryptedAtRest` | E-mail, phone and passwords not in clear in the database, session tokens stored hashed, contact visibility in the directory |

## Verified manually during development

| Check | Result |
|-------|--------|
| TLS: connection with TLS 1.3 succeeds, a TLS 1.2-only client is refused | ok |
| Server refuses to start without a certificate unless `insecure_http` is set | ok |
| Landlock applied at start (`filesystem restricted, outbound TCP blocked` in SQLite + local file mode) | ok (kernel 6.18) |
| `samling check-config` on `samling.example.yaml` | ok |
| `scripts/gen-selfsigned-cert.sh`: P-256 key, SAN with DNS names and IPv4/IPv6, permissions, `--force` guard | ok |
| `scripts/samling-ctl.sh` as an unprivileged user with the key as credential: `check-config`, `admin create`, `admin passwd` | ok |
| `scripts/backup.sh`: timestamped files, retention (`--keep`) | ok |
| CLI `backup` → `restore` into an empty directory; master key restored | ok |
| SQLite → PostgreSQL → SQLite migration via backup/restore | ok |
| `samling decrypt` on a backup container | ok |
| systemd units: `systemd-analyze verify`; `systemd-analyze security --offline` = 1.2 (service), 1.3 (backup) | ok |
| All scripts: `shellcheck` (0.11.0), no findings | ok |

## Translations

```bash
node scripts/check-i18n.mjs
```

Checks that the Swedish and English files have identical keys and
placeholders and that every key used in the client exists (also run by
`make test` when Node.js is installed).

## Automated browser test

```bash
make e2e
```

`tests/e2e/run.sh` builds the server, starts it over plain HTTP on a scratch
data directory and runs `tests/e2e/smoke.js` (Playwright, Chromium). The
script signs in as a fresh administrator on a desktop viewport and as a
member on a phone viewport and walks through: forced password change, account
creation, roles and audit log, chat (post, link label, attachment up- and
download, pin, new channel, direct message, live update between two
browsers), documents (upload, new version, copy), lists, polls (single and
ranked, vote, close), calendar (create, .ics export and import), meetings
(agenda, minutes, decision, protocol export, finalise), decision log (revise,
decision from a poll, history), kanban (card, move buttons, drag and drop,
columns), groups (shared and private), search (wildcard, invalid regex),
personal history, settings (theme, text size, language, MFA enrolment
screen, persistence after reload), encrypted backup download, and that the
member sees shared but not private content and no administration menu. It
fails on any uncaught JavaScript error. It needs Node.js with the
`playwright` package; it is not part of `make test`.

## Manual browser checklist

After starting a development server (`make run-dev`) and creating an account:

1. Sign in, change the temporary password, switch language and theme.
2. Post in *general*, attach a file, pin the message; open a direct message.
3. Upload a document, add a second version, download both.
4. Create a shared list and a private board; move a card.
5. Create each kind of poll, vote as two users, close one.
6. Create an event, export the calendar, import the file as another user.
7. Plan a meeting with agenda, write notes, record a decision, finalise,
   export the protocol.
8. Search with plain text, a wildcard and a regular expression; narrow by
   date.
9. As administrator: create a user, require MFA, look at the audit log, take
   a backup.
10. Repeat the main steps at phone width.

## Not covered

* **S3 storage** against a live S3 service (the back-end is only compiled).
* **SELinux** policy module: not compiled, not run.
* **`setup.sh` end to end** and the service running under the systemd
  sandbox / seccomp filter (no systemd as init in the development
  environment). The unit was verified statically only.
* **Let's Encrypt** issuance and renewal (`letsencrypt.sh`).
* **Accessibility audits** with assistive technology (the client uses labels, roles, focus management and keyboard alternatives, but has not been audited), and browsers other than Chromium.
* **Load and concurrency** tests; search performance on large data sets.
* **Upgrade paths** between schema versions (there is only one so far).
* An external security review or penetration test.
