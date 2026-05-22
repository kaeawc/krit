package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/pipeline"
)

// runDaemonFix replays --fix / --fix-binary against the daemon-shipped
// FindingColumns. The daemon never applies fixes (FixupPhase short-
// circuits daemon-side because args.Fix / args.FixBinary are not
// forwarded across the wire), so the CLI runs pipeline.FixupPhase
// locally against the deserialised columns. This preserves the
// "daemon never writes user files" invariant while still letting the
// scan itself benefit from daemon-resident state (parse cache,
// resolver, oracle, cross-file indexes).
//
// Always returns handled=true: the daemon-routed --fix path emits its
// own findings (via the in-process FixupPhase + emitDaemonFilteredColumns
// re-render) rather than streaming the daemon's pre-strip findings
// bytes through writeDaemonFindings. Falling through would double-
// write the findings.
func runDaemonFix(f *scanFlags, paths []string, res daemon.AnalyzeProjectResult) (handled bool, code int) {
	cols, err := decodeDaemonColumns(res.Columns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --fix via daemon: %v\n", err)
		return true, 2
	}
	maxFixLevel, ok := resolveMaxFixLevel(f)
	if !ok {
		return true, 2
	}
	preCount := cols.Len()
	start := time.Now()
	fixRes, _ := (pipeline.FixupPhase{}).Run(context.Background(), pipeline.FixupInput{
		CrossFileResult: pipeline.CrossFileResult{
			DispatchResult: pipeline.DispatchResult{
				Findings: *cols,
			},
		},
		Apply:         *f.Fix && !*f.DryRun,
		ApplyBinary:   *f.FixBinary,
		Suffix:        *f.FixSuffix,
		MaxFixLevel:   maxFixLevel,
		DryRunBinary:  *f.DryRun,
		CountOnly:     *f.DryRun,
		OnlyFindingID: *f.FixFindingID,
	})

	printFixupResult(f, fixRes, start)
	printBinaryFixResult(f, fixRes)

	if *f.Output == "" && resolveEffectiveFormat(f) == "json" && *f.Report == "" && *f.Fix {
		if preCount-fixRes.FixableCount > 0 {
			if !*f.Quiet {
				fmt.Fprintf(os.Stderr, "info: %d unfixable issue(s) remain.\n", preCount-fixRes.FixableCount)
			}
			return true, 1
		}
		return true, 0
	}
	// Non-short-circuit: re-render post-strip columns through
	// OutputPhase locally so the findings sink matches the in-process
	// flow byte-for-byte. The daemon's pre-strip findings JSON is
	// discarded — mirrors how --delta uses emitDaemonFilteredColumns.
	return true, emitDaemonFilteredColumns(f, paths, &fixRes.Findings, resolveBasePath(*f.BasePath, paths))
}

// runDaemonRemoveDeadCode replays --remove-dead-code against the
// daemon-shipped columns. The daemon does not own the disk write —
// dead-code removal mutates user source via fixer.ApplyAllFixesColumns,
// so the CLI runs it locally. The format / dry-run / suffix knobs
// mirror the in-process RunDeadCodeRemovalColumns invocation.
func runDaemonRemoveDeadCode(f *scanFlags, _ []string, raw json.RawMessage) int {
	cols, err := decodeDaemonColumns(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --remove-dead-code via daemon: %v\n", err)
		return 2
	}
	return RunDeadCodeRemovalColumns(cols, resolveEffectiveFormat(f), *f.DryRun, *f.FixSuffix)
}
