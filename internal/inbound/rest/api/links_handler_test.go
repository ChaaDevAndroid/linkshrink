package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golinkshrink.com/linkshrink/internal/domain/link"
	"golinkshrink.com/linkshrink/internal/inbound/rest/httperr"
)

var errBoom = errors.New("boom: secret internal detail")

// ---------- the fake service ----------

// fakeService uses function fields: each test sets only the behavior it needs, and can
// capture the arguments inside its own closure. Calling an unset one panics, which is
// what you want: it means the handler called something the test did not expect.
type fakeService struct {
	create  func(ctx context.Context, longURL string) (link.Link, error)
	resolve func(ctx context.Context, code string) (link.Link, error)
	stats   func(ctx context.Context, code string) (link.Link, error)
	list    func(ctx context.Context, limit int, after *link.Cursor) (link.Page, error)

	calls int // how many service methods were called in total
}

var _ link.Service = (*fakeService)(nil)

func (f *fakeService) Create(ctx context.Context, u string) (link.Link, error) {
	f.calls++
	return f.create(ctx, u)
}
func (f *fakeService) Resolve(ctx context.Context, c string) (link.Link, error) {
	f.calls++
	return f.resolve(ctx, c)
}
func (f *fakeService) Stats(ctx context.Context, c string) (link.Link, error) {
	f.calls++
	return f.stats(ctx, c)
}
func (f *fakeService) List(ctx context.Context, limit int, after *link.Cursor) (link.Page, error) {
	f.calls++
	return f.list(ctx, limit, after)
}

// ---------- helpers ----------

// do sends one request through the real router into the handler and returns the recorded response.
// No port, no network: ServeHTTP is just a function call.
func do(svc link.Service, method, target, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	NewLinksHandler(svc).Register(mux)

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// errorOf parses the standard error envelope out of a response.
func errorOf(t *testing.T, rec *httptest.ResponseRecorder) httperr.Detail {
	t.Helper() // a failure is reported on the CALLER's line, not inside this helper
	var b httperr.Body
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("response is not the standard error JSON: %v\nbody: %s", err, rec.Body.String())
	}
	return b.Error
}

var sampleLink = link.Link{
	ShortCode:  "abc123",
	LongURL:    "https://example.com",
	CreatedAt:  time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC),
	ClickCount: 4,
}

// ---------- POST /v1/links ----------

func TestCreate_Errors(t *testing.T) {
	tooBig := `{"url":"` + strings.Repeat("a", maxBodyBytes) + `"}`

	tests := []struct {
		name       string
		body       string
		svcErr     error // what the fake service returns, if it gets called
		wantStatus int
		wantCode   string
		wantMsg    string // substring of the message; "" = don't check
		wantCalled bool   // should the service have been reached?
	}{
		// Shape errors: decodeJSON stops them, the service is never reached.
		{"empty body", ``, nil, 400, httperr.CodeInvalidJSON, "must not be empty", false},
		{"ends too early", `{"url":`, nil, 400, httperr.CodeInvalidJSON, "ends too early", false},
		{"syntax error", `{url:1}`, nil, 400, httperr.CodeInvalidJSON, "at position 2", false},
		{"wrong field type", `{"url":123}`, nil, 400, httperr.CodeInvalidJSON, "has the wrong type", false},
		{"body is an array", `[]`, nil, 400, httperr.CodeInvalidJSON, "must be a JSON object", false},
		{"unknown field", `{"urll":"https://a.com"}`, nil, 400, httperr.CodeInvalidJSON, `unknown field "urll"`, false},
		{"trailing data", `{"url":"https://a.com"}garbage`, nil, 400, httperr.CodeInvalidJSON, "single JSON object", false},
		{"body over 1 MB", tooBig, nil, 413, httperr.CodeBodyTooLarge, "", false},

		// Business and server errors: the service IS reached, the handler maps its error.
		// %w wraps like the real service does, so this also proves errors.Is works through wrapping.
		{"invalid url", `{"url":"ftp://a.com"}`, fmt.Errorf("create link: %w", link.ErrInvalidURL), 422, httperr.CodeInvalidURL, "at most 2048", true},
		{"unexpected service error", `{"url":"https://a.com"}`, errBoom, 500, httperr.CodeInternal, "internal server error", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{create: func(context.Context, string) (link.Link, error) {
				return link.Link{}, tt.svcErr
			}}

			rec := do(svc, http.MethodPost, "/v1/links", tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d\nbody: %.200s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			e := errorOf(t, rec)
			if e.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", e.Code, tt.wantCode)
			}
			if e.Status != tt.wantStatus {
				t.Errorf("status inside the body = %d, want %d", e.Status, tt.wantStatus)
			}
			if tt.wantMsg != "" && !strings.Contains(e.Message, tt.wantMsg) {
				t.Errorf("message = %q, want it to contain %q", e.Message, tt.wantMsg)
			}
			if got := svc.calls > 0; got != tt.wantCalled {
				t.Errorf("service reached = %v, want %v", got, tt.wantCalled)
			}
			// Never leak the internal error text to the client.
			if strings.Contains(rec.Body.String(), "secret internal detail") {
				t.Error("the response leaks the internal error")
			}
		})
	}
}

