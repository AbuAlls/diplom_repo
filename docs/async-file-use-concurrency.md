# Concurrent async file use — implementation paths

Status: **partially implemented.** The shipping choice is **SERIALIZABLE
transactions with retry** for document status transitions (see "What we
implemented"). This document records the alternatives we weighed so the decision
is defensible and reversible.

## The problem

With corporate-account **groups**, several users from *different accounts* can
act on the **same shared document** at the same time. On top of that, the
**asynchronous recognition worker** (see `async-recognition-design.md`) is a
second writer to the same row: it transitions `uploaded → processing →
pending_review / failed` in the background.

So a single document row has multiple concurrent writers:

```
member A ─┐
member B ─┼─▶  documents.status  ◀─ recognition worker (background)
member C ─┘
```

Classic hazards:

- **Lost update / clobber.** Worker reads `processing`, decides to write
  `pending_review`. Meanwhile member A confirms (`confirmed`). If the worker's
  write lands last with no guard, it silently reverts the confirm.
- **Double action.** Two members both see `pending_review` and both confirm; or a
  member confirms while another rejects. Without atomicity both "succeed".
- **Read-modify-write races** in general: the decision ("is this confirmable?")
  is made on a value that is already stale by the time the write executes.

The request: *"make it like serializable read if async."* That is option C below,
and it is what we shipped.

---

## Path A — Pessimistic row locks (`SELECT … FOR UPDATE`)

Lock the row at the start of each transaction; other writers block until commit.

```sql
BEGIN;
SELECT status FROM documents WHERE id = $1 FOR UPDATE;  -- blocks rivals
-- decide in app code
UPDATE documents SET status = $2 WHERE id = $1;
COMMIT;
```

**Pros**
- Simple, intuitive, no retry loop — the lock serializes writers for you.
- Works at READ COMMITTED (Postgres default); no isolation-level change.
- Predictable: first writer wins, others queue.

**Cons**
- Writers **block** instead of failing fast; a slow transaction (e.g. the worker
  mid-recognition while holding the lock) stalls every member action on that doc.
  → Mitigate by locking only around the *status flip*, never across the AI call.
- Lock ordering across multiple rows can deadlock if not disciplined.
- Easy to forget the `FOR UPDATE` on one code path and reintroduce the race.

Good fit when contention is real and you want strict queueing on a single row.

---

## Path B — Optimistic concurrency (version column / conditional UPDATE)

No locks. Carry a version (or rely on the current status) and make the write
conditional; if it affects zero rows, someone else changed it first.

```sql
UPDATE documents SET status = 'confirmed', version = version + 1
WHERE id = $1 AND version = $expected;   -- 0 rows ⇒ conflict
```

or the status-conditional form we actually use:

```sql
UPDATE documents SET status = 'confirmed'
WHERE id = $1 AND status = 'pending_review';  -- 0 rows ⇒ 409 Conflict
```

**Pros**
- No blocking; conflicts surface immediately as a 409 the client can react to.
- Cheap under low contention (the common case here).
- The condition encodes the business rule ("only pending_review is confirmable")
  directly in the write, so the check and the write are atomic.

**Cons**
- Caller must handle the conflict (retry or report). For user actions that's a
  clean 409; for the worker it's "someone finished it, treat as no-op".
- A bare version column needs a schema migration and discipline to bump it
  everywhere.

This is essentially **belt** — and we wear it together with the **braces** of
option C: every conditional transition lists its allowed `expectFrom` states.

---

## Path C — SERIALIZABLE transactions with retry  ✅ shipped

Run the read-decide-write cycle at `ISOLATION LEVEL SERIALIZABLE`. Postgres
guarantees the outcome is equivalent to running the transactions one at a time;
if it cannot, it aborts one with SQLSTATE **40001** (`serialization_failure`),
which the application **retries**.

```go
for attempt := 0; attempt < maxRetries; attempt++ {
    tx := begin(SERIALIZABLE)
    status := tx.read(id)
    if !allowed(status) { return ErrConflict }   // precondition genuinely failed
    tx.update(id, newStatus)
    if err := tx.commit(); isSerializationFailure(err) {
        backoff(attempt); continue                // 40001 → retry
    }
    return
}
```

**Pros**
- **Strongest correctness with the least app-side reasoning**: you write the
  read-then-write logic naively and the database guarantees serializability. No
  manual lock placement, no version bookkeeping.
- Directly answers the requirement ("serializable read if async").
- Composes across multiple rows/tables automatically (matters if a transition
  later also touches `extracted_document_data`, artifacts, etc.).

**Cons**
- Requires a **retry loop** on 40001 (and 40P01 deadlock) — wrapped once in the
  repo so callers don't see it.
- Slightly higher abort rate under heavy contention than row locks; here
  contention per document is low, so retries are rare.
- Serialization failures can occur on transactions that "look" read-only if they
  inform a later write — must retry consistently.

---

## Path D — Single-writer queue / actor per document

Route every mutation of a document through one serialized worker (e.g. hash the
document ID to a goroutine or a partitioned queue, so all writes for a given doc
happen on one goroutine in arrival order).

**Pros**
- No DB-level contention at all; the race is eliminated structurally.
- Natural fit if we later move recognition fully onto NATS/JetStream with a
  per-key consumer.

**Cons**
- Heavy machinery for a single-process MVP; needs sticky routing and careful
  shutdown/draining.
- Doesn't help cross-instance writes unless the queue is the *only* writer —
  member HTTP actions would also have to go through it, which complicates the
  synchronous request/response path.

Right answer at larger scale; over-engineered for now.

---

## Comparison

| Criterion                     | A: FOR UPDATE | B: Optimistic | C: SERIALIZABLE | D: Single-writer |
|-------------------------------|:-------------:|:-------------:|:---------------:|:----------------:|
| Blocks writers                | yes           | no            | no              | n/a (serialized) |
| Needs retry loop              | no            | caller-side   | yes (40001)     | no               |
| Schema change                 | no            | version col*  | no              | no               |
| Multi-row correctness         | manual        | manual        | automatic       | automatic        |
| App reasoning required        | medium        | medium        | low             | high (infra)     |
| Cross-instance safe           | yes           | yes           | yes             | only if sole writer |
| Fit for this MVP              | ok            | ok            | **best**        | overkill         |

\* unless you use the status-conditional form, which needs no new column.

---

## What we implemented

We combined **C (SERIALIZABLE + retry)** with the **conditional-UPDATE flavor of
B** as a precondition, in one repository method:

- `ports.DocumentRepo.UpdateStatusSerializable(ctx, id, status, expectFrom...)`.
- `pgcore` implementation (`document_repo.go`):
  - `BeginTx(IsoLevel: pgx.Serializable)`, read current status, check it against
    `expectFrom`, update, commit.
  - On `40001`/`40P01` it retries up to `serializableMaxRetries` with a short
    linear backoff; `isSerializationFailure` inspects the `pgconn.PgError` code.
  - A failed precondition returns `ports.ErrConflict` (no retry — the state
    genuinely disallows the transition), surfaced to HTTP as **409**.

Where it is applied (`usecase/document_service.go`):

- **Worker claim** — `ProcessRecognition` flips `… → processing` conditionally on
  the doc *not* already being terminal; `ErrConflict` ⇒ another worker/member got
  there first, treated as a no-op success.
- **Worker finalize** — `applyRecognition` writes `… → pending_review` only from
  `{processing, uploaded, failed}`. If a member confirmed/rejected meanwhile, the
  finalize is skipped and the member's decision is preserved (regression-tested
  by `TestSerializableFinalizeDoesNotClobberConfirm`).
