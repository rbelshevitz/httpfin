ARG NGINX_VERSION=1.28.3
ARG ALPINE_VERSION=3.23

FROM golang:1.27-alpine AS go-build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/httpfin ./cmd/httpfin

FROM alpine:${ALPINE_VERSION} AS nginx-build
ARG NGINX_VERSION
RUN apk add --no-cache build-base linux-headers pcre2-dev zlib-dev openssl-dev curl tar
WORKDIR /src
RUN curl -fsSL "https://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz" | tar xz
COPY nginx-module /src/httpfin-module
RUN cd "/src/nginx-${NGINX_VERSION}" \
 && ./configure --prefix=/etc/nginx \
      --sbin-path=/usr/sbin/nginx \
      --modules-path=/usr/lib/nginx/modules \
      --conf-path=/etc/nginx/nginx.conf \
      --pid-path=/run/nginx.pid \
      --error-log-path=/dev/stderr \
      --http-log-path=/dev/stdout \
      --with-compat \
      --with-http_ssl_module \
      --with-http_v2_module \
      --with-stream \
      --with-stream_ssl_preread_module \
      --add-dynamic-module=/src/httpfin-module \
 && make -j2 \
 && make modules

FROM alpine:${ALPINE_VERSION}
ARG NGINX_VERSION
RUN apk add --no-cache ca-certificates openssl pcre2 zlib
COPY --from=go-build /out/httpfin /usr/local/bin/httpfin
COPY --from=nginx-build /src/nginx-${NGINX_VERSION}/objs/ngx_http_httpfin_module.so /usr/lib/nginx/modules/ngx_http_httpfin_module.so
COPY --from=nginx-build /src/nginx-${NGINX_VERSION}/objs/nginx /usr/sbin/nginx
COPY docker/nginx.conf /etc/nginx/nginx.conf
COPY docker/entrypoint.sh /entrypoint.sh
LABEL org.opencontainers.image.title="httpfin" \
      org.opencontainers.image.description="HTTP fingerprint inspector with Nginx integration" \
      org.opencontainers.image.version="${NGINX_VERSION}"
RUN chmod +x /entrypoint.sh && mkdir -p /run/httpfin /etc/nginx/certs
EXPOSE 8443
ENTRYPOINT ["/entrypoint.sh"]
