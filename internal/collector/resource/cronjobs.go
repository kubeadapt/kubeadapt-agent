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

// CronJobCollector watches Kubernetes CronJob objects via a SharedInformer
// and writes model.CronJobInfo to the store on every add/update/delete event.
type CronJobCollector struct {
	client       kubernetes.Interface
	store        *store.Store
	metrics      *observability.Metrics
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

// NewCronJobCollector creates a new CronJobCollector.
func NewCronJobCollector(client kubernetes.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *CronJobCollector {
	return &CronJobCollector{
		client:       client,
		store:        s,
		metrics:      m,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
		resyncPeriod: resyncPeriod,
	}
}

func (c *CronJobCollector) Name() string { return "cronjobs" }

func (c *CronJobCollector) Start(_ context.Context) error {
	factory := informers.NewSharedInformerFactory(c.client, c.resyncPeriod)
	c.informer = factory.Batch().V1().CronJobs().Informer()

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cj, ok := obj.(*batchv1.CronJob)
			if !ok {
				return
			}
			info := convert.CronJobToModel(cj)
			c.store.CronJobs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("cronjobs", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("cronjobs").Set(float64(c.store.CronJobs.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			cj, ok := newObj.(*batchv1.CronJob)
			if !ok {
				return
			}
			info := convert.CronJobToModel(cj)
			c.store.CronJobs.Set(nsNameKey(info.Namespace, info.Name), info)
			c.metrics.InformerEventsTotal.WithLabelValues("cronjobs", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("cronjobs").Set(float64(c.store.CronJobs.Len()))
		},
		DeleteFunc: func(obj interface{}) {
			cj, ok := obj.(*batchv1.CronJob)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				cj, ok = tombstone.Obj.(*batchv1.CronJob)
				if !ok {
					return
				}
			}
			c.store.CronJobs.Delete(nsNameKey(cj.Namespace, cj.Name))
			c.metrics.InformerEventsTotal.WithLabelValues("cronjobs", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("cronjobs").Set(float64(c.store.CronJobs.Len()))
		},
	})

	go func() {
		c.informer.Run(c.stopCh)
		close(c.done)
	}()
	return nil
}

func (c *CronJobCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("cronjobs informer cache sync failed")
	}
	return nil
}

func (c *CronJobCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}
