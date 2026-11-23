//go:build windows

package collector

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	gopsutilnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// collectWindows reads Windows host counters through native operating-system
// APIs exposed by gopsutil. Unsupported optional counters are omitted.
func (c *ProcCollector) collectWindows(ctx context.Context) ([]Sample, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	now := time.Now()
	observedAt := now.UTC().UnixNano()
	samples := make([]Sample, 0, 128)
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

	// Process CPU time and OS uptime are measured independently of wall-clock
	// time, so changes to the Windows clock do not create invalid rates.
	if cpuSeconds, ok := readAgentCPUSeconds(); ok {
		if c.hasPreviousAgentCPU {
			if utilization, valid := AgentCPUUtilization(
				c.previousAgentCPUSeconds, cpuSeconds, now.Sub(c.previousAt),
			); valid {
				add("agent.cpu.utilization", utilization, "percent", nil)
			}
		}
		c.previousAgentCPUSeconds = cpuSeconds
		c.hasPreviousAgentCPU = true
	}

	aggregateTimes, aggregateErr := cpu.TimesWithContext(ctx, false)
	perCPUTimes, perCPUErr := cpu.TimesWithContext(ctx, true)
	if aggregateErr != nil {
		collectionErrors = append(collectionErrors, fmt.Errorf("read Windows aggregate CPU counters: %w", aggregateErr))
	}
	// Per-core counters are an optional capability; keep valid aggregate
	// telemetry flowing when the OS cannot enumerate individual cores.
	current := make(map[string]cpuCounters, len(aggregateTimes)+len(perCPUTimes))
	for _, item := range aggregateTimes {
		current["cpu-total"] = cpuCounters{
			user: item.User, nice: item.Nice, system: item.System,
			idle: item.Idle, iowait: item.Iowait, irq: item.Irq,
			softirq: item.Softirq, steal: item.Steal,
		}
	}
	for _, item := range perCPUTimes {
		current[item.CPU] = cpuCounters{
			user: item.User, nice: item.Nice, system: item.System,
			idle: item.Idle, iowait: item.Iowait, irq: item.Irq,
			softirq: item.Softirq, steal: item.Steal,
		}
	}
	if !c.previousAt.IsZero() {
		for name, counters := range current {
			previous, exists := c.previousCPU[name]
			if !exists {
				continue
			}
			busy, user, system, wait, valid := cpuPercent(previous, counters)
			if !valid {
				continue
			}
			metricName := "host.cpu.core.utilization"
			if name == "cpu-total" || name == "cpu" {
				metricName = "host.cpu.utilization"
				add("host.cpu.user", user, "percent", nil)
				add("host.cpu.system", system, "percent", nil)
			}
			add(metricName, busy, "percent", map[string]string{"cpu": name})
		}
	}
	c.previousCPU = current

	virtualMemory, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		collectionErrors = append(collectionErrors, fmt.Errorf("read Windows memory counters: %w", err))
	} else {
		add("host.memory.total", float64(virtualMemory.Total), "bytes", nil)
		add("host.memory.available", float64(virtualMemory.Available), "bytes", nil)
		add("host.memory.used", float64(virtualMemory.Used), "bytes", nil)
		if virtualMemory.Cached > 0 {
			add("host.memory.cache", float64(virtualMemory.Cached), "bytes", nil)
		}
	}
	if swap, swapErr := mem.SwapMemoryWithContext(ctx); swapErr == nil && swap != nil {
		add("host.memory.swap.total", float64(swap.Total), "bytes", nil)
		add("host.memory.swap.used", float64(swap.Used), "bytes", nil)
	}

	diskIO, diskErr := disk.IOCountersWithContext(ctx)
	if diskErr == nil {
		current := make(map[string]diskCounters, len(diskIO))
		for name, counters := range diskIO {
			current[name] = diskCounters{
				readBytes: float64(counters.ReadBytes), writeBytes: float64(counters.WriteBytes),
				readOps: float64(counters.ReadCount), writeOps: float64(counters.WriteCount),
			}
		}
		if !c.previousAt.IsZero() {
			elapsed := now.Sub(c.previousAt)
			for device, counters := range current {
				previous, exists := c.previousDisk[device]
				if !exists {
					continue
				}
				labels := map[string]string{"device": device}
				if rate, valid := CounterRate(previous.readBytes, counters.readBytes, elapsed); valid {
					add("host.disk.read.bytes_per_second", rate, "bytes/second", labels)
				}
				if rate, valid := CounterRate(previous.writeBytes, counters.writeBytes, elapsed); valid {
					add("host.disk.write.bytes_per_second", rate, "bytes/second", labels)
				}
				if rate, valid := CounterRate(previous.readOps, counters.readOps, elapsed); valid {
					add("host.disk.read.operations_per_second", rate, "operations/second", labels)
				}
				if rate, valid := CounterRate(previous.writeOps, counters.writeOps, elapsed); valid {
					add("host.disk.write.operations_per_second", rate, "operations/second", labels)
				}
			}
		}
		c.previousDisk = current
	}

	// Capacity is reported per mounted volume. Duplicate mount points are
	// collapsed because Windows can return aliases for the same volume.
	partitions, partitionErr := disk.PartitionsWithContext(ctx, true)
	if partitionErr == nil {
		seenMounts := make(map[string]struct{}, len(partitions))
		for _, partition := range partitions {
			if partition.Mountpoint == "" {
				continue
			}
			if _, exists := seenMounts[partition.Mountpoint]; exists {
				continue
			}
			seenMounts[partition.Mountpoint] = struct{}{}
			usage, usageErr := disk.UsageWithContext(ctx, partition.Mountpoint)
			if usageErr != nil || usage == nil || usage.Total == 0 {
				continue
			}
			labels := map[string]string{
				"mountpoint": partition.Mountpoint,
				"device": partition.Device,
				"fstype": partition.Fstype,
			}
			add("host.disk.total.bytes", float64(usage.Total), "bytes", labels)
			add("host.disk.used.bytes", float64(usage.Used), "bytes", labels)
			add("host.disk.available.bytes", float64(usage.Free), "bytes", labels)
			add("host.disk.used.percent", usage.UsedPercent, "percent", labels)
		}
	}

	networkIO, networkErr := gopsutilnet.IOCountersWithContext(ctx, true)
	if networkErr == nil {
		current := make(map[string]networkCounters, len(networkIO))
		for _, counters := range networkIO {
			current[counters.Name] = networkCounters{
				receiveBytes: float64(counters.BytesRecv),
				transmitBytes: float64(counters.BytesSent),
				receivePackets: float64(counters.PacketsRecv),
				transmitPackets: float64(counters.PacketsSent),
				receiveErrors: float64(counters.Errin),
				transmitErrors: float64(counters.Errout),
			}
		}
		if !c.previousAt.IsZero() {
			elapsed := now.Sub(c.previousAt)
			for iface, counters := range current {
				previous, exists := c.previousNetwork[iface]
				if !exists {
					continue
				}
				labels := map[string]string{"interface": iface}
				if rate, valid := CounterRate(previous.receiveBytes, counters.receiveBytes, elapsed); valid {
					add("host.network.receive.bytes_per_second", rate, "bytes/second", labels)
				}
				if rate, valid := CounterRate(previous.transmitBytes, counters.transmitBytes, elapsed); valid {
					add("host.network.transmit.bytes_per_second", rate, "bytes/second", labels)
				}
				if rate, valid := CounterRate(previous.receivePackets, counters.receivePackets, elapsed); valid {
					add("host.network.receive.packets_per_second", rate, "packets/second", labels)
				}
				if rate, valid := CounterRate(previous.transmitPackets, counters.transmitPackets, elapsed); valid {
					add("host.network.transmit.packets_per_second", rate, "packets/second", labels)
				}
				if rate, valid := CounterRate(previous.receiveErrors, counters.receiveErrors, elapsed); valid {
					add("host.network.receive.errors_per_second", rate, "errors/second", labels)
				}
				if rate, valid := CounterRate(previous.transmitErrors, counters.transmitErrors, elapsed); valid {
					add("host.network.transmit.errors_per_second", rate, "errors/second", labels)
				}
			}
		}
		c.previousNetwork = current
	}

	if uptime, uptimeErr := host.UptimeWithContext(ctx); uptimeErr == nil {
		add("host.uptime.seconds", float64(uptime), "seconds", nil)
	}
	if info, infoErr := host.InfoWithContext(ctx); infoErr == nil && info != nil {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = info.Hostname
		}
		add("host.info", 1, "info", map[string]string{
			"hostname": hostname,
			"os": runtime.GOOS,
			"platform": info.Platform,
			"platform_version": info.PlatformVersion,
			"kernel_version": info.KernelVersion,
			"architecture": runtime.GOARCH,
		})
	}

	pids, processErr := process.PidsWithContext(ctx)
	if processErr != nil {
		collectionErrors = append(collectionErrors, fmt.Errorf("read Windows process inventory: %w", processErr))
	} else {
		top := make([]processMemory, 0, len(pids))
		for _, pid := range pids {
			if err := ctx.Err(); err != nil {
				return samples, err
			}
			proc, procErr := process.NewProcess(pid)
			if procErr != nil {
				continue
			}
			memory, memoryErr := proc.MemoryInfoWithContext(ctx)
			if memoryErr != nil || memory == nil {
				continue
			}
			name, nameErr := proc.NameWithContext(ctx)
			if nameErr != nil || name == "" {
				name = "unknown"
			}
			top = append(top, processMemory{
				pid: int(pid), name: name, rss: int64(memory.RSS),
				attributionMethod: "host",
			})
		}
		sort.Slice(top, func(i, j int) bool { return top[i].rss > top[j].rss })
		add("host.process.count", float64(len(pids)), "processes", nil)
		if len(top) > 128 {
			top = top[:128]
		}
		for _, item := range top {
			add("process.memory.rss", float64(item.rss), "bytes", map[string]string{
				"pid": strconv.Itoa(item.pid),
				"process": item.name,
				"attribution_method": item.attributionMethod,
			})
		}
	}

	add("agent.uptime.seconds", now.Sub(c.startedAt).Seconds(), "seconds", nil)
	c.previousAt = now
	if len(collectionErrors) > 0 {
		return samples, errors.Join(collectionErrors...)
	}
	return samples, nil
}
