package sysres

import (
	"bufio"
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// The Linux locations of the process cgroup list and the cgroup mount. Tests
// point cgroupMemoryLimit at a fake tree instead.
const (
	procSelfCgroup = "/proc/self/cgroup"
	cgroupRoot     = "/sys/fs/cgroup"
)

// unlimitedV1 is the smallest value treated as "no limit" in cgroup v1, which
// reports an unset limit as a page-rounded 2^63-1.
const unlimitedV1 = uint64(1) << 62

// cgroupMemoryLimit reads the memory limit for the cgroups listed in
// cgroupFile, under the cgroup mount root. It tries cgroup v2 first, then
// the v1 memory controller.
func cgroupMemoryLimit(cgroupFile, root string) (uint64, bool) {
	data, err := os.ReadFile(cgroupFile)
	if err != nil {
		return 0, false
	}
	v2Path, v1Path, hasV1 := parseProcCgroup(data)
	if v2Path != "" {
		if limit, ok := minLimitUpTree(root, v2Path, "memory.max"); ok {
			return limit, true
		}
	}
	if hasV1 {
		return minLimitUpTree(filepath.Join(root, "memory"), v1Path, "memory.limit_in_bytes")
	}
	return 0, false
}

// parseProcCgroup returns the cgroup v2 path (the "0::" entry) and the path of
// the v1 memory controller from /proc/self/cgroup content.
func parseProcCgroup(data []byte) (v2Path, v1Path string, hasV1 bool) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), ":", 3)
		if len(parts) != 3 {
			continue
		}
		if parts[0] == "0" && parts[1] == "" {
			v2Path = parts[2]
			continue
		}
		for _, controller := range strings.Split(parts[1], ",") {
			if controller == "memory" {
				v1Path, hasV1 = parts[2], true
			}
		}
	}
	return v2Path, v1Path, hasV1
}

// minLimitUpTree returns the smallest limit in file across cgroupPath and its
// ancestors under root. Without a cgroup namespace the listed path may name a
// host cgroup that is not visible in this mount, so the mount root itself is
// always checked as well.
func minLimitUpTree(root, cgroupPath, file string) (uint64, bool) {
	var best uint64
	found := false
	seen := map[string]bool{}
	for dir := path.Clean("/" + cgroupPath); ; dir = path.Dir(dir) {
		full := filepath.Join(root, filepath.FromSlash(dir), file)
		if !seen[full] {
			seen[full] = true
			if limit, ok := readLimit(full); ok && (!found || limit < best) {
				best, found = limit, true
			}
		}
		if dir == "/" {
			break
		}
	}
	return best, found
}

// readLimit parses a memory.max or memory.limit_in_bytes file. "max", values
// at or above the v1 unlimited sentinel, zero and unreadable files mean no
// limit.
func readLimit(file string) (uint64, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, false
	}
	value := strings.TrimSpace(string(data))
	if value == "" || value == "max" {
		return 0, false
	}
	limit, err := strconv.ParseUint(value, 10, 64)
	if err != nil || limit == 0 || limit >= unlimitedV1 {
		return 0, false
	}
	return limit, true
}
