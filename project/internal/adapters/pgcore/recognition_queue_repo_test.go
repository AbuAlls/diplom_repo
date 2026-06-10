//go:build integration

package pgcore

import (
	"context"
	"testing"
	"time"
)

// seedDocForQueue inserts the minimal chain (user → plan → goal → item → folder → document)
// needed for a recognition_jobs row (which FKs to documents).
func seedDocForQueue(t *testing.T, ctx context.Context, s *Store) int64 {
	t.Helper()
	userID := seedUser(t, ctx, s, "queue_user@test.com")
	_, folder := seedFolder(t, ctx, s, userID)
	r := NewDocumentRepo(s)
	return seedDocument(t, ctx, r, userID, folder, "uploaded")
}

func TestRecognitionQueueRepo_EnqueueAndClaim(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	q := NewRecognitionQueueRepo(store)

	docID := seedDocForQueue(t, ctx, store)

	if err := q.Enqueue(ctx, docID); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	job, ok, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if !ok {
		t.Fatalf("expected a claimable job")
	}
	if job.DocumentID != docID {
		t.Fatalf("wrong document_id: got %d want %d", job.DocumentID, docID)
	}
	if job.MaxAttempts != 3 {
		t.Fatalf("expected max_attempts 3, got %d", job.MaxAttempts)
	}

	// Second Claim on same queue → nothing left (SKIP LOCKED works).
	_, ok, err = q.Claim(ctx)
	if err != nil {
		t.Fatalf("second Claim: %v", err)
	}
	if ok {
		t.Fatalf("second Claim must return ok=false (job is running)")
	}
}

func TestRecognitionQueueRepo_Complete(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	q := NewRecognitionQueueRepo(store)

	docID := seedDocForQueue(t, ctx, store)
	_ = q.Enqueue(ctx, docID)
	job, _, _ := q.Claim(ctx)

	if err := q.Complete(ctx, job.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Completed job must not be claimable again.
	_, ok, _ := q.Claim(ctx)
	if ok {
		t.Fatalf("completed job must not be reclaimable")
	}
}

func TestRecognitionQueueRepo_Fail_Requeues(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	q := NewRecognitionQueueRepo(store)

	docID := seedDocForQueue(t, ctx, store)
	_ = q.Enqueue(ctx, docID)
	job, _, _ := q.Claim(ctx)

	// Fail with 0 backoff → immediately runnable again.
	if err := q.Fail(ctx, job.ID, "ai timeout", 0); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	// Job should be re-queued (attempts=1, status=queued).
	var attempts int
	var status string
	err := store.Pool.QueryRow(ctx,
		`select attempts, status from recognition_jobs where id = $1`, job.ID,
	).Scan(&attempts, &status)
	if err != nil {
		t.Fatalf("query after Fail: %v", err)
	}
	if attempts != 1 || status != "queued" {
		t.Fatalf("expected attempts=1 status=queued, got attempts=%d status=%q", attempts, status)
	}

	// Should be immediately claimable again (backoff=0).
	job2, ok, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim after Fail: %v", err)
	}
	if !ok {
		t.Fatalf("re-queued job must be claimable immediately (backoff=0)")
	}
	if job2.Attempts != 1 {
		t.Fatalf("expected attempts=1 on retry, got %d", job2.Attempts)
	}
}

func TestRecognitionQueueRepo_Fail_GivesUpAtMaxAttempts(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	q := NewRecognitionQueueRepo(store)

	docID := seedDocForQueue(t, ctx, store)
	_ = q.Enqueue(ctx, docID)

	// Exhaust all 3 attempts.
	for i := 0; i < 3; i++ {
		job, ok, err := q.Claim(ctx)
		if err != nil || !ok {
			t.Fatalf("attempt %d: Claim err=%v ok=%v", i, err, ok)
		}
		if err := q.Fail(ctx, job.ID, "permanent failure", 0); err != nil {
			t.Fatalf("attempt %d: Fail: %v", i, err)
		}
	}

	// After 3 failures the job must be marked failed (not re-queued).
	var status string
	var attempts int
	err := store.Pool.QueryRow(ctx,
		`select status, attempts from recognition_jobs where document_id = $1`, docID,
	).Scan(&status, &attempts)
	if err != nil {
		t.Fatalf("query final state: %v", err)
	}
	if status != "failed" {
		t.Fatalf("expected status=failed after %d attempts, got %q", attempts, status)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}

	// No more claimable jobs.
	_, ok, _ := q.Claim(ctx)
	if ok {
		t.Fatalf("exhausted job must not be claimable")
	}
}

func TestRecognitionQueueRepo_Fail_BackoffRespected(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	q := NewRecognitionQueueRepo(store)

	docID := seedDocForQueue(t, ctx, store)
	_ = q.Enqueue(ctx, docID)
	job, _, _ := q.Claim(ctx)

	// Fail with a 1-hour backoff.
	if err := q.Fail(ctx, job.ID, "timeout", time.Hour); err != nil {
		t.Fatalf("Fail with backoff: %v", err)
	}

	// Job should NOT be claimable before the backoff elapses.
	_, ok, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim during backoff: %v", err)
	}
	if ok {
		t.Fatalf("job must not be claimable during backoff window")
	}

	// Fast-forward run_after to now.
	if _, err := store.Pool.Exec(ctx,
		`update recognition_jobs set run_after = now() - interval '1 second' where id = $1`, job.ID,
	); err != nil {
		t.Fatalf("fast-forward run_after: %v", err)
	}

	_, ok, err = q.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim after backoff elapsed: %v", err)
	}
	if !ok {
		t.Fatalf("job must be claimable after backoff elapses")
	}
}
