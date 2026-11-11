// Package sandbox restricts the server process with Landlock after start-up:
// it may only touch its data directory, read its certificates and a few
// system files, and (in the self-contained SQLite + local file mode) cannot
// open outbound TCP connections at all. This limits what an attacker could
// do even with code execution inside the process. It complements, and does
// not replace, the systemd/SELinux/seccomp confinement in deploy/.
package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/landlock-lsm/go-landlock/landlock"
	llsys "github.com/landlock-lsm/go-landlock/landlock/syscall"

	"github.com/rom/samling/internal/config"
)

// Apply enables the sandbox. mode is "on" (best effort: use what the running
// kernel supports), "strict" (fail when Landlock is unavailable) or "off".
func Apply(cfg *config.Config) (string, error) {
	if cfg.Security.Landlock == "off" {
		return "disabled", nil
	}
	// Report what is really enforced: best-effort mode silently does less
	// (or nothing) on kernels with an older Landlock ABI.
	abi, err := llsys.LandlockGetABIVersion()
	if err != nil || abi < 1 {
		if cfg.Security.Landlock == "strict" {
			return "", errors.New("the kernel does not support Landlock")
		}
		return "unavailable (kernel without Landlock) - not sandboxed", nil
	}
	ll := landlock.V5
	if cfg.Security.Landlock != "strict" {
		ll = ll.BestEffort()
	}
	roDirs := []string{"/etc/ssl", "/etc/pki", "/etc/ca-certificates", "/usr/share/ca-certificates", "/usr/share/zoneinfo"}
	roFiles := []string{"/etc/resolv.conf", "/etc/hosts", "/etc/nsswitch.conf", "/etc/localtime", "/dev/urandom", "/dev/null"}
	rwDirs := []string{cfg.DataDir}
	// Certificates are re-read when renewed. Let's Encrypt uses symlinks from
	// live/ into archive/, so allow both the given and the resolved directory.
	for _, p := range []string{cfg.Server.TLS.Cert, cfg.Server.TLS.Key} {
		if p == "" {
			continue
		}
		roDirs = append(roDirs, filepath.Dir(p))
		if real, err := filepath.EvalSymlinks(p); err == nil {
			roDirs = append(roDirs, filepath.Dir(real))
		}
	}
	if dir := os.Getenv("CREDENTIALS_DIRECTORY"); dir != "" {
		roDirs = append(roDirs, dir)
	}
	if cfg.Database.Driver == "sqlite" {
		// The DSN may be a plain path or "file:path?options".
		path := strings.TrimPrefix(cfg.Database.DSN, "file:")
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
		rwDirs = append(rwDirs, filepath.Dir(path))
	} else {
		rwDirs = append(rwDirs, "/run/postgresql", "/var/run/postgresql") // unix socket
	}
	roFiles = append(roFiles, cfg.Security.MasterKeyFile)
	if err := ll.RestrictPaths(
		landlock.RWDirs(rwDirs...).IgnoreIfMissing(),
		landlock.RODirs(roDirs...).IgnoreIfMissing(),
		landlock.ROFiles(roFiles...).IgnoreIfMissing(),
	); err != nil {
		return "", err
	}
	status := fmt.Sprintf("ABI v%d: filesystem restricted", abi)
	// Without PostgreSQL or S3 the server never needs to connect out.
	if cfg.Database.Driver == "sqlite" && cfg.Storage.Backend == "fs" {
		if err := ll.RestrictNet(landlock.BindTCP(uint16(cfg.Server.Port))); err != nil {
			return "", err
		}
		if abi >= 4 {
			status += ", outbound TCP blocked"
		} else {
			status += ", network not restricted (needs ABI v4)"
		}
	}
	return status, nil
}
