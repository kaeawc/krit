//go:build !darwin && !linux

package sysres

func physicalMemoryBytes() uint64 { return 0 }
