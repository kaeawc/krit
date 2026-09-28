package oracle

import (
	"os"
	"strconv"
	"sync"
)

var daemonTestPIDMu sync.Mutex

// RecordTestDaemonPID records processes created under the test registry so
// package TestMain cleanup can find them even if a registry entry was removed.
func RecordTestDaemonPID(pid int) {
	path := os.Getenv("KRIT_DAEMON_TEST_PID_FILE")
	if path == "" || pid <= 0 {
		return
	}
	daemonTestPIDMu.Lock()
	defer daemonTestPIDMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(strconv.Itoa(pid) + "\n")
	_ = f.Close()
}
