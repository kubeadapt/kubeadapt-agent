// Package cluster provides Kind cluster lifecycle management for E2E tests.
package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	// defaultAgentImage is the default local Docker image name for the agent.
	defaultAgentImage = "localhost/kubeadapt-agent:e2e-test"
	// defaultStubImage is the default local Docker image name for the ingestion stub.
	defaultStubImage = "localhost/ingestion-stub:e2e-test"
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
		// Prepare images: retag CI images if needed, build stub if missing
		c.prepareImages(),
		// Create Kind cluster
		envfuncs.CreateClusterWithConfig(
			kind.NewProvider(),
			c.name,
			kindConfigPath,
			kind.WithImage(kindNodeImage),
		),
		// Load images into cluster (always use default names — prepareImages ensures they exist)
		c.loadImage(defaultAgentImage),
		c.loadImage(defaultStubImage),
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

// prepareImages ensures the required Docker images exist with the expected names.
// In CI, the agent image may be tagged differently (e.g. :test instead of :e2e-test).
// This function retags CI images and builds the stub image if not present.
func (c *Cluster) prepareImages() env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		// Handle agent image: retag if CI provides a different tag via E2E_AGENT_IMAGE.
		if src := os.Getenv("E2E_AGENT_IMAGE"); src != "" && src != defaultAgentImage {
			fmt.Printf("→ Retagging agent image %s → %s\n", src, defaultAgentImage)
			if err := dockerTag(src, defaultAgentImage); err != nil {
				return ctx, fmt.Errorf("retag agent image: %w", err)
			}
		}

		// Handle stub image: retag from env, or build from source if not present.
		if src := os.Getenv("E2E_STUB_IMAGE"); src != "" && src != defaultStubImage {
			fmt.Printf("→ Retagging stub image %s → %s\n", src, defaultStubImage)
			if err := dockerTag(src, defaultStubImage); err != nil {
				return ctx, fmt.Errorf("retag stub image: %w", err)
			}
		} else if !dockerImageExists(defaultStubImage) {
			fmt.Printf("→ Building stub image %s from source\n", defaultStubImage)
			// Stub Dockerfile is at tests/e2e/stub/Dockerfile, context is repo root.
			repoRoot := path.Join(c.baseDir, "..", "..")
			if err := dockerBuild(defaultStubImage, path.Join(c.baseDir, "stub", "Dockerfile"), repoRoot); err != nil {
				return ctx, fmt.Errorf("build stub image: %w", err)
			}
		}

		return ctx, nil
	}
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
			// Wrap status messages in env.Func so they print during execution, not construction.
			waitOrder := currentOrder
			funcs = append(funcs, readyFuncs...)
			funcs = append(funcs, logFunc("→ %s deployments ready", orderName(waitOrder)))
			readyFuncs = nil
			currentOrder = dep.Order
			funcs = append(funcs, logFunc("→ Starting %s deployments", orderName(currentOrder)))
		}

		funcs = append(funcs, dep.DeployFunc)

		if dep.Ready != nil {
			readyFuncs = append(readyFuncs, c.wrapReadiness(dep))
		}
	}

	if len(readyFuncs) > 0 {
		waitOrder := currentOrder
		funcs = append(funcs, readyFuncs...)
		funcs = append(funcs, logFunc("→ %s deployments ready", orderName(waitOrder)))
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

// logFunc wraps a fmt.Printf call in an env.Func for deferred execution.
func logFunc(format string, args ...any) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		fmt.Printf(format+"\n", args...)
		return ctx, nil
	}
}

// dockerTag retags a Docker image.
func dockerTag(src, dst string) error {
	cmd := exec.Command("docker", "tag", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", string(out), err)
	}
	return nil
}

// dockerImageExists checks if a Docker image exists locally.
func dockerImageExists(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	return cmd.Run() == nil
}

// dockerBuild builds a Docker image from a Dockerfile.
func dockerBuild(tag, dockerfile, context string) error {
	cmd := exec.Command("docker", "build", "-t", tag, "-f", dockerfile, context)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
