package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DBURL           string
	Port            string
	EnableDocs      bool
	RequestTimeout  time.Duration // max time one request may take, enforced by a context deadline
	ShutdownTimeout time.Duration // max time to let in-flight requests finish after SIGTERM
}

func Load() (Config, error) {
	enableDocs, _ := strconv.ParseBool(os.Getenv("ENABLE_DOCS"))

	requestTimeout, err := getDuration("REQUEST_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	// 8s, not 10s: `docker stop` sends SIGKILL after 10s by default, so we must finish before that.
	shutdownTimeout, err := getDuration("SHUTDOWN_TIMEOUT", 8*time.Second)
	if err != nil {
		return Config{}, err
	}

	// The invariant that makes graceful shutdown true: every in-flight request ends
	// (success or 503) BEFORE the shutdown deadline, so nobody is cut in half.
	if shutdownTimeout <= requestTimeout {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT (%s) must be longer than REQUEST_TIMEOUT (%s)",
			shutdownTimeout, requestTimeout)
	}

	return Config{
		DBURL:           getEnv("DB_URL", "postgres://postgres:postgres@localhost:5432/linkshrink"),
		Port:            getEnv("PORT", "8080"),
		EnableDocs:      enableDocs,
		RequestTimeout:  requestTimeout,
		ShutdownTimeout: shutdownTimeout,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v) // accepts "500ms", "5s", "1m"
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration like 500ms or 5s, got %q", key, v)
	}
	return d, nil
}