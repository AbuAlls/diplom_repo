package sub_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"diplom.com/m/internal/adapters/pganalysis"
	"diplom.com/m/internal/adapters/pgcore"
	"diplom.com/m/internal/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// =====================================================================
// Real-database integration tests for the persistence layer.
//
// These connect to an actual Postgres, provision an ISOLATED throwaway
// database, apply the committed migrations into its public schema (the
// AnalyticsQueryRepo hardcodes `public`), exercise every pgcore/pganalysis
// repository, and drop the database afterwards.
//
// They are SKIPPED automatically when no Postgres is reachable, so the default
// `go test ./...` stays green on machines without a database. To run them:
//
//	docker compose up -d db          # or: make up
//	SUB_TEST_DB_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' \
//	    go test ./internal/sub_test/ -run TestDB -v
//
// If SUB_TEST_DB_DSN is unset, the docker-compose default DSN above is tried.
// =====================================================================

const defaultDBDSN = "postgres://app:app@localhost:5432/app?sslmode=disable"

// dbEnv is a fully-migrated throwaway database plus ready-to-use stores.
type dbEnv struct {
	core     *pgcore.Store
	analysis *pganalysis.Store
	cleanup  func()
}

// setupDB connects, creates an isolated database, migrates it, and returns
// stores bound to it. It calls t.Skip when Postgres is unreachable.
func setupDB(t *testing.T) *dbEnv {
	t.Helper()
	dsn := os.Getenv("SUB_TEST_DB_DSN")
	if dsn == "" {
		dsn = defaultDBDSN
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	adminCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	admin, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Skipf("Postgres unavailable (pool): %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Skipf("Postgres unreachable at %s: %v — skipping DB integration tests", redact(dsn), err)
	}

	dbName := fmt.Sprintf("subtest_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+dbName+`"`); err != nil {
		admin.Close()
		t.Skipf("cannot CREATE DATABASE (insufficient privileges?): %v", err)
	}

	// Build a pool/conn bound to the new database.
	scopedCfg := adminCfg.Copy()
	scopedCfg.ConnConfig.Database = dbName

	// Apply migrations over a single simple-protocol connection (the .sql files
	// contain multiple statements per Exec).
	migCfg := scopedCfg.ConnConfig.Copy()
	migCfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	migConn, err := pgx.ConnectConfig(ctx, migCfg)
	if err != nil {
		dropDB(admin, dbName)
		admin.Close()
		t.Fatalf("connect to %s: %v", dbName, err)
	}
	for _, rel := range []string{
		"../../migrations/core/000001_init_core.up.sql",
		"../../migrations/analysis/000001_init_analysis.up.sql",
	} {
		sqlBytes, err := os.ReadFile(filepath.Clean(rel))
		if err != nil {
			migConn.Close(ctx)
			dropDB(admin, dbName)
			admin.Close()
			t.Fatalf("read migration %s: %v", rel, err)
		}
		if _, err := migConn.Exec(ctx, string(sqlBytes)); err != nil {
			migConn.Close(ctx)
			dropDB(admin, dbName)
			admin.Close()
			t.Fatalf("apply migration %s: %v", rel, err)
		}
	}
	migConn.Close(ctx)

	// Repo pool (extended protocol / parameterized queries, like production).
	pool, err := pgxpool.NewWithConfig(context.Background(), scopedCfg)
	if err != nil {
		dropDB(admin, dbName)
		admin.Close()
		t.Fatalf("repo pool: %v", err)
	}

	env := &dbEnv{
		core:     &pgcore.Store{Pool: pool},
		analysis: &pganalysis.Store{Pool: pool},
	}
	env.cleanup = func() {
		pool.Close()
		dropDB(admin, dbName)
		admin.Close()
	}
	return env
}

func dropDB(admin *pgxpool.Pool, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// FORCE (PG13+) terminates any lingering connections.
	_, _ = admin.Exec(ctx, `DROP DATABASE IF EXISTS "`+name+`" WITH (FORCE)`)
}

func redact(dsn string) string {
	if i := indexByte(dsn, '@'); i >= 0 {
		if j := indexByte(dsn, '/'); j >= 0 && j < i {
			return dsn[:j+2] + "***" + dsn[i:]
		}
	}
	return dsn
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// TestDB_CoreRepositories drives the full core hierarchy through the real repos:
// user → plan → goal → item (+update) → folder → document (+fields/status/arrays)
// and the document counters, all against Postgres.
func TestDB_CoreRepositories(t *testing.T) {
	env := setupDB(t)
	defer env.cleanup()
	ctx := context.Background()

	users := pgcore.NewUserRepo(env.core)
	plans := pgcore.NewPlanRepo(env.core)
	goals := pgcore.NewGoalRepo(env.core)
	items := pgcore.NewPlanItemRepo(env.core)
	folders := pgcore.NewFolderRepo(env.core)
	docs := pgcore.NewDocumentRepo(env.core)

	// users
	uid, err := users.Create(ctx, "db@example.com", "DB User", "hash")
	if err != nil {
		t.Fatalf("user create: %v", err)
	}
	if u, err := users.GetByEmail(ctx, "db@example.com"); err != nil || u.ID != uid {
		t.Fatalf("user get by email: %v (u=%+v)", err, u)
	}
	if _, err := users.GetByEmail(ctx, "nope@example.com"); err != ports.ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing user, got %v", err)
	}

	// plan + ownership listing
	plan, err := plans.Create(ctx, uid, "DB Plan", ptr("desc"), "active")
	if err != nil {
		t.Fatalf("plan create: %v", err)
	}
	if list, total, err := plans.ListByOwner(ctx, uid, 0, 10); err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("plan list: err=%v total=%d n=%d", err, total, len(list))
	}

	// goal
	goal, err := goals.Create(ctx, plan.ID, "DB Goal", nil, 3)
	if err != nil {
		t.Fatalf("goal create: %v", err)
	}
	if goal.SortOrder != 3 {
		t.Fatalf("goal sort_order: %d", goal.SortOrder)
	}

	// item + update; check numeric persistence
	target, current := 200.0, 50.0
	item, err := items.Create(ctx, goal.ID, ports.PlanItemInput{
		Name: "DB Item", ItemType: "metric", Status: "active",
		TargetValue: &target, CurrentValue: &current, SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("item create: %v", err)
	}
	newCurrent := 100.0
	upd, err := items.Update(ctx, item.ID, ports.PlanItemPatch{CurrentValue: &newCurrent})
	if err != nil {
		t.Fatalf("item update: %v", err)
	}
	if upd.CurrentValue == nil || *upd.CurrentValue != 100 {
		t.Fatalf("item current_value not persisted: %v", upd.CurrentValue)
	}
	if p := upd.ProgressPercent(); p == nil || *p != 50 {
		t.Fatalf("expected progress 50 (100/200), got %v", p)
	}

	// folder is created on first use and idempotent thereafter
	fid, err := folders.FindOrCreateItemFolder(ctx, item.ID, uid)
	if err != nil {
		t.Fatalf("folder create: %v", err)
	}
	if fid2, err := folders.FindOrCreateItemFolder(ctx, item.ID, uid); err != nil || fid2 != fid {
		t.Fatalf("folder not idempotent: fid=%d fid2=%d err=%v", fid, fid2, err)
	}

	// document create — plan_item_id is resolved through the folder join
	size := int64(len("bytes"))
	doc, err := docs.Create(ctx, ports.DocumentCreate{
		PlanItemID: item.ID, FolderID: fid, UploadedBy: uid,
		Title: "doc.pdf", Status: "pending_review",
		FileName: "doc.pdf", FilePath: "plan_item_1/doc.pdf", MimeType: "application/pdf", FileSize: &size,
	})
	if err != nil {
		t.Fatalf("doc create: %v", err)
	}
	if doc.PlanItemID != item.ID {
		t.Fatalf("doc plan_item_id not resolved via folder: got %d want %d", doc.PlanItemID, item.ID)
	}

	// UpdateFields with Postgres array columns (deadlines DATE[], prices TEXT[],
	// quantities INT[]) and scalar fields
	deadline := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	org := "ООО Тест"
	updatedDoc, err := docs.UpdateFields(ctx, doc.ID, ports.DocumentPatch{
		OrganizationName: &org,
		Deadlines:        []time.Time{deadline},
		Prices:           []string{"100.50", "200.00"},
		Quantities:       []int32{1, 2, 3},
		ProductNames:     []string{"A", "B"},
	})
	if err != nil {
		t.Fatalf("doc update fields: %v", err)
	}
	if updatedDoc.OrganizationName == nil || *updatedDoc.OrganizationName != org {
		t.Fatalf("organization_name not persisted: %v", updatedDoc.OrganizationName)
	}
	if len(updatedDoc.Prices) != 2 || len(updatedDoc.Quantities) != 3 || len(updatedDoc.Deadlines) != 1 {
		t.Fatalf("array columns not persisted: %+v", updatedDoc)
	}

	// status transition
	if d, err := docs.UpdateStatus(ctx, doc.ID, "confirmed"); err != nil || d.Status != "confirmed" {
		t.Fatalf("update status: err=%v status=%q", err, d.Status)
	}

	// counters / latest
	if n, err := docs.CountByPlanItem(ctx, item.ID); err != nil || n != 1 {
		t.Fatalf("count by item: err=%v n=%d", err, n)
	}
	if latest, err := docs.LatestDocIDByPlanItem(ctx, item.ID); err != nil || latest == nil || *latest != doc.ID {
		t.Fatalf("latest doc id: err=%v latest=%v", err, latest)
	}

	// owner-scoped listing with plan_item filter
	list, total, err := docs.ListByOwner(ctx, uid, &item.ID, 0, 10)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("list by owner: err=%v total=%d n=%d", err, total, len(list))
	}
}

// TestDB_AnalysisAndScoping exercises the analysis-side ExtractedDataRepo and,
// crucially, the AnalyticsQueryRepo's per-tenant CTE scoping against real SQL:
// two owners' documents are inserted, and a scoped read must only ever return
// the initiating owner's rows.
func TestDB_AnalysisAndScoping(t *testing.T) {
	env := setupDB(t)
	defer env.cleanup()
	ctx := context.Background()

	users := pgcore.NewUserRepo(env.core)
	plans := pgcore.NewPlanRepo(env.core)
	goals := pgcore.NewGoalRepo(env.core)
	items := pgcore.NewPlanItemRepo(env.core)
	folders := pgcore.NewFolderRepo(env.core)
	docs := pgcore.NewDocumentRepo(env.core)
	extracted := pganalysis.NewExtractedDataRepo(env.analysis)
	query := pgcore.NewAnalyticsQueryRepo(env.core)

	// Build a document for owner A and another for owner B.
	mkDoc := func(email, title string) (ownerID, docID int64) {
		uid, err := users.Create(ctx, email, email, "h")
		if err != nil {
			t.Fatalf("user: %v", err)
		}
		plan, err := plans.Create(ctx, uid, "P", nil, "active")
		if err != nil {
			t.Fatalf("plan: %v", err)
		}
		goal, err := goals.Create(ctx, plan.ID, "G", nil, 0)
		if err != nil {
			t.Fatalf("goal: %v", err)
		}
		item, err := items.Create(ctx, goal.ID, ports.PlanItemInput{Name: "I", ItemType: "generic", Status: "active"})
		if err != nil {
			t.Fatalf("item: %v", err)
		}
		fid, err := folders.FindOrCreateItemFolder(ctx, item.ID, uid)
		if err != nil {
			t.Fatalf("folder: %v", err)
		}
		doc, err := docs.Create(ctx, ports.DocumentCreate{
			PlanItemID: item.ID, FolderID: fid, UploadedBy: uid,
			Title: title, Status: "pending_review",
			FileName: title, FilePath: "p/" + title, MimeType: "application/pdf",
		})
		if err != nil {
			t.Fatalf("doc: %v", err)
		}
		return uid, doc.ID
	}

	ownerA, docA := mkDoc("a-db@example.com", "A-doc")
	ownerB, _ := mkDoc("b-db@example.com", "B-doc")

	// --- ExtractedDataRepo round trip (analysis DB) ---
	conf := 0.93
	model := "mock-v0"
	now := time.Now()
	ed, err := extracted.Create(ctx, ports.ExtractedDataCreate{
		DocumentID: docA, RecognizedText: "hello", StructuredJSON: []byte(`{"k":"v"}`),
		ConfidenceScore: &conf, ProcessingStatus: "done", ProcessedAt: &now, ModelVersion: &model,
	})
	if err != nil {
		t.Fatalf("extracted create: %v", err)
	}
	if got, err := extracted.GetByDocumentID(ctx, docA); err != nil || got.ID != ed.ID || got.RecognizedText != "hello" {
		t.Fatalf("extracted get: err=%v got=%+v", err, got)
	}
	cat := int64(7)
	if err := extracted.UpdateCategory(ctx, docA, &cat); err != nil {
		t.Fatalf("update category: %v", err)
	}
	if got, err := extracted.GetByDocumentID(ctx, docA); err != nil || got.RecognizedCategoryID == nil || *got.RecognizedCategoryID != 7 {
		t.Fatalf("category not persisted: err=%v got=%+v", err, got)
	}
	// Batch lookup: only docA has extracted data; a missing id is simply absent.
	if m, err := extracted.ListByDocumentIDs(ctx, []int64{docA, 999999}); err != nil {
		t.Fatalf("list by document ids: %v", err)
	} else if _, ok := m[docA]; !ok || len(m) != 1 {
		t.Fatalf("batch lookup wrong: %+v", m)
	}

	// --- Schema introspection sees the public tables ---
	schema, err := query.Schema(ctx)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	for _, want := range []string{"documents", "plans", "plan_items"} {
		if len(schema[want]) == 0 {
			t.Fatalf("schema missing table %q: %+v keys", want, keysOf(schema))
		}
	}

	// --- Per-tenant scoping: owner A only sees A's document ---
	cols, rows, err := query.RunReadOnlyQuery(ctx, "SELECT id, uploaded_by FROM documents ORDER BY id", ownerA)
	if err != nil {
		t.Fatalf("scoped query A: %v", err)
	}
	if len(cols) != 2 {
		t.Fatalf("unexpected columns: %v", cols)
	}
	if len(rows) != 1 {
		t.Fatalf("owner A should see exactly 1 document, got %d", len(rows))
	}
	if ub, _ := rows[0][1].(int64); ub != ownerA {
		t.Fatalf("scoped row leaked another owner: uploaded_by=%v want %d", rows[0][1], ownerA)
	}

	// owner B likewise sees only their own
	_, rowsB, err := query.RunReadOnlyQuery(ctx, "SELECT id FROM documents", ownerB)
	if err != nil || len(rowsB) != 1 {
		t.Fatalf("owner B scoping: err=%v rows=%d", err, len(rowsB))
	}

	// unscoped (ownerID = 0) sees both documents
	_, rowsAll, err := query.RunReadOnlyQuery(ctx, "SELECT id FROM documents", 0)
	if err != nil || len(rowsAll) != 2 {
		t.Fatalf("unscoped query should see 2 docs: err=%v rows=%d", err, len(rowsAll))
	}
}

func ptr(s string) *string { return &s }

func keysOf(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
