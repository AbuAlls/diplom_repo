-- Durable queue backing asynchronous document recognition (Option B in
-- docs/async-recognition-design.md). One row per recognition attempt-set for a
-- document; the worker claims rows with FOR UPDATE SKIP LOCKED.
CREATE TABLE recognition_jobs (
    id           BIGSERIAL PRIMARY KEY,
    document_id  BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    status       VARCHAR(50) NOT NULL DEFAULT 'queued', -- queued | running | done | failed
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    last_error   TEXT NULL,
    run_after    TIMESTAMP NOT NULL DEFAULT NOW(),       -- earliest time the job may be claimed (backoff)
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP NOT NULL DEFAULT NOW()
);

-- The worker's claim query filters on (status, run_after); this index keeps that
-- lookup cheap as the table grows.
CREATE INDEX idx_recognition_jobs_claim ON recognition_jobs(status, run_after);
CREATE INDEX idx_recognition_jobs_document_id ON recognition_jobs(document_id);
