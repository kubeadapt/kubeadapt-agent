package gpu

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

const (
	// sentinelThreshold is the threshold above which DCGM metric values are
	// treated as "blank" sentinel values (~1.8e19) and rejected.
	sentinelThreshold = 1e15

	// mibToBytes converts mebibytes to bytes.
	mibToBytes = 1048576
)

const (
	metricProfGrEngineActive = "DCGM_FI_PROF_GR_ENGINE_ACTIVE"
	metricProfTensorActive   = "DCGM_FI_PROF_PIPE_TENSOR_ACTIVE"
	metricDevMemCopyUtil     = "DCGM_FI_DEV_MEM_COPY_UTIL"
	metricDevGPUUtil         = "DCGM_FI_DEV_GPU_UTIL"
	metricDevFBUsed          = "DCGM_FI_DEV_FB_USED"
	metricDevFBFree          = "DCGM_FI_DEV_FB_FREE"
	metricDevFBTotal         = "DCGM_FI_DEV_FB_TOTAL"
	metricDevGPUTemp         = "DCGM_FI_DEV_GPU_TEMP"
	metricDevPowerUsage      = "DCGM_FI_DEV_POWER_USAGE"
	metricDevMIGMode         = "DCGM_FI_DEV_MIG_MODE"
)

type dcgmLabels struct {
	gpu           string
	uuid          string
	device        string
	modelName     string
	driverVersion string
	hostname      string
	podName       string
	namespace     string
	containerName string
	gpuInstanceID string
	gpuProfile    string
}

// parsedSample represents a single parsed Prometheus metric sample.
type parsedSample struct {
	name   string
	labels dcgmLabels
	value  float64
}

// ParseDCGMMetrics parses Prometheus exposition text from dcgm-exporter and returns
// per-GPU device metrics. It handles both old-style (pod_name, pod_namespace, container_name)
// and new-style (pod, namespace, container) label schemas.
func ParseDCGMMetrics(data []byte) ([]GPUDeviceMetrics, error) {
	samples := parsePrometheusText(data)

	gpus := make(map[string]*GPUDeviceMetrics)
	hasProf := make(map[string]bool)

	for _, s := range samples {
		if s.labels.uuid == "" && s.labels.gpu == "" {
			continue
		}

		key := s.labels.uuid
		if key == "" {
			key = s.labels.gpu
		}

		gpu := getOrCreateGPU(gpus, key, s.labels)

		switch s.name {
		case metricProfGrEngineActive:
			if isSentinel(s.value) {
				continue
			}
			pct := s.value * 100
			gpu.GPUUtilization = &pct
			hasProf[key] = true

		case metricDevGPUUtil:
			if isSentinel(s.value) {
				continue
			}
			if !hasProf[key] {
				v := s.value
				gpu.GPUUtilization = &v
			}

		case metricProfTensorActive:
			if isSentinel(s.value) {
				continue
			}
			pct := s.value * 100
			gpu.TensorActivePercent = &pct

		case metricDevMemCopyUtil:
			if isSentinel(s.value) {
				continue
			}
			v := s.value
			gpu.MemCopyUtilPercent = &v

		case metricDevFBUsed:
			if isSentinel(s.value) {
				continue
			}
			b := int64(s.value * mibToBytes)
			gpu.MemoryUsedBytes = &b

		case metricDevFBFree:
			if isSentinel(s.value) {
				continue
			}
			b := int64(s.value * mibToBytes)
			gpu.MemoryFreeBytes = &b

		case metricDevFBTotal:
			if isSentinel(s.value) {
				continue
			}
			b := int64(s.value * mibToBytes)
			gpu.MemoryTotalBytes = &b

		case metricDevGPUTemp:
			if isSentinel(s.value) {
				continue
			}
			v := s.value
			gpu.Temperature = &v

		case metricDevPowerUsage:
			if isSentinel(s.value) {
				continue
			}
			v := s.value
			gpu.PowerUsage = &v

		case metricDevMIGMode:
			if isSentinel(s.value) {
				continue
			}
			enabled := s.value == 1
			gpu.MIGEnabled = &enabled
		}
	}

	result := make([]GPUDeviceMetrics, 0, len(gpus))
	for _, gpu := range gpus {
		if gpu.MemoryTotalBytes == nil && gpu.MemoryUsedBytes != nil && gpu.MemoryFreeBytes != nil {
			total := *gpu.MemoryUsedBytes + *gpu.MemoryFreeBytes
			gpu.MemoryTotalBytes = &total
		}
		result = append(result, *gpu)
	}
	return result, nil
}

