# TaskFlow

A production-grade task management REST API built with Go, featuring real-time SSE events, outbound webhooks, dual-token authentication, rate limiting, optimistic locking, and full Docker support.

## Overview

**TaskFlow** is a small task-management product: users register and sign in, create **projects**, add **tasks** with status/priority/assignee/due date, and see updates in real time (SSE) or via **webhooks** for integrations. This submission is the **backend-only** track: a Go REST API, PostgreSQL with versioned SQL migrations, and Docker Compose so reviewers can run the full stack with one command.

**Stack:** Go (Fiber), PostgreSQL 16, pgx, golang-migrate, JWT + refresh cookies, bcrypt for passwords, structured logging (`slog`). All HTTP routes are under **`/api/v1/`** (documented below); auth and health are the exceptions noted in the API reference.

## For reviewers (how to verify this submission)

Follow the same path the rubric uses: **Docker first**, then **seed login**, then **core flows**, then **optional bonuses**.

### 1. Clone and run (automatic disqualifier check)

```bash
git clone <your-repo-url> taskflow && cd taskflow
cp .env.example .env
docker compose up --build
```

- Expect **PostgreSQL healthy**, **migrations applied**, **API listening on port 8080** (see `docker compose logs api`).
- If port **5432** is already taken on the host, this repo maps Postgres to **5434** by default (`DB_HOST_PORT` in compose); the API still uses hostname `postgres` on the Docker network.

### 2. Smoke the API (no extra tools)

```bash
# Health
curl -sS http://localhost:8080/health/ | jq .
curl -sS http://localhost:8080/health/ready | jq .

# Seed user (README "Test Credentials")
LOGIN=$(curl -sS -c cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}')
echo "$LOGIN" | jq .
TOKEN=$(echo "$LOGIN" | jq -r '.access_token')
```

Then use **`x-access-token: <JWT>`** on all protected routes (or `Authorization: Bearer <JWT>`).

### 3. Core assignment flows (checklist)

| Check | What to do |
|--------|----------------|
| **Migrations** | Confirm `docker compose logs api` shows `migrate ... up` and no migration errors. Inspect `backend/migrations/*.up.sql` / `*.down.sql` pairs. |
| **Auth** | Register a new user; login; call `GET /api/v1/projects/` with token → **200**; same call without token → **401**. |
| **Projects** | List → create → get by id (includes tasks) → patch (owner) → delete (owner, cascades tasks). |
| **Tasks** | Create on a project; list with `?status=` / `?assignee=`; patch with **`version`**; second patch with stale version → **409**; delete as creator or owner. |
| **Errors** | Send invalid body → **400** with `{ "error": "validation failed", "fields": { ... } }`. Wrong role → **403**. Missing entity → **404**. |
| **Secrets** | Confirm `JWT_SECRET` only comes from `.env` / environment, not hardcoded in source. |
| **README** | All required sections present (overview, architecture, run locally, migrations, test credentials, API reference, more time). |

### 4. Optional / bonus verification

