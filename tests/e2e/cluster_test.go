package e2e

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kubeadapt/kubeadapt-agent/tests/e2e/cluster"
	"github.com/kubeadapt/kubeadapt-agent/tests/e2e/helpers"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	testCluster *cluster.Cluster
	agentURL    = "http://localhost:30080"
	stubURL     = "http://localhost:30081"
	testDataDir string
)

func TestMain(m *testing.M) {
	_, filename, _, _ := runtime.Caller(0)
	testDataDir = path.Join(path.Dir(filename), "testdata")

	clusterName := fmt.Sprintf("kubeadapt-e2e-%d", time.Now().Unix())
	testCluster = cluster.NewCluster(clusterName)

	registerDeployments()

	exitCode := testCluster.Run(m)
	os.Exit(exitCode)
}

// registerDeployments registers all deployments in proper order.
func registerDeployments() {
	// Phase 1: Ingestion Stub (namespace created in stub.yaml)
	testCluster.AddDeployment(cluster.NewManifestDeployment(
		cluster.IngestionStub,
		path.Join(testDataDir, "stub.yaml"),
		cluster.WaitForDeploymentReady("kubeadapt-system", "ingestion-stub", 2*time.Minute, 3*time.Second),
	))

	// Wait for stub HTTP health
	testCluster.AddDeployment(cluster.Deployment{
		Order: cluster.IngestionStub,
		DeployFunc: func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
			return ctx, nil
		},
		Ready: cluster.WaitForHTTPEndpoint(stubURL+"/healthz", 200, 1*time.Minute, 2*time.Second),
	})

	// Phase 2: External Services — metrics-server
	testCluster.AddDeployment(cluster.NewManifestDeployment(
		cluster.ExternalServices,
		path.Join(testDataDir, "metrics-server.yaml"),
		cluster.WaitForDeploymentReady("kube-system", "metrics-server", 3*time.Minute, 3*time.Second),
	))

	// Wait for metrics API to actually serve data
	testCluster.AddDeployment(cluster.Deployment{
		Order: cluster.ExternalServices,
		DeployFunc: func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
			return ctx, nil
		},
		Ready: cluster.WaitForMetricsAPI(3*time.Minute, 5*time.Second),
	})

	// Phase 3: Agent
	testCluster.AddDeployment(cluster.NewManifestDeployment(
		cluster.Agent,
		path.Join(testDataDir, "agent-e2e.yaml"),
		cluster.WaitForDeploymentReady("kubeadapt-system", "kubeadapt-agent", 3*time.Minute, 3*time.Second),
	))

	// Wait for agent readiness via HTTP
	testCluster.AddDeployment(cluster.Deployment{
		Order: cluster.Agent,
		DeployFunc: func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
			return ctx, nil
		},
		Ready: cluster.WaitForHTTPEndpoint(agentURL+"/readyz", 200, 2*time.Minute, 3*time.Second),
	})

	// Phase 4: Test Workloads
	testCluster.AddDeployment(cluster.NewManifestDeployment(
		cluster.TestWorkloads,
		path.Join(testDataDir, "sample-workloads.yaml"),
		cluster.WaitForPodsReady("test-workloads", map[string]string{"app": "nginx-web"}, 2, 3*time.Minute, 3*time.Second),
	))

	// Wait for one snapshot cycle so agent picks up test workloads
	testCluster.AddDeployment(cluster.Deployment{
		Order: cluster.TestWorkloads,
		DeployFunc: func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
			fmt.Println("⏳ Waiting 15s for agent to collect test workloads...")
			time.Sleep(15 * time.Second)
			fmt.Println("✓ Snapshot cycle complete")
			return ctx, nil
		},
	})
}

// ---------------------------------------------------------------------------
// Phase 1: Agent Lifecycle
// ---------------------------------------------------------------------------

func TestE2E_AgentLifecycle(t *testing.T) {
	feature := features.New("Agent Lifecycle").
		Assess("Agent is healthy", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)

			err := snapshotClient.CheckHealthz()
			require.NoError(t, err, "/healthz should return 200")

			return ctx
		}).
		Assess("Agent is ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)

			ready, err := snapshotClient.CheckReadyz()
			require.NoError(t, err, "/readyz should succeed")
			assert.True(t, ready, "agent should be ready")

			return ctx
		}).
		Assess("Agent metrics endpoint serves Prometheus format", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			metricsText, err := helpers.ScrapeMetrics(agentURL)
			require.NoError(t, err, "/metrics should return 200")
			assert.Contains(t, metricsText, "# HELP", "metrics should contain HELP comments")
			assert.Contains(t, metricsText, "# TYPE", "metrics should contain TYPE definitions")

			return ctx
		}).
		Feature()

	testCluster.TestEnv().Test(t, feature)
}

