package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	addr := getenv("HTTP_ADDR", ":8080")

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// Lightweight checks (DB ping). For NATS/S3 you can add later.
		db, err := openDB()
		if err != nil {
			http.Error(w, "db open failed: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer db.Close()

		ctxTimeout := 2 * time.Second
		if err := pingDB(db, ctxTimeout); err != nil {
			http.Error(w, "db ping failed: "+err.Error(), http.StatusServiceUnavailable)
			return
		}

		fmt.Fprintln(w, "ok")
	})

	log.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func openDB() (*sql.DB, error) {
	host := getenv("DB_HOST", "db")
	port := getenv("DB_PORT", "5432")
	name := getenv("DB_NAME", "app")
	user := getenv("DB_USER", "app")
	pass := getenv("DB_PASSWORD", "app")
	ssl := getenv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, ssl)
	return sql.Open("pgx", dsn)
}

func pingDB(db *sql.DB, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := db.Ping(); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s", timeout)
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
