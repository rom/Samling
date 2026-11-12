# Deployment and operations

## Quick install (Linux with systemd)

Build on any machine with Go, or directly on the server:

```bash
make build
```

Install as root on the server (from a checkout that contains `bin/samling`):

```bash
sudo scripts/setup.sh --port 8443 --org "Our Club" --letsencrypt club.example.org --email admin@example.org
```

or, without a public domain:

```bash
sudo scripts/setup.sh --port 8443 --org "Our Club" --selfsigned club.lan
```

`setup.sh` (see `scripts/setup.sh --help`) is safe to re-run and never
overwrites existing configuration, keys, certificates or data. It

* creates the system user `samling`;
* installs `/usr/local/bin/samling`, `/usr/local/sbin/samling-ctl` and helper
  scripts in `/usr/local/lib/samling/`;
* writes `/etc/samling/samling.yaml` (if missing) with the chosen port;
* generates `/etc/samling/master.key` (root only) and a random backup
  passphrase in `/etc/samling/backup.passphrase` (root only);
* obtains or creates a TLS certificate in `/etc/samling/tls/`;
* installs the hardened units `samling.service`, `samling-backup.service`
  and `samling-backup.timer` (daily backups to `/var/backups/samling`);
* installs the SELinux module when SELinux is enabled and
  `selinux-policy-devel` is present;
* creates the first administrator and prints a temporary password once;
* enables and starts the service.

Afterwards open `https://<host>:<port>/`, sign in as the administrator and
choose a new password. **Copy the backup passphrase and the backups to a safe
place off the machine.**

> `setup.sh` could not be run end-to-end in the development environment (no
> systemd as init). Its parts — certificate script, `samling-ctl`, backup
> script, unit files — were tested individually. Read its output on first use.

### Layout on the server

| Path | Owner / mode | Contents |
|------|--------------|----------|
| `/usr/local/bin/samling` | root 0755 | the binary |
| `/etc/samling/samling.yaml` | root:samling 0640 | configuration |
| `/etc/samling/master.key` | root 0600 | master key (given to the service as a systemd credential) |
| `/etc/samling/backup.passphrase` | root 0600 | passphrase for scheduled backups |
| `/etc/samling/tls/` | root:samling 0750 | `fullchain.pem`, `privkey.pem` (0640) |
| `/var/lib/samling/` | samling 0700 | `samling.db`, `blobs/` |
| `/var/backups/samling/` | samling 0700 | `samling-YYYYmmdd-HHMMSS.bak` |

### Running CLI commands on an installed server

The master key is root-only, so use the wrapper, which runs the command as
the `samling` user with the installed configuration and key:

```bash
sudo samling-ctl admin create --username anna --role member
```

```bash
sudo samling-ctl admin passwd --username anna
```

```bash
sudo samling-ctl check-config
```

Never run `samling` commands as root directly against `/var/lib/samling`
(files would become root-owned), and never without the key (a new, different
key would be generated in the data directory).

## Configuration

The server reads a YAML file, then environment variables, then flags
(later wins). The file is located by `--config FILE`, else `$SAMLING_CONFIG`,
else `./samling.yaml`. Unknown keys are errors. Validate with
`samling check-config`. A fully commented file is in
[`samling.example.yaml`](../samling.example.yaml).

