package model

// HPAInfo represents a Kubernetes HorizontalPodAutoscaler (v2).
type HPAInfo struct {
	Name      string `json:"name"`
	UID       string `json:"uid"`
	Namespace string `json:"namespace"`

	TargetKind       string `json:"target_kind"`
	TargetName       string `json:"target_name"`
	TargetAPIVersion string `json:"target_api_version"`

	MinReplicas     *int32 `json:"min_replicas,omitempty"`
	MaxReplicas     int32  `json:"max_replicas"`
	CurrentReplicas int32  `json:"current_replicas"`
	DesiredReplicas int32  `json:"desired_replicas"`

	Metrics        []HPAMetricInfo        `json:"metrics"`
	CurrentMetrics []HPACurrentMetricInfo `json:"current_metrics"`

	ScaleUpBehavior   *HPAScalingBehavior `json:"scale_up_behavior,omitempty"`
	ScaleDownBehavior *HPAScalingBehavior `json:"scale_down_behavior,omitempty"`

	Conditions        []HPAConditionInfo `json:"conditions"`
	Labels            map[string]string  `json:"labels"`
	Annotations       map[string]string  `json:"annotations"`
	CreationTimestamp int64              `json:"creation_timestamp"`
	LastScaleTime     *int64             `json:"last_scale_time,omitempty"`
}

// HPAMetricInfo describes a metric that the HPA watches.
type HPAMetricInfo struct {
	Type          string `json:"type"`
	ResourceName  string `json:"resource_name"`
	ContainerName string `json:"container_name"`
	MetricName    string `json:"metric_name"`
	TargetType    string `json:"target_type"`
	TargetValue   string `json:"target_value"`
}

// HPACurrentMetricInfo describes the current value of a metric.
type HPACurrentMetricInfo struct {
	Type                string `json:"type"`
	ResourceName        string `json:"resource_name"`
	CurrentValue        string `json:"current_value"`
	CurrentAverageValue string `json:"current_average_value"`
	CurrentUtilization  *int32 `json:"current_utilization,omitempty"`
}

// HPAScalingBehavior defines the scaling behavior for scale up or scale down.
type HPAScalingBehavior struct {
	StabilizationWindowSeconds *int32             `json:"stabilization_window_seconds,omitempty"`
	Policies                   []HPAScalingPolicy `json:"policies"`
	SelectPolicy               string             `json:"select_policy"`
}

// HPAScalingPolicy defines a single scaling policy.
type HPAScalingPolicy struct {
	Type          string `json:"type"`
	Value         int32  `json:"value"`
	PeriodSeconds int32  `json:"period_seconds"`
}

// HPAConditionInfo represents an HPA condition.
type HPAConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// PDBInfo represents a Kubernetes PodDisruptionBudget.
type PDBInfo struct {
	Name      string `json:"name"`
	UID       string `json:"uid"`
	Namespace string `json:"namespace"`

	MatchLabels      map[string]string          `json:"match_labels"`
	MatchExpressions []LabelSelectorRequirement `json:"match_expressions"`

	TargetWorkloads []WorkloadReference `json:"target_workloads"`

	MinAvailable   string `json:"min_available"`
	MaxUnavailable string `json:"max_unavailable"`

	CurrentHealthy     int32 `json:"current_healthy"`
	DesiredHealthy     int32 `json:"desired_healthy"`
	DisruptionsAllowed int32 `json:"disruptions_allowed"`
	ExpectedPods       int32 `json:"expected_pods"`

	Conditions        []PDBConditionInfo `json:"conditions"`
	Labels            map[string]string  `json:"labels"`
	Annotations       map[string]string  `json:"annotations"`
	CreationTimestamp int64              `json:"creation_timestamp"`
}

// LabelSelectorRequirement is a selector requirement with key, operator, and values.
type LabelSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

// WorkloadReference identifies a workload by kind, name, and namespace.
type WorkloadReference struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// PDBConditionInfo represents a PDB condition.
type PDBConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// VPAInfo represents a Kubernetes VerticalPodAutoscaler.
type VPAInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`

	TargetKind       string `json:"target_kind"`
	TargetName       string `json:"target_name"`
	TargetAPIVersion string `json:"target_api_version"`

	UpdateMode string `json:"update_mode"`

	ContainerRecommendations []VPAContainerRecommendation `json:"container_recommendations"`

	Conditions        []VPAConditionInfo `json:"conditions"`
	Labels            map[string]string  `json:"labels"`
	CreationTimestamp int64              `json:"creation_timestamp"`
}

// VPAContainerRecommendation holds resource recommendations for a single container.
type VPAContainerRecommendation struct {
	ContainerName  string         `json:"container_name"`
	LowerBound     ResourceValues `json:"lower_bound"`
	Target         ResourceValues `json:"target"`
	UncappedTarget ResourceValues `json:"uncapped_target"`
	UpperBound     ResourceValues `json:"upper_bound"`
}

// ResourceValues holds CPU and memory resource values.
type ResourceValues struct {
	CPUCores    float64 `json:"cpu_cores"`
	MemoryBytes int64   `json:"memory_bytes"`
}

// VPAConditionInfo represents a VPA condition.
type VPAConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}
