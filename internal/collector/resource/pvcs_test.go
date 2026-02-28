package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestPVCCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewPVCCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "pvcs", c.Name())
}

func TestPVCCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewPVCCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: ptr.To("standard"),
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}
	_, err := env.client.CoreV1().PersistentVolumeClaims("default").Create(env.ctx, pvc, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.PVCs.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.PVCs.Get("default/data-pvc")
	require.True(t, ok)
	assert.Equal(t, "data-pvc", info.Name)
	assert.Equal(t, "Pending", info.Phase)

	// --- Update ---
	pvc.Status.Phase = corev1.ClaimBound
	_, err = env.client.CoreV1().PersistentVolumeClaims("default").UpdateStatus(env.ctx, pvc, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.PVCs.Get("default/data-pvc")
		return info.Phase == "Bound"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.CoreV1().PersistentVolumeClaims("default").Delete(env.ctx, "data-pvc", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.PVCs.Len() == 0
	}, waitTimeout, pollInterval)
}
