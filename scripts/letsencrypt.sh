#!/usr/bin/env bash
# letsencrypt.sh - obtain and renew Let's Encrypt certificates for Samling.
set -euo pipefail

TLS_DIR="${SAMLING_TLS_DIR:-/etc/samling/tls}"
GROUP="${SAMLING_GROUP:-samling}"
HOOK="/etc/letsencrypt/renewal-hooks/deploy/samling"

usage() {
	cat <<USAGE
Usage:
  letsencrypt.sh obtain <domain> --email <address> [--webroot DIR] [--staging]
  letsencrypt.sh renew
  letsencrypt.sh deploy <domain>

obtain   Requests a certificate with certbot and installs a deploy hook that
         copies it to $TLS_DIR (fullchain.pem, privkey.pem, key readable by
         group "$GROUP"). Then set in samling.yaml:
           server:
             tls:
               cert: $TLS_DIR/fullchain.pem
               key:  $TLS_DIR/privkey.pem
renew    Runs "certbot renew". Normally not needed: the certbot package
         installs a systemd timer (certbot.timer) or cron job that renews
         automatically, and the deploy hook copies the new certificate.
deploy   Copies the current certificate of <domain> to $TLS_DIR (what the
         hook does; useful after restoring a server).

Samling re-reads the certificate files when they change (within about a
minute), so no restart is needed after a renewal.

Requirements:
  * certbot installed, run as root.
  * The domain must point to this server.
  * Standalone mode (default): TCP port 80 must be reachable from the
    internet and free while the certificate is issued or renewed (Let's
    Encrypt validates over HTTP on port 80). Samling itself does not use
    port 80. If another web server occupies port 80, use --webroot DIR with
    that server's document root instead.
  * --staging uses the Let's Encrypt test environment (untrusted
    certificates, generous rate limits) - good for a first trial.
USAGE
}

need_root() {
	if [ "$(id -u)" -ne 0 ]; then
		echo "letsencrypt.sh: must be run as root" >&2
		exit 1
	fi
}

deploy() { # deploy <live-dir>
	local live="$1"
	[ -r "$live/fullchain.pem" ] || { echo "no certificate in $live" >&2; exit 1; }
	umask 077
	mkdir -p "$TLS_DIR"
	if getent group "$GROUP" >/dev/null 2>&1; then
		chown "root:$GROUP" "$TLS_DIR"
		chmod 0750 "$TLS_DIR"
		# Key first: the server reloads when the certificate file changes.
		install -m 0640 -o root -g "$GROUP" "$live/privkey.pem" "$TLS_DIR/privkey.pem"
		install -m 0644 -o root -g "$GROUP" "$live/fullchain.pem" "$TLS_DIR/fullchain.pem"
	else
		install -m 0600 "$live/privkey.pem" "$TLS_DIR/privkey.pem"
		install -m 0644 "$live/fullchain.pem" "$TLS_DIR/fullchain.pem"
	fi
	echo "installed certificate from $live into $TLS_DIR"
}

install_hook() {
	local self
	self="$(readlink -f "$0")"
	mkdir -p "$(dirname "$HOOK")"
	cat >"$HOOK" <<HOOKEOF
#!/bin/sh
# Installed by Samling's letsencrypt.sh. certbot sets RENEWED_LINEAGE to the
# directory of the certificate that was just issued or renewed.
SAMLING_TLS_DIR="$TLS_DIR" SAMLING_GROUP="$GROUP" exec "$self" deploy-lineage "\$RENEWED_LINEAGE"
HOOKEOF
	chmod 0755 "$HOOK"
}

cmd="${1:-}"
[ $# -gt 0 ] && shift
case "$cmd" in
obtain)
	domain=""
	email=""
	webroot=""
	staging=""
	while [ $# -gt 0 ]; do
		case "$1" in
		--email) email="$2"; shift 2 ;;
		--webroot) webroot="$2"; shift 2 ;;
		--staging) staging="--staging"; shift ;;
		-*) echo "unknown option: $1" >&2; exit 2 ;;
		*) domain="$1"; shift ;;
		esac
	done
	if [ -z "$domain" ] || [ -z "$email" ]; then
		usage >&2
		exit 2
	fi
	need_root
	command -v certbot >/dev/null || { echo "certbot is not installed (e.g. apt install certbot / dnf install certbot)" >&2; exit 1; }
	install_hook
	args=(certonly --non-interactive --agree-tos --email "$email" -d "$domain" --key-type ecdsa)
	if [ -n "$webroot" ]; then
		args+=(--webroot -w "$webroot")
	else
		args+=(--standalone)
	fi
	[ -n "$staging" ] && args+=("$staging")
	certbot "${args[@]}"
	# The hook only runs when a certificate was issued; make sure it is in place.
	deploy "/etc/letsencrypt/live/$domain"
	if command -v systemctl >/dev/null && ! systemctl is-enabled --quiet certbot.timer 2>/dev/null; then
		echo "NOTE: certbot.timer is not enabled. Enable it (systemctl enable --now certbot.timer)"
		echo "      or schedule '$(readlink -f "$0") renew' twice a day with cron."
	fi
	;;
renew)
	need_root
	command -v certbot >/dev/null || { echo "certbot is not installed" >&2; exit 1; }
	[ -x "$HOOK" ] || install_hook
	certbot renew --quiet
	;;
deploy)
	[ $# -eq 1 ] || { usage >&2; exit 2; }
	need_root
	deploy "/etc/letsencrypt/live/$1"
	;;
deploy-lineage) # internal: called by the certbot hook
	[ $# -eq 1 ] && [ -n "$1" ] || { echo "deploy-lineage needs RENEWED_LINEAGE" >&2; exit 2; }
	deploy "$1"
	;;
-h | --help | help | "")
	usage
	;;
*)
	echo "unknown command: $cmd" >&2
	usage >&2
	exit 2
	;;
esac
