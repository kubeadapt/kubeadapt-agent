package model

// JobInfo represents a Kubernetes Job.
type JobInfo struct {
	Name         string `json:"name"`
	UID          string `json:"uid"`
	Namespace    string `json:"namespace"`
	OwnerCronJob string `json:"owner_cronjob"`

	Completions             *int32 `json:"completions,omitempty"`
	Parallelism             *int32 `json:"parallelism,omitempty"`
	BackoffLimit            *int32 `json:"backoff_limit,omitempty"`
	ActiveDeadlineSeconds   *int64 `json:"active_deadline_seconds,omitempty"`
	TTLSecondsAfterFinished *int32 `json:"ttl_seconds_after_finished,omitempty"`

	Active         int32  `json:"active"`
	Succeeded      int32  `json:"succeeded"`
	Failed         int32  `json:"failed"`
	StartTime      *int64 `json:"start_time,omitempty"`
	CompletionTime *int64 `json:"completion_time,omitempty"`

	DurationSeconds *float64 `json:"duration_seconds,omitempty"`

	TotalCPURequest    float64  `json:"total_cpu_request"`
	TotalMemoryRequest int64    `json:"total_memory_request"`
	TotalCPUUsage      *float64 `json:"total_cpu_usage,omitempty"`
	TotalMemoryUsage   *int64   `json:"total_memory_usage,omitempty"`

	Labels            map[string]string  `json:"labels"`
	Annotations       map[string]string  `json:"annotations"`
	CreationTimestamp int64              `json:"creation_timestamp"`
	Conditions        []JobConditionInfo `json:"conditions"`
}

// CronJobInfo represents a Kubernetes CronJob.
type CronJobInfo struct {
	Name               string `json:"name"`
	UID                string `json:"uid"`
	Namespace          string `json:"namespace"`
	Schedule           string `json:"schedule"`
	Suspend            bool   `json:"suspend"`
	ConcurrencyPolicy  string `json:"concurrency_policy"`
	LastScheduleTime   *int64 `json:"last_schedule_time,omitempty"`
	LastSuccessfulTime *int64 `json:"last_successful_time,omitempty"`

	ActiveJobs []string `json:"active_jobs"`

	ContainerSpecs []ContainerSpecInfo `json:"container_specs"`

	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreationTimestamp int64             `json:"creation_timestamp"`
}

// JobConditionInfo represents a job condition (Complete, Failed, etc.).
type JobConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}
