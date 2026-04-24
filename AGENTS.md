# AGENTS.md

## Project purpose
This repository contains the MVP/beta backend architecture for a document storage and analysis system.
The codebase is expected to evolve quickly. At the current stage, changing any code files is allowed if it helps move the implementation forward in a coherent way.

## Read this first
Before making any changes, read the following files in this exact order:

1. `Specification.md`
2. `docs/docsapp_api_v0_v1.yaml`
3. `docs/domain-model.md`

Do not use `README_DEV.md` as a source of truth for implementation decisions.
`README_DEV.md` is not useful for the model and should be ignored unless explicitly requested by the user.

## Current authority rules
If documents conflict, use this priority:

1. `Specification.md`
2. `docs/docsapp_api_v0_v1.yaml`
3. `docs/domain-model.md`
4. existing code

If implementation and docs diverge, prefer the documented architecture and contracts unless the user explicitly asks to align docs to code.

## Code modification policy
At the current stage, you may change any code files when necessary.

Allowed:
- refactoring
- adding missing layers
- updating handlers, use cases, repositories, adapters
- updating DTOs and transport contracts
- improving project structure
- adding tests
- adjusting migrations if required by the current feature

Be careful with:
- public API shapes
- database migrations already committed
- environment variable names
- container startup assumptions
- cross-package dependency direction

## Architecture rules
Business logic belongs in `usecase`.

Use the following dependency direction:

- `cmd` -> wiring only
- `transport/http/handler` -> input/output mapping only
- `usecase` -> business logic and orchestration
- `ports` -> interfaces used by use cases
- `adapters` -> implementations for database, messaging, storage, external services
- `domain` -> entities, value objects, domain rules

Do not move business logic into:
- handlers
- repositories
- infrastructure adapters
- transport DTOs

## Implementation style
Prefer:
- small focused changes
- explicit interfaces
- readable code over clever code
- standard library first
- simple solutions suitable for MVP
- clear errors and predictable flows
- context propagation through all request-scoped operations

Avoid:
- unnecessary abstractions
- speculative generic frameworks
- over-engineering for future scale
- introducing new external dependencies without a strong reason

## Testing and validation
After making changes, run tests.

Minimum expectation after edits:
- run relevant tests for the changed packages
- if feasible, run the project-wide test command

Prefer these checks when available:
- `make test`
- `go test ./...`

If tests fail:
- fix the failures if they are caused by your changes
- clearly explain unresolved failures

## Migrations and database changes
When changing persistence:
- keep migrations forward-only
- do not silently rewrite old migrations that may already be applied
- name new migrations clearly
- keep schema naming consistent
- prefer explicit foreign keys and indexes

If database design is unclear, align with:
- `Specification.md`
- `docs/domain-model.md`

## API changes
When changing HTTP contracts:
- keep request/response structures explicit
- prefer consistent error shapes
- avoid undocumented breaking changes
- update API description files if the contract changes

## Output expectations
When asked to implement something:
- first understand existing structure
- preserve architectural direction
- make the smallest coherent change that solves the task
- explain important design decisions briefly
- mention which files were changed

## If context is missing
If the repository does not contain enough trustworthy information:
- infer conservatively from `Specification.md`
- keep decisions MVP-friendly
- avoid inventing large hidden subsystems
- document assumptions in the response