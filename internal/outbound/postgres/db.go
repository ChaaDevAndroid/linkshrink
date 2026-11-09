package postgres

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Pool limits. A *sql.DB is not one connection, it is a POOL of connections.
const (
	// Hard cap. Postgres allows 100 connections by default and every app instance
	// (plus migrations, DBeaver, psql) shares them. Unlimited is how you take the database down.
	maxOpenConns = 25
	// Equal to the cap: the default idle limit is 2, so under load the pool would
	// constantly close and reopen connections (a TCP + auth handshake each time).
	maxIdleConns = 25
	// Recycle connections periodically, so a firewall or failover can't leave dead ones forever.
	connMaxLifetime = 30 * time.Minute
	// Give unused connections back to Postgres after a quiet period.
	connMaxIdleTime = 5 * time.Minute

	pingTimeout = 5 * time.Second
)

func NewDB(ctx context.Context, dataSourceName string) (*sql.DB, error) {
	// sql.Open does NOT connect: it only validates the arguments and builds the pool.
	db, err := sql.Open("pgx", dataSourceName)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	// The first real connection happens here. The deadline stops an unreachable
	// database from hanging startup. The parent ctx also lets Ctrl+C interrupt it.
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}