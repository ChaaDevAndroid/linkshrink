package middleware

import (
	"net/http"
	"testing"
	"time"
)

func TestTimeout_SetsADeadline(t *testing.T) {
	var deadline time.Time
	var has bool
	h := Timeout(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, has = r.Context().Deadline()
	}))

	start := time.Now()
	serve(h)

	if !has {
		t.Fatal("the request context has no deadline")
	}
	// The deadline is "now + 50ms", measured from when the middleware ran.
	if d := deadline.Sub(start); d <= 0 || d > 50*time.Millisecond+time.Second {
		t.Errorf("deadline is %v after the start, want about 50ms", d)
	}
}

// The reason the middleware exists: slow work is told to stop at the deadline,
// instead of running for as long as it wants.
func TestTimeout_ReleasesSlowWork(t *testing.T) {
	h := Timeout(20 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done(): // what database/sql does while a query runs
			w.WriteHeader(http.StatusServiceUnavailable)
		case <-time.After(2 * time.Second): // the "slow query"
			w.WriteHeader(http.StatusOK)
		}
	}))

	start := time.Now()
	rec := serve(h)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503: the deadline did not reach the handler", rec.Code)
	}
	if time.Since(start) > time.Second {
		t.Errorf("took %v, the work should have been cut at about 20ms", time.Since(start))
	}
}
