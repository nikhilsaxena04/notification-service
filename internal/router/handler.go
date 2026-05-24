package router

import (
	"context"
	"fmt"
	"log/slog"

	pb "notification-service/proto/notificationpb"
)

// Handler implements the NotificationRouterServer gRPC interface.
type Handler struct {
	pb.UnimplementedNotificationRouterServer
	
	emailProvider Provider
	smsProvider   Provider
	pushProvider  Provider
	logger        *slog.Logger
}

// NewHandler creates a new router handler.
func NewHandler(email Provider, sms Provider, push Provider, logger *slog.Logger) *Handler {
	return &Handler{
		emailProvider: email,
		smsProvider:   sms,
		pushProvider:  push,
		logger:        logger,
	}
}

// Dispatch routes the request to the appropriate downstream provider based on the Channel.
func (h *Handler) Dispatch(ctx context.Context, req *pb.DispatchRequest) (*pb.DispatchResponse, error) {
	h.logger.Debug("Received dispatch request", "job_id", req.JobId, "channel", req.Channel.String(), "attempt", req.Attempt)

	var providerID string
	var err error

	switch req.Channel {
	case pb.Channel_CHANNEL_EMAIL:
		if req.GetEmail() == nil {
			return &pb.DispatchResponse{Success: false, ErrorMessage: "missing email payload"}, nil
		}
		providerID, err = h.emailProvider.Send(ctx, req)
	case pb.Channel_CHANNEL_SMS:
		if req.GetSms() == nil {
			return &pb.DispatchResponse{Success: false, ErrorMessage: "missing sms payload"}, nil
		}
		providerID, err = h.smsProvider.Send(ctx, req)
	case pb.Channel_CHANNEL_PUSH:
		if req.GetPush() == nil {
			return &pb.DispatchResponse{Success: false, ErrorMessage: "missing push payload"}, nil
		}
		providerID, err = h.pushProvider.Send(ctx, req)
	default:
		return &pb.DispatchResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("unsupported channel: %v", req.Channel),
		}, nil
	}

	if err != nil {
		h.logger.Error("Provider send failed", "job_id", req.JobId, "channel", req.Channel.String(), "error", err)
		return &pb.DispatchResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &pb.DispatchResponse{
		Success:    true,
		ProviderId: providerID,
	}, nil
}
