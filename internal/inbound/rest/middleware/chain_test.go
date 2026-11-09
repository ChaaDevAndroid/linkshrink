package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golinkshrink.com/linkshrink/internal/inbound/rest/httperr"
)

// panicking is a handler that always crashes, like a bug in production would.
var panicking = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	panic("something went very wrong")
})

func serve(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec
}

// The order used in server.go: JSONErrors OUTSIDE Recovery.
func TestPanicBecomesStandardJSON(t *testing.T) {
	rec := serve(RequestID(JSONErrors(Recovery(panicking))))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON", ct)
	}
	var b httperr.Body
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("body is not the standard JSON: %v\n%s", err, rec.Body.String())
	}
	if b.Error.Code != httperr.CodeInternal {
		t.Errorf("code = %q, want %q", b.Error.Code, httperr.CodeInternal)
	}
	if b.Error.RequestID == "" || b.Error.RequestID != rec.Header().Get(httperr.RequestIDHeader) {
		t.Errorf("request_id %q must match the response header", b.Error.RequestID)
	}
}

// The WRONG order: Recovery OUTSIDE JSONErrors. This test documents why the order matters.
func TestWrongOrderLeavesPlainText(t *testing.T) {
	rec := serve(RequestID(Recovery(JSONErrors(panicking))))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q: with the wrong order the client gets plain text, not JSON", ct)
	}
}
