package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"notification-service/internal/worker"
	"notification-service/pkg/config"
	"notification-service/pkg/metrics"
	"notification-service/pkg/queue"
	"notification-service/pkg/tracing"
	pb "notification-service/proto/notificationpb"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize OpenTelemetry Tracing
	tp, err := tracing.InitTracer(context.Background(), "worker-service", cfg.OTLPEndpoint)
	if err != nil {
		slog.Error("Failed to initialize tracer", "error", err)
	} else {
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				slog.Error("Error shutting down tracer", "error", err)
			}
		}()
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	slog.Info("Connected to Redis", "addr", cfg.RedisAddr)

	conn, err := grpc.Dial(
		cfg.GRPCPortRouter,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	)
	if err != nil {
		slog.Error("Failed to connect to router", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	routerClient := pb.NewNotificationRouterClient(conn)
	consumer := queue.NewConsumer(redisClient)
	dlq := queue.NewDLQ(redisClient)
	dispatcher := worker.NewDispatcher(routerClient, dlq)
	pool := worker.NewPool(consumer, dispatcher, cfg.WorkerConcurrency)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		slog.Info("Starting Worker metrics server", "port", cfg.MetricsPortWorker)
		if err := http.ListenAndServe(":"+cfg.MetricsPortWorker, mux); err != nil && err != http.ErrServerClosed {
			slog.Error("Worker metrics server failed", "error", err)
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ctx := context.Background()
			pending, _ := redisClient.LLen(ctx, "notifications:pending").Result()
			processing, _ := redisClient.LLen(ctx, "notifications:processing").Result()
			dlq, _ := redisClient.LLen(ctx, "notifications:dlq").Result()

			metrics.QueueDepth.WithLabelValues("pending").Set(float64(pending))
			metrics.QueueDepth.WithLabelValues("processing").Set(float64(processing))
			metrics.QueueDepth.WithLabelValues("dlq").Set(float64(dlq))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("Starting worker pool", "concurrency", cfg.WorkerConcurrency)
	pool.Start(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down worker gracefully...")
	cancel()
	pool.Stop()
	slog.Info("Worker stopped")
}
