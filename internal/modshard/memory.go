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

// workerHeapMB is the heap cap of a worker JVM that compiles threads of the parallel concurrent
// module compiles. Each compile gets what a JVM per module got before #1656: a few GB covers the
// largest module, shrinking when the compiles would together take more than 40% of the available
// memory, so they do not push the machine into swap or the sandbox past its limit, but never below
// the floor a large module needs. With the memory unknown, each compile gets the full cap.
func workerHeapMB(parallel, threads int) int {
	const most, least = 5120, 1536
	threads = max(1, threads)
	total := totalMemoryMB()
	if total == 0 || parallel < 1 {
		return most * threads
	}
	return max(least, min(most, total*40/100/parallel)) * threads
}

// maxThreadsPerJVM is how many modules one worker JVM compiles at a time unless KRIT_WORKER_THREADS
// says otherwise (#1656). Threads of one JVM share its JIT-compiled compiler and its heap, which a
// JVM per module would each build up again: on a 4-core machine two modules on two threads of one
// JVM recompiled 24 source sets about 20% faster than two JVMs did, with the same results.
const maxThreadsPerJVM = 4

// workerLayout splits parallel concurrent module compiles over as few worker JVMs as a cap of
// maxThreadsPerJVM (or KRIT_WORKER_THREADS) threads each allows. It returns each JVM's thread count;
// the counts differ by at most one and add up to parallel.
func workerLayout(parallel int) []int {
	per := maxThreadsPerJVM
	if v, err := strconv.Atoi(os.Getenv("KRIT_WORKER_THREADS")); err == nil && v >= 1 {
		per = v
	}
	parallel = max(1, parallel)
	jvms := (parallel + per - 1) / per
	threads := make([]int, jvms)
	for i := range threads {
		threads[i] = parallel / jvms
		if i < parallel%jvms {
			threads[i]++
		}
	}
	return threads
}
