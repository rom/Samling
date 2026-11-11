# Security

This document describes what Samling protects, how, and what is left to the
operator. Source references are given so claims can be checked.

## Threat model

Assets: members' personal data (names, e-mail, phone), private and shared
content (messages, documents, protocols, decisions, votes), credentials and
the integrity of the decision log.

| Threat | Main protections |
|--------|------------------|
| Outsider on the network | TLS 1.3 only, HSTS, no unauthenticated API except login |
| Password guessing / credential stuffing | Argon2id, rate limit per address, account lockout, optional or enforced TOTP MFA |
| Malicious or curious member | Per-resource sharing checked on every request, existence not revealed, search restricted by the same rules |
| Cross-site attacks (XSS, CSRF, clickjacking) | Strict CSP without inline script, SameSite=Strict cookie + CSRF header, frame denial, uploads never rendered in the app's origin |
| Malicious upload | Size limit, sanitised names, forced download, sandbox CSP |
| Stolen disk, database file or backup | Encrypted files, encrypted PII fields, encrypted backups; hashed passwords and session tokens. *Not* covered without disk encryption: message texts and titles in the database (see below) |
| Curious administrator | No implicit access to private content or direct messages; audit log of administrative actions; anonymous ballots are unlinkable even in the database |
| Compromise of the server process | No privileges, systemd sandbox, seccomp, Landlock, optional SELinux domain |

Out of scope: a malicious operating system administrator (root can read the
master key), and end-to-end encryption between members.

## Authentication

* **Passwords** — Argon2id (memory 19 MiB, 2 iterations, 1 thread, 16 byte
  salt, PHC string format). Minimum length `security.min_password_length`
  (default 12), maximum 256 bytes. `internal/crypto/crypto.go`.
* **Unknown accounts** take the same time and return the same error as wrong
  passwords (a dummy hash is verified).
* **Lockout** — 10 consecutive failures lock the account for 15 minutes. An
  administrator can unlock it.
* **Rate limits** — 30 password checks per minute per client address
  (sign-in, password change, MFA disable) and 3000 API requests per minute
  per address. The limits are sized so that a whole club behind one address
  (a meeting on shared Wi-Fi) is not locked out; targeted guessing is
  stopped by the per-account lockout above. Behind a reverse proxy set `server.trusted_proxy: true`,
  otherwise all users share the proxy's address.
* **Two-factor (TOTP, RFC 6238)** — users can enable it; administrators can
  require it per account (the account is then limited to the enrolment
  endpoints until done) and reset it. A code cannot be replayed: the last
  accepted time step is stored. The secret is stored encrypted. There are no
  recovery codes: a locked-out user needs an administrator
  (or `samling admin passwd --reset-mfa` for the last administrator — MFA of that account
  must then be reset by another administrator or in the database).
* **Forced password change** — accounts created or reset by an administrator
  must set a new password before anything else works.
* **Sessions** — 256-bit random token; only its SHA-256 hash is stored.
  Lifetime `security.session_hours` (default 14 days). Changing a password
  ends all other sessions; an administrative reset or disabling ends all.
  Expired sessions are purged hourly.
  * Browser: cookie `HttpOnly`, `SameSite=Strict`, `Secure` and the `__Host-`
    name prefix when served over TLS.
  * **CSRF**: every non-GET request with a cookie session must carry the
    session's CSRF token in the `X-CSRF-Token` header.
  * Native apps: `Authorization: Bearer <token>`. Cookie sessions and bearer
    sessions are distinct kinds and not interchangeable.

## Authorisation

### Permissions (roles)

| Permission | Allows |
|------------|--------|
| `users.manage` | Create, rename, disable, delete accounts; reset passwords; require/reset MFA; see all contact details |
| `roles.manage` | Create and edit roles, assign roles, manage global groups |
| `logs.view` | Read the audit log |
| `backup.manage` | Create backups and bulk exports |
| `channels.create` | Create chat channels |
| `content.moderate` | Manage any *shared* resource (not private ones, not direct messages) |
| `decisions.create` | Record and revise decisions |
| `meetings.create` | Plan meetings |

