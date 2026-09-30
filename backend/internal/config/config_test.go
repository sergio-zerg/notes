package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()

	if cfg.HTTPPort != ":8080" {
		t.Errorf("HTTPPort = %q, want :8080", cfg.HTTPPort)
	}
	if cfg.DBName != "notes_db" {
		t.Errorf("DBName = %q, want notes_db", cfg.DBName)
	}
	if !cfg.RateLimitEnabled {
		t.Error("RateLimitEnabled = false, want true by default")
	}
	if cfg.TraceExporter != "stdout" {
		t.Errorf("TraceExporter = %q, want stdout", cfg.TraceExporter)
	}
}

func TestLoadParsesAPIKeys(t *testing.T) {
	t.Setenv("API_KEYS", "key-one, key-two ,key-three")

	cfg := Load()

	want := []string{"key-one", "key-two", "key-three"}
	if len(cfg.APIKeys) != len(want) {
		t.Fatalf("APIKeys = %v, want %v", cfg.APIKeys, want)
	}
	for i, key := range want {
		if cfg.APIKeys[i] != key {
			t.Errorf("APIKeys[%d] = %q, want %q", i, cfg.APIKeys[i], key)
		}
	}
}

// A trailing comma must not yield an empty key that would authenticate anyone
// who omits the header.
func TestLoadDropsBlankAPIKeys(t *testing.T) {
	t.Setenv("API_KEYS", "key-one,,  ,key-two,")

	cfg := Load()

	for i, key := range cfg.APIKeys {
		if key == "" {
			t.Errorf("APIKeys[%d] is empty", i)
		}
	}
	if len(cfg.APIKeys) != 2 {
		t.Errorf("APIKeys = %v, want two entries", cfg.APIKeys)
	}
}

func TestValidateRejectsMissingAPIKeys(t *testing.T) {
	t.Setenv("API_KEYS", "")
	t.Setenv("ALLOW_INSECURE_AUTH", "")

	err := Load().Validate()

	if err == nil {
		t.Fatal("Validate() = nil, want an error when API_KEYS is empty")
	}
	if !contains(err.Error(), "API_KEYS") {
		t.Errorf("error = %v, want it to mention API_KEYS", err)
	}
}

func TestValidateAcceptsEmptyAPIKeysWithOptIn(t *testing.T) {
	t.Setenv("API_KEYS", "")
	t.Setenv("ALLOW_INSECURE_AUTH", "true")

	if err := Load().Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestValidateRejectsBadRateLimit(t *testing.T) {
	t.Setenv("API_KEYS", "key")

	t.Run("non-positive rps", func(t *testing.T) {
		t.Setenv("RATE_LIMIT_RPS", "0")
		if err := Load().Validate(); err == nil {
			t.Error("Validate() = nil, want an error for RATE_LIMIT_RPS=0")
		}
	})

	t.Run("zero burst", func(t *testing.T) {
		t.Setenv("RATE_LIMIT_RPS", "10")
		t.Setenv("RATE_LIMIT_BURST", "0")
		if err := Load().Validate(); err == nil {
			t.Error("Validate() = nil, want an error for RATE_LIMIT_BURST=0")
		}
	})

	t.Run("unparseable rps falls back to the default", func(t *testing.T) {
		t.Setenv("RATE_LIMIT_RPS", "not-a-number")
		t.Setenv("RATE_LIMIT_BURST", "10")
		cfg := Load()
		if cfg.RateLimitRPS != 20 {
			t.Errorf("RateLimitRPS = %v, want the 20 default", cfg.RateLimitRPS)
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
}

func TestValidateRejectsUnknownExporter(t *testing.T) {
	t.Setenv("API_KEYS", "key")
	t.Setenv("TRACE_EXPORTER", "kafka")

	if err := Load().Validate(); err == nil {
		t.Error("Validate() = nil, want an error for an unknown exporter")
	}
}

func TestValidateAcceptsEveryExporter(t *testing.T) {
	t.Setenv("API_KEYS", "key")
	for _, name := range []string{"stdout", "otlp", "none", "OTLP"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("TRACE_EXPORTER", name)
			if err := Load().Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil for %q", err, name)
			}
		})
	}
}

func TestValidateRejectsSampleRateOutsideUnitRange(t *testing.T) {
	t.Setenv("API_KEYS", "key")

	for _, ratio := range []string{"-0.1", "1.5"} {
		t.Run(ratio, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", ratio)
			if err := Load().Validate(); err == nil {
				t.Errorf("Validate() = nil, want an error for ratio %s", ratio)
			}
		})
	}
}

func TestShutdownTimeout(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "45s")

	if got := Load().ShutdownTimeout(); got != 45*time.Second {
		t.Errorf("ShutdownTimeout() = %v, want 45s", got)
	}

	t.Setenv("SHUTDOWN_TIMEOUT", "nonsense")
	if got := Load().ShutdownTimeout(); got != 10*time.Second {
		t.Errorf("ShutdownTimeout() = %v, want the 10s default", got)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
