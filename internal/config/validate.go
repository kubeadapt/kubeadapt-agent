package config

import (
	"fmt"
	"strings"
	"time"
)

// Validate checks that the Config contains valid values.
// Returns an error describing the first invalid field found.
func (c Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("config: KUBEADAPT_API_KEY is required")
	}

	if c.BackendURL == "" {
		return fmt.Errorf("config: KUBEADAPT_BACKEND_URL is required")
	}
	if !c.AllowInsecure && !strings.HasPrefix(c.BackendURL, "https://") {
		return fmt.Errorf("config: KUBEADAPT_BACKEND_URL must use https:// (got %q); set KUBEADAPT_ALLOW_INSECURE=true to override", c.BackendURL)
	}

	if c.SnapshotInterval < 10*time.Second {
		return fmt.Errorf("config: SnapshotInterval must be >= 10s, got %v", c.SnapshotInterval)
	}

	if c.MetricsInterval < 10*time.Second {
		return fmt.Errorf("config: MetricsInterval must be >= 10s, got %v", c.MetricsInterval)
	}

	if c.CompressionLevel < 1 || c.CompressionLevel > 4 {
		return fmt.Errorf("config: CompressionLevel must be 1-4, got %d", c.CompressionLevel)
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("config: MaxRetries must be >= 0, got %d", c.MaxRetries)
	}

	if c.HealthPort < 1 || c.HealthPort > 65535 {
		return fmt.Errorf("config: HealthPort must be 1-65535, got %d", c.HealthPort)
	}

	return nil
}
