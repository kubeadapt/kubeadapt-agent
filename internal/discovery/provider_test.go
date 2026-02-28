package discovery

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetectProvider_AWS(t *testing.T) {
	nodes := []*v1.Node{{
		Spec: v1.NodeSpec{
			ProviderID: "aws:///us-east-1a/i-1234567890abcdef0",
		},
	}}
	got := DetectProvider(nodes)
	if got != "aws" {
		t.Errorf("DetectProvider = %q, want %q", got, "aws")
	}
}

func TestDetectProvider_GCE(t *testing.T) {
	nodes := []*v1.Node{{
		Spec: v1.NodeSpec{
			ProviderID: "gce://my-project/us-central1-a/my-instance",
		},
	}}
	got := DetectProvider(nodes)
	if got != "gcp" {
		t.Errorf("DetectProvider = %q, want %q", got, "gcp")
	}
}

func TestDetectProvider_Azure(t *testing.T) {
	nodes := []*v1.Node{{
		Spec: v1.NodeSpec{
			ProviderID: "azure:///subscriptions/sub-123/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm-name",
		},
	}}
	got := DetectProvider(nodes)
	if got != "azure" {
		t.Errorf("DetectProvider = %q, want %q", got, "azure")
	}
}

func TestDetectProvider_NoNodes(t *testing.T) {
	got := DetectProvider(nil)
	if got != "unknown" {
		t.Errorf("DetectProvider(nil) = %q, want %q", got, "unknown")
	}

	got = DetectProvider([]*v1.Node{})
	if got != "unknown" {
		t.Errorf("DetectProvider([]) = %q, want %q", got, "unknown")
	}
}

func TestDetectProvider_UnknownProviderID(t *testing.T) {
	nodes := []*v1.Node{{
		Spec: v1.NodeSpec{
			ProviderID: "someprovider://instance-123",
		},
	}}
	got := DetectProvider(nodes)
	if got != "unknown" {
		t.Errorf("DetectProvider = %q, want %q", got, "unknown")
	}
}

func TestDetectProvider_FallbackToLabels_AWS(t *testing.T) {
	nodes := []*v1.Node{{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"eks.amazonaws.com/nodegroup": "my-group",
			},
		},
	}}
	got := DetectProvider(nodes)
	if got != "aws" {
		t.Errorf("DetectProvider (label fallback) = %q, want %q", got, "aws")
	}
}

func TestDetectProvider_FallbackToLabels_GCP(t *testing.T) {
	nodes := []*v1.Node{{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"cloud.google.com/gke-nodepool": "default-pool",
			},
		},
	}}
	got := DetectProvider(nodes)
	if got != "gcp" {
		t.Errorf("DetectProvider (label fallback) = %q, want %q", got, "gcp")
	}
}

func TestDetectProvider_FallbackToLabels_Azure(t *testing.T) {
	nodes := []*v1.Node{{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"kubernetes.azure.com/agentpool": "nodepool1",
			},
		},
	}}
	got := DetectProvider(nodes)
	if got != "azure" {
		t.Errorf("DetectProvider (label fallback) = %q, want %q", got, "azure")
	}
}

func TestDetectProvider_ProviderIDTakesPriority(t *testing.T) {
	// providerID says AWS, labels say GCP — providerID wins.
	nodes := []*v1.Node{{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"cloud.google.com/gke-nodepool": "pool-1",
			},
		},
		Spec: v1.NodeSpec{
			ProviderID: "aws:///us-east-1a/i-abc123",
		},
	}}
	got := DetectProvider(nodes)
	if got != "aws" {
		t.Errorf("DetectProvider (providerID priority) = %q, want %q", got, "aws")
	}
}
