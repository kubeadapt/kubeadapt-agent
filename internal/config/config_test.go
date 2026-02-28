package config

import (
	"os"
	"testing"
	"time"
)

// helper to clear all KUBEADAPT_ env vars before each test
func clearEnv(t *testing.T) {
	t.Helper()
	envVars := []string{
		"KUBEADAPT_API_KEY",
		"KUBEADAPT_CLUSTER_ID",
		"KUBEADAPT_CLUSTER_NAME",
		"KUBEADAPT_BACKEND_URL",
		"KUBEADAPT_SNAPSHOT_INTERVAL",
		"KUBEADAPT_METRICS_INTERVAL",
		"KUBEADAPT_INFORMER_RESYNC",
		"KUBEADAPT_INFORMER_SYNC_TIMEOUT",
		"KUBEADAPT_COMPRESSION_LEVEL",
		"KUBEADAPT_MAX_RETRIES",
		"KUBEADAPT_REQUEST_TIMEOUT",
		"KUBEADAPT_BUFFER_MAX_BYTES",
		"KUBEADAPT_HEALTH_PORT",
		"KUBEADAPT_GPU_METRICS_ENABLED",
		"KUBEADAPT_DCGM_PORT",
		"KUBEADAPT_DCGM_NAMESPACE",
		"KUBEADAPT_GPU_METRICS_INTERVAL",
		"KUBEADAPT_ALLOW_INSECURE",
		"KUBEADAPT_DEBUG_ENDPOINTS",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUBEADAPT_API_KEY", "test-key")

	cfg := Load()

	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key")
	}
	if cfg.ClusterID == "" {
		t.Error("ClusterID should be auto-generated when empty")
	}
	if cfg.BackendURL != "https://api.kubeadapt.io" {
		t.Errorf("BackendURL = %q, want %q", cfg.BackendURL, "https://api.kubeadapt.io")
	}
	if cfg.SnapshotInterval != 60*time.Second {
		t.Errorf("SnapshotInterval = %v, want 60s", cfg.SnapshotInterval)
	}
	if cfg.MetricsInterval != 60*time.Second {
		t.Errorf("MetricsInterval = %v, want 60s", cfg.MetricsInterval)
	}
	if cfg.InformerResyncPeriod != 300*time.Second {
		t.Errorf("InformerResyncPeriod = %v, want 300s", cfg.InformerResyncPeriod)
	}
	if cfg.InformerSyncTimeout != 5*time.Minute {
		t.Errorf("InformerSyncTimeout = %v, want 5m", cfg.InformerSyncTimeout)
	}
	if cfg.CompressionLevel != 3 {
		t.Errorf("CompressionLevel = %d, want 3", cfg.CompressionLevel)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want 30s", cfg.RequestTimeout)
	}
	if cfg.BufferMaxBytes != 52428800 {
		t.Errorf("BufferMaxBytes = %d, want 52428800", cfg.BufferMaxBytes)
	}
	if cfg.HealthPort != 8080 {
		t.Errorf("HealthPort = %d, want 8080", cfg.HealthPort)
	}
	if !cfg.GPUMetricsEnabled {
		t.Error("GPUMetricsEnabled should default to true")
	}
	if cfg.DCGMExporterPort != 9400 {
		t.Errorf("DCGMExporterPort = %d, want 9400", cfg.DCGMExporterPort)
	}
	if cfg.DCGMExporterNamespace != "" {
		t.Errorf("DCGMExporterNamespace = %q, want empty", cfg.DCGMExporterNamespace)
	}
	if cfg.GPUMetricsInterval != cfg.MetricsInterval {
		t.Errorf("GPUMetricsInterval = %v, want %v (same as MetricsInterval)", cfg.GPUMetricsInterval, cfg.MetricsInterval)
	}
}

