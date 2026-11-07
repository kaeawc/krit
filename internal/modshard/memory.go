package modshard

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// totalMemoryMB is the machine's physical memory, or 0 when unknown.
func totalMemoryMB() int {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.CommandContext(context.Background(), "sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		bytes, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		return int(bytes >> 20)
	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		for _, line := range strings.Split(string(data), "\n") {
			if rest, ok := strings.CutPrefix(line, "MemTotal:"); ok {
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					kb, _ := strconv.Atoi(fields[0])
					return kb >> 10
				}
			}
		}
	}
	return 0
}

// workerHeapMB is each worker JVM's heap cap. A few GB covers the largest modules; the cap shrinks
// when many workers would together take more than 40% of the machine's memory, so the workers do not
// push it into swap.
func workerHeapMB(workers int) int {
	const most, least = 5120, 1536
	total := totalMemoryMB()
	if total == 0 || workers < 1 {
		return most
	}
	return max(least, min(most, total*40/100/workers))
}
