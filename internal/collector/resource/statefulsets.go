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

// StatefulSetCollector watches Kubernetes StatefulSet objects via a SharedInformer
// and writes model.StatefulSetInfo to the store on every add/update/delete event.
type StatefulSetCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewStatefulSetCollector creates a new StatefulSetCollector.
func NewStatefulSetCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *StatefulSetCollector {
	return &StatefulSetCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *StatefulSetCollector) Name() string { return "statefulsets" }

// Start registers event handlers and begins the informer.
func (c *StatefulSetCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Apps().V1().StatefulSets().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ss, ok := obj.(*appsv1.StatefulSet)
			if !ok {
				return
			}
			info := convert.StatefulSetToModel(ss)
			c.store.StatefulSets.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("statefulsets", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("statefulsets").Set(float64(c.store.StatefulSets.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			ss, ok := newObj.(*appsv1.StatefulSet)
			if !ok {
				return
			}
			info := convert.StatefulSetToModel(ss)
			c.store.StatefulSets.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("statefulsets", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("statefulsets").Set(float64(c.store.StatefulSets.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			ss, ok := obj.(*appsv1.StatefulSet)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				ss, ok = tombstone.Obj.(*appsv1.StatefulSet)
				if !ok {
					return
				}
			}
			c.store.StatefulSets.Delete(nsNameKey(ss.Namespace, ss.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("statefulsets", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("statefulsets").Set(float64(c.store.StatefulSets.Len()))
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
func (c *StatefulSetCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("statefulsets informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *StatefulSetCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
