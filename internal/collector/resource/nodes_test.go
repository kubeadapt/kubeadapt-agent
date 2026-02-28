package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewNodeCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "nodes", c.Name())
}

func TestNodeCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewNodeCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	_, err := env.client.CoreV1().Nodes().Create(env.ctx, node, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Nodes.Len() == 1
	}, waitTimeout, pollInterval, "expected 1 node in store after add")

	info, ok := env.store.Nodes.Get("test-node")
	require.True(t, ok)
	assert.Equal(t, "test-node", info.Name)
	assert.True(t, info.Ready)

	// --- Update ---
	node.Spec.Unschedulable = true
	_, err = env.client.CoreV1().Nodes().Update(env.ctx, node, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.Nodes.Get("test-node")
		return info.Unschedulable
	}, waitTimeout, pollInterval, "expected node to be updated as unschedulable")

	// --- Delete ---
	err = env.client.CoreV1().Nodes().Delete(env.ctx, "test-node", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Nodes.Len() == 0
	}, waitTimeout, pollInterval, "expected 0 nodes in store after delete")
}

func TestNodeCollector_MultipleNodes(t *testing.T) {
	env := newTestEnv(t)
	c := NewNodeCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	for _, name := range []string{"node-1", "node-2", "node-3"} {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
		_, err := env.client.CoreV1().Nodes().Create(env.ctx, node, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		return env.store.Nodes.Len() == 3
	}, waitTimeout, pollInterval)

	for _, name := range []string{"node-1", "node-2", "node-3"} {
		_, ok := env.store.Nodes.Get(name)
		assert.True(t, ok, "expected %s in store", name)
	}
}
