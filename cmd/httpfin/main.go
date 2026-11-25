package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"time"

	"httpfin/internal/demo"
	"httpfin/internal/ingest"
	"httpfin/internal/model"
	"httpfin/internal/tlsinspect"
)

func main() {
	store := model.NewStore(demo.Connections())
	stopIngest, err := ingest.StartUnixDatagram(os.Getenv("httpfin_socket"), store)
	if err != nil {
		log.Fatal(err)
	}
	defer stopIngest()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/connections", connectionsHandler(store))
	mux.HandleFunc("/api/report", reportHandler(store))
	mux.HandleFunc("/healthz", healthHandler)
	mux.Handle("/", webHandler())

	addr := envOr("httpfin_addr", ":8080")
	log.Printf("httpfin inspector listening on http://%s", addr)
	errCh := make(chan error, 2)
	plainServer := &http.Server{
		Addr:              addr,
		Handler:           requestLogger(mux, store),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	go func() { errCh <- plainServer.ListenAndServe() }()
	if cert, key, ok := tlsinspect.EnvTLSConfigured(); ok {
		observer := tlsinspect.NewObserver()
		config, err := tlsinspect.Config(cert, key, observer)
		if err != nil {
			log.Fatal(err)
		}
		tlsAddr := envOr("httpfin_tls_addr", ":8443")
		log.Printf("httpfin inspector listening on https://%s (SNI + ALPN capture enabled)", tlsAddr)
		server := &http.Server{
			Addr:              tlsAddr,
			Handler:           NewTLSHandler(mux, store, observer),
			TLSConfig:         config,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    64 << 10,
		}
		go func() {
			listener, err := tls.Listen("tcp", tlsAddr, config)
			if err != nil {
				errCh <- err
				return
			}
			errCh <- server.Serve(listener)
		}()
	} else {
		log.Printf("TLS disabled: set httpfin_tls_cert and httpfin_tls_key to enable HTTPS/SNI capture")
	}
	log.Fatal(<-errCh)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
