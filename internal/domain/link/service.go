package link

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"strings"
	"time"
)

const (
	charset         = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLength      = 6 // one source of truth for the generator and the validator
	maxCodeAttempts = 5

	// MaxPageSize is the largest page a caller may ask for.
	MaxPageSize = 100

	// MaxURLLength is the longest long_url we accept.
	MaxURLLength = 2048
)

type service struct {
	repo Repository
}

// Compile-time check: fails the build if *service stops satisfying Service.
var _ Service = (*service)(nil)

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, longURL string) (Link, error) {
	if err := validateURL(longURL); err != nil {
		return Link{}, err
	}

	for attempt := 0; attempt < maxCodeAttempts; attempt++ {
		l := NewLink(longURL)

		err := s.repo.Save(ctx, l)
		if err == nil {
			return l, nil
		}
		if errors.Is(err, ErrCodeTaken) {
			continue // collision: generate a new code and try again
		}
		return Link{}, fmt.Errorf("create link: %w", err)
	}
	return Link{}, fmt.Errorf("create link: no free code after %d attempts", maxCodeAttempts)
}

func (s *service) List(ctx context.Context, limit int, after *Cursor) (Page, error) {
	if limit < 1 || limit > MaxPageSize {
		return Page{}, ErrInvalidLimit
	}

	// Ask for one extra row. If it comes back, there is a next page,
	// and we never need a separate COUNT query.
	links, err := s.repo.List(ctx, after, limit+1)
	if err != nil {
		return Page{}, fmt.Errorf("list links: %w", err)
	}

	page := Page{Links: links}
	if len(links) > limit {
		page.Links = links[:limit]
		last := page.Links[limit-1]
		page.Next = &Cursor{CreatedAt: last.CreatedAt, ShortCode: last.ShortCode}
	}
	return page, nil
}

func generateCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func (s *service) Resolve(ctx context.Context, code string) (Link, error) {
	// A string that can never be a code is "not found" without asking the database.
	if !ValidCode(code) {
		return Link{}, fmt.Errorf("resolve link: %w", ErrNotFound)
	}

	l, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return Link{}, fmt.Errorf("resolve link: %w", err)
	}

	// A failed counter must not break the redirect: log it and carry on.
	if err := s.repo.RecordVisit(ctx, code); err != nil {
		slog.Warn("record visit failed", "code", code, "error", err)
	}
	return l, nil
}

func (s *service) Stats(ctx context.Context, code string) (Link, error) {
	if !ValidCode(code) {
		return Link{}, fmt.Errorf("link stats: %w", ErrNotFound)
	}

	l, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return Link{}, fmt.Errorf("link stats: %w", err)
	}
	return l, nil
}

func validateURL(raw string) error {
	// Length first: it is the cheapest check, and it avoids parsing a huge string.
	if len(raw) > MaxURLLength {
		return ErrInvalidURL
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrInvalidURL
	}
	return nil
}

// ValidCode reports whether s has the shape of a short code we generate.
func ValidCode(s string) bool {
	if len(s) != codeLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(charset, rune(s[i])) {
			return false
		}
	}
	return true
}

func NewLink(longURL string) Link {
	return Link{
		ShortCode: generateCode(codeLength),
		LongURL:   longURL,
		CreatedAt: time.Now(),
	}
}
