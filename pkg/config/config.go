package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Config holds the environment configuration for all services.
type Config struct {
	// Redis
	RedisAddr string `envconfig:"NS_REDIS_ADDR" default:"localhost:6379"`

	// Ports
	GRPCPortAPI    string `envconfig:"NS_GRPC_PORT_API" default:"50051"`
	GRPCPortRouter string `envconfig:"NS_GRPC_PORT_ROUTER" default:"50052"`

	// Metrics Ports
	MetricsPortAPI    string `envconfig:"NS_METRICS_PORT_API" default:"8081"`
	MetricsPortWorker string `envconfig:"NS_METRICS_PORT_WORKER" default:"8082"`
	MetricsPortRouter string `envconfig:"NS_METRICS_PORT_ROUTER" default:"8083"`

	// Tuning
	WorkerConcurrency int `envconfig:"NS_WORKER_CONCURRENCY" default:"10"`

	// Authentication
	APIKey string `envconfig:"NS_API_KEY" default:""` // Empty means auth is disabled

	// OpenTelemetry Tracing
	OTLPEndpoint string `envconfig:"NS_OTLP_ENDPOINT" default:"localhost:4317"`
}

// Load parses environment variables into a Config struct.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("NS", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
