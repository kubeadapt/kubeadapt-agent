package discovery

import (
	"context"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// newFakeDiscovery creates a FakeDiscovery with the given API resource lists.
func newFakeDiscovery(resources []*metav1.APIResourceList) *fakediscovery.FakeDiscovery {
	fake := &clienttesting.Fake{}
	fake.Resources = resources
	return &fakediscovery.FakeDiscovery{Fake: fake}
}

func TestDetect_MetricsServerExists(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Spec:       v1.NodeSpec{ProviderID: "aws:///us-east-1a/i-abc123"},
	}
	client := fakeclientset.NewSimpleClientset(node)

	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "metrics.k8s.io/v1beta1"},
	})

	caps, err := Detect(context.Background(), client, disco)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !caps.MetricsServer {
		t.Error("expected MetricsServer=true")
	}
	if caps.Provider != "aws" {
		t.Errorf("Provider = %q, want %q", caps.Provider, "aws")
	}
}

func TestDetect_NoMetricsAPI(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()

	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "apps/v1"},
	})

	caps, err := Detect(context.Background(), client, disco)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if caps.MetricsServer {
		t.Error("expected MetricsServer=false when metrics.k8s.io not present")
	}
}

func TestDetect_VPAExists(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()

	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "autoscaling.k8s.io/v1"},
	})

	caps, err := Detect(context.Background(), client, disco)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !caps.VPA {
		t.Error("expected VPA=true when autoscaling.k8s.io present")
	}
}

func TestDetect_KarpenterExists(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()

	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "karpenter.sh/v1beta1"},
	})

	caps, err := Detect(context.Background(), client, disco)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !caps.Karpenter {
		t.Error("expected Karpenter=true when karpenter.sh present")
	}
}

func TestDetect_AllCapabilities(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Spec:       v1.NodeSpec{ProviderID: "gce://my-project/us-central1-a/instance-1"},
	}
	client := fakeclientset.NewSimpleClientset(node)

	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "metrics.k8s.io/v1beta1"},
		{GroupVersion: "autoscaling.k8s.io/v1"},
		{GroupVersion: "karpenter.sh/v1beta1"},
	})

	caps, err := Detect(context.Background(), client, disco)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !caps.MetricsServer {
		t.Error("expected MetricsServer=true")
	}
	if !caps.VPA {
		t.Error("expected VPA=true")
	}
	if !caps.Karpenter {
		t.Error("expected Karpenter=true")
	}
	if caps.Provider != "gcp" {
		t.Errorf("Provider = %q, want %q", caps.Provider, "gcp")
	}
}

func TestDetect_NoCapabilities(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()

	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "apps/v1"},
	})

	caps, err := Detect(context.Background(), client, disco)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if caps.MetricsServer || caps.VPA || caps.Karpenter {
		t.Error("expected all capabilities to be false with no matching API groups")
	}
	if caps.Provider != "unknown" {
		t.Errorf("Provider = %q, want %q", caps.Provider, "unknown")
	}
}

func TestHasAPIGroup_Found(t *testing.T) {
	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "metrics.k8s.io/v1beta1"},
	})

	found, err := HasAPIGroup(disco, "metrics.k8s.io")
	if err != nil {
		t.Fatalf("HasAPIGroup() error = %v", err)
	}
	if !found {
		t.Error("expected API group metrics.k8s.io to be found")
	}
}

func TestHasAPIGroup_NotFound(t *testing.T) {
	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "apps/v1"},
	})

	found, err := HasAPIGroup(disco, "metrics.k8s.io")
	if err != nil {
		t.Fatalf("HasAPIGroup() error = %v", err)
	}
	if found {
		t.Error("expected API group metrics.k8s.io to NOT be found")
	}
}

// Ensure Detect works when node list fails (e.g., RBAC).
func TestDetect_NodeListError_GracefulDegradation(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	// Add a reactor that forces node list to fail.
	client.PrependReactor("list", "nodes", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("forbidden: nodes is forbidden")
	})

	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "metrics.k8s.io/v1beta1"},
	})

	caps, err := Detect(context.Background(), client, disco)
	if err != nil {
		t.Fatalf("Detect() should not fail on node list error, got: %v", err)
	}
	// Provider detection fails gracefully.
	if caps.Provider != "unknown" {
		t.Errorf("Provider = %q, want %q when node list fails", caps.Provider, "unknown")
	}
	// Other capabilities still detected.
	if !caps.MetricsServer {
		t.Error("expected MetricsServer=true even when node list fails")
	}
}