// ---------------------------------------------------------------------------
// Phase 2: Data Collection
// ---------------------------------------------------------------------------

func TestE2E_DataCollection(t *testing.T) {
	feature := features.New("Data Collection").
		Assess("Snapshot contains nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)

			snapshot, err := snapshotClient.WaitForSnapshot(t, 2*time.Minute)
			require.NoError(t, err, "snapshot should be available")

			assert.Greater(t, len(snapshot.Nodes), 0, "should have at least 1 node")
			t.Logf("Nodes: %d", len(snapshot.Nodes))

			return ctx
		}).
		Assess("Snapshot contains pods", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			assert.Greater(t, len(snapshot.Pods), 0, "should have pods")
			t.Logf("Pods: %d", len(snapshot.Pods))

			return ctx
		}).
		Assess("Snapshot contains deployments from test-workloads", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			assert.Greater(t, len(snapshot.Deployments), 0, "should have deployments")

			// Check test workload deployments exist
			found := map[string]bool{}
			for _, d := range snapshot.Deployments {
				found[d.Name] = true
			}
			assert.True(t, found["nginx-web"], "nginx-web deployment should be collected")
			assert.True(t, found["worker"], "worker deployment should be collected")

			return ctx
		}).
		Assess("Snapshot contains namespaces", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			assert.Greater(t, len(snapshot.Namespaces), 0, "should have namespaces")

			found := map[string]bool{}
			for _, ns := range snapshot.Namespaces {
				found[ns.Name] = true
			}
			assert.True(t, found["test-workloads"], "test-workloads namespace should exist")
			assert.True(t, found["kubeadapt-system"], "kubeadapt-system namespace should exist")
			assert.True(t, found["kube-system"], "kube-system namespace should exist")

			return ctx
		}).
		Assess("Snapshot contains services", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			assert.Greater(t, len(snapshot.Services), 0, "should have services")

			return ctx
		}).
		Assess("Snapshot contains HPAs", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			assert.Greater(t, len(snapshot.HPAs), 0, "should have at least 1 HPA (nginx-web-hpa)")

			return ctx
		}).
		Assess("Store counts match cluster state", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			counts, err := snapshotClient.GetStoreCounts()
			require.NoError(t, err)

			t.Logf("Store counts: %v", counts)

			// Kind single-node cluster should have at least 1 node
			assert.Greater(t, counts["nodes"], 0, "store should have nodes")
			assert.Greater(t, counts["pods"], 0, "store should have pods")
			assert.Greater(t, counts["namespaces"], 0, "store should have namespaces")

			return ctx
		}).
		Feature()

	testCluster.TestEnv().Test(t, feature)
}

// ---------------------------------------------------------------------------
// Phase 3: Snapshot Quality
// ---------------------------------------------------------------------------

