package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

// IngressCollector watches Kubernetes Ingress objects via a SharedInformer
// and writes model.IngressInfo to the store on every add/update/delete event.
type IngressCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewIngressCollector creates a new IngressCollector.
func NewIngressCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *IngressCollector {
	return &IngressCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *IngressCollector) Name() string { return "ingresses" }

// Start registers event handlers and begins the informer.
func (c *IngressCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Networking().V1().Ingresses().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ing, ok := obj.(*networkingv1.Ingress)
			if !ok {
				return
			}
			info := convert.IngressToModel(ing)
			c.store.Ingresses.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("ingresses", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("ingresses").Set(float64(c.store.Ingresses.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			ing, ok := newObj.(*networkingv1.Ingress)
			if !ok {
				return
			}
			info := convert.IngressToModel(ing)
			c.store.Ingresses.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("ingresses", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("ingresses").Set(float64(c.store.Ingresses.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			ing, ok := obj.(*networkingv1.Ingress)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				ing, ok = tombstone.Obj.(*networkingv1.Ingress)
				if !ok {
					return
				}
			}
			c.store.Ingresses.Delete(nsNameKey(ing.Namespace, ing.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("ingresses", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("ingresses").Set(float64(c.store.Ingresses.Len()))
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
func (c *IngressCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("ingresses informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *IngressCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
