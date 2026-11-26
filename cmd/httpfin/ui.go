package main

import (
	"httpfin/internal/model"
	"httpfin/internal/report"
	"net/http"
	"time"
)

func webHandler(stores ...*model.Store) http.Handler {
	store := model.NewStore(nil)
	if len(stores) > 0 && stores[0] != nil {
		store = stores[0]
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		report.WriteHTML(w, report.BuildSnapshot(store.List(), time.Now()), true)
	})
}
