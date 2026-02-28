package convert

import (
	"testing"
)

func TestParseProviderID_AWS(t *testing.T) {
	id, az, region := ParseProviderID("aws:///us-east-1a/i-1234567890abcdef0")
	if id != "i-1234567890abcdef0" {
		t.Errorf("instanceID = %q, want %q", id, "i-1234567890abcdef0")
	}
	if az != "us-east-1a" {
		t.Errorf("az = %q, want %q", az, "us-east-1a")
	}
	if region != "us-east-1" {
		t.Errorf("region = %q, want %q", region, "us-east-1")
	}
}

func TestParseProviderID_AWS_Fargate(t *testing.T) {
	// Fargate nodes typically have provider IDs like:
	// aws:///us-east-1a/fargate-ip-10-0-1-123.ec2.internal/...
	// or just aws:///us-east-1a/some-fargate-id
	id, az, region := ParseProviderID("aws:///us-west-2b/fargate-ip-10-0-1-123")
	if id != "fargate-ip-10-0-1-123" {
		t.Errorf("instanceID = %q, want %q", id, "fargate-ip-10-0-1-123")
	}
	if az != "us-west-2b" {
		t.Errorf("az = %q, want %q", az, "us-west-2b")
	}
	if region != "us-west-2" {
		t.Errorf("region = %q, want %q", region, "us-west-2")
	}
}

func TestParseProviderID_GCE(t *testing.T) {
	id, az, region := ParseProviderID("gce://my-project/us-central1-a/my-instance")
	if id != "my-instance" {
		t.Errorf("instanceID = %q, want %q", id, "my-instance")
	}
	if az != "us-central1-a" {
		t.Errorf("az = %q, want %q", az, "us-central1-a")
	}
	if region != "us-central1" {
		t.Errorf("region = %q, want %q", region, "us-central1")
	}
}

func TestParseProviderID_GCE_MultiDash(t *testing.T) {
	id, az, region := ParseProviderID("gce://my-project/europe-west4-b/gke-node-pool-abc123")
	if id != "gke-node-pool-abc123" {
		t.Errorf("instanceID = %q, want %q", id, "gke-node-pool-abc123")
	}
	if az != "europe-west4-b" {
		t.Errorf("az = %q, want %q", az, "europe-west4-b")
	}
	if region != "europe-west4" {
		t.Errorf("region = %q, want %q", region, "europe-west4")
	}
}

func TestParseProviderID_Azure(t *testing.T) {
	providerID := "azure:///subscriptions/sub-123/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm-name"
	id, az, region := ParseProviderID(providerID)
	if id != "vm-name" {
		t.Errorf("instanceID = %q, want %q", id, "vm-name")
	}
	if az != "" {
		t.Errorf("az = %q, want empty", az)
	}
	if region != "" {
		t.Errorf("region = %q, want empty", region)
	}
}

func TestParseProviderID_Unknown(t *testing.T) {
	id, az, region := ParseProviderID("some-random-string")
	if id != "some-random-string" {
		t.Errorf("instanceID = %q, want %q", id, "some-random-string")
	}
	if az != "" {
		t.Errorf("az = %q, want empty", az)
	}
	if region != "" {
		t.Errorf("region = %q, want empty", region)
	}
}

func TestParseProviderID_Empty(t *testing.T) {
	id, az, region := ParseProviderID("")
	if id != "" {
		t.Errorf("instanceID = %q, want empty", id)
	}
	if az != "" {
		t.Errorf("az = %q, want empty", az)
	}
	if region != "" {
		t.Errorf("region = %q, want empty", region)
	}
}

func TestExtractCapacityType_EKS_OnDemand(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/capacityType": "ON_DEMAND",
	}
	got := ExtractCapacityType(labels)
	if got != "on-demand" {
		t.Errorf("ExtractCapacityType = %q, want %q", got, "on-demand")
	}
}

func TestExtractCapacityType_EKS_Spot(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/capacityType": "SPOT",
	}
	got := ExtractCapacityType(labels)
	if got != "spot" {
		t.Errorf("ExtractCapacityType = %q, want %q", got, "spot")
	}
}

func TestExtractCapacityType_Karpenter_OnDemand(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/capacity-type": "on-demand",
	}
	got := ExtractCapacityType(labels)
	if got != "on-demand" {
		t.Errorf("ExtractCapacityType = %q, want %q", got, "on-demand")
	}
}

func TestExtractCapacityType_Karpenter_Spot(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/capacity-type": "spot",
	}
	got := ExtractCapacityType(labels)
	if got != "spot" {
		t.Errorf("ExtractCapacityType = %q, want %q", got, "spot")
	}
}

func TestExtractCapacityType_Generic_Spot(t *testing.T) {
	labels := map[string]string{
		"node.kubernetes.io/lifecycle": "spot",
	}
	got := ExtractCapacityType(labels)
	if got != "spot" {
		t.Errorf("ExtractCapacityType = %q, want %q", got, "spot")
	}
}

func TestExtractCapacityType_Generic_Normal(t *testing.T) {
	labels := map[string]string{
		"node.kubernetes.io/lifecycle": "normal",
	}
	got := ExtractCapacityType(labels)
	if got != "on-demand" {
		t.Errorf("ExtractCapacityType = %q, want %q", got, "on-demand")
	}
}

func TestExtractCapacityType_NoLabel(t *testing.T) {
	labels := map[string]string{
		"some-other-label": "value",
	}
	got := ExtractCapacityType(labels)
	if got != "" {
		t.Errorf("ExtractCapacityType = %q, want empty", got)
	}
}

func TestExtractCapacityType_Nil(t *testing.T) {
	got := ExtractCapacityType(nil)
	if got != "" {
		t.Errorf("ExtractCapacityType = %q, want empty", got)
	}
}

func TestExtractCapacityType_EKS_Fargate(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/compute-type": "fargate",
	}
	got := ExtractCapacityType(labels)
	if got != "fargate" {
		t.Errorf("ExtractCapacityType = %q, want %q", got, "fargate")
	}
}

func TestExtractNodeGroup_EKS(t *testing.T) {
	labels := map[string]string{
		"eks.amazonaws.com/nodegroup": "my-node-group",
	}
	got := ExtractNodeGroup(labels)
	if got != "my-node-group" {
		t.Errorf("ExtractNodeGroup = %q, want %q", got, "my-node-group")
	}
}

func TestExtractNodeGroup_Karpenter(t *testing.T) {
	labels := map[string]string{
		"karpenter.sh/nodepool": "default",
	}
	got := ExtractNodeGroup(labels)
	if got != "default" {
		t.Errorf("ExtractNodeGroup = %q, want %q", got, "default")
	}
}

func TestExtractNodeGroup_NoLabel(t *testing.T) {
	labels := map[string]string{
		"some-label": "value",
	}
	got := ExtractNodeGroup(labels)
	if got != "" {
		t.Errorf("ExtractNodeGroup = %q, want empty", got)
	}
}

func TestExtractNodeGroup_Nil(t *testing.T) {
	got := ExtractNodeGroup(nil)
	if got != "" {
		t.Errorf("ExtractNodeGroup = %q, want empty", got)
	}
}

func TestExtractNodeGroup_EKS_Priority(t *testing.T) {
	// EKS label takes priority over Karpenter
	labels := map[string]string{
		"eks.amazonaws.com/nodegroup": "eks-group",
		"karpenter.sh/nodepool":       "karpenter-pool",
	}
	got := ExtractNodeGroup(labels)
	if got != "eks-group" {
		t.Errorf("ExtractNodeGroup = %q, want %q (EKS should take priority)", got, "eks-group")
	}
}
