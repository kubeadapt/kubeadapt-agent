package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

func newVPATestEnv(t *testing.T) (*dynamicfake.FakeDynamicClient, *store.Store, *observability.Metrics, context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "autoscaling.k8s.io", Version: "v1"}
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "VerticalPodAutoscalerList"},
		&unstructured.UnstructuredList{},
	)

	client := dynamicfake.NewSimpleDynamicClient(scheme)
	s := store.NewStore()
	m := observability.NewMetrics()
	return client, s, m, ctx, cancel
}

func TestVPACollector_Name(t *testing.T) {
	client, s, m, _, _ := newVPATestEnv(t)
	c := NewVPACollector(client, s, m, testResyncPeriod)
	assert.Equal(t, "vpas", c.Name())
}

func TestVPACollector_AddUpdateDelete(t *testing.T) {
	client, s, m, ctx, cancel := newVPATestEnv(t)
	defer cancel()

	c := NewVPACollector(client, s, m, testResyncPeriod)
	err := c.Start(ctx)
	require.NoError(t, err)
	err = c.WaitForSync(ctx)
	require.NoError(t, err)
	t.Cleanup(c.Stop)

	gvr := schema.GroupVersionResource{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"}

	// --- Add ---
	vpa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling.k8s.io/v1",
			"kind":       "VerticalPodAutoscaler",
			"metadata": map[string]interface{}{
				"name":      "web-vpa",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"targetRef": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "web",
				},
				"updatePolicy": map[string]interface{}{
					"updateMode": "Auto",
				},
			},
		},
	}
	_, err = client.Resource(gvr).Namespace("default").Create(ctx, vpa, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return s.VPAs.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := s.VPAs.Get("default/web-vpa")
	require.True(t, ok)
	assert.Equal(t, "web-vpa", info.Name)
	assert.Equal(t, "Deployment", info.TargetKind)
	assert.Equal(t, "Auto", info.UpdateMode)

	// --- Delete ---
	err = client.Resource(gvr).Namespace("default").Delete(ctx, "web-vpa", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return s.VPAs.Len() == 0
	}, waitTimeout, pollInterval)
}
