package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	_ "github.com/KimMachineGun/automemlimit"
	_ "go.uber.org/automaxprocs"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/kubeadapt/kubeadapt-agent/internal/agent"
	"github.com/kubeadapt/kubeadapt-agent/internal/collector"
	"github.com/kubeadapt/kubeadapt-agent/internal/collector/gpu"
	collectormetrics "github.com/kubeadapt/kubeadapt-agent/internal/collector/metrics"
	"github.com/kubeadapt/kubeadapt-agent/internal/collector/resource"
	"github.com/kubeadapt/kubeadapt-agent/internal/config"
	"github.com/kubeadapt/kubeadapt-agent/internal/discovery"
	"github.com/kubeadapt/kubeadapt-agent/internal/enrichment"
	"github.com/kubeadapt/kubeadapt-agent/internal/errors"
	"github.com/kubeadapt/kubeadapt-agent/internal/health"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/snapshot"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
	"github.com/kubeadapt/kubeadapt-agent/internal/transport"
)

func main() {
	// 1. Load and validate config.
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// 2. Create context with signal handling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		slog.Info("shutdown signal received", "signal", sig)
		cancel()
	}()

	slog.Info("kubeadapt-agent starting",
		"version", cfg.AgentVersion,
		"cluster_id", cfg.ClusterID,
		"backend_url", cfg.BackendURL,
		"snapshot_interval", cfg.SnapshotInterval,
	)

	// 3. Create shared infrastructure.
	metrics := observability.NewMetrics()
	errCollector := errors.NewErrorCollector(errors.RealClock{})
	st := store.NewStore()
	ms := store.NewMetricsStore()
	sm := agent.NewStateMachine(errors.RealClock{})

	// 4. Build Kubernetes clients.
	restCfg := buildKubeConfig()
	kubeClient := kubernetes.NewForConfigOrDie(restCfg)
	dynamicClient := dynamic.NewForConfigOrDie(restCfg)
	metricsClient := metricsclientset.NewForConfigOrDie(restCfg)

	// 5. Detect cluster capabilities.
	caps, err := discovery.Detect(ctx, kubeClient, kubeClient.Discovery())
	if err != nil {
		slog.Error("failed to detect cluster capabilities", "error", err)
		os.Exit(1)
	}
	slog.Info("cluster capabilities detected",
		"metrics_server", caps.MetricsServer,
		"vpa", caps.VPA,
		"karpenter", caps.Karpenter,
		"dcgm_exporter", caps.DCGMExporter,
		"provider", caps.Provider,
	)

	// 6. Register collectors.
	registry := collector.NewRegistry()
	resync := cfg.InformerResyncPeriod

	registry.Register(resource.NewNodeCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewPodCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewNamespaceCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewDeploymentCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewStatefulSetCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewDaemonSetCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewReplicaSetCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewJobCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewCronJobCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewHPACollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewPDBCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewServiceCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewIngressCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewPVCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewPVCCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewStorageClassCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewPriorityClassCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewLimitRangeCollector(kubeClient, st, metrics, resync))
	registry.Register(resource.NewResourceQuotaCollector(kubeClient, st, metrics, resync))

	if caps.VPA {
		registry.Register(resource.NewVPACollector(dynamicClient, st, metrics, resync))
	}
	if caps.Karpenter {
		registry.Register(resource.NewNodePoolCollector(dynamicClient, st, metrics, resync))
	}
	if caps.MetricsServer {
		registry.Register(collectormetrics.NewMetricsCollectorFromClient(
			metricsClient.MetricsV1beta1(), ms, metrics, cfg.MetricsInterval,
		))
	}

	// 6b. Conditional GPU collector.
	var gpuProvider snapshot.GPUMetricsProvider
	if (caps.DCGMExporter || len(cfg.DCGMExporterEndpoints) > 0) && cfg.GPUMetricsEnabled {
		gpuClient := gpu.NewDCGMExporterClient(&http.Client{Timeout: 10 * time.Second})

		var endpointsFn func() []string
		if len(cfg.DCGMExporterEndpoints) > 0 {
			// Static endpoints from env override — no refresh needed.
			staticEndpoints := cfg.DCGMExporterEndpoints
			endpointsFn = func() []string {
				urls := make([]string, 0, len(staticEndpoints))
				for _, ip := range staticEndpoints {
					urls = append(urls, fmt.Sprintf("http://%s:%d", ip, cfg.DCGMExporterPort))
				}
				return urls
			}
		} else {
			// Dynamic discovery — re-detect dcgm-exporter pods on each poll.
			endpointsFn = func() []string {
				_, ips := discovery.DetectDCGMEndpoints(ctx, kubeClient)
				urls := make([]string, 0, len(ips))
				for _, ip := range ips {
					urls = append(urls, fmt.Sprintf("http://%s:%d", ip, cfg.DCGMExporterPort))
				}
				return urls
			}
		}

		gpuCollector := gpu.NewGPUMetricsCollector(gpuClient, endpointsFn, cfg.GPUMetricsInterval)
		registry.Register(gpuCollector)
		gpuProvider = gpuCollector
	}

	// 7. Build enrichment pipeline and snapshot builder.
	pipeline := enrichment.NewPipeline(metrics,
		enrichment.NewAggregationEnricher(),
		enrichment.NewTargetsEnricher(),
		enrichment.NewMountsEnricher(),
	)
	builder := snapshot.NewSnapshotBuilder(st, ms, &cfg, metrics, errCollector, pipeline, gpuProvider)

	// 8. Create transport and agent.
	transportClient := transport.NewClient(&cfg, metrics, errCollector)
	ag := agent.NewAgent(&cfg, registry, builder, transportClient, sm, errCollector, metrics)

	// 9. Start health server.
	healthSrv := health.NewServer(cfg.HealthPort, metrics, ag, ag, st, cfg.DebugEndpoints)
	if err := healthSrv.Start(); err != nil {
		slog.Error("failed to start health server", "error", err)
		os.Exit(1)
	}

	// 10. Start memory pressure monitor.
	memMon := agent.NewMemoryPressureMonitor(0.8, func() { runtime.GC() }, 30*time.Second, nil)
	memMon.Start()

	// 11. Run agent (blocks until context is canceled).
	if err := ag.Run(ctx); err != nil && ctx.Err() == nil {
		slog.Error("agent exited with error", "error", err)
	}

	// 12. Graceful shutdown.
	memMon.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := healthSrv.Stop(shutdownCtx); err != nil {
		slog.Error("health server shutdown error", "error", err)
	}

	slog.Info("kubeadapt-agent stopped")
}

// buildKubeConfig creates a Kubernetes REST config.
// It tries in-cluster config first, then falls back to kubeconfig file
// (from $KUBECONFIG or the default ~/.kube/config).
func buildKubeConfig() *rest.Config {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		slog.Info("using in-cluster kubernetes config")
		return cfg
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}

	cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		slog.Error("failed to build kubernetes config", "error", err)
		os.Exit(1)
	}
	slog.Info("using kubeconfig file", "path", kubeconfig)
	return cfg
}
