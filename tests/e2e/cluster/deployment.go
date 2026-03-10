package cluster

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// DeployOrder specifies deployment execution order.
type DeployOrder int

const (
	// Preconditions — namespaces, RBAC, etc.
	Preconditions DeployOrder = iota
	// IngestionStub — deploy before agent so it has a backend target.
	IngestionStub
	// ExternalServices — metrics-server.
	ExternalServices
	// Agent — KubeAdapt agent deployment.
	Agent
	// TestWorkloads — test pods and services.
	TestWorkloads
)

// Deployment represents a Kubernetes deployment step.
type Deployment struct {
	Order      DeployOrder
	DeployFunc env.Func
	Ready      *Readiness
}

// Readiness defines a readiness check for a deployment.
type Readiness struct {
	Function    func(*envconf.Config) error
	Description string
	Timeout     time.Duration
	Retry       time.Duration
}

// NewManifestDeployment creates a deployment from a YAML manifest file.
func NewManifestDeployment(order DeployOrder, manifestPath string, ready *Readiness) Deployment {
	return Deployment{
		Order: order,
		DeployFunc: func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Printf("  → Applying manifest: %s\n", manifestPath)

			data, err := os.ReadFile(manifestPath)
			if err != nil {
				return ctx, fmt.Errorf("reading manifest: %w", err)
			}

			client, err := cfg.NewClient()
			if err != nil {
				return ctx, fmt.Errorf("creating client: %w", err)
			}

			if err := decoder.DecodeEach(ctx, bytes.NewReader(data), decoder.CreateIgnoreAlreadyExists(client.Resources())); err != nil {
				return ctx, fmt.Errorf("applying manifest: %w", err)
			}

			fmt.Printf("  ✓ Applied: %s\n", manifestPath)
			return ctx, nil
		},
		Ready: ready,
	}
}

// WaitForDeploymentReady creates a readiness check for a Deployment.
func WaitForDeploymentReady(namespace, name string, timeout, retry time.Duration) *Readiness {
	return &Readiness{
		Function: func(cfg *envconf.Config) error {
			client, err := cfg.NewClient()
			if err != nil {
				return err
			}

			var deploy appsv1.Deployment
			if err := client.Resources().Get(context.Background(), name, namespace, &deploy); err != nil {
				return err
			}

			if deploy.Spec.Replicas != nil && deploy.Status.ReadyReplicas == *deploy.Spec.Replicas && *deploy.Spec.Replicas > 0 {
				return nil
			}

			return fmt.Errorf("deployment not ready: %d/%d replicas",
				deploy.Status.ReadyReplicas, ptrOrZero(deploy.Spec.Replicas))
		},
		Description: fmt.Sprintf("Wait for Deployment %s/%s", namespace, name),
		Timeout:     timeout,
		Retry:       retry,
	}
}

// WaitForDaemonSetReady creates a readiness check for a DaemonSet.
func WaitForDaemonSetReady(namespace, name string, timeout, retry time.Duration) *Readiness {
	return &Readiness{
		Function: func(cfg *envconf.Config) error {
			client, err := cfg.NewClient()
			if err != nil {
				return err
			}

			var ds appsv1.DaemonSet
			if err := client.Resources().Get(context.Background(), name, namespace, &ds); err != nil {
				return err
			}

			if ds.Status.DesiredNumberScheduled > 0 &&
				ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
				return nil
			}

			return fmt.Errorf("daemonset not ready: %d/%d pods ready",
				ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
		},
		Description: fmt.Sprintf("Wait for DaemonSet %s/%s", namespace, name),
		Timeout:     timeout,
		Retry:       retry,
	}
}

// WaitForPodsReady creates a readiness check for pods with specific labels.
func WaitForPodsReady(namespace string, labelSelector map[string]string, minPods int, timeout, retry time.Duration) *Readiness {
	return &Readiness{
		Function: func(cfg *envconf.Config) error {
			client, err := cfg.NewClient()
			if err != nil {
				return err
			}

			var podList corev1.PodList
			if err := client.Resources(namespace).List(context.Background(), &podList); err != nil {
				return err
			}

			readyCount := 0
			for _, pod := range podList.Items {
				if matchesLabels(pod.Labels, labelSelector) && isPodReady(&pod) {
					readyCount++
				}
			}

			if readyCount >= minPods {
				return nil
			}

			return fmt.Errorf("not enough ready pods: %d/%d", readyCount, minPods)
		},
		Description: fmt.Sprintf("Wait for %d pods in %s", minPods, namespace),
		Timeout:     timeout,
		Retry:       retry,
	}
}

// WaitForMetricsAPI creates a readiness check for the metrics.k8s.io API.
func WaitForMetricsAPI(timeout, retry time.Duration) *Readiness {
	return &Readiness{
		Function: func(cfg *envconf.Config) error {
			restConfig := cfg.Client().RESTConfig()
			clientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return err
			}

			// Check if metrics API group is registered
			_, err = clientset.Discovery().ServerResourcesForGroupVersion("metrics.k8s.io/v1beta1")
			if err != nil {
				return fmt.Errorf("metrics API not available: %w", err)
			}

			// Actually query node metrics to verify data is flowing
			result := clientset.RESTClient().Get().
				AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").
				Do(context.Background())
			if err := result.Error(); err != nil {
				return fmt.Errorf("cannot query node metrics: %w", err)
			}

			return nil
		},
		Description: "Wait for metrics.k8s.io API availability",
		Timeout:     timeout,
		Retry:       retry,
	}
}

// WaitForHTTPEndpoint creates a readiness check for an HTTP endpoint.
func WaitForHTTPEndpoint(url string, expectedStatus int, timeout, retry time.Duration) *Readiness {
	return &Readiness{
		Function: func(_ *envconf.Config) error {
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				return fmt.Errorf("HTTP GET %s: %w", url, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != expectedStatus {
				return fmt.Errorf("HTTP %s returned %d, want %d", url, resp.StatusCode, expectedStatus)
			}
			return nil
		},
		Description: fmt.Sprintf("Wait for HTTP %d at %s", expectedStatus, url),
		Timeout:     timeout,
		Retry:       retry,
	}
}

// WaitForServiceEndpoints creates a readiness check for service endpoints.
func WaitForServiceEndpoints(namespace, serviceName string, timeout, retry time.Duration) *Readiness {
	return &Readiness{
		Function: func(cfg *envconf.Config) error {
			client, err := cfg.NewClient()
			if err != nil {
				return err
			}

			//nolint:staticcheck // SA1019: Using Endpoints API for E2E test compatibility
			var endpoints corev1.Endpoints
			if err := client.Resources().Get(context.Background(), serviceName, namespace, &endpoints); err != nil {
				return err
			}

			for _, subset := range endpoints.Subsets {
				if len(subset.Addresses) > 0 {
					return nil
				}
			}

			return fmt.Errorf("no endpoints ready for service %s/%s", namespace, serviceName)
		},
		Description: fmt.Sprintf("Wait for Service %s/%s endpoints", namespace, serviceName),
		Timeout:     timeout,
		Retry:       retry,
	}
}

// Helper functions

func matchesLabels(podLabels, selector map[string]string) bool {
	for key, value := range selector {
		if podLabels[key] != value {
			return false
		}
	}
	return true
}

func isPodReady(pod *corev1.Pod) bool {
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

func ptrOrZero(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}
