---
paths:
  - "**/*.go"
---
# Go Backend Rules
- Architecture: Hexagonal. Handlers -> Services -> Repositories/Queues.
- Context: `ctx context.Context` is mandatory as the first parameter for all I/O or long-running functions.
- Redis: Use `go-redis/v9`. Queue logic must use atomic operations (e.g., `BLMOVE` or `RPOPLPUSH`) to prevent lost jobs if a worker crashes.
- Tests: Write table-driven tests for core logic.