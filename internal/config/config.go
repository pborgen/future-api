// Package config centralizes runtime configuration loaded from the environment.
package config

import "os"

// Config holds all system configuration for the Future Appointment API.
type Config struct {
	// DatabaseURL is the Postgres DSN used by the application.
	DatabaseURL string
	// HTTPAddr is the address (host:port) the HTTP server binds to.
	HTTPAddr string
	// MigrationsDir is the filesystem path containing SQL migration files.
	MigrationsDir string
	// SeedFile is the path to the JSON seed file loaded at startup.
	SeedFile string
	// GinMode is the gin framework mode (debug, release, test). Empty means
	// the application picks a sensible default.
	GinMode string
}

// Load reads configuration from environment variables, falling back to
// development-friendly defaults when a variable is unset.
func Load() Config {
	return Config{
		DatabaseURL:   envOr("DATABASE_URL", "postgres://future:future@localhost:5432/future?sslmode=disable"),
		HTTPAddr:      envOr("HTTP_ADDR", ":8080"),
		MigrationsDir: envOr("MIGRATIONS_DIR", "migrations"),
		SeedFile:      envOr("SEED_FILE", "appointments.json"),
		GinMode:       os.Getenv("GIN_MODE"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
