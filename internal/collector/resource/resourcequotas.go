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

// ResourceQuotaCollector watches Kubernetes ResourceQuota objects via a SharedInformer
// and writes model.ResourceQuotaInfo to the store on every add/update/delete event.
type ResourceQuotaCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewResourceQuotaCollector creates a new ResourceQuotaCollector.
func NewResourceQuotaCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *ResourceQuotaCollector {
	return &ResourceQuotaCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *ResourceQuotaCollector) Name() string { return "resourcequotas" }

// Start registers event handlers and begins the informer.
func (c *ResourceQuotaCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Core().V1().ResourceQuotas().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rq, ok := obj.(*corev1.ResourceQuota)
			if !ok {
				return
			}
			info := convert.ResourceQuotaToModel(rq)
			c.store.ResourceQuotas.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("resourcequotas", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("resourcequotas").Set(float64(c.store.ResourceQuotas.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			rq, ok := newObj.(*corev1.ResourceQuota)
			if !ok {
				return
			}
			info := convert.ResourceQuotaToModel(rq)
			c.store.ResourceQuotas.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("resourcequotas", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("resourcequotas").Set(float64(c.store.ResourceQuotas.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			rq, ok := obj.(*corev1.ResourceQuota)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				rq, ok = tombstone.Obj.(*corev1.ResourceQuota)
				if !ok {
					return
				}
			}
			c.store.ResourceQuotas.Delete(nsNameKey(rq.Namespace, rq.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("resourcequotas", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("resourcequotas").Set(float64(c.store.ResourceQuotas.Len()))
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
func (c *ResourceQuotaCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("resourcequotas informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *ResourceQuotaCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
