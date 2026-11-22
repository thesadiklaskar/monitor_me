package collector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCounterRateRejectsResetAndInvalidElapsed(t *testing.T) {
	if rate, ok := CounterRate(100, 90, time.Second); ok || rate != 0 {
		t.Fatalf("counter reset must be missing, got rate=%v ok=%v", rate, ok)
	}
	if _, ok := CounterRate(10, 20, 0); ok {
		t.Fatal("zero elapsed time must not produce a rate")
	}
	rate, ok := CounterRate(10, 30, 2*time.Second)
	if !ok || rate != 10 {
		t.Fatalf("unexpected rate: %v, %v", rate, ok)
	}
}

func TestCPUPercentAndReset(t *testing.T) {
	previous := cpuCounters{user: 100, system: 50, idle: 850}
	current := cpuCounters{user: 130, system: 70, idle: 900}
	busy, user, system, _, ok := cpuPercent(previous, current)
	if !ok || busy != 50 || user != 30 || system != 20 {
		t.Fatalf("unexpected CPU percentages: %v %v %v %v", busy, user, system, ok)
	}
	current.user = 1
	if _, _, _, _, ok := cpuPercent(previous, current); ok {
		t.Fatal("CPU counter reset must invalidate the interval")
	}
}

func TestParseMemoryAndProcCounters(t *testing.T) {
	memory, err := parseMemInfo(strings.Join([]string{
		"MemTotal: 1000 kB", "MemAvailable: 400 kB", "MemFree: 300 kB",
		"Buffers: 20 kB", "Cached: 100 kB", "SReclaimable: 30 kB",
		"Shmem: 10 kB", "SwapTotal: 200 kB", "SwapFree: 150 kB",
	}, "\n"))
	if err != nil {
		t.Fatalf("parseMemInfo() error = %v", err)
	}
	if memory.total != 1024000 || memory.available != 409600 || memory.cache != 143360 {
		t.Fatalf("unexpected memory counters: %+v", memory)
	}

	cpu, err := parseCPUStat("cpu 10 2 3 20 1 0 0 0\ncpu0 5 1 1 10 0 0 0 0\n")
	if err != nil || len(cpu) != 2 {
		t.Fatalf("parseCPUStat() = %#v, %v", cpu, err)
	}
	disks, err := parseDiskStats("8 0 sda 10 0 20 0 5 0 40 0 0 0 0\n")
	if err != nil || disks["sda"].readBytes != 20*512 || disks["sda"].writeBytes != 40*512 {
		t.Fatalf("parseDiskStats() = %#v, %v", disks, err)
	}
	network, err := parseNetworkDev("Inter-| Receive | Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n eth0: 100 10 1 0 0 0 0 0 200 20 2 0 0 0 0 0\n")
	if err != nil || network["eth0"].transmitBytes != 200 || network["eth0"].receivePackets != 10 {
		t.Fatalf("parseNetworkDev() = %#v, %v", network, err)
	}
}

func TestProcCollectorCollectsFixture(t *testing.T) {
	root := t.TempDir()
	write := func(name, value string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("stat", "cpu 100 0 50 850 0 0 0 0\ncpu0 100 0 50 850 0 0 0 0\n")
	write("meminfo", "MemTotal: 1000 kB\nMemAvailable: 400 kB\nMemFree: 300 kB\nBuffers: 20 kB\nCached: 100 kB\nSReclaimable: 30 kB\nShmem: 10 kB\nSwapTotal: 200 kB\nSwapFree: 150 kB\n")
	write("diskstats", "8 0 sda 10 0 20 0 5 0 40 0 0 0 0\n")
	write("net/dev", "Inter-| Receive | Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n eth0: 100 10 1 0 0 0 0 0 200 20 2 0 0 0 0 0\n")
	write("uptime", "100.25 20.0\n")
	write("net/tcp", "  sl  local_address rem_address   st\n"+
		"   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 12345\n")
	write("net/tcp6", "  sl  local_address                         remote_address                        st\n")
	write("123/statm", "100 10 0 0 0 0 0\n")
	write("123/comm", "fixture-worker\n")
	write("123/cgroup", "0::/system.slice/example.service\n")
	collector := NewProcCollector(root)
	samples, err := collector.Collect(t.Context())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(samples) < 9 {
		t.Fatalf("expected host and agent gauge samples, got %d", len(samples))
	}
	foundMemory := false
	for _, sample := range samples {
		if sample.Name == "host.memory.used" && sample.Value == 600*1024 {
			foundMemory = true
		}
	}
	if !foundMemory {
		t.Fatal("expected the documented memory-used gauge")
	}
	foundWorkload := false
	for _, sample := range samples {
		if sample.Name == "process.memory.rss" &&
			sample.Labels["workload_type"] == "systemd" &&
			sample.Labels["workload_selector"] == "/system.slice/example.service" {
			foundWorkload = true
		}
	}
	if !foundWorkload {
		t.Fatal("expected process RSS to include explicit Systemd workload attribution")
	}
}

func TestParseWorkloadCgroupIdentifiesSystemdAndContainers(t *testing.T) {
	systemd := parseWorkloadCgroup("0::/system.slice/sshd.service\n")
	if systemd.WorkloadType != "systemd" || systemd.WorkloadSelector != "/system.slice/sshd.service" ||
		systemd.AttributionMethod != "systemd" {
		t.Fatalf("unexpected systemd attribution: %+v", systemd)
	}
	container := parseWorkloadCgroup("0::/system.slice/docker-abcdef012345.scope\n")
	if container.WorkloadType != "container" || container.WorkloadSelector != "abcdef012345" ||
		container.AttributionMethod != "container" {
		t.Fatalf("unexpected container attribution: %+v", container)
	}
	unknown := parseWorkloadCgroup("0::/user.slice/user-1000.slice/session-3.scope\n")
	if unknown.AttributionMethod != "cgroup" || unknown.WorkloadType != "process_tree" {
		t.Fatalf("unknown cgroup must remain explicit: %+v", unknown)
	}
}

func TestParseTCPTableCountsStatesAndListeningPorts(t *testing.T) {
	raw := "  sl  local_address rem_address   st\n" +
		"   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 12345\n" +
		"   1: 0100007F:1F91 00000000:0000 01 00000000:00000000 00:00000000 00000000 1000 0 12346\n"
	counts, listeners, err := parseTCPTable(raw, "ipv4")
	if err != nil {
		t.Fatalf("parseTCPTable() error = %v", err)
	}
	if counts["LISTEN"] != 1 || counts["ESTABLISHED"] != 1 || listeners[8080] != 1 {
		t.Fatalf("unexpected TCP inventory: states=%v listeners=%v", counts, listeners)
	}
}
