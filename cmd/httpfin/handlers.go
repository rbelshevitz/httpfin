package main

import (
	"encoding/json"
	"httpfin/internal/model"
	"httpfin/internal/report"
	"net/http"
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
func healthHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