- **Member confirm** — `Confirm` is an atomic conditional transition from
  `pending_review` only (no more read-then-check-then-write race).
- **Member reject** — `Reject` transitions from any non-`confirmed` state
  atomically.

### Why this combination

The conditional `expectFrom` (option B) makes the business rule explicit and
gives a clean 409 when a precondition fails — that's the common, expected
outcome. SERIALIZABLE (option C) is the safety net for the genuinely concurrent
case the condition alone can't see (two transactions reading the same snapshot),
and answers the "serializable read if async" requirement directly. Row locks (A)
were rejected because the worker can hold a transaction open across slow work;
locking the row that long would stall every member action on the file. The
single-writer actor (D) is the future direction once recognition moves onto a
broker, but it's too much infrastructure for the current single-process MVP.

### Tested

- `internal/usecase/recognition_async_test.go`:
  `TestSerializableFinalizeDoesNotClobberConfirm`, `TestConfirmIsConditional`,
  plus the existing async-worker suite.
- `internal/adapters/httpapi/groups_test.go`: corporate-account sharing,
  including a co-member confirming a shared document.
- All pass under `go test -race`.

### Follow-ups (not done yet)

- Replace the worker's poll loop with `LISTEN/NOTIFY` to cut latency (independent
  of this concurrency work).
- If recognition moves to NATS/JetStream, revisit option D (per-document-key
  consumer) so the broker provides ordering and the SERIALIZABLE guard can relax
  to the conditional-UPDATE alone.
- Extend the AI analytics agent's per-tenant CTE scoping
  (`pgcore/analytics_query_repo.go`) from single-owner to corporate-account
  co-members, so shared data is visible to the agent too.
