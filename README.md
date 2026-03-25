# Stellabill Backend

Go (Gin) API backend for Stellabill — subscription and billing plans API. This repo is backend-only; a separate frontend consumes these APIs.

---

## Table of contents

- [Tech stack](#tech-stack)
- [What this backend provides (for the frontend)](#what-this-backend-provides-for-the-frontend)
- [Background Worker](#background-worker)
- [Local setup](#local-setup)
- [Configuration](#configuration)
- [Testing](#testing)
- [API reference](#api-reference)
- [Contributing (open source)](#contributing-open-source)
- [Project layout](#project-layout)
- [License](#license)

---

## Tech stack

- **Language:** Go 1.22+
- **Framework:** [Gin](https://github.com/gin-gonic/gin)
- **Config:** Environment variables (no config files required for default dev)

---

## What this backend provides (for the frontend)

This service is the **backend only**. A separate frontend (or any client) can:

- **Health check** — `GET /api/health` to verify the API is up.
- **Plans** — `GET /api/plans` to list billing plans (id, name, amount, currency, interval, description). Currently returns an empty list; DB integration is planned.
- **Subscriptions** — `GET /api/subscriptions` to list subscriptions and `GET /api/subscriptions/:id` to fetch one. Responses include plan_id, customer, status, amount, interval, next_billing. Currently placeholder/mock data; DB integration is planned.

CORS is enabled for all origins in development so a frontend on another port or domain can call these endpoints.

---

## Background Worker

The backend includes a production-ready background worker system for automated billing job scheduling and execution.

### Key Features

- **Job Scheduling**: Schedule billing operations (charges, invoices, reminders) with configurable execution times
- **Distributed Locking**: Prevents duplicate processing when running multiple worker instances
- **Retry Policy**: Automatic retry with exponential backoff (1s, 4s, 9s) for failed jobs
- **Dead-Letter Queue**: Failed jobs after max attempts are moved for manual review
- **Graceful Shutdown**: Workers complete in-flight jobs before shutting down
- **Metrics Tracking**: Monitor job processing statistics (processed, succeeded, failed, dead-lettered)
- **Concurrent Workers**: Multiple workers can run safely without duplicate processing

### Documentation

- `internal/worker/README.md` - Complete worker documentation
- `internal/worker/INTEGRATION.md` - Integration guide with examples
- `internal/worker/SECURITY.md` - Security analysis and threat model
- `WORKER_IMPLEMENTATION.md` - Implementation summary

### Quick Example

```go
store := worker.NewMemoryStore()
executor := worker.NewBillingExecutor()
config := worker.DefaultConfig()

w := worker.NewWorker(store, executor, config)
w.Start()
defer w.Stop()

scheduler := worker.NewScheduler(store)
job, _ := scheduler.ScheduleCharge("sub-123", time.Now(), 3)
```

---

## Local setup

### Prerequisites

- **Go 1.22 or later**  
  - Check: `go version`  
  - Install: [https://go.dev/doc/install](https://go.dev/doc/install)

- **Git** (for cloning and contributing)

- **PostgreSQL** (optional for now; app runs without it using default config; DB will be used when persistence is added)

### 1. Clone the repository

```bash
git clone https://github.com/YOUR_ORG/stellabill-backend.git
cd stellabill-backend
```

### 2. Install dependencies

```bash
go mod download
```

### 3. (Optional) Environment variables

Create a `.env` file in the project root (do not commit it; it’s in `.gitignore`):

```bash
# Optional — defaults shown
ENV=development
PORT=8080
DATABASE_URL=postgres://localhost/stellarbill?sslmode=disable
JWT_SECRET=change-me-in-production
```

Or export them in your shell. The app will run with the defaults if you don’t set anything.

### 4. Run the server

```bash
go run ./cmd/server
```

Server listens on `http://localhost:8080` (or the port you set via `PORT`).

### 5. Verify

```bash
curl http://localhost:8080/api/health
# Expected: {"service":"stellarbill-backend","status":"ok"}
```

---

## Configuration

| Variable        | Default                                      | Description                    |
|----------------|----------------------------------------------|--------------------------------|
| `ENV`          | `development`                                | Environment (e.g. production)  |
| `PORT`         | `8080`                                       | HTTP server port               |
| `DATABASE_URL` | `postgres://localhost/stellarbill?sslmode=disable` | PostgreSQL connection string   |
| `JWT_SECRET`   | `change-me-in-production`                     | Secret for JWT (change in prod)|

In production, set these via your host’s environment or secrets manager; do not commit secrets.

---

## Testing

### Unit tests

Unit tests cover config validation, service logic, HTTP handler behaviour,
circuit breaker, and the background worker. They use in-memory mocks and
require **no external services**.

```bash
go test ./...
```

### Integration tests

Integration tests spin up a real ephemeral Postgres container via Docker and
validate the full request path — from route handler through service and
repository to the database — then tear the container down automatically.

**Prerequisites:** Docker must be running locally (or in CI with Docker socket
access). No manual database setup is required.

```bash
# Run integration tests
go test -tags integration -v -race -count=1 ./integration/...
```

The test suite in `integration/` covers:

| Scenario | Expected |
|---|---|
| Owner fetches own active subscription | 200 with full plan + billing envelope |
| Unknown subscription ID | 404 |
| Soft-deleted subscription | 410 |
| Caller does not own the subscription | 403 |
| Missing `Authorization` header | 401 |
| Malformed JWT | 401 |
| Subscription exists but referenced plan is missing | 200 with `"plan not found"` warning |
| Subscription has non-numeric amount | 500 |
| 10 concurrent reads of the same subscription | all 200, no data race |
| `GET /api/health` | 200 |
| `GET /api/plans` | 200 |
| `GET /api/subscriptions` | 200 |

**Migration timing and startup race handling:** `TestMain` applies all SQL
migrations before any test runs. The Postgres container wait strategy requires
the ready-to-accept-connections log line to appear **twice** (once during
recovery init, once when actually ready), preventing false-positive startup
races.

**CI example:**

```yaml
- name: Integration tests
  run: go test -tags integration -race -count=1 -timeout 120s ./integration/...
```

---

## API reference

Base URL (local): `http://localhost:8080`

| Method | Path                     | Description              |
|--------|--------------------------|--------------------------|
| GET    | `/api/health`            | Health check             |
| GET    | `/api/plans`             | List billing plans       |
| GET    | `/api/subscriptions`     | List subscriptions       |
| GET    | `/api/subscriptions/:id` | Get one subscription     |

All JSON responses. CORS allowed for `*` origin with common methods and headers.

---

## Contributing (open source)

We welcome contributions from the community. Below is a short guide to get you from “first look” to “merged change”.

### Code of conduct

- Be respectful and inclusive.
- Focus on constructive feedback and clear, factual communication.

### How to contribute

1. **Open an issue**  
   - Bug: describe what you did, what you expected, and what happened.  
   - Feature: describe the goal and why it helps.

2. **Fork and clone**  
   - Fork the repo on GitHub, then clone your fork locally.

3. **Create a branch**  
   ```bash
   git checkout -b fix/your-fix   # or feature/your-feature
   ```

4. **Make changes**  
   - Follow existing style (format with `go fmt`).  
   - Keep commits logical and messages clear (e.g. “Add validation for plan ID”).

5. **Run checks**  
   ```bash
   go build ./...
   go vet ./...
   go fmt ./...
   ```  
   Add or run tests if the project has them.

6. **Commit**  
   - Prefer small, atomic commits (one logical change per commit).

7. **Push and open a PR**  
   ```bash
   git push origin fix/your-fix
   ```  
   - Open a Pull Request against the main branch.  
   - Fill in the PR template (if any).  
   - Link related issues.  
   - Describe what you changed and why.

8. **Review**  
   - Address review comments. Maintainers will merge when everything looks good.

### Development workflow

- Use the [Local setup](#local-setup) steps to run the server.
- Change code, restart the server (or use a tool like `air` for live reload if the project adds it).
- Test with `curl` or the frontend that consumes this API.

### Project standards

- **Go:** `go fmt`, `go vet`, no unnecessary dependencies.  
- **APIs:** Keep JSON shape stable; document breaking changes in PRs.  
- **Secrets:** Never commit `.env`, keys, or passwords.

---

## Project layout

```
stellabill-backend/
├── cmd/
│   └── server/
│       └── main.go                  # Entry point, Gin router, server start
├── integration/                     # Integration tests (requires Docker)
│   ├── main_test.go                 # TestMain — starts container, runs migrations
│   ├── helpers_test.go              # JWT helpers, router builder, seed utilities
│   ├── health_test.go               # GET /api/health
│   ├── list_plans_test.go           # GET /api/plans
│   ├── list_subscriptions_test.go   # GET /api/subscriptions
│   └── subscriptions_test.go        # GET /api/subscriptions/:id (9 test cases)
├── internal/
│   ├── config/
│   │   └── config.go                # Loads ENV, PORT, DATABASE_URL, JWT_SECRET
│   ├── handlers/
│   │   ├── health.go                # GET /api/health
│   │   ├── plans.go                 # GET /api/plans
│   │   └── subscriptions.go        # GET /api/subscriptions, /api/subscriptions/:id
│   ├── middleware/
│   │   └── auth.go                  # JWT bearer-token middleware
│   ├── repository/
│   │   ├── interfaces.go            # SubscriptionRepository, PlanRepository interfaces
│   │   ├── models.go                # SubscriptionRow, PlanRow structs
│   │   ├── mock.go                  # In-memory implementations for unit tests
│   │   └── postgres/
│   │       ├── subscription_repo.go # Real Postgres implementation (pgx/v5)
│   │       └── plan_repo.go         # Real Postgres implementation (pgx/v5)
│   ├── routes/
│   │   └── routes.go                # Registers routes and CORS middleware
│   ├── service/
│   │   └── subscription_service.go  # Business logic — ownership, soft-delete, billing
│   ├── testutil/
│   │   └── db.go                    # Ephemeral container lifecycle helpers
│   └── worker/
│       ├── job.go                   # Job model and JobStore interface
│       ├── store_memory.go          # In-memory JobStore implementation
│       ├── worker.go                # Background worker with scheduler loop
│       ├── executor.go              # Billing job executor
│       └── scheduler.go             # Job scheduling utilities
├── migrations/
│   ├── migrations.go                # embed.FS export for the SQL files
│   ├── 001_create_plans.sql
│   └── 002_create_subscriptions.sql
├── go.mod
├── go.sum
└── README.md
```

---

## License

See the LICENSE file in the repository (if present). If none, assume proprietary until stated otherwise.
