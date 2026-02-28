package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeadapt/kubeadapt-agent/internal/convert"
	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
)

// JobCollector watches Kubernetes Job objects via a SharedInformer
// and writes model.JobInfo to the store on every add/update/delete event.
type JobCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewJobCollector creates a new JobCollector.
func NewJobCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *JobCollector {
	return &JobCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

// Name returns the collector name.
func (c *JobCollector) Name() string { return "jobs" }

// Start registers event handlers and begins the informer.
func (c *JobCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Batch().V1().Jobs().Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			job, ok := obj.(*batchv1.Job)
			if !ok {
				return
			}
			info := convert.JobToModel(job)
			c.store.Jobs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("jobs", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("jobs").Set(float64(c.store.Jobs.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			job, ok := newObj.(*batchv1.Job)
			if !ok {
				return
			}
			info := convert.JobToModel(job)
			c.store.Jobs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("jobs", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("jobs").Set(float64(c.store.Jobs.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			job, ok := obj.(*batchv1.Job)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				job, ok = tombstone.Obj.(*batchv1.Job)
				if !ok {
					return
				}
			}
			c.store.Jobs.Delete(nsNameKey(job.Namespace, job.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("jobs", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("jobs").Set(float64(c.store.Jobs.Len()))
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
func (c *JobCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("jobs informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *JobCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
