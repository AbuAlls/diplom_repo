package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddr        string
	CoreDBDSN       string
	AnalysisDBDSN   string
	StorageRootDir  string
	JWTSecret       string
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func Load() Config {
	host := getenv("DB_HOST", "db")
	port := getenv("DB_PORT", "5432")
	name := getenv("DB_NAME", "app")
	user := getenv("DB_USER", "app")
	pass := getenv("DB_PASSWORD", "app")
	ssl := getenv("DB_SSLMODE", "disable")
	coreDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, ssl)
	coreDSN = getenv("CORE_DB_DSN", coreDSN)

	return Config{
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		CoreDBDSN:       coreDSN,
		AnalysisDBDSN:   getenv("ANALYSIS_DB_DSN", coreDSN),
		StorageRootDir:  getenv("STORAGE_ROOT_DIR", "/tmp/docsapp-storage"),
		JWTSecret:       getenv("JWT_SECRET", "dev-only-secret-change-me"),
		JWTIssuer:       getenv("JWT_ISSUER", "docsapp"),
		AccessTokenTTL:  getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
	}
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func getDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
