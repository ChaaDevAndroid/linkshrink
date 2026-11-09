package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"golinkshrink.com/linkshrink/internal/inbound/rest/httperr"
)

// statusClientClosedRequest is nginx's non-standard code for "the client left before we answered".
// The client will never read it. It exists so OUR logs show what really happened.
const statusClientClosedRequest = 499

// requestID reads the ID the middleware put on the response headers.
func requestID(w http.ResponseWriter) string {
	return w.Header().Get(httperr.RequestIDHeader)
}

// writeError is a short alias so handlers read "writeError(...)" instead of "httperr.Write(...)".
func writeError(w http.ResponseWriter, status int, code, message string) {
	httperr.Write(w, status, code, message)
}

// writeInternal answers every error that is not the client's fault, and picks the
// right status AND the right log level:
//
//	deadline passed  -> 503 timeout, WARN   (a slow dependency: look at the database)
//	client went away -> 499,         INFO   (nothing is wrong with us)
//	anything else    -> 500,         ERROR  (a bug or a real failure)
//
// The raw error goes to the log only, never to the client.
func writeInternal(w http.ResponseWriter, r *http.Request, msg string, err error, attrs ...any) {
	args := append([]any{"request_id", requestID(w), "error", err}, attrs...)

	// We check BOTH the returned error and the request's own context. Drivers wrap
	// context errors in different ways, and if the deadline has passed, it is the cause
	// of whatever failed after it.
	ctxErr := r.Context().Err()

	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(ctxErr, context.DeadlineExceeded):
		slog.Warn(msg+" (timeout)", args...)
		writeError(w, http.StatusServiceUnavailable, httperr.CodeTimeout,
			"the server took too long to answer, please try again")

	case errors.Is(err, context.Canceled) || errors.Is(ctxErr, context.Canceled):
		slog.Info(msg+" (client canceled)", args...)
		writeError(w, statusClientClosedRequest, httperr.CodeCanceled, "request canceled")

	default:
		slog.Error(msg, args...)
		writeError(w, http.StatusInternalServerError, httperr.CodeInternal, "internal server error")
	}
}
