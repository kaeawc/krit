package modshard

import (
	"os"
	"strconv"

	"github.com/kaeawc/krit/internal/sysres"
)

// totalMemoryMB is the memory this process may use, or 0 when unknown. On Linux
// it is the cgroup limit when one is set, so a sandboxed run sizes its workers
// to the sandbox rather than the host.
var totalMemoryMB = sysres.MemoryLimitMB

// workerHeapMB is the heap cap of each of jvms worker JVMs that compile threads modules at a time. A
// few GB covers the largest module, so a JVM gets that per thread; the cap shrinks when the workers
// would together take more than 40% of the available memory, so they do not push the machine into
// swap or the sandbox past its limit. With the memory unknown, a JVM gets the one-module cap.
func workerHeapMB(jvms, threads int) int {
	const most, least = 5120, 1536
	total := totalMemoryMB()
	if total == 0 || jvms < 1 {
		return most
	}
	return max(least, min(most*max(1, threads), total*40/100/jvms))
}

// maxThreadsPerJVM is how many modules one worker JVM compiles at a time unless KRIT_WORKER_THREADS
// says otherwise (#1656). Threads of one JVM share its JIT-compiled compiler and its heap, which a
// JVM per module would each build up again: on a 4-core machine two modules on two threads of one
// JVM recompiled 24 source sets about 20% faster than two JVMs did, with the same results.
const maxThreadsPerJVM = 4

// workerLayout splits parallel concurrent module compiles over worker JVMs of at most
// maxThreadsPerJVM (or KRIT_WORKER_THREADS) threads each, as evenly as it can.
func workerLayout(parallel int) (jvms, threads int) {
	per := maxThreadsPerJVM
	if v, err := strconv.Atoi(os.Getenv("KRIT_WORKER_THREADS")); err == nil && v >= 1 {
		per = v
	}
	parallel = max(1, parallel)
	jvms = (parallel + per - 1) / per
	threads = (parallel + jvms - 1) / jvms
	return jvms, threads
}
