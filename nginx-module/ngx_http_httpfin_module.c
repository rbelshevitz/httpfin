/*
 * Minimal httpfin NGINX HTTP dynamic module.
 *
 * The module deliberately stays small: it observes an HTTP request at the
 * post-read/log phases and emits one bounded JSON datagram to a local Unix
 * socket. It never waits for the consumer and never owns fingerprint logic.
 */
#include <ngx_config.h>
#include <ngx_core.h>
#include <ngx_http.h>

#include <errno.h>
#include <fcntl.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#define httpfin_max_event 4096

typedef struct {
    ngx_flag_t enabled;
    ngx_str_t socket_path;
} ngx_http_httpfin_main_conf_t;

typedef struct {
    ngx_flag_t enabled;
} ngx_http_httpfin_loc_conf_t;

typedef struct {
    ngx_uint_t id;
    ngx_str_t handoff_status;
    u_char request_id[64];
} ngx_http_httpfin_ctx_t;

static ngx_int_t ngx_http_httpfin_preconfiguration(ngx_conf_t *cf);
static ngx_int_t ngx_http_httpfin_postconfiguration(ngx_conf_t *cf);
static void *ngx_http_httpfin_create_main_conf(ngx_conf_t *cf);
static char *ngx_http_httpfin_init_main_conf(ngx_conf_t *cf, void *conf);
static void *ngx_http_httpfin_create_loc_conf(ngx_conf_t *cf);
static char *ngx_http_httpfin_merge_loc_conf(ngx_conf_t *cf, void *parent,
    void *child);
static ngx_int_t ngx_http_httpfin_init_process(ngx_cycle_t *cycle);
static void ngx_http_httpfin_exit_process(ngx_cycle_t *cycle);
static ngx_int_t ngx_http_httpfin_request_init(ngx_http_request_t *r);
static ngx_int_t ngx_http_httpfin_log(ngx_http_request_t *r);
static ngx_int_t ngx_http_httpfin_var_request_id(ngx_http_request_t *r,
    ngx_http_variable_value_t *v, uintptr_t data);
static ngx_int_t ngx_http_httpfin_var_status(ngx_http_request_t *r,
    ngx_http_variable_value_t *v, uintptr_t data);
static ngx_int_t ngx_http_httpfin_emit(ngx_http_request_t *r,
    ngx_http_httpfin_ctx_t *ctx, ngx_http_httpfin_main_conf_t *mcf);
static void ngx_http_httpfin_get_variable(ngx_http_request_t *r,
    ngx_str_t *value, u_char *name, size_t name_len);
static u_char *ngx_http_httpfin_json_string(u_char *p, u_char *last,
    ngx_str_t *value);

static ngx_int_t ngx_http_httpfin_socket = -1;
static ngx_atomic_t ngx_http_httpfin_sequence;

static ngx_command_t ngx_http_httpfin_commands[] = {
    { ngx_string("httpfin"),
      NGX_HTTP_SRV_CONF|NGX_HTTP_LOC_CONF|NGX_CONF_FLAG,
      ngx_conf_set_flag_slot,
      NGX_HTTP_LOC_CONF_OFFSET,
      offsetof(ngx_http_httpfin_loc_conf_t, enabled),
      NULL },

    { ngx_string("httpfin_socket"),
      NGX_HTTP_MAIN_CONF|NGX_CONF_TAKE1,
      ngx_conf_set_str_slot,
      NGX_HTTP_MAIN_CONF_OFFSET,
      offsetof(ngx_http_httpfin_main_conf_t, socket_path),
      NULL },

      ngx_null_command
};

static ngx_http_module_t ngx_http_httpfin_module_ctx = {
    ngx_http_httpfin_preconfiguration,
    ngx_http_httpfin_postconfiguration,
    ngx_http_httpfin_create_main_conf,
    ngx_http_httpfin_init_main_conf,
    NULL,
    NULL,
    ngx_http_httpfin_create_loc_conf,
    ngx_http_httpfin_merge_loc_conf
};

