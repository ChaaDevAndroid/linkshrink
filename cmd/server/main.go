package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golinkshrink.com/linkshrink/internal/config"
	"golinkshrink.com/linkshrink/internal/domain/link"
	"golinkshrink.com/linkshrink/internal/inbound/rest/server"
	"golinkshrink.com/linkshrink/internal/outbound/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Created FIRST, so Ctrl+C also interrupts a slow startup (the DB ping), not just the running server.
	// ctx is cancelled by SIGINT (Ctrl+C) or SIGTERM (what `docker stop` and Kubernetes send).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := postgres.Migrate(cfg.DBURL); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	db, err := postgres.NewDB(ctx, cfg.DBURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	// Deferred calls run last-in-first-out, after Shutdown below has returned:
	// first we stop serving and let in-flight requests finish, THEN we close the pool.
	// The reverse order would make those requests fail with "sql: database is closed".
	defer db.Close()

	repo := postgres.NewRepository(db)
	// 4 workers: far below the pool of 25, so counting can never starve the real requests of connections.
	// 1000 slots: absorbs a burst of about 1000 clicks while the workers catch up.
	visits := link.NewAsyncRecorder(repo, 4, 1000)
	svc := link.NewService(repo, link.WithVisitRecorder(visits))

	handler := server.New(svc, cfg.EnableDocs, cfg.RequestTimeout)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// Derived, not hard-coded: the socket must stay open LONGER than the request
		// deadline, otherwise the connection dies before we can send our 503 JSON.
		WriteTimeout: cfg.RequestTimeout + 5*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", srv.Addr)
		// ListenAndServe always returns an error. ErrServerClosed is the NORMAL one after Shutdown.
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		// Give the signal handling back to Go's default: a SECOND Ctrl+C now kills
		// the process immediately, in case the graceful shutdown gets stuck.
		stop()
		slog.Info("shutting down", "timeout", cfg.ShutdownTimeout)
	}

	// A NEW context from Background, not derived from ctx: ctx is already cancelled,
	// and a cancelled context would make Shutdown give up instantly.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// ORDER MATTERS. Three stages, each one needing the previous to be finished:
	//   1. Shutdown: no more requests come in, in-flight ones finish. After this, nobody calls Record.
	//   2. visits.Close: workers drain what is queued. They still need the database.
	//   3. db.Close (deferred earlier, runs when run() returns): the database goes away last.
	// Closing the recorder BEFORE stage 1 would reject visits from requests still running.
	shutdownErr := srv.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		srv.Close() // out of time: cut the remaining connections instead of leaving them half-open
	}

	if err := visits.Close(shutdownCtx); err != nil {
		slog.Warn("visit queue not fully drained", "error", err)
	}

	if shutdownErr != nil {
		return fmt.Errorf("graceful shutdown failed, forced close: %w", shutdownErr)
	}
	slog.Info("shutdown complete", "visits_dropped", visits.Dropped())
	return nil
}
