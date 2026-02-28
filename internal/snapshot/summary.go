package snapshot

import "github.com/kubeadapt/kubeadapt-agent/pkg/model"

// ComputeSummary calculates entity counts and resource totals from a snapshot.
func ComputeSummary(snapshot *model.ClusterSnapshot) model.ClusterSummary {
	s := model.ClusterSummary{
		NodeCount:           len(snapshot.Nodes),
		PodCount:            len(snapshot.Pods),
		NamespaceCount:      len(snapshot.Namespaces),
		DeploymentCount:     len(snapshot.Deployments),
		StatefulSetCount:    len(snapshot.StatefulSets),
		DaemonSetCount:      len(snapshot.DaemonSets),
		JobCount:            len(snapshot.Jobs),
		CronJobCount:        len(snapshot.CronJobs),
		CustomWorkloadCount: len(snapshot.CustomWorkloads),
		HPACount:            len(snapshot.HPAs),
		ServiceCount:        len(snapshot.Services),
		IngressCount:        len(snapshot.Ingresses),
		PVCount:             len(snapshot.PVs),
		PVCCount:            len(snapshot.PVCs),
	}

	// Pod phase counts and container counts.
	for i := range snapshot.Pods {
		pod := &snapshot.Pods[i]
		switch pod.Phase {
		case "Running":
			s.RunningPodCount++
		case "Pending":
			s.PendingPodCount++
		case "Failed":
			s.FailedPodCount++
		}
		s.ContainerCount += len(pod.Containers)
	}

	// Node resource totals and metrics availability.
	var (
		metricsAvailable bool
		cpuUsageSum      float64
		memUsageSum      int64
		hasCPUUsage      bool
		hasMemUsage      bool

		gpuUsageSum     float64
		gpuTensorSum    float64
		gpuMemUtilSum   float64
		gpuMemUsedSum   int64
		gpuMemTotalSum  int64
		gpuNodeCount    int
		gpuTensorCount  int
		gpuMemUtilCount int
		hasGPUMetrics   bool
	)
	for i := range snapshot.Nodes {
		n := &snapshot.Nodes[i]
		s.TotalCPUCapacity += n.CPUCapacityCores
		s.TotalCPUAllocatable += n.CPUAllocatable
		s.TotalMemoryCapacity += n.MemoryCapacityBytes
		s.TotalMemoryAllocatable += n.MemoryAllocatable
		s.TotalGPUCapacity += n.GPUCapacity

		if n.CPUUsageCores != nil {
			metricsAvailable = true
			cpuUsageSum += *n.CPUUsageCores
			hasCPUUsage = true
		}
		if n.MemoryUsageBytes != nil {
			memUsageSum += *n.MemoryUsageBytes
			hasMemUsage = true
		}

		if n.GPUUtilizationPercent != nil {
			gpuUsageSum += *n.GPUUtilizationPercent
			gpuNodeCount++
			hasGPUMetrics = true
		}
		if n.GPUTensorActivePercent != nil {
			gpuTensorSum += *n.GPUTensorActivePercent
			gpuTensorCount++
		}
		if n.GPUMemoryUtilPercent != nil {
			gpuMemUtilSum += *n.GPUMemoryUtilPercent
			gpuMemUtilCount++
		}
		if n.GPUMemoryUsedBytes != nil {
			gpuMemUsedSum += *n.GPUMemoryUsedBytes
		}
		if n.GPUMemoryTotalBytes != nil {
			gpuMemTotalSum += *n.GPUMemoryTotalBytes
		}
	}
	s.MetricsAvailable = metricsAvailable
	s.GPUMetricsAvailable = hasGPUMetrics

	if hasCPUUsage {
		s.TotalCPUUsage = &cpuUsageSum
	}
	if hasMemUsage {
		s.TotalMemoryUsage = &memUsageSum
	}
	if hasGPUMetrics && gpuNodeCount > 0 {
		avg := gpuUsageSum / float64(gpuNodeCount)
		s.TotalGPUUsage = &avg
		s.TotalGPUMemoryUsed = &gpuMemUsedSum
		s.TotalGPUMemoryTotal = &gpuMemTotalSum
	}
	if gpuTensorCount > 0 {
		avg := gpuTensorSum / float64(gpuTensorCount)
		s.TotalGPUTensorActive = &avg
	}
	if gpuMemUtilCount > 0 {
		avg := gpuMemUtilSum / float64(gpuMemUtilCount)
		s.TotalGPUMemoryUtil = &avg
	}

	// Pod container resource requests and GPU.
	for i := range snapshot.Pods {
		for j := range snapshot.Pods[i].Containers {
			c := &snapshot.Pods[i].Containers[j]
			s.TotalCPURequested += c.CPURequestCores
			s.TotalMemoryRequested += c.MemoryRequestBytes
			s.TotalGPURequested += c.GPURequest
		}
	}

	// Storage totals from PVs and PVCs.
	for i := range snapshot.PVs {
		s.TotalStorageCapacity += snapshot.PVs[i].Capacity
	}
	for i := range snapshot.PVCs {
		s.TotalStorageRequested += snapshot.PVCs[i].RequestedBytes
	}

	return s
}
