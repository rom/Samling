# Samling
A simple support tool to manage a club, organisation or group of people

Samling ("gathering" in Swedish) is a self-hosted web application for the
day-to-day work of an association: talking, sharing documents, planning,
voting, meeting and keeping track of what was decided. It is one Go binary
with an embedded, responsive web client, uses SQLite by default (PostgreSQL
supported) and is built with security and privacy in mind.

**Status: MVP.** See [docs/requirements.md](docs/requirements.md) for what is
implemented, partial and planned.

## Features

* **Chat** — topic channels, direct messages, pinned messages, file
  attachments, live updates
* **Document archive** — folders, versions, access control, download, copy
* **Lists** — to-do, priority and plain lists, private or shared
* **Polls and votes** — single/multiple choice, yes/no, ranked, score;
  anonymous ballots, deadlines, result visibility
* **Calendar** — private and shared events, iCalendar import/export
* **Meetings** — plan → agenda → protocol → finalise, Markdown export
* **Decision log** — numbered decisions with audience and full revision
  history
* **Kanban boards** — private or shared
* **Search** — everything you have access to; plain, wildcard and regular
  expressions; time period and type filters
* **People** — directory, global groups, personal (local) groups, roles
* **History** — personal activity; audit log for administrators
* **Swedish and English**, themes (light, dark, spring, autumn, sand,
  contrast), text size and accessibility settings
* **Administration** — accounts, roles and permissions, MFA, logs, encrypted
  backups and exports

Security highlights: TLS 1.3 only, Argon2id, TOTP two-factor, role based
access control plus per-item sharing, encrypted files and personal data,
encrypted backups, strict Content-Security-Policy, hardened systemd unit
with seccomp filter, built-in Landlock sandbox, SELinux module.
Details: [docs/security.md](docs/security.md).

## Quick start (development)

Requires Go (see `go.mod` for the version).

```bash
make build
```

```bash
./bin/samling admin create --username admin --data-dir ./data
```

This prints a temporary password. Start the server over plain HTTP on a port
of your choice (development only):

```bash
make run-dev DEV_PORT=8080
```

Open <http://127.0.0.1:8080/>, sign in and choose a new password.

The port can always be set with `server.port` in the YAML configuration,
the `SAMLING_PORT` environment variable or the `--port` flag. See
[`samling.example.yaml`](samling.example.yaml) for every option.

## Production

```bash
sudo scripts/setup.sh --port 8443 --org "Our Club" --letsencrypt club.example.org --email admin@example.org
```

installs a hardened systemd service with TLS, a root-only master key and
daily encrypted backups. Read [docs/deployment.md](docs/deployment.md) first.

## Tests

```bash
make test
```

runs the Go test suite (the same suite also passes on PostgreSQL with
`make test-postgres`) and the translation check;

```bash
make e2e
```

drives the real client in a browser (needs Node.js with Playwright).
See [docs/testing.md](docs/testing.md).

## Documentation

| | |
|---|---|
| [Requirements and status](docs/requirements.md) | [Architecture](docs/architecture.md) |
| [Design decisions](docs/design-decisions.md) | [Security](docs/security.md) |
| [Deployment and operations](docs/deployment.md) | [User guide](docs/usage.md) |
| [REST API](docs/api.md) | [Mobile](docs/mobile.md) |
| [Testing](docs/testing.md) | [Example configuration](samling.example.yaml) |