ngx_module_t ngx_http_httpfin_module = {
    NGX_MODULE_V1,
    &ngx_http_httpfin_module_ctx,
    ngx_http_httpfin_commands,
    NGX_HTTP_MODULE,
    NULL,
    NULL,
    ngx_http_httpfin_init_process,
    NULL,
    NULL,
    ngx_http_httpfin_exit_process,
    NULL,
    NGX_MODULE_V1_PADDING
};

static ngx_http_variable_t ngx_http_httpfin_variables[] = {
    { ngx_string("httpfin_request_id"), NULL,
      ngx_http_httpfin_var_request_id, 0, NGX_HTTP_VAR_NOCACHEABLE, 0 },
    { ngx_string("httpfin_handoff_status"), NULL,
      ngx_http_httpfin_var_status, 0, NGX_HTTP_VAR_NOCACHEABLE, 0 },
      ngx_http_null_variable
};

static ngx_int_t
ngx_http_httpfin_preconfiguration(ngx_conf_t *cf)
{
    ngx_http_variable_t *var, *v;
    ngx_http_variable_t *added;

    for (v = ngx_http_httpfin_variables; v->name.len; v++) {
        var = ngx_http_add_variable(cf, &v->name, v->flags);
        if (var == NULL) {
            return NGX_ERROR;
        }
        var->get_handler = v->get_handler;
        var->data = v->data;
    }

    /* Keep the local copy referenced so older NGINX headers do not warn. */
    added = var;
    (void) added;
    return NGX_OK;
}

static ngx_int_t
ngx_http_httpfin_postconfiguration(ngx_conf_t *cf)
{
    ngx_http_core_main_conf_t *cmcf;
    ngx_http_handler_pt *h;

    cmcf = ngx_http_conf_get_module_main_conf(cf, ngx_http_core_module);
    h = ngx_array_push(&cmcf->phases[NGX_HTTP_POST_READ_PHASE].handlers);
    if (h == NULL) {
        return NGX_ERROR;
    }
    *h = ngx_http_httpfin_request_init;

    h = ngx_array_push(&cmcf->phases[NGX_HTTP_LOG_PHASE].handlers);
    if (h == NULL) {
        return NGX_ERROR;
    }
    *h = ngx_http_httpfin_log;
    return NGX_OK;
}

static void *
ngx_http_httpfin_create_main_conf(ngx_conf_t *cf)
{
    ngx_http_httpfin_main_conf_t *mcf;

    mcf = ngx_pcalloc(cf->pool, sizeof(*mcf));
    if (mcf == NULL) {
        return NULL;
    }
    mcf->enabled = NGX_CONF_UNSET;
    ngx_str_null(&mcf->socket_path);
    return mcf;
}

static char *
ngx_http_httpfin_init_main_conf(ngx_conf_t *cf, void *conf)
{
    ngx_http_httpfin_main_conf_t *mcf = conf;

    if (mcf->enabled == NGX_CONF_UNSET) {
        mcf->enabled = 1;
    }
    if (mcf->socket_path.len == 0) {
        ngx_str_set(&mcf->socket_path, "/run/httpfin/events.sock");
    }
    return NGX_CONF_OK;
}

static void *
ngx_http_httpfin_create_loc_conf(ngx_conf_t *cf)
{
    ngx_http_httpfin_loc_conf_t *lcf;

    lcf = ngx_pcalloc(cf->pool, sizeof(*lcf));
    if (lcf == NULL) {
        return NULL;
    }
    lcf->enabled = NGX_CONF_UNSET;
    return lcf;
}

static char *
ngx_http_httpfin_merge_loc_conf(ngx_conf_t *cf, void *parent, void *child)
{
    ngx_http_httpfin_loc_conf_t *prev = parent;
    ngx_http_httpfin_loc_conf_t *conf = child;

    ngx_conf_merge_value(conf->enabled, prev->enabled, NGX_CONF_UNSET);
    return NGX_CONF_OK;
}

