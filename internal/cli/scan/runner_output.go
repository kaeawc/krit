package scan

import (
	"fmt"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

func (r *runner) printVerboseBanner() {
	r.tracker.TrackVoid("verboseBanner", func() {
		dispatchCount, lineCount, crossFileCount, moduleAwareCount :=
			rules.NewDispatcher(r.activeRules, r.resolver).Stats()

		if !*r.f.Verbose {
			return
		}
		if len(r.javaPathsForDispatch) > 0 {
			fmt.Fprintf(os.Stderr, "verbose: Found %d Kotlin files and %d Java files\n", len(r.files), len(r.javaPathsForDispatch))
		} else {
			fmt.Fprintf(os.Stderr, "verbose: Found %d Kotlin files\n", len(r.files))
		}
		if r.resolver != nil {
			fmt.Fprintf(os.Stderr, "verbose: Type resolver active\n")
		} else {
			fmt.Fprintf(os.Stderr, "verbose: Type resolver disabled\n")
		}
		fmt.Fprintf(os.Stderr, "verbose: Running %d rules with %d workers (%d dispatch, %d line, %d cross-file, %d module-aware)\n",
			len(r.activeRules), *r.f.Jobs, dispatchCount, lineCount, crossFileCount, moduleAwareCount)
	})
}

func (r *runner) printFixupResult(fixRes pipeline.FixupResult) {
	printFixupResult(r.f, fixRes, r.start)
}

func (r *runner) printBinaryFixResult(fixRes pipeline.FixupResult) {
	printBinaryFixResult(r.f, fixRes)
}

// printFixupResult is the runner-independent fixup reporter. The
// daemon-routed --fix path uses it too so the user-visible info /
// dry-run / applied lines stay byte-identical regardless of which
// path executed the scan.
func printFixupResult(f *scanFlags, fixRes pipeline.FixupResult, start time.Time) {
	fixableCount := fixRes.FixableCount
	strippedByLevel := fixRes.StrippedByLevel
	if fixableCount == 0 {
		if !*f.Quiet {
			if strippedByLevel > 0 {
				fmt.Fprintf(os.Stderr, "info: No auto-fixable issues at level %s. %d fix(es) available at higher levels (use --fix-level=semantic).\n",
					*f.FixLevel, strippedByLevel)
			} else {
				fmt.Fprintln(os.Stderr, "info: No auto-fixable issues found.")
			}
		}
		return
	}
	if *f.DryRun {
		printDryRunFixResult(f, &fixRes.Findings, fixableCount)
	} else {
		printAppliedFixResult(f, fixRes, start)
	}
}

// printDryRunFixResult prints one file per line (the files with at
// least one available text fix) plus the summary line. Iterates the
// post-strip columns so the file list reflects the MaxFixLevel cap.
func printDryRunFixResult(f *scanFlags, cols *scanner.FindingColumns, fixableCount int) {
	seen := make(map[string]bool)
	for row := 0; row < cols.Len(); row++ {
		if !cols.HasFix(row) {
			continue
		}
		file := cols.FileAt(row)
		if !seen[file] {
			seen[file] = true
			fmt.Println(file)
		}
	}
	if !*f.Quiet {
		fmt.Fprintf(os.Stderr, "info: %d fix(es) available across %d file(s).\n", fixableCount, len(seen))
	}
}

func printAppliedFixResult(f *scanFlags, fixRes pipeline.FixupResult, start time.Time) {
	binarySet := make(map[error]bool, len(fixRes.BinaryErrors))
	for _, e := range fixRes.BinaryErrors {
		binarySet[e] = true
	}
	for _, e := range fixRes.FixErrors {
		if binarySet[e] {
			continue
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", e)
	}
	if !*f.Quiet {
		suffix := "in place"
		if *f.FixSuffix != "" {
			suffix = "with suffix '" + *f.FixSuffix + "'"
		}
		fmt.Fprintf(os.Stderr, "info: Applied %d fix(es) across %d file(s) %s in %v.\n",
			fixRes.TextApplied, len(fixRes.ModifiedFiles), suffix, time.Since(start).Round(time.Millisecond))
	}
}

func printBinaryFixResult(f *scanFlags, fixRes pipeline.FixupResult) {
	if !*f.FixBinary {
		return
	}
	for _, e := range fixRes.BinaryErrors {
		fmt.Fprintf(os.Stderr, "error: binary fix: %v\n", e)
	}
	if fixRes.BinaryApplied > 0 && !*f.Quiet {
		mode := "applied"
		if *f.DryRun {
			mode = "available"
		}
		fmt.Fprintf(os.Stderr, "info: %d binary fix(es) %s.\n", fixRes.BinaryApplied, mode)
	}
}
