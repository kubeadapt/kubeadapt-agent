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

// DeploymentCollector watches Kubernetes Deployment objects via a SharedInformer
// and writes model.DeploymentInfo to the store on every add/update/delete event.
type DeploymentCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewDeploymentCollector creates a new DeploymentCollector.
func NewDeploymentCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *DeploymentCollector {
	return &DeploymentCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name implements collector.Collector.
func (c *DeploymentCollector) Name() string { return "deployments" }

// nsNameKey returns the store key: "namespace/name".
func nsNameKey(namespace, name string) string {
	return namespace + "/" + name
}

// Start implements collector.Collector.
func (c *DeploymentCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Apps().V1().Deployments().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			dep, ok := obj.(*appsv1.Deployment)
			if !ok {
				return
			}
			info := convert.DeploymentToModel(dep)
			c.store.Deployments.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("deployments", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("deployments").Set(float64(c.store.Deployments.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			dep, ok := newObj.(*appsv1.Deployment)
			if !ok {
				return
			}
			info := convert.DeploymentToModel(dep)
			c.store.Deployments.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("deployments", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("deployments").Set(float64(c.store.Deployments.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			dep, ok := obj.(*appsv1.Deployment)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				dep, ok = tombstone.Obj.(*appsv1.Deployment)
				if !ok {
					return
				}
			}
			c.store.Deployments.Delete(nsNameKey(dep.Namespace, dep.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("deployments", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("deployments").Set(float64(c.store.Deployments.Len()))
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
func (c *DeploymentCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("deployments informer cache sync failed")
	}
	return nil
}

// Stop implements collector.Collector.
func (c *DeploymentCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