static ngx_int_t
ngx_http_httpfin_init_process(ngx_cycle_t *cycle)
{
    ngx_http_httpfin_main_conf_t *mcf;
    int flags;
    struct sockaddr_un address;

    mcf = ngx_http_cycle_get_module_main_conf(cycle,
                                              ngx_http_httpfin_module);
    if (mcf == NULL) {
        ngx_log_error(NGX_LOG_WARN, cycle->log, 0,
                      "httpfin: HTTP main configuration is unavailable");
        return NGX_OK;
    }
    if (!mcf->enabled || mcf->socket_path.len == 0) {
        return NGX_OK;
    }
    if (mcf->socket_path.len >= sizeof(address.sun_path)) {
        ngx_log_error(NGX_LOG_EMERG, cycle->log, 0,
                      "httpfin_socket path is too long");
        return NGX_ERROR;
    }

    ngx_http_httpfin_socket = socket(AF_UNIX, SOCK_DGRAM, 0);
    if (ngx_http_httpfin_socket == -1) {
        ngx_log_error(NGX_LOG_WARN, cycle->log, ngx_errno,
                      "httpfin: socket() failed");
        return NGX_OK;
    }
    flags = fcntl(ngx_http_httpfin_socket, F_GETFL, 0);
    if (flags == -1 || fcntl(ngx_http_httpfin_socket, F_SETFL, flags | O_NONBLOCK) == -1) {
        ngx_log_error(NGX_LOG_WARN, cycle->log, ngx_errno,
                      "httpfin: cannot set non-blocking socket");
        close(ngx_http_httpfin_socket);
        ngx_http_httpfin_socket = -1;
        return NGX_OK;
    }

    ngx_memzero(&address, sizeof(address));
    address.sun_family = AF_UNIX;
    ngx_memcpy(address.sun_path, mcf->socket_path.data, mcf->socket_path.len);
    if (connect(ngx_http_httpfin_socket, (struct sockaddr *) &address,
                offsetof(struct sockaddr_un, sun_path) + mcf->socket_path.len + 1) == -1)
    {
        ngx_log_error(NGX_LOG_WARN, cycle->log, ngx_errno,
                      "httpfin: connect(%V) failed; events will be dropped",
                      &mcf->socket_path);
        close(ngx_http_httpfin_socket);
        ngx_http_httpfin_socket = -1;
    } else {
        ngx_log_error(NGX_LOG_NOTICE, cycle->log, 0,
                      "httpfin: connected to %V", &mcf->socket_path);
    }
    return NGX_OK;
}

static void
ngx_http_httpfin_exit_process(ngx_cycle_t *cycle)
{
    (void) cycle;
    if (ngx_http_httpfin_socket != -1) {
        close(ngx_http_httpfin_socket);
        ngx_http_httpfin_socket = -1;
    }
}

static ngx_int_t
ngx_http_httpfin_request_init(ngx_http_request_t *r)
{
    ngx_http_httpfin_loc_conf_t *lcf;
    ngx_http_httpfin_ctx_t *ctx;
    ngx_uint_t id;

    if (r != r->main) {
        return NGX_DECLINED;
    }
    lcf = ngx_http_get_module_loc_conf(r, ngx_http_httpfin_module);
    if (lcf == NULL || lcf->enabled != 1) {
        return NGX_DECLINED;
    }
    ctx = ngx_http_get_module_ctx(r, ngx_http_httpfin_module);
    if (ctx != NULL) {
        return NGX_DECLINED;
    }
    ctx = ngx_pcalloc(r->pool, sizeof(*ctx));
    if (ctx == NULL) {
        return NGX_ERROR;
    }
    id = (ngx_uint_t) ngx_atomic_fetch_add(&ngx_http_httpfin_sequence, 1);
    ctx->id = id;
    ngx_snprintf(ctx->request_id, sizeof(ctx->request_id), "%P-%ui%Z",
                 ngx_pid, id);
    ngx_str_set(&ctx->handoff_status, "not_sent");
    ngx_http_set_ctx(r, ctx, ngx_http_httpfin_module);
    return NGX_DECLINED;
}

