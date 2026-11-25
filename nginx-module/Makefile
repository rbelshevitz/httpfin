NGINX ?= nginx
NGINX_SOURCE ?= ../nginx
PREFIX ?= /usr/local
NGINX_CONFIGURE_ARGS ?= --with-compat

.PHONY: help configure build install check

help:
	@echo "make configure NGINX_SOURCE=/path/to/nginx-source"
	@echo "make build NGINX_SOURCE=/path/to/nginx-source"

configure:
	cd "$(NGINX_SOURCE)" && ./configure $(NGINX_CONFIGURE_ARGS) --add-dynamic-module="$(CURDIR)"

build: configure
	$(MAKE) -C "$(NGINX_SOURCE)" modules

install: build
	install -D -m 0644 "$(NGINX_SOURCE)/objs/ngx_http_httpfin_module.so" \
		"$(PREFIX)/modules/ngx_http_httpfin_module.so"

check:
	@test -s ngx_http_httpfin_module.c
	@test -s config
	@grep -q 'NGX_HTTP_MODULE' ngx_http_httpfin_module.c
	@grep -q 'O_NONBLOCK' ngx_http_httpfin_module.c
	@grep -q 'NGX_HTTP_LOG_PHASE' ngx_http_httpfin_module.c
	@echo "httpfin nginx module checks passed"