| YAML key | Environment | Flag | Default | Meaning |
|----------|-------------|------|---------|---------|
| `org_name` | `SAMLING_ORG_NAME` | | `Samling` | Name shown in the client, exports and authenticator apps |
| `data_dir` | `SAMLING_DATA_DIR` | `--data-dir` | `./data` | Database, files, default key location |
| `server.host` | `SAMLING_HOST` | `--host` | empty (all interfaces) | Listen address |
| `server.port` | `SAMLING_PORT` | `--port` | `8443` | Listen port |
| `server.tls.cert` | `SAMLING_TLS_CERT` | | | Certificate chain (PEM) |
| `server.tls.key` | `SAMLING_TLS_KEY` | | | Private key (PEM) |
| `server.insecure_http` | `SAMLING_INSECURE_HTTP` | | `false` | Serve plain HTTP (development / behind a proxy) |
| `server.trusted_proxy` | `SAMLING_TRUSTED_PROXY` | | `false` | Trust `X-Forwarded-For` / `X-Forwarded-Proto` |
| `server.public_url` | `SAMLING_PUBLIC_URL` | | *(from the request)* | The address people use, e.g. `https://club.example.org`. Passkeys are bound to its host name: set it, and do not change it later |
| `database.driver` | `SAMLING_DB_DRIVER` | | `sqlite` | `sqlite` or `postgres` |
| `database.dsn` | `SAMLING_DB_DSN` | | `<data_dir>/samling.db` | SQLite file path or PostgreSQL URL |
| `storage.backend` | `SAMLING_STORAGE_BACKEND` | | `fs` | `fs` or `s3` |
| `storage.s3.endpoint` | `SAMLING_S3_ENDPOINT` | | | `host[:port]` |
| `storage.s3.region` | `SAMLING_S3_REGION` | | `us-east-1` | |
| `storage.s3.bucket` | `SAMLING_S3_BUCKET` | | | Required for S3 |
| `storage.s3.access_key` | `SAMLING_S3_ACCESS_KEY` | | | |
| `storage.s3.secret_key` | `SAMLING_S3_SECRET_KEY` | | | Prefer the environment |
| `storage.s3.prefix` | `SAMLING_S3_PREFIX` | | | Key prefix inside the bucket |
| `storage.s3.insecure` | `SAMLING_S3_INSECURE` | | `false` | Plain HTTP to the S3 endpoint |
| `storage.max_upload_mb` | `SAMLING_MAX_UPLOAD_MB` | | `50` | Largest upload (MiB) |
| `storage.user_quota_mb` | `SAMLING_USER_QUOTA_MB` | | `2048` | Total size of the files one account may have uploaded (MiB, 0 = unlimited) |
| `security.idle_timeout_minutes` | `SAMLING_IDLE_TIMEOUT_MINUTES` | | `0` | Sign out after this long without activity (0 = off) |
| `security.master_key_file` | `SAMLING_MASTER_KEY_FILE` | | `<data_dir>/master.key` | Master key file (a systemd credential named `master.key` takes precedence) |
| `security.min_password_length` | `SAMLING_MIN_PASSWORD_LENGTH` | | `12` | Minimum 8 |
| `security.session_hours` | `SAMLING_SESSION_HOURS` | | `336` | Session lifetime |
| `security.landlock` | `SAMLING_LANDLOCK` | | `on` | `on`, `strict`, `off` |

Boolean environment variables accept `1`, `true`, `yes`, `on`.

Restart the service after changing the configuration:

```bash
sudo systemctl restart samling
```

### Choosing the port

Set `server.port` in `/etc/samling/samling.yaml` (or pass `--port` to
`setup.sh` on first install, or `--port` / `SAMLING_PORT` when starting by
hand). Ports below 1024, such as 443, need one capability. `setup.sh` writes
this drop-in automatically when the port is below 1024; to do it by hand
create `/etc/systemd/system/samling.service.d/low-port.conf`:

```ini
[Service]
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
```

```bash
sudo systemctl daemon-reload && sudo systemctl restart samling
```

## TLS

Samling serves TLS 1.3 only and refuses to start without a certificate
unless `server.insecure_http` is enabled.

### Self-signed

```bash
sudo scripts/gen-selfsigned-cert.sh club.lan 192.168.1.10
```

Creates an ECDSA P-256 key and a certificate with the given DNS names and IP
addresses in `/etc/samling/tls/` (`--out`, `--days`, `--force` available).
Browsers warn until the certificate is trusted manually; compare the printed
fingerprint.

### Let's Encrypt

```bash
sudo scripts/letsencrypt.sh obtain club.example.org --email admin@example.org
```

Requires `certbot`, a DNS name pointing at the server and — in the default
standalone mode — TCP port 80 reachable from the internet and free during
issuance and renewal (use `--webroot DIR` if another web server owns port
80; `--staging` for a trial run). The script installs a certbot deploy hook
that copies each new certificate to `/etc/samling/tls/`. Renewal is done by
certbot's own timer (`certbot.timer`); `scripts/letsencrypt.sh renew` runs it
by hand. Samling picks up a renewed certificate within about a minute, with
no restart.

