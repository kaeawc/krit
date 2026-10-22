//go:build darwin

package daemoncmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// processArgv uses `ps`, which joins argv with spaces; paths containing
// spaces therefore fail to match and the stop is refused (fail closed).
func processArgv(pid int) ([]string, error) {
	out, err := exec.CommandContext(context.Background(), "ps", "-o", "command=", "-p", fmt.Sprint(pid)).Output()
	if err != nil {
		return nil, err
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return nil, fmt.Errorf("no command for pid %d", pid)
	}
	return f, nil
}
