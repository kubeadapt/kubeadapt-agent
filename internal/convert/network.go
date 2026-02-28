package convert

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// ServiceToModel converts a Kubernetes Service to model.ServiceInfo.
// Pure function — no side effects.
// TargetWorkloads are left empty (resolved by enrichment).
func ServiceToModel(svc *corev1.Service) model.ServiceInfo {
	info := model.ServiceInfo{
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Type:      string(svc.Spec.Type),

		ClusterIP:  svc.Spec.ClusterIP,
		ClusterIPs: svc.Spec.ClusterIPs,

		Selector: svc.Spec.Selector,

		Labels:            svc.Labels,
		Annotations:       FilterAnnotations(svc.Annotations),
		CreationTimestamp: svc.CreationTimestamp.UnixMilli(),

		SessionAffinity: string(svc.Spec.SessionAffinity),
	}

	// ExternalIPs
	if len(svc.Spec.ExternalIPs) > 0 {
		info.ExternalIPs = svc.Spec.ExternalIPs
	}

	// Ports
	if len(svc.Spec.Ports) > 0 {
		info.Ports = make([]model.ServicePortInfo, len(svc.Spec.Ports))
		for i, p := range svc.Spec.Ports {
			info.Ports[i] = model.ServicePortInfo{
				Name:       p.Name,
				Protocol:   string(p.Protocol),
				Port:       p.Port,
				TargetPort: p.TargetPort.String(),
				NodePort:   p.NodePort,
			}
		}
	}

	// LoadBalancer info
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		lb := &model.LoadBalancerInfo{}

		// Ingress points from status
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			lb.Ingress = make([]model.LoadBalancerIngress, len(svc.Status.LoadBalancer.Ingress))
			for i, ing := range svc.Status.LoadBalancer.Ingress {
				lb.Ingress[i] = model.LoadBalancerIngress{
					IP:       ing.IP,
					Hostname: ing.Hostname,
				}
			}
		}

		// AWS annotations
		if svc.Annotations != nil {
			lb.AWSLoadBalancerType = svc.Annotations["service.beta.kubernetes.io/aws-load-balancer-type"]
			lb.AWSScheme = svc.Annotations["service.beta.kubernetes.io/aws-load-balancer-scheme"]
			lb.AWSARNAnnotation = svc.Annotations["service.beta.kubernetes.io/aws-load-balancer-arn"]
		}

		// LoadBalancerClass
		if svc.Spec.LoadBalancerClass != nil {
			lb.Class = *svc.Spec.LoadBalancerClass
		}

		info.LoadBalancer = lb
	}

	return info
}

// IngressToModel converts a Kubernetes Ingress to model.IngressInfo.
// Pure function — no side effects.
func IngressToModel(ing *networkingv1.Ingress) model.IngressInfo {
	info := model.IngressInfo{
		Name:      ing.Name,
		Namespace: ing.Namespace,

		Labels:            ing.Labels,
		Annotations:       FilterAnnotations(ing.Annotations),
		CreationTimestamp: ing.CreationTimestamp.UnixMilli(),
	}

	// IngressClassName
	if ing.Spec.IngressClassName != nil {
		info.IngressClassName = *ing.Spec.IngressClassName
	}

	// Rules
	if len(ing.Spec.Rules) > 0 {
		info.Rules = make([]model.IngressRuleInfo, len(ing.Spec.Rules))
		for i, r := range ing.Spec.Rules {
			rule := model.IngressRuleInfo{
				Host: r.Host,
			}
			if r.HTTP != nil {
				rule.Paths = make([]model.IngressPathInfo, len(r.HTTP.Paths))
				for j, p := range r.HTTP.Paths {
					pi := model.IngressPathInfo{
						Path: p.Path,
					}
					if p.PathType != nil {
						pi.PathType = string(*p.PathType)
					}
					if p.Backend.Service != nil {
						pi.BackendService = p.Backend.Service.Name
						if p.Backend.Service.Port.Number != 0 {
							pi.BackendPort = fmt.Sprintf("%d", p.Backend.Service.Port.Number)
						} else {
							pi.BackendPort = p.Backend.Service.Port.Name
						}
					}
					rule.Paths[j] = pi
				}
			}
			info.Rules[i] = rule
		}
	}

	// TLS
	if len(ing.Spec.TLS) > 0 {
		info.TLS = make([]model.IngressTLSInfo, len(ing.Spec.TLS))
		for i, t := range ing.Spec.TLS {
			info.TLS[i] = model.IngressTLSInfo{
				Hosts:      t.Hosts,
				SecretName: t.SecretName,
			}
		}
	}

	// DefaultBackend
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		db := &model.IngressBackendInfo{
			ServiceName: ing.Spec.DefaultBackend.Service.Name,
		}
		if ing.Spec.DefaultBackend.Service.Port.Number != 0 {
			db.ServicePort = fmt.Sprintf("%d", ing.Spec.DefaultBackend.Service.Port.Number)
		} else {
			db.ServicePort = ing.Spec.DefaultBackend.Service.Port.Name
		}
		info.DefaultBackend = db
	}

	// LoadBalancer hostnames from status
	if len(ing.Status.LoadBalancer.Ingress) > 0 {
		hostnames := make([]string, 0, len(ing.Status.LoadBalancer.Ingress))
		for _, lbi := range ing.Status.LoadBalancer.Ingress {
			if lbi.Hostname != "" {
				hostnames = append(hostnames, lbi.Hostname)
			} else if lbi.IP != "" {
				hostnames = append(hostnames, lbi.IP)
			}
		}
		if len(hostnames) > 0 {
			info.LoadBalancerHostnames = hostnames
		}
	}

	return info
}
