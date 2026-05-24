// Package queue defines the Job struct that is serialized to JSON
// and pushed onto the Redis notification queue.
package queue

import "time"

// Channel represents the notification delivery channel.
type Channel int

const (
	ChannelUnspecified Channel = iota
	ChannelEmail
	ChannelSMS
	ChannelPush
)

// Priority controls queue ordering and retry budgets.
type Priority int

const (
	PriorityUnspecified Priority = iota
	PriorityLow
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// Status tracks the lifecycle state of a Job.
type Status int

const (
	StatusPending   Status = iota // Enqueued, not yet picked up.
	StatusActive                  // Popped by a worker, in-flight.
	StatusDelivered               // Provider confirmed delivery.
	StatusFailed                  // All retries exhausted; moved to DLQ.
)

// ──────────────────────────────────────────────────────────────
// Channel-specific payload types
// ──────────────────────────────────────────────────────────────

// EmailPayload holds the fields required to dispatch an email.
type EmailPayload struct {
	From     string   `json:"from"`
	To       []string `json:"to"`
	CC       []string `json:"cc,omitempty"`
	Subject  string   `json:"subject"`
	BodyHTML string   `json:"body_html"`
	BodyText string   `json:"body_text,omitempty"`
}

// SMSPayload holds the fields required to dispatch an SMS.
type SMSPayload struct {
	From string `json:"from"`
	To   string `json:"to"`
	Body string `json:"body"`
}

// PushPayload holds the fields required to dispatch a push notification.
type PushPayload struct {
	DeviceToken string            `json:"device_token"`
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Data        map[string]string `json:"data,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Job — the canonical unit of work in the Redis queue
// ──────────────────────────────────────────────────────────────

// Job is the struct that gets JSON-serialized and pushed to the
// Redis notification queue via RPUSH. Workers pop it with BLMOVE,
// execute the delivery, and either ACK or escalate to the DLQ.
//
// Design decisions:
//   - Flat struct with a type discriminator (Channel) rather than
//     an interface{} payload, to keep JSON round-trips deterministic
//     and avoid reflect-based deserialization.
//   - IdempotencyKey is client-provided to enable exactly-once
//     deduplication at the api-service layer (SET NX with TTL).
//   - MaxRetries and Attempt are co-located on the job so the
//     worker pool can make retry decisions without external lookups.
//   - CreatedAt/NextRetryAt use time.Time (RFC 3339 in JSON) for
//     clean interop with Prometheus histogram buckets.
type Job struct {
	// ── Identity ──────────────────────────────────────────────
	// JobID is server-assigned (UUIDv7) for monotonic ordering.
	JobID string `json:"job_id"`
	// IdempotencyKey is client-provided (UUIDv4) for dedup.
	IdempotencyKey string `json:"idempotency_key"`

	// ── Routing ───────────────────────────────────────────────
	Channel  Channel  `json:"channel"`
	Priority Priority `json:"priority"`
	Status   Status   `json:"status"`

	// ── Payload ───────────────────────────────────────────────
	// Exactly one of these is non-nil, matching Channel.
	Email *EmailPayload `json:"email,omitempty"`
	SMS   *SMSPayload   `json:"sms,omitempty"`
	Push  *PushPayload  `json:"push,omitempty"`

	// ── Retry state ───────────────────────────────────────────
	Attempt     int       `json:"attempt"`
	MaxRetries  int       `json:"max_retries"`
	NextRetryAt time.Time `json:"next_retry_at,omitempty"`
	Error       string    `json:"error,omitempty"`

	// ── Distributed Tracing ───────────────────────────────────
	TraceCarrier map[string]string `json:"trace_carrier,omitempty"`

	// ── Metadata ──────────────────────────────────────────────
	// Caller-scoped k/v for tracing and filtering.
	Metadata map[string]string `json:"metadata,omitempty"`

	// ── Timestamps ────────────────────────────────────────────
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
