package queue

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available on localhost:6379, skipping test: %v", err)
	}

	// Clean up lists used in tests
	client.Del(ctx, "notifications:pending", "notifications:processing")
	
	return client
}

func TestProducerConsumerFlow(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	ctx := context.Background()
	producer := NewProducer(client)
	consumer := NewConsumer(client)

	jobID := uuid.New().String()
	idempotencyKey := uuid.New().String()

	job := &Job{
		JobID:          jobID,
		IdempotencyKey: idempotencyKey,
		Channel:        ChannelEmail,
		Priority:       PriorityNormal,
		Status:         StatusPending,
		Email: &EmailPayload{
			From:    "test@example.com",
			To:      []string{"user@example.com"},
			Subject: "Test",
		},
		// Truncate to Millisecond to ensure JSON unmarshal exact match
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}

	// 1. Enqueue
	err := producer.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Verify pending length is 1
	pendingLen := client.LLen(ctx, "notifications:pending").Val()
	if pendingLen != 1 {
		t.Fatalf("Expected pending length 1, got %d", pendingLen)
	}

	// 2. Dequeue
	popCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	
	poppedJob, err := consumer.Dequeue(popCtx)
	if err != nil {
		t.Fatalf("Failed to dequeue: %v", err)
	}

	if poppedJob.JobID != job.JobID {
		t.Fatalf("Expected job ID %s, got %s", job.JobID, poppedJob.JobID)
	}

	// Verify pending is 0, processing is 1
	pendingLen = client.LLen(ctx, "notifications:pending").Val()
	processingLen := client.LLen(ctx, "notifications:processing").Val()
	if pendingLen != 0 {
		t.Fatalf("Expected pending length 0, got %d", pendingLen)
	}
	if processingLen != 1 {
		t.Fatalf("Expected processing length 1, got %d", processingLen)
	}

	// 3. Requeue
	poppedJob.Attempt++
	err = consumer.Requeue(ctx, poppedJob)
	if err != nil {
		t.Fatalf("Failed to requeue: %v", err)
	}

	pendingLen = client.LLen(ctx, "notifications:pending").Val()
	if pendingLen != 1 {
		t.Fatalf("Expected pending length 1 after requeue, got %d", pendingLen)
	}

	// 4. Ack the original popped job (which removes the old state from processing)
	err = consumer.Ack(ctx, job) // Note: using the unmodified job to ACK
	if err != nil {
		t.Fatalf("Failed to ack: %v", err)
	}

	processingLen = client.LLen(ctx, "notifications:processing").Val()
	if processingLen != 0 {
		t.Fatalf("Expected processing length 0 after ack, got %d", processingLen)
	}
}
