package convert

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// FilterAnnotations returns a filtered copy of the annotations map.
// It skips kubectl.kubernetes.io/last-applied-configuration entirely
// and truncates any value longer than 1024 bytes.
func FilterAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		return nil
	}
	filtered := make(map[string]string, len(annotations))
	for k, v := range annotations {
		if k == "kubectl.kubernetes.io/last-applied-configuration" {
			continue
		}
		if len(v) > 1024 {
			filtered[k] = v[:1024]
			continue
		}
		filtered[k] = v
	}
	return filtered
}

// ParseQuantity converts a K8s resource.Quantity to float64.
// For CPU quantities (e.g. "500m"), returns cores as float64.
// For memory/storage quantities, returns bytes as float64.
func ParseQuantity(q resource.Quantity) float64 {
	// AsApproximateFloat64 handles both milli-values and large values correctly.
	return q.AsApproximateFloat64()
}

// ParseQuantityString parses a quantity string (e.g. "500m", "2Gi") to float64.
// Returns 0 on parse error.
func ParseQuantityString(s string) float64 {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return 0
	}
	return ParseQuantity(q)
}
