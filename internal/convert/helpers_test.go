package convert

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestFilterAnnotations_Nil(t *testing.T) {
	got := FilterAnnotations(nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestFilterAnnotations_Normal(t *testing.T) {
	in := map[string]string{
		"app.kubernetes.io/name":     "myapp",
		"helm.sh/chart":              "myapp-1.0.0",
		"service.beta.kubernetes.io": "value",
	}
	got := FilterAnnotations(in)
	if len(got) != 3 {
		t.Fatalf("expected 3 annotations, got %d", len(got))
	}
	for k, v := range in {
		if got[k] != v {
			t.Errorf("key %q: expected %q, got %q", k, v, got[k])
		}
	}
}

func TestFilterAnnotations_SkipLastApplied(t *testing.T) {
	in := map[string]string{
		"app": "test",
		"kubectl.kubernetes.io/last-applied-configuration": strings.Repeat("x", 50000),
	}
	got := FilterAnnotations(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(got))
	}
	if _, ok := got["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		t.Fatal("last-applied-configuration should be filtered out")
	}
	if got["app"] != "test" {
		t.Errorf("expected 'test', got %q", got["app"])
	}
}

func TestFilterAnnotations_TruncateLong(t *testing.T) {
	longVal := strings.Repeat("a", 2048)
	in := map[string]string{
		"long-annotation": longVal,
		"short":           "ok",
	}
	got := FilterAnnotations(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(got))
	}
	if len(got["long-annotation"]) != 1024 {
		t.Errorf("expected truncated to 1024, got %d", len(got["long-annotation"]))
	}
	if got["short"] != "ok" {
		t.Errorf("expected 'ok', got %q", got["short"])
	}
}

func TestFilterAnnotations_DoesNotMutateInput(t *testing.T) {
	longVal := strings.Repeat("b", 2048)
	in := map[string]string{
		"key": longVal,
	}
	_ = FilterAnnotations(in)
	if len(in["key"]) != 2048 {
		t.Fatal("input map was mutated")
	}
}

func TestParseQuantity_CPU(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"500m", 0.5},
		{"2", 2.0},
		{"100m", 0.1},
		{"1", 1.0},
		{"250m", 0.25},
		{"4000m", 4.0},
	}
	for _, tc := range tests {
		q := resource.MustParse(tc.input)
		got := ParseQuantity(q)
		if got != tc.expected {
			t.Errorf("ParseQuantity(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestParseQuantity_Memory(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"256Mi", 268435456},
		{"2Gi", 2147483648},
		{"1Ki", 1024},
	}
	for _, tc := range tests {
		q := resource.MustParse(tc.input)
		got := ParseQuantity(q)
		if got != tc.expected {
			t.Errorf("ParseQuantity(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestParseQuantityString_Valid(t *testing.T) {
	got := ParseQuantityString("500m")
	if got != 0.5 {
		t.Errorf("ParseQuantityString('500m') = %v, want 0.5", got)
	}
}

func TestParseQuantityString_Invalid(t *testing.T) {
	got := ParseQuantityString("garbage")
	if got != 0 {
		t.Errorf("ParseQuantityString('garbage') = %v, want 0", got)
	}
}

func TestParseQuantityString_Empty(t *testing.T) {
	got := ParseQuantityString("")
	if got != 0 {
		t.Errorf("ParseQuantityString('') = %v, want 0", got)
	}
}
