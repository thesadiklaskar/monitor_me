package collector

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Sample is a timestamped telemetry value stored in the local write-ahead log.
type Sample struct {
	Name                string            `json:"name"`
	Value               float64           `json:"value"`
	Unit                string            `json:"unit"`
	Labels              map[string]string `json:"labels,omitempty"`
	ObservedAtUnixNanos int64             `json:"observed_at_unix_nanos"`
}

type cpuCounters struct {
	user, nice, system, idle, iowait, irq, softirq, steal float64
}

func (c cpuCounters) total() float64 {
	return c.user + c.nice + c.system + c.idle + c.iowait + c.irq + c.softirq + c.steal
}

type diskCounters struct {
	readBytes, writeBytes, readOps, writeOps float64
}

type networkCounters struct {
	receiveBytes, transmitBytes, receivePackets, transmitPackets, receiveErrors, transmitErrors float64
}

// ProcCollector reads Linux kernel counters and retains only the previous
// snapshot required to calculate rates over monotonic elapsed time.
type ProcCollector struct {
	root            string
	startedAt       time.Time
	previousAt      time.Time
	previousCPU     map[string]cpuCounters
	previousDisk    map[string]diskCounters
	previousNetwork map[string]networkCounters
}

func NewProcCollector(root string) *ProcCollector {
	if root == "" {
		root = "/proc"
	}
	return &ProcCollector{
		root: root, startedAt: time.Now(),
		previousCPU:     make(map[string]cpuCounters),
		previousDisk:    make(map[string]diskCounters),
		previousNetwork: make(map[string]networkCounters),
	}
}

// Capabilities reports only metrics supported by this collector.
func (c *ProcCollector) Capabilities() []string {
	if runtime.GOOS != "linux" {
		return []string{"agent.health", "agent.spool"}
	}
	return []string{
		"host.cpu", "host.cpu.per_core", "host.memory", "host.swap",
		"host.disk", "host.network", "host.network.tcp", "host.uptime",
		"host.process_inventory", "workload.cgroup", "workload.systemd", "workload.container",
		"agent.health", "agent.spool",
	}
}

