package convert

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ---- Service Tests ----

func TestServiceToModel_ClusterIP(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web-svc",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Labels:            map[string]string{"app": "web"},
			Annotations:       map[string]string{"note": "test"},
		},
		Spec: corev1.ServiceSpec{
			Type:       corev1.ServiceTypeClusterIP,
			ClusterIP:  "10.0.0.42",
			ClusterIPs: []string{"10.0.0.42"},
			Selector:   map[string]string{"app": "web"},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt32(8080),
				},
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       443,
					TargetPort: intstr.FromString("https"),
				},
			},
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}

	info := ServiceToModel(svc)

	assertEqual(t, "Name", info.Name, "web-svc")
	assertEqual(t, "Namespace", info.Namespace, "production")
	assertEqual(t, "Type", info.Type, "ClusterIP")
	assertEqual(t, "ClusterIP", info.ClusterIP, "10.0.0.42")
	assertEqual(t, "SessionAffinity", info.SessionAffinity, "None")
	assertEqual(t, "Selector[app]", info.Selector["app"], "web")

	if len(info.ClusterIPs) != 1 {
		t.Fatalf("ClusterIPs len: want 1, got %d", len(info.ClusterIPs))
	}

	// Ports
	if len(info.Ports) != 2 {
		t.Fatalf("Ports len: want 2, got %d", len(info.Ports))
	}
	assertEqual(t, "Ports[0].Name", info.Ports[0].Name, "http")
	assertEqual(t, "Ports[0].Protocol", info.Ports[0].Protocol, "TCP")
	if info.Ports[0].Port != 80 {
		t.Errorf("Ports[0].Port: want 80, got %d", info.Ports[0].Port)
	}
	assertEqual(t, "Ports[0].TargetPort", info.Ports[0].TargetPort, "8080")
	assertEqual(t, "Ports[1].TargetPort", info.Ports[1].TargetPort, "https")

	// No load balancer for ClusterIP
	if info.LoadBalancer != nil {
		t.Error("LoadBalancer should be nil for ClusterIP service")
	}

	// TargetWorkloads should be empty (enrichment's job)
	if len(info.TargetWorkloads) != 0 {
		t.Errorf("TargetWorkloads should be empty, got %d", len(info.TargetWorkloads))
	}
}

func TestServiceToModel_LoadBalancer_AWS(t *testing.T) {
	lbClass := "service.k8s.aws/nlb"
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "api-lb",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Annotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-type":   "nlb",
				"service.beta.kubernetes.io/aws-load-balancer-scheme": "internet-facing",
				"service.beta.kubernetes.io/aws-load-balancer-arn":    "arn:aws:elasticloadbalancing:us-east-1:123456:loadbalancer/net/my-lb/abc123",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:              corev1.ServiceTypeLoadBalancer,
			LoadBalancerClass: &lbClass,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt32(8080),
					NodePort:   30080,
				},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{Hostname: "my-lb-123.us-east-1.elb.amazonaws.com"},
					{IP: "203.0.113.1"},
				},
			},
		},
	}

	info := ServiceToModel(svc)

	assertEqual(t, "Type", info.Type, "LoadBalancer")

	if info.LoadBalancer == nil {
		t.Fatal("LoadBalancer should not be nil")
	}
	lb := info.LoadBalancer

	assertEqual(t, "LB.Class", lb.Class, "service.k8s.aws/nlb")
	assertEqual(t, "LB.AWSLoadBalancerType", lb.AWSLoadBalancerType, "nlb")
	assertEqual(t, "LB.AWSScheme", lb.AWSScheme, "internet-facing")
	assertEqual(t, "LB.AWSARNAnnotation", lb.AWSARNAnnotation, "arn:aws:elasticloadbalancing:us-east-1:123456:loadbalancer/net/my-lb/abc123")

	if len(lb.Ingress) != 2 {
		t.Fatalf("LB.Ingress len: want 2, got %d", len(lb.Ingress))
	}
	assertEqual(t, "LB.Ingress[0].Hostname", lb.Ingress[0].Hostname, "my-lb-123.us-east-1.elb.amazonaws.com")
	assertEqual(t, "LB.Ingress[1].IP", lb.Ingress[1].IP, "203.0.113.1")

	// NodePort
	if info.Ports[0].NodePort != 30080 {
		t.Errorf("Ports[0].NodePort: want 30080, got %d", info.Ports[0].NodePort)
	}
}

