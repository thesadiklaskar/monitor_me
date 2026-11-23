//go:build windows

package collector

import (
	"context"
	"testing"
)

func TestWindowsCollectorReportsHostVitals(t *testing.T) {
	collector := NewProcCollector("")
	samples, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect Windows host metrics: %v", err)
	}

	values := make(map[string]Sample, len(samples))
	for _, sample := range samples {
		values[sample.Name] = sample
	}
	for _, name := range []string{
		"host.memory.total",
		"host.memory.used",
		"host.uptime.seconds",
		"host.process.count",
		"host.info",
	} {
		if _, ok := values[name]; !ok {
			t.Errorf("required Windows host metric %q was not collected", name)
		}
	}
	if values["host.memory.total"].Value <= 0 {
		t.Error("host.memory.total must be positive")
	}
	if values["host.process.count"].Value <= 0 {
		t.Error("host.process.count must be positive")
	}
	if values["host.info"].Labels["os"] != "windows" {
		t.Errorf("host.info os label = %q, want windows", values["host.info"].Labels["os"])
	}
}

func TestWindowsCollectorAdvertisesOnlySupportedHostCapabilities(t *testing.T) {
	capabilities := NewProcCollector("").Capabilities()
	required := map[string]bool{
		"host.cpu": false,
		"host.memory": false,
		"host.disk": false,
		"host.network": false,
		"host.system_info": false,
		"agent.health": false,
	}
	for _, capability := range capabilities {
		if _, exists := required[capability]; exists {
			required[capability] = true
		}
	}
	for capability, present := range required {
		if !present {
			t.Errorf("Windows capabilities do not include %q", capability)
		}
	}
	for _, capability := range capabilities {
		if capability == "host.network.tcp" || capability == "workload.systemd" || capability == "workload.container" {
			t.Errorf("Windows collector must not advertise unsupported capability %q", capability)
		}
	}
}
