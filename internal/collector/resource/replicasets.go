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

// ReplicaSetCollector watches Kubernetes ReplicaSet objects via a SharedInformer
// and writes model.ReplicaSetInfo to the store on every add/update/delete event.
// ReplicaSets are primarily used for ownership resolution of pods to deployments.
type ReplicaSetCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewReplicaSetCollector creates a new ReplicaSetCollector.
func NewReplicaSetCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *ReplicaSetCollector {
	return &ReplicaSetCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name implements collector.Collector.
func (c *ReplicaSetCollector) Name() string { return "replicasets" }

// Start implements collector.Collector.
func (c *ReplicaSetCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Apps().V1().ReplicaSets().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rs, ok := obj.(*appsv1.ReplicaSet)
			if !ok {
				return
			}
			info := convert.ReplicaSetToModel(rs)
			c.store.ReplicaSets.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("replicasets", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("replicasets").Set(float64(c.store.ReplicaSets.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			rs, ok := newObj.(*appsv1.ReplicaSet)
			if !ok {
				return
			}
			info := convert.ReplicaSetToModel(rs)
			c.store.ReplicaSets.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("replicasets", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("replicasets").Set(float64(c.store.ReplicaSets.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			rs, ok := obj.(*appsv1.ReplicaSet)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				rs, ok = tombstone.Obj.(*appsv1.ReplicaSet)
				if !ok {
					return
				}
			}
			c.store.ReplicaSets.Delete(nsNameKey(rs.Namespace, rs.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("replicasets", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("replicasets").Set(float64(c.store.ReplicaSets.Len()))
		},
	}); err != nil {
		return fmt.Errorf("%s: add event handler: %w", c.Name(), err)
	}

	go func() {
		c.informer.Run(c.stopCh)
		close(c.done)
	}()
	return nil
}

// WaitForSync implements collector.Collector.
func (c *ReplicaSetCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("replicasets informer cache sync failed")
	}
	return nil
}

// Stop implements collector.Collector.
func (c *ReplicaSetCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
