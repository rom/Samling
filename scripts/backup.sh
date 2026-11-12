#!/usr/bin/env bash
# backup.sh - timestamped, encrypted Samling backups with retention.
set -euo pipefail

DIR="/var/backups/samling"
KEEP=14
PASS_FILE=""
CONFIG="${SAMLING_CONFIG:-/etc/samling/samling.yaml}"
SAMLING_BIN="${SAMLING_BIN:-/usr/local/bin/samling}"
SAMLING_CTL="${SAMLING_CTL:-/usr/local/sbin/samling-ctl}"

usage() {
	cat <<USAGE
Usage: backup.sh [--dir DIR] [--keep N] [--passphrase-file FILE] [--config FILE]

Writes DIR/samling-YYYYmmdd-HHMMSS.bak with "samling backup" and deletes all
but the newest N backups. Safe to run while the server is running.

Options:
  --dir DIR               backup directory (default: $DIR); must be writable
                          by the samling user
  --keep N                number of backups to keep (default: $KEEP, 0 = keep all)
  --passphrase-file FILE  file with the backup passphrase (min 12 characters).
                          Default: \$CREDENTIALS_DIRECTORY/backup.pass when run
                          from samling-backup.service, else
                          /etc/samling/backup.passphrase
  --config FILE           samling.yaml (default: $CONFIG)
  -h, --help              show this help

A backup contains the database, all files and the master key, encrypted with
the passphrase. WITHOUT THE PASSPHRASE A BACKUP IS USELESS: store the
passphrase somewhere else than the backups, and copy the backups off this
machine.

Run it from the provided systemd timer (deploy/systemd/samling-backup.timer),
from cron as root, or by hand.

Restore (into an EMPTY installation, service stopped):
  SAMLING_BACKUP_PASSPHRASE=... samling restore --in FILE --config /etc/samling/samling.yaml
See docs/deployment.md, "Backup and restore".
USAGE
}

while [ $# -gt 0 ]; do
	case "$1" in
	--dir) DIR="$2"; shift 2 ;;
	--keep) KEEP="$2"; shift 2 ;;
	--passphrase-file) PASS_FILE="$2"; shift 2 ;;
	--config) CONFIG="$2"; shift 2 ;;
	-h | --help) usage; exit 0 ;;
	*) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
	esac
done
case "$KEEP" in '' | *[!0-9]*) echo "--keep must be a number" >&2; exit 2 ;; esac

if [ -z "$PASS_FILE" ]; then
	if [ -n "${CREDENTIALS_DIRECTORY:-}" ] && [ -r "$CREDENTIALS_DIRECTORY/backup.pass" ]; then
		PASS_FILE="$CREDENTIALS_DIRECTORY/backup.pass"
	else
		PASS_FILE="/etc/samling/backup.passphrase"
	fi
fi
[ -r "$PASS_FILE" ] || { echo "backup.sh: cannot read passphrase file $PASS_FILE" >&2; exit 1; }
[ -d "$DIR" ] || { echo "backup.sh: backup directory $DIR does not exist" >&2; exit 1; }

SAMLING_BACKUP_PASSPHRASE="$(head -n 1 "$PASS_FILE")"
export SAMLING_BACKUP_PASSPHRASE
export SAMLING_CONFIG="$CONFIG"

out="$DIR/samling-$(date -u +%Y%m%d-%H%M%S).bak"
if [ -n "${CREDENTIALS_DIRECTORY:-}" ] && [ -r "$CREDENTIALS_DIRECTORY/master.key" ]; then
	# Started by systemd with the master key as a credential.
	"$SAMLING_BIN" backup --out "$out"
elif [ "$(id -u)" -eq 0 ] && [ -x "$SAMLING_CTL" ]; then
	"$SAMLING_CTL" backup --out "$out"
else
	# Development / installations that keep master.key in the data directory.
	"$SAMLING_BIN" backup --out "$out"
fi

if [ "$KEEP" -gt 0 ]; then
	# Names sort chronologically; remove everything but the newest $KEEP.
	find "$DIR" -maxdepth 1 -type f -name 'samling-*.bak' | sort -r | tail -n "+$((KEEP + 1))" |
		while IFS= read -r old; do
			rm -f -- "$old"
			echo "removed old backup $old"
		done
fi
