package config
import (
	"fmt"
	"os"
	"strings"
)
type Config struct {
	Env string
	Addr string
	DSN         string
	ClerkAPIKey string
	ClerkPublishableKey string
	ClerkFrontendAPI    string
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
}
func Load() (Config, error) {
	cfg := Config{
		Env:                 os.Getenv("ENV"),
		Addr:                os.Getenv("ADDR"),
		DSN:                 os.Getenv("DSN"),
		ClerkAPIKey:         os.Getenv("CLERK_API_KEY"),
		ClerkPublishableKey: os.Getenv("CLERK_PUBLISHABLE_KEY"),
		ClerkFrontendAPI:    strings.TrimRight(os.Getenv("CLERK_FRONTEND_API"), "/"),
		R2AccountID:         os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:       os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:   os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:            os.Getenv("R2_BUCKET"),
	}
	if cfg.Addr == "" {
		cfg.Addr = ":3000"
	}
	required := []struct{ name, value string }{
		{"DSN", cfg.DSN},
		{"CLERK_API_KEY", cfg.ClerkAPIKey},
		{"CLERK_PUBLISHABLE_KEY", cfg.ClerkPublishableKey},
		{"CLERK_FRONTEND_API", cfg.ClerkFrontendAPI},
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
func (c Config) Development() bool {
	switch c.Env {
	case "development", "dev", "local":
		return true
	}
	return false
}
