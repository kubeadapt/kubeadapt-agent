package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPriorityClassCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewPriorityClassCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "priorityclasses", c.Name())
}

func TestPriorityClassCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewPriorityClassCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	pc := &schedulingv1.PriorityClass{
		ObjectMeta:    metav1.ObjectMeta{Name: "high-priority"},
		Value:         1000000,
		GlobalDefault: false,
		Description:   "High priority workloads",
	}
	_, err := env.client.SchedulingV1().PriorityClasses().Create(env.ctx, pc, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.PriorityClasses.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.PriorityClasses.Get("high-priority")
	require.True(t, ok)
	assert.Equal(t, "high-priority", info.Name)
	assert.Equal(t, int32(1000000), info.Value)

	// --- Update ---
	pc.Value = 2000000
	_, err = env.client.SchedulingV1().PriorityClasses().Update(env.ctx, pc, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.PriorityClasses.Get("high-priority")
		return info.Value == 2000000
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.SchedulingV1().PriorityClasses().Delete(env.ctx, "high-priority", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.PriorityClasses.Len() == 0
	}, waitTimeout, pollInterval)
}
