package store

import (
	"reflect"
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

func TestNewStore(t *testing.T) {
	s := NewStore()

	// Use reflection to verify all 22 fields are non-nil TypedStore pointers.
	v := reflect.ValueOf(s).Elem()
	typ := v.Type()

	if typ.NumField() != 22 {
		t.Fatalf("expected Store to have 22 fields, got %d", typ.NumField())
	}

	for i := 0; i < typ.NumField(); i++ {
		field := v.Field(i)
		if field.IsNil() {
			t.Errorf("Store.%s is nil, expected initialized TypedStore", typ.Field(i).Name)
		}
	}
}

func TestNewStore_BasicOperations(t *testing.T) {
	s := NewStore()

	// Test that we can use the Nodes store
	node := model.NodeInfo{Name: "node-1", Ready: true}
	s.Nodes.Set("node-1", node)

	got, ok := s.Nodes.Get("node-1")
	if !ok {
		t.Fatal("expected node-1 to exist")
	}
	if got.Name != "node-1" || !got.Ready {
		t.Fatalf("unexpected node: %+v", got)
	}

	// Test that we can use the Pods store
	pod := model.PodInfo{Name: "pod-1", Namespace: "default"}
	s.Pods.Set("default/pod-1", pod)

	gotPod, ok := s.Pods.Get("default/pod-1")
	if !ok {
		t.Fatal("expected pod to exist")
	}
	if gotPod.Name != "pod-1" || gotPod.Namespace != "default" {
		t.Fatalf("unexpected pod: %+v", gotPod)
	}
}

func TestNewMetricsStore(t *testing.T) {
	ms := NewMetricsStore()

	if ms.NodeMetrics == nil {
		t.Error("MetricsStore.NodeMetrics is nil")
	}
	if ms.PodMetrics == nil {
		t.Error("MetricsStore.PodMetrics is nil")
	}

	// Basic operation
	nm := model.NodeMetrics{Name: "node-1", CPUUsageCores: 2.5, MemoryUsageBytes: 1024}
	ms.NodeMetrics.Set("node-1", nm)

	got, ok := ms.NodeMetrics.Get("node-1")
	if !ok {
		t.Fatal("expected node metrics to exist")
	}
	if got.CPUUsageCores != 2.5 {
		t.Fatalf("unexpected CPU usage: %f", got.CPUUsageCores)
	}
}
