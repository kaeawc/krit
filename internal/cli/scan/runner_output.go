package scan

import (
	"fmt"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
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
	fixableCount := fixRes.FixableCount
	strippedByLevel := fixRes.StrippedByLevel
	if fixableCount == 0 {
		if !*r.f.Quiet {
			if strippedByLevel > 0 {
				fmt.Fprintf(os.Stderr, "info: No auto-fixable issues at level %s. %d fix(es) available at higher levels (use --fix-level=semantic).\n",
					*r.f.FixLevel, strippedByLevel)
			} else {
				fmt.Fprintln(os.Stderr, "info: No auto-fixable issues found.")
			}
		}
		return
	}
	if *r.f.DryRun {
		r.printDryRunFixResult(fixableCount)
	} else {
		r.printAppliedFixResult(fixRes)
	}
}

func (r *runner) printDryRunFixResult(fixableCount int) {
	seen := make(map[string]bool)
	for row := 0; row < r.allColumns.Len(); row++ {
		if !r.allColumns.HasFix(row) {
			continue
		}
		file := r.allColumns.FileAt(row)
		if !seen[file] {
			seen[file] = true
			fmt.Println(file)
		}
	}
	if !*r.f.Quiet {
		fmt.Fprintf(os.Stderr, "info: %d fix(es) available across %d file(s).\n", fixableCount, len(seen))
	}
}

func (r *runner) printAppliedFixResult(fixRes pipeline.FixupResult) {
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
	if !*r.f.Quiet {
		suffix := "in place"
		if *r.f.FixSuffix != "" {
			suffix = "with suffix '" + *r.f.FixSuffix + "'"
		}
		fmt.Fprintf(os.Stderr, "info: Applied %d fix(es) across %d file(s) %s in %v.\n",
			fixRes.TextApplied, len(fixRes.ModifiedFiles), suffix, time.Since(r.start).Round(time.Millisecond))
	}
}

func (r *runner) printBinaryFixResult(fixRes pipeline.FixupResult) {
	if !*r.f.FixBinary {
		return
	}
	for _, e := range fixRes.BinaryErrors {
		fmt.Fprintf(os.Stderr, "error: binary fix: %v\n", e)
	}
	if fixRes.BinaryApplied > 0 && !*r.f.Quiet {
		mode := "applied"
		if *r.f.DryRun {
			mode = "available"
		}
		fmt.Fprintf(os.Stderr, "info: %d binary fix(es) %s.\n", fixRes.BinaryApplied, mode)
	}
}
