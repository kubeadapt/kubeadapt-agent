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

// ServiceCollector watches Kubernetes Service objects via a SharedInformer
// and writes model.ServiceInfo to the store on every add/update/delete event.
type ServiceCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewServiceCollector creates a new ServiceCollector.
func NewServiceCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *ServiceCollector {
	return &ServiceCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *ServiceCollector) Name() string { return "services" }

// Start registers event handlers and begins the informer.
func (c *ServiceCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Core().V1().Services().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc, ok := obj.(*corev1.Service)
			if !ok {
				return
			}
			info := convert.ServiceToModel(svc)
			c.store.Services.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("services", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("services").Set(float64(c.store.Services.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			svc, ok := newObj.(*corev1.Service)
			if !ok {
				return
			}
			info := convert.ServiceToModel(svc)
			c.store.Services.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("services", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("services").Set(float64(c.store.Services.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			svc, ok := obj.(*corev1.Service)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				svc, ok = tombstone.Obj.(*corev1.Service)
				if !ok {
					return
				}
			}
			c.store.Services.Delete(nsNameKey(svc.Namespace, svc.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("services", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("services").Set(float64(c.store.Services.Len()))
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
func (c *ServiceCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("services informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *ServiceCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
