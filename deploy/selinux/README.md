# SELinux policy module for Samling

Confines the server to its own domain, `samling_t`:

| Type | Applies to | Access |
|------|------------|--------|
| `samling_exec_t` | `/usr/local/bin/samling` | entry point (started by systemd) |
| `samling_etc_t` | `/etc/samling` (config, TLS files, master key) | read only |
| `samling_var_lib_t` | `/var/lib/samling`, `/var/backups/samling` | read/write |
| `samling_port_t` | the listening port | bind |

Booleans (both off by default):

| Boolean | Enable when |
|---------|-------------|
| `samling_can_connect_postgresql` | `database.driver: postgres` |
| `samling_can_network_connect` | `storage.backend: s3` (outbound TCP to any port) |

> **Status: untested.** The module was written against the reference policy
> interfaces (Fedora / RHEL / CentOS Stream `selinux-policy-devel`) but could
> **not be compiled or run** in the development environment, which has no
> SELinux tooling. Expect to adjust it: install it, run the domain in
> permissive mode first and review the denials (steps below).

## Build and install

```bash
dnf install selinux-policy-devel policycoreutils-python-utils
```

```bash
make -f /usr/share/selinux/devel/Makefile samling.pp
```

```bash
semodule -i samling.pp
```

Label the listening port (use your `server.port`). Ports 443 and 8443 are
already labelled `http_port_t` by the distribution policy, which the module
also allows, so this step is only needed for other ports:

```bash
semanage port -a -t samling_port_t -p tcp 9443
```

Apply the file contexts:

```bash
restorecon -Rv /usr/local/bin/samling /etc/samling /var/lib/samling /var/backups/samling
```

```bash
systemctl restart samling
```

`scripts/setup.sh` performs these steps automatically when SELinux is enabled
and `selinux-policy-devel` is installed.

## First run: permissive, then enforce

```bash
semanage permissive -a samling_t
```

Use the application (sign in, upload a file, run a backup), then list what
would have been denied:

```bash
ausearch -m avc -ts recent -c samling | audit2allow -R
```

Add any legitimate rules to `samling.te`, rebuild, reinstall, and switch the
domain to enforcing:

```bash
semanage permissive -d samling_t
```

Things to check in particular:

* **systemd credential** – the master key is delivered through
  `LoadCredential=` under `/run/credentials/samling.service/`. Depending on
  the policy version, reading it may need an extra rule. If you cannot allow
  it, remove the `LoadCredential=` line from the unit, make
  `/etc/samling/master.key` owned by `root:samling` with mode `0640`, and set
  `security.master_key_file: /etc/samling/master.key`.
* **Booleans** – `setsebool -P samling_can_connect_postgresql on` for
  PostgreSQL, `setsebool -P samling_can_network_connect on` for S3.
* `samling-ctl` and the backup script start `samling` from a root shell or a
  shell script, not directly from systemd, so those runs are not transitioned
  into `samling_t` (they run in the caller's domain).

## Remove

```bash
semodule -r samling
```