// parsePrometheusText parses Prometheus exposition text format line-by-line,
// extracting metric samples with their labels and values.
func parsePrometheusText(data []byte) []parsedSample {
	var samples []parsedSample
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		s, ok := parseSampleLine(line)
		if !ok {
			continue
		}
		samples = append(samples, s)
	}

	return samples
}

// parseSampleLine parses a single Prometheus metric line:
//
//	metric_name{label1="val1",label2="val2"} value [timestamp]
func parseSampleLine(line string) (parsedSample, bool) {
	var s parsedSample

	braceStart := strings.IndexByte(line, '{')
	if braceStart < 0 {
		// No labels: "name value"
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return s, false
		}
		s.name = parts[0]
		v, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return s, false
		}
		s.value = v
		return s, true
	}

	s.name = line[:braceStart]

	braceEnd := strings.LastIndexByte(line, '}')
	if braceEnd <= braceStart {
		return s, false
	}

	s.labels = parseLabels(line[braceStart+1 : braceEnd])

	valueStr := strings.TrimSpace(line[braceEnd+1:])
	parts := strings.Fields(valueStr)
	if len(parts) == 0 {
		return s, false
	}
	v, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return s, false
	}
	s.value = v

	return s, true
}

// parseLabels parses the label portion of a Prometheus metric line:
//
//	label1="val1",label2="val2"
//
// It handles escaped characters within quoted label values.
func parseLabels(s string) dcgmLabels {
	var l dcgmLabels
	for len(s) > 0 {
		// Find key=
		eq := strings.IndexByte(s, '=')
		if eq < 0 {
			break
		}
		key := s[:eq]
		s = s[eq+1:]

		// Expect opening quote
		if len(s) == 0 || s[0] != '"' {
			break
		}
		s = s[1:]

		// Read value until unescaped closing quote
		var val strings.Builder
		i := 0
		for i < len(s) {
			if s[i] == '\\' && i+1 < len(s) {
				switch s[i+1] {
				case '"':
					val.WriteByte('"')
				case '\\':
					val.WriteByte('\\')
				case 'n':
					val.WriteByte('\n')
				default:
					val.WriteByte('\\')
					val.WriteByte(s[i+1])
				}
				i += 2
				continue
			}
			if s[i] == '"' {
				break
			}
			val.WriteByte(s[i])
			i++
		}

		value := val.String()
		if i < len(s) {
			s = s[i+1:] // skip closing quote
		} else {
			s = ""
		}

		// Skip comma separator
		if len(s) > 0 && s[0] == ',' {
			s = s[1:]
		}

		// Map label to struct field, normalizing old/new schemas.
		// New-style labels always overwrite; old-style only set if empty.
		switch key {
		case "gpu":
			l.gpu = value
		case "UUID", "uuid":
			l.uuid = value
		case "device":
			l.device = value
		case "modelName":
			l.modelName = value
		case "DCGM_FI_DRIVER_VERSION":
			l.driverVersion = value
		case "Hostname":
			l.hostname = value
		// New-style labels (take priority)
		case "pod":
			l.podName = value
		case "namespace":
			l.namespace = value
		case "container":
			l.containerName = value
		// Old-style labels (fallback)
		case "pod_name":
			if l.podName == "" {
				l.podName = value
			}
		case "pod_namespace":
			if l.namespace == "" {
				l.namespace = value
			}
		case "container_name":
			if l.containerName == "" {
				l.containerName = value
			}
		case "GPU_I_ID":
			l.gpuInstanceID = value
		case "GPU_I_PROFILE":
			l.gpuProfile = value
		}
	}
	return l
}

// getOrCreateGPU returns the GPUDeviceMetrics for the given key, creating it if needed.
func getOrCreateGPU(gpus map[string]*GPUDeviceMetrics, key string, labels dcgmLabels) *GPUDeviceMetrics {
	if g, ok := gpus[key]; ok {
		return g
	}
	g := &GPUDeviceMetrics{
		GPU:           labels.gpu,
		UUID:          labels.uuid,
		Device:        labels.device,
		ModelName:     labels.modelName,
		DriverVersion: labels.driverVersion,
		Hostname:      labels.hostname,
		PodName:       labels.podName,
		Namespace:     labels.namespace,
		ContainerName: labels.containerName,
		GPUInstanceID: labels.gpuInstanceID,
		GPUProfile:    labels.gpuProfile,
	}
	gpus[key] = g
	return g
}

// isSentinel returns true if the value is a DCGM sentinel ("blank") value.
// DCGM uses very large values (~1.8e19) to indicate missing/blank metrics.
func isSentinel(v float64) bool {
	return v > sentinelThreshold
}
