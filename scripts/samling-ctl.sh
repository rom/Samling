#!/usr/bin/env bash
# samling-ctl - run the samling CLI on an installed server.
#
# The service receives its master key from systemd (LoadCredential=), so the
# key file is readable by root only. This wrapper gives a one-off CLI command
# the same key and configuration and runs it as the unprivileged service user.
set -euo pipefail

SAMLING_BIN="${SAMLING_BIN:-/usr/local/bin/samling}"
SAMLING_CONFIG="${SAMLING_CONFIG:-/etc/samling/samling.yaml}"
SAMLING_KEY="${SAMLING_KEY:-/etc/samling/master.key}"
SAMLING_USER="${SAMLING_USER:-samling}"
RUN_DIR="${SAMLING_CTL_RUN_DIR:-/run}"

usage() {
	cat <<USAGE
Usage: samling-ctl <samling command> [flags]

Runs "samling <command>" as user "$SAMLING_USER" with the installed
configuration ($SAMLING_CONFIG) and master key ($SAMLING_KEY).
Must be run as root.

Examples:
  samling-ctl admin create --username anna --role member
  samling-ctl admin passwd --username anna
  samling-ctl check-config
  SAMLING_BACKUP_PASSPHRASE=... samling-ctl backup --out /var/backups/samling/manual.bak

Files written by the command (e.g. backup --out) must be in a directory that
user "$SAMLING_USER" may write to. Do not run "restore" against a running
service; see docs/deployment.md.
USAGE
}

case "${1:-}" in
"" | -h | --help)
	usage
	exit 0
	;;
esac

if [ "$(id -u)" -ne 0 ]; then
	echo "samling-ctl: must be run as root" >&2
	exit 1
fi
[ -x "$SAMLING_BIN" ] || { echo "samling-ctl: $SAMLING_BIN not found" >&2; exit 1; }
[ -r "$SAMLING_KEY" ] || { echo "samling-ctl: master key $SAMLING_KEY not found" >&2; exit 1; }

umask 077
# Private, short-lived copy of the key for the service user (on tmpfs).
cred_dir="$(mktemp -d "$RUN_DIR/samling-ctl.XXXXXX")"
trap 'rm -rf "$cred_dir"' EXIT
install -m 0400 -o "$SAMLING_USER" "$SAMLING_KEY" "$cred_dir/master.key"
chown "$SAMLING_USER" "$cred_dir"
chmod 0500 "$cred_dir"

# The settings are handed over in the environment, which runuser passes on.
# Nothing secret is put on a command line: arguments are visible to every
# local user in the process list, the environment is not.
export CREDENTIALS_DIRECTORY="$cred_dir"
export SAMLING_CONFIG
if [ -n "${SAMLING_BACKUP_PASSPHRASE:-}" ]; then
	export SAMLING_BACKUP_PASSPHRASE
fi
runuser -u "$SAMLING_USER" -- "$SAMLING_BIN" "$@"