func (c *ProcCollector) Collect(ctx context.Context) ([]Sample, error) {
	if runtime.GOOS != "linux" {
		return nil, errors.New("procfs collection is supported only on Linux")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	now := time.Now()
	observedAt := now.UTC().UnixNano()
	samples := make([]Sample, 0, 96)
	add := func(name string, value float64, unit string, labels map[string]string) {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return
		}
		samples = append(samples, Sample{
			Name: name, Value: value, Unit: unit, Labels: labels,
			ObservedAtUnixNanos: observedAt,
		})
	}

	var collectionErrors []error
	cpuRaw, err := os.ReadFile(filepath.Join(c.root, "stat"))
	if err != nil {
		return nil, fmt.Errorf("read procfs CPU counters: %w", err)
	}
	cpuNow, err := parseCPUStat(string(cpuRaw))
	if err != nil {
		return nil, err
	}
	if !c.previousAt.IsZero() {
		for name, current := range cpuNow {
			previous, exists := c.previousCPU[name]
			if !exists {
				continue
			}
			busyPercent, userPercent, systemPercent, waitPercent, ok := cpuPercent(previous, current)
			if !ok {
				continue
			}
			labels := map[string]string{"cpu": name}
			metric := "host.cpu.utilization"
			if name != "cpu" {
				metric = "host.cpu.core.utilization"
			}
			add(metric, busyPercent, "percent", labels)
			if name == "cpu" {
				add("host.cpu.user", userPercent, "percent", nil)
				add("host.cpu.system", systemPercent, "percent", nil)
				add("host.cpu.iowait", waitPercent, "percent", nil)
			}
		}
	}
	c.previousCPU = cpuNow

	memRaw, err := os.ReadFile(filepath.Join(c.root, "meminfo"))
	if err != nil {
		collectionErrors = append(collectionErrors, fmt.Errorf("read procfs memory counters: %w", err))
	} else if memory, parseErr := parseMemInfo(string(memRaw)); parseErr != nil {
		collectionErrors = append(collectionErrors, parseErr)
	} else {
		add("host.memory.total", memory.total, "bytes", nil)
		add("host.memory.available", memory.available, "bytes", nil)
		add("host.memory.used", math.Max(0, memory.total-memory.available), "bytes", nil)
		add("host.memory.cache", memory.cache, "bytes", nil)
		add("host.memory.swap.total", memory.swapTotal, "bytes", nil)
		add("host.memory.swap.used", math.Max(0, memory.swapTotal-memory.swapFree), "bytes", nil)
	}

	diskRaw, err := os.ReadFile(filepath.Join(c.root, "diskstats"))
	if err != nil {
		collectionErrors = append(collectionErrors, fmt.Errorf("read procfs disk counters: %w", err))
	} else if current, parseErr := parseDiskStats(string(diskRaw)); parseErr != nil {
		collectionErrors = append(collectionErrors, parseErr)
	} else {
		if !c.previousAt.IsZero() {
			elapsed := now.Sub(c.previousAt)
			for device, counters := range current {
				previous, exists := c.previousDisk[device]
				if !exists {
					continue
				}
				labels := map[string]string{"device": device}
				if rate, ok := CounterRate(previous.readBytes, counters.readBytes, elapsed); ok {
					add("host.disk.read.bytes_per_second", rate, "bytes/second", labels)
				}
				if rate, ok := CounterRate(previous.writeBytes, counters.writeBytes, elapsed); ok {
					add("host.disk.write.bytes_per_second", rate, "bytes/second", labels)
				}
				if rate, ok := CounterRate(previous.readOps, counters.readOps, elapsed); ok {
					add("host.disk.read.operations_per_second", rate, "operations/second", labels)
				}
				if rate, ok := CounterRate(previous.writeOps, counters.writeOps, elapsed); ok {
					add("host.disk.write.operations_per_second", rate, "operations/second", labels)
				}
			}
		}
		c.previousDisk = current
	}

	networkRaw, err := os.ReadFile(filepath.Join(c.root, "net/dev"))
	if err != nil {
		collectionErrors = append(collectionErrors, fmt.Errorf("read procfs network counters: %w", err))
	} else if current, parseErr := parseNetworkDev(string(networkRaw)); parseErr != nil {
		collectionErrors = append(collectionErrors, parseErr)
	} else {
		if !c.previousAt.IsZero() {
			elapsed := now.Sub(c.previousAt)
			for device, counters := range current {
				previous, exists := c.previousNetwork[device]
				if !exists {
					continue
				}
				labels := map[string]string{"interface": device}
				if rate, ok := CounterRate(previous.receiveBytes, counters.receiveBytes, elapsed); ok {
					add("host.network.receive.bytes_per_second", rate, "bytes/second", labels)
				}
				if rate, ok := CounterRate(previous.transmitBytes, counters.transmitBytes, elapsed); ok {
					add("host.network.transmit.bytes_per_second", rate, "bytes/second", labels)
				}
				if rate, ok := CounterRate(previous.receivePackets, counters.receivePackets, elapsed); ok {
					add("host.network.receive.packets_per_second", rate, "packets/second", labels)
				}
				if rate, ok := CounterRate(previous.transmitPackets, counters.transmitPackets, elapsed); ok {
					add("host.network.transmit.packets_per_second", rate, "packets/second", labels)
				}
				if rate, ok := CounterRate(previous.receiveErrors, counters.receiveErrors, elapsed); ok {
					add("host.network.receive.errors_per_second", rate, "errors/second", labels)
				}
				if rate, ok := CounterRate(previous.transmitErrors, counters.transmitErrors, elapsed); ok {
					add("host.network.transmit.errors_per_second", rate, "errors/second", labels)
				}
			}
		}
		c.previousNetwork = current
	}

	tcpStates := make(map[string]float64)
	tcpListeners := make(map[string]float64)
	for _, source := range []struct {
		file   string
		family string
	}{
		{file: "net/tcp", family: "ipv4"},
		{file: "net/tcp6", family: "ipv6"},
	} {
		raw, readErr := os.ReadFile(filepath.Join(c.root, source.file))
		if readErr != nil {
			if source.family == "ipv4" {
				collectionErrors = append(collectionErrors, fmt.Errorf("read procfs TCP inventory: %w", readErr))
			}
			continue
		}
		states, listeners, parseErr := parseTCPTable(string(raw), source.family)
		if parseErr != nil {
			collectionErrors = append(collectionErrors, parseErr)
			continue
		}
		for state, count := range states {
			key := source.family + ":" + state
			tcpStates[key] += float64(count)
		}
		for port, count := range listeners {
			key := source.family + ":" + strconv.Itoa(int(port))
			tcpListeners[key] += float64(count)
		}
	}
	for key, count := range tcpStates {
		parts := strings.SplitN(key, ":", 2)
		add("host.network.tcp.connections", count, "connections", map[string]string{
			"family": parts[0], "state": parts[1],
		})
	}
	for key, count := range tcpListeners {
		parts := strings.SplitN(key, ":", 2)
		add("host.network.tcp.listening_port", count, "sockets", map[string]string{
			"family": parts[0], "port": parts[1],
		})
	}

	uptimeRaw, err := os.ReadFile(filepath.Join(c.root, "uptime"))
	if err == nil {
		fields := strings.Fields(string(uptimeRaw))
		if len(fields) > 0 {
			if seconds, parseErr := strconv.ParseFloat(fields[0], 64); parseErr == nil {
				add("host.uptime.seconds", seconds, "seconds", nil)
			}
		}
	} else {
		collectionErrors = append(collectionErrors, fmt.Errorf("read procfs uptime: %w", err))
	}

	processes, processErr := readProcesses(c.root)
	if processErr != nil {
		collectionErrors = append(collectionErrors, processErr)
	} else {
		add("host.process.count", float64(processes.count), "processes", nil)
		for _, process := range processes.top {
			labels := map[string]string{
				"pid": strconv.Itoa(process.pid), "process": process.name,
				"attribution_method": process.attributionMethod,
			}
			if process.cgroupPath != "" {
				labels["cgroup_path"] = process.cgroupPath
			}
			if process.workloadType != "" {
				labels["workload_type"] = process.workloadType
			}
			if process.workloadSelector != "" {
				labels["workload_selector"] = process.workloadSelector
			}
			add("process.memory.rss", float64(process.rss), "bytes", labels)
			if process.pid == os.Getpid() {
				add("agent.memory.rss", float64(process.rss), "bytes", nil)
			}
		}
	}
	add("agent.uptime.seconds", now.Sub(c.startedAt).Seconds(), "seconds", nil)

	c.previousAt = now
	if len(collectionErrors) > 0 {
		return samples, errors.Join(collectionErrors...)
	}
	return samples, nil
}

