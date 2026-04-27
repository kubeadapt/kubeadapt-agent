package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

// PDBCollector watches Kubernetes PodDisruptionBudget objects via a SharedInformer
// and writes model.PDBInfo to the store on every add/update/delete event.
type PDBCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewPDBCollector creates a new PDBCollector.
func NewPDBCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *PDBCollector {
	return &PDBCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name implements collector.Collector.
func (c *PDBCollector) Name() string { return "pdbs" }

// Start implements collector.Collector.
func (c *PDBCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Policy().V1().PodDisruptionBudgets().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pdb, ok := obj.(*policyv1.PodDisruptionBudget)
			if !ok {
				return
			}
			info := convert.PDBToModel(pdb)
			c.store.PDBs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("pdbs", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("pdbs").Set(float64(c.store.PDBs.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			pdb, ok := newObj.(*policyv1.PodDisruptionBudget)
			if !ok {
				return
			}
			info := convert.PDBToModel(pdb)
			c.store.PDBs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("pdbs", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("pdbs").Set(float64(c.store.PDBs.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			pdb, ok := obj.(*policyv1.PodDisruptionBudget)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				pdb, ok = tombstone.Obj.(*policyv1.PodDisruptionBudget)
				if !ok {
					return
				}
			}
			c.store.PDBs.Delete(nsNameKey(pdb.Namespace, pdb.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("pdbs", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("pdbs").Set(float64(c.store.PDBs.Len()))
		},
	}); err != nil {
		return fmt.Errorf("%s: add event handler: %w", c.Name(), err)
	}

	runInformerWithRecovery(c.informer, c.Name(), c.stopCh, c.done)
	return nil
}

// WaitForSync implements collector.Collector.
func (c *PDBCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("pdbs informer cache sync failed")
	}
	return nil
}

// Stop implements collector.Collector.
func (c *PDBCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}

// IsHealthy implements collector.HealthChecker.
func (c *PDBCollector) IsHealthy() (bool, string) {
	return informerHealthy(c.stopCh, c.done)
}
