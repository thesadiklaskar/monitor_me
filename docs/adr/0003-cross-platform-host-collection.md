# ADR 0003: Cross-platform host collection with gopsutil

- **Status:** Accepted for the initial Windows collector
- **Date:** 2026-11-23

## Context

Monitor Me's first native collector reads Linux procfs and reports only agent health when built for other operating systems. The owner needs to verify the local Windows PC without buying a VPS or running a separate Linux VM. The Windows collector must preserve the existing agent transport, durable spool, metric names and per-OS capability reporting.

## Decision

Use `github.com/shirou/gopsutil/v4` version `v4.26.9` in the agent's Windows-only collector implementation. The existing Linux procfs collector remains unchanged. The Windows collector reads CPU times, memory/swap, mounted-volume usage, disk/network counters, host uptime/identity and process RSS. Unsupported Linux-specific features are not advertised on Windows.

## Purpose

- Provide native Windows host metrics without requiring WSL, a separate Linux VM, or a VPS.
- Reuse the existing TLS gRPC enrollment and telemetry pipeline.
- Keep OS-specific collection code behind Go build constraints.

## License

gopsutil is distributed under the BSD 3-Clause license. The project must retain the dependency's license notices in any redistributions as required by that license.

## Operational cost

- Adds a small Go module dependency graph and extra dependency download time during agent builds.
- Windows process and volume APIs have permission-dependent visibility; inaccessible processes/volumes are omitted rather than reported as zero.
- The collector is configured for a five-second interval in the local PowerShell helper to avoid unnecessary high-frequency host inspection.
- This decision does not add Windows service installation, code signing, TCP/listening-port inventory, per-process CPU, or workload attribution.

## Alternatives considered

1. **Keep Linux-only and require WSL/Linux VM.** Rejected for the owner's requested local Windows host view; it reports the Linux guest rather than the full Windows host.
2. **Implement Windows API bindings manually.** Rejected for the first slice because it increases low-level API layout and maintenance risk for CPU, memory, volume, network and process counters.
3. **Shell out to PowerShell/WMI on every collection cycle.** Rejected because process startup and text parsing add overhead and produce a less stable metric interface.

## Consequences

- Linux remains the verified full-capability collector.
- Windows has a practical initial collector with explicit capability differences.
- Windows runtime tests, service installation, signed installers and broader platform acceptance remain required before claiming complete Windows support.
