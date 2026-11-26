#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"

if ! command -v docker >/dev/null 2>&1; then
	echo "SKIP: docker is not installed" >&2
	exit 0
fi
if ! docker compose version >/dev/null 2>&1; then
	echo "SKIP: docker compose plugin is not installed" >&2
	exit 0
fi
if ! docker info >/dev/null 2>&1; then
	echo "SKIP: docker daemon is unavailable or docker socket is not accessible" >&2
	exit 0
fi
if ! command -v curl >/dev/null 2>&1; then
	echo "SKIP: curl is not installed" >&2
	exit 0
fi

curl_http2_args=""
if curl --version | grep -q 'HTTP2'; then
	curl_http2_args="--http2"
fi

project="httpfin-smoke-$$"
compose="docker compose -p $project"
load_tmp=$(mktemp -d)
cleanup() {
	$compose down --volumes --remove-orphans >/dev/null 2>&1 || true
	rm -rf "$load_tmp"
}
trap cleanup EXIT INT TERM

$compose build --pull=false httpfin

runtime=$($compose run --rm --no-deps --entrypoint /bin/sh httpfin -c '
set -eu
nginx -V 2>&1 | grep -Fq "nginx/1.28.3"
ldd /usr/sbin/nginx | grep -Fq "libssl.so.3"
ldd /usr/sbin/nginx | grep -Fq "libcrypto.so.3"
ldd /usr/sbin/nginx | grep -Fq "libpcre2-8.so.0"
ldd /usr/sbin/nginx | grep -Fq "libz.so.1"
apk info -e openssl pcre2 zlib >/dev/null
module_ldd=$(ldd /usr/lib/nginx/modules/ngx_http_httpfin_module.so 2>&1 || true)
if printf "%s\\n" "$module_ldd" | grep -Eq "libssl|libcrypto|libpcre2|libz"; then
	exit 1
fi
printf "runtime dependencies: nginx=1.28.3 openssl/pcre2/zlib=present module=no-direct-crypto\\n"
')
printf '%s\n' "$runtime" | grep -Fq 'module=no-direct-crypto'
HTTPFIN_TEST_ENDPOINT=1 \
$compose up -d httpfin

i=0
until curl -kfsS --resolve inspector.example.test:8443:127.0.0.1 \
	${curl_http2_args} https://inspector.example.test:8443/healthz >/dev/null 2>&1; do
	i=$((i + 1))
	if [ "$i" -ge 60 ]; then
		echo "Nginx TLS endpoint did not become ready" >&2
		$compose logs --no-color httpfin >&2 || true
		exit 1
	fi
	sleep 1
done

body=""
j=0
until printf '%s\n' "$body" | grep -Fq '"url":"/healthz"'; do
	body=$(curl -kfsS --resolve inspector.example.test:8443:127.0.0.1 \
		${curl_http2_args} https://inspector.example.test:8443/api/connections)
	j=$((j + 1))
	if [ "$j" -ge 50 ]; then
		echo "collector did not receive the healthz event" >&2
		$compose logs --no-color httpfin >&2 || true
		exit 1
	fi
	sleep 0.1
done
printf '%s\n' "$body" | grep -Fq '"url":"/healthz"'
printf '%s\n' "$body" | grep -Fq '"tls_version":"TLSv1.3"'
printf '%s\n' "$body" | grep -Fq '"sni":"inspector.example.test"'
printf '%s\n' "$body" | grep -Fq '"alpn":"h2"'
printf '%s\n' "$body" | grep -Fq '"host":"inspector.example.test:8443"'
printf '%s\n' "$body" | grep -Fq '"protocol":"HTTP/2.0"'
printf '%s\n' "$body" | grep -Fq '"elapsed_ms":'
printf '%s\n' "$body" | grep -Fq '"bytes":'

ingest=$(curl -kfsS --resolve inspector.example.test:8443:127.0.0.1 \
	${curl_http2_args} https://inspector.example.test:8443/api/ingest)
printf '%s\n' "$ingest" | grep -Fq '"status":"ok"'
printf '%s\n' "$ingest" | grep -Fq '"received":'
printf '%s\n' "$ingest" | grep -Fq '"decoded":'
printf '%s\n' "$ingest" | grep -Fq '"stored":'
printf '%s\n' "$ingest" | grep -Fq '"dropped":'

run_load() {
	phase=$1
	total=$2
	completed=0
	while [ "$completed" -lt "$total" ]; do
		pids=""
		batch=0
		while [ "$batch" -lt 20 ] && [ "$completed" -lt "$total" ]; do
			completed=$((completed + 1))
			curl -kfsS --max-time 5 --resolve inspector.example.test:8443:127.0.0.1 \
				${curl_http2_args} -o /dev/null -w '%{http_code}' \
				https://inspector.example.test:8443/app/api/catalog \
				>"$load_tmp/$phase-$completed" 2>/dev/null &
			pids="$pids $!"
			batch=$((batch + 1))
		done
		for pid in $pids; do
			wait "$pid"
		done
	done
	for result in "$load_tmp/$phase"-*; do
		code=$(tr -d '\r\n' < "$result")
		if [ "$code" != "200" ]; then
			echo "load phase $phase returned HTTP $code" >&2
			return 1
		fi
	done
}

run_load warmup 200

collector_pid=$($compose exec -T httpfin sh -c 'pidof httpfin')
if [ -z "$collector_pid" ]; then
	echo "cannot find collector process for load smoke" >&2
	exit 1
fi
before=$($compose exec -T httpfin wget -qO- http://127.0.0.1:8080/api/ingest)
$compose exec -T httpfin sh -c "kill -STOP $collector_pid"
run_load collector-stopped 500
$compose exec -T httpfin sh -c "kill -CONT $collector_pid"
run_load recovery 40

after=$($compose exec -T httpfin wget -qO- http://127.0.0.1:8080/api/ingest)
printf '%s\n' "$after" | grep -Fq '"healthy":true'
before_stored=$(printf '%s\n' "$before" | sed -n 's/.*"stored":\([0-9][0-9]*\).*/\1/p')
after_stored=$(printf '%s\n' "$after" | sed -n 's/.*"stored":\([0-9][0-9]*\).*/\1/p')
if [ -z "$before_stored" ] || [ -z "$after_stored" ] || [ "$after_stored" -le "$before_stored" ]; then
	echo "collector did not resume storing events after SIGSTOP/SIGCONT" >&2
	exit 1
fi

mock=$(curl -kfsS --resolve inspector.example.test:8443:127.0.0.1 \
	${curl_http2_args} https://inspector.example.test:8443/app/api/catalog)
printf '%s\n' "$mock" | grep -Fq 'httpfin-mock-api'

logs=$($compose logs --no-color httpfin)
printf '%s\n' "$logs" | grep -Fq '"sni":"inspector.example.test"'
printf '%s\n' "$logs" | grep -Eq '"alpn":"(h2|http/1.1)"'

$compose exec -T httpfin wget --no-check-certificate -qO- \
	--header='Host: inspector.example.test' \
	https://127.0.0.1:8443/__httpfin_test/unauthorized >/dev/null 2>&1 && {
	echo "expected test endpoint to return 401" >&2
	exit 1
} || true

echo "httpfin container TLS/SNI -> Nginx module -> Go smoke test passed"
