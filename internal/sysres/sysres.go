// Package sysres reports the CPU and memory a krit process may use. On Linux
// it reads the cgroup limits a container or build-system sandbox sets, so
// defaults sized from it fit the sandbox rather than the host.
package sysres

import (
	"runtime"
)

// CPUs returns the default parallelism: GOMAXPROCS, never less than 1. The Go
// runtime derives GOMAXPROCS from the CPU affinity mask and, on Linux, the
// cgroup CPU quota, and the GOMAXPROCS environment variable overrides both.
// runtime.NumCPU ignores the quota, so a container with a 2-CPU quota on a
// 64-core host would otherwise start 64 workers.
func CPUs() int {
	return max(1, runtime.GOMAXPROCS(0))
}

// MemoryLimitBytes returns the memory this process may use: the smaller of
// the machine's physical memory and, on Linux, the cgroup memory limit. It
// returns 0 when neither is known.
func MemoryLimitBytes() uint64 {
	total := physicalMemoryBytes()
	if limit, ok := CgroupMemoryLimitBytes(); ok && (total == 0 || limit < total) {
		return limit
	}
	return total
}

// MemoryLimitMB is MemoryLimitBytes in mebibytes.
func MemoryLimitMB() int {
	return int(MemoryLimitBytes() >> 20)
}

// CgroupMemoryLimitBytes returns the memory limit of this process's cgroup
// (v2 memory.max, or v1 memory.limit_in_bytes), the smallest one set on the
// cgroup or any of its ancestors. ok is false when no limit is set, or off
// Linux.
func CgroupMemoryLimitBytes() (limit uint64, ok bool) {
	return cgroupMemoryLimit(procSelfCgroup, cgroupRoot)
}
