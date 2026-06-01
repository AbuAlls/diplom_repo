package pgcore

import (
	"context"
	"time"

	"diplom.com/m/internal/ports"
)

// RecognitionQueueRepo is the durable, Postgres-backed recognition queue
// (Option B in docs/async-recognition-design.md). Producers call Enqueue; the
// worker calls Claim/Complete/Fail. Claim uses FOR UPDATE SKIP LOCKED so many
// workers (or instances) can drain the same queue without stepping on one
// another, and jobs survive restarts because they live in the database.
type RecognitionQueueRepo struct{ Store *Store }

func NewRecognitionQueueRepo(store *Store) *RecognitionQueueRepo {
	return &RecognitionQueueRepo{Store: store}
}

const defaultMaxAttempts = 3

// Enqueue inserts a queued job for documentID.
func (r *RecognitionQueueRepo) Enqueue(ctx context.Context, documentID int64) error {
	const q = `
insert into recognition_jobs (document_id, status, max_attempts)
values ($1, 'queued', $2)`
	_, err := r.Store.Pool.Exec(ctx, q, documentID, defaultMaxAttempts)
	return err
}

// Claim atomically reserves the oldest runnable job (queued and past its
// run_after) and flips it to running. Returns ok=false when there is nothing to
// do. SKIP LOCKED lets concurrent workers each grab a different row.
func (r *RecognitionQueueRepo) Claim(ctx context.Context) (ports.RecognitionJob, bool, error) {
	const q = `
update recognition_jobs
set status = 'running', updated_at = now()
where id = (
	select id from recognition_jobs
	where status = 'queued' and run_after <= now()
	order by run_after, id
	for update skip locked
	limit 1
)
returning id, document_id, attempts, max_attempts`
	var job ports.RecognitionJob
	err := r.Store.Pool.QueryRow(ctx, q).Scan(&job.ID, &job.DocumentID, &job.Attempts, &job.MaxAttempts)
	if err != nil {
		if mapErr(err) == ports.ErrNotFound {
			return ports.RecognitionJob{}, false, nil
		}
		return ports.RecognitionJob{}, false, err
	}
	return job, true, nil
}

// Complete marks a running job done.
func (r *RecognitionQueueRepo) Complete(ctx context.Context, jobID int64) error {
	const q = `update recognition_jobs set status = 'done', last_error = null, updated_at = now() where id = $1`
	_, err := r.Store.Pool.Exec(ctx, q, jobID)
	return err
}

// Fail records a failed attempt. It increments attempts and, while attempts are
// below max_attempts, re-queues the job to run again after retryIn (backoff);
// once the cap is reached the job is marked failed and will not be retried.
func (r *RecognitionQueueRepo) Fail(ctx context.Context, jobID int64, cause string, retryIn time.Duration) error {
	const q = `
update recognition_jobs
set attempts   = attempts + 1,
    last_error = $2,
    status     = case when attempts + 1 >= max_attempts then 'failed' else 'queued' end,
    run_after  = now() + ($3 || ' milliseconds')::interval,
    updated_at = now()
where id = $1`
	_, err := r.Store.Pool.Exec(ctx, q, jobID, cause, retryIn.Milliseconds())
	return err
}

var _ ports.RecognitionQueue = (*RecognitionQueueRepo)(nil)
