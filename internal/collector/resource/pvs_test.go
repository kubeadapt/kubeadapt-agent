package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPVCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewPVCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "pvs", c.Name())
}

func TestPVCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewPVCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-data"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              "standard",
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeAvailable},
	}
	_, err := env.client.CoreV1().PersistentVolumes().Create(env.ctx, pv, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.PVs.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.PVs.Get("pv-data")
	require.True(t, ok)
	assert.Equal(t, "pv-data", info.Name)
	assert.Equal(t, "Available", info.Phase)

	// --- Update ---
	pv.Status.Phase = corev1.VolumeBound
	_, err = env.client.CoreV1().PersistentVolumes().UpdateStatus(env.ctx, pv, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.PVs.Get("pv-data")
		return info.Phase == "Bound"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.CoreV1().PersistentVolumes().Delete(env.ctx, "pv-data", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.PVs.Len() == 0
	}, waitTimeout, pollInterval)
}
