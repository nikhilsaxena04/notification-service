package test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"notification-service/internal/api"
	"notification-service/pkg/queue"
	pb "notification-service/proto/notificationpb"
)

func TestIntegration_APItoRedis(t *testing.T) {
	// Setup Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	// Clear queue before test
	ctx := context.Background()
	rdb.Del(ctx, "notifications:pending")

	// Start API Service
	producer := queue.NewProducer(rdb)
	svc := api.NewService(producer)
	handler := api.NewHandler(svc)

	lis, err := net.Listen("tcp", ":0") // random port
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()
	pb.RegisterNotificationAPIServer(grpcServer, handler)
	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Create gRPC client
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewNotificationAPIClient(conn)

	// Send RPC
	idempKey := uuid.New().String()
	req := &pb.SendNotificationRequest{
		IdempotencyKey: idempKey,
		Channel:        pb.Channel_CHANNEL_EMAIL,
		Payload: &pb.SendNotificationRequest_Email{
			Email: &pb.EmailPayload{
				From:     "test@example.com",
				To:       []string{"user@example.com"},
				Subject:  "Integration Test",
				BodyHtml: "<p>Success</p>",
			},
		},
	}

	resp, err := client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if resp.JobId == "" {
		t.Error("Expected non-empty Job ID")
	}

	// Verify Redis queue length
	time.Sleep(100 * time.Millisecond)
	len, err := rdb.LLen(ctx, "notifications:pending").Result()
	if err != nil {
		t.Fatalf("Failed to check redis: %v", err)
	}
	if len != 1 {
		t.Errorf("Expected 1 job in queue, got %d", len)
	}
}
