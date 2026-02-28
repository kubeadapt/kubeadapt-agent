package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestPDBCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewPDBCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "pdbs", c.Name())
}

func TestPDBCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewPDBCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	minAvail := intstr.FromInt32(1)
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "web-pdb", Namespace: "default"},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvail,
			Selector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
	}
	_, err := env.client.PolicyV1().PodDisruptionBudgets("default").Create(env.ctx, pdb, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.PDBs.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.PDBs.Get("default/web-pdb")
	require.True(t, ok)
	assert.Equal(t, "web-pdb", info.Name)
	assert.Equal(t, "1", info.MinAvailable)

	// --- Update ---
	newMin := intstr.FromInt32(2)
	pdb.Spec.MinAvailable = &newMin
	_, err = env.client.PolicyV1().PodDisruptionBudgets("default").Update(env.ctx, pdb, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.PDBs.Get("default/web-pdb")
		return info.MinAvailable == "2"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.PolicyV1().PodDisruptionBudgets("default").Delete(env.ctx, "web-pdb", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.PDBs.Len() == 0
	}, waitTimeout, pollInterval)
}
