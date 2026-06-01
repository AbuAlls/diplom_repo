package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	CoreDBDSN       string
	AnalysisDBDSN   string
	S3Endpoint      string
	S3Bucket        string
	S3Region        string
	S3AccessKey     string
	S3SecretKey     string
	JWTSecret       string
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// AI analytics service integration.
	RecognizerKind   string // "mock" (default) or "audit"
	AIServiceURL     string
	AIModel          string
	AIRequestTimeout time.Duration
	InternalAPIToken string // shared secret for the agent's /api/* callbacks

	// Asynchronous recognition queue.
	// RecognitionQueueKind selects the producer behind document upload:
	//   "db"      (default) — durable Postgres-backed queue + polling worker (Option B)
	//   "inproc"  — in-process goroutine pool, example-only/non-durable (Option A)
	//   "sync"    — no queue; recognize inline during upload (legacy behavior)
	RecognitionQueueKind    string
	RecognitionPollInterval time.Duration // Option B: how often the worker polls
	RecognitionRetryBackoff time.Duration // Option B: delay before retrying a failed job
	RecognitionWorkers      int           // Option A: number of goroutine workers

	// CORSAllowedOrigin is the Access-Control-Allow-Origin value sent on every
	// response. Default "*" is safe here because auth is a Bearer header (not a
	// cookie), so credentialed-CORS restrictions do not apply.
	CORSAllowedOrigin string
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
		S3Endpoint:      getenv("S3_ENDPOINT", "http://minio:9000"),
		S3Bucket:        getenv("S3_BUCKET", "docsapp"),
		S3Region:        getenv("S3_REGION", "us-east-1"),
		S3AccessKey:     getenv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:     getenv("S3_SECRET_KEY", "minioadmin"),
		JWTSecret:       getenv("JWT_SECRET", "dev-only-secret-change-me"),
		JWTIssuer:       getenv("JWT_ISSUER", "docsapp"),
		AccessTokenTTL:  getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour),

		RecognizerKind:   getenv("RECOGNIZER", "mock"),
		AIServiceURL:     getenv("AI_SERVICE_URL", "http://localhost:8000"),
		AIModel:          getenv("AI_MODEL", "yc:qwen3.5-35b-a3b"),
		AIRequestTimeout: getDuration("AI_REQUEST_TIMEOUT", 60*time.Second),
		InternalAPIToken: getenv("INTERNAL_API_TOKEN", "dev-internal-token-change-me"),

		RecognitionQueueKind:    getenv("RECOGNITION_QUEUE", "db"),
		RecognitionPollInterval: getDuration("RECOGNITION_POLL_INTERVAL", time.Second),
		RecognitionRetryBackoff: getDuration("RECOGNITION_RETRY_BACKOFF", 30*time.Second),
		RecognitionWorkers:      getInt("RECOGNITION_WORKERS", 4),

		CORSAllowedOrigin: getenv("CORS_ALLOWED_ORIGIN", "*"),
	}
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func getInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
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
