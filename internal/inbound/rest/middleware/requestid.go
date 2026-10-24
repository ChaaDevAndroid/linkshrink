package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const RequestIDHeader = "X-Request-ID"

// ctxKey is a private type, so no other package can collide with our context key.
type ctxKey struct{}

// RequestIDFromContext returns the request ID stored by the RequestID middleware, or "".
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

func newRequestID() string {
	b := make([]byte, 8)
	rand.Read(b) // since Go 1.24 this cannot fail: it crashes the program if the OS has no randomness
	return hex.EncodeToString(b)
}

// validRequestID accepts only 8 to 64 characters of letters, digits, '-' and '_'.
// This stops a client from putting newlines or control characters into our logs.
func validRequestID(s string) bool {
	if len(s) < 8 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID(id) {
			id = newRequestID()
		}

		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), ctxKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
