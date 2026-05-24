# Changelog

All notable changes to the **Distributed Notification Service** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Added

#### gRPC Contracts (`proto/notification.proto`)
- Defined `NotificationAPI` service with `Send` and `BatchSend` RPCs for external ingress (up to 100 notifications per batch).
- Defined `NotificationRouter` service with `Dispatch` RPC for internal worker → router communication.
- Created channel-specific payload messages: `EmailPayload`, `SMSPayload`, `PushPayload`.
- Used `oneof payload` to enforce exactly-one-channel semantics at the protobuf level.
- Added `idempotency_key` field on `SendNotificationRequest` for client-side deduplication.
- Introduced `Channel`, `Priority`, and `Status` enums with `UNSPECIFIED` zero-values per proto3 best practice.

#### Local Development Infrastructure (`docker-compose.yml`)
- Added Redis service (`ns-redis`) on port `6379` with AOF persistence enabled and `noeviction` memory policy (256 MB cap).
- Added Prometheus service (`ns-prometheus`) on port `9090` with 7-day TSDB retention.
- Configured Redis healthcheck (`redis-cli ping`) with Prometheus dependency gating.
- Created `deployments/prometheus.yml` scrape config targeting all three microservices (`:8081`, `:8082`, `:8083`) via `host.docker.internal`.

#### Core Job Struct (`pkg/queue/job.go`)
- Defined the `Job` struct — the canonical unit of work serialized to JSON and pushed to the Redis queue.
- Used pointer-based `*EmailPayload`, `*SMSPayload`, `*PushPayload` fields (not `interface{}`) for deterministic JSON round-trips.
- Included `JobID` (UUIDv7, server-assigned, monotonic) and `IdempotencyKey` (UUIDv4, client-provided) for ordering and deduplication.
- Added retry state fields: `Attempt`, `MaxRetries`, `NextRetryAt` for exponential backoff within the worker pool.
- Defined `Channel`, `Priority`, and `Status` Go enums mirroring the protobuf definitions.

---

#### Ingress Layer — `api-service` (Phase 2)
- Created `pkg/config/config.go` — single env-based config struct shared across all services, using `envconfig` with `NS_` prefix. Includes metrics ports (`8081`–`8083`) pre-wired for Phase 5.
- Created `pkg/validator/validator.go` — stateless validation of `SendNotificationRequest`: UUIDv4 idempotency key, non-UNSPECIFIED channel, `oneof` payload presence, and per-channel required field checks (`to`, `subject`, `device_token`, etc.).
- Created `pkg/queue/producer.go` — `Enqueue` via `RPUSH notifications:pending` with `SET NX EX 86400` idempotency guard. Returns `ErrDuplicateJob` on collision.
- Created `internal/api/service.go` — business logic: validate → dedup → generate UUIDv7 `job_id` → map to `queue.Job` → enqueue.
- Created `internal/api/handler.go` — gRPC `NotificationAPIServer` delegating to the service layer.
- Created `cmd/api/main.go` — wires Redis → Producer → Service → Handler, starts gRPC server with graceful SIGINT/SIGTERM shutdown and gRPC reflection enabled.

---

#### Worker Pool — `worker-service` (Phase 3)
- Created `pkg/queue/consumer.go` — `Dequeue` via `BLMOVE notifications:pending notifications:processing LEFT RIGHT` with a 2-second timeout loop (prevents indefinite blocking during shutdown). Defines `ErrNoJob` sentinel for clean no-op continues. `Ack` uses `LREM` for exact-JSON removal; `Requeue` uses `RPUSH` after backoff state mutation.
- Created `internal/worker/dispatcher.go` — maps `queue.Job` → `pb.DispatchRequest`, calls router gRPC, handles three outcomes: success (`Ack`), transient failure (exponential backoff + `Requeue`), terminal failure (DLQ escalation). Backoff formula: `min(base × 2^attempt, max_backoff) + rand(0..200ms)` with per-priority `base` and `max_backoff` tiers.
- Created `internal/worker/pool.go` — concurrent pool with dual-context shutdown: `dequeueCtx` cancels idle `BLMOVE` pops instantly; `runCtx` (15-second dispatch timeout) lets in-flight gRPC calls finish. `sync.WaitGroup` tracks all goroutines — zero leaks verified by race detector.
- Created `cmd/worker/main.go` — wires Redis (pool-sized to `WORKER_CONCURRENCY + 5` to prevent connection exhaustion) → gRPC router client → Consumer → Dispatcher → Pool.

---

#### Dispatch & DLQ — `notification-router` (Phase 4)
- Created `internal/router/provider.go` — defines the `Provider` interface (`Send(ctx, *DispatchRequest) (providerID string, error)`). Implements `MockEmailProvider`, `MockSMSProvider`, and `MockPushProvider` simulating Mailtrap, Twilio, and FCM respectively. Each returns a UUID-prefixed `providerID` for downstream tracking.
- Created `internal/router/handler.go` — gRPC `NotificationRouterServer` switching on `req.Channel`, validating payload presence, delegating to the correct provider, and returning `DispatchResponse`.
- Created `cmd/router/main.go` — wires providers → Handler, starts gRPC server on `NS_GRPC_PORT_ROUTER` (default `:50052`) with graceful `GracefulStop()` shutdown.
- Created `pkg/queue/dlq.go` — `Escalate(ctx, *Job)` marshals the failed job to JSON and `RPUSH notifications:dlq`. Isolated from the consumer to allow a dedicated DLQ drain consumer in future phases.
- Updated `internal/worker/dispatcher.go` — wired DLQ escalation into the terminal failure path with strict ordering: `Escalate` is called **before** `Ack`, so a crash mid-escalation leaves the job in `notifications:processing` (recoverable) rather than silently dropping it.

---

#### Tooling & Analysis
- Generated and embedded project-wide Model Context Protocol (MCP) semantic code graph via `graphify`.
- Parsed 36 files and 18,253 words into a unified `graph.json` defining 502 structural nodes and 549 edges.
- Successfully verified `SendNotificationRequest` and `DispatchRequest` payloads as the highest betweenness-centrality "God Nodes" holding the architecture together, proving the effectiveness of the strict gRPC payload contracts.
