package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"httpfin/internal/baseline"
	"httpfin/internal/demo"
	"httpfin/internal/fingerprint"
	"httpfin/internal/ingest"
	"httpfin/internal/model"
	"httpfin/internal/policy"
	"httpfin/internal/securitylog"
	"httpfin/internal/tlsinspect"
)

func main() {
	// Demo data is opt-in. A production collector must never present synthetic
	// observations as if they came from the edge.
	var initial []model.Connection
	if os.Getenv("httpfin_demo") == "1" {
		initial = demo.Connections()
	}
	store := model.NewStore(initial)
	clientBaseline, err := baseline.Load(os.Getenv("httpfin_baseline_file"))
	if err != nil {
		log.Printf("baseline disabled: %v", err)
	}
	security, err := securitylog.New(os.Getenv("httpfin_security_log"))
	if err != nil {
		log.Printf("cannot start security log: %v", err)
		return
	}
	defer security.Close()
	nginxEventHandler := func(c model.Connection) {
		c.DetectedProfile = fingerprint.ProfileFromUA(c.UserAgent)
		c = clientBaseline.Observe(c)
		store.Add(c)
		_ = security.RecordRequest(c.ClientIP, c.URL, c.Status)
		decision := policy.Evaluate(c)
		_ = security.RecordBotDecision(c.ClientIP, c.URL, decision.Action, decision.Reason, decision.Confidence)
	}
	stopIngest, ingestHealthy, err := ingest.StartUnixDatagramWithHandler(os.Getenv("httpfin_socket"), store, nginxEventHandler)
	if err != nil {
		log.Printf("cannot start ingest: %v", err)
		return
	}
	defer stopIngest()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/connections", connectionsHandler(store))
	mux.HandleFunc("/api/report", reportHandler(store))
	mux.HandleFunc("/healthz", healthHandler(ingestHealthy))
	if os.Getenv("httpfin_test_endpoint") == "1" {
		mux.HandleFunc("/__httpfin_test/unauthorized", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
	}
	mux.Handle("/", webHandler())

	// The collector API is intended to be reached through Nginx or a private
	// control plane. Bind loopback by default; explicitly opt into a wider bind.
	addr := envOr("httpfin_addr", "127.0.0.1:8080")
	log.Printf("httpfin inspector listening on http://%s", addr)
	errCh := make(chan error, 2)
	plainServer := &http.Server{
		Addr:              addr,
		Handler:           requestLogger(mux, store, security),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	go func() { errCh <- plainServer.ListenAndServe() }()
	var tlsServer *http.Server
	var tlsListener net.Listener
	if cert, key, ok := tlsinspect.EnvTLSConfigured(); ok {
		observer := tlsinspect.NewObserver()
		config, err := tlsinspect.Config(cert, key, observer)
		if err != nil {
			log.Printf("TLS disabled: cannot load certificate: %v", err)
		} else {
			tlsAddr := envOr("httpfin_tls_addr", ":8443")
			log.Printf("httpfin inspector listening on https://%s (SNI + ALPN capture enabled)", tlsAddr)
			tlsServer = &http.Server{
				Addr:              tlsAddr,
				Handler:           NewTLSHandler(mux, store, observer, security, clientBaseline),
				TLSConfig:         config,
				ReadHeaderTimeout: 5 * time.Second,
				ReadTimeout:       15 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       60 * time.Second,
				MaxHeaderBytes:    64 << 10,
			}
			tlsListener, err = tls.Listen("tcp", tlsAddr, config)
			if err != nil {
				log.Printf("TLS listener failed: %v", err)
				tlsServer = nil
			} else {
				go func() { errCh <- tlsServer.Serve(tlsListener) }()
			}
		}
	} else {
		log.Printf("TLS disabled: set httpfin_tls_cert and httpfin_tls_key to enable HTTPS/SNI capture")
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server stopped: %v", err)
		}
	case sig := <-signals:
		log.Printf("received %s, shutting down", sig)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = plainServer.Shutdown(ctx)
	if tlsServer != nil {
		_ = tlsServer.Shutdown(ctx)
	}
	if tlsListener != nil {
		_ = tlsListener.Close()
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
