package testutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// IsolateDaemons gives a test package a private daemon registry and records
// every JVM it starts. Call cleanup after m.Run, including on test failure.
// Existing daemons in the developer's registry are never connected to or
// stopped. The PID snapshot also guards a caller-supplied registry override.
func IsolateDaemons() (func() error, error) {
	root, err := os.MkdirTemp("", "krit-test-daemons-")
	if err != nil {
		return nil, err
	}
	registry := filepath.Join(root, "registry")
	if err := os.Mkdir(registry, 0700); err != nil {
		_ = os.RemoveAll(root)
		return nil, err
	}
	pidFile := filepath.Join(root, "started-pids")
	previous := map[string]string{}
	for _, key := range []string{"KRIT_DAEMON_REGISTRY_DIR", "KRIT_DAEMON_TEST_PID_FILE", "KRIT_EPHEMERAL_DAEMONS"} {
		previous[key] = os.Getenv(key)
	}
	_ = os.Setenv("KRIT_DAEMON_REGISTRY_DIR", registry)
	_ = os.Setenv("KRIT_DAEMON_TEST_PID_FILE", pidFile)
	_ = os.Setenv("KRIT_EPHEMERAL_DAEMONS", "1")
	before := registryPIDs(registry)
	return func() error {
		pids := registryPIDs(registry)
		data, _ := os.ReadFile(pidFile)
		for _, line := range strings.Fields(string(data)) {
			if pid, err := strconv.Atoi(line); err == nil {
				pids[pid] = true
			}
		}
		var failures []string
		for pid := range pids {
			if before[pid] || !kritJVM(pid) {
				continue
			}
			proc, err := os.FindProcess(pid)
			if err != nil {
				failures = append(failures, fmt.Sprintf("PID %d: %v", pid, err))
				continue
			}
			_ = proc.Kill()
			for i := 0; i < 20 && kritJVM(pid); i++ {
				time.Sleep(100 * time.Millisecond)
			}
			if kritJVM(pid) {
				failures = append(failures, fmt.Sprintf("PID %d still alive", pid))
			}
		}
		for key, value := range previous {
			if value == "" {
				_ = os.Unsetenv(key)
			} else {
				_ = os.Setenv(key, value)
			}
		}
		_ = os.RemoveAll(root)
		if len(failures) > 0 {
			return fmt.Errorf("test daemon cleanup: %s", strings.Join(failures, "; "))
		}
		return nil
	}, nil
}

func registryPIDs(dir string) map[int]bool {
	pids := map[int]bool{}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".pid") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				pids[pid] = true
			}
		}
	}
	return pids
}

func kritJVM(pid int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return false
	}
	cmd := string(output)
	return strings.Contains(cmd, "krit-fir.jar") || strings.Contains(cmd, "krit-types.jar")
}
