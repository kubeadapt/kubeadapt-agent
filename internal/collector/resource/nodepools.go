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

	"github.com/kubeadapt/kubeadapt-agent/internal/observability"
	"github.com/kubeadapt/kubeadapt-agent/internal/store"
	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

var nodePoolGVR = schema.GroupVersionResource{
	Group:    "karpenter.sh",
	Version:  "v1",
	Resource: "nodepools",
}

// NodePoolCollector watches Karpenter NodePool CRD objects via a dynamic SharedInformer
// and writes model.NodePoolInfo to the store on every add/update/delete event.
type NodePoolCollector struct {
	dynamicClient dynamic.Interface
	store         *store.Store
	metrics       *observability.Metrics
	informer      cache.SharedIndexInformer
	stopCh        chan struct{}
	done          chan struct{}
	stopOnce      sync.Once
	resyncPeriod  time.Duration
}

// NewNodePoolCollector creates a new NodePoolCollector.
func NewNodePoolCollector(dynamicClient dynamic.Interface, s *store.Store, m *observability.Metrics, resyncPeriod time.Duration) *NodePoolCollector {
	return &NodePoolCollector{
		dynamicClient: dynamicClient,
		store:         s,
		metrics:       m,
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		resyncPeriod:  resyncPeriod,
	}
}

// Name returns the collector name.
func (c *NodePoolCollector) Name() string { return "nodepools" }

// Start registers event handlers and begins the informer.
func (c *NodePoolCollector) Start(_ context.Context) error {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(c.dynamicClient, c.resyncPeriod)
	c.informer = factory.ForResource(nodePoolGVR).Informer()

	if _, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			info := nodePoolToModel(u)
			c.store.NodePools.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("nodepools", "add").Inc()
			c.metrics.StoreItems.WithLabelValues("nodepools").Set(float64(c.store.NodePools.Len()))
		},
		UpdateFunc: func(_, newObj interface{}) {
			u, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			info := nodePoolToModel(u)
			c.store.NodePools.Set(info.Name, info)
			c.metrics.InformerEventsTotal.WithLabelValues("nodepools", "update").Inc()
			c.metrics.StoreItems.WithLabelValues("nodepools").Set(float64(c.store.NodePools.Len()))
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
			c.store.NodePools.Delete(u.GetName())
			c.metrics.InformerEventsTotal.WithLabelValues("nodepools", "delete").Inc()
			c.metrics.StoreItems.WithLabelValues("nodepools").Set(float64(c.store.NodePools.Len()))
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
func (c *NodePoolCollector) WaitForSync(ctx context.Context) error {
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return fmt.Errorf("nodepools informer cache sync failed")
	}
	return nil
}

// Stop signals the collector to stop and waits for the goroutine to exit.
func (c *NodePoolCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	<-c.done
}

// nodePoolToModel converts an unstructured Karpenter NodePool to model.NodePoolInfo.
func nodePoolToModel(obj *unstructured.Unstructured) model.NodePoolInfo {
	info := model.NodePoolInfo{
		Name:              obj.GetName(),
		Labels:            obj.GetLabels(),
		Annotations:       obj.GetAnnotations(),
		CreationTimestamp: obj.GetCreationTimestamp().UnixMilli(),
	}

	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return info
	}

	// spec.template.spec.nodeClassRef.name
	if tmpl, ok := spec["template"].(map[string]interface{}); ok {
		if tmplSpec, ok := tmpl["spec"].(map[string]interface{}); ok {
			if ncRef, ok := tmplSpec["nodeClassRef"].(map[string]interface{}); ok {
				info.NodeClassName, _ = ncRef["name"].(string)
			}

			// spec.template.spec.requirements
			if reqs, ok := tmplSpec["requirements"].([]interface{}); ok {
				for _, r := range reqs {
					rm, ok := r.(map[string]interface{})
					if !ok {
						continue
					}
					req := model.NodeSelectorRequirement{
						Key:      stringField(rm, "key"),
						Operator: stringField(rm, "operator"),
					}
					if vals, ok := rm["values"].([]interface{}); ok {
						for _, v := range vals {
							if s, ok := v.(string); ok {
								req.Values = append(req.Values, s)
							}
						}
					}
					info.Requirements = append(info.Requirements, req)
				}
			}

			// spec.template.spec.taints
			if taints, ok := tmplSpec["taints"].([]interface{}); ok {
				for _, t := range taints {
					tm, ok := t.(map[string]interface{})
					if !ok {
						continue
					}
					info.Taints = append(info.Taints, model.TaintInfo{
						Key:    stringField(tm, "key"),
						Value:  stringField(tm, "value"),
						Effect: stringField(tm, "effect"),
					})
				}
			}
		}
	}

	// spec.limits — extract min/max replicas if present
	if limits, ok := spec["limits"].(map[string]interface{}); ok {
		if maxVal, ok := limits["maxReplicas"]; ok {
			if v, ok := toInt(maxVal); ok {
				info.MaxReplicas = &v
			}
		}
		if minVal, ok := limits["minReplicas"]; ok {
			if v, ok := toInt(minVal); ok {
				info.MinReplicas = &v
			}
		}
	}

	return info
}

func stringField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}
