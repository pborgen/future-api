// Package config centralizes runtime configuration loaded from the
// environment (with optional .env hydration). The loaded configuration is
// memoized as a process-wide singleton so callers can use config.Get() from
// anywhere without re-reading files or env vars.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
)

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

var (
	once    sync.Once
	cached  Config
	loadErr error
)

// Get returns the process-wide configuration singleton. On first call it
// loads from defaults, an optional .env file, and the environment — in that
// precedence order, lowest to highest. Subsequent calls return the same
// value without re-reading anything.
//
// Panics if the .env file is malformed: the server cannot start with a
// misconfigured app and we want to fail loudly.
func Get() Config {
	once.Do(func() {
		cached, loadErr = load()
	})
	if loadErr != nil {
		panic(fmt.Sprintf("config: %v", loadErr))
	}
	return cached
}

func load() (Config, error) {
	// If a .env file exists in the working directory, hydrate any vars it
	// defines into the process environment (without overwriting values that
	// were already set by the real shell — real env always wins).
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	c := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		HTTPAddr:      os.Getenv("HTTP_ADDR"),
		MigrationsDir: os.Getenv("MIGRATIONS_DIR"),
		SeedFile:      os.Getenv("SEED_FILE"),
		GinMode:       os.Getenv("GIN_MODE"),
	}

	return c, nil
}

// loadDotEnv reads a simple KEY=VALUE file and populates os.Environ for any
// keys not already set. Lines may be blank, start with `#` (comment), or be
// of the form `KEY=VALUE` / `export KEY=VALUE`. Values may be optionally
// wrapped in single or double quotes; quotes are stripped. Missing files are
// not an error — they just mean "no .env, that's fine".
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNo)
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])

		// Strip a single matched pair of surrounding quotes.
		if len(val) >= 2 {
			first, last := val[0], val[len(val)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		// Real shell env always wins.
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return fmt.Errorf("%s:%d: setenv %s: %w", path, lineNo, key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}