static ngx_int_t
ngx_http_httpfin_log(ngx_http_request_t *r)
{
    ngx_http_httpfin_main_conf_t *mcf;
    ngx_http_httpfin_loc_conf_t *lcf;
    ngx_http_httpfin_ctx_t *ctx;

    if (r != r->main) {
        return NGX_DECLINED;
    }
    lcf = ngx_http_get_module_loc_conf(r, ngx_http_httpfin_module);
    if (lcf == NULL || lcf->enabled != 1) {
        return NGX_DECLINED;
    }
    ctx = ngx_http_get_module_ctx(r, ngx_http_httpfin_module);
    if (ctx == NULL) {
        return NGX_DECLINED;
    }
    mcf = ngx_http_get_module_main_conf(r, ngx_http_httpfin_module);
    (void) ngx_http_httpfin_emit(r, ctx, mcf);
    return NGX_DECLINED;
}

static ngx_int_t
ngx_http_httpfin_var_request_id(ngx_http_request_t *r,
    ngx_http_variable_value_t *v, uintptr_t data)
{
    ngx_http_httpfin_ctx_t *ctx;
    (void) data;
    ctx = ngx_http_get_module_ctx(r, ngx_http_httpfin_module);
    if (ctx == NULL) {
        v->not_found = 1;
        return NGX_OK;
    }
    v->data = ctx->request_id;
    v->len = ngx_strlen(ctx->request_id);
    v->valid = 1;
    v->no_cacheable = 1;
    return NGX_OK;
}

static ngx_int_t
ngx_http_httpfin_var_status(ngx_http_request_t *r,
    ngx_http_variable_value_t *v, uintptr_t data)
{
    ngx_http_httpfin_ctx_t *ctx;
    (void) data;
    ctx = ngx_http_get_module_ctx(r, ngx_http_httpfin_module);
    if (ctx == NULL) {
        v->not_found = 1;
        return NGX_OK;
    }
    v->data = ctx->handoff_status.data;
    v->len = ctx->handoff_status.len;
    v->valid = 1;
    v->no_cacheable = 1;
    return NGX_OK;
}

