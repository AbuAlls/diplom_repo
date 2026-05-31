# Specification

## Project overview

This project implements an MVP (beta) version of a document storage and analysis system.

The system is designed to:
- allow users to upload and manage documents
- organize documents in folders
- store structured metadata
- perform asynchronous document analysis (OCR / LLM-based)
- expose API for mobile applications

At the current stage, the system prioritizes:
- simplicity
- fast iteration
- clear architecture
- testability

---

## MVP scope

### Included
- authentication (JWT-based)
- document upload and storage
- folder hierarchy
- document metadata storage
- basic document status lifecycle
- simple async processing (without message broker)
- API for mobile clients
- local development via ngrok

### Not included (for MVP)
- no NATS / message broker
- no event streaming infrastructure
- no web frontend
- no complex RBAC system
- no distributed microservices
- no WebSocket layer (SSE optional later)

---

## System architecture

The system follows a simplified modular architecture:

- HTTP API server (Go)
- PostgreSQL database
- Object storage (MinIO / S3-compatible)
- Background processing (in-process or simple worker)
- Mobile clients (external)

No message broker is used at this stage.

---

## Development environment

### ngrok usage

For MVP, ngrok is used to expose local backend services to mobile devices.

Flow:

mobile app → HTTPS (ngrok) → local backend (HTTP)

This allows:
- testing on real mobile devices
- avoiding TLS setup
- rapid iteration

Important constraints:
- backend runs locally
- ngrok provides temporary public HTTPS endpoint
- API must be compatible with HTTPS clients

---

## High-level flow

### Document upload

1. mobile client sends upload request
2. backend receives file
3. file is stored in object storage
4. metadata is stored in PostgreSQL
5. document status is set to `uploaded`
6. backend triggers async processing (internal)

---

### Document processing (simplified async)

For MVP, async processing is implemented without NATS.

Possible implementation:
- goroutine-based worker
- simple job queue in database
- background worker loop

Flow:

1. document marked as `processing`
2. worker picks document
3. OCR / LLM service is called
4. extracted data is stored
5. document status updated (`processed` or `failed`)

---

## API design

The backend exposes a REST API used by mobile clients.

### General rules
- JSON-based
- stateless
- JWT authentication
- clear request/response contracts
- explicit error responses

### Core domains

- auth
- users
- folders
- documents
- plans (optional MVP subset)
- analysis results

---

## Authentication

Authentication is based on JWT tokens.

Flow:
1. user signs in
2. receives access token
3. token is sent in `Authorization: Bearer` header

No complex refresh rotation is required for MVP.

---

## Data model

### Core data
- users
- groups (basic, optional usage)
- folders
- documents
- plans
- plan goals
- plan items

### Analysis data
- extracted document data
- audit checks
- analytics reports

For MVP, both may live in the same database.

---

## Folder system

Folders:
- hierarchical (parent_id)
- owned by user
- may be linked to:
  - plan
  - goal
  - item

Used for:
- organization
- navigation
- grouping documents

---

## Document model

Documents contain:

- metadata (title, category, dates, organization info)
- file reference (object storage path)
- status
- extracted arrays (optional)
- timestamps

### Status lifecycle

uploaded → processing → processed
                     ↘ failed

---

## Async processing (MVP)

No message broker is used.

Instead:
- simple worker loop or goroutines
- optional DB-based queue

Requirements:
- idempotent processing
- safe retries
- clear status updates

---

## Storage

### PostgreSQL
Used for:
- all relational data
- document metadata
- status tracking
- analysis results

### Object storage
Used for:
- raw uploaded files
- generated artifacts (optional)

---

## Error handling

Errors must:
- be explicit
- use consistent structure
- not leak internal details

Example:

{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request"
  }
}

---

## Logging

Minimal logging required:

- request logs
- error logs
- processing logs

Future improvements:
- structured logging
- correlation IDs

---

## Testing

At minimum:

- unit tests for use cases
- repository tests (optional)
- handler tests for key endpoints

Run after changes:

go test ./...

---

## Code structure

Expected structure:

cmd/
internal/
  domain/
  ports/
  usecase/
  adapters/
    http/
    pgcore/
    storage/
    analysis/
migrations/
docs/

---

## Design principles

- business logic in `usecase`
- adapters only perform IO
- handlers only map transport
- avoid premature complexity
- prefer explicit code over abstraction
- keep MVP simple

---

## Future evolution (not part of MVP)

- introduce NATS for async processing
- separate analysis service
- add web frontend
- introduce WebSocket/SSE
- improve RBAC
- split core and analytics databases
- add observability (metrics, tracing)

---

## Summary

This MVP is intentionally simple:

- HTTP-only backend
- no message broker
- mobile-first API
- ngrok for external access
- minimal async processing

The goal is to validate core functionality quickly before introducing architectural complexity.
