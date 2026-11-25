#!/bin/sh
set -eu

mkdir -p /run/httpfin /etc/nginx/certs
if [ ! -s /etc/nginx/certs/server.crt ] || [ ! -s /etc/nginx/certs/server.key ]; then
  openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
    -keyout /etc/nginx/certs/server.key \
    -out /etc/nginx/certs/server.crt \
    -subj '/CN=inspector.example.test' \
    -addext 'subjectAltName=DNS:inspector.example.test' >/dev/null 2>&1
fi

httpfin_socket=/run/httpfin/events.sock \
httpfin_socket_group=nobody \
/usr/local/bin/httpfin &
httpfin_pid=$!
nginx_pid=
cleanup() {
	kill "$httpfin_pid" 2>/dev/null || true
	if [ -n "$nginx_pid" ]; then kill "$nginx_pid" 2>/dev/null || true; fi
	wait "$httpfin_pid" 2>/dev/null || true
	if [ -n "$nginx_pid" ]; then wait "$nginx_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT INT TERM

i=0
while [ ! -S /run/httpfin/events.sock ] && [ "$i" -lt 50 ]; do
  i=$((i + 1))
  sleep 0.1
done

test -S /run/httpfin/events.sock
nginx -t
/usr/sbin/nginx -g 'daemon off;' &
nginx_pid=$!
wait "$nginx_pid"
