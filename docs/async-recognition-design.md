# Async document recognition — design options

Status: **proposal / not yet implemented.** Written to capture the idea so we can
implement it later.

## Problem

Today the upload pipeline is **synchronous**. `POST /v0/documents/upload/{id}`
calls `DocumentService.Upload`, which inside the same HTTP request:

1. saves the file to MinIO (`Store.Save`)
2. creates the `documents` row with status `processing`
3. calls the AI recognizer **inline** (`applyRecognition` → `Recognizer.Recognize`)
4. writes extracted data, sets status `pending_review`
5. returns the recognized document in the HTTP response

See `project/internal/usecase/document_service.go:62` (`Upload`) and
`:114` (`applyRecognition`).

This works, but the client's upload request is blocked for the entire AI call.
With the real `auditai` service (LLM invoice parsing) that can be many seconds,
risking HTTP timeouts, no retry on transient AI failures, and no way to upload a
batch quickly.

`Specification.md` already anticipates this: it describes the lifecycle as
`uploaded → processing → processed/failed` driven by a background worker, and
explicitly says **no message broker for MVP**.

Goal: decouple "file is stored" from "file is recognized" so upload returns
immediately and recognition happens in the background.

---

## Target flow (any option)

```
client → POST upload → save to MinIO + create row (status=uploaded) → 202 + doc
                                          │
                                          ▼  (background)
                          worker picks job → call AI → store extracted
                                          │
                              status: processing → pending_review / failed
                                          │
client polls GET /v0/documents/{id} ──────┘  (sees pending_review when done)
   → edit (PATCH) / confirm / reject   (unchanged)
```

Client-visible changes:

- Upload returns **202 Accepted** with the doc in status `uploaded` (or
  `processing`), **without** extracted fields yet.
- Client **polls** `GET /v0/documents/{id}` (already exists) until status is
  `pending_review` or `failed`. SSE/WebSocket push is a later optional add.
- Editing / confirm / reject endpoints are **unchanged**.

New status: add `uploaded` as the initial state before `processing`. Existing
states (`processing`, `pending_review`, `confirmed`, `failed`, `rejected`) stay.

---

## Option A — Goroutine + in-process channel queue

Spawn a background goroutine pool at startup; `Upload` pushes a job
(`documentID`) onto a buffered channel after saving the file, then returns.

```go
type recognitionJob struct{ documentID int64 }

// started in cmd/api/main.go
func (w *RecognitionWorker) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case job := <-w.jobs:
            w.process(ctx, job.documentID) // load doc, open from MinIO, recognize, store
        }
    }
}
```

**Pros**
- Smallest change. No new infra, no DB schema change.
- Fits the MVP "goroutine-based worker" suggestion in `Specification.md`.

**Cons**
- **Not durable.** If the process restarts while a job is in-flight (or still in
  the channel), that document is stuck in `processing`/`uploaded` forever.
- No retry/visibility. Backpressure = a full channel blocks the uploader.
- Single-instance only; doesn't survive horizontal scaling.

Verdict: fine for a quick demo, weak for a thesis defense because it loses the
"idempotent processing / safe retries" requirement from the spec.

---

## Option B — DB-backed job queue + worker loop (RECOMMENDED for MVP)

Use the `documents.status` column itself (or a small `recognition_jobs` table) as
a durable queue. A worker loop polls for work.

**Minimal version (no new table):** drive it off `status`.

1. `Upload` saves file, creates row `status=uploaded`, returns 202.
2. Worker loop every ~1s runs:
   ```sql
   UPDATE documents SET status='processing', updated_at=now()
   WHERE id = (
     SELECT id FROM documents
     WHERE status='uploaded'
     ORDER BY created_at
     FOR UPDATE SKIP LOCKED
     LIMIT 1
   )
   RETURNING id;
   ```
   `FOR UPDATE SKIP LOCKED` makes it safe with multiple workers/instances.
3. Worker opens the file from MinIO, calls `Recognizer.Recognize`, writes
   extracted data, sets `pending_review`. On error → `failed` (+ attempt count).

**Robust version:** add a `recognition_jobs` table:

