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
}
