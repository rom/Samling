#!/usr/bin/env bash
# gen-selfsigned-cert.sh - create a self-signed TLS certificate for Samling.
set -euo pipefail

OUT_DIR="/etc/samling/tls"
DAYS=825
GROUP="samling"
FORCE=0
NAMES=()

usage() {
	cat <<USAGE
Usage: gen-selfsigned-cert.sh [options] <hostname-or-ip> [more names or IPs...]

Creates an ECDSA P-256 key and a self-signed certificate whose Subject
Alternative Names are the given DNS names and IP addresses. The first name
becomes the common name.

Options:
  --out DIR     output directory (default: $OUT_DIR)
  --days N      validity in days (default: $DAYS)
  --group NAME  group that may read the private key (default: $GROUP;
                only applied when run as root and the group exists)
  --force       overwrite an existing certificate
  -h, --help    show this help

Writes DIR/fullchain.pem (certificate) and DIR/privkey.pem (key, mode 0640).
Then set in samling.yaml:
  server:
    tls:
      cert: DIR/fullchain.pem
      key:  DIR/privkey.pem

Browsers will warn about a self-signed certificate until it is trusted
manually. Use scripts/letsencrypt.sh for a publicly trusted certificate.

Example:
  gen-selfsigned-cert.sh club.example.org 192.168.1.10 localhost 127.0.0.1
USAGE
}

while [ $# -gt 0 ]; do
	case "$1" in
	--out) OUT_DIR="$2"; shift 2 ;;
	--days) DAYS="$2"; shift 2 ;;
	--group) GROUP="$2"; shift 2 ;;
	--force) FORCE=1; shift ;;
	-h | --help) usage; exit 0 ;;
	-*) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
	*) NAMES+=("$1"); shift ;;
	esac
done

if [ "${#NAMES[@]}" -eq 0 ]; then
	usage >&2
	exit 2
fi
case "$DAYS" in '' | *[!0-9]*) echo "--days must be a number" >&2; exit 2 ;; esac
command -v openssl >/dev/null || { echo "openssl is required" >&2; exit 1; }

cert="$OUT_DIR/fullchain.pem"
key="$OUT_DIR/privkey.pem"
if [ -e "$cert" ] && [ "$FORCE" -ne 1 ]; then
	echo "$cert already exists (use --force to replace it)" >&2
	exit 1
fi

# Build the subjectAltName list: IP:... for addresses, DNS:... otherwise.
san=""
for n in "${NAMES[@]}"; do
	case "$n" in
	*[!A-Za-z0-9.:_-]*) echo "invalid name: $n" >&2; exit 2 ;;
	esac
	if [[ "$n" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] || [[ "$n" == *:* ]]; then
		san="${san:+$san,}IP:$n"
	else
		san="${san:+$san,}DNS:$n"
	fi
done

umask 077
mkdir -p "$OUT_DIR"
tmp_key="$(mktemp "$OUT_DIR/.privkey.XXXXXX")"
tmp_cert="$(mktemp "$OUT_DIR/.fullchain.XXXXXX")"
trap 'rm -f "$tmp_key" "$tmp_cert"' EXIT

openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes \
	-sha256 -days "$DAYS" \
	-subj "/CN=${NAMES[0]}" \
	-addext "subjectAltName=$san" \
	-addext "keyUsage=critical,digitalSignature" \
	-addext "extendedKeyUsage=serverAuth" \
	-addext "basicConstraints=critical,CA:FALSE" \
	-keyout "$tmp_key" -out "$tmp_cert" 2>/dev/null

chmod 0640 "$tmp_key"
chmod 0644 "$tmp_cert"
if [ "$(id -u)" -eq 0 ] && getent group "$GROUP" >/dev/null 2>&1; then
	chown "root:$GROUP" "$tmp_key" "$tmp_cert"
	chown "root:$GROUP" "$OUT_DIR"
	chmod 0750 "$OUT_DIR"
fi
# Key first: the server reloads when the certificate file changes.
mv -f "$tmp_key" "$key"
mv -f "$tmp_cert" "$cert"
trap - EXIT

echo "Certificate: $cert"
echo "Private key: $key"
echo "Names:       $san"
echo "Valid for:   $DAYS days"
echo "SHA-256 fingerprint (compare when the browser warns):"
openssl x509 -in "$cert" -noout -fingerprint -sha256 | sed 's/^/  /'
