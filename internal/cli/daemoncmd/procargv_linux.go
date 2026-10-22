//go:build linux

package daemoncmd

import (
	"fmt"
	"os"
	"strings"
)

func processArgv(pid int) ([]string, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil, err
	}
	s := strings.TrimRight(string(b), "\x00")
	if s == "" {
		return nil, fmt.Errorf("empty cmdline for pid %d", pid)
	}
	return strings.Split(s, "\x00"), nil
}
