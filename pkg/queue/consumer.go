package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNoJob = errors.New("no job available")
)

// Consumer handles dequeuing jobs from Redis and managing their lifecycle.
type Consumer struct {
	client *redis.Client
}

// NewConsumer creates a new Consumer.
func NewConsumer(client *redis.Client) *Consumer {
	return &Consumer{
		client: client,
	}
}

// Dequeue pops a job from the pending queue and moves it to the processing queue.
// This is a blocking operation until a job is available or the context is canceled.
func (c *Consumer) Dequeue(ctx context.Context) (*Job, error) {
	// BLMOVE from notifications:pending to notifications:processing
	// LEFT means pop from the left (head) of the list.
	// RIGHT means push to the right (tail) of the list.
	// 2*time.Second ensures it periodically unblocks to check context cancellation.
	res, err := c.client.BLMove(ctx, "notifications:pending", "notifications:processing", "LEFT", "RIGHT", 2*time.Second).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNoJob
		}
		return nil, err
	}

	var job Job
	if err := json.Unmarshal([]byte(res), &job); err != nil {
		return nil, err
	}

	return &job, nil
}

// Ack removes the job from the processing queue.
// It uses the exact state of the job to ensure the serialized JSON matches what's in Redis.
func (c *Consumer) Ack(ctx context.Context, job *Job) error {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return err
	}

	return c.client.LRem(ctx, "notifications:processing", 1, jobJSON).Err()
}

// Requeue pushes the job back to the pending queue.
// Used for transient failures after updating attempt counts and backoff.
func (c *Consumer) Requeue(ctx context.Context, job *Job) error {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return err
	}

	return c.client.RPush(ctx, "notifications:pending", jobJSON).Err()
}
