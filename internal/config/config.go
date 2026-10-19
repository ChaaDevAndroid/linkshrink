package config

import "os"

type Config struct {
	DBURL string
	Port  string
}

func Load() Config {
	return Config{
		DBURL: getEnv("DB_URL", "postgres://postgres:postgres@localhost:5432/linkshrink"),
		Port:  getEnv("PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
