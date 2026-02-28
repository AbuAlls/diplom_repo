DocsApp — Backend Specification

Architecture: Variant A (Modular Monolith + Async Pipeline)
Language: Go (1.22+)

1. Project Goal

Build a backend system that:

Allows users to upload documents

Stores documents in S3-compatible object storage

Runs asynchronous OCR + LLM analysis (mock for now)

Saves structured results in SQL format

Ensures strict tenant isolation (users access only their data)

Supports web and mobile clients

Is testable, scalable, and containerized

Is safe to evolve with Codex assistance

2. Architecture Overview
2.1 Pattern

Modular Monolith + Async Processing Pipeline

Two Go binaries:

cmd/api — HTTP API

cmd/worker — async job processor

External services:

Postgres (Core DB)

Postgres (Analysis DB)

NATS (JetStream enabled)

MinIO (S3-compatible storage)

2.2 System Flow

Client uploads document:

Client → API → Object Storage (S3)
↓
Core DB (documents + jobs)
↓
NATS (JetStream)
↓
Worker
OCR → LLM → Normalize
↓
Analysis DB
↓
jobs.completed
↓
SSE → Client

3. Development Environment
3.1 Docker-First

All development must run through Docker Compose.

No direct usage of local:

Postgres

NATS

MinIO

Command:

make up

3.2 Environment Configuration

.env file controls configuration.

No secrets should be hardcoded.

All configuration must be injected via environment variables.

4. Repository Structure
.
├──project/
│   ├── cmd/
│   │   ├── api/
│   │   └── worker/
│   │
│   ├── internal/
│   │   ├── domain/
│   │   ├── ports/
│   │   ├── usecase/
│   │   ├── adapters/
│   │   │   ├── pgcore/
│   │   │   ├── pganalysis/
│   │   │   ├── nats/
│   │   │   ├── s3/
│   │   │   └── httpapi/
│   │
│   ├── migrations/
│   │   ├── core/
│   │   └── analysis/
│
│
├── docker/
│   ├── docker-compose.yml
│   ├── Dockerfile
│
├── Makefile
├── .env
└── README.md

5. Data Model
5.1 Core Database
Tables

users
refresh_sessions
documents
jobs
job_steps
(optional) outbox

Enums

doc_status:

UPLOADED

PROCESSING

READY

FAILED

job_status:

CREATED

RUNNING

COMPLETED

FAILED

step_name:

OCR

LLM

NORMALIZE

step_status:

PENDING

RUNNING

COMPLETED

FAILED

5.2 Analysis Database

extractions
extracted_fields

Each extraction is tied to:

document_id

owner_id

6. Tenant Isolation (CRITICAL)

All queries MUST filter by:

owner_id = current_user_id


No shared queries.
No cross-user joins.
No admin bypass logic unless explicitly defined.

7. Authentication
7.1 Requirements

Password hashing: bcrypt or argon2id

Access token: JWT (15 minutes)

Refresh token:

Stored in HttpOnly cookie

Secure in production

SameSite=Lax

Path=/auth/refresh

Access token is returned in JSON.

Refresh token is never exposed to JavaScript.

7.2 Endpoints

POST /auth/register
POST /auth/login
POST /auth/refresh
POST /auth/logout

All protected routes require:

Authorization: Bearer <access_token>

8. Async Processing
8.1 JetStream

Stream name: DOCS
Subjects: jobs.>

Event types:

jobs.created

jobs.step.requested

jobs.step.completed

jobs.completed

jobs.failed

8.2 UI Realtime (Plain NATS)

Subject:

ui.jobs.{job_id}


Used only for SSE streaming.

8.3 Event Envelope

All events must contain:

event_id

occurred_at

type

user_id

document_id

job_id

step (optional)

attempt (optional)

data (JSON payload)

9. Pipeline Logic
9.1 Happy Path

API receives upload

File stored in S3

documents row created

jobs row created

Publish jobs.created

Worker processes:

OCR step

LLM step

Normalize

Save to Analysis DB

Mark document READY

Publish jobs.completed

9.2 Retry Logic

Each step:

Must increment attempt counter

Must be idempotent

Must not duplicate results

If attempt >= MAX_ATTEMPTS:

Mark job FAILED

Mark document FAILED

Publish jobs.failed

10. HTTP API
10.1 Documents

POST /documents
GET /documents
GET /documents/{id}
GET /documents/{id}/download
GET /documents/{id}/results

10.2 SSE

GET /jobs/{job_id}/events

Content-Type: text/event-stream

Heartbeat every 15 seconds

Subscribe to NATS subject ui.jobs.{job_id}

11. Worker Requirements

Worker must:

Ensure JetStream stream exists at startup

Subscribe to jobs.*

Use durable consumer

Be idempotent

Use context cancellation

Not store state in memory

12. Codex Workflow Rules
12.1 Branching

Branches:

main — stable

dev — human development

codex — AI editing branch

Codex must:

Never work on main directly

Only modify code in codex branch

Never delete migrations

Never modify docker-compose unless explicitly instructed

Never break port/interface contracts

Never mix domain logic into adapters

12.2 Architectural Guardrails

Codex must:

Keep business logic in usecase layer

Keep IO in adapters

Avoid global state

Use dependency injection

Use context everywhere

Keep handlers thin

13. Testing Strategy
13.1 Unit Tests

Test usecase layer

Mock ports

No external services

13.2 Integration Tests

Use testcontainers

Spin Postgres + NATS + MinIO

Test full pipeline:
upload → completed → results available

14. Frontend Assumptions

Frontend is:

Next.js (Hybrid SSR + SPA)

Access token in memory

Refresh cookie HttpOnly

React Query

SSE for status updates

No localStorage token storage

15. Production Hardening (Future)

OpenTelemetry

Prometheus metrics

Rate limiting (Redis)

Presigned uploads

DLQ stream

RLS in Postgres

16. Definition of Done

Feature is complete when:

Migration exists (if DB change)

Unit test exists

Integration test exists (if pipeline-related)

Runs via docker-compose

Passes make test

Does not violate architectural guardrails