func TestLoad_AllEnvVars(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUBEADAPT_API_KEY", "my-api-key")
	t.Setenv("KUBEADAPT_CLUSTER_ID", "cluster-123")
	t.Setenv("KUBEADAPT_CLUSTER_NAME", "prod-cluster")
	t.Setenv("KUBEADAPT_BACKEND_URL", "https://custom.example.com")
	t.Setenv("KUBEADAPT_SNAPSHOT_INTERVAL", "120s")
	t.Setenv("KUBEADAPT_METRICS_INTERVAL", "30s")
	t.Setenv("KUBEADAPT_INFORMER_RESYNC", "600s")
	t.Setenv("KUBEADAPT_COMPRESSION_LEVEL", "2")
	t.Setenv("KUBEADAPT_MAX_RETRIES", "10")
	t.Setenv("KUBEADAPT_REQUEST_TIMEOUT", "45s")
	t.Setenv("KUBEADAPT_BUFFER_MAX_BYTES", "104857600")
	t.Setenv("KUBEADAPT_HEALTH_PORT", "9090")

	cfg := Load()

	if cfg.APIKey != "my-api-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "my-api-key")
	}
	if cfg.ClusterID != "cluster-123" {
		t.Errorf("ClusterID = %q, want %q", cfg.ClusterID, "cluster-123")
	}
	if cfg.ClusterName != "prod-cluster" {
		t.Errorf("ClusterName = %q, want %q", cfg.ClusterName, "prod-cluster")
	}
	if cfg.BackendURL != "https://custom.example.com" {
		t.Errorf("BackendURL = %q, want %q", cfg.BackendURL, "https://custom.example.com")
	}
	if cfg.SnapshotInterval != 120*time.Second {
		t.Errorf("SnapshotInterval = %v, want 120s", cfg.SnapshotInterval)
	}
	if cfg.MetricsInterval != 30*time.Second {
		t.Errorf("MetricsInterval = %v, want 30s", cfg.MetricsInterval)
	}
	if cfg.InformerResyncPeriod != 600*time.Second {
		t.Errorf("InformerResyncPeriod = %v, want 600s", cfg.InformerResyncPeriod)
	}
	if cfg.CompressionLevel != 2 {
		t.Errorf("CompressionLevel = %d, want 2", cfg.CompressionLevel)
	}
	if cfg.MaxRetries != 10 {
		t.Errorf("MaxRetries = %d, want 10", cfg.MaxRetries)
	}
	if cfg.RequestTimeout != 45*time.Second {
		t.Errorf("RequestTimeout = %v, want 45s", cfg.RequestTimeout)
	}
	if cfg.BufferMaxBytes != 104857600 {
		t.Errorf("BufferMaxBytes = %d, want 104857600", cfg.BufferMaxBytes)
	}
	if cfg.HealthPort != 9090 {
		t.Errorf("HealthPort = %d, want 9090", cfg.HealthPort)
	}
}

func TestLoad_DurationParsing(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUBEADAPT_API_KEY", "test-key")

	// Test with duration string "60s"
	t.Setenv("KUBEADAPT_SNAPSHOT_INTERVAL", "60s")
	cfg := Load()
	if cfg.SnapshotInterval != 60*time.Second {
		t.Errorf("SnapshotInterval with '60s' = %v, want 60s", cfg.SnapshotInterval)
	}

	// Test with plain integer "60" (treated as seconds)
	t.Setenv("KUBEADAPT_SNAPSHOT_INTERVAL", "60")
	cfg = Load()
	if cfg.SnapshotInterval != 60*time.Second {
		t.Errorf("SnapshotInterval with '60' = %v, want 60s", cfg.SnapshotInterval)
	}

	// Test with "2m"
	t.Setenv("KUBEADAPT_METRICS_INTERVAL", "2m")
	cfg = Load()
	if cfg.MetricsInterval != 2*time.Minute {
		t.Errorf("MetricsInterval with '2m' = %v, want 2m", cfg.MetricsInterval)
	}
}

func TestValidate_MissingAPIKey(t *testing.T) {
	cfg := Config{
		APIKey:           "",
		SnapshotInterval: 60 * time.Second,
		MetricsInterval:  60 * time.Second,
		CompressionLevel: 3,
		MaxRetries:       5,
		HealthPort:       8080,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing APIKey, got nil")
	}
}

func TestValidate_BadInterval(t *testing.T) {
	cfg := Config{
		APIKey:           "test-key",
		BackendURL:       "https://api.kubeadapt.io",
		SnapshotInterval: 5 * time.Second, // too low
		MetricsInterval:  60 * time.Second,
		CompressionLevel: 3,
		MaxRetries:       5,
		HealthPort:       8080,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for SnapshotInterval < 10s, got nil")
	}

	cfg.SnapshotInterval = 60 * time.Second
	cfg.MetricsInterval = 5 * time.Second // too low
	err = cfg.Validate()
	if err == nil {
		t.Error("expected error for MetricsInterval < 10s, got nil")
	}
}

