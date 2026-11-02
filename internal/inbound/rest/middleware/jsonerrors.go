package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"golinkshrink.com/linkshrink/internal/inbound/rest/httperr"
)

// errorRewriter replaces plain-text error responses (written by the router or by
// http.Error) with the standard JSON error format. Responses that are already JSON,
// and all non-error responses, pass through untouched.
type errorRewriter struct {
	http.ResponseWriter
	wroteHeader bool
	rewriting   bool
}

func (e *errorRewriter) WriteHeader(status int) {
	if e.wroteHeader {
		return
	}
	e.wroteHeader = true

	// Our own handlers already send JSON through httperr.Write; only foreign errors
	// (router 404/405, http.Error) are plain text and need rewriting.
	isJSON := strings.HasPrefix(e.Header().Get("Content-Type"), "application/json")
	if status >= 400 && !isJSON {
		e.rewriting = true
		e.Header().Set("Content-Type", "application/json")
		e.Header().Del("Content-Length") // the JSON body has a different length than the original text
		e.ResponseWriter.WriteHeader(status)

		body := httperr.Body{Error: httperr.Detail{
			Status:    status,
			Code:      httperr.CodeFor(status),
			Message:   http.StatusText(status),
			RequestID: e.Header().Get(httperr.RequestIDHeader),
		}}
		json.NewEncoder(e.ResponseWriter).Encode(body)
		return
	}
	e.ResponseWriter.WriteHeader(status)
}

func (e *errorRewriter) Write(b []byte) (int, error) {
	if !e.wroteHeader {
		e.WriteHeader(http.StatusOK)
	}
	if e.rewriting {
		return len(b), nil // drop the original plain-text body: we already sent our JSON
	}
	return e.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the original writer underneath.
func (e *errorRewriter) Unwrap() http.ResponseWriter {
	return e.ResponseWriter
}

func JSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&errorRewriter{ResponseWriter: w}, r)
	})
}
