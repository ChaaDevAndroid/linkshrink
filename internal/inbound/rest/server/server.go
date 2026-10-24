package server

import (
	"net/http"

	"golinkshrink.com/linkshrink/internal/domain/link"
	"golinkshrink.com/linkshrink/internal/inbound/rest/api"
	"golinkshrink.com/linkshrink/internal/inbound/rest/docs"
	"golinkshrink.com/linkshrink/internal/inbound/rest/middleware"
)

func New(linkService link.Service, enableDocs bool) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthCheck)
	api.NewLinksHandler(linkService).Register(mux)

	// root is assigned below. The docs runner needs the final handler (with middleware),
	// but that handler needs mux, which docs.Register is still filling in.
	// The closure reads root at request time, so the order resolves itself.
	var root http.Handler
	if enableDocs {
		docs.Register(mux, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			root.ServeHTTP(w, r)
		}))
	}

	root = middleware.RequestID(middleware.Logging(middleware.JSONErrors(middleware.Recovery(mux))))
	return root
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("LinkShrink is alive"))
}
