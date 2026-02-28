package model

// ServiceInfo represents a Kubernetes Service.
type ServiceInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`

	ClusterIP   string   `json:"cluster_ip"`
	ClusterIPs  []string `json:"cluster_ips"`
	ExternalIPs []string `json:"external_ips"`

	LoadBalancer *LoadBalancerInfo `json:"load_balancer,omitempty"`

	Ports    []ServicePortInfo `json:"ports"`
	Selector map[string]string `json:"selector"`

	TargetWorkloads []WorkloadReference `json:"target_workloads"`

	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`

	SessionAffinity string `json:"session_affinity"`
}

// LoadBalancerInfo holds load balancer details for LoadBalancer-type services.
type LoadBalancerInfo struct {
	Ingress             []LoadBalancerIngress `json:"ingress"`
	Class               string                `json:"class"`
	AWSLoadBalancerType string                `json:"aws_load_balancer_type"`
	AWSScheme           string                `json:"aws_scheme"`
	AWSARNAnnotation    string                `json:"aws_arn_annotation"`
}

// LoadBalancerIngress represents a single load balancer ingress point.
type LoadBalancerIngress struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

// ServicePortInfo represents a port on a service.
type ServicePortInfo struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"`
	NodePort   int32  `json:"node_port"`
}

// IngressInfo represents a Kubernetes Ingress.
type IngressInfo struct {
	Name                  string              `json:"name"`
	Namespace             string              `json:"namespace"`
	IngressClassName      string              `json:"ingress_class_name"`
	Rules                 []IngressRuleInfo   `json:"rules"`
	TLS                   []IngressTLSInfo    `json:"tls"`
	DefaultBackend        *IngressBackendInfo `json:"default_backend,omitempty"`
	Labels                map[string]string   `json:"labels"`
	Annotations           map[string]string   `json:"annotations"`
	CreationTimestamp     int64               `json:"creation_timestamp"`
	LoadBalancerHostnames []string            `json:"load_balancer_hostnames"`
}

// IngressRuleInfo represents a single ingress rule.
type IngressRuleInfo struct {
	Host  string            `json:"host"`
	Paths []IngressPathInfo `json:"paths"`
}

// IngressPathInfo represents a single path within an ingress rule.
type IngressPathInfo struct {
	Path           string `json:"path"`
	PathType       string `json:"path_type"`
	BackendService string `json:"backend_service"`
	BackendPort    string `json:"backend_port"`
}

// IngressTLSInfo represents TLS configuration for an ingress.
type IngressTLSInfo struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secret_name"`
}

// IngressBackendInfo represents a default backend for an ingress.
type IngressBackendInfo struct {
	ServiceName string `json:"service_name"`
	ServicePort string `json:"service_port"`
}
