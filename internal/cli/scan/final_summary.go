package scan

import (
	"fmt"
	"io"
	"time"
)

// finalScanExit prints the end-of-run summary line ("info: Found N issue(s)
// in T") and returns the process exit code: 1 when any findings remain,
// 0 otherwise. The summary is suppressed when quiet is true. Elapsed is
// rounded to milliseconds in the printed line.
//
// This is the very tail of scan.Run — pulling it out keeps the exit-code
// rule (any-finding => 1) in one named place that's straightforward to
// unit test.
func finalScanExit(w io.Writer, findingCount int, elapsed time.Duration, quiet bool) int {
	if !quiet {
		fmt.Fprintf(w, "info: Found %d issue(s) in %v.\n",
			findingCount, elapsed.Round(time.Millisecond))
	}
	if findingCount > 0 {
		return 1
	}
	return 0
}
