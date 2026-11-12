#!/usr/bin/env bash
# setup.sh - install Samling as a hardened systemd service.
set -euo pipefail

REPO="$(cd "$(dirname "$(readlink -f "$0")")/.." && pwd)"
BINARY=""
PORT=8443
PORT_SET=0
ORG="Samling"
ADMIN="admin"
SELFSIGNED=""
LE_DOMAIN=""
LE_EMAIL=""
START=1
BACKUP_TIMER=1

ETC=/etc/samling
DATA=/var/lib/samling
BACKUPS=/var/backups/samling
LIBDIR=/usr/local/lib/samling
UNITS=/etc/systemd/system

usage() {
	cat <<USAGE
Usage: setup.sh [options]

Installs or upgrades Samling on this machine (run as root). Safe to re-run:
existing configuration, keys, certificates and data are never overwritten.

Options:
  --binary FILE          samling binary to install (default: bin/samling in the
                         repository; built with "make build" if Go is installed)
  --port N               port to listen on (default: 8443). Only used when the
                         configuration file is created. Ports below 1024 get
                         CAP_NET_BIND_SERVICE through a systemd drop-in.
  --org NAME             organisation name shown in the client (default: Samling)
  --admin NAME           user name of the first administrator (default: admin)
  --no-admin             do not create an administrator account
  --selfsigned HOST      create a self-signed certificate for HOST
  --letsencrypt DOMAIN   obtain a Let's Encrypt certificate for DOMAIN
  --email ADDRESS        contact address for Let's Encrypt (required with it)
  --no-backup-timer      do not enable the daily backup timer
  --no-start             install everything but do not enable/start the service
  -h, --help             show this help

Without --selfsigned/--letsencrypt and without an existing certificate in
$ETC/tls, a self-signed certificate for this host's name is created so the
server can start.

What it does:
  * creates the system user "samling"
  * $ETC (0750 root:samling): samling.yaml, master.key (0600 root), tls/
  * $DATA (0700 samling): database and encrypted files
  * $BACKUPS (0700 samling) and a random backup passphrase in
    $ETC/backup.passphrase (0600 root)
  * /usr/local/bin/samling, /usr/local/sbin/samling-ctl, $LIBDIR/*.sh
  * systemd units samling.service, samling-backup.service/.timer
  * the SELinux policy module when SELinux is enabled and the policy
    development files are installed
  * the first administrator account (temporary password is printed once)
USAGE
}

while [ $# -gt 0 ]; do
	case "$1" in
	--binary) BINARY="$2"; shift 2 ;;
	--port) PORT="$2"; PORT_SET=1; shift 2 ;;
	--org) ORG="$2"; shift 2 ;;
	--admin) ADMIN="$2"; shift 2 ;;
	--no-admin) ADMIN=""; shift ;;
	--selfsigned) SELFSIGNED="$2"; shift 2 ;;
	--letsencrypt) LE_DOMAIN="$2"; shift 2 ;;
	--email) LE_EMAIL="$2"; shift 2 ;;
	--no-backup-timer) BACKUP_TIMER=0; shift ;;
	--no-start) START=0; shift ;;
	-h | --help) usage; exit 0 ;;
	*) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
	esac
done

say() { printf '\n==> %s\n' "$*"; }
die() { echo "setup.sh: $*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "must be run as root"
case "$PORT" in '' | *[!0-9]*) die "--port must be a number" ;; esac
{ [ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ]; } || die "--port must be 1-65535"
case "$ORG" in *\"* | *\\*) die "--org must not contain quotes or backslashes" ;; esac
[ -z "$LE_DOMAIN" ] || [ -n "$LE_EMAIL" ] || die "--letsencrypt requires --email"
[ -z "$LE_DOMAIN" ] || [ -z "$SELFSIGNED" ] || die "use either --selfsigned or --letsencrypt"
command -v systemctl >/dev/null || die "systemd is required"

# --- binary -------------------------------------------------------------
if [ -z "$BINARY" ]; then
	BINARY="$REPO/bin/samling"
	if [ ! -x "$BINARY" ]; then
		command -v go >/dev/null || die "no binary found: build with 'make build' or pass --binary FILE"
		say "Building samling"
		(cd "$REPO" && make build)
	fi
fi
[ -x "$BINARY" ] || die "$BINARY is not an executable file"

# --- user and directories -----------------------------------------------
say "Creating user and directories"
if ! getent passwd samling >/dev/null; then
	useradd --system --user-group --home-dir "$DATA" --no-create-home --shell /usr/sbin/nologin samling
fi
install -d -m 0750 -o root -g samling "$ETC" "$ETC/tls"
install -d -m 0700 -o samling -g samling "$DATA" "$BACKUPS"
install -d -m 0755 "$LIBDIR"

# --- programs -----------------------------------------------------------
say "Installing programs"
was_active=0
if systemctl is-active --quiet samling.service; then
	was_active=1
fi
install -m 0755 "$BINARY" /usr/local/bin/samling.new
mv -f /usr/local/bin/samling.new /usr/local/bin/samling
install -m 0755 "$REPO/scripts/samling-ctl.sh" /usr/local/sbin/samling-ctl
install -m 0755 "$REPO/scripts/backup.sh" "$REPO/scripts/letsencrypt.sh" "$REPO/scripts/gen-selfsigned-cert.sh" "$LIBDIR/"
install -m 0644 "$REPO/samling.example.yaml" "$ETC/samling.example.yaml"
/usr/local/bin/samling version

# --- TLS ----------------------------------------------------------------
if [ -n "$LE_DOMAIN" ]; then
	say "Obtaining a Let's Encrypt certificate for $LE_DOMAIN"
	"$LIBDIR/letsencrypt.sh" obtain "$LE_DOMAIN" --email "$LE_EMAIL"
elif [ -n "$SELFSIGNED" ]; then
	say "Creating a self-signed certificate for $SELFSIGNED"
	"$LIBDIR/gen-selfsigned-cert.sh" --out "$ETC/tls" --force "$SELFSIGNED"
elif [ ! -e "$ETC/tls/fullchain.pem" ]; then
	host="$(hostname -f 2>/dev/null || hostname)"
	say "No certificate yet: creating a self-signed one for $host"
	"$LIBDIR/gen-selfsigned-cert.sh" --out "$ETC/tls" "$host" localhost 127.0.0.1
fi

# --- configuration ------------------------------------------------------
if [ ! -e "$ETC/samling.yaml" ]; then
	say "Writing $ETC/samling.yaml (port $PORT)"
	umask 027
	cat >"$ETC/samling.yaml" <<YAML
# Samling configuration. All options are described in $ETC/samling.example.yaml
org_name: "$ORG"
data_dir: "$DATA"

server:
  host: ""
  port: $PORT
  tls:
    cert: "$ETC/tls/fullchain.pem"
    key: "$ETC/tls/privkey.pem"

database:
  driver: "sqlite"

storage:
  backend: "fs"

security:
  landlock: "on"
YAML
	umask 022
	chown root:samling "$ETC/samling.yaml"
	chmod 0640 "$ETC/samling.yaml"
else
	say "Keeping existing $ETC/samling.yaml"
	if [ "$PORT_SET" -eq 1 ]; then
		echo "NOTE: --port is ignored because the configuration exists; edit server.port in $ETC/samling.yaml"
	fi
	# Use the configured port for the capability drop-in below.
	cfg_port="$(sed -n 's/^[[:space:]]*port:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$ETC/samling.yaml" | head -n 1)"
	[ -z "$cfg_port" ] || PORT="$cfg_port"
fi

# --- secrets ------------------------------------------------------------
if [ ! -e "$ETC/master.key" ]; then
	say "Generating the master key"
	(umask 077 && /usr/local/bin/samling genkey >"$ETC/master.key")
	chmod 0600 "$ETC/master.key"
fi
new_pass=0
if [ ! -e "$ETC/backup.passphrase" ]; then
	(umask 077 && head -c 24 /dev/urandom | base64 | tr -d '\n=' >"$ETC/backup.passphrase" && echo >>"$ETC/backup.passphrase")
	chmod 0600 "$ETC/backup.passphrase"
	new_pass=1
fi

# --- systemd ------------------------------------------------------------
say "Installing systemd units"
install -m 0644 "$REPO/deploy/systemd/samling.service" "$REPO/deploy/systemd/samling-backup.service" \
	"$REPO/deploy/systemd/samling-backup.timer" "$UNITS/"
if [ "$PORT" -lt 1024 ]; then
	install -d -m 0755 "$UNITS/samling.service.d"
	cat >"$UNITS/samling.service.d/low-port.conf" <<'DROPIN'
# Written by setup.sh: allow binding to a port below 1024.
[Service]
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
DROPIN
else
	rm -f "$UNITS/samling.service.d/low-port.conf"
fi
systemctl daemon-reload

# --- SELinux ------------------------------------------------------------
if command -v selinuxenabled >/dev/null && selinuxenabled; then
	if [ -f /usr/share/selinux/devel/Makefile ] && command -v semodule >/dev/null; then
		say "Installing the SELinux policy module"
		tmp="$(mktemp -d)"
		cp "$REPO"/deploy/selinux/samling.te "$REPO"/deploy/selinux/samling.fc "$REPO"/deploy/selinux/samling.if "$tmp/"
		if make -C "$tmp" -f /usr/share/selinux/devel/Makefile samling.pp >/dev/null && semodule -i "$tmp/samling.pp"; then
			if command -v semanage >/dev/null; then
				semanage port -a -t samling_port_t -p tcp "$PORT" 2>/dev/null ||
					echo "NOTE: port $PORT already has an SELinux type (fine if it is http_port_t, see deploy/selinux/README.md)"
			fi
			restorecon -R /usr/local/bin/samling "$ETC" "$DATA" "$BACKUPS" || true
		else
			echo "WARNING: the SELinux module could not be built/installed; see deploy/selinux/README.md"
		fi
		rm -rf "$tmp"
	else
		echo "NOTE: SELinux is enabled but the policy development files are missing"
		echo "      (dnf install selinux-policy-devel); see deploy/selinux/README.md"
	fi
fi

# --- first administrator ------------------------------------------------
# Only once, on a fresh installation.
if [ -n "$ADMIN" ] && [ ! -e "$ETC/.admin-created" ] && [ ! -e "$DATA/samling.db" ]; then
	say "Creating the first administrator \"$ADMIN\""
	if /usr/local/sbin/samling-ctl admin create --username "$ADMIN" --role admin; then
		echo "Write the temporary password down now: it is not shown again."
		touch "$ETC/.admin-created"
	else
		echo "WARNING: could not create the administrator; run: samling-ctl admin create --username $ADMIN"
	fi
fi

# --- start --------------------------------------------------------------
if [ "$START" -eq 1 ]; then
	say "Starting the service"
	systemctl enable samling.service >/dev/null
	if [ "$was_active" -eq 1 ]; then
		systemctl restart samling.service
	else
		systemctl start samling.service
	fi
	if [ "$BACKUP_TIMER" -eq 1 ]; then
		systemctl enable --now samling-backup.timer >/dev/null
	fi
	sleep 1
	systemctl --no-pager --lines=5 status samling.service || true
fi

say "Done"
echo "Samling listens on port $PORT (https)."
echo "Configuration: $ETC/samling.yaml    Logs: journalctl -u samling"
echo "Manage accounts from the shell with: samling-ctl admin create --username NAME --role member"
if [ "$new_pass" -eq 1 ]; then
	echo
	echo "IMPORTANT: daily backups are written to $BACKUPS, encrypted with the"
	echo "passphrase in $ETC/backup.passphrase. Copy that passphrase to a safe"
	echo "place off this machine, and copy the backups off this machine as well."
fi
