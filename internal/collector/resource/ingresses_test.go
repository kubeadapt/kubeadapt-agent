package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestIngressCollector_Name(t *testing.T) {
	env := newTestEnv(t)
	c := NewIngressCollector(env.client, env.store, env.metrics, testResyncPeriod)
	assert.Equal(t, "ingresses", c.Name())
}

func TestIngressCollector_AddUpdateDelete(t *testing.T) {
	env := newTestEnv(t)
	c := NewIngressCollector(env.client, env.store, env.metrics, testResyncPeriod)
	startCollector(t, env, c)

	// --- Add ---
	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "web-ing", Namespace: "default"},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			Rules: []networkingv1.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "web-svc",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err := env.client.NetworkingV1().Ingresses("default").Create(env.ctx, ing, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Ingresses.Len() == 1
	}, waitTimeout, pollInterval)

	info, ok := env.store.Ingresses.Get("default/web-ing")
	require.True(t, ok)
	assert.Equal(t, "web-ing", info.Name)
	assert.Equal(t, "nginx", info.IngressClassName)

	// --- Update ---
	ing.Spec.IngressClassName = ptr.To("traefik")
	_, err = env.client.NetworkingV1().Ingresses("default").Update(env.ctx, ing, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, _ := env.store.Ingresses.Get("default/web-ing")
		return info.IngressClassName == "traefik"
	}, waitTimeout, pollInterval)

	// --- Delete ---
	err = env.client.NetworkingV1().Ingresses("default").Delete(env.ctx, "web-ing", metav1.DeleteOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.store.Ingresses.Len() == 0
	}, waitTimeout, pollInterval)
}
