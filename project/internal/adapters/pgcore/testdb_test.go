//go:build integration

package pgcore

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testDB spins up a real Postgres connection for integration tests.
//
// It reads POSTGRES_TEST_DSN (default: postgres://app:app@localhost:5432/app?sslmode=disable).
// If the database is unreachable, the test is skipped — so "go test ./..." always works
// without Docker; integration tests only run when Postgres is available:
//
//	go test -tags integration ./internal/adapters/pgcore/...
//	# or from the project root:
//	make integration-test
//
// Each call creates an isolated schema (pgtest_<hex>) and applies all core + analysis
// migrations into it, then registers a cleanup that drops the schema.
func testDB(t *testing.T) *Store {
	t.Helper()

	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://app:app@localhost:5432/app?sslmode=disable"
	}

	ctx := context.Background()

	// Quick reachability probe — skip instead of failing when Postgres isn't up.
	probe, err := pgxpool.New(ctx, dsn)
	if err != nil || probe.Ping(ctx) != nil {
		if probe != nil {
			probe.Close()
		}
		t.Skipf("postgres not reachable (%v) — set POSTGRES_TEST_DSN or run 'make up'; skipping integration test", err)
	}
	probe.Close()

	// Create a unique schema so concurrent/parallel test runs don't collide.
	schema := fmt.Sprintf("pgtest_%08x", rand.Uint32())
	adminPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer adminPool.Close()

	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
		t.Fatalf("create schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		cleanCtx := context.Background()
		if _, err := adminPool.Exec(cleanCtx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
			t.Logf("drop schema %s: %v", schema, err)
		}
	})

	// Open a pool scoped to the test schema.
	schemaDSN := dsn
	if strings.Contains(dsn, "?") {
		schemaDSN += "&search_path=" + schema
	} else {
		schemaDSN += "?search_path=" + schema
	}
	pool, err := pgxpool.New(ctx, schemaDSN)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// Apply migrations in sequence order.
	applyMigrations(t, ctx, pool, schema)

	return &Store{Pool: pool}
}

// applyMigrations reads all *.up.sql files from migrations/core and migrations/analysis
// (relative to the package directory where Go tests run) and executes them in
// alphabetical order so they apply as 000001 → 000002 → 000003.
func applyMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()

	// Tests run from the package dir: internal/adapters/pgcore/
	// Migrations live at: ../../../migrations/{core,analysis}/
	dirs := []string{
		filepath.Join("..", "..", "..", "migrations", "core"),
		filepath.Join("..", "..", "..", "migrations", "analysis"),
	}

	var files []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read migration dir %s: %v", dir, err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	// Sort by filename (000001_*.up.sql < 000002_*.up.sql …) — alphabetical within each dir,
	// but core must come before analysis because analysis has no FK to core in its own schema.
	sort.Strings(files)

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
		t.Logf("applied %s", filepath.Base(f))
	}
}
