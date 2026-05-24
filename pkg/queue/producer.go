package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

var (
	ErrDuplicateJob = errors.New("duplicate job: idempotency key already exists")
)

// Producer handles enqueuing jobs to Redis.
type Producer struct {
	client *redis.Client
}

// NewProducer creates a new Producer.
func NewProducer(client *redis.Client) *Producer {
	return &Producer{
		client: client,
	}
}

// Enqueue pushes a job to the pending queue.
// It uses SET NX to ensure idempotency using the job's IdempotencyKey.
func (p *Producer) Enqueue(ctx context.Context, job *Job) error {
	// 1. Idempotency guard: SET NX idempotency:<key> EX 86400
	idempKey := "idempotency:" + job.IdempotencyKey
	setNX, err := p.client.SetNX(ctx, idempKey, job.JobID, 24*time.Hour).Result()
	if err != nil {
		return err
	}
	if !setNX {
		return ErrDuplicateJob
	}

	// 2. Marshal job to JSON
	job.TraceCarrier = make(map[string]string)
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(job.TraceCarrier))

	jobJSON, err := json.Marshal(job)
	if err != nil {
		// Cleanup idempotency key if marshal fails
		p.client.Del(ctx, idempKey)
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// 3. RPUSH to notifications:pending
	err = p.client.RPush(ctx, "notifications:pending", jobJSON).Err()
	if err != nil {
		// Cleanup idempotency key if RPUSH fails
		p.client.Del(ctx, idempKey)
		return err
	}

	return nil
}
