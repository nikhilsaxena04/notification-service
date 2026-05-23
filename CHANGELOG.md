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
