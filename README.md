# Distributed Notification Service

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev/)
[![gRPC](https://img.shields.io/badge/gRPC-Protocol%20Buffers-244c5a?style=flat&logo=google&logoColor=white)](https://grpc.io/)
[![Redis](https://img.shields.io/badge/Redis-Queue%20%26%20DLQ-DC382D?style=flat&logo=redis&logoColor=white)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker-Compose%20%26%20K8s-2496ED?style=flat&logo=docker&logoColor=white)](https://www.docker.com/)
[![Prometheus](https://img.shields.io/badge/Prometheus-Metrics-E6522C?style=flat&logo=prometheus&logoColor=white)](https://prometheus.io/)
[![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-Tracing-7B5EA7?style=flat&logo=opentelemetry&logoColor=white)](https://opentelemetry.io/)
[![CI](https://img.shields.io/badge/CI-GitHub%20Actions-2088FF?style=flat&logo=githubactions&logoColor=white)](.github/workflows/ci.yml)

A production-grade, event-driven notification platform built entirely in **Go**. Accepts requests over **gRPC**, queues them in **Redis**, and asynchronously dispatches to downstream providers (Email, SMS, Push) through a decoupled 3-tier microservice architecture — with reliable delivery semantics, priority-based exponential backoff, and a Dead Letter Queue for terminal failures.

---

## Highlights

| Metric | Value |
|---|---|
| **Sustained Throughput** | 2,962 req/s (177,756 requests in 60s) |
| **P95 Latency** | 54 ms |
| **Success Rate** | 99.94% |
| **Delivery Guarantee** | Zero silent job loss (atomic `BLMOVE` + DLQ) |
| **Container Image Size** | < 20 MB (Distroless) |

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
│ notification-router  │  Routes to Email / SMS / Push providers
└──────────────────────┘
```

### Services

| Service | Binary | Default Port | Role |
|---|---|---|---|
| `api-service` | `cmd/api` | gRPC `:50051`, metrics `:8081` | Ingress & validation |
| `worker-service` | `cmd/worker` | metrics `:8082` | Queue consumer & dispatcher |
| `notification-router` | `cmd/router` | gRPC `:50052`, metrics `:8083` | Provider routing |

### Queue Design

| Redis Key | Semantics |
|---|---|
| `notifications:pending` | Jobs awaiting pickup (`RPUSH` / `BLMOVE` source) |
| `notifications:processing` | Jobs currently being dispatched (crash-safe) |
| `notifications:dlq` | Terminally failed jobs exceeding `max_retries` |
| `idempotency:<key>` | 24-hour deduplication guard (`SET NX EX 86400`) |

---

## Tech Stack

| Layer | Technologies |
|---|---|
| **Language & Framework** | Go 1.22+, gRPC / Protocol Buffers |
| **Message Queue** | Redis (BLMOVE, AOF persistence, `noeviction` policy) |
| **Observability** | Prometheus, Grafana, OpenTelemetry, Jaeger |
| **Containerization** | Docker (multi-stage Distroless builds), Docker Compose |
| **Orchestration** | Kubernetes (Deployments, StatefulSets, HPA) |
| **CI/CD** | GitHub Actions (`golangci-lint`, `go test -race`, Docker build) |
| **Security** | gRPC Bearer token auth interceptor, Distroless non-root runtime |

---

## Quick Start

### 1. Launch the Full Stack

The entire platform — all three Go microservices, Redis, Prometheus, Grafana, and Jaeger — is fully containerized. No local Go installation required.

```bash
docker compose up --build -d
```

This provisions:
- **API Service** — `localhost:50051` (gRPC ingress)
- **Worker & Router Services** — Background execution and dispatching
- **Redis** — `localhost:6379` (AOF persistence, `noeviction` policy)
- **Prometheus** — `localhost:9090` (metrics aggregation)
- **Grafana** — [localhost:3000](http://localhost:3000) (live dashboards · `admin/admin`)
- **Jaeger** — [localhost:16686](http://localhost:16686) (distributed tracing UI)

### 2. Run Load Test

Simulate high-concurrency traffic using the included benchmark script (runs [`ghz`](https://ghz.sh/) via Docker — no local binaries needed):

```bash
./scripts/benchmark.sh
```

### 3. Observe the System

While the benchmark is running (or immediately after):

1. **Grafana** — Open [localhost:3000](http://localhost:3000), login with `admin/admin`, and navigate to the **Notification Service Metrics** dashboard. Watch the live queue depth spike to thousands of jobs and drain rapidly, along with throughput and P95 latency charts.
2. **Jaeger** — Open [localhost:16686](http://localhost:16686), select `api-service` from the dropdown, and click **Find Traces**. Click any trace to see the exact millisecond breakdown of a request's journey across all service boundaries.

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
│   ├── metrics/        # Prometheus metric vectors & registry
│   ├── queue/          # Job struct, Producer, Consumer, DLQ
│   └── validator/      # Stateless request validation
├── proto/
│   └── notificationpb/ # Generated gRPC/Protobuf Go stubs
├── deployments/
│   ├── k8s/            # Kubernetes manifests (Deployments, HPA, StatefulSet)
│   ├── grafana/        # Dashboard provisioning & JSON models
│   ├── prometheus.yml  # Prometheus scrape config
│   └── Dockerfile.*    # Multi-stage Distroless builds
├── scripts/
│   └── benchmark.sh    # ghz load-testing harness
├── docker-compose.yml  # Full local stack (Redis, Prometheus, Grafana, Jaeger)
└── .github/
    └── workflows/ci.yml # Lint, test, Docker build pipeline
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

### Trace Propagation Across Async Boundaries
OpenTelemetry trace contexts are serialized into a `TraceCarrier` map embedded in the Redis job payload. This enables end-to-end distributed tracing across the asynchronous queue boundary — a non-trivial problem since standard gRPC interceptors only cover synchronous call chains.

---

## Performance & Benchmarks

The system was load-tested using [`ghz`](https://ghz.sh/) against the fully containerized architecture running locally.

| Metric | Result |
|---|---|
| **Total Requests** | 177,756 |
| **Duration** | 60 seconds |
| **Throughput** | 2,962 req/s |
| **Average Latency** | 25.11 ms |
| **P95 Latency** | 54.11 ms |
| **P99 Latency** | 81.87 ms |
| **Success Rate** | 99.94% (97 failures from OS TCP socket exhaustion, not application errors) |

<details>
<summary><strong>Raw ghz Output</strong></summary>

```text
Summary:
  Count:        177756
  Total:        60.00 s
  Slowest:      198.26 ms
  Fastest:      1.62 ms
  Average:      25.11 ms
  Requests/sec: 2962.41

Response time histogram:
  1.621   [1]     |
  21.285  [88350] |∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
  40.948  [70430] |∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
  60.612  [12771] |∎∎∎∎∎∎
  80.275  [4148]  |∎∎
  99.939  [1401]  |∎
  119.602 [339]   |
  139.266 [143]   |
  158.929 [53]    |
  178.593 [18]    |
  198.256 [5]     |

Latency distribution:
  10 % in 12.35 ms
  25 % in 16.07 ms
  50 % in 21.35 ms
  75 % in 29.39 ms
  90 % in 41.84 ms
  95 % in 54.11 ms
  99 % in 81.87 ms

Status code distribution:
  [OK]            177659 responses
  [Unavailable]   97 responses
```

</details>

### Customizing Load Tests

Edit `scripts/benchmark.sh` to tweak parameters (runs via Docker, no local install needed):

| Parameter | Default | Description |
|---|---|---|
| `-c` | `100` | Concurrent clients |
| `-z` | `60s` | Test duration |
| `-n` | — | Total request count (alternative to duration) |

---

## Configuration

All services share a unified config (`pkg/config/config.go`), driven by environment variables with the `NS_` prefix:

| Variable | Default | Description |
|---|---|---|
| `NS_REDIS_ADDR` | `localhost:6379` | Redis address |
| `NS_GRPC_PORT_API` | `50051` | api-service gRPC port |
| `NS_GRPC_PORT_ROUTER` | `50052` | notification-router gRPC port |
| `NS_METRICS_PORT_API` | `8081` | api-service `/metrics` port |
| `NS_METRICS_PORT_WORKER` | `8082` | worker-service `/metrics` port |
| `NS_METRICS_PORT_ROUTER` | `8083` | notification-router `/metrics` port |
| `NS_WORKER_CONCURRENCY` | `10` | Concurrent worker goroutines |
| `NS_API_KEY` | — | Bearer token for gRPC auth |

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

## Prerequisites

- **Docker & Docker Compose** — Required for all local development (builds Go inside containers)
- **Go 1.22+** — Only if running/testing outside Docker
- **`protoc`** with `protoc-gen-go` / `protoc-gen-go-grpc` — Only if regenerating gRPC stubs
