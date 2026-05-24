package api

import (
	"context"
	"time"

	"github.com/google/uuid"
	"notification-service/pkg/metrics"
	"notification-service/pkg/queue"
	"notification-service/pkg/validator"
	pb "notification-service/proto/notificationpb"
)

// Service encapsulates the core business logic for the API layer.
type Service struct {
	producer *queue.Producer
}

// NewService creates a new API Service.
func NewService(producer *queue.Producer) *Service {
	return &Service{
		producer: producer,
	}
}

// SendNotification processes a single notification request.
func (s *Service) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	// 1. Validate payload
	if err := validator.ValidateSendRequest(req); err != nil {
		return nil, err
	}

	// 2. Generate UUIDv7 JobID
	jobID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	// 3. Map to queue.Job
	job := &queue.Job{
		JobID:          jobID.String(),
		IdempotencyKey: req.IdempotencyKey,
		Channel:        queue.Channel(req.Channel),
		Priority:       queue.Priority(req.Priority),
		Status:         queue.StatusPending,
		Metadata:       req.Metadata,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Extract payload based on channel
	switch req.Channel {
	case pb.Channel_CHANNEL_EMAIL:
		email := req.GetEmail()
		job.Email = &queue.EmailPayload{
			From:     email.From,
			To:       email.To,
			CC:       email.Cc,
			Subject:  email.Subject,
			BodyHTML: email.BodyHtml,
			BodyText: email.BodyText,
		}
	case pb.Channel_CHANNEL_SMS:
		sms := req.GetSms()
		job.SMS = &queue.SMSPayload{
			From: sms.From,
			To:   sms.To,
			Body: sms.Body,
		}
	case pb.Channel_CHANNEL_PUSH:
		push := req.GetPush()
		job.Push = &queue.PushPayload{
			DeviceToken: push.DeviceToken,
			Title:       push.Title,
			Body:        push.Body,
			Data:        push.Data,
		}
	}

	// Set retry limits based on priority (can be refined later, defaults to 3 for now)
	job.MaxRetries = 3

	// 4. Enqueue (Dedup happens here via SET NX)
	if err := s.producer.Enqueue(ctx, job); err != nil {
		if err == queue.ErrDuplicateJob {
			// If duplicate, we might just return ACCEPTED to be idempotent from client perspective,
			// or we could return an error. Returning a successful response with a known status is usually better.
			// However, since we don't know the exact job ID that was already created, we'll return an error for now
			// or handle it gracefully. Let's return the error.
			return nil, err
		}
		return nil, err
	}

	// Instrument successful enqueue
	metrics.JobsEnqueuedTotal.WithLabelValues(req.Channel.String(), req.Priority.String()).Inc()

	// 5. Return success
	return &pb.SendNotificationResponse{
		JobId: jobID.String(),
		Status: pb.Status_STATUS_ACCEPTED,
	}, nil
}
