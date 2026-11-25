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
cleanup() {
	$compose down --volumes --remove-orphans >/dev/null 2>&1 || true
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
HTTPFIN_SECURITY_LOG=/run/httpfin/security.log \
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

body=$(curl -kfsS --resolve inspector.example.test:8443:127.0.0.1 \
	${curl_http2_args} https://inspector.example.test:8443/api/connections)
printf '%s\n' "$body" | grep -Fq '"url":"/healthz"'
printf '%s\n' "$body" | grep -Fq '"tls_version":"TLSv1.3"'
printf '%s\n' "$body" | grep -Fq '"sni":"inspector.example.test"'
printf '%s\n' "$body" | grep -Fq '"alpn":"h2"'

logs=$($compose logs --no-color httpfin)
printf '%s\n' "$logs" | grep -Fq '"sni":"inspector.example.test"'
printf '%s\n' "$logs" | grep -Eq '"alpn":"(h2|http/1.1)"'

$compose exec -T httpfin wget --no-check-certificate -qO- \
	--header='Host: inspector.example.test' \
	https://127.0.0.1:8443/__httpfin_test/unauthorized >/dev/null 2>&1 && {
	echo "expected test endpoint to return 401" >&2
	exit 1
} || true

security_log=$($compose exec -T httpfin sh -c 'cat /run/httpfin/security.log')
printf '%s\n' "$security_log" | grep -Eq \
	'event=auth_failure endpoint=/__httpfin_test/unauthorized status=401 reason=invalid_credentials'
if printf '%s\n' "$security_log" | grep -Eq 'token=|cookie=|authorization='; then
	echo "security log contains a forbidden secret-like field" >&2
	exit 1
fi

echo "httpfin container TLS/SNI -> Nginx module -> Go smoke test passed"
