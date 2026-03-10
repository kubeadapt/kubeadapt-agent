// Package helpers provides test assertion helpers for E2E tests.
package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/klient"
)

// WaitForPodReady waits for a specific pod to be ready.
func WaitForPodReady(ctx context.Context, t *testing.T, client klient.Client, namespace, podName string, timeout time.Duration) error {
	t.Helper()
	t.Logf("Waiting for pod %s/%s to be ready", namespace, podName)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for pod %s/%s", namespace, podName)
			}

			var pod corev1.Pod
			if err := client.Resources().Get(ctx, podName, namespace, &pod); err != nil {
				t.Logf("  pod %s/%s not found yet: %v", namespace, podName, err)
				continue
			}

			if IsPodReady(&pod) {
				t.Logf("✓ Pod %s/%s is ready", namespace, podName)
				return nil
			}

			t.Logf("  pod %s/%s not ready (phase: %s)", namespace, podName, pod.Status.Phase)
		}
	}
}

// GetFirstPodWithLabel gets the first pod matching label selector.
func GetFirstPodWithLabel(ctx context.Context, t *testing.T, client klient.Client, namespace string, labelSelector map[string]string) (*corev1.Pod, error) {
	t.Helper()

	var podList corev1.PodList
	if err := client.Resources(namespace).List(ctx, &podList); err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	for _, pod := range podList.Items {
		if matchesLabels(pod.Labels, labelSelector) {
			return &pod, nil
		}
	}

	return nil, fmt.Errorf("no pod found with labels %v in %s", labelSelector, namespace)
}

// ListPodsInNamespace lists all pods in a namespace.
func ListPodsInNamespace(ctx context.Context, t *testing.T, client klient.Client, namespace string) ([]corev1.Pod, error) {
	t.Helper()

	var podList corev1.PodList
	if err := client.Resources(namespace).List(ctx, &podList); err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	t.Logf("Found %d pods in %s", len(podList.Items), namespace)
	return podList.Items, nil
}

// IsPodReady checks if a pod is in Ready state.
func IsPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

// GetPodStatus returns a human-readable pod status.
func GetPodStatus(pod *corev1.Pod) string {
	if IsPodReady(pod) {
		return "Ready"
	}
	if pod.Status.Phase == corev1.PodRunning {
		return "Running (not ready)"
	}
	return string(pod.Status.Phase)
}

func matchesLabels(podLabels, selector map[string]string) bool {
	for key, value := range selector {
		if podLabels[key] != value {
			return false
		}
	}
	return true
}
