# Learning Plan — Distributed Notification Service

A running log of key concepts and design patterns introduced in each phase of the project.

---

## Phase 1: Architecture & Contracts

> **Goal:** Define the gRPC API surface, stand up local infrastructure, and design the queue payload — all before writing any Go server code.

### Concepts Introduced

#### 1. Protobuf `oneof` Pattern
- `oneof payload { EmailPayload | SMSPayload | PushPayload }` enforces at the wire level that exactly one channel-specific payload is set per request.
- Prevents invalid states (e.g., a request with both `email` and `sms` populated) without runtime validation logic.
- The Go generated code exposes a type-switch interface, which maps cleanly to the `Channel` enum discriminator.

#### 2. Proto3 Enum Zero-Value Convention
- Every enum starts with an `UNSPECIFIED = 0` sentinel. This distinguishes "field was never set" from a valid value like `CHANNEL_EMAIL = 1`.
- Critical for backward compatibility — new enum values can be added without breaking existing consumers.

#### 3. UUIDv7 vs UUIDv4
- **UUIDv4** (random): Used for `idempotency_key` — provided by the client, no ordering needed, purely for deduplication via `SET NX`.
- **UUIDv7** (timestamp-sortable): Used for `job_id` — server-assigned, enables monotonic ordering in Redis and natural chronological sorting in logs/metrics.

#### 4. Redis AOF Persistence
- Append-Only File (`--appendonly yes`) logs every write operation to disk, providing durability guarantees for the notification queue.
- Combined with `--maxmemory-policy noeviction`: Redis will return errors on writes when memory is full rather than silently evicting queue entries. This is critical for a job queue where data loss is unacceptable.

#### 5. Redis Reliable Queue Pattern (`BLMOVE`)
- Per `.rules/backend.md`, queue operations must use atomic primitives like `BLMOVE` (or `RPOPLPUSH`) to atomically pop from the active queue and push to a processing list.
- If a worker crashes mid-processing, the job remains in the processing list and can be recovered — preventing silent job loss that `LPOP` would cause.

#### 6. Pointer-Based Payload Discrimination (Go)
- The `Job` struct uses `*EmailPayload`, `*SMSPayload`, `*PushPayload` (pointer fields) instead of `interface{}`.
- This avoids `reflect`-based JSON deserialization and keeps `json.Unmarshal` deterministic — important for the high-throughput `BLMOVE → unmarshal` hot path in the worker pool.
- `omitempty` on pointer fields means only the active channel's payload appears in the serialized JSON.

#### 7. Prometheus Scrape Architecture
- Prometheus operates on a **pull model**: it scrapes `/metrics` endpoints on each service at a configured interval (15s).
- Each Go service will expose a `/metrics` HTTP endpoint (typically via `promhttp.Handler()`), registering counters, histograms, and gauges for latency, error rates, and queue depth.
- `host.docker.internal` allows Prometheus (running in Docker) to reach Go services running on the host during local development.

#### 8. Hexagonal Architecture (Ports & Adapters)
- Per `.rules/backend.md`: `Handlers → Services → Repositories/Queues`.
- The proto services define the **ports** (inbound: `NotificationAPI`, internal: `NotificationRouter`).
- Redis queue operations and downstream provider calls are **adapters** that implement these ports.
- This separation means the worker pool's retry logic is testable without a live Redis connection.
