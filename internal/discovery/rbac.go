package discovery

import (
	"context"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

// CheckResource performs a 3-phase conditional check for a Kubernetes resource:
//
//  1. API group exists — via ServerGroups discovery
//  2. Resource exists — via ServerResourcesForGroupVersion
//  3. RBAC allows list+watch — via SelfSubjectAccessReview
//
// If any phase fails or the resource is unavailable, it returns false with no error.
// Errors are only returned for unexpected failures (e.g., network issues).
func CheckResource(ctx context.Context, client kubernetes.Interface, discoveryClient discovery.DiscoveryInterface, group, version, resource string) (bool, error) {
	// Phase 1: Check if API group exists.
	groupExists, err := HasAPIGroup(discoveryClient, group)
	if err != nil {
		return false, fmt.Errorf("discovery: phase 1 check API group %q: %w", group, err)
	}
	if !groupExists {
		return false, nil
	}

	// Phase 2: Check if specific resource exists in the group.
	resourceExists, err := hasResource(discoveryClient, group, version, resource)
	if err != nil {
		return false, fmt.Errorf("discovery: phase 2 check resource %q in %s/%s: %w", resource, group, version, err)
	}
	if !resourceExists {
		return false, nil
	}

	// Phase 3: Verify RBAC allows list+watch.
	canAccess, err := CanListWatch(ctx, client, group, resource)
	if err != nil {
		return false, fmt.Errorf("discovery: phase 3 RBAC check for %q: %w", resource, err)
	}

	return canAccess, nil
}

// hasResource checks if a specific resource exists in a group/version.
func hasResource(discoveryClient discovery.DiscoveryInterface, group, version, resource string) (bool, error) {
	groupVersion := version
	if group != "" {
		groupVersion = group + "/" + version
	}

	resources, err := discoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		// If the group/version is not found, treat as resource missing — not an error.
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	for _, r := range resources.APIResources {
		if r.Name == resource {
			return true, nil
		}
	}
	return false, nil
}

// CanListWatch checks if the current service account has list and watch
// permissions for the given resource via SelfSubjectAccessReview.
func CanListWatch(ctx context.Context, client kubernetes.Interface, group, resource string) (bool, error) {
	for _, verb := range []string{"list", "watch"} {
		allowed, err := checkAccess(ctx, client, group, resource, verb)
		if err != nil {
			return false, err
		}
		if !allowed {
			return false, nil
		}
	}
	return true, nil
}

// checkAccess creates a SelfSubjectAccessReview for a single verb.
func checkAccess(ctx context.Context, client kubernetes.Interface, group, resource, verb string) (bool, error) {
	review := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:     verb,
				Group:    group,
				Resource: resource,
			},
		},
	}

	result, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("SelfSubjectAccessReview for %s/%s verb=%s: %w", group, resource, verb, err)
	}

	return result.Status.Allowed, nil
}
