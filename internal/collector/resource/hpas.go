package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

// HPACollector watches Kubernetes HorizontalPodAutoscaler (v2) objects via a SharedInformer
// and writes model.HPAInfo to the store on every add/update/delete event.
type HPACollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewHPACollector creates a new HPACollector.
func NewHPACollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *HPACollector {
	return &HPACollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *HPACollector) Name() string { return "hpas" }

// Start registers event handlers and begins the informer.
func (c *HPACollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Autoscaling().V2().HorizontalPodAutoscalers().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			hpa, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
			if !ok {
				return
			}
			info := convert.HPAToModel(hpa)
			c.store.HPAs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("hpas", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("hpas").Set(float64(c.store.HPAs.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			hpa, ok := newObj.(*autoscalingv2.HorizontalPodAutoscaler)
			if !ok {
				return
			}
			info := convert.HPAToModel(hpa)
			c.store.HPAs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("hpas", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("hpas").Set(float64(c.store.HPAs.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			hpa, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				hpa, ok = tombstone.Obj.(*autoscalingv2.HorizontalPodAutoscaler)
				if !ok {
					return
				}
			}
			c.store.HPAs.Delete(nsNameKey(hpa.Namespace, hpa.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("hpas", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("hpas").Set(float64(c.store.HPAs.Len()))
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
func (c *HPACollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("hpas informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *HPACollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
