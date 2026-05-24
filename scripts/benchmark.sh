#!/bin/bash
set -e

echo "Starting Benchmark against Notification API..."
echo "Target: localhost:50051"

# We assume you have ghz installed: go install github.com/bojand/ghz/cmd/ghz@latest
# Alternatively, you can use the Docker image.

docker run --rm --network host -v $(pwd):/workspace ghcr.io/bojand/ghz:latest \
  --insecure \
  --proto /workspace/proto/notification.proto \
  --call notification.v1.NotificationAPI.Send \
  -d '{
    "idempotency_key": "{{newUUID}}",
    "channel": "CHANNEL_EMAIL",
    "priority": "PRIORITY_HIGH",
    "email": {
      "from": "bench@example.com",
      "to": ["user@example.com"],
      "subject": "Load Test",
      "body_html": "<p>High load</p>"
    }
  }' \
  -m '{"authorization": "Bearer test-api-key"}' \
  -c 500 \
  -z 30s \
  localhost:50051

echo "Benchmark Complete."