### Behind a reverse proxy

```yaml
server:
  host: "127.0.0.1"
  port: 8080
  insecure_http: true
  trusted_proxy: true
```

The proxy must run on the same host, terminate TLS (1.3), set
`X-Forwarded-Proto: https`, append the client address to `X-Forwarded-For`,
and must not buffer `GET /api/events` (Server-Sent Events). Without
`trusted_proxy`, rate limits and the audit log would see every user as the
proxy's address, and cookies would not be marked `Secure`.

## PostgreSQL

```sql
CREATE ROLE samling LOGIN PASSWORD 'choose-a-password';
CREATE DATABASE samling OWNER samling;
```

```yaml
database:
  driver: "postgres"
```

Put the connection string in the environment rather than the YAML file, for
example in a root-only file referenced by a systemd drop-in
(`EnvironmentFile=/etc/samling/samling.env`):

```
SAMLING_DB_DSN=postgres://samling:choose-a-password@127.0.0.1:5432/samling?sslmode=disable
```

Use `sslmode=verify-full` when the database is on another host. The schema is
created automatically at start-up. The same test suite passes on SQLite and
PostgreSQL. With PostgreSQL (or S3) the Landlock network restriction is not
applied, because outbound connections are needed.

## S3 compatible file storage

```yaml
storage:
  backend: "s3"
  s3:
    endpoint: "s3.eu-north-1.amazonaws.com"
    region: "eu-north-1"
    bucket: "our-club-samling"
    prefix: "files/"
```

with `SAMLING_S3_ACCESS_KEY` and `SAMLING_S3_SECRET_KEY` in the environment.
Objects are encrypted by Samling before upload, so the bucket only contains
ciphertext. **The S3 back-end has not been tested against a live service in
the MVP** — verify upload, download and a backup before relying on it.

## Backup and restore

A backup is one encrypted file with the whole database (as database
independent JSON), all file contents and the master key. The passphrase
(minimum 12 characters) is the only thing needed besides the file.

**Create**

* Scheduled: `samling-backup.timer` runs `backup.sh` daily, keeps the 14
  newest files in `/var/backups/samling`, and reads the passphrase from
  `/etc/samling/backup.passphrase`.
* By hand on the server:

```bash
sudo /usr/local/lib/samling/backup.sh --dir /var/backups/samling --keep 14
```

* From the web client: Admin → Backup (permission `backup.manage`); you
  choose the passphrase and download the file.
* Plain CLI (development, or key in the data directory):

```bash
SAMLING_BACKUP_PASSPHRASE='a long passphrase' samling backup --out samling.bak --data-dir ./data
```

Backups can be taken while the server runs.

**Restore** (CLI only, into an empty installation with the same Samling
version). On a fresh server run `setup.sh --no-admin --no-start`, then as
root:

```bash
systemctl stop samling
```

```bash
rm -rf /var/lib/samling/*
```

```bash
SAMLING_BACKUP_PASSPHRASE='the passphrase' samling restore --in samling-20260101-023000.bak --config /etc/samling/samling.yaml
```

The restore always uses the master key stored inside the backup (never a key
that happens to be configured), writes the database and re-encrypts the
files. When no key file exists at `security.master_key_file` (default
`<data_dir>/master.key`) the key from the backup is written there once the
restore has succeeded; if a *different* key is already in place the restore
stops before changing anything. With the standard layout, move the key to
where the service expects it and hand the data to the service user:

```bash
install -m 0600 -o root -g root /var/lib/samling/master.key /etc/samling/master.key && rm /var/lib/samling/master.key
```

```bash
chown -R samling:samling /var/lib/samling && systemctl start samling
```

Run `restore` as root as shown. Restore
refuses a non-empty database, a wrong passphrase, a modified file or a
backup made by a newer release (older backups are accepted and upgraded). Everyone has to sign in again afterwards.

