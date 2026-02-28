package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespaceCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewNamespaceCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "namespaces", c.Name())
}

func TestNamespaceCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewNamespaceCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-ns",
			Labels: map[string]string{"env": "test"},
		},
		Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	_, err := env.client.CoreV1().Namespaces().Create(env.ctx, ns, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Namespaces.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.Namespaces.Get("test-ns")
	require.True(t, ok)
	assert.Equal(t, "test-ns", info.Name)
	assert.Equal(t, "Active", info.Phase)

	// --- Update ---
	ns.Labels["env"] = "staging"
	_, err = env.client.CoreV1().Namespaces().Update(env.ctx, ns, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.Namespaces.Get("test-ns")
		return info.Labels["env"] == "staging"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.CoreV1().Namespaces().Delete(env.ctx, "test-ns", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Namespaces.Len() == 0
	}, waitTimeout, pollInterval)
}