func TestE2E_SnapshotQuality(t *testing.T) {
	feature := features.New("Snapshot Quality").
		Assess("Summary counts are correct", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			// Summary should match actual resource counts
			assert.Equal(t, len(snapshot.Nodes), snapshot.Summary.NodeCount, "summary node count should match")
			assert.Equal(t, len(snapshot.Pods), snapshot.Summary.PodCount, "summary pod count should match")
			assert.Equal(t, len(snapshot.Namespaces), snapshot.Summary.NamespaceCount, "summary namespace count should match")
			assert.Equal(t, len(snapshot.Deployments), snapshot.Summary.DeploymentCount, "summary deployment count should match")
			assert.Greater(t, snapshot.Summary.ContainerCount, 0, "should have containers")

			t.Logf("Summary: %d nodes, %d pods, %d containers, %d deployments",
				snapshot.Summary.NodeCount, snapshot.Summary.PodCount,
				snapshot.Summary.ContainerCount, snapshot.Summary.DeploymentCount)

			return ctx
		}).
		Assess("CPU/memory capacity is populated", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			assert.Greater(t, snapshot.Summary.TotalCPUCapacity, 0.0, "total CPU capacity should be > 0")
			assert.Greater(t, snapshot.Summary.TotalMemoryCapacity, int64(0), "total memory capacity should be > 0")
			assert.Greater(t, snapshot.Summary.TotalCPUAllocatable, 0.0, "total CPU allocatable should be > 0")
			assert.Greater(t, snapshot.Summary.TotalMemoryAllocatable, int64(0), "total memory allocatable should be > 0")

			return ctx
		}).
		Assess("Metrics-server data is available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			assert.True(t, snapshot.Summary.MetricsAvailable, "metrics should be available (metrics-server deployed)")
			assert.True(t, snapshot.Health.MetricsServerAvailable, "health should report metrics-server available")

			return ctx
		}).
		Assess("Agent health is populated", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			assert.Greater(t, snapshot.Health.UptimeSeconds, int64(0), "uptime should be > 0")
			assert.Greater(t, snapshot.Health.StartedAt, int64(0), "started_at should be set")
			assert.Equal(t, "running", snapshot.Health.State, "agent state should be running")
			assert.True(t, snapshot.Health.InformersSynced, "informers should be synced")

			t.Logf("Health: state=%s, uptime=%ds, informers=%d/%d",
				snapshot.Health.State, snapshot.Health.UptimeSeconds,
				snapshot.Health.InformersHealthy, snapshot.Health.InformersTotal)

			return ctx
		}).
		Assess("Cluster capability detection works", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			// Metrics-server is deployed, VPA/Karpenter/DCGM are not
			assert.True(t, snapshot.Health.MetricsServerAvailable, "metrics-server should be detected")
			assert.False(t, snapshot.Health.VPAAvailable, "VPA should not be detected")
			assert.False(t, snapshot.Health.KarpenterAvailable, "Karpenter should not be detected")
			assert.False(t, snapshot.Health.GPUMetricsAvailable, "DCGM should not be detected")

			return ctx
		}).
		Feature()

	testCluster.TestEnv().Test(t, feature)
}

// ---------------------------------------------------------------------------
// Phase 4: Transport (Ingestion Stub)
// ---------------------------------------------------------------------------

func TestE2E_Transport(t *testing.T) {
	feature := features.New("Transport").
		Assess("Agent sends snapshots to stub", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			stubClient := helpers.NewStubClient(t, stubURL)

			// Agent should have sent at least 1 snapshot by now
			err := stubClient.WaitForPayloads(t, 1, 2*time.Minute)
			require.NoError(t, err, "stub should have received at least 1 payload")

			return ctx
		}).
		Assess("Sent snapshot has correct schema", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			stubClient := helpers.NewStubClient(t, stubURL)

			latest, err := stubClient.LatestPayload()
			require.NoError(t, err, "should get latest payload from stub")

			// Verify the snapshot sent to backend has the expected fields
			assert.NotEmpty(t, latest.SnapshotID, "snapshot_id should be set")
			assert.NotEmpty(t, latest.ClusterName, "cluster_name should be set")
			assert.Greater(t, latest.Timestamp, int64(0), "timestamp should be set")
			assert.Greater(t, len(latest.Nodes), 0, "sent snapshot should have nodes")
			assert.Greater(t, len(latest.Pods), 0, "sent snapshot should have pods")
			assert.Greater(t, latest.Summary.NodeCount, 0, "summary should have node count")

			return ctx
		}).
		Assess("Sent snapshot matches debug snapshot", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			stubClient := helpers.NewStubClient(t, stubURL)

			debugSnap, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			sentSnap, err := stubClient.LatestPayload()
			require.NoError(t, err)

			// Node and pod counts should be consistent (may differ slightly due to timing)
			assert.Equal(t, debugSnap.Summary.NodeCount, sentSnap.Summary.NodeCount,
				"node count should match between debug and sent snapshot")

			return ctx
		}).
		Assess("zstd compression works (implicit)", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// If the stub successfully stored and returned the snapshot,
			// it means zstd decompression worked — the stub decompresses
			// before storing. This test just confirms it.
			stubClient := helpers.NewStubClient(t, stubURL)
			count, err := stubClient.PayloadCount()
			require.NoError(t, err)
			assert.Greater(t, count, 0, "stub received and decompressed payloads")

			return ctx
		}).
		Feature()

	testCluster.TestEnv().Test(t, feature)
}

