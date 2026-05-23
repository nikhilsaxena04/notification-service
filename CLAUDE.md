# Core System Context: Distributed Notification Service

## 1. System Architecture
The system is a distributed, high-throughput notification engine written in Go (1.21+), broken into three microservices interacting via gRPC and Redis.

[ Client / Producer ]
       │ (REST / gRPC)
       ▼
┌────────────────────────────────────────────────────────┐
│ 1. api-service                                         │
│    ─ Exposes HTTP/gRPC ingress endpoints               │
│    ─ Validates payload, enqueues jobs to Redis         │
└───────────────────────┬────────────────────────────────┘
                        │
                        ▼ (Redis RPOPLPUSH / BLMOVE)
┌────────────────────────────────────────────────────────┐
│ 2. worker-service                                      │
│    ─ Manages stateful concurrent Goroutine Worker Pool │
│    ─ Pulls from active queue, executes retry loops     │
│    ─ Escalates unrecoverable failures to Redis DLQ     │
└───────────────────────┬────────────────────────────────┘
                        │
                        ▼ (gRPC Internal)
┌────────────────────────────────────────────────────────┐
│ 3. notification-router                                 │
│    ─ Downstream dispatchers (Mocked Mailtrap/Twilio)   │
└────────────────────────────────────────────────────────┘

## 2. Infrastructure
* **State / Queue:** Redis v7.x.
* **Deployment Target:** Kubernetes (Minikube locally) via HPA.
* **Metrics:** Prometheus scraping `/metrics` -> Grafana (P50/P95 latency, error rates, queue depth).
* **CI/CD:** GitHub Actions (multi-stage minimal Docker builds).

## 3. Agent Rules
* **Architect (Opus):** Owns system design, `.proto` files, and K8s YAMLs.
* **Developer (Gemini Pro):** Writes raw, production-grade Go code. No stubs.
* **Reviewer (Gemini Low):** Audits for race conditions and memory leaks.
* **Intern (Gemini Flash):** Maintains `CHANGELOG.md` and `README.md`.

## 4. Constraints
* Zero external queuing dependencies outside of standard Redis primitives.
* Strict thread safety (`sync.Pool`, `sync.RWMutex`). No leaked goroutines.