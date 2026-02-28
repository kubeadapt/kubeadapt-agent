package model

// PVInfo represents a Kubernetes PersistentVolume.
type PVInfo struct {
	Name             string   `json:"name"`
	Capacity         int64    `json:"capacity"`
	AccessModes      []string `json:"access_modes"`
	ReclaimPolicy    string   `json:"reclaim_policy"`
	StorageClassName string   `json:"storage_class_name"`
	VolumeMode       string   `json:"volume_mode"`
	Phase            string   `json:"phase"`
	MountOptions     []string `json:"mount_options"`

	Source PVSourceInfo `json:"source"`

	ClaimRef *PVClaimRefInfo `json:"claim_ref,omitempty"`

	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`
}

// PVSourceInfo describes the storage source of a PV.
type PVSourceInfo struct {
	Type            string `json:"type"`
	CSIDriver       string `json:"csi_driver"`
	CSIVolumeHandle string `json:"csi_volume_handle"`
	CSIFSType       string `json:"csi_fs_type"`
	AWSVolumeID     string `json:"aws_volume_id"`
	AWSPartition    int    `json:"aws_partition"`
	NFSServer       string `json:"nfs_server"`
	NFSPath         string `json:"nfs_path"`
}

// PVClaimRefInfo identifies the PVC a PV is bound to.
type PVClaimRefInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

// PVCInfo represents a Kubernetes PersistentVolumeClaim.
type PVCInfo struct {
	Name             string   `json:"name"`
	Namespace        string   `json:"namespace"`
	Phase            string   `json:"phase"`
	AccessModes      []string `json:"access_modes"`
	StorageClassName string   `json:"storage_class_name"`
	VolumeMode       string   `json:"volume_mode"`
	VolumeName       string   `json:"volume_name"`

	RequestedBytes int64 `json:"requested_bytes"`
	CapacityBytes  int64 `json:"capacity_bytes"`

	MountedByPods []string `json:"mounted_by_pods"`

	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`

	Conditions []PVCConditionInfo `json:"conditions"`
}

// PVCConditionInfo represents a PVC condition.
type PVCConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// StorageClassInfo represents a Kubernetes StorageClass.
type StorageClassInfo struct {
	Name                 string            `json:"name"`
	Provisioner          string            `json:"provisioner"`
	ReclaimPolicy        string            `json:"reclaim_policy"`
	VolumeBindingMode    string            `json:"volume_binding_mode"`
	AllowVolumeExpansion bool              `json:"allow_volume_expansion"`
	Parameters           map[string]string `json:"parameters"`
	MountOptions         []string          `json:"mount_options"`
	Labels               map[string]string `json:"labels"`
	Annotations          map[string]string `json:"annotations"`
	IsDefault            bool              `json:"is_default"`
}
