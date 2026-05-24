package worker

import (
	"context"
	"log/slog"
	"math/rand"
	"time"

	"notification-service/pkg/queue"
	pb "notification-service/proto/notificationpb"
)

// Dispatcher is responsible for mapping jobs to gRPC router calls and managing retries.
type Dispatcher struct {
	routerClient pb.NotificationRouterClient
	consumer     *queue.Consumer
	dlq          *queue.DLQ
	logger       *slog.Logger
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(routerClient pb.NotificationRouterClient, consumer *queue.Consumer, dlq *queue.DLQ, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		routerClient: routerClient,
		consumer:     consumer,
		dlq:          dlq,
		logger:       logger,
	}
}

// getBackoff calculates the retry backoff duration based on the job's priority and attempt count.
func getBackoff(priority queue.Priority, attempt int) time.Duration {
	var base, maxBackoff time.Duration
	switch priority {
	case queue.PriorityLow:
		base = 5 * time.Second
		maxBackoff = 30 * time.Second
	case queue.PriorityHigh:
		base = 1 * time.Second
		maxBackoff = 120 * time.Second
	case queue.PriorityCritical:
		base = 500 * time.Millisecond
		maxBackoff = 300 * time.Second
	case queue.PriorityNormal:
		fallthrough
	default:
		base = 2 * time.Second
		maxBackoff = 60 * time.Second
	}

	// Calculate exponential backoff: base * 2^attempt
	shift := attempt
	if shift > 30 {
		shift = 30 // prevent overflow
	}
	backoff := base * time.Duration(1<<shift)
	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	// Add up to 200ms of random jitter
	jitter := time.Duration(rand.Intn(200)) * time.Millisecond
	return backoff + jitter
}

// Dispatch executes a single job delivery and handles success/failure paths.
func (d *Dispatcher) Dispatch(ctx context.Context, job *queue.Job) {
	// 1. Map queue.Job to pb.DispatchRequest
	req := &pb.DispatchRequest{
		JobId:   job.JobID,
		Channel: pb.Channel(job.Channel),
		Attempt: int32(job.Attempt),
	}

	switch job.Channel {
	case queue.ChannelEmail:
		if job.Email != nil {
			req.Payload = &pb.DispatchRequest_Email{
				Email: &pb.EmailPayload{
					From:     job.Email.From,
					To:       job.Email.To,
					Cc:       job.Email.CC,
					Subject:  job.Email.Subject,
					BodyHtml: job.Email.BodyHTML,
					BodyText: job.Email.BodyText,
				},
			}
		}
	case queue.ChannelSMS:
		if job.SMS != nil {
			req.Payload = &pb.DispatchRequest_Sms{
				Sms: &pb.SMSPayload{
					From: job.SMS.From,
					To:   job.SMS.To,
					Body: job.SMS.Body,
				},
			}
		}
	case queue.ChannelPush:
		if job.Push != nil {
			req.Payload = &pb.DispatchRequest_Push{
				Push: &pb.PushPayload{
					DeviceToken: job.Push.DeviceToken,
					Title:       job.Push.Title,
					Body:        job.Push.Body,
					Data:        job.Push.Data,
				},
			}
		}
	}

	// 2. Call the router via gRPC
	resp, err := d.routerClient.Dispatch(ctx, req)
	
	// 3. Handle Success
	if err == nil && resp.Success {
		d.logger.Info("Job dispatched successfully", "job_id", job.JobID, "provider_id", resp.ProviderId)
		if ackErr := d.consumer.Ack(ctx, job); ackErr != nil {
			d.logger.Error("Failed to ACK successful job", "job_id", job.JobID, "error", ackErr)
		}
		return
	}

	// 4. Handle Failure
	d.logger.Warn("Job dispatch failed", "job_id", job.JobID, "error", err, "attempt", job.Attempt)
	
	// Clone the job for updating to prevent mutating the original struct needed for Ack
	retryJob := *job

	if retryJob.Attempt >= retryJob.MaxRetries {
		// Terminal failure: Escalate to DLQ
		d.logger.Error("Job reached max retries, escalating to DLQ", "job_id", retryJob.JobID, "max_retries", retryJob.MaxRetries)
		
		// 1. Escalate to DLQ first
		if err := d.dlq.Escalate(ctx, &retryJob); err != nil {
			d.logger.Error("Failed to escalate job to DLQ", "job_id", retryJob.JobID, "error", err)
			return // Do NOT Ack if DLQ push fails, to avoid losing the message completely
		}

		// 2. ACK from processing only if DLQ escalation succeeded
		if ackErr := d.consumer.Ack(ctx, job); ackErr != nil {
			d.logger.Error("Failed to ACK terminally failed job after DLQ escalation", "job_id", job.JobID, "error", ackErr)
		}
		return
	}

	// Transient failure -> Requeue
	backoff := getBackoff(retryJob.Priority, retryJob.Attempt)
	retryJob.Attempt++
	retryJob.NextRetryAt = time.Now().Add(backoff).UTC()
	retryJob.UpdatedAt = time.Now().UTC()

	if reqErr := d.consumer.Requeue(ctx, &retryJob); reqErr != nil {
		d.logger.Error("Failed to requeue job", "job_id", retryJob.JobID, "error", reqErr)
		// We do NOT ack if requeue fails, so the job remains in processing for a crash-recovery reaper
		return
	}

	// Requeue succeeded, remove the old job from processing
	if ackErr := d.consumer.Ack(ctx, job); ackErr != nil {
		d.logger.Error("Failed to ACK job after requeue", "job_id", job.JobID, "error", ackErr)
	}
}
