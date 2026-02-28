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

// Name returns the collector name.
func (c *DeploymentCollector) Name() string { return "deployments" }

// nsNameKey returns the store key: "namespace/name".
func nsNameKey(namespace, name string) string {
	return namespace + "/" + name
}

// Start registers event handlers and begins the informer.
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
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	go func() {
		c.informer.Run(c.stopCh)
		close(c.done)
	}()
	return nil
}

// WaitForSync blocks until the informer cache is synced or ctx is canceled.
func (c *DeploymentCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("deployments informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *DeploymentCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
