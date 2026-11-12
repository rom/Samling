#!/usr/bin/env bash
# Builds Samling, starts it over plain HTTP on a scratch data directory and
# runs the browser smoke test (tests/e2e/smoke.js) against it.
#
# Requires Node.js with the "playwright" package and a Chromium it can launch
# (npm install playwright && npx playwright install chromium).
#
# Usage: tests/e2e/run.sh [--port N] [--keep] [--work DIR]
#   --port N    port for the test server (default 8097)
#   --keep      keep the work directory (screenshots, server log) afterwards
#   --work DIR  use DIR (must be empty or absent) as work directory and keep it
#
# Set E2E_SCRIPT to run another script against the scratch server (used by
# "make manual-screenshots").
set -euo pipefail

port=8097
keep=0
work=""
while [ $# -gt 0 ]; do
	case "$1" in
	--port) port="$2"; shift 2 ;;
	--keep) keep=1; shift ;;
	--work) work="$2"; keep=1; shift 2 ;;
	-h | --help) sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
	*) echo "unknown argument: $1" >&2; exit 2 ;;
	esac
done

root="$(cd "$(dirname "$0")/../.." && pwd)"
if [ -n "$work" ]; then
	mkdir -p "$work"
	[ -z "$(ls -A "$work")" ] || { echo "work directory $work is not empty" >&2; exit 2; }
	work="$(cd "$work" && pwd)"
else
	work="$(mktemp -d)"
fi
server_pid=""
cleanup() {
	[ -n "$server_pid" ] && kill "$server_pid" 2>/dev/null || true
	if [ "$keep" = 1 ]; then echo "work directory kept: $work"; else rm -rf "$work"; fi
}
trap cleanup EXIT

(cd "$root" && CGO_ENABLED=0 go build -o "$work/samling" ./cmd/samling)
"$work/samling" admin create --username admin --name "Anna Admin" --email anna@example.org --data-dir "$work/data" >"$work/admin.txt"
password="$(sed -n 's/^Temporary password: //p' "$work/admin.txt")"

SAMLING_INSECURE_HTTP=1 SAMLING_ORG_NAME="Björkens IF" \
	"$work/samling" serve --host 127.0.0.1 --port "$port" --data-dir "$work/data" >"$work/server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 50); do
	curl -fsS -o /dev/null "http://127.0.0.1:$port/" 2>/dev/null && break
	sleep 0.1
done

E2E_URL="http://localhost:$port" E2E_ADMIN_PASSWORD="$password" E2E_OUT="$work/out" node "${E2E_SCRIPT:-$root/tests/e2e/smoke.js}"
