package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
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

	isJSON := strings.HasPrefix(e.Header().Get("Content-Type"), "application/json")
	if status >= 400 && !isJSON {
		e.rewriting = true
		e.Header().Set("Content-Type", "application/json")
		e.Header().Del("Content-Length")
		e.ResponseWriter.WriteHeader(status)

		body := errorBody{Error: errorDetail{
			Status:    status,
			Code:      codeFor(status),
			Message:   http.StatusText(status),
			RequestID: e.Header().Get(RequestIDHeader),
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
		return len(b), nil // drop the original plain-text body
	}
	return e.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the original writer underneath.
func (e *errorRewriter) Unwrap() http.ResponseWriter {
	return e.ResponseWriter
}

func codeFor(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusInternalServerError:
		return "internal_error"
	default:
		return "error"
	}
}

func JSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&errorRewriter{ResponseWriter: w}, r)
	})
}