Built-in roles: **admin** (all permissions, cannot lose them), **leader**
(`channels.create`, `content.moderate`, `decisions.create`,
`meetings.create`), **member** (none). Custom roles can combine permissions.

Safeguards: an account manager without `roles.manage` cannot modify, reset or
delete accounts that hold permissions (no takeover of an administrator);
the system refuses any change that would leave it without an account holding
`users.manage` and `roles.manage`; nobody can delete or disable themselves.

Everything else — lists, boards, events, documents, polls, direct messages,
local groups — any signed-in member may create and share.

### Sharing levels

| Level | Meaning |
|-------|---------|
| none | The resource appears not to exist (404) |
| 1 read | See it (and vote, for polls) |
| 2 write | Contribute: post, add items/cards/versions, edit an event or protocol |
| 3 manage | Owner (or moderator of a shared resource): edit, change sharing, delete |

Subjects: everyone, a global group, one of *your own* local groups, a role, a
user. See [architecture.md](architecture.md#access-model).

## Personal data (PII)

* **Field encryption** — e-mail, phone and TOTP secret are encrypted with
  AES-256-GCM. The key is derived from the master key with HKDF-SHA256 (one
  sub-key per purpose: fields, blind index, files). The column name is bound
  as additional authenticated data, so a ciphertext cannot be moved to another
  column. A keyed hash (blind index) of the e-mail is stored for equality
  lookups without decryption (reserved; not yet used by a feature).
* **Visibility** — each user decides whether their contact details are shown
  to other members. Hidden details are neither listed nor searchable, except
  for holders of `users.manage`.
* **Data minimisation** — IP addresses are stored only with security events
  (the audit log), never with ordinary activity. Activity and audit entries
  older than two years are deleted automatically.
* **Access and portability** — every user can download their own data
  (`GET /api/me/export`).
* **Erasure** — deleting an account erases name, contact details,
  credentials, sessions, memberships and private content; shared
  contributions remain, attributed to a deleted user.
* **Logs** — the server log contains request paths and error references,
  not request bodies. Failed logins are recorded in the audit log with the
  attempted user name (truncated).

## Encryption at rest

| Data | Protection |
|------|------------|
| Uploaded files (attachments, document versions) | Always encrypted by Samling (AES-256-GCM, streaming) before reaching disk or S3 |
| E-mail, phone, TOTP secrets | Encrypted per field |
| Passwords | Argon2id hashes |
| Session tokens | SHA-256 hashes |
| Backups | Always encrypted with a passphrase |
| Member export | Optionally encrypted with a passphrase |
| Everything else in the database (messages, titles, notes, decisions, names, user names) | **Not encrypted by Samling** |

**Encrypting the whole database** is delegated to the platform:

* SQLite: put the data directory on an encrypted volume (LUKS/dm-crypt) or
  an fscrypt-encrypted directory. SQLite is opened with `secure_delete`, so
  deleted content is overwritten in the database file.
* PostgreSQL: use an encrypted volume for the cluster, or a managed service
  with encryption at rest; use TLS to the database
  (`sslmode=verify-full`) when it is on another host.
* S3: files are already encrypted by Samling; server-side encryption can be
  enabled in addition.

### File format (files, backups, exports)

`internal/crypto/stream.go`. A stream is `"SMLSTRM1"`, a random 7-byte nonce
prefix, then chunks of up to 64 KiB plaintext, each sealed with AES-256-GCM
and prefixed with its length. The nonce is prefix ‖ 32-bit counter ‖ last
flag. Chunks therefore cannot be reordered, and truncation is detected
because the final chunk must carry the flag. Passphrase protected files
(`"SMLPASS1"` + 16 byte salt + stream) derive the key with Argon2id
(64 MiB, 3 iterations).

### Key management

* The **master key** is 32 random bytes (base64 in a file). Installed by
  `setup.sh` as `/etc/samling/master.key` (root only) and passed to the
  service as a systemd credential; in development it is created as
  `<data_dir>/master.key` (0600). A key file readable by group/others is
  refused.
* A master key is only ever generated for a brand-new installation. If the
  database already contains accounts and the key file is missing, the server
  and CLI refuse to start instead of silently creating a new key (which
  would make the existing encrypted data unreadable).
* Losing the master key makes encrypted fields and all files unreadable.
  Every backup contains it (inside the passphrase encryption), so **a backup
  plus its passphrase restores everything, and the passphrase must be
  protected accordingly**.
* Key rotation is not automated in the MVP.

## Transport

* TLS 1.3 only (`MinVersion: tls.VersionTLS13`); the server will not start
  without a certificate unless `server.insecure_http` is set explicitly.
* HSTS (`max-age` two years, `includeSubDomains`) on TLS responses.
* Certificates are re-read when the file changes — renewals need no restart.
* Reverse proxy mode: `insecure_http: true` + `trusted_proxy: true`, proxy on
  the same host, bind Samling to `127.0.0.1`. The proxy must set
  `X-Forwarded-Proto` and append to `X-Forwarded-For`, and should itself be
  limited to TLS 1.3.

## Web application hardening

* **CSP**: `default-src 'self'; script-src 'self'; style-src 'self';
  img-src 'self' data: blob:; connect-src 'self'; object-src 'none';
  frame-ancestors 'none'; base-uri 'none'; form-action 'self'` — no inline
  script, no `eval`, no third-party origins. The client loads nothing from
  other sites.
* **Headers**: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: no-referrer`, `Permissions-Policy` (camera, microphone,
  geolocation, payment, usb off), `Cross-Origin-Opener-Policy` and
  `Cross-Origin-Resource-Policy: same-origin`, `Cache-Control: no-store` on
  the API.
* **Downloads**: always `Content-Disposition: attachment` with
  `Content-Security-Policy: default-src 'none'; sandbox`; the content type is
  derived from the file extension, never taken from the uploader. An uploaded
  HTML or SVG file can therefore not run script in Samling's origin.
* **Uploads**: size limit (`storage.max_upload_mb`), file names stripped of
  paths and control characters, stored under random keys (no path built from
  user input). A file can only be attached by the person who uploaded it.
* **Links**: only `http`, `https` and `mailto` URLs are accepted for events
  and meetings.
* **CSV exports** neutralise cells starting with `= + - @` (formula
  injection).
* **Input limits**: JSON bodies ≤ 1 MiB, unknown fields rejected, lengths
  capped per field, .ics imports ≤ 5 MiB / 2000 events.
* **Search** uses RE2: linear time, no ReDoS; queries ≤ 300 characters.
* **Cross-origin requests**: in addition to the CSRF token and
  `SameSite=Strict` cookies, every state-changing browser request must carry
  this site's own `Origin` (and must not be marked `Sec-Fetch-Site:
  cross-site`); otherwise it is refused with 403 `cross_origin`. A reverse
  proxy must therefore pass the `Host` header through unchanged (or set
  `X-Forwarded-Host` together with `server.trusted_proxy`).
* **SQL**: parameterised statements throughout; identifiers are never taken
  from user input.
* **Errors**: internal errors return only a reference id.

## Operating system hardening

### systemd (`deploy/systemd/samling.service`)

| Directive | Effect |
|-----------|--------|
| `User=samling`, `NoNewPrivileges=yes`, `CapabilityBoundingSet=` | Unprivileged, cannot gain privileges, no capabilities (a drop-in adds only `CAP_NET_BIND_SERVICE` for ports < 1024) |
| `LoadCredential=master.key:…` | Master key stays root-only on disk |
| `ProtectSystem=strict`, `ReadWritePaths=/var/lib/samling` | Whole file system read-only except the data directory |
| `ProtectHome`, `PrivateTmp`, `PrivateDevices`, `ProtectProc=invisible`, `ProcSubset=pid` | No home directories, private /tmp, no devices, no view of other processes |
| `ProtectKernelTunables/Modules/Logs`, `ProtectControlGroups`, `ProtectClock`, `ProtectHostname` | Kernel interfaces off limits |
| `RestrictNamespaces`, `RestrictRealtime`, `RestrictSUIDSGID`, `LockPersonality`, `MemoryDenyWriteExecute` | No namespaces, no realtime scheduling, no setuid files, no W+X memory |
| `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX` | No raw, netlink or packet sockets |
| `SystemCallFilter=…`, `SystemCallArchitectures=native` | seccomp allow-list, see below |
| `UMask=0077`, `LimitCORE=0` | Private files, no core dumps (which could contain keys) |

`systemd-analyze security` rates the unit **1.2 (OK)**.

### seccomp

Applied by systemd from the unit: `@system-service` plus the three Landlock
calls, minus `@privileged` and `@resources`; denied calls return `EPERM`.
Details in [`deploy/seccomp/README.md`](../deploy/seccomp/README.md).

### Landlock (`internal/sandbox`)

Applied by the server to itself once the listener, database and storage are
open — also when it is not started by systemd:

* read/write: the data directory (and the SQLite directory, or the
  PostgreSQL socket directory);
* read only: the certificate directories (including the Let's Encrypt
  `archive` target of symlinks), CA certificates, time zone data,
  `resolv.conf`/`hosts`, the master key and credential directory;
* everything else on the file system: no access;
* with SQLite and local file storage, TCP is limited to binding the
  configured port — **the process cannot open outbound connections**.
  With PostgreSQL or S3 the network is not restricted by Landlock.

Modes (`security.landlock`): `on` (best effort for the running kernel),
`strict` (refuse to start without Landlock), `off`. The start-up log line
contains the applied state.

### SELinux

`deploy/selinux/` contains a policy module that confines the service to
`samling_t` (read `/etc/samling`, write `/var/lib/samling`, bind its port,
optional booleans for PostgreSQL and S3). **It has not been compiled or
tested**; follow its README and start in permissive mode.

## Audit log

Security relevant events are stored with time, account and client address
and are visible to holders of `logs.view`: sign-ins, failed sign-ins,
sign-outs, password and MFA changes, account/role/global group
administration, sharing changes, deletion of channels/polls/meetings,
moderator deletions of messages, decisions created or revised, meetings
finalised or reopened, backups and exports. Ordinary activity (posting,
editing a list…) is only visible to the person who did it, as personal
history.

## Backup and restore security

* Backups are created by holders of `backup.manage` (web) or on the server
  (CLI). The passphrase must be at least 12 characters; choose a long random
  one — it protects the master key too.
* A backup is authenticated: a wrong passphrase or any modification makes
  restore fail before or while reading, and restore runs in one database
  transaction.
* Restore is only possible from the server's command line, into an empty
  installation with the same schema version.
* Sessions are not part of a backup: everyone signs in again after a restore.

## Known limitations

* No built-in whole-database encryption (see above).
* No MFA recovery codes; no WebAuthn/passkeys.
* No automated master key rotation.
* Rate limiters and the event hub are in memory: single instance, counters
  reset on restart.
* Active sessions have a fixed lifetime; there is no idle timeout and no
  "sign out everywhere" button besides changing the password.
* The S3 back-end and the SELinux module are untested.
* With `security.landlock: on`, a kernel without Landlock support runs
  without the Landlock sandbox. The start-up log line states what is really
  enforced (`landlock: "ABI v7: filesystem restricted, outbound TCP
  blocked"` or `unavailable ... not sandboxed`); use `strict` to make a
  missing sandbox an error.
* No external security audit has been performed.

## Hardening checklist for operators

1. Use a real certificate (`scripts/letsencrypt.sh`) and keep the default
   TLS termination in Samling, or a TLS 1.3 reverse proxy on the same host.
2. Install with `scripts/setup.sh` so the hardened unit and the root-only
   master key are used.
3. Put `/var/lib/samling` (and backups) on an encrypted volume.
4. Set `security.landlock: strict` on kernels that support it.
5. Require MFA for every account with a role above member.
6. Copy backups **and separately** the backup passphrase off the machine;
   test a restore.
7. Keep at least two administrator accounts.
8. On SELinux systems, install and test the policy module.
9. Keep the host patched; restrict SSH; firewall everything but the
   service port (and port 80 during certificate issuance).
10. Review the audit log regularly.
