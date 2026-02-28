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

// LimitRangeCollector watches Kubernetes LimitRange objects via a SharedInformer
// and writes model.LimitRangeInfo to the store on every add/update/delete event.
type LimitRangeCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewLimitRangeCollector creates a new LimitRangeCollector.
func NewLimitRangeCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *LimitRangeCollector {
	return &LimitRangeCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *LimitRangeCollector) Name() string { return "limitranges" }

// Start registers event handlers and begins the informer.
func (c *LimitRangeCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Core().V1().LimitRanges().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			lr, ok := obj.(*corev1.LimitRange)
			if !ok {
				return
			}
			info := convert.LimitRangeToModel(lr)
			c.store.LimitRanges.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("limitranges", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("limitranges").Set(float64(c.store.LimitRanges.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			lr, ok := newObj.(*corev1.LimitRange)
			if !ok {
				return
			}
			info := convert.LimitRangeToModel(lr)
			c.store.LimitRanges.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("limitranges", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("limitranges").Set(float64(c.store.LimitRanges.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			lr, ok := obj.(*corev1.LimitRange)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				lr, ok = tombstone.Obj.(*corev1.LimitRange)
				if !ok {
					return
				}
			}
			c.store.LimitRanges.Delete(nsNameKey(lr.Namespace, lr.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("limitranges", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("limitranges").Set(float64(c.store.LimitRanges.Len()))
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
func (c *LimitRangeCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("limitranges informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *LimitRangeCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
