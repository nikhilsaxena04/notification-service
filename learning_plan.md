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

---

## Phase 2: Ingress Layer (api-service)

> **Goal:** Build the external-facing gRPC endpoint that validates, deduplicates, and enqueues incoming notification requests into Redis.

### Concepts Introduced

#### 9. Stateless Validation as a Pure Function
- `pkg/validator/validator.go` exposes `ValidateSendRequest(req) error` with no struct receiver and no dependencies.
- Stateless validators are trivially unit-testable (no mocks, no setup) and safe to call concurrently — a critical property for high-throughput ingress.

#### 10. Idempotency via Redis `SET NX`
- `SET idempotency:<key> "" EX 86400 NX` atomically sets a key only if it does not already exist. If it does, `nil` is returned and we surface `ErrDuplicateJob`.
- This pattern gives **at-most-once** delivery from the producer's perspective: a client can safely retry the same `idempotency_key` under network partitions without causing duplicate notifications.

#### 11. UUIDv7 for Server-Assigned Job IDs
- `uuid.NewV7()` produces a time-ordered UUID where the first 48 bits encode millisecond-precision Unix time.
- This gives `job_id` natural chronological sort order in Redis lists and log streams — useful for debugging processing timelines without adding extra timestamp fields.

#### 12. Strict Layer Isolation (Hexagonal DI in Practice)
- The gRPC handler calls only `service.SendNotification(ctx, req)` — it has no Redis import.
- The service calls only `producer.Enqueue(ctx, job)` — it has no gRPC import.
- The concrete Redis client is wired exclusively in `cmd/api/main.go`.
- Result: each layer is independently testable with a mock of the layer below it.

---

## Phase 3: Worker Pool (worker-service)

> **Goal:** Build a concurrent, stateful worker that reliably dequeues jobs from Redis and dispatches them to the notification router with exponential backoff and crash-safe semantics.

### Concepts Introduced

#### 13. Blocking Pop with Periodic Unblocking
- `BLMOVE` with timeout `0` blocks indefinitely — the goroutine can never check `ctx.Done()`, preventing clean shutdown.
- Using a **2-second timeout** creates a heartbeat loop: unblock, check `ctx.Done()`, block again if empty.
- This mirrors Kafka consumer's `poll(timeout)` pattern and avoids both busy-looping and deadlocking on shutdown.

#### 14. Dual-Context Graceful Shutdown
- The pool holds two contexts with different cancellation semantics:
  - **`dequeueCtx`** — cancelled first, instantly wakes all goroutines blocked on `BLMOVE`.
  - **`runCtx`** (15s timeout) — passed to `Dispatch`, lets in-flight gRPC calls complete before the process exits.
- This models "stop accepting new work, but finish current work" — the standard contract for `preStop` hooks in Kubernetes.

#### 15. Exponential Backoff with Jitter
- Formula: `min(base × 2^attempt, max_backoff) + rand(0, 200ms)`
- The **jitter** term prevents the thundering herd: without it, 100 simultaneously-failed jobs at the same attempt count would all retry at the exact same instant, causing another wave of failures.
- Priority tiers (`Low` → `Critical`) use different `base` and `max_backoff` values, ensuring critical jobs retry faster than low-priority background work.

#### 16. Deterministic JSON for `LREM` (Exact-Match Removal)
- Redis `LREM` removes elements by **exact byte-string equality** — it has no JSON-awareness.
- The `Job` struct uses flat typed pointer fields (`*EmailPayload`, never `interface{}`) with `omitempty`, ensuring `json.Marshal` output is deterministic across invocations.
- The dispatcher clones the job (`retryJob := *job`) before mutating `Attempt` or `NextRetryAt`, so the original untouched copy is used in the subsequent `Ack` call to guarantee the `LREM` finds the correct string.

#### 17. `sync.WaitGroup` as a Goroutine Lifecycle Fence
- `wg.Add(1)` is called before each `go workerLoop()`, and `defer wg.Done()` is the first statement inside the goroutine.
- `wg.Wait()` in `Stop()` blocks until every worker has returned, giving the shutdown path a hard guarantee of zero leaked goroutines.
- Verifiable with `go test -race` — the race detector also tracks goroutine lifetimes.

---

## Phase 4: Dispatch & DLQ (notification-router)

> **Goal:** Implement the internal routing layer that accepts gRPC requests from workers, delivers to mock downstream providers, and escalates terminal failures to a Dead Letter Queue.

### Concepts Introduced

#### 18. Interface-Based Provider Abstraction
- `Provider` interface (`Send(ctx, *DispatchRequest) (string, error)`) is the **port**; mock implementations are **adapters**.
- Accepting `*pb.DispatchRequest` (not `interface{}`) preserves type safety: each mock calls `req.GetEmail()` directly without type assertions, matching how real provider SDKs would be called.
- Swapping a mock for a real Mailtrap/Twilio/FCM client requires only changing the constructor in `cmd/router/main.go` — the handler is untouched.

