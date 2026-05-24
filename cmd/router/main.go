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
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"notification-service/internal/router"
	"notification-service/pkg/config"
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
	tp, err := tracing.InitTracer(context.Background(), "notification-router", cfg.OTLPEndpoint)
	if err != nil {
		slog.Error("Failed to initialize tracer", "error", err)
	} else {
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				slog.Error("Error shutting down tracer", "error", err)
			}
		}()
	}

	handler := router.NewHandler()

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPortRouter)
	if err != nil {
		slog.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
	)
	pb.RegisterNotificationRouterServer(grpcServer, handler)
	reflection.Register(grpcServer)

	go func() {
		slog.Info("Starting notification-router gRPC server", "addr", ":"+cfg.GRPCPortRouter)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		slog.Info("Starting Router metrics server", "port", cfg.MetricsPortRouter)
		if err := http.ListenAndServe(":"+cfg.MetricsPortRouter, mux); err != nil && err != http.ErrServerClosed {
			slog.Error("Router metrics server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down router gracefully...")
	grpcServer.GracefulStop()
	slog.Info("Router stopped")
}
