// Package config loads runtime configuration from the environment.
package config

import "os"

// Config holds all runtime settings for the panel.
type Config struct {
	// Addr is the host:port the HTTP API listens on, e.g. ":8080".
	Addr string
}

// Load reads configuration from environment variables, applying defaults.
func Load() Config {
	return Config{
		Addr: env("WISP_ADDR", ":8080"),
	}
}

// env returns the value of key, or def if the variable is unset or empty.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
