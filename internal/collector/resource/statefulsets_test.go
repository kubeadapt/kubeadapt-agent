package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestStatefulSetCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewStatefulSetCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "statefulsets", c.Name())
}

func TestStatefulSetCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewStatefulSetCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptr.To(int32(3)),
			ServiceName: "db-svc",
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
		},
	}
	_, err := env.client.AppsV1().StatefulSets("default").Create(env.ctx, ss, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.StatefulSets.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.StatefulSets.Get("default/db")
	require.True(t, ok)
	assert.Equal(t, "db", info.Name)
	assert.Equal(t, int32(3), info.Replicas)
	assert.Equal(t, "db-svc", info.ServiceName)

	// --- Update ---
	ss.Spec.Replicas = ptr.To(int32(5))
	_, err = env.client.AppsV1().StatefulSets("default").Update(env.ctx, ss, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.StatefulSets.Get("default/db")
		return info.Replicas == 5
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.AppsV1().StatefulSets("default").Delete(env.ctx, "db", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.StatefulSets.Len() == 0
	}, waitTimeout, pollInterval)
}
