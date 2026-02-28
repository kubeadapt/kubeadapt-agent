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

// PVCCollector watches Kubernetes PersistentVolumeClaim objects via a SharedInformer
// and writes model.PVCInfo to the store on every add/update/delete event.
type PVCCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewPVCCollector creates a new PVCCollector.
func NewPVCCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *PVCCollector {
	return &PVCCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *PVCCollector) Name() string { return "pvcs" }

// Start registers event handlers and begins the informer.
func (c *PVCCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Core().V1().PersistentVolumeClaims().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pvc, ok := obj.(*corev1.PersistentVolumeClaim)
			if !ok {
				return
			}
			info := convert.PVCToModel(pvc)
			c.store.PVCs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("pvcs", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("pvcs").Set(float64(c.store.PVCs.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			pvc, ok := newObj.(*corev1.PersistentVolumeClaim)
			if !ok {
				return
			}
			info := convert.PVCToModel(pvc)
			c.store.PVCs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("pvcs", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("pvcs").Set(float64(c.store.PVCs.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			pvc, ok := obj.(*corev1.PersistentVolumeClaim)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				pvc, ok = tombstone.Obj.(*corev1.PersistentVolumeClaim)
				if !ok {
					return
				}
			}
			c.store.PVCs.Delete(nsNameKey(pvc.Namespace, pvc.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("pvcs", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("pvcs").Set(float64(c.store.PVCs.Len()))
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
func (c *PVCCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("pvcs informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *PVCCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
