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
	if enableDocs {
		docs.Register(mux)
	}
	api.NewLinksHandler(linkService).Register(mux)

	return middleware.Logging(middleware.Recovery(mux))
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("LinkShrink is alive"))
}
