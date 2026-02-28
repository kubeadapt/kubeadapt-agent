package store

import "github.com/kubeadapt/kubeadapt-agent/pkg/model"

// Store is the composite in-memory store that aggregates all 22 resource-typed stores.
// Each TypedStore has its own RWMutex, so concurrent access to different resource types
// does not contend on a single lock.
type Store struct {
	Nodes           *TypedStore[model.NodeInfo]
	Pods            *TypedStore[model.PodInfo]
	Namespaces      *TypedStore[model.NamespaceInfo]
	Deployments     *TypedStore[model.DeploymentInfo]
	StatefulSets    *TypedStore[model.StatefulSetInfo]
	DaemonSets      *TypedStore[model.DaemonSetInfo]
	ReplicaSets     *TypedStore[model.ReplicaSetInfo]
	Jobs            *TypedStore[model.JobInfo]
	CronJobs        *TypedStore[model.CronJobInfo]
	CustomWorkloads *TypedStore[model.CustomWorkloadInfo]
	HPAs            *TypedStore[model.HPAInfo]
	VPAs            *TypedStore[model.VPAInfo]
	PDBs            *TypedStore[model.PDBInfo]
	Services        *TypedStore[model.ServiceInfo]
	Ingresses       *TypedStore[model.IngressInfo]
	PVs             *TypedStore[model.PVInfo]
	PVCs            *TypedStore[model.PVCInfo]
	StorageClasses  *TypedStore[model.StorageClassInfo]
	PriorityClasses *TypedStore[model.PriorityClassInfo]
	LimitRanges     *TypedStore[model.LimitRangeInfo]
	ResourceQuotas  *TypedStore[model.ResourceQuotaInfo]
	NodePools       *TypedStore[model.NodePoolInfo]
}

// LastUpdatedTimes returns the UnixMilli timestamp of the last update for each typed store.
// Used by the snapshot builder for staleness detection.
func (s *Store) LastUpdatedTimes() map[string]int64 {
	return map[string]int64{
		"nodes":            s.Nodes.LastUpdated(),
		"pods":             s.Pods.LastUpdated(),
		"namespaces":       s.Namespaces.LastUpdated(),
		"deployments":      s.Deployments.LastUpdated(),
		"statefulsets":     s.StatefulSets.LastUpdated(),
		"daemonsets":       s.DaemonSets.LastUpdated(),
		"replicasets":      s.ReplicaSets.LastUpdated(),
		"jobs":             s.Jobs.LastUpdated(),
		"cronjobs":         s.CronJobs.LastUpdated(),
		"custom_workloads": s.CustomWorkloads.LastUpdated(),
		"hpas":             s.HPAs.LastUpdated(),
		"vpas":             s.VPAs.LastUpdated(),
		"pdbs":             s.PDBs.LastUpdated(),
		"services":         s.Services.LastUpdated(),
		"ingresses":        s.Ingresses.LastUpdated(),
		"pvs":              s.PVs.LastUpdated(),
		"pvcs":             s.PVCs.LastUpdated(),
		"storageclasses":   s.StorageClasses.LastUpdated(),
		"priorityclasses":  s.PriorityClasses.LastUpdated(),
		"limitranges":      s.LimitRanges.LastUpdated(),
		"resourcequotas":   s.ResourceQuotas.LastUpdated(),
		"nodepools":        s.NodePools.LastUpdated(),
	}
}

// ItemCounts returns the number of items in each typed store.
// Implements health.StoreStats.
func (s *Store) ItemCounts() map[string]int {
	return map[string]int{
		"nodes":            s.Nodes.Len(),
		"pods":             s.Pods.Len(),
		"namespaces":       s.Namespaces.Len(),
		"deployments":      s.Deployments.Len(),
		"statefulsets":     s.StatefulSets.Len(),
		"daemonsets":       s.DaemonSets.Len(),
		"replicasets":      s.ReplicaSets.Len(),
		"jobs":             s.Jobs.Len(),
		"cronjobs":         s.CronJobs.Len(),
		"custom_workloads": s.CustomWorkloads.Len(),
		"hpas":             s.HPAs.Len(),
		"vpas":             s.VPAs.Len(),
		"pdbs":             s.PDBs.Len(),
		"services":         s.Services.Len(),
		"ingresses":        s.Ingresses.Len(),
		"pvs":              s.PVs.Len(),
		"pvcs":             s.PVCs.Len(),
		"storageclasses":   s.StorageClasses.Len(),
		"priorityclasses":  s.PriorityClasses.Len(),
		"limitranges":      s.LimitRanges.Len(),
		"resourcequotas":   s.ResourceQuotas.Len(),
		"nodepools":        s.NodePools.Len(),
	}
}

// NewStore creates a Store with all 22 TypedStores initialized.
func NewStore() *Store {
	return &Store{
		Nodes:           NewTypedStore[model.NodeInfo](),
		Pods:            NewTypedStore[model.PodInfo](),
		Namespaces:      NewTypedStore[model.NamespaceInfo](),
		Deployments:     NewTypedStore[model.DeploymentInfo](),
		StatefulSets:    NewTypedStore[model.StatefulSetInfo](),
		DaemonSets:      NewTypedStore[model.DaemonSetInfo](),
		ReplicaSets:     NewTypedStore[model.ReplicaSetInfo](),
		Jobs:            NewTypedStore[model.JobInfo](),
		CronJobs:        NewTypedStore[model.CronJobInfo](),
		CustomWorkloads: NewTypedStore[model.CustomWorkloadInfo](),
		HPAs:            NewTypedStore[model.HPAInfo](),
		VPAs:            NewTypedStore[model.VPAInfo](),
		PDBs:            NewTypedStore[model.PDBInfo](),
		Services:        NewTypedStore[model.ServiceInfo](),
		Ingresses:       NewTypedStore[model.IngressInfo](),
		PVs:             NewTypedStore[model.PVInfo](),
		PVCs:            NewTypedStore[model.PVCInfo](),
		StorageClasses:  NewTypedStore[model.StorageClassInfo](),
		PriorityClasses: NewTypedStore[model.PriorityClassInfo](),
		LimitRanges:     NewTypedStore[model.LimitRangeInfo](),
		ResourceQuotas:  NewTypedStore[model.ResourceQuotaInfo](),
		NodePools:       NewTypedStore[model.NodePoolInfo](),
	}
}