type memoryCounters struct {
	total, available, cache, swapTotal, swapFree float64
}

func parseMemInfo(raw string) (memoryCounters, error) {
	values := make(map[string]float64)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		if len(fields) > 2 && fields[2] == "kB" {
			value *= 1024
		}
		values[key] = value
	}
	total, ok := values["MemTotal"]
	if !ok || total <= 0 {
		return memoryCounters{}, errors.New("procfs memory information has no valid MemTotal")
	}
	available, ok := values["MemAvailable"]
	if !ok {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	cache := values["Cached"] + values["SReclaimable"] + values["Buffers"] - values["Shmem"]
	return memoryCounters{
		total: total, available: math.Min(total, math.Max(0, available)),
		cache: math.Max(0, cache), swapTotal: values["SwapTotal"], swapFree: values["SwapFree"],
	}, nil
}

func parseCPUStat(raw string) (map[string]cpuCounters, error) {
	result := make(map[string]cpuCounters)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || (fields[0] != "cpu" && !strings.HasPrefix(fields[0], "cpu")) {
			continue
		}
		if len(fields) < 5 {
			continue
		}
		values := make([]float64, 8)
		for index := 1; index < len(fields) && index <= 8; index++ {
			value, err := strconv.ParseFloat(fields[index], 64)
			if err != nil || value < 0 {
				return nil, errors.New("invalid CPU counter in procfs stat")
			}
			values[index-1] = value
		}
		result[fields[0]] = cpuCounters{
			user: values[0], nice: values[1], system: values[2], idle: values[3],
			iowait: values[4], irq: values[5], softirq: values[6], steal: values[7],
		}
	}
	if _, ok := result["cpu"]; !ok {
		return nil, errors.New("procfs stat has no aggregate CPU counters")
	}
	return result, nil
}

