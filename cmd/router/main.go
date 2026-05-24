package main

import (
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"notification-service/internal/router"
	"notification-service/pkg/config"
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

	// 3. Initialize mock providers
	emailProvider := router.NewMockEmailProvider(logger)
	smsProvider := router.NewMockSMSProvider(logger)
	pushProvider := router.NewMockPushProvider(logger)

	// 4. Initialize handler
	handler := router.NewHandler(emailProvider, smsProvider, pushProvider, logger)

	// 5. Setup gRPC server
	port := cfg.GRPCPortRouter
	if port == "" {
		port = "50052" // default router port
	}
	
	// Add colon if missing
	addr := port
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = ":" + port
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterNotificationRouterServer(grpcServer, handler)

	// 6. Start server
	go func() {
		logger.Info("Starting notification-router gRPC server", "addr", addr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve gRPC", "error", err)
		}
	}()

	// 7. Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info("Received termination signal", "signal", sig.String())

	// 8. Graceful shutdown
	grpcServer.GracefulStop()
	logger.Info("notification-router shutdown complete")
}
