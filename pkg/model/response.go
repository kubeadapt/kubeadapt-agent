package model

// SnapshotResponse is returned by backend on successful snapshot ingestion.
type SnapshotResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ClusterID   string `json:"cluster_id"`
	ReceivedAt  int64  `json:"received_at"`
	ProcessedAt int64  `json:"processed_at"`

	Quota      QuotaStatus `json:"quota"`
	Directives Directives  `json:"directives"`
	Stats      IngestStats `json:"stats,omitempty"`
}

// SnapshotErrorResponse is returned on rejection (4xx errors).
type SnapshotErrorResponse struct {
	Success           bool         `json:"success"`
	Error             string       `json:"error"`
	Message           string       `json:"message"`
	Quota             *QuotaStatus `json:"quota,omitempty"`
	RetryAfterSeconds *int         `json:"retry_after_seconds,omitempty"`
}

// QuotaStatus represents the organization's quota state.
type QuotaStatus struct {
	PlanType        string  `json:"plan_type"`
	CPULimit        float64 `json:"cpu_limit"`
	CurrentCPUUsage float64 `json:"current_cpu_usage"`
	RemainingCPU    float64 `json:"remaining_cpu"`
	IsWithinQuota   bool    `json:"is_within_quota"`

	ClusterCPU float64 `json:"cluster_cpu"`

	GracePeriodActive bool   `json:"grace_period_active,omitempty"`
	GracePeriodEndsAt *int64 `json:"grace_period_ends_at,omitempty"`
	MetricsBlocked    bool   `json:"metrics_blocked,omitempty"`
}

// Directives tell the agent what to do next.
type Directives struct {
	NextSnapshotInSeconds int  `json:"next_snapshot_in_seconds"`
	RetryAfterSeconds     *int `json:"retry_after_seconds,omitempty"`
	CollectVPAs           bool `json:"collect_vpas"`
	CollectKarpenter      bool `json:"collect_karpenter"`
}

// IngestStats returned after successful processing.
type IngestStats struct {
	NodesProcessed     int   `json:"nodes_processed"`
	PodsProcessed      int   `json:"pods_processed"`
	WorkloadsProcessed int   `json:"workloads_processed"`
	ProcessingTimeMs   int64 `json:"processing_time_ms"`
}
