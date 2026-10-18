package server

import "net/http"

func New() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthCheck)
	return mux
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("LinkShrink is alive"))
}
