package link

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

var errBoom = errors.New("boom") // a generic "something broke" error

// ---------- the fake repository ----------

// fakeRepo implements Repository in memory. Every field is either a "what to return"
// knob set by the test, or a "what happened" counter the test reads afterwards.
type fakeRepo struct {
	saveResults []error // result of the 1st, 2nd, 3rd... Save call; past the end, Save succeeds
	saveCalls   int

	findLink  Link
	findErr   error
	findCalls int

	visitErr   error
	visitCalls int

	listLinks []Link
	listErr   error
	listCalls int
	listAfter *Cursor // the cursor the service passed down
	listLimit int     // the limit the service passed down
}

// Compile-time check, same trick as in service.go: if Repository gains a method,
// this line stops compiling and tells you the fake is out of date.
var _ Repository = (*fakeRepo)(nil)

func (f *fakeRepo) Save(ctx context.Context, l Link) error {
	i := f.saveCalls
	f.saveCalls++
	if i < len(f.saveResults) {
		return f.saveResults[i]
	}
	return nil
}

func (f *fakeRepo) FindByCode(ctx context.Context, code string) (Link, error) {
	f.findCalls++
	return f.findLink, f.findErr
}

func (f *fakeRepo) RecordVisit(ctx context.Context, code string) error {
	f.visitCalls++
	return f.visitErr
}

func (f *fakeRepo) List(ctx context.Context, after *Cursor, limit int) ([]Link, error) {
	f.listCalls++
	f.listAfter = after
	f.listLimit = limit
	return f.listLinks, f.listErr
}

// makeLinks builds n links, newest first, like the real repository would return them.
func makeLinks(n int) []Link {
	base := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	out := make([]Link, n)
	for i := range out {
		out[i] = Link{
			ShortCode: fmt.Sprintf("c%05d", i),
			LongURL:   "https://example.com",
			CreatedAt: base.Add(-time.Duration(i) * time.Second),
		}
	}
	return out
}

// ---------- pure functions ----------

func TestValidCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{"lowercase and digits", "abc123", true},
		{"mixed case", "ABCxyz", true},
		{"only digits", "000000", true},
		{"empty", "", false},
		{"too short", "abc12", false},
		{"too long", "abc1234", false},
		{"dash", "abc-12", false},
		{"space", "abc 12", false},
		{"newline", "abc12\n", false},
		{"favicon", "favicon.ico", false},
		{"robots", "robots.txt", false},
		// 6 BYTES but only 5 characters: the length check passes, the charset check must catch it.
		{"multibyte character", "ábcde", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidCode(tt.code); got != tt.want {
				t.Errorf("ValidCode(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	const prefix = "https://a.com/"
	tests := []struct {
		name    string
		url     string
		wantErr error // nil means "must be valid"
	}{
		{"http", "http://example.com", nil},
		{"https with path and query", "https://example.com/a/b?x=1&y=2#frag", nil},
		{"port", "http://localhost:8080/x", nil},
		{"exactly the max length", prefix + strings.Repeat("a", MaxURLLength-len(prefix)), nil},
		{"one over the max length", prefix + strings.Repeat("a", MaxURLLength-len(prefix)+1), ErrInvalidURL},
		{"empty", "", ErrInvalidURL},
		{"no scheme", "example.com", ErrInvalidURL},
		{"ftp scheme", "ftp://example.com", ErrInvalidURL},
		{"javascript scheme", "javascript:alert(1)", ErrInvalidURL},
		{"scheme but no host", "http://", ErrInvalidURL},
		{"relative path", "/just/a/path", ErrInvalidURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)
			// errors.Is(nil, nil) is true, so ONE check covers both "expected valid" and
			// "expected this exact sentinel error".
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateURL(%.40q) = %v, want %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// ---------- Create ----------

func TestService_Create(t *testing.T) {
	const goodURL = "https://example.com/page"

	tests := []struct {
		name          string
		url           string
		saveResults   []error
		wantErr       error
		wantSaveCalls int
	}{
		{"free code on the first try", goodURL, nil, nil, 1},
		{"one collision then success", goodURL, []error{ErrCodeTaken}, nil, 2},
		{"four collisions then success", goodURL, []error{ErrCodeTaken, ErrCodeTaken, ErrCodeTaken, ErrCodeTaken}, nil, 5},
		// The service must refuse BEFORE touching storage: zero Save calls.
		{"invalid url never reaches the repository", "ftp://example.com", nil, ErrInvalidURL, 0},
		// A non-collision error must stop immediately: retrying a broken database is pointless.
		{"database error stops at once and is wrapped", goodURL, []error{errBoom}, errBoom, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{saveResults: tt.saveResults}
			svc := NewService(repo)

			got, err := svc.Create(context.Background(), tt.url)

			// errors.Is also sees through fmt.Errorf("...: %w", err), so this proves the
			// error is WRAPPED, not replaced.
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if repo.saveCalls != tt.wantSaveCalls {
				t.Errorf("Save called %d times, want %d", repo.saveCalls, tt.wantSaveCalls)
			}
			if tt.wantErr != nil {
				return
			}
			if got.LongURL != tt.url {
				t.Errorf("LongURL = %q, want %q", got.LongURL, tt.url)
			}
			if !ValidCode(got.ShortCode) {
				t.Errorf("ShortCode %q is not a valid code", got.ShortCode)
			}
			if got.CreatedAt.IsZero() {
				t.Error("CreatedAt was not set")
			}
		})
	}
}

func TestService_Create_GivesUpAfterMaxAttempts(t *testing.T) {
	collisions := make([]error, maxCodeAttempts)
	for i := range collisions {
		collisions[i] = ErrCodeTaken
	}
	repo := &fakeRepo{saveResults: collisions}

	_, err := NewService(repo).Create(context.Background(), "https://example.com")

	if err == nil {
		t.Fatal("expected an error after every attempt collided")
	}
	if repo.saveCalls != maxCodeAttempts {
		t.Errorf("Save called %d times, want exactly %d", repo.saveCalls, maxCodeAttempts)
	}
	// This is a SERVER problem, not a client problem: it must not look like a validation error,
	// otherwise the handler would answer 422 instead of 500.
	if errors.Is(err, ErrInvalidURL) {
		t.Error("exhausted attempts must not be reported as ErrInvalidURL")
	}
}

// ---------- Resolve and Stats ----------

func TestService_Resolve(t *testing.T) {
	found := Link{ShortCode: "abc123", LongURL: "https://example.com"}

	tests := []struct {
		name       string
		code       string
		findLink   Link
		findErr    error
		visitErr   error
		wantErr    error
		wantFinds  int
		wantVisits int
	}{
		{"malformed code never reaches the repository", "favicon.ico", Link{}, nil, nil, ErrNotFound, 0, 0},
		{"unknown code", "abc123", Link{}, ErrNotFound, nil, ErrNotFound, 1, 0},
		{"found, visit recorded", "abc123", found, nil, nil, nil, 1, 1},
		// "A failed counter must not break the redirect": the user still gets their link.
		{"visit counter fails, redirect still works", "abc123", found, nil, errBoom, nil, 1, 1},
		{"database error on lookup", "abc123", Link{}, errBoom, nil, errBoom, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{findLink: tt.findLink, findErr: tt.findErr, visitErr: tt.visitErr}

			got, err := NewService(repo).Resolve(context.Background(), tt.code)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if repo.findCalls != tt.wantFinds {
				t.Errorf("FindByCode called %d times, want %d", repo.findCalls, tt.wantFinds)
			}
			if repo.visitCalls != tt.wantVisits {
				t.Errorf("RecordVisit called %d times, want %d", repo.visitCalls, tt.wantVisits)
			}
			if tt.wantErr == nil && got.ShortCode != tt.findLink.ShortCode {
				t.Errorf("got link %q, want %q", got.ShortCode, tt.findLink.ShortCode)
			}
		})
	}
}

func TestService_Stats(t *testing.T) {
	found := Link{ShortCode: "abc123", LongURL: "https://example.com", ClickCount: 7}

	tests := []struct {
		name      string
		code      string
		findLink  Link
		findErr   error
		wantErr   error
		wantFinds int
	}{
		{"malformed code never reaches the repository", "robots.txt", Link{}, nil, ErrNotFound, 0},
		{"unknown code", "abc123", Link{}, ErrNotFound, ErrNotFound, 1},
		{"found", "abc123", found, nil, nil, 1},
		{"database error", "abc123", Link{}, errBoom, errBoom, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{findLink: tt.findLink, findErr: tt.findErr}

			got, err := NewService(repo).Stats(context.Background(), tt.code)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if repo.findCalls != tt.wantFinds {
				t.Errorf("FindByCode called %d times, want %d", repo.findCalls, tt.wantFinds)
			}
			// The documented contract of Stats: reading statistics is NOT a visit. Always 0.
			if repo.visitCalls != 0 {
				t.Errorf("RecordVisit called %d times, Stats must never record a visit", repo.visitCalls)
			}
			if tt.wantErr == nil && got.ClickCount != found.ClickCount {
				t.Errorf("ClickCount = %d, want %d", got.ClickCount, found.ClickCount)
			}
		})
	}
}

// ---------- List ----------

func TestService_List_InvalidLimit(t *testing.T) {
	for _, limit := range []int{-1, 0, MaxPageSize + 1} {
		t.Run(fmt.Sprintf("limit=%d", limit), func(t *testing.T) {
			repo := &fakeRepo{}

			_, err := NewService(repo).List(context.Background(), limit, nil)

			if !errors.Is(err, ErrInvalidLimit) {
				t.Errorf("err = %v, want ErrInvalidLimit", err)
			}
			if repo.listCalls != 0 {
				t.Error("the repository must not be called with an invalid limit")
			}
		})
	}
}

// This is the test that proves the "limit+1" trick from Step 1.
func TestService_List_Paging(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		repoRows  int // how many rows the fake repository returns
		wantItems int
		wantNext  bool
	}{
		{"empty table", 3, 0, 0, false},
		{"fewer rows than the limit", 3, 2, 2, false},
		// The boundary case: exactly `limit` rows means NO next page. Off-by-one bugs live here.
		{"exactly the limit", 3, 3, 3, false},
		{"one extra row means there is a next page", 3, 4, 3, true},
		{"limit of 1", 1, 2, 1, true},
		{"maximum limit", MaxPageSize, MaxPageSize + 1, MaxPageSize, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := makeLinks(tt.repoRows)
			repo := &fakeRepo{listLinks: rows}
			after := &Cursor{CreatedAt: time.Now(), ShortCode: "zzzzzz"}

			page, err := NewService(repo).List(context.Background(), tt.limit, after)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// The service must ask the repository for ONE MORE row than the caller wants.
			if repo.listLimit != tt.limit+1 {
				t.Errorf("repository asked for %d rows, want %d", repo.listLimit, tt.limit+1)
			}
			// The cursor must travel down to the repository untouched (same pointer).
			if repo.listAfter != after {
				t.Error("the cursor was not passed to the repository")
			}
			if len(page.Links) != tt.wantItems {
				t.Fatalf("got %d links, want %d", len(page.Links), tt.wantItems)
			}
			if !tt.wantNext {
				if page.Next != nil {
					t.Errorf("Next = %+v, want nil", page.Next)
				}
				return
			}
			if page.Next == nil {
				t.Fatal("Next is nil, want a cursor")
			}
			// The cursor must point at the LAST link of the page, not at the extra row.
			// If it pointed at the extra row, that row would be skipped on the next page.
			last := rows[tt.limit-1]
			if page.Next.ShortCode != last.ShortCode || !page.Next.CreatedAt.Equal(last.CreatedAt) {
				t.Errorf("Next = %+v, want the position of %q", page.Next, last.ShortCode)
			}
		})
	}
}

func TestService_List_RepositoryError(t *testing.T) {
	repo := &fakeRepo{listErr: errBoom}

	_, err := NewService(repo).List(context.Background(), 10, nil)

	if !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want it to wrap errBoom", err)
	}
}
