package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStorageClassCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewStorageClassCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "storageclasses", c.Name())
}

func TestStorageClassCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewStorageClassCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	sc := &storagev1.StorageClass{
		ObjectMeta:  metav1.ObjectMeta{Name: "gp3"},
		Provisioner: "ebs.csi.aws.com",
		Parameters:  map[string]string{"type": "gp3"},
	}
	_, err := env.client.StorageV1().StorageClasses().Create(env.ctx, sc, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.StorageClasses.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.StorageClasses.Get("gp3")
	require.True(t, ok)
	assert.Equal(t, "gp3", info.Name)
	assert.Equal(t, "ebs.csi.aws.com", info.Provisioner)

	// --- Update ---
	sc.Parameters["type"] = "gp2"
	_, err = env.client.StorageV1().StorageClasses().Update(env.ctx, sc, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.StorageClasses.Get("gp3")
		return info.Parameters["type"] == "gp2"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.StorageV1().StorageClasses().Delete(env.ctx, "gp3", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.StorageClasses.Len() == 0
	}, waitTimeout, pollInterval)
}
