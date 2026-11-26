#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"

if ! command -v curl >/dev/null 2>&1 || ! command -v firefox >/dev/null 2>&1; then
	echo "SKIP: curl and firefox are required for dashboard smoke" >&2
	exit 0
fi

port=${HTTPFIN_SMOKE_PORT:-18080}
base="http://127.0.0.1:$port"
output=${HTTPFIN_SCREENSHOT:-"${TMPDIR:-/tmp}/httpfin-dashboard-smoke.png"}
log=${TMPDIR:-/tmp}/httpfin-dashboard-smoke.$$.log
binary=${TMPDIR:-/tmp}/httpfin-dashboard-smoke.$$.bin

cleanup() {
	if [ "${app_pid:-}" != "" ]; then kill "$app_pid" >/dev/null 2>&1 || true; fi
	if [ "${app_pid:-}" != "" ]; then wait "$app_pid" >/dev/null 2>&1 || true; fi
	rm -f "$log"
	rm -f "$binary"
}
trap cleanup EXIT INT TERM

GOCACHE=${GOCACHE:-/tmp/httpfin-go-cache} go build -o "$binary" ./cmd/httpfin

GOCACHE=${GOCACHE:-/tmp/httpfin-go-cache} \
httpfin_test_endpoint=1 \
httpfin_demo=1 \
httpfin_addr="127.0.0.1:$port" httpfin_socket="" \
"$binary" >"$log" 2>&1 &
app_pid=$!

i=0
until curl -fsS "$base/healthz" >/dev/null 2>&1; do
	i=$((i + 1))
	if [ "$i" -ge 30 ]; then
		cat "$log" >&2
		echo "httpfin did not become ready" >&2
		exit 1
	fi
	sleep 1
done

bot() {
	ua=$1
	path=$2
	status_path=$3
	count=$4
	i=0
	while [ "$i" -lt "$count" ]; do
		curl -sS -o /dev/null -A "$ua" "$base$status_path?scenario=smoke" || true
		curl -sS -o /dev/null -A "$ua" "$base$path?scenario=smoke" || true
		i=$((i + 1))
	done
}

pids=""
bot 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131 Safari/537.36' /probe/catalog /not-found 8 & pids="$pids $!"
bot 'Mozilla/5.0 (X11; Linux x86_64; rv:133.0) Gecko/20100101 Firefox/133.0' /probe/search /missing 6 & pids="$pids $!"
bot 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile Safari/604.1' /probe/mobile /missing 7 & pids="$pids $!"
bot 'Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/130 Mobile Safari/537.36' /probe/mobile /not-found 9 & pids="$pids $!"
bot 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Edg/131.0.0.0 Chrome/131 Safari/537.36' /probe/edge /missing 5 & pids="$pids $!"
bot 'curl/8.10.1' /probe/health /__httpfin_test/unauthorized 4 & pids="$pids $!"
for pid in $pids; do wait "$pid"; done

snapshot=$(curl -fsS "$base/api/snapshot")
printf '%s\n' "$snapshot" | grep -Fq '"user_agents"'
printf '%s\n' "$snapshot" | grep -Fq '"destinations"'
printf '%s\n' "$snapshot" | grep -Fq '"browsers"'
printf '%s\n' "$snapshot" | grep -Fq 'Android'
printf '%s\n' "$snapshot" | grep -Fq 'iOS'

firefox --headless --screenshot="$output" --window-size=1440,1800 "$base/" >/dev/null 2>&1
test -s "$output"
printf 'dashboard smoke passed; screenshot: %s\n' "$output"
