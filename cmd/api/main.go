package main

import (
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"notification-service/internal/api"
	"notification-service/pkg/config"
	"notification-service/pkg/queue"
	pb "notification-service/proto/notificationpb"
)

func main() {
	// Parse config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})

	// Hexagonal DI
	producer := queue.NewProducer(redisClient)
	svc := api.NewService(producer)
	handler := api.NewHandler(svc)

	// Setup gRPC server
	lis, err := net.Listen("tcp", ":"+cfg.GRPCPortAPI)
	if err != nil {
		slog.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterNotificationAPIServer(grpcServer, handler)
	reflection.Register(grpcServer)

	// Graceful shutdown
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
	redisClient.Close()
	slog.Info("API server stopped")
}
