# Architecture

## Overview

This project is an MVP backend for a document storage and analysis system.

Current MVP priorities:
- mobile-first API
- simple backend architecture
- fast local iteration
- testability
- minimal infrastructure complexity

At the current stage:
- there is no web frontend
- mobile applications are the only client applications
- NATS is not used in MVP
- ngrok is used in development to expose the local backend to mobile devices over HTTPS

The goal is to validate the core product flow before introducing more complex infrastructure.

## Current MVP scope

The backend currently supports or is expected to support:

- user authentication
- folder-based document organization
- document upload
- document metadata storage
- object storage integration
- asynchronous document processing
- OCR / AI-based analysis integration
- processing status tracking
- API access for mobile applications

Not part of the current MVP:
- web frontend
- message broker
- event-driven architecture with NATS
- complex RBAC
- separate production-grade analysis microservice
- advanced realtime communication

## Development topology

During MVP development, mobile devices connect to the local backend through ngrok.

Flow:

mobile app -> HTTPS via ngrok -> local backend -> PostgreSQL / MinIO / analysis integration

This setup is used to:
- test real mobile clients
- avoid local TLS configuration complexity
- validate API behavior on actual devices

## Architectural style

The project follows a pragmatic layered architecture with clean boundaries.

Main rule:
- business logic lives in `usecase`

Supporting rules:
- transport layer handles HTTP concerns only
- adapters perform IO and external integration work
- domain contains business entities and core concepts
- ports define interfaces required by the application layer

This is a practical MVP-oriented architecture, not a fully formalized enterprise architecture.

## Main layers

### cmd

Entrypoints for application startup.

Responsibilities:
- load config
- initialize dependencies
- wire modules
- start HTTP server
- start background worker if needed

Should not contain:
- business logic
- SQL queries
- request validation logic beyond basic bootstrapping

### domain

Core business concepts and entities.

Examples:
- user
- group
- folder
- document
- plan
- plan goal
- plan item
- analysis result

Responsibilities:
- define important concepts
- express core invariants where useful
- remain independent from transport and infrastructure

### ports

Interfaces required by use cases.

Examples:
- user repository
- document repository
- folder repository
- object storage client
- analysis client
- processing job interface

Responsibilities:
- describe what the application needs
- isolate use cases from implementation details

### usecase

Main business logic layer.

Responsibilities:
- orchestrate application flows
- validate business rules
- coordinate repositories and external integrations
- manage document lifecycle and processing transitions

Examples:
- sign in user
- upload document
- place document into folder
- mark document for processing
- read processing result
- fetch user documents

### adapters

Implementations of ports.

Examples:
- PostgreSQL repositories
- MinIO / S3 object storage adapter
- HTTP client for OCR / AI service
- background worker implementation

Responsibilities:
- database access
- storage access
- integration with external services
- infrastructure-specific logic

### transport

HTTP layer.

Responsibilities:
- parse incoming requests
- validate transport-level fields
- call use cases
- map results to JSON responses
- return consistent error responses

Should not contain:
- business orchestration
- persistence logic
- hidden cross-module rules

## Core subsystems

## 1. Authentication

Authentication is JWT-based.

Responsibilities:
- sign in user
- authenticate requests
- propagate user identity into use cases

The MVP should keep auth simple and predictable.

## 2. Folder and document management

The core user-facing subsystem is document organization.

Folders:
- may be hierarchical
- belong to users or are system-managed
- can group related documents

Documents:
- are uploaded through the API
- are stored in object storage
- have metadata in PostgreSQL
- move through processing statuses

## 3. Document processing

Document analysis is asynchronous, but simplified for MVP.

There is no message broker.

Recommended MVP approaches:
- in-process background worker
- goroutine-triggered processing
- optional DB-backed job polling loop

Typical flow:
1. document is uploaded
2. file is saved to object storage
3. metadata is persisted
4. status becomes `uploaded`
5. backend schedules internal processing
6. worker moves status to `processing`
7. OCR / AI analysis is performed
8. results are saved
9. status becomes `processed` or `failed`

## 4. Analysis integration

The backend may call an external OCR / AI analysis service.

For MVP this integration should remain simple:
- HTTP-based
- explicit request/response contracts
- clear timeout and retry behavior
- status persistence in the backend

Analysis output may include:
- recognized text
- extracted structured fields
- validation or audit signals
- report-oriented derived data

## Data model split

The project distinguishes between two logical data groups.

### Core data

Operational and user-facing entities:
- users
- groups
- folders
- documents
- plans
- plan goals
- plan items
- access relations

### Analysis data

Derived or machine-generated entities:
- extracted document data
- audit checks
- analysis results
- report data

For MVP, both groups may live in the same PostgreSQL database.
Later they may be split into separate schemas, databases, or services.

## Storage

### PostgreSQL

Used for:
- relational business data
- metadata
- folder hierarchy
- statuses
- analysis results
- plan-related structures

### Object storage

Used for:
- original uploaded files
- derived artifacts if needed later

MinIO is acceptable for local development and MVP environments.

## API design

The backend provides a REST API for mobile clients.

Principles:
- JSON request and response bodies
- JWT in `Authorization: Bearer`
- explicit resource-oriented endpoints
- clear error structure
- stable contracts where possible

The API should be designed primarily around mobile client needs.

## Error handling

Errors should:
- be explicit
- be consistent across endpoints
- avoid leaking internal infrastructure details
- include enough information for client-side handling

Recommended categories:
- validation errors
- authentication errors
- authorization errors
- not found errors
- conflict errors
- internal errors

## Logging

Minimum logging for MVP:
- incoming request logs
- processing state changes
- integration failures
- unexpected errors

Future improvements may include:
- structured logging
- correlation IDs
- metrics and tracing

## Testing strategy

The project should remain easy to test.

Preferred levels:
- use case unit tests
- repository integration tests
- handler tests for key API flows

At minimum, changed code should be followed by:
- relevant package tests
- preferably `go test ./...`

## Why no NATS in MVP

NATS is intentionally excluded from the MVP.

Reasons:
- reduce infrastructure complexity
- keep async flow easier to debug
- move faster on core product validation
- avoid premature event-driven abstractions

NATS may be introduced later when:
- processing load increases
- multiple workers are needed
- notification fan-out becomes important
- more subsystems need asynchronous decoupling

## Why no web frontend in MVP

The current MVP is mobile-first.

Reasons:
- reduce product surface area
- validate the API with the main intended client type
- speed up delivery
- keep backend contracts focused

A web frontend may be introduced later if needed.

## Future evolution

Possible next steps after MVP:
- introduce NATS
- split analysis into a dedicated service
- add web frontend
- add SSE or WebSocket support where justified
- improve RBAC and permissions
- separate core and analysis persistence
- add stronger observability

## Summary

The current architecture is intentionally simple:

- Go backend
- REST API
- PostgreSQL
- MinIO / S3-compatible storage
- internal async processing without message broker
- mobile-only clients for MVP
- ngrok for local mobile testing

The architecture is designed to support fast iteration now and cleaner expansion later.