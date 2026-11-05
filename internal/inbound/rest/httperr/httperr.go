// Package httperr owns the one JSON error format of the REST adapter.
// It imports nothing from this project, so both api and middleware can use it
// without creating an import cycle.
package httperr

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// RequestIDHeader is set by middleware.RequestID before any handler runs.
const RequestIDHeader = "X-Request-ID"

// Stable, machine-readable error codes. Clients branch on these, never on messages.
const (
	CodeBadRequest       = "bad_request"
	CodeInvalidJSON      = "invalid_json"
	CodeBodyTooLarge     = "body_too_large"
	CodeInvalidURL       = "invalid_url"
	CodeInvalidLimit     = "invalid_limit"
	CodeInvalidCursor    = "invalid_cursor"
	CodeNotFound         = "not_found"
	CodeMethodNotAllowed = "method_not_allowed"
	CodeUnsupportedMedia = "unsupported_media_type"
	CodeUnprocessable    = "unprocessable_entity"
	CodeInternal         = "internal_error"
	CodeUnknown          = "error"
	CodeTimeout          = "timeout"
	CodeCanceled         = "request_canceled"
)

type Body struct {
	Error Detail `json:"error"`
}

type Detail struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// Write sends the standard error response.
func Write(w http.ResponseWriter, status int, code, message string) {
	// Content-Type must be set BEFORE WriteHeader: headers are frozen once the status is sent.
	// It also tells the JSONErrors middleware "this is already JSON, don't rewrite it".
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	body := Body{Error: Detail{
		Status:    status,
		Code:      code,
		Message:   message,
		RequestID: w.Header().Get(RequestIDHeader), // put there by the RequestID middleware
	}}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// Too late to change the status: the only useful thing left is to log it.
		slog.Error("write error response failed", "request_id", w.Header().Get(RequestIDHeader), "error", err)
	}
}

// CodeFor maps a bare HTTP status to a code. Used by middleware for errors that
// did not come from our handlers (the router's 404/405, http.Error in Recovery).
func CodeFor(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusMethodNotAllowed:
		return CodeMethodNotAllowed
	case http.StatusRequestEntityTooLarge:
		return CodeBodyTooLarge
	case http.StatusUnsupportedMediaType:
		return CodeUnsupportedMedia
	case http.StatusUnprocessableEntity:
		return CodeUnprocessable
	case http.StatusInternalServerError:
		return CodeInternal
	default:
		return CodeUnknown
	}
}