func TestValidate_BadCompressionLevel(t *testing.T) {
	base := Config{
		APIKey:           "test-key",
		BackendURL:       "https://api.kubeadapt.io",
		SnapshotInterval: 60 * time.Second,
		MetricsInterval:  60 * time.Second,
		MaxRetries:       5,
		HealthPort:       8080,
	}

	// Level 0 — invalid
	cfg := base
	cfg.CompressionLevel = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for CompressionLevel 0, got nil")
	}

	// Level 5 — invalid
	cfg = base
	cfg.CompressionLevel = 5
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for CompressionLevel 5, got nil")
	}
}

func TestValidate_Valid(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUBEADAPT_API_KEY", "valid-key")

	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

func TestValidate_HTTPSRequired(t *testing.T) {
	cfg := Config{
		APIKey:           "test-key",
		BackendURL:       "http://insecure.example.com",
		SnapshotInterval: 60 * time.Second,
		MetricsInterval:  60 * time.Second,
		CompressionLevel: 3,
		MaxRetries:       5,
		HealthPort:       8080,
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for http:// BackendURL without AllowInsecure")
	}

	// With AllowInsecure, http:// should be allowed.
	cfg.AllowInsecure = true
	err = cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error with AllowInsecure=true, got: %v", err)
	}
}

func TestValidate_EmptyBackendURL(t *testing.T) {
	cfg := Config{
		APIKey:           "test-key",
		BackendURL:       "",
		SnapshotInterval: 60 * time.Second,
		MetricsInterval:  60 * time.Second,
		CompressionLevel: 3,
		MaxRetries:       5,
		HealthPort:       8080,
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty BackendURL")
	}
}

func TestLoad_SecurityDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUBEADAPT_API_KEY", "test-key")

	cfg := Load()
	if cfg.AllowInsecure {
		t.Error("AllowInsecure should default to false")
	}
	if cfg.DebugEndpoints {
		t.Error("DebugEndpoints should default to false")
	}
}

func TestLoad_GPUConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUBEADAPT_API_KEY", "test-key")
	t.Setenv("KUBEADAPT_GPU_METRICS_ENABLED", "false")
	t.Setenv("KUBEADAPT_DCGM_PORT", "9401")
	t.Setenv("KUBEADAPT_DCGM_NAMESPACE", "gpu-operator")
	t.Setenv("KUBEADAPT_GPU_METRICS_INTERVAL", "15s")

	cfg := Load()

	if cfg.GPUMetricsEnabled {
		t.Error("GPUMetricsEnabled = true, want false")
	}
	if cfg.DCGMExporterPort != 9401 {
		t.Errorf("DCGMExporterPort = %d, want 9401", cfg.DCGMExporterPort)
	}
	if cfg.DCGMExporterNamespace != "gpu-operator" {
		t.Errorf("DCGMExporterNamespace = %q, want %q", cfg.DCGMExporterNamespace, "gpu-operator")
	}
	if cfg.GPUMetricsInterval != 15*time.Second {
		t.Errorf("GPUMetricsInterval = %v, want 15s", cfg.GPUMetricsInterval)
	}
}

func TestLoad_GPUMetricsIntervalDefaultsToMetricsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("KUBEADAPT_API_KEY", "test-key")
	t.Setenv("KUBEADAPT_METRICS_INTERVAL", "45s")

	cfg := Load()

	if cfg.GPUMetricsInterval != 45*time.Second {
		t.Errorf("GPUMetricsInterval = %v, want 45s (same as MetricsInterval)", cfg.GPUMetricsInterval)
	}
}

func TestLoad_GPUMetricsEnabledParsing(t *testing.T) {
	tests := []struct {
		envVal string
		want   bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run("val="+tt.envVal, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("KUBEADAPT_API_KEY", "test-key")
			if tt.envVal != "" {
				t.Setenv("KUBEADAPT_GPU_METRICS_ENABLED", tt.envVal)
			}
			cfg := Load()
			if cfg.GPUMetricsEnabled != tt.want {
				t.Errorf("GPUMetricsEnabled = %v, want %v for env=%q", cfg.GPUMetricsEnabled, tt.want, tt.envVal)
			}
		})
	}
}
