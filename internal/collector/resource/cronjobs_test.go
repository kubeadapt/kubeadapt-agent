package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCronJobCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewCronJobCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "cronjobs", c.Name())
}

func TestCronJobCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewCronJobCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "nightly-backup", Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 2 * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:    []corev1.Container{{Name: "backup", Image: "busybox"}},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
	_, err := env.client.BatchV1().CronJobs("default").Create(env.ctx, cj, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.CronJobs.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.CronJobs.Get("default/nightly-backup")
	require.True(t, ok)
	assert.Equal(t, "nightly-backup", info.Name)
	assert.Equal(t, "0 2 * * *", info.Schedule)

	// --- Update ---
	cj.Spec.Schedule = "0 3 * * *"
	_, err = env.client.BatchV1().CronJobs("default").Update(env.ctx, cj, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.CronJobs.Get("default/nightly-backup")
		return info.Schedule == "0 3 * * *"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.BatchV1().CronJobs("default").Delete(env.ctx, "nightly-backup", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.CronJobs.Len() == 0
	}, waitTimeout, pollInterval)
}
