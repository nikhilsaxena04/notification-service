# Graph Report - notification-service  (2026-05-24)

## Corpus Check
- 37 files · ~20,175 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 520 nodes · 566 edges · 40 communities (20 shown, 20 thin omitted)
- Extraction: 95% EXTRACTED · 5% INFERRED · 0% AMBIGUOUS · INFERRED: 29 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `ce558e19`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 19|Community 19]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 28|Community 28]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 38|Community 38]]
- [[_COMMUNITY_Community 39|Community 39]]

## God Nodes (most connected - your core abstractions)
1. `SendNotificationRequest` - 14 edges
2. `SendNotificationRequest` - 14 edges
3. `DispatchRequest` - 13 edges
4. `file_notification_proto_rawDescGZIP()` - 13 edges
5. `DispatchRequest` - 13 edges
6. `file_notification_proto_rawDescGZIP()` - 13 edges
7. `EmailPayload` - 12 edges
8. `EmailPayload` - 12 edges
9. `Implementation Plan — Distributed Notification Service` - 11 edges
10. `PushPayload` - 10 edges

## Surprising Connections (you probably didn't know these)
- `TestPoolAndDispatcher()` --calls--> `NewProducer()`  [INFERRED]
  internal/worker/pool_test.go → pkg/queue/producer.go
- `main()` --calls--> `Load()`  [INFERRED]
  cmd/worker/main.go → pkg/config/config.go
- `main()` --calls--> `NewMockEmailProvider()`  [INFERRED]
  cmd/router/main.go → internal/router/provider.go
- `main()` --calls--> `NewMockSMSProvider()`  [INFERRED]
  cmd/router/main.go → internal/router/provider.go
- `main()` --calls--> `NewMockPushProvider()`  [INFERRED]
  cmd/router/main.go → internal/router/provider.go

## Communities (40 total, 20 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.12
Nodes (10): DispatchRequest_Email, DispatchRequest_Push, DispatchRequest_Sms, isDispatchRequest_Payload, isSendNotificationRequest_Payload, file_notification_proto_init(), init(), SendNotificationRequest_Email (+2 more)

### Community 2 - "Community 2"
Cohesion: 0.05
Nodes (41): 10. Idempotency via Redis `SET NX`, 11. UUIDv7 for Server-Assigned Job IDs, 12. Strict Layer Isolation (Hexagonal DI in Practice), 13. Blocking Pop with Periodic Unblocking, 14. Dual-Context Graceful Shutdown, 15. Exponential Backoff with Jitter, 16. Deterministic JSON for `LREM` (Exact-Match Removal), 17. `sync.WaitGroup` as a Goroutine Lifecycle Fence (+33 more)

### Community 3 - "Community 3"
Cohesion: 0.06
Nodes (6): BatchSendRequest, BatchSendResponse, Channel, file_notification_proto_rawDescGZIP(), Priority, Status

### Community 4 - "Community 4"
Cohesion: 0.10
Nodes (14): Consumer, NewConsumer(), DLQ, NewDLQ(), Dispatcher, getBackoff(), NewDispatcher(), main() (+6 more)

### Community 5 - "Community 5"
Cohesion: 0.09
Nodes (13): _NotificationAPI_BatchSend_Handler(), _NotificationAPI_Send_Handler(), _NotificationRouter_Dispatch_Handler(), RegisterNotificationAPIServer(), RegisterNotificationRouterServer(), NotificationAPIClient, NotificationAPIServer, NotificationRouterClient (+5 more)

### Community 6 - "Community 6"
Cohesion: 0.09
Nodes (13): _NotificationAPI_BatchSend_Handler(), _NotificationAPI_Send_Handler(), _NotificationRouter_Dispatch_Handler(), RegisterNotificationAPIServer(), RegisterNotificationRouterServer(), NotificationAPIClient, NotificationAPIServer, NotificationRouterClient (+5 more)

### Community 7 - "Community 7"
Cohesion: 0.07
Nodes (26): 1. Start local infrastructure, 2. Run the services (three terminals), 3. Send a notification, Architecture, Code Graph Analysis, code:block1 (Client), code:block2 (notification-service/), code:bash (docker compose up -d) (+18 more)

### Community 9 - "Community 9"
Cohesion: 0.06
Nodes (21): main(), Service, NewService(), Config, Load(), Producer, NewProducer(), setupTestRedis() (+13 more)

### Community 10 - "Community 10"
Cohesion: 0.04
Nodes (13): DispatchRequest_Email, DispatchRequest_Push, DispatchRequest_Sms, EmailPayload, isDispatchRequest_Payload, isSendNotificationRequest_Payload, file_notification_proto_init(), init() (+5 more)

### Community 11 - "Community 11"
Cohesion: 0.12
Nodes (15): 10. Key Design Decisions, 1. System Overview, 2. End-to-End Data Flow, 3. Redis Key Schema, 4. Architecture: Hexagonal (Ports & Adapters), 5. Directory Structure, 6. Go Package Dependencies, 7. Concurrency Model (Worker Pool) (+7 more)

### Community 16 - "Community 16"
Cohesion: 0.15
Nodes (12): Added, Changelog, Core Job Struct (`pkg/queue/job.go`), Dispatch & DLQ — `notification-router` (Phase 4), gRPC Contracts (`proto/notification.proto`), Ingress Layer — `api-service` (Phase 2), Local Development Infrastructure (`docker-compose.yml`), Observability — Prometheus Metrics (Phase 5) (+4 more)

### Community 21 - "Community 21"
Cohesion: 0.25
Nodes (7): args, command, args, command, mcpServers, filesystem, graphify

### Community 23 - "Community 23"
Cohesion: 0.25
Nodes (7): Phase 1: Core Contracts ✅, Phase 2: Ingress Layer (api-service), Phase 3: The Worker Pool (worker-service) ✅, Phase 4: Dispatch & DLQ (notification-router) ✅, Phase 5: Observability (Prometheus Metrics), Phase 6: Production (Docker + Kubernetes), Task Checklist — Distributed Notification Service

### Community 25 - "Community 25"
Cohesion: 0.25
Nodes (7): Channel, EmailPayload, Job, Priority, PushPayload, SMSPayload, Status

### Community 28 - "Community 28"
Cohesion: 0.33
Nodes (5): 1. System Architecture, 2. Infrastructure, 3. Agent Rules, 4. Constraints, Core System Context: Distributed Notification Service

## Knowledge Gaps
- **99 isolated node(s):** `command`, `args`, `command`, `args`, `isSendNotificationRequest_Payload` (+94 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **20 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `SendNotificationRequest` connect `Community 13` to `Community 0`, `Community 29`?**
  _High betweenness centrality (0.011) - this node is a cross-community bridge._
- **Why does `SendNotificationRequest` connect `Community 12` to `Community 10`, `Community 3`?**
  _High betweenness centrality (0.011) - this node is a cross-community bridge._
- **Why does `DispatchRequest` connect `Community 14` to `Community 0`, `Community 29`?**
  _High betweenness centrality (0.010) - this node is a cross-community bridge._
- **What connects `command`, `args`, `command` to the rest of the system?**
  _99 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.125 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.047619047619047616 - nodes in this community are weakly interconnected._
- **Should `Community 3` be split into smaller, more focused modules?**
  _Cohesion score 0.057692307692307696 - nodes in this community are weakly interconnected._