static ngx_int_t
ngx_http_httpfin_emit(ngx_http_request_t *r, ngx_http_httpfin_ctx_t *ctx,
    ngx_http_httpfin_main_conf_t *mcf)
{
    u_char event[httpfin_max_event], *p, *last;
    ngx_str_t method, uri, host, ua, remote, tls_version, sni, alpn;
    ssize_t n;

    if (ngx_http_httpfin_socket == -1) {
        ngx_str_set(&ctx->handoff_status, "unavailable");
        return NGX_DECLINED;
    }

    method = r->method_name;
    uri = r->unparsed_uri;
    ngx_str_null(&host);
    if (r->headers_in.host) {
        host = r->headers_in.host->value;
    }
    ngx_str_null(&ua);
    if (r->headers_in.user_agent) {
        ua = r->headers_in.user_agent->value;
    }
    remote = r->connection->addr_text;
    ngx_http_httpfin_get_variable(r, &tls_version,
                                  (u_char *) "ssl_protocol",
                                  sizeof("ssl_protocol") - 1);
    ngx_http_httpfin_get_variable(r, &sni,
                                  (u_char *) "ssl_server_name",
                                  sizeof("ssl_server_name") - 1);
    ngx_http_httpfin_get_variable(r, &alpn,
                                  (u_char *) "ssl_alpn_protocol",
                                  sizeof("ssl_alpn_protocol") - 1);
    p = event;
    last = event + sizeof(event);
    p = ngx_cpymem(p, "{\"version\":1,\"request_id\":\"",
                   sizeof("{\"version\":1,\"request_id\":\"") - 1);
    p = ngx_http_httpfin_json_string(p, last, &(ngx_str_t) {
        ngx_strlen(ctx->request_id), ctx->request_id });
    p = ngx_cpymem(p, "\",\"method\":\"", sizeof("\",\"method\":\"") - 1);
    p = ngx_http_httpfin_json_string(p, last, &method);
    p = ngx_cpymem(p, "\",\"uri\":\"", sizeof("\",\"uri\":\"") - 1);
    p = ngx_http_httpfin_json_string(p, last, &uri);
    p = ngx_cpymem(p, "\",\"host\":\"", sizeof("\",\"host\":\"") - 1);
    p = ngx_http_httpfin_json_string(p, last, &host);
    p = ngx_cpymem(p, "\",\"user_agent\":\"",
                   sizeof("\",\"user_agent\":\"") - 1);
    p = ngx_http_httpfin_json_string(p, last, &ua);
    p = ngx_cpymem(p, "\",\"remote_addr\":\"",
                   sizeof("\",\"remote_addr\":\"") - 1);
    p = ngx_http_httpfin_json_string(p, last, &remote);
    p = ngx_cpymem(p, "\",\"status\":", sizeof("\",\"status\":") - 1);
    p = ngx_sprintf(p, "%ui", (ngx_uint_t) r->headers_out.status);
    p = ngx_cpymem(p, ",\"tls_version\":\"",
                   sizeof(",\"tls_version\":\"") - 1);
    p = ngx_http_httpfin_json_string(p, last, &tls_version);
    p = ngx_cpymem(p, "\",\"sni\":\"",
                   sizeof("\",\"sni\":\"") - 1);
    p = ngx_http_httpfin_json_string(p, last, &sni);
    p = ngx_cpymem(p, "\",\"alpn\":\"",
                   sizeof("\",\"alpn\":\"") - 1);
    p = ngx_http_httpfin_json_string(p, last, &alpn);
    p = ngx_cpymem(p, "\"}\n", sizeof("\"}\n") - 1);
    if (p >= last) {
        ngx_str_set(&ctx->handoff_status, "oversize");
        return NGX_DECLINED;
    }
    n = send(ngx_http_httpfin_socket, event, p - event, MSG_DONTWAIT | MSG_NOSIGNAL);
    if (n == (ssize_t) (p - event)) {
        ngx_str_set(&ctx->handoff_status, "sent");
    } else if (errno == EAGAIN || errno == EWOULDBLOCK) {
        ngx_str_set(&ctx->handoff_status, "dropped_backpressure");
    } else {
        ngx_str_set(&ctx->handoff_status, "dropped_error");
    }
    (void) mcf;
    return NGX_OK;
}

static void
ngx_http_httpfin_get_variable(ngx_http_request_t *r, ngx_str_t *value,
    u_char *name, size_t name_len)
{
    ngx_str_t variable_name;
    ngx_http_variable_value_t *variable;

    variable_name.data = name;
    variable_name.len = name_len;
    variable = ngx_http_get_variable(r, &variable_name,
                                      ngx_hash_key_lc(name, name_len));
    if (variable == NULL || variable->not_found) {
        ngx_str_null(value);
        return;
    }
    value->data = variable->data;
    value->len = variable->len;
}

static u_char *
ngx_http_httpfin_json_string(u_char *p, u_char *last, ngx_str_t *value)
{
    u_char *end;
    u_char c;

    if (p >= last) {
        return last;
    }
    for (end = value->data + value->len; value->data < end && p + 2 < last;
         value->data++)
    {
        c = *value->data;
        if (c == '\\' || c == '"') {
            *p++ = '\\';
        }
        if (c == '\n' || c == '\r' || c == '\t') {
            *p++ = '\\';
            *p++ = (c == '\n') ? 'n' : (c == '\r' ? 'r' : 't');
        } else if (c < 0x20) {
            *p++ = '?';
        } else {
            *p++ = c;
        }
    }
    return p;
}