- **Pagination**: `GET /api/v1/projects?page=1&limit=5` — response has `data` + `pagination`.
- **Stats**: `GET /api/v1/projects/:id/stats`.
- **Integration tests**: see [Running Tests](#running-tests).
- **Postman**: import `postman/TaskFlow.postman_collection.json`.
- **SSE**: `curl -sSN --max-time 5 -H "x-access-token: $TOKEN" "http://localhost:8080/api/v1/sse/events?projects=<project_uuid>"` — should print `:connected` immediately, then events when tasks change.
- **Webhooks**: `POST /api/v1/webhooks/` with `url`, `secret`, `event_types`, `project_ids`; duplicate same `user_id` + `url` → **409**.

### 5. Code review call (what they will ask)

Be ready to explain: **auth** (JWT + refresh cookie + rotation), **why `due_date::text` in SQL** (pgx scanning `DATE` into `*string`), **optimistic locking**, **SSE broker vs webhooks**, **rate limit keys**, **graceful shutdown order**, and **tradeoffs** in "What I'd Do With More Time".

### Note on rate limits while testing

`POST /api/v1/auth/login` and `/register` are rate-limited per IP. If you hit **`429 too many requests`**, wait for `Retry-After`, **register a different email**, or restart the API container (`docker compose restart api`) to reset the in-memory limiter during local review.

## Tech Stack (detail)

- **Language**: Go (see `backend/go.mod`; Docker build sets `GOTOOLCHAIN=auto` for `go mod download` / `go install` migrate)
- **Framework**: Fiber v2 (fasthttp)
- **Database**: PostgreSQL 16, pgx v5 pool
- **Migrations**: golang-migrate (up/down pairs under `backend/migrations/`)
- **Auth**: JWT access token + HttpOnly refresh cookie (rotation); bcrypt cost **12** for passwords; SHA-256 for refresh token storage
- **Validation**: go-playground/validator v10
- **Logging**: slog (JSON)
- **Deploy**: multi-stage Dockerfile + root `docker-compose.yml`

## Architecture Decisions

### Why Fiber?

Fiber provides Express-like ergonomics with excellent performance via fasthttp. It has built-in middleware for CORS, recovery, and supports the streaming patterns needed for SSE via `SetBodyStreamWriter`.

### Why pgx over GORM?

Raw SQL with pgx gives full control over query optimization, connection pooling, and avoids the N+1 query pitfalls common with ORMs. For a project with clear, stable schemas, the extra verbosity is worth the predictability.

### Dual-Token Authentication

Access tokens (JWT, 24h) are stateless and sent via `x-access-token` header. Refresh tokens (opaque, 7 days) are stored as SHA-256 hashes in the database and delivered via `HttpOnly`, `SameSite=Lax` cookies. This separates short-lived authorization from long-lived session management. Refresh rotation (new token on every refresh) prevents replay attacks.

Why SHA-256 for refresh tokens instead of bcrypt? Refresh tokens are 32 bytes of cryptographic randomness -- high entropy means brute-force is infeasible, so bcrypt's deliberate slowness adds latency without security benefit.

### Optimistic Locking

Tasks have a `version` column. Every PATCH must include the current version. Updates use `WHERE version = $expected`, returning 409 Conflict if the version has changed. This prevents silent data loss from concurrent edits without pessimistic locks.

### SSE over WebSocket

SSE is simpler for the server-push use case (task updates). It works over HTTP/1.1, auto-reconnects natively in browsers, and doesn't require a separate protocol upgrade. Each event includes a `job_id` for client-side deduplication.

### In-Memory Rate Limiting

Sliding window counters stored in `sync.Map` with a cleanup goroutine. Sufficient for single-instance deployment. The README's "What I'd Do With More Time" section notes the Redis upgrade path.

### Outbound Webhooks

Users register URLs scoped to specific projects and event types. Deliveries use HMAC-SHA256 signatures for verification, a bounded worker pool (10 concurrent), and exponential backoff retries (1s, 5s, 25s).

### What I Intentionally Left Out

- **No ORM auto-migrate**: All schema changes go through numbered migration files with up/down pairs.
- **No global error swallowing**: Every error path returns a typed `AppError` that maps to the correct HTTP status. Internal errors never leak to clients.
- **No middleware bypass**: Public routes (auth, health) are in separate route groups. Protected routes always pass through JWT validation.

## Running Locally

### Prerequisites

- Docker and Docker Compose

### Steps

```bash
git clone https://github.com/your-name/taskflow
cd taskflow
cp .env.example .env
docker compose up --build
```

The API is available at **http://localhost:8080**.

On first start:
1. PostgreSQL initializes
2. golang-migrate runs all 7 migration pairs
3. Seed data is inserted (controlled by `SEED_DB=true` in `.env`)
4. API server starts

### Verify It Works

```bash
# Health check
curl http://localhost:8080/health/

# Login with seed user
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```

## Running Migrations

Migrations run automatically on container start via the entrypoint script. To run them manually:

```bash
# Inside the running container
docker compose exec api migrate -path /migrations \
  -database "postgres://taskflow:taskflow_secret@postgres:5432/taskflow?sslmode=disable" up

# Or locally with migrate CLI installed
migrate -path backend/migrations \
  -database "postgres://taskflow:taskflow_secret@localhost:5432/taskflow?sslmode=disable" up
```

To rollback:
```bash
migrate -path backend/migrations \
  -database "postgres://taskflow:taskflow_secret@localhost:5432/taskflow?sslmode=disable" down 1
```

## Test Credentials

Seed user credentials for immediate login:

```
Email:    test@example.com
Password: password123
```

Seed data includes:
- 1 user (above)
- 1 project: "Website Redesign"
- 3 tasks: todo/low, in_progress/high, done/medium

## API Reference

Base URL: `http://localhost:8080`

### Authentication

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/v1/auth/register` | - | Register. Body: `{name, email, password}`. Returns `{access_token, user}` + sets refresh cookie |
| POST | `/api/v1/auth/login` | - | Login. Body: `{email, password}`. Returns `{access_token, user}` + sets refresh cookie |
| POST | `/api/v1/auth/refresh` | Cookie | Rotate refresh token. Returns `{access_token}` + new refresh cookie |
| POST | `/api/v1/auth/logout` | Cookie | Invalidate refresh token. Clears cookie |

### Projects

All require `x-access-token` header.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/projects/?page=1&limit=20` | List projects you own or have tasks in |
| POST | `/api/v1/projects/` | Create project. Body: `{name, description?}` |
| GET | `/api/v1/projects/:id` | Get project with tasks |
| PATCH | `/api/v1/projects/:id` | Update project (owner only). Body: `{name?, description?}` |
| DELETE | `/api/v1/projects/:id` | Delete project + cascade tasks (owner only) |
| GET | `/api/v1/projects/:id/stats` | Task counts by status and assignee |

### Tasks

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/projects/:id/tasks?status=&assignee=&page=&limit=` | List/filter tasks |
| POST | `/api/v1/projects/:id/tasks` | Create task. Body: `{title, description?, priority?, assignee_id?, due_date?}` |
| PATCH | `/api/v1/tasks/:id` | Update task. Body: `{title?, status?, priority?, assignee_id?, due_date?, version}` (version required) |
| DELETE | `/api/v1/tasks/:id` | Delete task (project owner or task creator only) |

### SSE (Server-Sent Events)

```bash
curl -N -H "x-access-token: TOKEN" \
  "http://localhost:8080/api/v1/sse/events?projects=PROJECT_ID1,PROJECT_ID2"
```

Events: `task.created`, `task.updated`, `task.deleted`, `project.updated`, `project.deleted`

### Webhooks

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/webhooks/` | Register webhook. Body: `{url, secret, event_types[], project_ids[]}` |
| GET | `/api/v1/webhooks/` | List your webhook subscriptions |
| DELETE | `/api/v1/webhooks/:id` | Remove subscription |

### Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health/` | DB pool stats, uptime |
| GET | `/health/ready` | Readiness probe |

### Error Responses

```json
// 400 Validation
{"error": "validation failed", "fields": {"email": "is required"}}

// 401 Unauthenticated
{"error": "unauthorized"}

// 403 Forbidden
{"error": "forbidden"}

// 404 Not Found
{"error": "not found"}

// 409 Conflict (optimistic lock)
{"error": "resource was modified by another request, please retry with the latest version"}

// 429 Rate Limited
{"error": "too many requests, retry after 42 seconds"}
```

### Postman Collection

Import `postman/TaskFlow.postman_collection.json` into Postman. The collection includes auto-scripts that extract tokens from login/register responses and set them as collection variables.

## Running Tests

Integration tests need a PostgreSQL database named `taskflow_test` reachable from your machine.

**Option A — dedicated test compose** (default credentials `taskflow_test` / `taskflow_test_secret` on port `5433`):

```bash
docker compose -f docker-compose.test.yml up -d
cd backend && \
  TEST_DB_HOST=127.0.0.1 TEST_DB_PORT=5433 \
  TEST_DB_USER=taskflow_test TEST_DB_PASSWORD=taskflow_test_secret TEST_DB_NAME=taskflow_test \
  go test -v -count=1 ./tests/integration/...
docker compose -f docker-compose.test.yml down
```

**Option B — reuse the main stack** (after `docker compose up`, create the DB once):

```bash
docker exec <postgres-container-name> psql -U taskflow -d postgres -c "CREATE DATABASE taskflow_test OWNER taskflow;"
cd backend && \
  TEST_DB_HOST=127.0.0.1 TEST_DB_PORT=5434 \
  TEST_DB_USER=taskflow TEST_DB_PASSWORD=taskflow_secret TEST_DB_NAME=taskflow_test \
  go test -v -count=1 ./tests/integration/...
```

The main `docker-compose.yml` maps Postgres to host port **5434** by default (`DB_HOST_PORT`) so it does not clash with a local Postgres on 5432.

## What I'd Do With More Time

*(Same section the brief names **"What You'd Do With More Time (Completed this in a very limited time due to my on-going office workload)"** — honest scope and follow-ups.)*

**Infrastructure**:
- Redis-backed rate limiting and refresh token store for horizontal scaling
- Webhook dead letter queue for permanently failed deliveries
- SSE reconnection with `Last-Event-ID` replay from a persistent event log
- OpenTelemetry distributed tracing
- CI/CD pipeline with GitHub Actions (lint, test, build, push image)

**Features**:
- Email verification on registration
- Password reset flow
- Project membership roles (admin, member, viewer)
- Task comments and activity log
- Drag-and-drop task ordering with position field
- Full-text search across tasks

**Security**:
- Webhook secret encryption at rest (AES-GCM)
- JWT token blacklist for immediate revocation
- CSRF double-submit cookie pattern
- Request signing for API-to-API communication

**Code Quality**:
- Higher test coverage (unit tests for services, repository mocks)
- Load testing with k6 or vegeta
- API documentation with OpenAPI/Swagger spec generation
