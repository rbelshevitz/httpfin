#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
module_dir="$repo_dir/nginx-module"
source_file="$module_dir/ngx_http_httpfin_module.c"

test -s "$source_file"
test -s "$module_dir/config"
test -s "$module_dir/Makefile"
make -C "$module_dir" check

# These are source-level invariants of the integration format. They catch
# accidental removal of non-blocking handoff or lifecycle hooks in CI without
# requiring NGINX headers or a network download.
grep -Fq 'AF_UNIX' "$source_file"
grep -Fq 'SOCK_DGRAM' "$source_file"
grep -Fq 'O_NONBLOCK' "$source_file"
grep -Fq 'MSG_DONTWAIT' "$source_file"
grep -Fq 'MSG_NOSIGNAL' "$source_file"
grep -Fq 'NGX_HTTP_POST_READ_PHASE' "$source_file"
grep -Fq 'NGX_HTTP_LOG_PHASE' "$source_file"
grep -Fq 'httpfin_socket' "$source_file"
grep -Fq 'httpfin_handoff_status' "$source_file"
grep -Fq 'httpfin_max_event 4096' "$source_file"
grep -Fq '\"version\":1' "$source_file"
grep -Fq '\"request_id\"' "$source_file"
grep -Fq '\"method\"' "$source_file"
grep -Fq '\"uri\"' "$source_file"
grep -Fq '\"status\"' "$source_file"
grep -Fq 'ngx_http_cycle_get_module_main_conf' "$source_file"
grep -Fq 'ngx_http_httpfin_append' "$source_file"
test "$(grep -Fc 'ngx_cpymem' "$source_file")" -eq 0
grep -Fq 'goto oversize' "$source_file"
grep -Fq 'ngx_http_httpfin_connect' "$source_file"
grep -Fq 'dropped_error' "$source_file"
grep -Fq 'ssl_protocol' "$source_file"
grep -Fq 'ssl_server_name' "$source_file"
grep -Fq 'ssl_alpn_protocol' "$source_file"

# If an NGINX source tree is supplied, also exercise the real module build.
# This remains opt-in so static CI does not need a compiler toolchain or
# network access.
if [ "${NGINX_SOURCE:-}" != "" ]; then
	make -C "$module_dir" build NGINX_SOURCE="$NGINX_SOURCE"
	test -s "$NGINX_SOURCE/objs/ngx_http_httpfin_module.so"
fi

echo "httpfin nginx module static checks passed"
