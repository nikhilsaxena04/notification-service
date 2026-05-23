# Implementation Plan — Distributed Notification Service

## 1. System Overview

A high-throughput, distributed notification engine in Go, composed of three microservices communicating via gRPC and Redis. Zero external queuing dependencies — all queue semantics are built on standard Redis primitives (`RPUSH`, `BLMOVE`, `LREM`).

---

## 2. End-to-End Data Flow

```
Client (REST/gRPC)
  │
  ▼
┌─────────────────────────────────────────────────────────────┐
│ api-service (cmd/api)                                       │
│                                                             │
│  1. Receive SendNotificationRequest via gRPC                │
│  2. Validate payload (channel ↔ oneof match, required fields)│
│  3. Idempotency check: SET NX idempotency:<key> EX 86400   │
│  4. Generate UUIDv7 job_id                                  │
│  5. Marshal queue.Job → JSON                                │
│  6. RPUSH notifications:pending <job_json>                  │
│  7. Return SendNotificationResponse { job_id, ACCEPTED }    │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼  Redis List: notifications:pending
                      │
┌─────────────────────┴───────────────────────────────────────┐
│ worker-service (cmd/worker)                                 │
│                                                             │
│  1. Goroutine pool (N workers, configurable)                │
│  2. Each worker: BLMOVE pending → processing (blocking pop) │
│  3. Unmarshal JSON → queue.Job                              │
│  4. Call notification-router via gRPC Dispatch RPC           │
│  5. On success: LREM processing, mark DELIVERED              │
│  6. On failure:                                             │
│     a. attempt < max_retries → recompute backoff, RPUSH back│
│     b. attempt >= max_retries → RPUSH notifications:dlq     │
│  7. ACK: LREM from processing list                          │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼  gRPC (internal)
                      │
┌─────────────────────┴───────────────────────────────────────┐
│ notification-router (cmd/router)                            │
│                                                             │
│  1. Receive DispatchRequest                                 │
│  2. Switch on Channel → select provider adapter             │
│  3. Call mocked provider (Mailtrap/Twilio/FCM stub)         │
│  4. Return DispatchResponse { success, provider_id }        │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Redis Key Schema

| Key | Type | Purpose |
|-----|------|---------|
| `notifications:pending` | List | Main job queue. `api-service` pushes, `worker-service` pops. |
| `notifications:processing` | List | In-flight jobs. `BLMOVE` target. Crash recovery source. |
| `notifications:dlq` | List | Dead-letter queue. Jobs that exhausted all retries. |
| `idempotency:<key>` | String | Dedup guard. `SET NX EX 86400` (24h TTL). |

---

## 4. Architecture: Hexagonal (Ports & Adapters)

Per `.rules/backend.md`: **Handlers → Services → Repositories/Queues**.

```
┌─────────────────────────────────────────────────┐
│                    Handlers                      │
│  (gRPC server impls — inbound adapters)          │
├─────────────────────────────────────────────────┤
│                    Services                      │
│  (Business logic — pure, testable)               │
├─────────────────────────────────────────────────┤
│             Repositories / Queues                │
│  (Redis queue, gRPC client — outbound adapters)  │
└─────────────────────────────────────────────────┘
```

Each service binary in `cmd/` wires together the layers using constructor injection. No global state.

---

## 5. Directory Structure

```
notification-service/
├── cmd/
│   ├── api/              # api-service entrypoint
│   │   └── main.go
│   ├── worker/           # worker-service entrypoint
│   │   └── main.go
│   └── router/           # notification-router entrypoint
│       └── main.go
├── proto/
│   └── notification.proto          # ✅ Phase 1 (done)
├── pkg/
│   ├── queue/
│   │   ├── job.go                  # ✅ Phase 1 (done)
│   │   ├── producer.go             # RPUSH enqueue logic
│   │   ├── consumer.go             # BLMOVE dequeue logic
│   │   └── queue_test.go           # Table-driven tests
│   ├── metrics/
│   │   └── metrics.go              # Prometheus counters, histograms
│   ├── validator/
│   │   ├── validator.go            # Payload validation logic
│   │   └── validator_test.go       # Table-driven tests
│   └── config/
│       └── config.go               # Env-based configuration
├── internal/
│   ├── api/
│   │   ├── handler.go              # gRPC NotificationAPI impl
│   │   └── service.go              # Enqueue orchestration
│   ├── worker/
│   │   ├── pool.go                 # Goroutine worker pool manager
│   │   ├── dispatcher.go           # Per-job dispatch + retry logic
│   │   └── pool_test.go            # Table-driven tests
│   └── router/
│       ├── handler.go              # gRPC NotificationRouter impl
│       ├── provider.go             # Provider interface + mocks
│       └── provider_test.go        # Table-driven tests
├── deployments/
│   ├── prometheus.yml              # ✅ Phase 1 (done)
│   ├── Dockerfile.api
│   ├── Dockerfile.worker
│   ├── Dockerfile.router
│   └── k8s/
│       ├── api-deployment.yaml
│       ├── worker-deployment.yaml
│       ├── router-deployment.yaml
│       └── hpa.yaml
├── docker-compose.yml              # ✅ Phase 1 (done)
├── go.mod
├── go.sum
├── CLAUDE.md
├── CHANGELOG.md
├── learning_plan.md
├── implementation_plan.md
└── task.md
```

---

## 6. Go Package Dependencies

| Package | Import Path | Purpose | Phase |
|---------|-------------|---------|-------|
| **gRPC** | `google.golang.org/grpc` | gRPC server/client for all three services | 2 |
| **protobuf** | `google.golang.org/protobuf` | Protobuf runtime (generated code) | 2 |
| **protoc-gen-go** | `google.golang.org/protobuf/cmd/protoc-gen-go` | Proto code generation (tool) | 2 |
| **protoc-gen-go-grpc** | `google.golang.org/grpc/cmd/protoc-gen-go-grpc` | gRPC stub generation (tool) | 2 |
| **go-redis** | `github.com/redis/go-redis/v9` | Redis client (mandated by `.rules/backend.md`) | 2 |
| **uuid** | `github.com/google/uuid` | UUIDv4 (idempotency) + UUIDv7 (job IDs) | 2 |
| **prometheus client** | `github.com/prometheus/client_golang` | `/metrics` endpoint, counters, histograms | 5 |
| **slog** | `log/slog` (stdlib) | Structured logging | 2 |
| **envconfig** | `github.com/kelseyhightower/envconfig` | Env-based config parsing | 2 |

---

## 7. Concurrency Model (Worker Pool)

```
main goroutine
  │
  ├── Starts N worker goroutines (configurable via WORKER_CONCURRENCY)
  │     Each goroutine:
  │       └── for { BLMOVE → unmarshal → dispatch → ACK/retry }
  │
  ├── Signal handler (SIGINT/SIGTERM)
  │     └── Sets context.Done() → workers drain gracefully
  │
  └── sync.WaitGroup.Wait() → clean shutdown, zero leaked goroutines
