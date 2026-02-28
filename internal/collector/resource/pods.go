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

// PodCollector watches Kubernetes Pod objects via a SharedInformer
// and writes model.PodInfo to the store on every add/update/delete event.
type PodCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewPodCollector creates a new PodCollector.
func NewPodCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *PodCollector {
	return &PodCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *PodCollector) Name() string { return "pods" }

// podKey returns the store key for a pod: "namespace/name".
func podKey(namespace, name string) string {
	return namespace + "/" + name
}

// Start registers event handlers and begins the informer.
func (c *PodCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Core().V1().Pods().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}
			info := convert.PodToModel(pod)
			c.store.Pods.Set(podKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("pods", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("pods").Set(float64(c.store.Pods.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			pod, ok := newObj.(*corev1.Pod)
			if !ok {
				return
			}
			info := convert.PodToModel(pod)
			c.store.Pods.Set(podKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("pods", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("pods").Set(float64(c.store.Pods.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				pod, ok = tombstone.Obj.(*corev1.Pod)
				if !ok {
					return
				}
			}
			c.store.Pods.Delete(podKey(pod.Namespace, pod.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("pods", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("pods").Set(float64(c.store.Pods.Len()))
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
func (c *PodCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("pods informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *PodCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