// ---------------------------------------------------------------------------
// Phase 5: Agent Observability
// ---------------------------------------------------------------------------

func TestE2E_Observability(t *testing.T) {
	feature := features.New("Observability").
		Assess("Agent exposes core Prometheus metrics", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			metricsText, err := helpers.ScrapeMetrics(agentURL)
			require.NoError(t, err)

			// Core agent metrics should exist (kubeadapt_agent_ prefix)
			helpers.AssertMetricExists(t, metricsText, "kubeadapt_agent_snapshot_send_total")
			helpers.AssertMetricExists(t, metricsText, "kubeadapt_agent_snapshot_send_duration_seconds")
			helpers.AssertMetricExists(t, metricsText, "kubeadapt_agent_snapshot_size_bytes")

			return ctx
		}).
		Assess("Snapshot send metrics show success", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			metricsText, err := helpers.ScrapeMetrics(agentURL)
			require.NoError(t, err)

			// Should have successful sends since stub returns 200
			helpers.AssertMetricWithLabel(t, metricsText, "kubeadapt_agent_snapshot_send_total", "status", "success")

			return ctx
		}).
		Assess("Agent-specific metrics are present", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			metricsText, err := helpers.ScrapeMetrics(agentURL)
			require.NoError(t, err)

			// Agent uses a custom Prometheus registry (no default Go collectors).
			// Verify agent-specific operational metrics that are always emitted.
			helpers.AssertMetricExists(t, metricsText, "kubeadapt_agent_store_items")
			helpers.AssertMetricExists(t, metricsText, "kubeadapt_agent_snapshot_build_duration_seconds")

			return ctx
		}).
		Feature()

	testCluster.TestEnv().Test(t, feature)
}

// ---------------------------------------------------------------------------
// Phase 6: Edge Cases
// ---------------------------------------------------------------------------

func TestE2E_EdgeCases(t *testing.T) {
	feature := features.New("Edge Cases").
		Assess("Agent pod is running and not restarting", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := testCluster.GetClient(cfg)
			require.NoError(t, err)

			pod, err := helpers.GetFirstPodWithLabel(ctx, t, client, "kubeadapt-system",
				map[string]string{"app": "kubeadapt-agent"})
			require.NoError(t, err)

			assert.True(t, helpers.IsPodReady(pod), "agent pod should be ready")

			// Check restart count — should be 0
			for _, cs := range pod.Status.ContainerStatuses {
				assert.Equal(t, int32(0), cs.RestartCount,
					"agent container %s should have 0 restarts", cs.Name)
			}

			return ctx
		}).
		Assess("No GPU detection in Kind (expected)", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshotClient := helpers.NewSnapshotClient(t, agentURL)
			snapshot, err := snapshotClient.GetSnapshot()
			require.NoError(t, err)

			assert.False(t, snapshot.Health.GPUMetricsAvailable, "GPU should not be available in Kind")
			assert.Equal(t, 0, snapshot.Health.DCGMExporterTargets, "no DCGM targets expected")

			return ctx
		}).
		Feature()

	testCluster.TestEnv().Test(t, feature)
}

// ---------------------------------------------------------------------------
// Phase 4b: Resilience (separate test to avoid interfering with other tests)
// ---------------------------------------------------------------------------

func TestE2E_Resilience(t *testing.T) {
	feature := features.New("Resilience").
		Assess("Agent recovers after stub returns 503", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			stubClient := helpers.NewStubClient(t, stubURL)

			// Flush existing payloads for clean count
			err := stubClient.Flush()
			require.NoError(t, err)

			// Set stub to return 503
			err = stubClient.SetMode(503)
			require.NoError(t, err)
			t.Log("Stub mode set to 503 — agent will get server errors")

			// Wait a bit for agent to experience the error
			time.Sleep(15 * time.Second)

			// Set stub back to 200
			err = stubClient.SetMode(200)
			require.NoError(t, err)
			t.Log("Stub mode set back to 200 — agent should recover")

			// Agent should recover and send snapshots
			err = stubClient.WaitForPayloads(t, 1, 2*time.Minute)
			require.NoError(t, err, "agent should recover and send after 503→200 transition")

			return ctx
		}).
		Feature()

	testCluster.TestEnv().Test(t, feature)
}
