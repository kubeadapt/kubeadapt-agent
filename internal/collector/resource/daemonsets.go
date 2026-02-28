package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

// DaemonSetCollector watches Kubernetes DaemonSet objects via a SharedInformer
// and writes model.DaemonSetInfo to the store on every add/update/delete event.
type DaemonSetCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewDaemonSetCollector creates a new DaemonSetCollector.
func NewDaemonSetCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *DaemonSetCollector {
	return &DaemonSetCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *DaemonSetCollector) Name() string { return "daemonsets" }

// Start registers event handlers and begins the informer.
func (c *DaemonSetCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Apps().V1().DaemonSets().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ds, ok := obj.(*appsv1.DaemonSet)
			if !ok {
				return
			}
			info := convert.DaemonSetToModel(ds)
			c.store.DaemonSets.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("daemonsets", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("daemonsets").Set(float64(c.store.DaemonSets.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			ds, ok := newObj.(*appsv1.DaemonSet)
			if !ok {
				return
			}
			info := convert.DaemonSetToModel(ds)
			c.store.DaemonSets.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("daemonsets", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("daemonsets").Set(float64(c.store.DaemonSets.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			ds, ok := obj.(*appsv1.DaemonSet)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				ds, ok = tombstone.Obj.(*appsv1.DaemonSet)
				if !ok {
					return
				}
			}
			c.store.DaemonSets.Delete(nsNameKey(ds.Namespace, ds.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("daemonsets", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("daemonsets").Set(float64(c.store.DaemonSets.Len()))
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
func (c *DaemonSetCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("daemonsets informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *DaemonSetCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
