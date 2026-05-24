# Distributed Notification Service

A production-grade, multi-service notification platform built in Go. Accepts notification requests over gRPC, queues them in Redis, and asynchronously dispatches them to downstream providers (Email, SMS, Push) with reliable delivery semantics, priority-based exponential backoff, and a Dead Letter Queue for terminal failures.

---

## Architecture

```
Client
  │
  │  gRPC (port 50051)
  ▼
┌─────────────────┐
│   api-service   │  Validates, deduplicates, enqueues
└────────┬────────┘
         │  RPUSH  notifications:pending
         ▼
┌─────────────────┐
│      Redis      │  notifications:pending / processing / dlq
└────────┬────────┘
         │  BLMOVE (atomic pop → processing)
         ▼
┌─────────────────┐
│ worker-service  │  Concurrent pool, retry backoff, DLQ escalation
└────────┬────────┘
         │  gRPC (port 50052)
         ▼
┌──────────────────────┐
│ notification-router  │  Routes to mock Email / SMS / Push providers
└──────────────────────┘
```

### Services

| Service | Binary | Default Port | Role |
|---|---|---|---|
| `api-service` | `cmd/api` | gRPC `:50051`, metrics `:8081` | Ingress & validation |
| `worker-service` | `cmd/worker` | metrics `:8082` | Queue consumer & dispatcher |
| `notification-router` | `cmd/router` | gRPC `:50052`, metrics `:8083` | Provider routing |

### Redis Queue Keys

| Key | Semantics |
|---|---|
| `notifications:pending` | Jobs awaiting pickup (`RPUSH` / `BLMOVE` source) |
| `notifications:processing` | Jobs currently being dispatched (crash-safe) |
| `notifications:dlq` | Terminally failed jobs exceeding `max_retries` |
| `idempotency:<key>` | 24-hour deduplication guard (`SET NX EX 86400`) |

---

## Project Structure

```
notification-service/
├── cmd/
│   ├── api/            # api-service entrypoint
│   ├── worker/         # worker-service entrypoint
│   └── router/         # notification-router entrypoint
├── internal/
│   ├── api/            # gRPC handler + business logic (service.go)
│   ├── worker/         # Pool, Dispatcher, retry/backoff logic
│   └── router/         # Provider interface + mock implementations
├── pkg/
│   ├── config/         # Shared env-based configuration (envconfig)
│   ├── queue/          # Job struct, Producer, Consumer, DLQ
│   └── validator/      # Stateless request validation
├── proto/
│   └── notificationpb/ # Generated gRPC/Protobuf Go stubs
├── deployments/
│   ├── prometheus.yml  # Prometheus scrape config
│   └── Dockerfile.*    # Multi-stage builds (Phase 6)
├── docker-compose.yml  # Redis + Prometheus for local dev
└── task.md             # Phase-by-phase execution checklist
```

---

## Prerequisites

- Go 1.22+
- Docker & Docker Compose
- `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc` (only needed to regenerate stubs)

---

## Quick Start

### 1. Start local infrastructure

```bash
docker compose up -d
```

This starts:
- **Redis** on `localhost:6379` (AOF persistence, `noeviction` policy)
- **Prometheus** on `localhost:9090`

### 2. Run the services (three terminals)

```bash
# Terminal 1 — notification-router (must start first, worker dials it)
go run ./cmd/router

# Terminal 2 — worker-service
go run ./cmd/worker

# Terminal 3 — api-service
go run ./cmd/api
```

### 3. Send a notification

Use any gRPC client (e.g. `grpcurl`, `Evans`):

```bash
grpcurl -plaintext -d '{
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",
  "channel": "CHANNEL_EMAIL",
  "priority": "PRIORITY_NORMAL",
  "email": {
    "from": "sender@example.com",
    "to": ["recipient@example.com"],
    "subject": "Hello",
    "body_text": "Test notification"
  }
}' localhost:50051 notification.NotificationAPI/Send
```

The job flows: **api → Redis pending → worker → router → mock provider log**.

---

## Configuration

All services share `pkg/config/config.go`, driven by environment variables with an `NS_` prefix:

| Variable | Default | Description |
|---|---|---|
| `NS_REDIS_ADDR` | `localhost:6379` | Redis address |
| `NS_GRPC_PORT_API` | `50051` | api-service gRPC port |
| `NS_GRPC_PORT_ROUTER` | `50052` | notification-router gRPC port |
| `NS_METRICS_PORT_API` | `8081` | api-service `/metrics` port |
| `NS_METRICS_PORT_WORKER` | `8082` | worker-service `/metrics` port |
| `NS_METRICS_PORT_ROUTER` | `8083` | notification-router `/metrics` port |
| `NS_WORKER_CONCURRENCY` | `10` | Number of concurrent worker goroutines |

---

## Running Tests

Tests require a live Redis instance on `localhost:6379`. Start via `docker compose up -d` first.

```bash
# Run all tests sequentially (required — tests share the same Redis)
go test -p 1 -v ./...

# With race detector
go test -race -p 1 -v ./...
```

---

## Key Design Decisions

### Reliable Queue with `BLMOVE`
Jobs are atomically moved from `notifications:pending` to `notifications:processing` using `BLMOVE`. If a worker crashes, the job stays in `notifications:processing` and can be recovered by a future reaper process — no silent job loss.

### Dual-Context Graceful Shutdown
The worker pool uses two separate `context.Context` values: `dequeueCtx` is cancelled first to instantly wake up goroutines blocked on `BLMOVE`; `runCtx` (with 15-second timeout) is cancelled after, giving in-flight gRPC dispatches time to complete cleanly.

### DLQ Atomicity Guarantee
On terminal failure (`attempt >= max_retries`), the dispatcher calls `Escalate` (→ `RPUSH notifications:dlq`) *before* `Ack` (→ `LREM notifications:processing`). If the DLQ push fails, the job stays in `processing` rather than being silently dropped.

Redis `LREM` performs exact byte-string matching. The `Job` struct uses flat typed pointer fields (no `interface{}`) and consistent `json.Marshal` serialization to guarantee the ACK operation always finds and removes the correct entry.

---

## Code Graph Analysis

The project integrates [Graphify](https://github.com/graphify) to generate a Model Context Protocol (MCP) compatible semantic map of the codebase. 

Current Graph Topology:
- **Scope:** 36 files mapping 502 structural nodes and 549 relationship edges.
- **Accuracy:** 95% explicitly extracted by AST, 5% inferred cross-boundary semantics.
- **Core Hubs (God Nodes):** The highest betweenness-centrality nodes accurately reflect the strict gRPC contracts:
  1. `SendNotificationRequest` (API Ingress Payload)
  2. `DispatchRequest` (Internal Worker → Router Payload)
  3. `EmailPayload` / `PushPayload` (Downstream execution types)

You can regenerate the code graph locally at any time to feed AI assistants topological context:
```bash
uv run --with graphifyy python -m graphify update .
```

---

## Phase Status

| Phase | Description | Status |
|---|---|---|
| 1 | Core Contracts (proto, Redis, Job schema) | ✅ Complete |
| 2 | Ingress Layer (api-service) | ✅ Complete |
| 3 | Worker Pool (consumer, dispatcher, pool) | ✅ Complete |
| 4 | Dispatch & DLQ (router, mock providers) | ✅ Complete |
| 5 | Observability (Prometheus metrics) | 🔄 In Progress |
| 6 | Production (Dockerfiles, Kubernetes, CI) | ⏳ Pending |
