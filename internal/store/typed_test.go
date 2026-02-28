package store

import (
	"fmt"
	"sync"
	"testing"
)

// testItem is a simple struct used across TypedStore tests.
type testItem struct {
	Name  string
	Value int
}

func TestTypedStore_SetGet(t *testing.T) {
	s := NewTypedStore[testItem]()

	item := testItem{Name: "alpha", Value: 42}
	s.Set("key1", item)

	got, ok := s.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if got.Name != "alpha" || got.Value != 42 {
		t.Fatalf("expected {alpha 42}, got %+v", got)
	}

	// Non-existent key
	_, ok = s.Get("missing")
	if ok {
		t.Fatal("expected missing key to return false")
	}
}

func TestTypedStore_Delete(t *testing.T) {
	s := NewTypedStore[testItem]()

	s.Set("key1", testItem{Name: "alpha", Value: 1})
	s.Delete("key1")

	_, ok := s.Get("key1")
	if ok {
		t.Fatal("expected key1 to be deleted")
	}

	// Delete non-existent key should not panic
	s.Delete("nonexistent")
}

func TestTypedStore_Len(t *testing.T) {
	s := NewTypedStore[testItem]()

	s.Set("a", testItem{Name: "a", Value: 1})
	s.Set("b", testItem{Name: "b", Value: 2})
	s.Set("c", testItem{Name: "c", Value: 3})

	if s.Len() != 3 {
		t.Fatalf("expected Len() == 3, got %d", s.Len())
	}

	s.Delete("b")
	if s.Len() != 2 {
		t.Fatalf("expected Len() == 2 after delete, got %d", s.Len())
	}
}

func TestTypedStore_Snapshot(t *testing.T) {
	s := NewTypedStore[testItem]()

	s.Set("a", testItem{Name: "a", Value: 1})
	s.Set("b", testItem{Name: "b", Value: 2})

	snap := s.Snapshot()

	// Verify snapshot contents
	if len(snap) != 2 {
		t.Fatalf("expected snapshot len 2, got %d", len(snap))
	}
	if snap["a"].Value != 1 || snap["b"].Value != 2 {
		t.Fatalf("unexpected snapshot contents: %+v", snap)
	}

	// Mutate the copy — original must be unchanged
	snap["a"] = testItem{Name: "mutated", Value: 999}
	snap["c"] = testItem{Name: "new", Value: 3}

	original, _ := s.Get("a")
	if original.Name != "a" || original.Value != 1 {
		t.Fatal("snapshot mutation affected original store")
	}
	if s.Len() != 2 {
		t.Fatal("snapshot mutation added key to original store")
	}
}

func TestTypedStore_Values(t *testing.T) {
	s := NewTypedStore[testItem]()

	s.Set("a", testItem{Name: "a", Value: 1})
	s.Set("b", testItem{Name: "b", Value: 2})
	s.Set("c", testItem{Name: "c", Value: 3})

	vals := s.Values()
	if len(vals) != 3 {
		t.Fatalf("expected 3 values, got %d", len(vals))
	}

	// Collect values into a map for order-independent comparison
	found := make(map[string]int)
	for _, v := range vals {
		found[v.Name] = v.Value
	}
	for _, name := range []string{"a", "b", "c"} {
		if _, ok := found[name]; !ok {
			t.Fatalf("expected value with Name=%q in Values()", name)
		}
	}
}

func TestTypedStore_Clear(t *testing.T) {
	s := NewTypedStore[testItem]()

	s.Set("a", testItem{Name: "a", Value: 1})
	s.Set("b", testItem{Name: "b", Value: 2})

	s.Clear()

	if s.Len() != 0 {
		t.Fatalf("expected Len() == 0 after Clear(), got %d", s.Len())
	}
	_, ok := s.Get("a")
	if ok {
		t.Fatal("expected key 'a' to not exist after Clear()")
	}
}

func TestTypedStore_ConcurrentReadWrite(t *testing.T) {
	s := NewTypedStore[testItem]()
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 4) // Set + Get + Snapshot + Delete goroutines

	// Concurrent Sets
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			s.Set(key, testItem{Name: key, Value: i})
		}(i)
	}

	// Concurrent Gets
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			s.Get(key) // may or may not find it; just no race
		}(i)
	}

	// Concurrent Snapshots
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = s.Snapshot()
		}()
	}

	// Concurrent Deletes
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			s.Delete(key)
		}(i)
	}

	wg.Wait()
	// If we get here without -race detecting issues, we're good.
}
