package config

import (
	"os"
)

type Config struct {
	HTTPPort string
	GRPCPort string
	DBPath   string
}

func Load() *Config {
	return &Config{
		HTTPPort: getEnv("HTTP_PORT", ":8080"),
		GRPCPort: getEnv("GRPC_PORT", ":50051"),
		DBPath:   getEnv("DB_PATH", "notes.db"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
