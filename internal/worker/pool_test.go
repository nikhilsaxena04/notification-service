package worker

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"notification-service/pkg/queue"
	pb "notification-service/proto/notificationpb"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available on localhost:6379: %v", err)
	}
	client.Del(ctx, "notifications:pending", "notifications:processing")
	return client
}

type mockRouter struct {
	pb.NotificationRouterClient
	dispatched int32
	success    bool
}

func (m *mockRouter) Dispatch(ctx context.Context, in *pb.DispatchRequest, opts ...grpc.CallOption) (*pb.DispatchResponse, error) {
	atomic.AddInt32(&m.dispatched, 1)
	return &pb.DispatchResponse{
		Success:    m.success,
		ProviderId: "mock-123",
	}, nil
}

func TestPoolAndDispatcher(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	consumer := queue.NewConsumer(client)
	producer := queue.NewProducer(client)
	dlq := queue.NewDLQ(client)
	router := &mockRouter{success: true}

	dispatcher := NewDispatcher(router, consumer, dlq, logger)
	pool := NewPool(2, consumer, dispatcher, logger)

	ctx := context.Background()
	pool.Start(ctx)

	// Enqueue a test job
	job := &queue.Job{
		JobID:          uuid.New().String(),
		IdempotencyKey: uuid.New().String(),
		Channel:        queue.ChannelEmail,
		MaxRetries:     3,
	}
	if err := producer.Enqueue(ctx, job); err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	pool.Stop()

	// Verify dispatch was called
	if atomic.LoadInt32(&router.dispatched) != 1 {
		t.Errorf("Expected 1 dispatch, got %d", atomic.LoadInt32(&router.dispatched))
	}

	// Verify queues are empty
	if l := client.LLen(ctx, "notifications:pending").Val(); l != 0 {
		t.Errorf("Pending queue not empty: %d", l)
	}
	if l := client.LLen(ctx, "notifications:processing").Val(); l != 0 {
		t.Errorf("Processing queue not empty: %d", l)
	}
}

func TestDispatcherRetry(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	consumer := queue.NewConsumer(client)
	dlq := queue.NewDLQ(client)
	// Mock router to ALWAYS fail
	router := &mockRouter{success: false}

	dispatcher := NewDispatcher(router, consumer, dlq, logger)
	
	ctx := context.Background()

	job := &queue.Job{
		JobID:          uuid.New().String(),
		IdempotencyKey: uuid.New().String(),
		Channel:        queue.ChannelSMS,
		MaxRetries:     3,
		Attempt:        0,
		Priority:       queue.PriorityLow,
	}
	
	// Test dispatch directly, which will handle the failure and requeue
	dispatcher.Dispatch(ctx, job)
	
	// Should be requeued in pending
	pendingLen := client.LLen(ctx, "notifications:pending").Val()
	if pendingLen != 1 {
		t.Errorf("Expected 1 job in pending after failure, got %d", pendingLen)
	}
}