#### 19. DLQ Write-Before-ACK Ordering Guarantee
- Terminal failure path: `Escalate(ctx, job)` (→ `RPUSH notifications:dlq`) is always called **before** `Ack(ctx, job)` (→ `LREM notifications:processing`).
- If the sequence were reversed and the process crashed between ACK and DLQ write, the job would be permanently lost with no record.
- With write-before-ACK: a crash after the DLQ write but before ACK leaves a harmless duplicate in both `processing` and `dlq` — recoverable over-delivery, which is always preferable to silent under-delivery.

#### 20. `grpc.GracefulStop()` vs `Stop()`
- `GracefulStop()` closes the listener (no new connections) then blocks until all active RPCs complete — correct for shutdown in a stateful service handling in-flight dispatches.
- `Stop()` immediately terminates all connections regardless of in-flight state — appropriate only for test teardown or forced kills where data integrity is not a concern.

---

## Phase 5: Observability (Prometheus Metrics)

> **Goal:** Expose critical application metrics via HTTP endpoints to enable scraping by Prometheus and visualization of distributed state.

### Concepts Introduced

#### 21. RED Method (Rate, Errors, Duration)
- **Rate & Errors:** Captured via `ns_jobs_processed_total` and `ns_jobs_dlq_total` Counters. By observing the rate of `status="success"` vs `status="error"` over time (`rate(...)`), we get throughput and error percentage.
- **Duration:** Captured via `ns_job_duration_seconds` and `ns_dispatch_duration_seconds` Histograms. Histograms sort observations into buckets to calculate percentiles (e.g., p95 latency) accurately.

#### 22. Prometheus Metric Types
- **Counters (`CounterVec`)**: Monotonically increasing values (e.g., total jobs enqueued). Resets to 0 only on process restart. Useful for calculating rates.
- **Gauges (`GaugeVec`)**: Values that can go up and down (e.g., current queue depth). Represents an instantaneous snapshot of state.
- **Histograms (`HistogramVec`)**: Samples observations and counts them in configurable buckets. Essential for latency calculations where average/mean is misleading due to outliers.

#### 23. Sidecar HTTP Servers for Telemetry
- Exposing an HTTP metrics server (`promhttp.Handler()`) alongside a gRPC server within the same process.
- Runs in an isolated background goroutine on a separate port (`8081` for API, `8082` for Worker, `8083` for Router).
- Prevents external ingress load from impacting the ability of the observability system to scrape internal telemetry.

#### 24. Active Polling vs Event-Driven Metrics
- Tracking `queue_depth` is challenging because Redis lists mutate rapidly and without notifying external clients.
- We implemented an **Active Poller**: a background goroutine in `worker-service` that executes `LLEN` every 5 seconds and updates the Gauge.
- This decoupling prevents adding Redis latency directly into the hot path (e.g., we don't query `LLEN` every time we enqueue/dequeue) while still providing sufficient temporal resolution for HPA (Horizontal Pod Autoscaling) scaling decisions.

---

## Phase 6: Production (Docker & Kubernetes)

> **Goal:** Package the microservices for cloud-native deployment using Docker, define cluster orchestration using Kubernetes, and set up a CI pipeline.

### Concepts Introduced

#### 25. Multi-stage Docker Builds & Distroless
- **Stage 1 (Builder):** Uses a heavy `golang:alpine` image with SDKs/tools to compile the binary. `CGO_ENABLED=0` ensures a statically linked binary without C library dependencies.
- **Stage 2 (Runtime):** Uses `gcr.io/distroless/static:nonroot`, which contains only the bare minimum runtime dependencies (like CA certificates) and no shell or package manager.
- **Benefit:** Drastically reduces the attack surface area and the final image size (often <20MB), making deployments faster and more secure.

#### 26. Kubernetes Architectural Separation
- **StatefulSet vs Deployment:** Redis requires persistent state (AOF log), so it uses a `StatefulSet` + `PersistentVolumeClaim`. The Go services are stateless and use `Deployment`s, which can be freely killed and restarted on different nodes.
- **Headless Worker:** The `worker-service` deployment does not have an accompanying `Service`. It actively consumes from Redis and makes outbound gRPC calls. Since no external system dials into the worker, exposing a cluster IP is unnecessary (except for Prometheus scraping, which can discover pods directly or via a headless service).

#### 27. Horizontal Pod Autoscaling (HPA)
- Autoscaling is configured via the `HorizontalPodAutoscaler` resource.
- Our initial implementation triggers scaling based on CPU `Utilization` (e.g., target 70%).
- As the dispatcher loops unblock under high load (due to an influx of jobs in Redis), CPU utilization rises, prompting the HPA to dynamically inject more `worker-service` pods, proportionally increasing concurrent dequeues.

#### 28. CI Pipeline Isolation via Service Containers
- In GitHub Actions (`.github/workflows/ci.yml`), tests involving Redis are notoriously flaky if a real Redis instance isn't available.
- Using `services: redis:` spins up an isolated, ephemeral Redis container specifically for the job lifecycle, ensuring `go test -race ./...` has a real queue to test producer/consumer/DLQ integration without external infrastructure dependencies.
