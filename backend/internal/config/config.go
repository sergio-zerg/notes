package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPPort string
	GRPCPort string
	DBHost   string
	DBPort   string
	DBUser   string
	DBPass   string
	DBName   string

	// APIKeys are the shared secrets accepted on the notes endpoints. An empty
	// slice leaves authentication switched off, which is convenient locally but
	// must never happen in a deployment; NewAPIKeyAuthenticator refuses it
	// unless AllowInsecure explicitly permits it.
	APIKeys          []string
	AllowInsecure    bool
	RateLimitEnabled bool
	RateLimitRPS     float64
	RateLimitBurst   int

	TraceExporter   string
	OTLPEndpoint    string
	TraceSampleRate float64

	// ShutdownTimeout bounds the graceful shutdown of both transports. It is
	// captured here, alongside every other setting, so that the whole
	// configuration is one immutable snapshot: mutating the environment after
	// startup must not silently change the behaviour of a running process.
	ShutdownTimeout time.Duration
}

func Load() *Config {
	return &Config{
		HTTPPort: getEnv("HTTP_PORT", ":8080"),
		GRPCPort: getEnv("GRPC_PORT", ":50051"),
		DBHost:   getEnv("DB_HOST", "localhost"),
		DBPort:   getEnv("DB_PORT", "5432"),
		DBUser:   getEnv("DB_USER", "postgres"),
		DBPass:   getEnv("DB_PASS", "password"),
		DBName:   getEnv("DB_NAME", "notes_db"),

		APIKeys:          splitList(getEnv("API_KEYS", "")),
		AllowInsecure:    getEnvBool("ALLOW_INSECURE_AUTH", false),
		RateLimitEnabled: getEnvBool("RATE_LIMIT_ENABLED", true),
		RateLimitRPS:     getEnvFloat("RATE_LIMIT_RPS", 20),
		RateLimitBurst:   getEnvInt("RATE_LIMIT_BURST", 40),

		TraceExporter:   strings.ToLower(getEnv("TRACE_EXPORTER", "stdout")),
		OTLPEndpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		TraceSampleRate: getEnvFloat("OTEL_TRACES_SAMPLER_ARG", 1.0),

		ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

// Validate rejects a configuration that would start the service in a state its
// operator did not intend. It is called once, at startup, so an unusable
// combination fails immediately instead of surfacing on the first request.
func (c *Config) Validate() error {
	if len(c.APIKeys) == 0 && !c.AllowInsecure {
		return fmt.Errorf("API_KEYS is empty: set at least one key, or set ALLOW_INSECURE_AUTH=true to run without authentication")
	}

	if c.RateLimitEnabled {
		if c.RateLimitRPS <= 0 {
			return fmt.Errorf("RATE_LIMIT_RPS must be positive, got %v", c.RateLimitRPS)
		}
		if c.RateLimitBurst < 1 {
			return fmt.Errorf("RATE_LIMIT_BURST must be at least 1, got %d", c.RateLimitBurst)
		}
	}

	switch c.TraceExporter {
	case "stdout", "otlp", "none":
	default:
		return fmt.Errorf("TRACE_EXPORTER must be one of stdout, otlp, none; got %q", c.TraceExporter)
	}

	// A ratio outside [0,1] is silently clamped by the SDK, which would hide
	// the operator's mistake behind unexpected sampling behaviour.
	if c.TraceSampleRate < 0 || c.TraceSampleRate > 1 {
		return fmt.Errorf("OTEL_TRACES_SAMPLER_ARG must be within [0,1], got %v", c.TraceSampleRate)
	}

	// A zero or negative budget would make the context expire before the
	// signal is even delivered, turning a graceful shutdown into an immediate
	// kill of every in-flight request.
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive, got %v", c.ShutdownTimeout)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// splitList parses a comma-separated environment value, dropping blank entries
// so that a trailing comma does not create an empty key that would let anyone
// in by sending an empty header.
func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if value, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseBool(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if parsed, err := time.ParseDuration(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return fallback
}