```

**Thread safety requirements** (per `CLAUDE.md`):
- `sync.WaitGroup` for goroutine lifecycle tracking
- `context.Context` cancellation for graceful shutdown
- No shared mutable state between workers (each owns its Redis connection from the pool)

---

## 8. Retry Strategy

| Priority | Max Retries | Base Backoff | Max Backoff |
|----------|-------------|-------------|-------------|
| LOW | 2 | 5s | 30s |
| NORMAL | 3 | 2s | 60s |
| HIGH | 5 | 1s | 120s |
| CRITICAL | 8 | 500ms | 300s |

Formula: `backoff = min(base * 2^attempt, max_backoff) + jitter`

On `attempt >= max_retries` → `RPUSH notifications:dlq` + `LREM notifications:processing`.

---

## 9. Metrics (Prometheus)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ns_jobs_enqueued_total` | Counter | `channel`, `priority` | Total jobs pushed to pending queue |
| `ns_jobs_processed_total` | Counter | `channel`, `status` | Jobs completed (delivered/failed) |
| `ns_jobs_dlq_total` | Counter | `channel` | Jobs escalated to DLQ |
| `ns_queue_depth` | Gauge | `queue` | Current length of pending/processing/dlq lists |
| `ns_job_duration_seconds` | Histogram | `channel` | End-to-end processing latency (P50/P95) |
| `ns_dispatch_duration_seconds` | Histogram | `channel`, `provider` | Router dispatch latency |

---

## 10. Key Design Decisions

1. **No external message broker** — Redis lists with `BLMOVE` provide reliable queue semantics with crash recovery. This satisfies the `CLAUDE.md` constraint of zero external queuing dependencies.

2. **Flat Job struct over interface{}** — Deterministic JSON serialization on the hot path. No reflect-based unmarshaling.

3. **Separate processing list** — `BLMOVE` atomically moves from `pending` to `processing`. If a worker crashes, jobs in `processing` can be recovered by a background reaper (future enhancement).

4. **UUIDv7 for job IDs** — Timestamp-sortable, enabling chronological ordering in Redis and clean Prometheus histogram bucketing.

5. **Idempotency via Redis SET NX** — 24-hour TTL prevents duplicate job creation from client retries without requiring a database.

---

**APPROVED ARCHITECTURE**
