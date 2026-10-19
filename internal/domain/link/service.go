package link

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type service struct {
	repo Repository
}

// Compile-time check: fails the build if *service stops satisfying Service.
var _ Service = (*service)(nil)

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, longURL string) (Link, error) {
	l := NewLink(longURL)
	if err := s.repo.Save(ctx, l); err != nil {
		return Link{}, fmt.Errorf("create link: %w", err)
	}
	return l, nil
}

func (s *service) Resolve(ctx context.Context, code string) (Link, error) {
	l, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return Link{}, fmt.Errorf("resolve link: %w", err)
	}
	return l, nil
}

func NewLink(longURL string) Link {
	return Link{
		ShortCode: generateCode(6), //Business Logic
		LongURL:   longURL,
		CreatedAt: time.Now(), //Business Logic
	}
}

// Business Logic: generateCode generates a random string of length n using the charset
func generateCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