```sql
CREATE TABLE recognition_jobs (
    id           BIGSERIAL PRIMARY KEY,
    document_id  BIGINT NOT NULL REFERENCES documents(id),
    status       TEXT NOT NULL DEFAULT 'queued',  -- queued|running|done|failed
    attempts     INT  NOT NULL DEFAULT 0,
    max_attempts INT  NOT NULL DEFAULT 3,
    last_error   TEXT,
    run_after    TIMESTAMPTZ NOT NULL DEFAULT now(), -- for backoff retries
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON recognition_jobs (status, run_after);
```

Retry = on failure, `attempts++`, set `run_after = now() + backoff`, status back
to `queued` until `attempts >= max_attempts` → `failed`.

**Pros**
- **Durable**: survives restarts; jobs picked up again on boot.
- **Idempotent + retries with backoff** — matches the spec requirements directly.
- `SKIP LOCKED` gives safe concurrency and horizontal scaling for free.
- No new infrastructure — we already run PostgreSQL.

**Cons**
- Polling adds a tiny constant DB load (negligible at MVP scale; tune interval).
- Slightly more code than Option A (a repo method + worker loop + 1 migration).

Verdict: **best balance** for this project. Durable and demonstrable without
adding a broker, and it's exactly what `Specification.md` hints at
("simple job queue in database / background worker loop"). `LISTEN/NOTIFY` can
later replace polling to cut latency without changing the model.

---

## Option C — NATS (or JetStream) message broker

`Upload` publishes a `document.uploaded` message; a separate consumer service
recognizes and writes results. JetStream gives persistence + acks + redelivery.

**Pros**
- Proper decoupling; recognition can be its own deployable service.
- Built-in durability, retries, acks, backpressure, fan-out to multiple consumers.
- The clean "future evolution" target — `Specification.md` lists NATS explicitly
  under future, not-MVP work.

**Cons**
- New infrastructure to run, secure, and reason about (broker + consumer service).
- Explicitly **out of MVP scope** per the spec.
- Overkill for current single-backend, single-DB scale; more moving parts to
  break during a demo.

Verdict: right long-term answer, wrong for now. Design Option B so the seam
(a `RecognitionQueue` port) can later be backed by NATS with no use-case changes.

---

## Comparison

| Criterion            | A: Goroutine | B: DB queue | C: NATS |
|----------------------|:------------:|:-----------:|:-------:|
| Durable / restart-safe | ✗          | ✓           | ✓       |
| Retries + backoff    | ✗            | ✓           | ✓       |
| Multi-instance safe  | ✗            | ✓ (SKIP LOCKED) | ✓   |
| New infrastructure   | none         | none        | broker + consumer |
| Implementation cost  | lowest       | low–medium  | high    |
| In MVP scope (spec)  | yes          | yes         | **no**  |

---

## Recommendation

Implement **Option B (DB-backed queue + worker loop)**, designed behind a port so
it can be swapped for NATS (Option C) later without touching use-case logic.

Suggested seam:

```go
// ports
type RecognitionQueue interface {
    Enqueue(ctx context.Context, documentID int64) error
}
```

- `DocumentService.Upload` calls `queue.Enqueue` instead of `applyRecognition`,
  then returns the doc in status `uploaded`.
- `applyRecognition` moves into the worker, mostly unchanged.
- MVP backs `RecognitionQueue` with the DB/`status` implementation; a future NATS
  adapter implements the same interface.

### Rough implementation checklist

- [ ] Add `uploaded` to the status set; `Upload` creates rows as `uploaded`.
- [ ] Add `RecognitionQueue` port + DB-backed implementation (Option B minimal or
      with `recognition_jobs` table + migration).
- [ ] Extract recognition into a worker (`internal/usecase` worker +
      `cmd/api/main.go` startup goroutine with graceful shutdown via existing
      signal context).
- [ ] Change the upload handler to return **202 Accepted**.
- [ ] Keep `GET /v0/documents/{id}` for polling; document the contract.
- [ ] Tests: upload returns `uploaded`; worker transitions to `pending_review`;
      AI failure → `failed` with retry; idempotent re-processing.
- [ ] (Optional, later) `LISTEN/NOTIFY` to reduce poll latency; SSE for push.
```
