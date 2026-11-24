// Package jvmargs keeps JVM launches inside the operating system's
// command-line limits. Linux caps one argument at MAX_ARG_STRLEN (128 KiB)
// and the whole argv at ARG_MAX; a large project's file list, source-dir
// list or classpath goes past either. The `java` launcher (JDK 9+) reads its
// arguments from an `@argfile` instead, on every platform with the same
// format, so long argument vectors are moved into one.
package jvmargs

import (
	"fmt"
	"os"
	"strings"
)

// Threshold is the command-line length, in bytes, above which Prepare moves
// the arguments into an argfile.
const Threshold = 8000

// Prepare moves a long complete Java argument vector into a JDK argument
// file. The sum of token lengths plus separators is a conservative
// command-line length proxy; short calls retain their original argv shape.
// The caller must keep the file until Java has started reading its
// arguments: a one-shot launch until it exits, a daemon until it reports
// ready. The returned cleanup removes the file and is never nil on success.
//
// Only the `java` launcher understands argfiles; never pass the result to a
// native launcher.
func Prepare(args []string) ([]string, func(), error) {
	length := 0
	for _, arg := range args {
		length += len(arg) + 1
	}
	if length <= Threshold {
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
			arg = strings.ReplaceAll(arg, "\n", `\n`)
			arg = strings.ReplaceAll(arg, "\r", `\r`)
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
