package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

// PVCollector watches Kubernetes PersistentVolume objects via a SharedInformer
// and writes model.PVInfo to the store on every add/update/delete event.
type PVCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewPVCollector creates a new PVCollector.
func NewPVCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *PVCollector {
	return &PVCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *PVCollector) Name() string { return "pvs" }

// Start registers event handlers and begins the informer.
func (c *PVCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Core().V1().PersistentVolumes().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pv, ok := obj.(*corev1.PersistentVolume)
			if !ok {
				return
			}
			info := convert.PVToModel(pv)
			c.store.PVs.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("pvs", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("pvs").Set(float64(c.store.PVs.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			pv, ok := newObj.(*corev1.PersistentVolume)
			if !ok {
				return
			}
			info := convert.PVToModel(pv)
			c.store.PVs.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("pvs", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("pvs").Set(float64(c.store.PVs.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			pv, ok := obj.(*corev1.PersistentVolume)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				pv, ok = tombstone.Obj.(*corev1.PersistentVolume)
				if !ok {
					return
				}
			}
			c.store.PVs.Delete(pv.Name)
			c.metrics.InformerEventsTotal.WithLabelValues("pvs", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("pvs").Set(float64(c.store.PVs.Len()))
		},
	}); err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	go func() {
		c.informer.Run(c.stopCh)
		close(c.done)
	}()
	return nil
}

// WaitForSync blocks until the informer cache is synced or ctx is canceled.
func (c *PVCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("pvs informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *PVCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
