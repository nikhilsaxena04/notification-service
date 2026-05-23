package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Config holds the environment configuration for all services.
type Config struct {
	RedisAddr         string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	GRPCPortAPI       string `envconfig:"GRPC_PORT_API" default:"50051"`
	GRPCPortRouter    string `envconfig:"GRPC_PORT_ROUTER" default:"50052"`
	MetricsPortAPI    string `envconfig:"METRICS_PORT_API" default:"8081"`
	MetricsPortWorker string `envconfig:"METRICS_PORT_WORKER" default:"8082"`
	MetricsPortRouter string `envconfig:"METRICS_PORT_ROUTER" default:"8083"`
	WorkerConcurrency int    `envconfig:"WORKER_CONCURRENCY" default:"10"`
}

// Load parses environment variables into a Config struct.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("NS", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
