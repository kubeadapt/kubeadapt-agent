package enrichment

import (
	"fmt"
	"testing"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

func TestOwnership_PodToRSToDeployment(t *testing.T) {
	rs := []model.ReplicaSetInfo{{
		Name:      "nginx-abc123",
		Namespace: "default",
		OwnerKind: "Deployment",
		OwnerName: "nginx",
		OwnerUID:  "deploy-uid-1",
	}}
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{{
			Name:      "nginx-abc123-xyz",
			Namespace: "default",
			OwnerKind: "ReplicaSet",
			OwnerName: "nginx-abc123",
			OwnerUID:  "rs-uid-1",
		}},
	}

	e := NewOwnershipEnricher(rs)
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pod := snap.Pods[0]
	if pod.OwnerKind != "Deployment" {
		t.Errorf("expected OwnerKind=Deployment, got %s", pod.OwnerKind)
	}
	if pod.OwnerName != "nginx" {
		t.Errorf("expected OwnerName=nginx, got %s", pod.OwnerName)
	}
	if pod.OwnerUID != "deploy-uid-1" {
		t.Errorf("expected OwnerUID=deploy-uid-1, got %s", pod.OwnerUID)
	}
}

func TestOwnership_PodToRSToArgoRollout(t *testing.T) {
	rs := []model.ReplicaSetInfo{{
		Name:      "rollout-rs-abc",
		Namespace: "default",
		OwnerKind: "Rollout",
		OwnerName: "my-rollout",
		OwnerUID:  "rollout-uid-1",
	}}
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{{
			Name:      "rollout-rs-abc-pod",
			Namespace: "default",
			OwnerKind: "ReplicaSet",
			OwnerName: "rollout-rs-abc",
			OwnerUID:  "rs-uid-2",
		}},
	}

	e := NewOwnershipEnricher(rs)
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pod := snap.Pods[0]
	if pod.OwnerKind != "Rollout" {
		t.Errorf("expected OwnerKind=Rollout, got %s", pod.OwnerKind)
	}
	if pod.OwnerName != "my-rollout" {
		t.Errorf("expected OwnerName=my-rollout, got %s", pod.OwnerName)
	}
}

func TestOwnership_PodToDaemonSet(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{{
			Name:      "fluentd-xyz",
			Namespace: "kube-system",
			OwnerKind: "DaemonSet",
			OwnerName: "fluentd",
			OwnerUID:  "ds-uid-1",
		}},
	}

	e := NewOwnershipEnricher(nil)
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pod := snap.Pods[0]
	if pod.OwnerKind != "DaemonSet" {
		t.Errorf("expected OwnerKind=DaemonSet, got %s", pod.OwnerKind)
	}
	if pod.OwnerName != "fluentd" {
		t.Errorf("expected OwnerName=fluentd, got %s", pod.OwnerName)
	}
}

func TestOwnership_StandaloneRS(t *testing.T) {
	rs := []model.ReplicaSetInfo{{
		Name:      "standalone-rs",
		Namespace: "default",
		// No owner
	}}
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{{
			Name:      "standalone-rs-pod",
			Namespace: "default",
			OwnerKind: "ReplicaSet",
			OwnerName: "standalone-rs",
			OwnerUID:  "rs-uid-3",
		}},
	}

	e := NewOwnershipEnricher(rs)
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pod := snap.Pods[0]
	if pod.OwnerKind != "ReplicaSet" {
		t.Errorf("expected OwnerKind=ReplicaSet, got %s", pod.OwnerKind)
	}
	if pod.OwnerName != "standalone-rs" {
		t.Errorf("expected OwnerName=standalone-rs, got %s", pod.OwnerName)
	}
}

func TestOwnership_StaticPod(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{{
			Name:      "kube-apiserver-master",
			Namespace: "kube-system",
			OwnerKind: "Node",
			OwnerName: "master-node",
			OwnerUID:  "node-uid-1",
		}},
	}

	e := NewOwnershipEnricher(nil)
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pod := snap.Pods[0]
	if pod.OwnerKind != "Node" {
		t.Errorf("expected OwnerKind=Node, got %s", pod.OwnerKind)
	}
	if pod.OwnerName != "master-node" {
		t.Errorf("expected OwnerName=master-node, got %s", pod.OwnerName)
	}
}

