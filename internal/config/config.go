package config

import (
	"os"
	"strings"
)

type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   string
	CORSOrigin  string
	FrontendURL string
}

func Load() Config {
	return Config{
		Env:         get("APP_ENV", "development"),
		HTTPAddr:    get("HTTP_ADDR", ":8080"),
		DatabaseURL: get("DATABASE_URL", "file:restaurant.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_busy_timeout=5000"),
		JWTSecret:   get("JWT_SECRET", "dev-secret-change-me"),
		CORSOrigin:  get("CORS_ORIGIN", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:5174,http://127.0.0.1:5174"),
		FrontendURL: get("FRONTEND_BASE_URL", ""),
	}
}

func (c Config) AllowedOrigins() []string {
	parts := strings.Split(c.CORSOrigin, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func get(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
