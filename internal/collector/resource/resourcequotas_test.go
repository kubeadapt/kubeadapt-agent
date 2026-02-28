package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceQuotaCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewResourceQuotaCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "resourcequotas", c.Name())
}

func TestResourceQuotaCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewResourceQuotaCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "compute-quota", Namespace: "default"},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("20Gi"),
			},
		},
	}
	_, err := env.client.CoreV1().ResourceQuotas("default").Create(env.ctx, rq, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.ResourceQuotas.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.ResourceQuotas.Get("default/compute-quota")
	require.True(t, ok)
	assert.Equal(t, "compute-quota", info.Name)
	assert.Equal(t, "10", info.Hard["cpu"])

	// --- Update ---
	rq.Spec.Hard[corev1.ResourceCPU] = resource.MustParse("20")
	_, err = env.client.CoreV1().ResourceQuotas("default").Update(env.ctx, rq, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.ResourceQuotas.Get("default/compute-quota")
		return info.Hard["cpu"] == "20"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.CoreV1().ResourceQuotas("default").Delete(env.ctx, "compute-quota", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.ResourceQuotas.Len() == 0
	}, waitTimeout, pollInterval)
}
