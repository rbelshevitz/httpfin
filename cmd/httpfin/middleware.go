package main

import (
	"bufio"
	"fmt"
	"httpfin/internal/baseline"
	"httpfin/internal/fingerprint"
	"httpfin/internal/model"
	"httpfin/internal/policy"
	"httpfin/internal/securitylog"
	"httpfin/internal/tlsinspect"
	"net"
	"net/http"
	"strings"
	"time"
)

func requestLogger(next http.Handler, store *model.Store, security *securitylog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		_ = security.RecordRequest(fingerprint.ClientIP(r), r.URL.RequestURI(), recorder.status)
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			return
		}
		ua := r.UserAgent()
		c := model.Connection{ID: fmt.Sprintf("http-%d", time.Now().UnixNano()), Timestamp: started.UTC(), ElapsedMS: time.Since(started).Milliseconds(), ClientIP: fingerprint.ClientIP(r), Method: r.Method, URL: r.URL.RequestURI(), Status: recorder.status, Bytes: int64(recorder.bytes), UserAgent: ua, Protocol: r.Proto, DetectedProfile: fingerprint.ProfileFromUA(ua), UAConsistency: "Unknown", Risk: "Info"}
		store.Add(c)
		decision := policy.Evaluate(c)
		_ = security.RecordBotDecision(c.ClientIP, c.URL, decision.Action, decision.Reason, decision.Confidence)
	})
}

// NewTLSHandler is the integration point for an HTTPS listener that terminates TLS in Go.
func NewTLSHandler(next http.Handler, store *model.Store, observer *tlsinspect.Observer, security *securitylog.Logger, clientBaseline *baseline.Set) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		ua := r.UserAgent()
		c := model.Connection{ID: fmt.Sprintf("tls-%d", time.Now().UnixNano()), Timestamp: started.UTC(), ClientIP: fingerprint.ClientIP(r), Method: r.Method, URL: r.URL.RequestURI(), UserAgent: ua, Protocol: r.Proto, DetectedProfile: fingerprint.ProfileFromUA(ua), UAConsistency: "Matched", Risk: "Low"}
		if r.TLS != nil {
			c.TLSVersion = fingerprint.TLSVersionName(r.TLS.Version)
			c.ALPN = r.TLS.NegotiatedProtocol
			c.SNI = r.TLS.ServerName
			if observer != nil {
				hello := observer.Take(r.RemoteAddr)
				c.ALPNOffered = hello.OfferedALPN
				c.ClientHelloSig = hello.Signature
			}
			c = clientBaseline.Observe(c)
		}
		writer := &captureWriter{ResponseWriter: w, c: &c}
		next.ServeHTTP(writer, r)
		c.ElapsedMS = time.Since(started).Milliseconds()
		_ = security.RecordRequest(fingerprint.ClientIP(r), r.URL.RequestURI(), c.Status)
		store.Add(c)
		decision := policy.Evaluate(c)
		_ = security.RecordBotDecision(c.ClientIP, c.URL, decision.Action, decision.Reason, decision.Confidence)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status, bytes int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

type captureWriter struct {
	http.ResponseWriter
	c *model.Connection
}

func (w *captureWriter) WriteHeader(status int) {
	w.c.Status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *captureWriter) Write(p []byte) (int, error) {
	if w.c.Status == 0 {
		w.c.Status = 200
	}
	n, err := w.ResponseWriter.Write(p)
	w.c.Bytes += int64(n)
	return n, err
}
func (w *captureWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
func (w *captureWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}
func (w *captureWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
