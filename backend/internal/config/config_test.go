package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Hermetic: the surrounding CI sets some of these in the job environment,
	// and getEnv treats an empty value as set, so unset them (restoring after)
	// to actually exercise the built-in defaults.
	for _, k := range []string{
		"HTTP_PORT", "GRPC_PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME",
		"RATE_LIMIT_ENABLED", "RATE_LIMIT_RPS", "RATE_LIMIT_BURST",
		"TRACE_EXPORTER", "API_KEYS", "ALLOW_INSECURE_AUTH", "CORS_ALLOWED_ORIGINS", "SHUTDOWN_TIMEOUT",
	} {
		if orig, ok := os.LookupEnv(k); ok {
			os.Unsetenv(k)
			t.Cleanup(func() { os.Setenv(k, orig) })
		}
	}

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

// A wildcard is the default so a web client works without configuration, but a
// deployment that lists origins must be able to express them.
func TestCORSAllowedOrigins(t *testing.T) {
	t.Setenv("API_KEYS", "key")

	t.Run("defaults to the wildcard", func(t *testing.T) {
		unsetEnv(t, "CORS_ALLOWED_ORIGINS")

		cfg := Load()
		if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "*" {
			t.Errorf("CORSAllowedOrigins = %v, want [*]", cfg.CORSAllowedOrigins)
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil for the wildcard default", err)
		}
	})

	t.Run("parses a comma-separated list", func(t *testing.T) {
		t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:8081, https://app.example.com")

		cfg := Load()
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
		want := []string{"http://localhost:8081", "https://app.example.com"}
		if len(cfg.CORSAllowedOrigins) != len(want) {
			t.Fatalf("CORSAllowedOrigins = %v, want %v", cfg.CORSAllowedOrigins, want)
		}
		for i, origin := range want {
			if cfg.CORSAllowedOrigins[i] != origin {
				t.Errorf("CORSAllowedOrigins[%d] = %q, want %q", i, cfg.CORSAllowedOrigins[i], origin)
			}
		}
	})

	t.Run("a port is optional", func(t *testing.T) {
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")

		if err := Load().Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
}

// A malformed origin silently never matches the header a browser sends, which
// the client sees only as an opaque network error. Catching it at startup turns
// that into a clear failure.
func TestValidateRejectsMalformedCORSOrigins(t *testing.T) {
	t.Setenv("API_KEYS", "key")

	for _, origin := range []string{
		"localhost:8081",             // no scheme
		"ftp://localhost:8081",       // unsupported scheme
		"http://",                    // no host
		"http://localhost:8080/",     // trailing slash never matches verbatim
		"http://localhost:8080/api",  // path is not part of an origin
		"http://user@localhost:8080", // credentials have no place here
	} {
		t.Run(origin, func(t *testing.T) {
			t.Setenv("CORS_ALLOWED_ORIGINS", origin)

			err := Load().Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error for CORS_ALLOWED_ORIGINS=%s", origin)
			}
			if !contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
				t.Errorf("error = %v, want it to mention CORS_ALLOWED_ORIGINS", err)
			}
		})
	}
}

func TestShutdownTimeout(t *testing.T) {
	t.Setenv("API_KEYS", "key")

	t.Run("parsed from the environment", func(t *testing.T) {
		t.Setenv("SHUTDOWN_TIMEOUT", "45s")

		if got := Load().ShutdownTimeout; got != 45*time.Second {
			t.Errorf("ShutdownTimeout = %v, want 45s", got)
		}
	})

	t.Run("unparseable value falls back to the default", func(t *testing.T) {
		t.Setenv("SHUTDOWN_TIMEOUT", "nonsense")

		if got := Load().ShutdownTimeout; got != 10*time.Second {
			t.Errorf("ShutdownTimeout = %v, want the 10s default", got)
		}
	})

	t.Run("defaults to 10s when unset", func(t *testing.T) {
		t.Setenv("SHUTDOWN_TIMEOUT", "")

		if got := Load().ShutdownTimeout; got != 10*time.Second {
			t.Errorf("ShutdownTimeout = %v, want the 10s default", got)
		}
	})
}

// Load must capture the whole configuration, including the shutdown budget:
// reading the environment later would let an already-running process change
// behaviour underneath the operator who set it.
func TestLoadSnapshotsShutdownTimeout(t *testing.T) {
	t.Setenv("API_KEYS", "key")
	t.Setenv("SHUTDOWN_TIMEOUT", "45s")

	cfg := Load()

	t.Setenv("SHUTDOWN_TIMEOUT", "1s")

	if cfg.ShutdownTimeout != 45*time.Second {
		t.Errorf("ShutdownTimeout = %v after the env changed, want the 45s captured at Load", cfg.ShutdownTimeout)
	}
}

// A non-positive budget would expire the shutdown context before the signal is
// handled, so it has to be rejected at startup rather than silently accepted.
func TestValidateRejectsNonPositiveShutdownTimeout(t *testing.T) {
	t.Setenv("API_KEYS", "key")

	for _, value := range []string{"0s", "-5s"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SHUTDOWN_TIMEOUT", value)

			err := Load().Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error for SHUTDOWN_TIMEOUT=%s", value)
			}
			if !contains(err.Error(), "SHUTDOWN_TIMEOUT") {
				t.Errorf("error = %v, want it to mention SHUTDOWN_TIMEOUT", err)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

// unsetEnv removes a variable for the duration of the test. t.Setenv only sets
// it, and an empty value is not the same as an absent one: getEnv treats a
// present-but-empty variable as a deliberate override.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	if original, ok := os.LookupEnv(key); ok {
		t.Cleanup(func() { _ = os.Setenv(key, original) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv(key) })
	}
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
}
