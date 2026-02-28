package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLimitRangeCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewLimitRangeCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "limitranges", c.Name())
}

func TestLimitRangeCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewLimitRangeCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: "default-limits", Namespace: "default"},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
		},
	}
	_, err := env.client.CoreV1().LimitRanges("default").Create(env.ctx, lr, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.LimitRanges.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.LimitRanges.Get("default/default-limits")
	require.True(t, ok)
	assert.Equal(t, "default-limits", info.Name)
	require.Len(t, info.Limits, 1)
	assert.Equal(t, "Container", info.Limits[0].Type)

	// --- Update ---
	lr.Spec.Limits[0].Default[corev1.ResourceCPU] = resource.MustParse("1")
	_, err = env.client.CoreV1().LimitRanges("default").Update(env.ctx, lr, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.LimitRanges.Get("default/default-limits")
		return len(info.Limits) == 1 && info.Limits[0].Default["cpu"] == "1"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.CoreV1().LimitRanges("default").Delete(env.ctx, "default-limits", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.LimitRanges.Len() == 0
	}, waitTimeout, pollInterval)
}
