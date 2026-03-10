// Package cluster provides Kind cluster lifecycle management for E2E tests.
package cluster

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
)

const (
	// agentImageName is the local Docker image name for the agent.
	agentImageName = "localhost/kubeadapt-agent:e2e-test"
	// stubImageName is the local Docker image name for the ingestion stub.
	stubImageName = "localhost/ingestion-stub:e2e-test"
	// kindNodeImage is the Kind node image to use.
	kindNodeImage = "kindest/node:v1.32.8"
	// logsDir is the subdirectory for storing cluster logs.
	logsDir = "e2e-logs"
)

// Cluster represents a Kind cluster for E2E testing.
type Cluster struct {
	name        string
	baseDir     string
	deployments []Deployment
	testEnv     env.Environment
}

// NewCluster creates a new E2E test cluster.
func NewCluster(name string) *Cluster {
	// Get base directory (tests/e2e)
	_, filename, _, _ := runtime.Caller(0)
	baseDir := path.Join(path.Dir(filename), "..")

	return &Cluster{
		name:        name,
		baseDir:     baseDir,
		testEnv:     env.New(),
		deployments: []Deployment{},
	}
}

// Run executes the E2E test suite with proper cluster lifecycle.
func (c *Cluster) Run(m *testing.M) int {
	kindConfigPath := path.Join(c.baseDir, "testdata", "kind-config.yaml")

	// Setup functions to run before tests
	setupFuncs := []env.Func{
		// Create Kind cluster
		envfuncs.CreateClusterWithConfig(
			kind.NewProvider(),
			c.name,
			kindConfigPath,
			kind.WithImage(kindNodeImage),
		),
		// Load images into cluster
		c.loadImage(agentImageName),
		c.loadImage(stubImageName),
	}

	// Add deployment functions
	setupFuncs = append(setupFuncs, c.getDeploymentFuncs()...)

	// Teardown functions to run after tests
	teardownFuncs := []env.Func{
		// Export logs for debugging
		c.exportLogs(),
	}

	// Only destroy cluster if E2E_SKIP_CLEANUP is not set
	if os.Getenv("E2E_SKIP_CLEANUP") == "" {
		teardownFuncs = append(teardownFuncs, envfuncs.DestroyCluster(c.name))
	} else {
		fmt.Println("⚠️  E2E_SKIP_CLEANUP set — cluster will NOT be destroyed")
	}

	// Run the test suite
	return c.testEnv.Setup(setupFuncs...).
		Finish(teardownFuncs...).
		Run(m)
}

// TestEnv returns the test environment for use in tests.
func (c *Cluster) TestEnv() env.Environment {
	return c.testEnv
}

// AddDeployment adds a deployment to the cluster setup.
func (c *Cluster) AddDeployment(dep Deployment) {
	c.deployments = append(c.deployments, dep)
}

// loadImage loads a Docker image into the Kind cluster.
func (c *Cluster) loadImage(imageName string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		fmt.Printf("→ Loading image '%s' into Kind cluster '%s'\n", imageName, c.name)

		ctx, err := envfuncs.LoadDockerImageToCluster(c.name, imageName)(ctx, cfg)
		if err != nil {
			return ctx, fmt.Errorf("failed to load image %s: %w", imageName, err)
		}

		fmt.Printf("✓ Image '%s' loaded\n", imageName)
		return ctx, nil
	}
}

// getDeploymentFuncs returns env.Func slice for all deployments in order.
func (c *Cluster) getDeploymentFuncs() []env.Func {
	var funcs []env.Func

	sorted := c.sortedDeployments()

	currentOrder := Preconditions
	var readyFuncs []env.Func

	for _, dep := range sorted {
		if dep.Order != currentOrder {
			fmt.Printf("→ Waiting for %s deployments to be ready\n", orderName(currentOrder))
			funcs = append(funcs, readyFuncs...)
			readyFuncs = nil
			currentOrder = dep.Order
			fmt.Printf("→ Starting %s deployments\n", orderName(currentOrder))
		}

		funcs = append(funcs, dep.DeployFunc)

		if dep.Ready != nil {
			readyFuncs = append(readyFuncs, c.wrapReadiness(dep))
		}
	}

	if len(readyFuncs) > 0 {
		fmt.Printf("→ Waiting for %s deployments to be ready\n", orderName(currentOrder))
		funcs = append(funcs, readyFuncs...)
	}

	return funcs
}

// wrapReadiness wraps a readiness check with timeout and retry logic.
func (c *Cluster) wrapReadiness(dep Deployment) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		fmt.Printf("  → %s\n", dep.Ready.Description)

		readyCtx, cancel := context.WithTimeout(ctx, dep.Ready.Timeout)
		defer cancel()

		// Check immediately first
		if err := dep.Ready.Function(cfg); err == nil {
			fmt.Printf("  ✓ %s\n", dep.Ready.Description)
			return ctx, nil
		}

		ticker := time.NewTicker(dep.Ready.Retry)
		defer ticker.Stop()

		for {
			select {
			case <-readyCtx.Done():
				return ctx, fmt.Errorf("readiness check timed out: %s", dep.Ready.Description)
			case <-ticker.C:
				if err := dep.Ready.Function(cfg); err != nil {
					continue
				}
				fmt.Printf("  ✓ %s\n", dep.Ready.Description)
				return ctx, nil
			}
		}
	}
}

// sortedDeployments returns deployments sorted by Order.
func (c *Cluster) sortedDeployments() []Deployment {
	sorted := make([]Deployment, len(c.deployments))
	copy(sorted, c.deployments)

	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j].Order > key.Order {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}

	return sorted
}

// exportLogs exports cluster logs to e2e-logs directory.
func (c *Cluster) exportLogs() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		logsPath := path.Join(c.baseDir, logsDir)
		fmt.Printf("→ Exporting cluster logs to %s\n", logsPath)

		if err := os.MkdirAll(logsPath, 0755); err != nil {
			fmt.Printf("⚠️  Failed to create logs directory: %v\n", err)
			return ctx, nil // Non-fatal
		}

		ctx, err := envfuncs.ExportClusterLogs(c.name, logsPath)(ctx, cfg)
		if err != nil {
			fmt.Printf("⚠️  Failed to export logs: %v\n", err)
			return ctx, nil // Non-fatal
		}

		fmt.Println("✓ Logs exported")
		return ctx, nil
	}
}

// GetClient returns a Kubernetes client for the cluster.
func (c *Cluster) GetClient(cfg *envconf.Config) (klient.Client, error) {
	return cfg.NewClient()
}

// GetClientset returns a typed Kubernetes clientset.
func (c *Cluster) GetClientset(cfg *envconf.Config) (kubernetes.Interface, error) {
	restConfig := cfg.Client().RESTConfig()
	return kubernetes.NewForConfig(restConfig)
}

func orderName(order DeployOrder) string {
	switch order {
	case Preconditions:
		return "Preconditions"
	case IngestionStub:
		return "IngestionStub"
	case ExternalServices:
		return "ExternalServices"
	case Agent:
		return "Agent"
	case TestWorkloads:
		return "TestWorkloads"
	default:
		return fmt.Sprintf("Unknown(%d)", order)
	}
}
