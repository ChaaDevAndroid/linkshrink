package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Stable, machine-readable error codes. Clients should branch on these, not on messages.
const (
	codeInvalidJSON  = "invalid_json"
	codeBodyTooLarge = "body_too_large"
	codeInvalidURL   = "invalid_url"
	codeNotFound     = "not_found"
	codeInternal     = "internal_error"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := errorBody{Error: errorDetail{Status: status, Code: code, Message: message}}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("write error response failed", "error", err)
	}
}
