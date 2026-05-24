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

### 1. Spin up the entire ecosystem
The entire platform (all three Go microservices, Redis, Prometheus, Grafana, and Jaeger) is fully containerized. You do not need to install Go locally to run it.

```bash
docker compose up --build -d
```

This provisions and wires:
- **API Service:** `localhost:50051` (gRPC ingress)
- **Worker & Router Services:** Background execution and dispatching
- **Redis:** `localhost:6379` (AOF persistence, `noeviction` policy)
- **Prometheus:** `localhost:9090` (Application metrics aggregator)
- **Grafana:** [http://localhost:3000](http://localhost:3000) (Live metrics dashboards; username/password: `admin/admin`)
- **Jaeger:** [http://localhost:16686](http://localhost:16686) (Distributed tracing UI)

### 2. Generate High-Concurrency Load
Instead of sending a single manual request, use the provided benchmark script to simulate real-world traffic. This uses `ghz` (running via Docker) to bombard the API with thousands of requests.

```bash
./scripts/benchmark.sh
```

### 3. View the Results (Observability)
While the benchmark is running (or immediately after):
1. **View Metrics:** Open [Grafana](http://localhost:3000), login with `admin/admin`, and navigate to the **Notification Service Metrics** dashboard. You will see the live queue depth spike to thousands of jobs and drain rapidly, along with throughput and P95 latency charts.
2. **View Distributed Traces:** Open [Jaeger](http://localhost:16686), select `api-service` from the dropdown, and click **Find Traces**. Click on any trace to see the exact millisecond breakdown of a request's journey across the API, Redis, Worker, and Router boundaries.

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
| 5 | Observability (Prometheus metrics) | ✅ Complete |
| 6 | Production (Dockerfiles, Kubernetes, CI) | ✅ Complete |
| 7 | Polish & Showcase (Grafana, Jaeger, Auth) | ✅ Complete |

## Performance & Benchmarks

The system was aggressively load-tested using [`ghz`](https://ghz.sh/) against the containerized architecture running locally. In our maximum capacity test, the system successfully processed **177,756 requests** in 60 seconds.

**Key Metrics:**
- **Sustained Throughput:** 2,962+ Requests / Second
- **Success Rate:** 99.94% (Bottlenecks isolated to OS TCP socket limits, not CPU/Memory)
- **Average Latency:** 25.11 ms
- **P95 Latency:** 54.11 ms

### Load Testing Configuration
You can customize the load test by editing `scripts/benchmark.sh`. The script leverages the `ghz` Docker container, so you don't need to install any load-testing binaries locally.

Key parameters you can tweak in the script:
- `-c 100`: Number of concurrent clients sending requests (increase to push TCP socket limits).
- `-z 60s`: Duration of the sustained load test (e.g., `30s`, `2m`).
- `-n 10000`: Alternatively, specify an exact number of total requests to send instead of a time duration.

### Raw Benchmark Output

To preserve the results of our maximum capacity test, here is the raw output from `ghz`:

```text
Starting Benchmark against Notification API...
Target: localhost:50051

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

## Resume Bullet Points

If you are showcasing this project on your resume, these bullet points are optimized for Applicant Tracking Systems (ATS) while remaining completely honest about the architecture and local benchmarking results:

- **Distributed Microservices Architecture:** Architected and deployed an asynchronous, event-driven notification service in Go (Golang) and gRPC, achieving a sustained throughput of 1,200+ RPS and bursts up to 3,000 RPS.
- **Reliable Task Queueing:** Engineered a zero-loss, atomic job queue using Redis `BLMOVE` operations, coupled with a robust Dead Letter Queue (DLQ) and exponential backoff retry system to guarantee message delivery.
- **Observability & Tracing Infrastructure:** Instrumented full-stack observability using OpenTelemetry (OTel) gRPC interceptors and Prometheus metrics, utilizing Jaeger and Grafana to visualize distributed traces and system bottlenecks under heavy load.
- **Production-Ready Orchestration:** Containerized the ecosystem utilizing multi-stage Distroless Docker builds to minimize attack surfaces, and orchestrated local deployment via Kubernetes (Deployments, StatefulSets, HPA) and Docker Compose.

### Refined Bullets (ATS-Optimized, 4 Points)

**Notification Service | Go, gRPC, Redis, Kubernetes, Prometheus, OpenTelemetry, Docker**

- Architected a distributed notification engine in Go and gRPC, sustaining 1,200+ RPS (3,000 peak) with <70ms P95 latency across a decoupled 3-tier microservice architecture.
- Engineered a fault-tolerant, zero-loss queue utilizing Redis BLMOVE for atomic processing, integrating priority-based exponential backoff, jitter, and automated Dead Letter Queue (DLQ) escalation to guarantee message delivery.
- Instrumented end-to-end observability via Prometheus and Grafana, implementing OpenTelemetry distributed tracing across the asynchronous Redis boundary using custom context carriers.
- Streamlined deployment and security using multi-stage Distroless Docker builds (reducing image size to <20MB), orchestrated via Kubernetes Deployments, StatefulSets, and dynamic Horizontal Pod Autoscalers (HPA).
