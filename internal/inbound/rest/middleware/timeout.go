package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout gives every request a deadline. Everything downstream that receives
// r.Context() (service, repository, SQL) is cancelled when it passes.
//
// Why here and not in the service? "How long is a user willing to wait" is a
// delivery concern (HTTP today, a queue worker with a different limit tomorrow).
// The domain only has to RESPECT the ctx it is given, which it already does.
//
// It takes a duration and returns a middleware, so main can set it from config.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel() // frees the timer when the handler returns early; go vet checks this

			// WithContext returns a shallow copy of the request carrying the new context.
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}