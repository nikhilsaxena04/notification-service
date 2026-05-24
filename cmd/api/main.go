package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"notification-service/internal/api"
	"notification-service/pkg/auth"
	"notification-service/pkg/config"
	"notification-service/pkg/queue"
	"notification-service/pkg/tracing"
	pb "notification-service/proto/notificationpb"
)

func main() {
	// Parse config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize OpenTelemetry Tracing
	tp, err := tracing.InitTracer(context.Background(), "api-service", cfg.OTLPEndpoint)
	if err != nil {
		slog.Error("Failed to initialize tracer", "error", err)
	} else {
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				slog.Error("Error shutting down tracer", "error", err)
			}
		}()
	}

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}

	// Hexagonal DI
	producer := queue.NewProducer(redisClient)
	svc := api.NewService(producer)
	handler := api.NewHandler(svc)

	// Start metrics server
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		slog.Info("Starting API metrics server", "port", cfg.MetricsPortAPI)
		if err := http.ListenAndServe(":"+cfg.MetricsPortAPI, mux); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	// Setup gRPC server
	lis, err := net.Listen("tcp", ":"+cfg.GRPCPortAPI)
	if err != nil {
		slog.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			otelgrpc.UnaryServerInterceptor(),
			auth.UnaryServerInterceptor(cfg.APIKey),
		),
	)
	pb.RegisterNotificationAPIServer(grpcServer, handler)
	reflection.Register(grpcServer)

	go func() {
		slog.Info("Starting API gRPC server", "port", cfg.GRPCPortAPI)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down API server gracefully...")
	grpcServer.GracefulStop()
	slog.Info("API server stopped")
}
