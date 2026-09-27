package oracle

import (
	"fmt"
	"os"
	"strings"
)

const javaArgfileThreshold = 8000

// prepareJavaArgs moves a long complete Java argument vector into a JDK
// argument file. The sum of token lengths plus separators is a conservative
// command-line length proxy; short calls retain their original argv shape.
// The caller must keep the file until Java has started reading its arguments.
func prepareJavaArgs(args []string) ([]string, func(), error) {
	length := 0
	for _, arg := range args {
		length += len(arg) + 1
	}
	if length <= javaArgfileThreshold {
		return args, func() {}, nil
	}
	file, err := os.CreateTemp("", "krit-java-*.args")
	if err != nil {
		return nil, nil, fmt.Errorf("create Java argfile: %w", err)
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\r\n\f\"'#\\") {
			arg = strings.ReplaceAll(arg, "\\", "\\\\")
			arg = "\"" + strings.ReplaceAll(arg, "\"", "\\\"") + "\""
		}
		if _, err := fmt.Fprintln(file, arg); err != nil {
			_ = file.Close()
			cleanup()
			return nil, nil, fmt.Errorf("write Java argfile: %w", err)
		}
	}
	if err := file.Close(); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("close Java argfile: %w", err)
	}
	return []string{"@" + file.Name()}, cleanup, nil
}
