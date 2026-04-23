# Architecture

## Overview
This project implements an MVP/beta backend architecture for a document storage and analysis platform.

The system supports:
- user authentication and authorization
- document upload and storage
- folder-based organization
- metadata persistence
- asynchronous document processing
- OCR / LLM-based extraction
- analytical projections and reports
- support for web and mobile clients

The primary goal of the current stage is to keep the architecture simple, testable, and implementation-friendly while preserving clean separation between business logic and infrastructure.

## Architectural style
The project follows a modular clean architecture approach.

Main principles:
- business logic lives in use cases
- infrastructure details are isolated in adapters
- domain entities are not coupled to transport or storage
- external integrations are accessed through ports/interfaces
- transport layer maps requests/responses and delegates to use cases

This is not a strict academic clean architecture implementation.
It is a pragmatic version optimized for MVP speed and maintainability.

## Main layers

### cmd
Application entrypoints.

Responsibilities:
- bootstrap configuration
- create dependencies
- assemble modules
- start HTTP server / workers

Should not contain:
- business logic
- SQL logic
- transport-specific validation logic

### domain
Core business entities and value objects.

Examples:
- user
- document
- folder
- plan
- plan goal
- plan item
- notification
- analysis result

Responsibilities:
- describe core concepts
- hold domain invariants where appropriate

### ports
Interfaces used by use cases to access external dependencies.

Examples:
- user repository
- document repository
- storage client
- event publisher
- OCR/analysis gateway

Responsibilities:
- define what use cases need
- hide adapter-specific details

### usecase
The main business logic layer.

Responsibilities:
- orchestrate flows
- enforce business rules
- coordinate repositories, storage, queues, and analysis services
- define application-level behavior

Examples:
- upload document
- move document to folder
- create analysis job
- fetch document status
- build report
- manage plan progress

### adapters
Implementations of ports.

Examples:
- PostgreSQL repositories
- MinIO / S3 storage adapters
- NATS publisher/subscriber
- HTTP clients for OCR/AI services
- SSE transport support

Responsibilities:
- contain IO code
- translate between external systems and internal models

### transport
HTTP handlers and DTO mapping.

Responsibilities:
- parse requests
- validate input
- call use cases
- map domain/application results to HTTP responses

Should not contain:
- core business rules
- direct persistence logic
- orchestration logic

## Core subsystems

## 1. Auth and access control
The system uses authenticated users and group-based access relationships.

Core concepts:
- users
- groups
- group properties / permissions
- user-group relations
- group access to folders and plans

This allows access control to be managed independently from document content.

## 2. Document storage
Documents are uploaded by users and stored as files in object storage.

The backend stores:
- file metadata
- ownership / folder placement
- status
- extracted arrays / normalized fields
- category links
- timestamps

Object storage is used for raw file content.
PostgreSQL stores metadata and relational state.

## 3. Folder hierarchy
Folders organize documents and may optionally be associated with:
- plans
- plan goals
- plan items

Folders may also be nested.

This enables:
- user-oriented browsing
- system folders
- logical grouping of evidence and artifacts

## 4. Planning domain
The planning subsystem includes:
- plans
- plan goals
- plan items
- item-linked artifacts

This subsystem is intended to connect operational documents with goals, measurable items, and supporting evidence.

## 5. Analysis subsystem
Document analysis is logically separated from the core transactional model.

The analysis side stores:
- extracted document data
- OCR / LLM structured output
- audit checks
- analytics reports
- analytical projections for plans and goals

This separation is useful because:
- core data is transactional and user-facing
- analysis data may be heavier, more asynchronous, and more derived
- the system can evolve toward separate storage or services later

## Data ownership
The architecture distinguishes between two logical data zones:

### Core data
Transactional, user-facing, operational data:
- users
- groups
- folders
- documents
- plans
- goals
- plan items
- access relations

### Analysis data
Derived, asynchronous, machine-produced, or reporting-oriented data:
- extracted text
- structured extraction payloads
- audit checks
- analytics reports
- analytical projections

These zones may live in:
- separate schemas
- separate databases
- or separate services later

For MVP, they may still be hosted together if operationally simpler.

## Request flow

### Synchronous flow
Typical synchronous request path:

1. client sends HTTP request
2. handler validates and maps input
3. handler calls use case
4. use case loads/saves state through ports
5. adapters perform DB/storage operations
6. use case returns result
7. handler maps result to HTTP response

### Asynchronous flow
Typical asynchronous document processing path:

1. user uploads document
2. backend stores file in object storage
3. backend writes initial metadata and status
4. backend creates processing job / emits event
5. worker consumes job
6. worker calls OCR / AI analysis service
7. worker persists extraction and analysis results
8. worker updates document processing status
9. client fetches updated status or receives live update

## Realtime updates
For MVP, server-to-client updates should prefer simple mechanisms.

Recommended:
- SSE for one-way status updates and notifications

Possible use cases:
- document processing status changes
- analysis completion
- notifications
- long-running job progress

WebSocket support may be added later if bidirectional realtime communication becomes necessary.

## Storage choices

### PostgreSQL
Primary relational store.

Used for:
- transactional data
- relationships
- statuses
- access control
- normalized business entities

### Object storage
Used for:
- raw uploaded files
- derived artifacts
- previews or generated files if needed

### Messaging
NATS or another broker may be used for:
- async job dispatch
- background processing
- notifications
- future fan-out scenarios

For MVP, messaging should remain pragmatic and not overcomplicate the request path.

## API design principles
The backend serves both web and mobile clients.

Guidelines:
- keep contracts explicit
- use stable JSON response shapes
- keep auth consistent across clients
- avoid transport-specific business branching where possible
- support pagination for list endpoints
- model long-running operations with status fields

## Error handling
Errors should be:
- explicit
- typed where useful
- logged with enough context
- safe for client exposure

Prefer:
- validation errors for invalid input
- domain/application errors for business rule violations
- infrastructure errors wrapped with context

## Testing strategy
The architecture is designed for testability.

Preferred testing levels:
- use case unit tests with mocked ports
- repository/integration tests for adapters
- end-to-end API tests for critical flows

At minimum:
- all meaningful business logic should be testable without requiring the full application runtime

## Evolution path
The current architecture is intentionally MVP-friendly.

Likely future evolution:
- stronger RBAC model
- richer document lifecycle states
- dedicated analysis service boundaries
- more formal API contracts
- better observability
- mobile/web-specific optimization layers
- event-driven notification subsystem
- separate core and analysis databases

## Non-goals at the current stage
The architecture does not currently optimize for:
- full enterprise-scale microservice fragmentation
- complex workflow engines
- heavy CQRS/event sourcing
- premature infrastructure complexity

The priority is to create a coherent, extensible baseline that can be implemented incrementally.