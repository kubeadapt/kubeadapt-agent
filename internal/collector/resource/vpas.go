package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

var vpaGVR = schema.GroupVersionResource{
	Group:    "autoscaling.k8s.io",
	Version:  "v1",
	Resource: "verticalpodautoscalers",
}

// VPACollector watches Kubernetes VPA CRD objects via a dynamic SharedInformer
// and writes model.VPAInfo to the store on every add/update/delete event.
type VPACollector struct {
	dynamicClient dynamic.Interface
	store         *store.Store
	metrics       *observability.Metrics
	informer      cache.SharedIndexInformer
	stopCh        chan struct{}
	done          chan struct{}
	stopOnce      sync.Once
	resyncPeriod  time.Duration
}

// NewVPACollector creates a new VPACollector.
func NewVPACollector(dynamicClient dynamic.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *VPACollector {
	return &VPACollector{
		dynamicClient: dynamicClient,
		store:         s,
		metrics:       m,
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		resyncPeriod:  resyncPeriod,
	}
}

// Name returns the collector name.
func (c *VPACollector) Name() string { return "vpas" }

// Start registers event handlers and begins the informer.
func (c *VPACollector) Start(_ context.Context) error {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(c.dynamicClient, c.resyncPeriod)
	c.informer = factory.ForResource(vpaGVR).Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			info := convert.VPAToModel(u)
			c.store.VPAs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("vpas", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("vpas").Set(float64(c.store.VPAs.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			u, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			info := convert.VPAToModel(u)
			c.store.VPAs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("vpas", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("vpas").Set(float64(c.store.VPAs.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				u, ok = tombstone.Obj.(*unstructured.Unstructured)
				if !ok {
					return
				}
			}
			c.store.VPAs.Delete(nsNameKey(u.GetNamespace(), u.GetName()))
			c.metrics.InformerEventsTotal.WithLabelValues("vpas", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("vpas").Set(float64(c.store.VPAs.Len()))
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
func (c *VPACollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("vpas informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *VPACollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
