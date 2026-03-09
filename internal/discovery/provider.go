package discovery

import (
	"strings"

	v1 "k8s.io/api/core/v1"
)

// Well-known provider-specific node labels used as confirmation signals.
const (
	labelEKSNodeGroup    = "eks.amazonaws.com/nodegroup"
	labelEKSCapacity     = "eks.amazonaws.com/capacityType"
	labelGKENodePool     = "cloud.google.com/gke-nodepool"
	labelAKSNodepoolName = "kubernetes.azure.com/agentpool"
)

// DetectProvider determines the cloud provider from node metadata.
// It inspects spec.providerID prefixes and provider-specific labels on the
// first available node. Pure function — no API calls.
//
// Returns "aws", "gcp", "azure", or "unknown".
func DetectProvider(nodes []*v1.Node) string {
	if len(nodes) == 0 {
		return "unknown"
	}

	node := nodes[0]

	// Phase 1: Check providerID prefix (most reliable).
	if provider := providerFromID(node.Spec.ProviderID); provider != "" {
		return provider
	}

	// Phase 2: Fall back to provider-specific labels.
	if provider := providerFromLabels(node.Labels); provider != "" {
		return provider
	}

	return "unknown"
}

// providerFromID extracts the provider name from a node's spec.providerID.
func providerFromID(providerID string) string {
	switch {
	case strings.HasPrefix(providerID, "aws://"):
		return "aws"
	case strings.HasPrefix(providerID, "gce://"):
		return "gcp"
	case strings.HasPrefix(providerID, "azure://"):
		return "azure"
	default:
		return ""
	}
}

// providerFromLabels checks for provider-specific labels as a fallback signal.
func providerFromLabels(labels map[string]string) string {
	if labels == nil {
		return ""
	}

	if _, ok := labels[labelEKSNodeGroup]; ok {
		return "aws"
	}
	if _, ok := labels[labelEKSCapacity]; ok {
		return "aws"
	}
	if _, ok := labels[labelGKENodePool]; ok {
		return "gcp"
	}
	if _, ok := labels[labelAKSNodepoolName]; ok {
		return "azure"
	}

	return ""
}
