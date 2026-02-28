package convert

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ---- PV Tests ----

func TestPVToModel_CSISource(t *testing.T) {
	volMode := corev1.PersistentVolumeFilesystem

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pv-csi-001",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Labels:            map[string]string{"type": "ebs"},
			Annotations:       map[string]string{"provisioner": "ebs.csi.aws.com"},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("100Gi"),
			},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			StorageClassName:              "gp3",
			VolumeMode:                    &volMode,
			MountOptions:                  []string{"noatime"},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "ebs.csi.aws.com",
					VolumeHandle: "vol-0123456789abcdef0",
					FSType:       "ext4",
				},
			},
			ClaimRef: &corev1.ObjectReference{
				Namespace: "production",
				Name:      "data-pvc",
				UID:       types.UID("abc-123"),
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}

	info := PVToModel(pv)

	assertEqual(t, "Name", info.Name, "pv-csi-001")
	// 100Gi = 107374182400 bytes
	if info.Capacity != 107374182400 {
		t.Errorf("Capacity: want 107374182400, got %d", info.Capacity)
	}
	if len(info.AccessModes) != 1 || info.AccessModes[0] != "ReadWriteOnce" {
		t.Errorf("AccessModes: want [ReadWriteOnce], got %v", info.AccessModes)
	}
	assertEqual(t, "ReclaimPolicy", info.ReclaimPolicy, "Delete")
	assertEqual(t, "StorageClassName", info.StorageClassName, "gp3")
	assertEqual(t, "VolumeMode", info.VolumeMode, "Filesystem")
	assertEqual(t, "Phase", info.Phase, "Bound")

	if len(info.MountOptions) != 1 || info.MountOptions[0] != "noatime" {
		t.Errorf("MountOptions: want [noatime], got %v", info.MountOptions)
	}

	// Source
	assertEqual(t, "Source.Type", info.Source.Type, "csi")
	assertEqual(t, "Source.CSIDriver", info.Source.CSIDriver, "ebs.csi.aws.com")
	assertEqual(t, "Source.CSIVolumeHandle", info.Source.CSIVolumeHandle, "vol-0123456789abcdef0")
	assertEqual(t, "Source.CSIFSType", info.Source.CSIFSType, "ext4")

	// ClaimRef
	if info.ClaimRef == nil {
		t.Fatal("ClaimRef should not be nil")
	}
	assertEqual(t, "ClaimRef.Namespace", info.ClaimRef.Namespace, "production")
	assertEqual(t, "ClaimRef.Name", info.ClaimRef.Name, "data-pvc")
	assertEqual(t, "ClaimRef.UID", info.ClaimRef.UID, "abc-123")
}

func TestPVToModel_AWSEBSSource(t *testing.T) {
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pv-ebs-legacy",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("50Gi"),
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				AWSElasticBlockStore: &corev1.AWSElasticBlockStoreVolumeSource{
					VolumeID:  "vol-abcdef",
					Partition: 2,
				},
			},
		},
	}

	info := PVToModel(pv)

	assertEqual(t, "Source.Type", info.Source.Type, "awsElasticBlockStore")
	assertEqual(t, "Source.AWSVolumeID", info.Source.AWSVolumeID, "vol-abcdef")
	if info.Source.AWSPartition != 2 {
		t.Errorf("Source.AWSPartition: want 2, got %d", info.Source.AWSPartition)
	}
	// No VolumeMode set — default to Filesystem
	assertEqual(t, "VolumeMode", info.VolumeMode, "Filesystem")
}

func TestPVToModel_NFSSource(t *testing.T) {
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pv-nfs",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Ti"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				NFS: &corev1.NFSVolumeSource{
					Server: "nfs.example.com",
					Path:   "/exports/data",
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeAvailable,
		},
	}

	info := PVToModel(pv)

	assertEqual(t, "Source.Type", info.Source.Type, "nfs")
	assertEqual(t, "Source.NFSServer", info.Source.NFSServer, "nfs.example.com")
	assertEqual(t, "Source.NFSPath", info.Source.NFSPath, "/exports/data")
	assertEqual(t, "Phase", info.Phase, "Available")

	if len(info.AccessModes) != 1 || info.AccessModes[0] != "ReadWriteMany" {
		t.Errorf("AccessModes: want [ReadWriteMany], got %v", info.AccessModes)
	}
}

