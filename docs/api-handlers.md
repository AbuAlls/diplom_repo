# API handlers reference

Every HTTP endpoint the backend exposes, where it's wired, and what it does.
Source of truth: `project/internal/adapters/httpapi/`. Routes are registered in
`api.go` → `Routes()`.

- **Base URL**: same origin as the frontend (e.g. `http://api.sedaidiplomver2.ru`).
- **Auth**: `Authorization: Bearer <access_token>` header on all `/v0/*` routes
  except auth. Obtained from register/token. Validated by `authenticated()`
  middleware (`api.go`).
- **Errors**: uniform JSON `{"error":{"code":"...","message":"..."}}` via
  `writeUsecaseError` (`respond.go`). Codes → HTTP: `VALIDATION_ERROR` 400,
  `INVALID_CREDENTIALS` 401, `UNAUTHORIZED` 401, `FORBIDDEN` 403, `NOT_FOUND`
  404, `CONFLICT` 409, `INTERNAL_ERROR` 500.
- **Pagination**: list endpoints accept `?page=` (default 1) and `?size=`
  (default 20, max 100); respond with `{items:[...], meta:{page,size,total,total_pages}}`.

---

## Health

### `GET /healthz`
- **Auth**: none. **File**: `api.go` (`healthz`).
- Returns `{"status":"ok"}`. Used by the Caddy/compose health checks.

---

## Auth — `auth.go`

### `POST /v0/auth/register`
- **Auth**: none. **Body** (JSON): `{ "email", "password", "full_name" }`.
- Creates a user and returns tokens. **201** →
  `{ access_token, refresh_token, expires_in, token_type:"Bearer" }`.
- Errors: `VALIDATION_ERROR` (missing/invalid fields, duplicate email).

### `POST /v0/auth/token`
- **Auth**: none. **Body** (form-urlencoded): `username`, `password`
  (OAuth2 password-grant style — **not** JSON).
- Returns the same token shape, **200**. Errors: `INVALID_CREDENTIALS` 401.

> Frontend note: the demo-login and register buttons call these two. "Load
> failed" in the browser is a transport/CORS/HSTS problem reaching these URLs,
> not a handler bug (see docs/deployment.md / troubleshooting).

---

## Plans — `plans.go`

### `GET /v0/plans`
- Lists plans visible to the caller: their own **plus** any owned by a
  corporate-account co-member (group sharing). Paginated.

### `POST /v0/plans`
- **Body**: `{ "name", "description"?, "status"? }` (status defaults to
  `active`). **201** → plan detail. Errors: `VALIDATION_ERROR` (empty name).

---

## Goals — `goals.go`

### `GET /v0/plans/{id_plan}/goals`
- Lists goals under a plan the caller can access. Paginated.

### `POST /v0/plans/{id_plan}/goals`
- **Body**: `{ "name", "description"?, "sort_order"? }`. **201** → goal.
- Errors: `FORBIDDEN`/`NOT_FOUND` (plan not accessible), `VALIDATION_ERROR`.

---

## Plan items — `items.go`

### `GET /v0/plans/{id_plan}/goals/{id_goal}/items`
- Lists items under a goal. Each item includes a computed `progress_percent`
  (from `current_value` / `target_value`). Paginated.

### `POST /v0/plans/{id_plan}/goals/{id_goal}/items`
- **Body**: `{ name, description?, sort_order?, item_type?, status?,
  target_value?, current_value?, unit? }`. **201** → item.

### `PATCH /v0/plans/{id_plan}/goals/{id_goal}/items/{id_item}`
- Partial update; any subset of the create fields. Empty body → 400. **200** →
  updated item.

---

## Documents — `documents.go`

All routes that share the `/v0/documents/{seg1}/{seg2}` shape (upload, confirm,
reject, reanalyze) are dispatched by `postDocumentAction` because the stdlib mux
can't disambiguate them.

### `POST /v0/documents/upload/{id_plan_item}`
- **Auth**: bearer. **Body**: `multipart/form-data` with a `file` field.
- Stores the file in object storage, creates the document row, and **enqueues
  recognition** (async). Returns **202 Accepted** with the document in status
  `uploaded` (no extracted fields yet) when a queue is wired, or **200** with
  fully recognized data in the synchronous fallback.
