package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// requestIDHeader is set by middleware.RequestID before any handler runs.
const requestIDHeader = "X-Request-ID"

// Stable, machine-readable error codes. Clients should branch on these, not on messages.
const (
	codeInvalidJSON   = "invalid_json"
	codeBodyTooLarge  = "body_too_large"
	codeInvalidURL    = "invalid_url"
	codeInvalidLimit  = "invalid_limit"
	codeInvalidCursor = "invalid_cursor"
	codeNotFound      = "not_found"
	codeInternal      = "internal_error"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// requestID reads the ID the middleware put on the response headers.
func requestID(w http.ResponseWriter) string {
	return w.Header().Get(requestIDHeader)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := errorBody{Error: errorDetail{
		Status:    status,
		Code:      code,
		Message:   message,
		RequestID: requestID(w),
	}}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("write error response failed", "request_id", requestID(w), "error", err)
	}
}
