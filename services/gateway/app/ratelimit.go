package app

import "context"

// RateLimitConfig configures the gateway's fixed-window IP/account rate limiter.
// Overridable via APP_RATELIMIT_*.
type RateLimitConfig struct {
	// Enabled toggles the limiter middleware.
	Enabled bool `koanf:"enabled"`
	// Limit is the max number of requests allowed per window.
	Limit int `koanf:"limit"`
	// WindowSeconds is the window length in seconds.
	WindowSeconds int `koanf:"windowseconds"`
}

// NewRateLimitConfig loads the limiter config from the `ratelimit` sub-tree,
// defaulting to 100 requests / 10s.
func NewRateLimitConfig(_ context.Context, c *KConfig) (*RateLimitConfig, error) {
	cfg := &RateLimitConfig{
		Enabled:       true,
		Limit:         100, //nolint:mnd
		WindowSeconds: 10,  //nolint:mnd
	}
	if err := c.Unmarshal("ratelimit", cfg); err != nil {
		return nil, err
	}

	if cfg.Limit <= 0 {
		cfg.Limit = 100
	}

	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 10
	}

	return cfg, nil
}