func TestCreate_Success(t *testing.T) {
	var gotURL string
	svc := &fakeService{create: func(_ context.Context, u string) (link.Link, error) {
		gotURL = u // capture what the handler passed to the service
		return sampleLink, nil
	}}

	rec := do(svc, http.MethodPost, "/v1/links", `{"url":"https://example.com"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if gotURL != "https://example.com" {
		t.Errorf("service got url %q", gotURL)
	}

	var got linkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if got.ShortCode != sampleLink.ShortCode || got.LongURL != sampleLink.LongURL ||
		got.ClickCount != sampleLink.ClickCount || !got.CreatedAt.Equal(sampleLink.CreatedAt) {
		t.Errorf("response = %+v, want the fields of %+v", got, sampleLink)
	}
}

// ---------- GET /v1/links ----------

func TestList_Defaults(t *testing.T) {
	var gotLimit int
	var gotAfter *link.Cursor
	svc := &fakeService{list: func(_ context.Context, limit int, after *link.Cursor) (link.Page, error) {
		gotLimit, gotAfter = limit, after
		return link.Page{}, nil // an empty page
	}}

	rec := do(svc, http.MethodGet, "/v1/links", "")

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotLimit != defaultPageSize {
		t.Errorf("limit = %d, want the default %d", gotLimit, defaultPageSize)
	}
	if gotAfter != nil {
		t.Errorf("cursor = %+v, want nil", gotAfter)
	}
	// Clients loop over items: it must be [] and not null, and next_cursor must be an explicit null.
	body := rec.Body.String()
	if !strings.Contains(body, `"items":[]`) || !strings.Contains(body, `"next_cursor":null`) {
		t.Errorf("unexpected empty-page body: %s", body)
	}
}

func TestList_BadQuery(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		svcErr     error
		wantStatus int
		wantCode   string
		wantCalled bool
	}{
		{"limit is not a number", "?limit=abc", nil, 400, httperr.CodeInvalidLimit, false},
		{"cursor is garbage", "?cursor=garbage", nil, 400, httperr.CodeInvalidCursor, false},
		{"limit out of range, caught by the domain", "?limit=0", fmt.Errorf("list links: %w", link.ErrInvalidLimit), 400, httperr.CodeInvalidLimit, true},
		{"unexpected service error", "", errBoom, 500, httperr.CodeInternal, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{list: func(context.Context, int, *link.Cursor) (link.Page, error) {
				return link.Page{}, tt.svcErr
			}}

			rec := do(svc, http.MethodGet, "/v1/links"+tt.query, "")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if e := errorOf(t, rec); e.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", e.Code, tt.wantCode)
			}
			if got := svc.calls > 0; got != tt.wantCalled {
				t.Errorf("service reached = %v, want %v", got, tt.wantCalled)
			}
		})
	}
}

func TestList_PassesLimitAndCursorToTheService(t *testing.T) {
	sent := link.Cursor{CreatedAt: time.Date(2026, 11, 2, 12, 0, 0, 123456000, time.UTC), ShortCode: "abc123"}
	var gotLimit int
	var gotAfter *link.Cursor
	svc := &fakeService{list: func(_ context.Context, limit int, after *link.Cursor) (link.Page, error) {
		gotLimit, gotAfter = limit, after
		return link.Page{}, nil
	}}

	// The cursor is URL-safe base64, so it needs no escaping in the query string.
	rec := do(svc, http.MethodGet, "/v1/links?limit=5&cursor="+encodeCursor(sent), "")

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotLimit != 5 {
		t.Errorf("limit = %d, want 5", gotLimit)
	}
	if gotAfter == nil || gotAfter.ShortCode != sent.ShortCode || !gotAfter.CreatedAt.Equal(sent.CreatedAt) {
		t.Errorf("cursor = %+v, want %+v", gotAfter, sent)
	}
}

func TestList_NextCursorRoundTrip(t *testing.T) {
	next := link.Cursor{CreatedAt: time.Date(2026, 11, 2, 11, 59, 0, 5000, time.UTC), ShortCode: "xyz789"}
	svc := &fakeService{list: func(context.Context, int, *link.Cursor) (link.Page, error) {
		return link.Page{Links: []link.Link{sampleLink}, Next: &next}, nil
	}}

	rec := do(svc, http.MethodGet, "/v1/links", "")

	var resp listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(resp.Items))
	}
	if resp.NextCursor == nil {
		t.Fatal("next_cursor is null, want a cursor")
	}
	// What the client receives must decode back to exactly the position the domain produced.
	back, err := decodeCursor(*resp.NextCursor)
	if err != nil {
		t.Fatalf("next_cursor does not decode: %v", err)
	}
	if back.ShortCode != next.ShortCode || !back.CreatedAt.Equal(next.CreatedAt) {
		t.Errorf("round trip = %+v, want %+v", back, next)
	}
}

// ---------- GET /v1/links/{code} and GET /{code} ----------

func TestCodeEndpoints_Errors(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{"stats: not found", "/v1/links/abc123", fmt.Errorf("link stats: %w", link.ErrNotFound), 404, httperr.CodeNotFound},
		{"stats: internal error", "/v1/links/abc123", errBoom, 500, httperr.CodeInternal},
		{"redirect: not found", "/abc123", fmt.Errorf("resolve link: %w", link.ErrNotFound), 404, httperr.CodeNotFound},
		{"redirect: internal error", "/abc123", errBoom, 500, httperr.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail := func(context.Context, string) (link.Link, error) { return link.Link{}, tt.svcErr }
			svc := &fakeService{stats: fail, resolve: fail}

			rec := do(svc, http.MethodGet, tt.path, "")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if e := errorOf(t, rec); e.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", e.Code, tt.wantCode)
			}
		})
	}
}

func TestStats_Success(t *testing.T) {
	svc := &fakeService{stats: func(_ context.Context, code string) (link.Link, error) {
		if code != "abc123" { // proves the {code} path value reached the service
			t.Errorf("service got code %q, want abc123", code)
		}
		return sampleLink, nil
	}}

	rec := do(svc, http.MethodGet, "/v1/links/abc123", "")

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got linkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if got.ShortCode != "abc123" || got.ClickCount != 4 {
		t.Errorf("response = %+v", got)
	}
}

func TestRedirect_Success(t *testing.T) {
	svc := &fakeService{resolve: func(context.Context, string) (link.Link, error) {
		return sampleLink, nil
	}}

	rec := do(svc, http.MethodGet, "/abc123", "")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != sampleLink.LongURL {
		t.Errorf("Location = %q, want %q", loc, sampleLink.LongURL)
	}
}

// ---------- context errors, on every endpoint ----------

func TestContextErrors_AllEndpoints(t *testing.T) {
	endpoints := []struct{ name, method, path, body string }{
		{"create", http.MethodPost, "/v1/links", `{"url":"https://a.com"}`},
		{"list", http.MethodGet, "/v1/links", ""},
		{"stats", http.MethodGet, "/v1/links/abc123", ""},
		{"redirect", http.MethodGet, "/abc123", ""},
	}
	failures := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		// Wrapped with %w like the real service does: the mapping must see through it.
		{"deadline exceeded", fmt.Errorf("op: %w", context.DeadlineExceeded), 503, httperr.CodeTimeout},
		{"client canceled", fmt.Errorf("op: %w", context.Canceled), 499, httperr.CodeCanceled},
		{"real failure", errBoom, 500, httperr.CodeInternal},
	}

	for _, ep := range endpoints {
		for _, f := range failures {
			t.Run(ep.name+"/"+f.name, func(t *testing.T) {
				one := func(context.Context, string) (link.Link, error) { return link.Link{}, f.err }
				svc := &fakeService{
					create:  one,
					resolve: one,
					stats:   one,
					list: func(context.Context, int, *link.Cursor) (link.Page, error) {
						return link.Page{}, f.err
					},
				}

				rec := do(svc, ep.method, ep.path, ep.body)

				if rec.Code != f.wantStatus {
					t.Fatalf("status = %d, want %d", rec.Code, f.wantStatus)
				}
				if e := errorOf(t, rec); e.Code != f.wantCode {
					t.Errorf("code = %q, want %q", e.Code, f.wantCode)
				}
				if strings.Contains(rec.Body.String(), "secret internal detail") {
					t.Error("the response leaks the internal error")
				}
			})
		}
	}
}
