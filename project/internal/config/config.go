package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddr             string
	CoreDBDSN            string
	AnalysisDBDSN        string
	JWTSecret            string
	JWTIssuer            string
	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
	WorkerPollInterval   time.Duration
	WorkerBatchSize      int
	WorkerMaxAttempts    int
	StorageRootDir       string
	StorageDownloadRoute string
}

func Load() Config {
	host := getenv("DB_HOST", "db")
	port := getenv("DB_PORT", "5432")
	name := getenv("DB_NAME", "app")
	user := getenv("DB_USER", "app")
	pass := getenv("DB_PASSWORD", "app")
	ssl := getenv("DB_SSLMODE", "disable")
	coreDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, ssl)

	analysisDSN := getenv("ANALYSIS_DB_DSN", coreDSN)

	return Config{
		HTTPAddr:             getenv("HTTP_ADDR", ":8080"),
		CoreDBDSN:            getenv("CORE_DB_DSN", coreDSN),
		AnalysisDBDSN:        analysisDSN,
		JWTSecret:            getenv("JWT_SECRET", "dev-only-secret-change-me"),
		JWTIssuer:            getenv("JWT_ISSUER", "docsapp"),
		AccessTokenTTL:       getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:      getDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		WorkerPollInterval:   getDuration("WORKER_POLL_INTERVAL", 2*time.Second),
		WorkerBatchSize:      getInt("WORKER_BATCH_SIZE", 10),
		WorkerMaxAttempts:    getInt("MAX_ATTEMPTS", 3),
		StorageRootDir:       getenv("STORAGE_ROOT_DIR", "/tmp/docsapp-storage"),
		StorageDownloadRoute: getenv("DOWNLOAD_BASE_URL", ""),
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

func getInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	var i int
	if _, err := fmt.Sscanf(v, "%d", &i); err != nil {
		return def
	}
	return i
}
