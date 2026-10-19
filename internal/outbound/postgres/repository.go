package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golinkshrink.com/linkshrink/internal/domain/link"
)

type repository struct {
	db *sql.DB
}

// Compile-time check: fails the build if *repository stops satisfying link.Repository.
var _ link.Repository = (*repository)(nil)

func NewRepository(db *sql.DB) link.Repository {
	return &repository{db: db}
}

func (r *repository) Save(ctx context.Context, l link.Link) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO links (short_code, long_url, created_at) VALUES ($1, $2, $3)",
		l.ShortCode, l.LongURL, l.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save link: %w", err)
	}
	return nil
}

func (r *repository) FindByCode(ctx context.Context, code string) (link.Link, error) {
	var l link.Link
	err := r.db.QueryRowContext(ctx,
		"SELECT short_code, long_url, created_at FROM links WHERE short_code = $1",
		code,
	).Scan(&l.ShortCode, &l.LongURL, &l.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return link.Link{}, link.ErrNotFound
		}
		return link.Link{}, fmt.Errorf("find link: %w", err)
	}
	return l, nil
}
