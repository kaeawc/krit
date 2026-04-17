package pipeline

import (
	"context"
	"errors"
	"sort"

	"github.com/kaeawc/krit/internal/fixer"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// FixupInput configures the Fixup phase. It wraps CrossFileResult rather
// than stuffing options onto the shared result type so non-fix callers
// (e.g. --dry-run pipelines, LSP) can leave the behaviour flags unset.
type FixupInput struct {
	CrossFileResult
	// Apply, when true, applies text fixes to disk. When false, Fixup
	// is a no-op (returns FixupResult with zero AppliedFixes). --dry-run
	// callers also set this false and use the findings themselves.
	Apply bool
	// ApplyBinary, when true, additionally applies binary-format fixes
	// (file renames, resource moves). Independent of Apply.
	ApplyBinary bool
	// Suffix is an optional file suffix passed to fixer.ApplyAllFixesColumns
	// (e.g., ".fixed"). Empty string = in-place.
	Suffix string
	// MaxFixLevel caps which fix levels are applied (FixCosmetic <
	// FixIdiomatic < FixSemantic). Zero means apply all.
	MaxFixLevel rules.FixLevel
	// DryRunBinary, when true, runs binary fix application in dry-run
	// mode (reports what would happen without modifying disk).
	DryRunBinary bool
}

// FixupPhase applies auto-fixes (text and/or binary) that were attached
// to findings by earlier phases. Fix-level filtering happens before
// application so callers can cap the acceptable safety level.
type FixupPhase struct{}

// Name returns the phase name used in timings and error wrapping.
func (FixupPhase) Name() string { return "fixup" }

// Run executes the Fixup phase. When neither Apply nor ApplyBinary is
// true, Run is a no-op and returns the input's CrossFileResult unchanged.
//
// Per-file fix errors are returned via FixupResult.FixErrors — they do
// not cause Run itself to fail. Only catastrophic failures bubble up as
// a non-nil error return.
func (FixupPhase) Run(_ context.Context, in FixupInput) (FixupResult, error) {
	if !in.Apply && !in.ApplyBinary {
		return FixupResult{CrossFileResult: in.CrossFileResult}, nil
	}

	var columns scanner.FindingColumns = in.CrossFileResult.Findings

	// Filter by fix level: strip text fixes whose rule exceeds the cap.
	// Findings stay in the result; only the Fix pointer is dropped so
	// downstream Output still reports them.
	if in.MaxFixLevel > 0 {
		ruleLevels := make(map[string]rules.FixLevel, len(rules.Registry))
		for _, r := range rules.Registry {
			ruleLevels[r.Name()] = rules.GetFixLevel(r)
		}
		columns.StripTextFixes(func(row int) bool {
			return ruleLevels[columns.RuleAt(row)] > in.MaxFixLevel
		})
	}

	var (
		fixErrs       []error
		textFixes     int
		binaryFixes   int
		modifiedFiles []string
	)

	// Track files whose fixes were applied so we can report ModifiedFiles.
	// We snapshot the files that still carry a text fix post-filter before
	// calling the fixer — the fixer strips nothing, so these are the files
	// it will touch.
	if in.Apply {
		touched := make(map[string]struct{})
		columns.VisitRowsWithTextFixes(func(row int) {
			touched[columns.FileAt(row)] = struct{}{}
		})

		applied, _, errs := fixer.ApplyAllFixesColumns(&columns, in.Suffix)
		textFixes = applied
		fixErrs = append(fixErrs, errs...)

		for file := range touched {
			modifiedFiles = append(modifiedFiles, file)
		}
	}

	if in.ApplyBinary {
		applied, errs := fixer.ApplyBinaryFixesBatchColumns(&columns, in.DryRunBinary)
		binaryFixes = applied
		fixErrs = append(fixErrs, errs...)
	}

	sort.Strings(modifiedFiles)

	// Reflect post-filter fixes into the carried CrossFileResult so
	// Output sees the same column set we operated on.
	out := in.CrossFileResult
	out.Findings = columns

	result := FixupResult{
		CrossFileResult: out,
		AppliedFixes:    textFixes + binaryFixes,
		ModifiedFiles:   modifiedFiles,
		FixErrors:       fixErrs,
	}

	// errors.Join on an empty slice returns nil — catastrophic failures
	// would have to be surfaced explicitly; fix errors travel through
	// FixErrors, not the error return.
	return result, errors.Join( /* no catastrophic errors in current flow */ )
}

// Compile-time check: FixupPhase satisfies Phase[FixupInput, FixupResult].
var _ Phase[FixupInput, FixupResult] = FixupPhase{}
