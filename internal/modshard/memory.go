package modshard

import "github.com/kaeawc/krit/internal/sysres"

// totalMemoryMB is the memory this process may use, or 0 when unknown. On Linux
// it is the cgroup limit when one is set, so a sandboxed run sizes its workers
// to the sandbox rather than the host.
var totalMemoryMB = sysres.MemoryLimitMB

// workerHeapMB is each worker JVM's heap cap. A few GB covers the largest modules; the cap shrinks
// when many workers would together take more than 40% of the available memory, so the workers do not
// push the machine into swap or the sandbox past its limit.
func workerHeapMB(workers int) int {
	const most, least = 5120, 1536
	total := totalMemoryMB()
	if total == 0 || workers < 1 {
		return most
	}
	return max(least, min(most, total*40/100/workers))
}
