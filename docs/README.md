# Samling documentation

Samling is a self-hosted support tool for running a club or organisation:
chat, documents, lists, polls, calendar, meetings, a decision log and kanban
boards, with role based access control and a strong focus on security.

| Document | Contents |
|----------|----------|
| [requirements.md](requirements.md) | The requirements and the status of each in the MVP |
| [architecture.md](architecture.md) | Components, packages, data model, access model, storage |
| [design-decisions.md](design-decisions.md) | Why things are built the way they are, with trade-offs |
| [security.md](security.md) | Threat model, authentication, authorisation, encryption, hardening |
| [deployment.md](deployment.md) | Installation, configuration reference, TLS, PostgreSQL, S3, backup/restore, CLI |
| [manual.md](manual.md) | **User manual with screenshots** |
| [usage.md](usage.md) | Reference guide for members and administrators |
| [api.md](api.md) | REST API reference (also used by native apps) |
| [mobile.md](mobile.md) | Phones and tablets: PWA today, native apps next |
| [testing.md](testing.md) | How to run the tests and what they cover |

Operational files live outside `docs/`:

| Path | Contents |
|------|----------|
| [`../samling.example.yaml`](../samling.example.yaml) | Commented configuration file |
| [`../scripts/`](../scripts) | `setup.sh`, `gen-selfsigned-cert.sh`, `letsencrypt.sh`, `backup.sh`, `samling-ctl.sh` |
| [`../deploy/systemd/`](../deploy/systemd) | Hardened service unit, backup service and timer |
| [`../deploy/selinux/`](../deploy/selinux) | SELinux policy module |
| [`../deploy/seccomp/`](../deploy/seccomp) | How the system call filter is applied |
