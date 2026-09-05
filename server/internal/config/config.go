// Package config reads the process environment exactly once, at startup, into
// a struct the rest of the program is handed. Nothing outside this package
// calls os.Getenv: a missing variable is a startup failure that names it,
// not a nil-pointer or an empty string discovered on the first request that
// needed it.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	// Env is the deployment environment. Only an explicit development value
	// relaxes anything; a missing or misspelled one is treated as production.
	Env string

	// Addr is the listen address, ":3000" unless ADDR says otherwise.
	Addr string

	DSN         string
	ClerkAPIKey string

	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
}

// Load reads the environment and reports every required variable that is
// missing in one error rather than the first one.
func Load() (Config, error) {
	cfg := Config{
		Env:               os.Getenv("ENV"),
		Addr:              os.Getenv("ADDR"),
		DSN:               os.Getenv("DSN"),
		ClerkAPIKey:       os.Getenv("CLERK_API_KEY"),
		R2AccountID:       os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:          os.Getenv("R2_BUCKET"),
	}
	if cfg.Addr == "" {
		cfg.Addr = ":3000"
	}

	required := []struct{ name, value string }{
		{"DSN", cfg.DSN},
		{"CLERK_API_KEY", cfg.ClerkAPIKey},
		{"R2_ACCOUNT_ID", cfg.R2AccountID},
		{"R2_ACCESS_KEY_ID", cfg.R2AccessKeyID},
		{"R2_SECRET_ACCESS_KEY", cfg.R2SecretAccessKey},
		{"R2_BUCKET", cfg.R2Bucket},
	}
	var missing []string
	for _, v := range required {
		if v.value == "" {
			missing = append(missing, v.name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("config: missing environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

// Development reports whether Env names a local development environment.
// This is the only thing that ever relaxes a security default, and it has to
// be asked for by name.
func (c Config) Development() bool {
	switch c.Env {
	case "development", "dev", "local":
		return true
	}
	return false
}
