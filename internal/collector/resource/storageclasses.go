package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

// StorageClassCollector watches Kubernetes StorageClass objects via a SharedInformer
// and writes model.StorageClassInfo to the store on every add/update/delete event.
type StorageClassCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewStorageClassCollector creates a new StorageClassCollector.
func NewStorageClassCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *StorageClassCollector {
	return &StorageClassCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *StorageClassCollector) Name() string { return "storageclasses" }

// Start registers event handlers and begins the informer.
func (c *StorageClassCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Storage().V1().StorageClasses().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sc, ok := obj.(*storagev1.StorageClass)
			if !ok {
				return
			}
			info := convert.StorageClassToModel(sc)
			c.store.StorageClasses.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("storageclasses", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("storageclasses").Set(float64(c.store.StorageClasses.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			sc, ok := newObj.(*storagev1.StorageClass)
			if !ok {
				return
			}
			info := convert.StorageClassToModel(sc)
			c.store.StorageClasses.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("storageclasses", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("storageclasses").Set(float64(c.store.StorageClasses.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			sc, ok := obj.(*storagev1.StorageClass)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				sc, ok = tombstone.Obj.(*storagev1.StorageClass)
				if !ok {
					return
				}
			}
			c.store.StorageClasses.Delete(sc.Name)
			c.metrics.InformerEventsTotal.WithLabelValues("storageclasses", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("storageclasses").Set(float64(c.store.StorageClasses.Len()))
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
func (c *StorageClassCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("storageclasses informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *StorageClassCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
