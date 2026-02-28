package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewServiceCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "services", c.Name())
}

func TestServiceCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewServiceCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": "web"},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	_, err := env.client.CoreV1().Services("default").Create(env.ctx, svc, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Services.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.Services.Get("default/web-svc")
	require.True(t, ok)
	assert.Equal(t, "web-svc", info.Name)
	assert.Equal(t, "ClusterIP", info.Type)

	// --- Update ---
	svc.Spec.Type = corev1.ServiceTypeNodePort
	_, err = env.client.CoreV1().Services("default").Update(env.ctx, svc, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.Services.Get("default/web-svc")
		return info.Type == "NodePort"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.CoreV1().Services("default").Delete(env.ctx, "web-svc", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Services.Len() == 0
	}, waitTimeout, pollInterval)
}
