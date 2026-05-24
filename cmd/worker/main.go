package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"notification-service/internal/worker"
	"notification-service/pkg/config"
	"notification-service/pkg/queue"
	pb "notification-service/proto/notificationpb"
)

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Setup structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 3. Initialize Redis client
	// Set PoolSize to at least WORKER_CONCURRENCY + 5 to avoid connection exhaustion during blocking pops
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		PoolSize: cfg.WorkerConcurrency + 5,
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()
	logger.Info("Connected to Redis", "addr", cfg.RedisAddr)

	// 4. Connect to Notification Router gRPC service
	routerAddr := cfg.GRPCPortRouter
	if _, _, err := net.SplitHostPort(routerAddr); err != nil {
		routerAddr = ":" + cfg.GRPCPortRouter
	}

	conn, err := grpc.Dial(
		routerAddr, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("Failed to dial router service", "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	routerClient := pb.NewNotificationRouterClient(conn)

	// 5. Wire dependencies
	consumer := queue.NewConsumer(rdb)
	dlq := queue.NewDLQ(rdb)
	dispatcher := worker.NewDispatcher(routerClient, consumer, dlq, logger)
	pool := worker.NewPool(cfg.WorkerConcurrency, consumer, dispatcher, logger)

	// 6. Start the worker pool
	pool.Start(ctx)

	// 7. Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("Received termination signal", "signal", sig.String())

	// 8. Graceful shutdown
	pool.Stop()
	logger.Info("Worker service shutdown complete")
}
