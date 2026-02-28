package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestDeploymentCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewDeploymentCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "deployments", c.Name())
}

func TestDeploymentCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewDeploymentCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(3)),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
	}
	_, err := env.client.AppsV1().Deployments("default").Create(env.ctx, dep, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Deployments.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.Deployments.Get("default/web")
	require.True(t, ok)
	assert.Equal(t, "web", info.Name)
	assert.Equal(t, int32(3), info.Replicas)

	// --- Update ---
	dep.Spec.Replicas = ptr.To(int32(5))
	_, err = env.client.AppsV1().Deployments("default").Update(env.ctx, dep, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.Deployments.Get("default/web")
		return info.Replicas == 5
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.AppsV1().Deployments("default").Delete(env.ctx, "web", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Deployments.Len() == 0
	}, waitTimeout, pollInterval)
}
