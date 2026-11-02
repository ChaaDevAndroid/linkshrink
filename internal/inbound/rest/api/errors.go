package api

import (
	"net/http"

	"golinkshrink.com/linkshrink/internal/inbound/rest/httperr"
)

// requestID reads the ID the middleware put on the response headers.
func requestID(w http.ResponseWriter) string {
	return w.Header().Get(httperr.RequestIDHeader)
}

// writeError is a short alias so handlers read "writeError(...)" instead of "httperr.Write(...)".
func writeError(w http.ResponseWriter, status int, code, message string) {
	httperr.Write(w, status, code, message)
}
