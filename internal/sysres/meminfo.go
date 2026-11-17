package sysres

import (
	"os"
	"strconv"
	"strings"
)

// memTotalBytes reads MemTotal from a /proc/meminfo file. Inside a container
// this is the host's memory, which is why MemoryLimitBytes also reads the
// cgroup limit.
func memTotalBytes(file string) uint64 {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		rest, ok := strings.CutPrefix(line, "MemTotal:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return 0
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0
		}
		return kb << 10
	}
	return 0
}