**Disaster recovery test.** Do the procedure above on a spare machine at
least once; an untested backup is a hope, not a backup.

### Moving between SQLite and PostgreSQL

Because backups are database independent, migration is backup + restore:

1. Take a backup of the running system.
2. Stop the service, change `database.driver` / `SAMLING_DB_DSN` to the new
   (empty) database, move the old data directory away.
3. Restore as described above.

Both directions were verified in development.

### Encrypted exports

The members export (Admin → Export) can be encrypted with a passphrase.
Decrypt it with:

```bash
samling decrypt --in members.csv.enc --out members.csv
```

(`decrypt` also turns a backup file into a plain `.tar.gz` for inspection.)

## Upgrading

1. Take a backup.
2. Build or download the new binary and run `sudo scripts/setup.sh` again
   from the new checkout (it replaces the binary and units, keeps everything
   else, and restarts the service), or replace `/usr/local/bin/samling` and
   `systemctl restart samling`.

Database migrations run automatically at start-up. Downgrading is not
supported: restore the backup with the old binary instead.

## Monitoring and logs

```bash
journalctl -u samling -f
```

The server logs JSON lines to standard output. The start-up line shows the
version, listen address, TLS, database, storage and Landlock state. Internal
errors carry a `ref` that is also returned to the client. Security events are
in the audit log inside the application (Admin → Logs).

## CLI reference

```
samling <command> [flags]
```

Common flags for commands that use the configuration: `--config FILE`,
`--port N`, `--host ADDR`, `--data-dir DIR`.

| Command | Purpose |
|---------|---------|
| `serve` | Run the server |
| `admin create --username NAME [--name "Full Name"] [--email ADDR] [--role admin\|leader\|member] [--password-file FILE]` | Create an account (default role `admin`). Without `--password-file` a temporary password is generated and printed. The password must be changed at first sign-in. |
| `admin passwd --username NAME [--password-file FILE] [--reset-mfa]` | Set a new (temporary) password, sign the account out everywhere, unlock and re-enable it; `--reset-mfa` also removes two-factor authentication (lost authenticator). The recovery path for a locked-out administrator. |
| `backup --out FILE [--passphrase-file FILE]` | Write an encrypted backup |
| `restore --in FILE [--passphrase-file FILE]` | Restore into an empty installation |
| `decrypt --in FILE --out FILE [--passphrase-file FILE]` | Decrypt an encrypted export or backup container |
| `genkey` | Print a new random master key |
| `check-config` | Validate the configuration and print the effective listen address, database and storage |
| `version` | Print the version |

Passphrases come from `--passphrase-file`, else `$SAMLING_BACKUP_PASSPHRASE`,
else a prompt on standard input.

## Development

```bash
make build
```

```bash
./bin/samling admin create --username admin --data-dir ./data
```

```bash
make run-dev DEV_PORT=8080
```

The last command serves plain HTTP on `127.0.0.1:8080` with data in `./data`.

## Making a release

Releases are tagged from the default branch (`rom`) and built by GitHub
Actions.

```bash
make tag VERSION=v1.2.3
```

```bash
make publish VERSION=v1.2.3
```

* `make tag` creates the annotated tag `v1.2.3` on the current commit. It
  refuses a dirty working tree, an existing tag, a version that is not of
  the form `vX.Y.Z` (optionally `-rc.1` and the like), and a commit that is
  not on `origin/rom`.
* `make publish` pushes the tag. The workflow `.github/workflows/release.yml`
  then checks that the tag is on the default branch, runs the tests, builds
  the archives and publishes a GitHub release with generated notes. A
  version with a suffix (`v1.2.3-rc.1`) is published as a pre-release.
* `make release` builds the same archives locally into `dist/`
  (`samling-<version>-linux-amd64.tar.gz`, `…-arm64.tar.gz` and
  `SHA256SUMS`), without tagging or publishing anything. Each archive holds
  the static binary — which reports the version with `samling version` —
  the example configuration, `scripts/`, `deploy/` and the documentation.

Verify a download with `sha256sum -c SHA256SUMS` before installing it.

