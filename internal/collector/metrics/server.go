package metrics

import (
	"context"
	"log/slog"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv1beta1client "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// MetricsAPI abstracts the metrics-server API for testability.
type MetricsAPI interface {
	ListNodeMetrics(ctx context.Context) ([]metricsv1beta1.NodeMetrics, error)
	ListPodMetrics(ctx context.Context) ([]metricsv1beta1.PodMetrics, error)
}

// metricsAPIClient wraps the real metrics client to implement MetricsAPI.
type metricsAPIClient struct {
	client metricsv1beta1client.MetricsV1beta1Interface
}

func (c *metricsAPIClient) ListNodeMetrics(ctx context.Context) ([]metricsv1beta1.NodeMetrics, error) {
	list, err := c.client.NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *metricsAPIClient) ListPodMetrics(ctx context.Context) ([]metricsv1beta1.PodMetrics, error) {
	list, err := c.client.PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// MetricsCollector polls the metrics-server API on a timer and stores
// node and pod resource usage data.
type MetricsCollector struct {
	api          MetricsAPI
	metricsStore *store.MetricsStore
	metrics      *observability.Metrics
	interval     time.Duration
	stopCh       chan struct{}
	done         chan struct{}

	syncOnce sync.Once
	synced   chan struct{}
}

// NewMetricsCollector creates a MetricsCollector that polls using the given MetricsAPI.
func NewMetricsCollector(api MetricsAPI, metricsStore *store.MetricsStore, metrics *observability.Metrics, interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		api:          api,
		metricsStore: metricsStore,
		metrics:      metrics,
		interval:     interval,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		synced:       make(chan struct{}),
	}
}

// NewMetricsCollectorFromClient creates a MetricsCollector using a real metrics-server client.
func NewMetricsCollectorFromClient(client metricsv1beta1client.MetricsV1beta1Interface, metricsStore *store.MetricsStore, metrics *observability.Metrics, interval time.Duration) *MetricsCollector {
	return NewMetricsCollector(&metricsAPIClient{client: client}, metricsStore, metrics, interval)
}

// Name returns the collector name.
func (c *MetricsCollector) Name() string { return "metrics" }

// Start launches the background polling goroutine.
func (c *MetricsCollector) Start(ctx context.Context) error {
	go c.run(ctx)
	return nil
}

// WaitForSync blocks until the first metrics poll completes or ctx is canceled.
func (c *MetricsCollector) WaitForSync(ctx context.Context) error {
	select {
	case <-c.synced:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *MetricsCollector) Stop() {
	close(c.stopCh)
	<-c.done
}

func (c *MetricsCollector) run(ctx context.Context) {
	defer close(c.done)

	// Poll immediately on start.
	c.poll(ctx)
	c.syncOnce.Do(func() { close(c.synced) })

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.poll(ctx)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *MetricsCollector) poll(ctx context.Context) {
	start := time.Now()

	c.pollNodeMetrics(ctx)
	c.pollPodMetrics(ctx)

	c.metrics.MetricsAPIDuration.Observe(time.Since(start).Seconds())
}

func (c *MetricsCollector) pollNodeMetrics(ctx context.Context) {
	nodeMetricsList, err := c.api.ListNodeMetrics(ctx)
	if err != nil {
		slog.Error("failed to list node metrics", "error", err)
		return
	}

	for _, nm := range nodeMetricsList {
		memQ := nm.Usage["memory"]
		c.metricsStore.NodeMetrics.Set(nm.Name, model.NodeMetrics{
			Name:             nm.Name,
			CPUUsageCores:    convert.ParseQuantity(nm.Usage["cpu"]),
			MemoryUsageBytes: memQ.Value(),
			Timestamp:        nm.Timestamp.UnixMilli(),
		})
	}
}

func (c *MetricsCollector) pollPodMetrics(ctx context.Context) {
	podMetricsList, err := c.api.ListPodMetrics(ctx)
	if err != nil {
		slog.Error("failed to list pod metrics", "error", err)
		return
	}

	for _, pm := range podMetricsList {
		containers := make([]model.ContainerMetrics, 0, len(pm.Containers))
		for _, cm := range pm.Containers {
			memQ := cm.Usage["memory"]
			containers = append(containers, model.ContainerMetrics{
				Name:             cm.Name,
				CPUUsageCores:    convert.ParseQuantity(cm.Usage["cpu"]),
				MemoryUsageBytes: memQ.Value(),
			})
		}

		key := pm.Namespace + "/" + pm.Name
		c.metricsStore.PodMetrics.Set(key, model.PodMetrics{
			Name:       pm.Name,
			Namespace:  pm.Namespace,
			Containers: containers,
			Timestamp:  pm.Timestamp.UnixMilli(),
		})
	}
}