// ---- Ingress Tests ----

func TestIngressToModel_RulesTLSDefaultBackend(t *testing.T) {
	className := "nginx"
	pathPrefix := networkingv1.PathTypePrefix
	pathExact := networkingv1.PathTypeExact

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web-ingress",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Labels:            map[string]string{"app": "web"},
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "default-svc",
					Port: networkingv1.ServiceBackendPort{Number: 80},
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/api",
									PathType: &pathPrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api-svc",
											Port: networkingv1.ServiceBackendPort{Number: 8080},
										},
									},
								},
								{
									Path:     "/health",
									PathType: &pathExact,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "health-svc",
											Port: networkingv1.ServiceBackendPort{Name: "http"},
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{"example.com", "www.example.com"},
					SecretName: "example-tls",
				},
			},
		},
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{
					{Hostname: "k8s-ingress.us-east-1.elb.amazonaws.com"},
				},
			},
		},
	}

	info := IngressToModel(ing)

	assertEqual(t, "Name", info.Name, "web-ingress")
	assertEqual(t, "Namespace", info.Namespace, "production")
	assertEqual(t, "IngressClassName", info.IngressClassName, "nginx")

	// DefaultBackend
	if info.DefaultBackend == nil {
		t.Fatal("DefaultBackend should not be nil")
	}
	assertEqual(t, "DefaultBackend.ServiceName", info.DefaultBackend.ServiceName, "default-svc")
	assertEqual(t, "DefaultBackend.ServicePort", info.DefaultBackend.ServicePort, "80")

	// Rules
	if len(info.Rules) != 1 {
		t.Fatalf("Rules len: want 1, got %d", len(info.Rules))
	}
	rule := info.Rules[0]
	assertEqual(t, "Rule.Host", rule.Host, "example.com")
	if len(rule.Paths) != 2 {
		t.Fatalf("Paths len: want 2, got %d", len(rule.Paths))
	}
	assertEqual(t, "Path[0].Path", rule.Paths[0].Path, "/api")
	assertEqual(t, "Path[0].PathType", rule.Paths[0].PathType, "Prefix")
	assertEqual(t, "Path[0].BackendService", rule.Paths[0].BackendService, "api-svc")
	assertEqual(t, "Path[0].BackendPort", rule.Paths[0].BackendPort, "8080")

	assertEqual(t, "Path[1].Path", rule.Paths[1].Path, "/health")
	assertEqual(t, "Path[1].PathType", rule.Paths[1].PathType, "Exact")
	assertEqual(t, "Path[1].BackendService", rule.Paths[1].BackendService, "health-svc")
	assertEqual(t, "Path[1].BackendPort", rule.Paths[1].BackendPort, "http")

	// TLS
	if len(info.TLS) != 1 {
		t.Fatalf("TLS len: want 1, got %d", len(info.TLS))
	}
	assertEqual(t, "TLS[0].SecretName", info.TLS[0].SecretName, "example-tls")
	if len(info.TLS[0].Hosts) != 2 {
		t.Fatalf("TLS[0].Hosts len: want 2, got %d", len(info.TLS[0].Hosts))
	}

	// LoadBalancerHostnames
	if len(info.LoadBalancerHostnames) != 1 {
		t.Fatalf("LoadBalancerHostnames len: want 1, got %d", len(info.LoadBalancerHostnames))
	}
	assertEqual(t, "LBHostname", info.LoadBalancerHostnames[0], "k8s-ingress.us-east-1.elb.amazonaws.com")
}
