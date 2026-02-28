package convert

import (
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

// PVToModel converts a Kubernetes PersistentVolume to model.PVInfo.
// Pure function — no side effects.
func PVToModel(pv *corev1.PersistentVolume) model.PVInfo {
	info := model.PVInfo{
		Name:             pv.Name,
		StorageClassName: pv.Spec.StorageClassName,
		Phase:            string(pv.Status.Phase),
		MountOptions:     pv.Spec.MountOptions,

		Labels:            pv.Labels,
		Annotations:       FilterAnnotations(pv.Annotations),
		CreationTimestamp: pv.CreationTimestamp.UnixMilli(),
	}

	// Capacity
	info.Capacity = quantityValue(pv.Spec.Capacity, corev1.ResourceStorage)

	// AccessModes
	if len(pv.Spec.AccessModes) > 0 {
		info.AccessModes = make([]string, len(pv.Spec.AccessModes))
		for i, m := range pv.Spec.AccessModes {
			info.AccessModes[i] = string(m)
		}
	}

	// ReclaimPolicy
	if pv.Spec.PersistentVolumeReclaimPolicy != "" {
		info.ReclaimPolicy = string(pv.Spec.PersistentVolumeReclaimPolicy)
	}

	// VolumeMode
	if pv.Spec.VolumeMode != nil {
		info.VolumeMode = string(*pv.Spec.VolumeMode)
	} else {
		info.VolumeMode = "Filesystem"
	}

	// Source
	info.Source = extractPVSource(pv)

	// ClaimRef
	if pv.Spec.ClaimRef != nil {
		info.ClaimRef = &model.PVClaimRefInfo{
			Namespace: pv.Spec.ClaimRef.Namespace,
			Name:      pv.Spec.ClaimRef.Name,
			UID:       string(pv.Spec.ClaimRef.UID),
		}
	}

	return info
}

func extractPVSource(pv *corev1.PersistentVolume) model.PVSourceInfo {
	src := model.PVSourceInfo{}

	switch {
	case pv.Spec.CSI != nil:
		src.Type = "csi"
		src.CSIDriver = pv.Spec.CSI.Driver
		src.CSIVolumeHandle = pv.Spec.CSI.VolumeHandle
		src.CSIFSType = pv.Spec.CSI.FSType
	case pv.Spec.AWSElasticBlockStore != nil:
		src.Type = "awsElasticBlockStore"
		src.AWSVolumeID = pv.Spec.AWSElasticBlockStore.VolumeID
		src.AWSPartition = int(pv.Spec.AWSElasticBlockStore.Partition)
	case pv.Spec.NFS != nil:
		src.Type = "nfs"
		src.NFSServer = pv.Spec.NFS.Server
		src.NFSPath = pv.Spec.NFS.Path
	case pv.Spec.HostPath != nil:
		src.Type = "hostPath"
	case pv.Spec.GCEPersistentDisk != nil:
		src.Type = "gcePersistentDisk"
	case pv.Spec.AzureDisk != nil:
		src.Type = "azureDisk"
	case pv.Spec.AzureFile != nil:
		src.Type = "azureFile"
	case pv.Spec.FC != nil:
		src.Type = "fc"
	case pv.Spec.ISCSI != nil:
		src.Type = "iscsi"
	case pv.Spec.Local != nil:
		src.Type = "local"
	}

	return src
}

// PVCToModel converts a Kubernetes PersistentVolumeClaim to model.PVCInfo.
// Pure function — no side effects.
// MountedByPods is left empty (resolved by enrichment).
func PVCToModel(pvc *corev1.PersistentVolumeClaim) model.PVCInfo {
	info := model.PVCInfo{
		Name:       pvc.Name,
		Namespace:  pvc.Namespace,
		Phase:      string(pvc.Status.Phase),
		VolumeName: pvc.Spec.VolumeName,

		Labels:            pvc.Labels,
		Annotations:       FilterAnnotations(pvc.Annotations),
		CreationTimestamp: pvc.CreationTimestamp.UnixMilli(),
	}

	// AccessModes
	if len(pvc.Spec.AccessModes) > 0 {
		info.AccessModes = make([]string, len(pvc.Spec.AccessModes))
		for i, m := range pvc.Spec.AccessModes {
			info.AccessModes[i] = string(m)
		}
	}

	// StorageClassName
	if pvc.Spec.StorageClassName != nil {
		info.StorageClassName = *pvc.Spec.StorageClassName
	}

	// VolumeMode
	if pvc.Spec.VolumeMode != nil {
		info.VolumeMode = string(*pvc.Spec.VolumeMode)
	} else {
		info.VolumeMode = "Filesystem"
	}

	// RequestedBytes
	info.RequestedBytes = quantityValue(pvc.Spec.Resources.Requests, corev1.ResourceStorage)

	// CapacityBytes
	info.CapacityBytes = quantityValue(pvc.Status.Capacity, corev1.ResourceStorage)

	// Conditions
	info.Conditions = convertPVCConditions(pvc.Status.Conditions)

	return info
}

func convertPVCConditions(conditions []corev1.PersistentVolumeClaimCondition) []model.PVCConditionInfo {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]model.PVCConditionInfo, len(conditions))
	for i, c := range conditions {
		out[i] = model.PVCConditionInfo{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		}
	}
	return out
}

// StorageClassToModel converts a Kubernetes StorageClass to model.StorageClassInfo.
// Pure function — no side effects.
func StorageClassToModel(sc *storagev1.StorageClass) model.StorageClassInfo {
	info := model.StorageClassInfo{
		Name:         sc.Name,
		Provisioner:  sc.Provisioner,
		Parameters:   sc.Parameters,
		MountOptions: sc.MountOptions,
		Labels:       sc.Labels,
		Annotations:  sc.Annotations,
	}

	// ReclaimPolicy
	if sc.ReclaimPolicy != nil {
		info.ReclaimPolicy = string(*sc.ReclaimPolicy)
	}

	// VolumeBindingMode
	if sc.VolumeBindingMode != nil {
		info.VolumeBindingMode = string(*sc.VolumeBindingMode)
	}

	// AllowVolumeExpansion
	if sc.AllowVolumeExpansion != nil {
		info.AllowVolumeExpansion = *sc.AllowVolumeExpansion
	}

	// IsDefault
	if sc.Annotations != nil {
		info.IsDefault = sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true"
	}

	return info
}
