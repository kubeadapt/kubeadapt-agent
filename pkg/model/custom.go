package model

// CustomWorkloadInfo represents a CRD-based workload that owns pods.
type CustomWorkloadInfo struct {
	APIVersion    string `json:"api_version"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	Replicas      *int32 `json:"replicas,omitempty"`
	ReadyReplicas *int32 `json:"ready_replicas,omitempty"`

	Status map[string]interface{} `json:"status"`

	PodCount           int      `json:"pod_count"`
	TotalCPURequest    float64  `json:"total_cpu_request"`
	TotalMemoryRequest int64    `json:"total_memory_request"`
	TotalCPUUsage      *float64 `json:"total_cpu_usage,omitempty"`
	TotalMemoryUsage   *int64   `json:"total_memory_usage,omitempty"`

	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`
}

// NodePoolInfo represents a Karpenter NodePool.
type NodePoolInfo struct {
	Name              string                    `json:"name"`
	MinReplicas       *int                      `json:"min_replicas,omitempty"`
	MaxReplicas       *int                      `json:"max_replicas,omitempty"`
	NodeClassName     string                    `json:"node_class_name"`
	Labels            map[string]string         `json:"labels"`
	Annotations       map[string]string         `json:"annotations"`
	Taints            []TaintInfo               `json:"taints"`
	Requirements      []NodeSelectorRequirement `json:"requirements"`
	CreationTimestamp int64                     `json:"creation_timestamp"`
}

// NodeSelectorRequirement is a selector requirement with key, operator, and values.
type NodeSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}
