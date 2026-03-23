package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const envPrefix = "CP_"

// Config defines shared configuration for ClusterProbe services.
type Config struct {
	PostgresDSN       string
	RedisDSN          string
	MongoDSN          string
	RabbitMQURL       string
	OTLPEndpoint      string
	APIListenAddr     string
	UIListenAddr      string
	ChaosListenAddr   string
	WorkerConcurrency int
	LogLevel          string
}

// Load reads configuration from environment variables using the CP_ prefix.
func Load() (Config, error) {
	var missing []string

	requireString := func(key string) string {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			missing = append(missing, key)
		}
		return value
	}

	requireInt := func(key string) (int, error) {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			missing = append(missing, key)
			return 0, nil
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer: %w", key, err)
		}
		return parsed, nil
	}

	cfg := Config{
		PostgresDSN:     requireString(envPrefix + "POSTGRES_DSN"),
		RedisDSN:        requireString(envPrefix + "REDIS_DSN"),
		MongoDSN:        requireString(envPrefix + "MONGO_DSN"),
		RabbitMQURL:     requireString(envPrefix + "RABBITMQ_URL"),
		OTLPEndpoint:    requireString(envPrefix + "OTEL_ENDPOINT"),
		APIListenAddr:   requireString(envPrefix + "API_LISTEN_ADDR"),
		UIListenAddr:    requireString(envPrefix + "UI_LISTEN_ADDR"),
		ChaosListenAddr: requireString(envPrefix + "CHAOS_LISTEN_ADDR"),
		LogLevel:        requireString(envPrefix + "LOG_LEVEL"),
	}

	workerConcurrency, err := requireInt(envPrefix + "WORKER_CONCURRENCY")
	if err != nil {
		return Config{}, err
	}
	cfg.WorkerConcurrency = workerConcurrency

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate ensures required configuration values are set and sane.
func (c Config) Validate() error {
	if c.WorkerConcurrency <= 0 {
		return fmt.Errorf("%sWORKER_CONCURRENCY must be greater than zero", envPrefix)
	}
	return nil
}
