package resource

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kubeadapt/kubeadapt-agent/internal/collector"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testResyncPeriod = 0 // no resync in tests
	waitTimeout      = 5 * time.Second
	pollInterval     = 50 * time.Millisecond
)

// testEnv bundles the dependencies shared by every collector test.
type testEnv struct {
	client  *fake.Clientset
	store   *store.Store
	metrics *observability.Metrics
	ctx     context.Context
	cancel  context.CancelFunc
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &testEnv{
		client:  fake.NewSimpleClientset(),
		store:   store.NewStore(),
		metrics: observability.NewMetrics(),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// startCollector is a helper that starts a collector and waits for sync.
func startCollector(t *testing.T, env *testEnv, c collector.Collector) {
	t.Helper()
	err := c.Start(env.ctx)
	require.NoError(t, err, "Start() should succeed")
	err = c.WaitForSync(env.ctx)
	require.NoError(t, err, "WaitForSync() should succeed")
	t.Cleanup(c.Stop)
}
