# Task Checklist — Distributed Notification Service

Sequential execution roadmap for the Developer agent. Complete each phase fully before advancing to the next.

---

## Phase 1: Core Contracts ✅

- [x] Define `proto/notification.proto` with `NotificationAPI` and `NotificationRouter` services
- [x] Define `EmailPayload`, `SMSPayload`, `PushPayload` messages with `oneof` semantics
- [x] Define `Channel`, `Priority`, `Status` enums with `UNSPECIFIED` zero-values
- [x] Define `BatchSendRequest` / `BatchSendResponse` for bulk ingress
- [x] Define `DispatchRequest` / `DispatchResponse` for worker → router internal RPC
- [x] Create `docker-compose.yml` with Redis (AOF, noeviction, healthcheck) and Prometheus
- [x] Create `deployments/prometheus.yml` scrape config for all three services
- [x] Define `pkg/queue/job.go` with `Job` struct and channel-specific payload types
- [x] Validate `docker compose config` passes cleanly

---

## Phase 2: Ingress Layer (api-service)

- [x] Run `protoc` to generate Go stubs into `proto/notificationpb/`
- [x] Add Go module dependencies: `go-redis/v9`, `google.golang.org/grpc`, `google.golang.org/protobuf`, `github.com/google/uuid`, `github.com/kelseyhightower/envconfig`
- [x] Create `pkg/config/config.go` — env-based configuration struct (`REDIS_ADDR`, `GRPC_PORT`, `WORKER_CONCURRENCY`, etc.)
- [x] Create `pkg/validator/validator.go` — validate `SendNotificationRequest`:
  - [x] `channel` must not be `UNSPECIFIED`
  - [x] `oneof payload` must match `channel` (e.g., `CHANNEL_EMAIL` requires `email` field)
  - [x] Required field checks per payload type (e.g., `to` non-empty for email)
  - [x] `idempotency_key` must be a valid UUIDv4
- [x] Create `pkg/validator/validator_test.go` — table-driven tests for all validation rules
- [x] Create `pkg/queue/producer.go`:
  - [x] `Enqueue(ctx, job *Job) error` — `json.Marshal` → `RPUSH notifications:pending`
  - [x] Idempotency guard: `SET NX idempotency:<key> EX 86400`
  - [x] Return `ErrDuplicateJob` if SET NX fails
- [x] Create `internal/api/service.go` — business logic: validate → dedup → generate UUIDv7 → enqueue
- [x] Create `internal/api/handler.go` — gRPC `NotificationAPIServer` implementation, calls service layer
- [x] Create `cmd/api/main.go`:
  - [x] Parse config from env
  - [x] Initialize Redis client (`go-redis/v9`)
  - [x] Wire handler → service → producer (hexagonal DI)
  - [x] Start gRPC server on configured port
  - [x] Graceful shutdown on SIGINT/SIGTERM
- [x] Integration test: start Redis (docker compose), send a `Send` RPC, verify job appears in `notifications:pending`

---

## Phase 3: The Worker Pool (worker-service) ✅

- [x] Create `pkg/queue/consumer.go`:
  - [x] `Dequeue(ctx) (*Job, error)` — `BLMOVE notifications:pending notifications:processing LEFT RIGHT 0`
  - [x] `Ack(ctx, jobID string) error` — `LREM notifications:processing 1 <job_json>`
  - [x] `Requeue(ctx, job *Job) error` — `RPUSH notifications:pending` (after backoff update)
- [x] Create `pkg/queue/queue_test.go` — table-driven tests for producer + consumer with a real Redis (testcontainers or miniredis)
- [x] Create `internal/worker/pool.go`:
  - [x] `Pool` struct with configurable concurrency (`WORKER_CONCURRENCY`)
  - [x] `Start(ctx)` — launches N goroutines, each running the dequeue → dispatch → ACK loop
  - [x] `Stop()` — cancels context, `sync.WaitGroup.Wait()` for clean drain
  - [x] Zero leaked goroutines on shutdown
- [x] Create `internal/worker/dispatcher.go`:
  - [x] Per-job dispatch logic: unmarshal → call router gRPC `Dispatch` → handle response
  - [x] Retry backoff: `min(base * 2^attempt, max_backoff) + jitter`
  - [x] On success: `consumer.Ack()`
  - [x] On transient failure (attempt < max): update `Attempt`, `NextRetryAt`, `consumer.Requeue()`
  - [x] On terminal failure (attempt >= max): escalate to DLQ (Phase 4)
- [x] Create `cmd/worker/main.go`:
  - [x] Parse config from env
  - [x] Initialize Redis client + gRPC client to notification-router
  - [x] Wire pool → dispatcher → consumer (hexagonal DI)
  - [x] Start pool, block until SIGINT/SIGTERM, drain gracefully
- [x] Unit test: `pool_test.go` — verify pool starts N goroutines, shuts down cleanly, no goroutine leaks

