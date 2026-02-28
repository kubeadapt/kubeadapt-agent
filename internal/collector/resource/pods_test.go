package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewPodCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "pods", c.Name())
}

func TestPodCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewPodCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	_, err := env.client.CoreV1().Pods("default").Create(env.ctx, pod, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Pods.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.Pods.Get("default/test-pod")
	require.True(t, ok)
	assert.Equal(t, "test-pod", info.Name)
	assert.Equal(t, "default", info.Namespace)

	// --- Update ---
	pod.Status.Phase = corev1.PodSucceeded
	_, err = env.client.CoreV1().Pods("default").Update(env.ctx, pod, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.Pods.Get("default/test-pod")
		return info.Phase == "Succeeded"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.CoreV1().Pods("default").Delete(env.ctx, "test-pod", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Pods.Len() == 0
	}, waitTimeout, pollInterval)
}

func TestPodCollector_MultipleNamespaces(t *testing.T) {
	env := newTestEnv(t)
	c := NewPodCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	pods := []struct{ ns, name string }{
		{"default", "pod-a"},
		{"kube-system", "pod-b"},
		{"production", "pod-c"},
	}
	for _, p := range pods {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: p.name, Namespace: p.ns},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx"}}},
		}
		_, err := env.client.CoreV1().Pods(p.ns).Create(env.ctx, pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		return env.store.Pods.Len() == 3
	}, waitTimeout, pollInterval)

	for _, p := range pods {
		_, ok := env.store.Pods.Get(p.ns + "/" + p.name)
		assert.True(t, ok, "expected %s/%s in store", p.ns, p.name)
	}
}
