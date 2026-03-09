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

func newNodePoolTestEnv(t *testing.T) (*dynamicfake.FakeDynamicClient, *store.Store, *observability.Metrics, context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1"}
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "NodePoolList"},
		&unstructured.UnstructuredList{},
	)

	client := dynamicfake.NewSimpleDynamicClient(scheme)
	s := store.NewStore()
	m := observability.NewMetrics()
	return client, s, m, ctx, cancel
}

func TestNodePoolCollector_Name(t *testing.T) {
	client, s, m, _, _ := newNodePoolTestEnv(t)
	c := NewNodePoolCollector(client, s, m, testResyncPeriod)
	assert.Equal(t, "nodepools", c.Name())
}

func TestNodePoolCollector_AddUpdateDelete(t *testing.T) {
	client, s, m, ctx, cancel := newNodePoolTestEnv(t)
	defer cancel()

	c := NewNodePoolCollector(client, s, m, testResyncPeriod)
	err := c.Start(ctx)
	require.NoError(t, err)
	err = c.WaitForSync(ctx)
	require.NoError(t, err)
	t.Cleanup(c.Stop)

	gvr := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodepools"}

	// --- Add ---
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "karpenter.sh/v1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":   "default-pool",
				"labels": map[string]interface{}{"team": "platform"},
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"nodeClassRef": map[string]interface{}{
							"name": "default",
						},
						"requirements": []interface{}{
							map[string]interface{}{
								"key":      "kubernetes.io/arch",
								"operator": "In",
								"values":   []interface{}{"amd64"},
							},
						},
					},
				},
			},
		},
	}
	_, err = client.Resource(gvr).Create(ctx, np, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return s.NodePools.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := s.NodePools.Get("default-pool")
	require.True(t, ok)
	assert.Equal(t, "default-pool", info.Name)
	assert.Equal(t, "default", info.NodeClassName)
	require.Len(t, info.Requirements, 1)
	assert.Equal(t, "kubernetes.io/arch", info.Requirements[0].Key)

	// --- Delete ---
	err = client.Resource(gvr).Delete(ctx, "default-pool", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return s.NodePools.Len() == 0
	}, waitTimeout, pollInterval)
}