// ---- PVC Tests ----

func TestPVCToModel_BoundToPV(t *testing.T) {
	scName := "gp3"
	volMode := corev1.PersistentVolumeFilesystem

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "data-pvc",
			Namespace:         "production",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)),
			Labels:            map[string]string{"app": "database"},
			Annotations:       map[string]string{"volume.kubernetes.io/selected-node": "node-1"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &scName,
			VolumeMode:       &volMode,
			VolumeName:       "pv-csi-001",
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("100Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("100Gi"),
			},
			Conditions: []corev1.PersistentVolumeClaimCondition{
				{
					Type:    corev1.PersistentVolumeClaimResizing,
					Status:  corev1.ConditionFalse,
					Reason:  "NotResizing",
					Message: "Volume is not being resized",
				},
			},
		},
	}

	info := PVCToModel(pvc)

	assertEqual(t, "Name", info.Name, "data-pvc")
	assertEqual(t, "Namespace", info.Namespace, "production")
	assertEqual(t, "Phase", info.Phase, "Bound")
	assertEqual(t, "StorageClassName", info.StorageClassName, "gp3")
	assertEqual(t, "VolumeMode", info.VolumeMode, "Filesystem")
	assertEqual(t, "VolumeName", info.VolumeName, "pv-csi-001")

	// 100Gi = 107374182400 bytes
	if info.RequestedBytes != 107374182400 {
		t.Errorf("RequestedBytes: want 107374182400, got %d", info.RequestedBytes)
	}
	if info.CapacityBytes != 107374182400 {
		t.Errorf("CapacityBytes: want 107374182400, got %d", info.CapacityBytes)
	}

	if len(info.AccessModes) != 1 || info.AccessModes[0] != "ReadWriteOnce" {
		t.Errorf("AccessModes: want [ReadWriteOnce], got %v", info.AccessModes)
	}

	// MountedByPods should be empty
	if len(info.MountedByPods) != 0 {
		t.Errorf("MountedByPods should be empty, got %d", len(info.MountedByPods))
	}

	// Conditions
	if len(info.Conditions) != 1 {
		t.Fatalf("Conditions len: want 1, got %d", len(info.Conditions))
	}
	assertEqual(t, "Condition.Type", info.Conditions[0].Type, "Resizing")
}

// ---- StorageClass Tests ----

func TestStorageClassToModel_Default(t *testing.T) {
	reclaimDelete := corev1.PersistentVolumeReclaimDelete
	bindImmediate := storagev1.VolumeBindingImmediate
	allowExpansion := true

	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gp3",
			Labels: map[string]string{"tier": "standard"},
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		Provisioner:          "ebs.csi.aws.com",
		ReclaimPolicy:        &reclaimDelete,
		VolumeBindingMode:    &bindImmediate,
		AllowVolumeExpansion: &allowExpansion,
		Parameters: map[string]string{
			"type":      "gp3",
			"encrypted": "true",
		},
		MountOptions: []string{"noatime", "nodiratime"},
	}

	info := StorageClassToModel(sc)

	assertEqual(t, "Name", info.Name, "gp3")
	assertEqual(t, "Provisioner", info.Provisioner, "ebs.csi.aws.com")
	assertEqual(t, "ReclaimPolicy", info.ReclaimPolicy, "Delete")
	assertEqual(t, "VolumeBindingMode", info.VolumeBindingMode, "Immediate")

	if !info.AllowVolumeExpansion {
		t.Error("AllowVolumeExpansion should be true")
	}
	if !info.IsDefault {
		t.Error("IsDefault should be true")
	}

	assertEqual(t, "Parameters[type]", info.Parameters["type"], "gp3")
	assertEqual(t, "Parameters[encrypted]", info.Parameters["encrypted"], "true")

	if len(info.MountOptions) != 2 {
		t.Fatalf("MountOptions len: want 2, got %d", len(info.MountOptions))
	}
}
