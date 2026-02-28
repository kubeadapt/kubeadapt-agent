package discovery

import (
	"context"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// addSelfSubjectAccessReviewReactor installs a reactor on the fake client
// that returns the given allowed value for all SelfSubjectAccessReview requests.
func addSelfSubjectAccessReviewReactor(client *fakeclientset.Clientset, allowed bool) {
	client.PrependReactor("create", "selfsubjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, &authorizationv1.SelfSubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: allowed,
			},
		}, nil
	})
}

func TestCanListWatch_Allowed(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	addSelfSubjectAccessReviewReactor(client, true)

	ok, err := CanListWatch(context.Background(), client, "metrics.k8s.io", "pods")
	if err != nil {
		t.Fatalf("CanListWatch() error = %v", err)
	}
	if !ok {
		t.Error("expected CanListWatch=true when both list and watch are allowed")
	}
}

func TestCanListWatch_Denied(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	addSelfSubjectAccessReviewReactor(client, false)

	ok, err := CanListWatch(context.Background(), client, "metrics.k8s.io", "pods")
	if err != nil {
		t.Fatalf("CanListWatch() error = %v", err)
	}
	if ok {
		t.Error("expected CanListWatch=false when access is denied")
	}
}

func TestCanListWatch_PartialDeny(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	callCount := 0
	client.PrependReactor("create", "selfsubjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		callCount++
		// Allow "list" but deny "watch".
		allowed := callCount == 1
		return true, &authorizationv1.SelfSubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{
				Allowed: allowed,
			},
		}, nil
	})

	ok, err := CanListWatch(context.Background(), client, "metrics.k8s.io", "pods")
	if err != nil {
		t.Fatalf("CanListWatch() error = %v", err)
	}
	if ok {
		t.Error("expected CanListWatch=false when watch is denied")
	}
}

func TestCheckResource_AllPhasesPass(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	addSelfSubjectAccessReviewReactor(client, true)

	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{
			GroupVersion: "metrics.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Verbs: metav1.Verbs{"get", "list", "watch"}},
				{Name: "nodes", Verbs: metav1.Verbs{"get", "list", "watch"}},
			},
		},
	})

	available, err := CheckResource(context.Background(), client, disco, "metrics.k8s.io", "v1beta1", "pods")
	if err != nil {
		t.Fatalf("CheckResource() error = %v", err)
	}
	if !available {
		t.Error("expected resource to be available when all 3 phases pass")
	}
}

func TestCheckResource_APIGroupMissing(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()

	// No metrics.k8s.io group registered.
	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{GroupVersion: "apps/v1"},
	})

	available, err := CheckResource(context.Background(), client, disco, "metrics.k8s.io", "v1beta1", "pods")
	if err != nil {
		t.Fatalf("CheckResource() should not error when API group missing, got: %v", err)
	}
	if available {
		t.Error("expected resource to be unavailable when API group is missing")
	}
}

func TestCheckResource_ResourceMissing(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()

	// Group exists but resource "pods" is not in it.
	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{
			GroupVersion: "metrics.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "nodes", Verbs: metav1.Verbs{"get", "list"}},
			},
		},
	})

	available, err := CheckResource(context.Background(), client, disco, "metrics.k8s.io", "v1beta1", "pods")
	if err != nil {
		t.Fatalf("CheckResource() should not error when resource missing, got: %v", err)
	}
	if available {
		t.Error("expected resource to be unavailable when resource is not in group")
	}
}

func TestCheckResource_RBACDenied(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	addSelfSubjectAccessReviewReactor(client, false)

	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{
			GroupVersion: "metrics.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Verbs: metav1.Verbs{"get", "list", "watch"}},
			},
		},
	})

	available, err := CheckResource(context.Background(), client, disco, "metrics.k8s.io", "v1beta1", "pods")
	if err != nil {
		t.Fatalf("CheckResource() error = %v", err)
	}
	if available {
		t.Error("expected resource to be unavailable when RBAC denies access")
	}
}

func TestCheckResource_GroupVersionNotFound(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()

	// Group exists with v1, but we ask for v1beta1.
	disco := newFakeDiscovery([]*metav1.APIResourceList{
		{
			GroupVersion: "metrics.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Verbs: metav1.Verbs{"get", "list", "watch"}},
			},
		},
	})

	available, err := CheckResource(context.Background(), client, disco, "metrics.k8s.io", "v1beta1", "pods")
	if err != nil {
		t.Fatalf("CheckResource() should not error when group version not found, got: %v", err)
	}
	if available {
		t.Error("expected resource to be unavailable when group version doesn't match")
	}
}
