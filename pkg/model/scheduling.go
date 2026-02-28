package model

// NamespaceInfo represents a Kubernetes Namespace.
type NamespaceInfo struct {
	Name              string            `json:"name"`
	Phase             string            `json:"phase"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`
}

// PriorityClassInfo represents a Kubernetes PriorityClass.
type PriorityClassInfo struct {
	Name             string `json:"name"`
	Value            int32  `json:"value"`
	GlobalDefault    bool   `json:"global_default"`
	PreemptionPolicy string `json:"preemption_policy"`
	Description      string `json:"description"`
}

// LimitRangeInfo represents a Kubernetes LimitRange.
type LimitRangeInfo struct {
	Name      string               `json:"name"`
	Namespace string               `json:"namespace"`
	Limits    []LimitRangeItemInfo `json:"limits"`
}

// LimitRangeItemInfo represents a single limit within a LimitRange.
type LimitRangeItemInfo struct {
	Type                 string            `json:"type"`
	Default              map[string]string `json:"default"`
	DefaultRequest       map[string]string `json:"default_request"`
	Max                  map[string]string `json:"max"`
	Min                  map[string]string `json:"min"`
	MaxLimitRequestRatio map[string]string `json:"max_limit_request_ratio"`
}

// ResourceQuotaInfo represents a Kubernetes ResourceQuota.
type ResourceQuotaInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Hard      map[string]string `json:"hard"`
	Used      map[string]string `json:"used"`
	Labels    map[string]string `json:"labels"`
}