- Client then polls `GET /v0/documents/{id}` until status is `pending_review`
  or `failed`. Max upload 32 MiB.

### `GET /v0/documents`
- Lists documents visible to the caller (own + group co-members), optional
  `?plan_item_id=`. Paginated.

### `GET /v0/documents/{id_document}`
- Returns one document with its merged extracted/analysis data. Used for polling
  recognition status.

### `GET /v0/documents/{id_document}/storage`
- Returns the DB row plus an object-store HEAD-style status (exists, size,
  content-type, etag) — for verifying the file landed in storage.

### `GET /v0/documents/{id_document}/download`
- **Streams the raw file bytes** through the API (`Content-Disposition:
  attachment`). The object-store URL is never exposed to the client.

### `PATCH /v0/documents/{id_document}`
- Edits recognized/core fields (e.g. `organization_name`, `inn`,
  `document_date`, `deadlines`, `recognized_category_id`, …). At least one field
  required. **200** → updated document.

### `POST /v0/documents/{id_document}/confirm`
- Marks a `pending_review` document `confirmed`. Conditional + SERIALIZABLE:
  only `pending_review` may be confirmed, else **409 CONFLICT**.

### `POST /v0/documents/{id_document}/reject`
- Marks any non-`confirmed` document `rejected`. **409** if already confirmed.

### `POST /v0/documents/{id_document}/reanalyze`
- Re-runs recognition on a non-`confirmed` document (re-reads the file from
  storage). **409** if confirmed.

---

## Groups (corporate accounts) — `groups.go`

A group is a shared workspace; **all members see each other's plans and
documents**. Creator is admin.

### `GET /v0/groups`
- Lists the groups the caller belongs to. Paginated.

### `POST /v0/groups`
- **Body**: `{ "name", "description"? }`. Creator becomes the first member.
  **201** → group.

### `GET /v0/groups/{id_group}`
- Group detail **with members**. **403** if the caller isn't a member.

### `POST /v0/groups/{id_group}/members`
- **Creator only.** **Body**: `{ "email" }` — adds an existing user by email.
  **201** → member. **403** non-creator, **404** unknown email.

### `DELETE /v0/groups/{id_group}/members/{id_member}`
- **Creator only.** Removes a member. **204** No Content. The creator cannot
  remove themselves (**409**).

---

## Analytics — `analytics.go`, `ai.go`

### `GET /v0/items/{id_item}/analytics`
- Returns a per-item projection: the item plus document-derived counters
  (`source_documents_count`, `latest_document_id`) and progress.

### `POST /v0/items/{id_item}/analyze`
- **Body**: `{ "message", "model"? }`. Runs the AI analytics agent for an owned
  item; returns `{ "recommendations": "..." }`. Requires the AI service to be
  configured (`RECOGNIZER`/`AI_SERVICE_URL`); the agent calls back into the
  internal endpoints below.

---

## Internal AI-agent callbacks — `ai.go`

Guarded by `internalAuth` (shared secret `X-Internal-Token`, or a per-session
nonce that scopes queries to one user). **Not for public/mobile clients** — these
are how the AI service reads the DB. Consider blocking `/api/*` at the proxy if
the AI service isn't a remote caller (see `Caddyfile`).

### `GET /api/schema`
- Returns the public DB schema (`{table: [{column}]}`) for the agent.

### `POST /api/analytics/query`
- **Body**: `{ "query": "SELECT ..." }`. Executes a **read-only** SELECT and
  returns `{ columns, rows, row_count }`. When a per-session token is used, rows
  are scoped to that user (per-tenant CTE wrapping); with the global token,
  unscoped.

---

## Middleware & cross-cutting — `api.go`, `respond.go`

- **`authenticated(next)`** — requires `Authorization: Bearer`; validates the
  JWT, injects `userID` into context. Missing/invalid → **401 UNAUTHORIZED**.
- **`internalAuth(next)`** — validates `X-Internal-Token` (per-session nonce or
  global secret) for `/api/*`.
- **`WithCORS`** — permissive `Access-Control-Allow-Origin: *` (safe because auth
  is a bearer header, not cookies); answers `OPTIONS` preflight with 204.
- **Ownership/access** — handlers delegate to use cases that enforce owner-or-
  co-member access (`usecase/access.go`); violations surface as `FORBIDDEN` or
  `NOT_FOUND`.
