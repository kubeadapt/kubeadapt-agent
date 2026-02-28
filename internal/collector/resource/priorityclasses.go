package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

// PriorityClassCollector watches Kubernetes PriorityClass objects via a SharedInformer
// and writes model.PriorityClassInfo to the store on every add/update/delete event.
type PriorityClassCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewPriorityClassCollector creates a new PriorityClassCollector.
func NewPriorityClassCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *PriorityClassCollector {
	return &PriorityClassCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *PriorityClassCollector) Name() string { return "priorityclasses" }

// Start registers event handlers and begins the informer.
func (c *PriorityClassCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Scheduling().V1().PriorityClasses().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pc, ok := obj.(*schedulingv1.PriorityClass)
			if !ok {
				return
			}
			info := convert.PriorityClassToModel(pc)
			c.store.PriorityClasses.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("priorityclasses", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("priorityclasses").Set(float64(c.store.PriorityClasses.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			pc, ok := newObj.(*schedulingv1.PriorityClass)
			if !ok {
				return
			}
			info := convert.PriorityClassToModel(pc)
			c.store.PriorityClasses.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("priorityclasses", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("priorityclasses").Set(float64(c.store.PriorityClasses.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			pc, ok := obj.(*schedulingv1.PriorityClass)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				pc, ok = tombstone.Obj.(*schedulingv1.PriorityClass)
				if !ok {
					return
				}
			}
			c.store.PriorityClasses.Delete(pc.Name)
			c.metrics.InformerEventsTotal.WithLabelValues("priorityclasses", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("priorityclasses").Set(float64(c.store.PriorityClasses.Len()))
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
func (c *PriorityClassCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("priorityclasses informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *PriorityClassCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
