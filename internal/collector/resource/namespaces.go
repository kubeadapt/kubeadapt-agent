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

// NamespaceCollector watches Kubernetes Namespace objects via a SharedInformer
// and writes model.NamespaceInfo to the store on every add/update/delete event.
type NamespaceCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewNamespaceCollector creates a new NamespaceCollector.
func NewNamespaceCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *NamespaceCollector {
	return &NamespaceCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *NamespaceCollector) Name() string { return "namespaces" }

// Start registers event handlers and begins the informer.
func (c *NamespaceCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Core().V1().Namespaces().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				return
			}
			info := convert.NamespaceToModel(ns)
			c.store.Namespaces.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("namespaces", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("namespaces").Set(float64(c.store.Namespaces.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			ns, ok := newObj.(*corev1.Namespace)
			if !ok {
				return
			}
			info := convert.NamespaceToModel(ns)
			c.store.Namespaces.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("namespaces", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("namespaces").Set(float64(c.store.Namespaces.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				ns, ok = tombstone.Obj.(*corev1.Namespace)
				if !ok {
					return
				}
			}
			c.store.Namespaces.Delete(ns.Name)
			c.metrics.InformerEventsTotal.WithLabelValues("namespaces", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("namespaces").Set(float64(c.store.Namespaces.Len()))
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
func (c *NamespaceCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("namespaces informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *NamespaceCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