func cpuPercent(previous, current cpuCounters) (busy, user, system, wait float64, ok bool) {
	currentValues := []float64{current.user, current.nice, current.system, current.idle, current.iowait, current.irq, current.softirq, current.steal}
	previousValues := []float64{previous.user, previous.nice, previous.system, previous.idle, previous.iowait, previous.irq, previous.softirq, previous.steal}
	deltas := make([]float64, len(currentValues))
	for index := range currentValues {
		if currentValues[index] < previousValues[index] {
			return 0, 0, 0, 0, false
		}
		deltas[index] = currentValues[index] - previousValues[index]
	}
	total := current.total() - previous.total()
	if total <= 0 {
		return 0, 0, 0, 0, false
	}
	busyTicks := total - deltas[3] - deltas[4]
	return 100 * busyTicks / total,
		100 * (deltas[0] + deltas[1]) / total,
		100 * deltas[2] / total,
		100 * deltas[4] / total, true
}

func parseDiskStats(raw string) (map[string]diskCounters, error) {
	result := make(map[string]diskCounters)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}
		var numbers [4]float64
		for index, fieldIndex := range []int{3, 5, 7, 9} {
			value, err := strconv.ParseFloat(fields[fieldIndex], 64)
			if err != nil || value < 0 {
				return nil, errors.New("invalid disk counter in procfs diskstats")
			}
			numbers[index] = value
		}
		result[fields[2]] = diskCounters{
			readOps: numbers[0], readBytes: numbers[1] * 512,
			writeOps: numbers[2], writeBytes: numbers[3] * 512,
		}
	}
	return result, nil
}

func parseNetworkDev(raw string) (map[string]networkCounters, error) {
	result := make(map[string]networkCounters)
	for lineNumber, line := range strings.Split(raw, "\n") {
		if lineNumber < 2 {
			continue
		}
		name, counters, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(counters)
		if len(fields) < 16 {
			continue
		}
		values := make([]float64, 6)
		for index, fieldIndex := range []int{0, 1, 2, 8, 9, 10} {
			value, err := strconv.ParseFloat(fields[fieldIndex], 64)
			if err != nil || value < 0 {
				return nil, errors.New("invalid network counter in procfs net/dev")
			}
			values[index] = value
		}
		result[strings.TrimSpace(name)] = networkCounters{
			receiveBytes: values[0], receivePackets: values[1], receiveErrors: values[2],
			transmitBytes: values[3], transmitPackets: values[4], transmitErrors: values[5],
		}
	}
	return result, nil
}

type processMemory struct {
	pid               int
	name              string
	rss               int64
	cgroupPath        string
	workloadType      string
	workloadSelector  string
	attributionMethod string
}

type processSnapshot struct {
	count int
	top   []processMemory
}

func readProcesses(procRoot string) (processSnapshot, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return processSnapshot{}, fmt.Errorf("read procfs process inventory: %w", err)
	}
	snapshot := processSnapshot{}
	for _, entry := range entries {
		pid, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || !entry.IsDir() {
			continue
		}
		snapshot.count++
		raw, readErr := os.ReadFile(filepath.Join(procRoot, entry.Name(), "statm"))
		if readErr != nil {
			continue
		}
		fields := strings.Fields(string(raw))
		if len(fields) < 2 {
			continue
		}
		pages, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr != nil || pages < 0 {
			continue
		}
		nameRaw, _ := os.ReadFile(filepath.Join(procRoot, entry.Name(), "comm"))
		name := strings.TrimSpace(string(nameRaw))
		if name == "" {
			name = "unknown"
		}
		snapshot.top = append(snapshot.top, processMemory{pid: pid, name: name, rss: pages * int64(os.Getpagesize())})
	}
	sort.Slice(snapshot.top, func(i, j int) bool { return snapshot.top[i].rss > snapshot.top[j].rss })
	if len(snapshot.top) > 128 {
		snapshot.top = snapshot.top[:128]
	}
	for index := range snapshot.top {
		process := &snapshot.top[index]
		raw, readErr := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(process.pid), "cgroup"))
		if readErr != nil {
			process.attributionMethod = "unclassified"
			continue
		}
		workload := parseWorkloadCgroup(string(raw))
		process.cgroupPath = workload.CgroupPath
		process.workloadType = workload.WorkloadType
		process.workloadSelector = workload.WorkloadSelector
		process.attributionMethod = workload.AttributionMethod
	}
	return snapshot, nil
}

// CounterRate converts a cumulative counter into a per-second rate. Counter
// resets and invalid elapsed intervals are missing data, never negative rates.
func CounterRate(previous, current float64, elapsed time.Duration) (float64, bool) {
	if elapsed <= 0 || current < previous || math.IsNaN(current) || math.IsInf(current, 0) {
		return 0, false
	}
	return (current - previous) / elapsed.Seconds(), true
}
