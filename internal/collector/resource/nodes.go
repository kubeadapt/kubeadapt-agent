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

// NodeCollector watches Kubernetes Node objects via a SharedInformer
// and writes model.NodeInfo to the store on every add/update/delete event.
type NodeCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewNodeCollector creates a new NodeCollector.
func NewNodeCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *NodeCollector {
	return &NodeCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *NodeCollector) Name() string { return "nodes" }

// Start registers event handlers and begins the informer.
func (c *NodeCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Core().V1().Nodes().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node, ok := obj.(*corev1.Node)
			if !ok {
				return
			}
			info := convert.NodeToModel(node)
			c.store.Nodes.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("nodes", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("nodes").Set(float64(c.store.Nodes.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			node, ok := newObj.(*corev1.Node)
			if !ok {
				return
			}
			info := convert.NodeToModel(node)
			c.store.Nodes.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("nodes", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("nodes").Set(float64(c.store.Nodes.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			node, ok := obj.(*corev1.Node)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				node, ok = tombstone.Obj.(*corev1.Node)
				if !ok {
					return
				}
			}
			c.store.Nodes.Delete(node.Name)
			c.metrics.InformerEventsTotal.WithLabelValues("nodes", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("nodes").Set(float64(c.store.Nodes.Len()))
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
func (c *NodeCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("nodes informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *NodeCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
