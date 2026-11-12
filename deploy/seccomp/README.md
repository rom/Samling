# seccomp

Samling does not ship a separate seccomp profile. The system call filter is
applied by systemd, which compiles the `SystemCallFilter=` lines of
[`../systemd/samling.service`](../systemd/samling.service) into a seccomp-BPF
program and installs it before the server starts:

```ini
SystemCallArchitectures=native
SystemCallFilter=@system-service
SystemCallFilter=landlock_create_ruleset landlock_add_rule landlock_restrict_self
SystemCallFilter=~@privileged @resources
SystemCallErrorNumber=EPERM
```

* `@system-service` is systemd's allow-list for ordinary daemons.
* The three `landlock_*` calls are listed explicitly so the server can apply
  its own Landlock sandbox even on systemd versions whose `@system-service`
  set does not include them.
* `~@privileged @resources` removes calls that change credentials, load
  modules, mount file systems, change scheduling or resource limits.
* `SystemCallArchitectures=native` blocks the 32-bit compatibility ABI.
* Denied calls return `EPERM` (the Go runtime probes a few optional calls at
  start-up and tolerates the error) instead of killing the process.

Inspect the result on the server:

```bash
systemd-analyze security samling.service
```

```bash
systemd-analyze syscall-filter @system-service
```

Running in a container instead of under systemd? Use the runtime's default
seccomp profile (Docker and Podman apply one automatically) together with
`--cap-drop=ALL --security-opt=no-new-privileges --read-only` and a volume
for the data directory. A container image is not part of the MVP.
