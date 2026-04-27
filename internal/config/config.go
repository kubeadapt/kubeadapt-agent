package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all agent configuration values.
type Config struct {
	APIKey               string
	BackendURL           string
	SnapshotInterval     time.Duration
	MetricsInterval      time.Duration
	InformerResyncPeriod time.Duration
	InformerSyncTimeout  time.Duration
	CompressionLevel     int
	MaxRetries           int
	RequestTimeout       time.Duration
	BufferMaxBytes       int64
	HealthPort           int
	AgentVersion         string
	KubernetesVersion    string

	// MaxCompressedBodyBytes mirrors the server's MAX_COMPRESSED_BODY_SIZE; oversize
	// snapshots fail locally with ErrPayloadTooLarge. Both sides must agree.
	MaxCompressedBodyBytes int64

	// Kubernetes pod metadata (injected via Helm downward API)
	ChartVersion    string // KUBEADAPT_CHART_VERSION
	HelmReleaseName string // HELM_RELEASE_NAME
	PodName         string // POD_NAME
	PodNamespace    string // POD_NAMESPACE
	NodeName        string // NODE_NAME

	// Security
	AllowInsecure  bool // KUBEADAPT_ALLOW_INSECURE, default: false — allows http:// BackendURL
	DebugEndpoints bool // KUBEADAPT_DEBUG_ENDPOINTS, default: false — enables pprof/debug on health port

	// GPU monitoring
	GPUMetricsEnabled     bool          // KUBEADAPT_GPU_METRICS_ENABLED, default: true
	DCGMExporterPort      int           // KUBEADAPT_DCGM_PORT, default: 9400
	DCGMExporterNamespace string        // KUBEADAPT_DCGM_NAMESPACE, default: "" (auto-detect)
	DCGMExporterEndpoints []string      // KUBEADAPT_DCGM_ENDPOINTS, comma-separated IPs/hosts override (for local dev)
	GPUMetricsInterval    time.Duration // KUBEADAPT_GPU_METRICS_INTERVAL, default: MetricsInterval
}

// Load reads configuration from environment variables and returns a Config
// with defaults applied for any unset values.
func Load() Config {
	cfg := Config{
		APIKey:               envOrFallback("KUBEADAPT_API_KEY", "KUBEADAPT_AGENT_TOKEN"),
		BackendURL:           envOrFallbackOrDefault("KUBEADAPT_BACKEND_URL", "KUBEADAPT_BACKEND_API_ENDPOINT", "https://agent.kubeadapt.io"),
		SnapshotInterval:     parseDuration("KUBEADAPT_SNAPSHOT_INTERVAL", 60*time.Second),
		MetricsInterval:      parseDuration("KUBEADAPT_METRICS_INTERVAL", 60*time.Second),
		InformerResyncPeriod: parseDuration("KUBEADAPT_INFORMER_RESYNC", 300*time.Second),
		InformerSyncTimeout:  parseDuration("KUBEADAPT_INFORMER_SYNC_TIMEOUT", 5*time.Minute),
		CompressionLevel:     parseInt("KUBEADAPT_COMPRESSION_LEVEL", 3),
		MaxRetries:           parseInt("KUBEADAPT_MAX_RETRIES", 5),
		RequestTimeout:       parseDuration("KUBEADAPT_REQUEST_TIMEOUT", 30*time.Second),
		BufferMaxBytes:       parseInt64("KUBEADAPT_BUFFER_MAX_BYTES", 52428800),
		HealthPort:           parseInt("KUBEADAPT_HEALTH_PORT", 8080),
		// Must match server's MAX_COMPRESSED_BODY_SIZE or the smaller value wins.
		MaxCompressedBodyBytes: parseInt64("KUBEADAPT_MAX_COMPRESSED_BODY_BYTES", 52428800),
	}

	cfg.ChartVersion = os.Getenv("KUBEADAPT_CHART_VERSION")
	cfg.HelmReleaseName = os.Getenv("HELM_RELEASE_NAME")
	cfg.PodName = os.Getenv("POD_NAME")
	cfg.PodNamespace = os.Getenv("POD_NAMESPACE")
	cfg.NodeName = os.Getenv("NODE_NAME")

	cfg.AllowInsecure = parseBool("KUBEADAPT_ALLOW_INSECURE", false)
	cfg.DebugEndpoints = parseBool("KUBEADAPT_DEBUG_ENDPOINTS", false)

	cfg.GPUMetricsEnabled = parseBool("KUBEADAPT_GPU_METRICS_ENABLED", true)
	cfg.DCGMExporterPort = parseInt("KUBEADAPT_DCGM_PORT", 9400)
	cfg.DCGMExporterNamespace = envOrDefault("KUBEADAPT_DCGM_NAMESPACE", "")
	cfg.DCGMExporterEndpoints = parseStringSlice("KUBEADAPT_DCGM_ENDPOINTS")
	cfg.GPUMetricsInterval = parseDuration("KUBEADAPT_GPU_METRICS_INTERVAL", cfg.MetricsInterval)

	return cfg
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// envOrFallback tries the primary key first, then falls back to the fallback key.
// Returns empty string if neither is set.
func envOrFallback(primary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	return os.Getenv(fallback)
}

// envOrFallbackOrDefault tries the primary key, then fallback key, then default value.
func envOrFallbackOrDefault(primary, fallback, defaultVal string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	if v := os.Getenv(fallback); v != "" {
		return v
	}
	return defaultVal
}

// parseDuration tries time.ParseDuration first, then falls back to treating
// the value as integer seconds.
func parseDuration(key string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}

	d, err := time.ParseDuration(v)
	if err == nil {
		return d
	}

	// Fallback: treat as integer seconds
	secs, err := strconv.Atoi(v)
	if err == nil {
		return time.Duration(secs) * time.Second
	}

	return defaultVal
}

func parseBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

func parseInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func parseStringSlice(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	var result []string
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func parseInt64(key string, defaultVal int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return n
}

// ResolveAgentVersion determines the agent version using this precedence:
//  1. Build-time ldflags value (main.Version)
//  2. KUBEADAPT_AGENT_VERSION env var (Helm chart / runtime override)
//  3. "dev" fallback
func ResolveAgentVersion(buildVersion string) string {
	if buildVersion != "" && buildVersion != "dev" {
		return buildVersion
	}
	if v := os.Getenv("KUBEADAPT_AGENT_VERSION"); v != "" {
		return v
	}
	if buildVersion != "" {
		return buildVersion
	}
	return "dev"
}
