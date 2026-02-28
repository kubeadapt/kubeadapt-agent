package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestHPACollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewHPACollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "hpas", c.Name())
}

func TestHPACollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewHPACollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "web-hpa", Namespace: "default"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "web",
			},
			MinReplicas: ptr.To(int32(2)),
			MaxReplicas: 10,
		},
	}
	_, err := env.client.AutoscalingV2().HorizontalPodAutoscalers("default").Create(env.ctx, hpa, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.HPAs.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.HPAs.Get("default/web-hpa")
	require.True(t, ok)
	assert.Equal(t, "web-hpa", info.Name)
	assert.Equal(t, int32(10), info.MaxReplicas)

	// --- Update ---
	hpa.Spec.MaxReplicas = 20
	_, err = env.client.AutoscalingV2().HorizontalPodAutoscalers("default").Update(env.ctx, hpa, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.HPAs.Get("default/web-hpa")
		return info.MaxReplicas == 20
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.AutoscalingV2().HorizontalPodAutoscalers("default").Delete(env.ctx, "web-hpa", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.HPAs.Len() == 0
	}, waitTimeout, pollInterval)
}
