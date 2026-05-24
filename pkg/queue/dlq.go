package queue

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// DLQ manages the Dead Letter Queue for terminally failed jobs.
type DLQ struct {
	client *redis.Client
}

// NewDLQ creates a new DLQ manager.
func NewDLQ(client *redis.Client) *DLQ {
	return &DLQ{
		client: client,
	}
}

// Escalate pushes a terminally failed job to the DLQ in Redis.
func (d *DLQ) Escalate(ctx context.Context, job *Job) error {
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return err
	}

	// RPUSH to notifications:dlq
	return d.client.RPush(ctx, "notifications:dlq", jobJSON).Err()
}
