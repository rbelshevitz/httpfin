package main

import (
	"encoding/json"
	"fmt"
	"httpfin/internal/model"
	"httpfin/internal/report"
	"net/http"
	"time"
)

func connectionsHandler(store *model.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, store.List())
	}
}
func reportHandler(store *model.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		report.Write(w, store.List())
	}
}
func snapshotHandler(store *model.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, report.BuildSnapshot(store.List(), time.Now()))
	}
}
func htmlReportHandler(store *model.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		report.WriteHTML(w, report.BuildSnapshot(store.List(), time.Now()), false)
	}
}

func materialSymbolsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(report.MaterialSymbolsData())
}

func eventsHandler(store *model.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		stream, cancel := store.Subscribe()
		defer cancel()
		_, _ = fmt.Fprint(w, "event: ready\ndata: {}\n\n")
		flusher.Flush()
		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case c, ok := <-stream:
				if !ok {
					return
				}
				payload, err := json.Marshal(c)
				if err != nil {
					continue
				}
				_, _ = fmt.Fprintf(w, "event: observation\ndata: %s\n\n", payload)
				flusher.Flush()
			case <-heartbeat.C:
				_, _ = fmt.Fprint(w, ": heartbeat\n\n")
				flusher.Flush()
			}
		}
	}
}
func healthHandler(ingestHealthy func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if ingestHealthy != nil && !ingestHealthy() {
			http.Error(w, "ingest unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
