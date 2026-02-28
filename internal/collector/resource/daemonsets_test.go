package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDaemonSetCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewDaemonSetCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "daemonsets", c.Name())
}

func TestDaemonSetCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewDaemonSetCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "monitor", Namespace: "kube-system"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "monitor"}},
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 3,
			NumberReady:            2,
		},
	}
	_, err := env.client.AppsV1().DaemonSets("kube-system").Create(env.ctx, ds, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.DaemonSets.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.DaemonSets.Get("kube-system/monitor")
	require.True(t, ok)
	assert.Equal(t, "monitor", info.Name)
	assert.Equal(t, "kube-system", info.Namespace)

	// --- Update ---
	ds.Status.NumberReady = 3
	_, err = env.client.AppsV1().DaemonSets("kube-system").Update(env.ctx, ds, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.DaemonSets.Get("kube-system/monitor")
		return info.NumberReady == 3
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.AppsV1().DaemonSets("kube-system").Delete(env.ctx, "monitor", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.DaemonSets.Len() == 0
	}, waitTimeout, pollInterval)
}
