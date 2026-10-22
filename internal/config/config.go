package config

import (
	"os"
	"strconv"
)

type Config struct {
	DBURL      string
	Port       string
	EnableDocs bool
}

func Load() Config {
	enableDocs, _ := strconv.ParseBool(os.Getenv("ENABLE_DOCS"))

	return Config{
		DBURL:      getEnv("DB_URL", "postgres://postgres:postgres@localhost:5432/linkshrink"),
		Port:       getEnv("PORT", "8080"),
		EnableDocs: enableDocs,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
