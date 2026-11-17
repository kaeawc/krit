package sysres

func physicalMemoryBytes() uint64 { return memTotalBytes("/proc/meminfo") }
