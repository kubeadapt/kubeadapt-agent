package discovery

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

// Well-known API groups used for capability detection.
const (
	apiGroupMetrics   = "metrics.k8s.io"
	apiGroupVPA       = "autoscaling.k8s.io"
	apiGroupKarpenter = "karpenter.sh"
)

// Capabilities describes optional cluster features detected at startup.
// Results are computed once and cached for the agent's lifetime.
type Capabilities struct {
	MetricsServer         bool     // metrics.k8s.io API group exists
	VPA                   bool     // autoscaling.k8s.io API group exists (VPA CRD)
	Karpenter             bool     // karpenter.sh API group exists
	Provider              string   // "aws", "gcp", "azure", "unknown"
	DCGMExporter          bool     // dcgm-exporter pods found on GPU nodes
	DCGMExporterEndpoints []string // pod IPs of discovered dcgm-exporter instances
}

// Detect probes the cluster for optional capabilities and the cloud provider.
// It uses the discovery API to check for API groups and lists a single node
// for provider detection. This is intended to run once at startup.
func Detect(ctx context.Context, client kubernetes.Interface, discoveryClient discovery.DiscoveryInterface) (*Capabilities, error) {
	caps := &Capabilities{
		Provider: "unknown",
	}

	// Detect API groups.
	groups, err := discoveryClient.ServerGroups()
	if err != nil {
		return nil, fmt.Errorf("discovery: failed to list server groups: %w", err)
	}

	groupSet := make(map[string]bool, len(groups.Groups))
	for _, g := range groups.Groups {
		groupSet[g.Name] = true
	}

	caps.MetricsServer = groupSet[apiGroupMetrics]
	caps.VPA = groupSet[apiGroupVPA]
	caps.Karpenter = groupSet[apiGroupKarpenter]

	// Detect cloud provider from node metadata.
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err == nil && len(nodeList.Items) > 0 {
		node := nodeList.Items[0]
		caps.Provider = DetectProvider([]*v1.Node{&node})
	}

	caps.DCGMExporter, caps.DCGMExporterEndpoints = detectDCGMExporter(ctx, client)

	return caps, nil
}

// HasAPIGroup checks whether a specific API group is registered with the cluster.
func HasAPIGroup(discoveryClient discovery.DiscoveryInterface, group string) (bool, error) {
	groups, err := discoveryClient.ServerGroups()
	if err != nil {
		return false, fmt.Errorf("discovery: failed to list server groups: %w", err)
	}

	for _, g := range groups.Groups {
		if g.Name == group {
			return true, nil
		}
	}
	return false, nil
}

// DetectDCGMEndpoints probes the cluster for dcgm-exporter pods on GPU nodes
// and returns their pod IPs. Safe to call repeatedly for endpoint refresh.
func DetectDCGMEndpoints(ctx context.Context, client kubernetes.Interface) (bool, []string) {
	return detectDCGMExporter(ctx, client)
}

func detectDCGMExporter(ctx context.Context, client kubernetes.Interface) (bool, []string) {
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, nil
	}

	hasGPU := false
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if q, ok := node.Status.Allocatable[v1.ResourceName("nvidia.com/gpu")]; ok && q.Value() > 0 {
			hasGPU = true
			break
		}
		for rName := range node.Status.Allocatable {
			if strings.HasPrefix(string(rName), "nvidia.com/mig-") {
				hasGPU = true
				break
			}
		}
		if hasGPU {
			break
		}
	}

	if !hasGPU {
		return false, nil
	}

	selectors := []string{
		"app=nvidia-dcgm-exporter",
		"app.kubernetes.io/name=dcgm-exporter",
	}

	for _, sel := range selectors {
		pods, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			LabelSelector: sel,
		})
		if err != nil || len(pods.Items) == 0 {
			continue
		}
		var endpoints []string
		for _, pod := range pods.Items {
			if pod.Status.PodIP != "" {
				endpoints = append(endpoints, pod.Status.PodIP)
			}
		}
		if len(endpoints) > 0 {
			return true, endpoints
		}
	}

	return false, nil
}
