// Package config provides centralized configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns an env var value or a default.
func String(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// Int returns an env var as int or a default.
func Int(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultValue
}

// Int64 returns an env var as int64 or a default.
func Int64(key string, defaultValue int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return defaultValue
}

// Bool returns an env var as bool or a default.
func Bool(key string, defaultValue bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultValue
}

// Duration returns an env var as duration or a default.
func Duration(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}

// List returns an env var split by comma.
func List(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// Required returns an env var or panics if missing.
func Required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required env var missing: " + key)
	}
	return v
}

// RequiredE returns an env var or an error if missing.
func RequiredE(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", &MissingEnvError{Key: key}
	}
	return v, nil
}

// MissingEnvError is returned when a required env var is missing.
type MissingEnvError struct {
	Key string
}

func (e *MissingEnvError) Error() string {
	return "required env var missing: " + e.Key
}
