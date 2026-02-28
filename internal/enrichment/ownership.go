package enrichment

import (
	"fmt"
	"log/slog"

	"github.com/kubeadapt/kubeadapt-agent/pkg/model"
)

const maxOwnerDepth = 10

// OwnershipEnricher resolves pod ownership chains.
// It walks from Pod → ReplicaSet → Deployment (or other top-level owner),
// updating the pod's OwnerKind/OwnerName/OwnerUID to the top-level owner.
type OwnershipEnricher struct {
	replicaSets []model.ReplicaSetInfo
}

// NewOwnershipEnricher creates an enricher with the given ReplicaSets
// for ownership resolution. ReplicaSets are not in the snapshot since
// they are internal; the store passes them separately.
func NewOwnershipEnricher(replicaSets []model.ReplicaSetInfo) *OwnershipEnricher {
	return &OwnershipEnricher{replicaSets: replicaSets}
}

// Name returns the enricher name.
func (o *OwnershipEnricher) Name() string { return "ownership" }

// Enrich resolves ownership for each pod in the snapshot.
func (o *OwnershipEnricher) Enrich(snapshot *model.ClusterSnapshot) error {
	rsMap := o.buildReplicaSetMap()
	jobMap := buildJobMap(snapshot.Jobs)

	for i := range snapshot.Pods {
		pod := &snapshot.Pods[i]
		o.resolveOwner(pod, rsMap, jobMap)
	}
	return nil
}

// buildReplicaSetMap indexes ReplicaSets by "namespace/name".
func (o *OwnershipEnricher) buildReplicaSetMap() map[string]model.ReplicaSetInfo {
	m := make(map[string]model.ReplicaSetInfo, len(o.replicaSets))
	for _, rs := range o.replicaSets {
		key := fmt.Sprintf("%s/%s", rs.Namespace, rs.Name)
		m[key] = rs
	}
	return m
}

// buildJobMap indexes Jobs by "namespace/name".
func buildJobMap(jobs []model.JobInfo) map[string]model.JobInfo {
	m := make(map[string]model.JobInfo, len(jobs))
	for _, j := range jobs {
		key := fmt.Sprintf("%s/%s", j.Namespace, j.Name)
		m[key] = j
	}
	return m
}

// resolveOwner walks the ownership chain for a pod, stopping at the
// top-level owner or after maxOwnerDepth hops to prevent infinite loops.
func (o *OwnershipEnricher) resolveOwner(
	pod *model.PodInfo,
	rsMap map[string]model.ReplicaSetInfo,
	jobMap map[string]model.JobInfo,
) {
	if pod.OwnerKind == "" {
		return // orphan pod
	}
	if pod.OwnerKind == "Node" {
		return // static pod
	}

	kind := pod.OwnerKind
	name := pod.OwnerName
	uid := pod.OwnerUID
	ns := pod.Namespace

	for depth := 0; depth < maxOwnerDepth; depth++ {
		resolved := false

		switch kind {
		case "ReplicaSet":
			key := fmt.Sprintf("%s/%s", ns, name)
			rs, ok := rsMap[key]
			if !ok {
				break
			}
			if rs.OwnerKind == "" {
				break // standalone RS
			}
			kind = rs.OwnerKind
			name = rs.OwnerName
			uid = rs.OwnerUID
			resolved = true

		case "Job":
			key := fmt.Sprintf("%s/%s", ns, name)
			job, ok := jobMap[key]
			if !ok {
				break
			}
			if job.OwnerCronJob == "" {
				break // standalone Job
			}
			kind = "CronJob"
			name = job.OwnerCronJob
			uid = "" // CronJob UID not stored on JobInfo
			resolved = true
		}

		if !resolved {
			break
		}
	}

	if kind != pod.OwnerKind || name != pod.OwnerName {
		slog.Debug("resolved pod owner",
			"pod", pod.Name,
			"namespace", pod.Namespace,
			"from", pod.OwnerKind+"/"+pod.OwnerName,
			"to", kind+"/"+name,
		)
	}

	pod.OwnerKind = kind
	pod.OwnerName = name
	pod.OwnerUID = uid
}
