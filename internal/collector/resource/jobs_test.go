package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestJobCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewJobCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "jobs", c.Name())
}

func TestJobCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewJobCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "default"},
		Spec: batchv1.JobSpec{
			Completions: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "worker", Image: "busybox"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	_, err := env.client.BatchV1().Jobs("default").Create(env.ctx, job, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Jobs.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.Jobs.Get("default/test-job")
	require.True(t, ok)
	assert.Equal(t, "test-job", info.Name)
	assert.Equal(t, "default", info.Namespace)

	// --- Update ---
	job.Status.Active = 1
	_, err = env.client.BatchV1().Jobs("default").UpdateStatus(env.ctx, job, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.Jobs.Get("default/test-job")
		return info.Active == 1
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.BatchV1().Jobs("default").Delete(env.ctx, "test-job", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Jobs.Len() == 0
	}, waitTimeout, pollInterval)
}