---

## Phase 4: Dispatch & DLQ (notification-router) ✅

- [x] Create `internal/router/provider.go`:
  - [x] `Provider` interface: `Send(ctx, channel, payload) (providerID string, err error)`
  - [x] `MockEmailProvider` — logs + returns success (simulates Mailtrap)
  - [x] `MockSMSProvider` — logs + returns success (simulates Twilio)
  - [x] `MockPushProvider` — logs + returns success (simulates FCM)
- [x] Create `internal/router/handler.go`:
  - [x] gRPC `NotificationRouterServer` implementation
  - [x] Switch on `channel` → select provider → call `provider.Send()`
  - [x] Map provider result to `DispatchResponse`
- [x] Create `cmd/router/main.go`:
  - [x] Parse config, start gRPC server, register handler
  - [x] Graceful shutdown on SIGINT/SIGTERM
- [x] Create `internal/router/provider_test.go` — table-driven tests for provider selection and error handling
- [x] Add DLQ escalation to `internal/worker/dispatcher.go`:
  - [x] `pkg/queue/dlq.go` — `Escalate(ctx, job *Job) error` → `RPUSH notifications:dlq`
  - [x] Wire into dispatcher: on `attempt >= max_retries`, call `Escalate()` + `Ack()`
- [x] End-to-end test: `api-service` → Redis → `worker-service` → `notification-router` (all three processes)

---

## Phase 5: Observability (Prometheus Metrics)

- [x] Create `pkg/metrics/metrics.go`:
  - [x] `ns_jobs_enqueued_total` — Counter, labels: `channel`, `priority`
  - [x] `ns_jobs_processed_total` — Counter, labels: `channel`, `status`
  - [x] `ns_jobs_dlq_total` — Counter, labels: `channel`
  - [x] `ns_queue_depth` — Gauge, labels: `queue` (pending/processing/dlq)
  - [x] `ns_job_duration_seconds` — Histogram, labels: `channel`
  - [x] `ns_dispatch_duration_seconds` — Histogram, labels: `channel`, `provider`
- [x] Instrument `api-service`: increment `enqueued_total` on successful `RPUSH`
- [x] Instrument `worker-service`: observe `job_duration`, increment `processed_total`
- [x] Instrument `notification-router`: observe `dispatch_duration`
- [x] Add `/metrics` HTTP endpoint to each service via `promhttp.Handler()` (side-car HTTP server alongside gRPC)
- [x] Add background goroutine to `worker-service` polling `LLEN` for `queue_depth` gauge
- [x] Verify: `docker compose up`, send requests, confirm metrics appear at `localhost:9090`

---

## Phase 6: Production (Docker + Kubernetes)

- [x] Create `deployments/Dockerfile.api` — multi-stage build (Go builder → distroless/static)
- [x] Create `deployments/Dockerfile.worker` — multi-stage build
- [x] Create `deployments/Dockerfile.router` — multi-stage build
- [x] Add all three services to `docker-compose.yml` with `build:` directives for local testing
- [x] Verify: `docker compose up --build` — all services start, send notification end-to-end
- [x] Create `deployments/k8s/api-deployment.yaml` — Deployment + Service (ClusterIP, port 50051)
- [x] Create `deployments/k8s/worker-deployment.yaml` — Deployment (no Service, internal consumer)
- [x] Create `deployments/k8s/router-deployment.yaml` — Deployment + Service (ClusterIP, port 50052)
- [x] Create `deployments/k8s/redis-deployment.yaml` — StatefulSet + PVC for Redis
- [x] Create `deployments/k8s/hpa.yaml` — HPA for worker-service based on CPU / custom queue-depth metric
- [x] Create `.github/workflows/ci.yml` — lint, test, build Docker images on push
- [x] Verify: `minikube start`, `kubectl apply -f deployments/k8s/`, end-to-end smoke test

---

## Phase 7: Polish & Showcase (Tracing, Auth, Load Testing)

- [x] Add Jaeger and Grafana to `docker-compose.yml`
- [x] Provision Prometheus datasource and basic Grafana Dashboard
- [x] Integrate OpenTelemetry (`otelgrpc`) tracing into all three microservices
- [x] Add `TraceCarrier` to `Job` struct to propagate trace IDs across Redis `RPUSH`/`BLMOVE`
- [x] Create a gRPC Auth Interceptor and require static API key (`NS_API_KEY`)
- [x] Create `scripts/benchmark.sh` using `ghz` to generate load
- [x] Verify: Run load test and view traces in Jaeger and load spikes in Grafana

---

> **Developer agent:** Execute phases sequentially. Do not skip ahead. Each phase must compile and pass tests before moving to the next. Use `ctx context.Context` as the first parameter for all I/O functions. Use `go-redis/v9` for all Redis operations. Write table-driven tests for all core logic.
