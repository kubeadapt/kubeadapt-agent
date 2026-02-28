package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestReplicaSetCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewReplicaSetCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "replicasets", c.Name())
}

func TestReplicaSetCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewReplicaSetCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-abc123",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Deployment",
					Name: "web",
					UID:  types.UID("deploy-uid-1"),
				},
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: ptr.To(int32(3)),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
		Status: appsv1.ReplicaSetStatus{
			ReadyReplicas: 2,
		},
	}
	_, err := env.client.AppsV1().ReplicaSets("default").Create(env.ctx, rs, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.ReplicaSets.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.ReplicaSets.Get("default/web-abc123")
	require.True(t, ok)
	assert.Equal(t, "web-abc123", info.Name)
	assert.Equal(t, "Deployment", info.OwnerKind)
	assert.Equal(t, "web", info.OwnerName)

	// --- Update ---
	rs.Status.ReadyReplicas = 3
	_, err = env.client.AppsV1().ReplicaSets("default").Update(env.ctx, rs, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.ReplicaSets.Get("default/web-abc123")
		return info.ReadyReplicas == 3
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.AppsV1().ReplicaSets("default").Delete(env.ctx, "web-abc123", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.ReplicaSets.Len() == 0
	}, waitTimeout, pollInterval)
}
