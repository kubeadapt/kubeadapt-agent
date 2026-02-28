package convert

import (
	"strings"
)

// ParseProviderID extracts cloud instance metadata from a Kubernetes node's
// spec.providerID field. Pure string parsing — no cloud API calls.
//
// Supported formats:
//   - AWS:   aws:///us-east-1a/i-1234567890abcdef0
//   - GCE:   gce://project-id/us-central1-a/instance-name
//   - Azure: azure:///subscriptions/.../virtualMachines/vm-name
func ParseProviderID(providerID string) (instanceID, az, region string) {
	if providerID == "" {
		return "", "", ""
	}

	switch {
	case strings.HasPrefix(providerID, "aws://"):
		return parseAWSProviderID(providerID)
	case strings.HasPrefix(providerID, "gce://"):
		return parseGCEProviderID(providerID)
	case strings.HasPrefix(providerID, "azure://"):
		return parseAzureProviderID(providerID)
	default:
		return providerID, "", ""
	}
}

// parseAWSProviderID handles: aws:///us-east-1a/i-1234567890abcdef0
// The triple slash means host is empty: scheme://host/path → aws:///az/instance
func parseAWSProviderID(providerID string) (instanceID, az, region string) {
	// Strip "aws:///" prefix
	trimmed := strings.TrimPrefix(providerID, "aws:///")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return providerID, "", ""
	}
	az = parts[0]
	instanceID = parts[1]
	// Region = AZ minus last character (e.g. us-east-1a → us-east-1)
	if len(az) > 0 {
		region = az[:len(az)-1]
	}
	return instanceID, az, region
}

// parseGCEProviderID handles: gce://project-id/us-central1-a/instance-name
func parseGCEProviderID(providerID string) (instanceID, az, region string) {
	// Strip "gce://" prefix
	trimmed := strings.TrimPrefix(providerID, "gce://")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return providerID, "", ""
	}
	// parts[0] = project, parts[1] = zone, parts[2] = instance
	az = parts[1]
	instanceID = parts[2]
	// Region = strip last "-X" segment from az (e.g. us-central1-a → us-central1)
	if idx := strings.LastIndex(az, "-"); idx > 0 {
		region = az[:idx]
	}
	return instanceID, az, region
}

// parseAzureProviderID handles: azure:///subscriptions/.../virtualMachines/vm-name
// AZ and region come from labels instead of providerID for Azure.
func parseAzureProviderID(providerID string) (instanceID, az, region string) {
	// Strip "azure:///" prefix
	trimmed := strings.TrimPrefix(providerID, "azure:///")
	// Instance ID = last path segment
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		instanceID = trimmed[idx+1:]
	} else {
		instanceID = trimmed
	}
	// Azure doesn't encode az/region in providerID
	return instanceID, "", ""
}

// ExtractCapacityType determines the capacity type (on-demand, spot, fargate)
// from node labels. Returns lowercase normalized values.
// Check order: EKS compute-type (fargate) → EKS capacityType → Karpenter → generic lifecycle
func ExtractCapacityType(labels map[string]string) string {
	if labels == nil {
		return ""
	}

	// Check for Fargate first
	if v, ok := labels["eks.amazonaws.com/compute-type"]; ok && v == "fargate" {
		return "fargate"
	}

	// EKS managed node groups
	if v, ok := labels["eks.amazonaws.com/capacityType"]; ok {
		switch strings.ToUpper(v) {
		case "ON_DEMAND":
			return "on-demand"
		case "SPOT":
			return "spot"
		default:
			return strings.ToLower(v)
		}
	}

	// Karpenter
	if v, ok := labels["karpenter.sh/capacity-type"]; ok {
		return strings.ToLower(v)
	}

	// Generic lifecycle label (some providers)
	if v, ok := labels["node.kubernetes.io/lifecycle"]; ok {
		switch strings.ToLower(v) {
		case "spot":
			return "spot"
		case "normal":
			return "on-demand"
		default:
			return strings.ToLower(v)
		}
	}

	return ""
}

// ExtractNodeGroup determines the node group name from node labels.
// Check order: EKS nodegroup → Karpenter nodepool
func ExtractNodeGroup(labels map[string]string) string {
	if labels == nil {
		return ""
	}

	if v, ok := labels["eks.amazonaws.com/nodegroup"]; ok {
		return v
	}
	if v, ok := labels["karpenter.sh/nodepool"]; ok {
		return v
	}
	return ""
}
