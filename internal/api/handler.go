package api

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"notification-service/pkg/queue"
	pb "notification-service/proto/notificationpb"
)

// Handler implements the NotificationAPIServer interface.
type Handler struct {
	pb.UnimplementedNotificationAPIServer
	service *Service
}

// NewHandler creates a new gRPC handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// Send handles a single SendNotificationRequest.
func (h *Handler) Send(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	resp, err := h.service.SendNotification(ctx, req)
	if err != nil {
		if err == queue.ErrDuplicateJob {
			slog.Info("Duplicate job detected", "idempotency_key", req.IdempotencyKey)
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		slog.Error("Failed to send notification", "error", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	
	resp.AcceptedAt = timestamppb.Now()
	return resp, nil
}

// BatchSend handles an array of SendNotificationRequests.
func (h *Handler) BatchSend(ctx context.Context, req *pb.BatchSendRequest) (*pb.BatchSendResponse, error) {
	var results []*pb.SendNotificationResponse
	
	// Process sequentially for simplicity; could be parallelized if needed.
	for _, notifReq := range req.Notifications {
		resp, err := h.service.SendNotification(ctx, notifReq)
		if err != nil {
			// For batch, we could collect errors, but for simplicity we'll just fail the whole batch 
			// or append failed status. Let's append a failed status.
			results = append(results, &pb.SendNotificationResponse{
				Status: pb.Status_STATUS_FAILED,
			})
			slog.Error("Failed to process item in batch", "error", err)
			continue
		}
		resp.AcceptedAt = timestamppb.Now()
		results = append(results, resp)
	}

	return &pb.BatchSendResponse{
		Results: results,
	}, nil
}
