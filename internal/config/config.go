package config

import (
	"os"
	"strings"
)

type Config struct {
	Env              string
	HTTPAddr         string
	DatabaseURL      string
	JWTSecret        string
	CORSOrigin       string
	FrontendURL      string
	MPesaShortCode   string
	MPesaCallbackURL string
	MPesaAutoApprove bool
}

func Load() Config {
	return Config{
		Env:              get("APP_ENV", "development"),
		HTTPAddr:         get("HTTP_ADDR", ":8080"),
		DatabaseURL:      get("DATABASE_URL", "file:restaurant.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_busy_timeout=5000"),
		JWTSecret:        get("JWT_SECRET", "dev-secret-change-me"),
		CORSOrigin:       get("CORS_ORIGIN", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:4173,http://127.0.0.1:4173"),
		FrontendURL:      get("FRONTEND_BASE_URL", "http://localhost:5173"),
		MPesaShortCode:   get("MPESA_SHORT_CODE", "174379"),
		MPesaCallbackURL: get("MPESA_CALLBACK_URL", ""),
		MPesaAutoApprove: getBool("MPESA_AUTO_APPROVE", true),
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

func getBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
