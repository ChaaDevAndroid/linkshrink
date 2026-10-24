package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

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
		// 23505 is Postgres's code for a unique violation (duplicate short_code).
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
			return link.ErrCodeTaken
		}
		return fmt.Errorf("save link: %w", err)
	}
	return nil
}

func (r *repository) FindByCode(ctx context.Context, code string) (link.Link, error) {
	var l link.Link
	err := r.db.QueryRowContext(ctx,
		"SELECT short_code, long_url, created_at, click_count FROM links WHERE short_code = $1",
		code,
	).Scan(&l.ShortCode, &l.LongURL, &l.CreatedAt, &l.ClickCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return link.Link{}, link.ErrNotFound
		}
		return link.Link{}, fmt.Errorf("find link: %w", err)
	}
	return l, nil
}

func (r *repository) RecordVisit(ctx context.Context, code string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() // safety net: a no-op after a successful Commit

	res, err := tx.ExecContext(ctx,
		"UPDATE links SET click_count = click_count + 1 WHERE short_code = $1", code)
	if err != nil {
		return fmt.Errorf("increment click count: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return link.ErrNotFound
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO visits (short_code) VALUES ($1)", code); err != nil {
		return fmt.Errorf("insert visit: %w", err)
	}

	return tx.Commit()
}
