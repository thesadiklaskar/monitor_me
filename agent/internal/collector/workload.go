package collector

import (
	"regexp"
	"strings"
)

type workloadAttribution struct {
	CgroupPath        string
	WorkloadType      string
	WorkloadSelector  string
	AttributionMethod string
}

var containerIDPattern = regexp.MustCompile("[a-f0-9]{12,64}")

// parseWorkloadCgroup extracts stable workload selectors without assuming that
// every process belongs to a named service or container.
func parseWorkloadCgroup(raw string) workloadAttribution {
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		path := strings.TrimSpace(parts[2])
		if len(path) > 256 {
			path = path[:256]
		}
		if path == "" || path == "/" {
			continue
		}
		if strings.HasSuffix(path, ".service") {
			return workloadAttribution{
				CgroupPath: path, WorkloadType: "systemd",
				WorkloadSelector: path, AttributionMethod: "systemd",
			}
		}
		lowerPath := strings.ToLower(path)
		if strings.Contains(lowerPath, "docker") || strings.Contains(lowerPath, "containerd") ||
			strings.Contains(lowerPath, "kubepods") || strings.Contains(lowerPath, "libpod") ||
			strings.Contains(lowerPath, "crio") {
			if id := containerIDPattern.FindString(lowerPath); id != "" {
				return workloadAttribution{
					CgroupPath: path, WorkloadType: "container",
					WorkloadSelector: id, AttributionMethod: "container",
				}
			}
		}
		return workloadAttribution{
			CgroupPath: path, WorkloadType: "process_tree",
			WorkloadSelector: path, AttributionMethod: "cgroup",
		}
	}
	return workloadAttribution{AttributionMethod: "unclassified"}
}
