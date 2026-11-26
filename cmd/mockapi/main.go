package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/app/api/catalog", jsonHandler(http.StatusOK, map[string]any{
		"service": "httpfin-mock-api",
		"items":   []string{"alpha", "beta", "gamma"},
	}))
	mux.HandleFunc("/app/api/orders", jsonHandler(http.StatusOK, map[string]any{
		"service": "httpfin-mock-api",
		"orders":  []string{"demo-1001", "demo-1002"},
	}))
	mux.HandleFunc("/app/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonHandler(http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})(w, r)
			return
		}
		jsonHandler(http.StatusUnauthorized, map[string]string{"error": "demo credentials rejected"})(w, r)
	})
	mux.HandleFunc("/app/", jsonHandler(http.StatusNotFound, map[string]string{"error": "mock route not found"}))

	log.Printf("httpfin mock API listening on http://127.0.0.1:9000")
	if err := http.ListenAndServe("127.0.0.1:9000", mux); err != nil {
		log.Fatal(err)
	}
}

func jsonHandler(status int, value any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
}