func TestOwnership_OrphanPod(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{{
			Name:      "orphan-pod",
			Namespace: "default",
			// No owner
		}},
	}

	e := NewOwnershipEnricher(nil)
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pod := snap.Pods[0]
	if pod.OwnerKind != "" {
		t.Errorf("expected empty OwnerKind, got %s", pod.OwnerKind)
	}
	if pod.OwnerName != "" {
		t.Errorf("expected empty OwnerName, got %s", pod.OwnerName)
	}
}

func TestOwnership_MaxDepthPreventsInfiniteLoop(t *testing.T) {
	// Create a chain of ReplicaSets that each point to another RS.
	// This should stop after maxOwnerDepth iterations.
	var rsList []model.ReplicaSetInfo
	for i := 0; i < maxOwnerDepth+5; i++ {
		rs := model.ReplicaSetInfo{
			Name:      rsName(i),
			Namespace: "default",
			OwnerKind: "ReplicaSet",
			OwnerName: rsName(i + 1),
			OwnerUID:  rsUID(i + 1),
		}
		rsList = append(rsList, rs)
	}
	// Make the last RS point to a Deployment (but it won't be reached).
	rsList[maxOwnerDepth+4].OwnerKind = "Deployment"
	rsList[maxOwnerDepth+4].OwnerName = "final-deploy"
	rsList[maxOwnerDepth+4].OwnerUID = "final-uid"

	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{{
			Name:      "deep-pod",
			Namespace: "default",
			OwnerKind: "ReplicaSet",
			OwnerName: rsName(0),
			OwnerUID:  rsUID(0),
		}},
	}

	e := NewOwnershipEnricher(rsList)
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pod := snap.Pods[0]
	// After maxOwnerDepth hops, the chain should stop at some intermediate RS.
	// The pod should NOT still reference rsName(0) because it followed the chain.
	if pod.OwnerKind != "ReplicaSet" {
		t.Errorf("expected OwnerKind=ReplicaSet (stopped at chain), got %s", pod.OwnerKind)
	}
	if pod.OwnerName == rsName(0) {
		t.Errorf("expected owner to have changed from %s", rsName(0))
	}
}

func TestOwnership_JobToCronJob(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{{
			Name:      "backup-job-abc-pod",
			Namespace: "default",
			OwnerKind: "Job",
			OwnerName: "backup-job-abc",
			OwnerUID:  "job-uid-1",
		}},
		Jobs: []model.JobInfo{{
			Name:         "backup-job-abc",
			Namespace:    "default",
			OwnerCronJob: "backup-cron",
		}},
	}

	e := NewOwnershipEnricher(nil)
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pod := snap.Pods[0]
	if pod.OwnerKind != "CronJob" {
		t.Errorf("expected OwnerKind=CronJob, got %s", pod.OwnerKind)
	}
	if pod.OwnerName != "backup-cron" {
		t.Errorf("expected OwnerName=backup-cron, got %s", pod.OwnerName)
	}
}

func TestOwnership_StandaloneJob(t *testing.T) {
	snap := &model.ClusterSnapshot{
		Pods: []model.PodInfo{{
			Name:      "migration-pod",
			Namespace: "default",
			OwnerKind: "Job",
			OwnerName: "migration-job",
			OwnerUID:  "job-uid-2",
		}},
		Jobs: []model.JobInfo{{
			Name:      "migration-job",
			Namespace: "default",
			// No OwnerCronJob
		}},
	}

	e := NewOwnershipEnricher(nil)
	if err := e.Enrich(snap); err != nil {
		t.Fatal(err)
	}

	pod := snap.Pods[0]
	if pod.OwnerKind != "Job" {
		t.Errorf("expected OwnerKind=Job, got %s", pod.OwnerKind)
	}
	if pod.OwnerName != "migration-job" {
		t.Errorf("expected OwnerName=migration-job, got %s", pod.OwnerName)
	}
}

func rsName(i int) string {
	return fmt.Sprintf("rs-%d", i)
}

func rsUID(i int) string {
	return fmt.Sprintf("rs-uid-%d", i)
}
