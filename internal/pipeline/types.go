package pipeline

import (
	"io"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// PhaseTimings accumulates wall-clock durations for each phase. A zero
// PhaseTimings is valid; fields only populate for phases that actually
// ran. Consumers read the durations to emit --perf summaries.
type PhaseTimings struct {
	Parse     time.Duration
	Index     time.Duration
	Dispatch  time.Duration
	CrossFile time.Duration
	Fixup     time.Duration
	Output    time.Duration
}

// ParseInput is the entry value for the Parse phase.
type ParseInput struct {
	// Config is the loaded krit.yml / .krit.yml (or the default config).
	Config *config.Config
	// Paths are the CLI arguments: files or directories to analyse.
	Paths []string
	// Excludes are glob patterns pulled from Config that Parse honours
	// when walking directories.
	Excludes []string
	// ActiveRules declares which rules are enabled for this run. Parse
	// inspects the union of Needs bits to decide whether to collect
	// Java sources (NeedsCrossFile) or skip file-size-based LPT sort
	// (no heavy per-file rules).
	ActiveRules []*v2.Rule
	// IncludeGenerated, when true, retains files under /generated/.
	IncludeGenerated bool
}

// ParseResult is the output of Parse and the input of Index.
type ParseResult struct {
	// Config is carried forward unchanged for downstream phases.
	Config *config.Config
	// ActiveRules is carried forward unchanged.
	ActiveRules []*v2.Rule
	// KotlinFiles are successfully parsed Kotlin sources, sorted by
	// content length descending for LPT scheduling.
	KotlinFiles []*scanner.File
	// JavaFiles are Java sources collected when any active rule needs
	// cross-file analysis. Nil otherwise.
	JavaFiles []*scanner.File
	// Paths is the original input list, preserved for Output (base
	// path calculation for relative paths in reports).
	Paths []string
	// ParseErrors captures non-fatal read/parse failures. Parse still
	// returns a non-nil ParseResult so downstream phases can run on
	// the files that did parse.
	ParseErrors []error
	// Timings carries the cumulative phase timings so Output can emit
	// a --perf summary at the end.
	Timings PhaseTimings
}

// IndexResult is the output of Index and the input of Dispatch. It
// embeds ParseResult so callers can inspect the parse output from any
// later phase without chasing pointers.
type IndexResult struct {
	ParseResult
	// Resolver is the type resolver wired into rules that declare
	// NeedsResolver. Nil when no active rule needs it.
	Resolver typeinfer.TypeResolver
	// Oracle is the Kotlin Analysis API-backed type oracle. Nil when
	// --no-type-oracle or no rule needs oracle data.
	Oracle *oracle.Oracle
	// CodeIndex is the cross-file symbol/reference index. Nil when
	// no active rule declares NeedsCrossFile.
	CodeIndex *scanner.CodeIndex
	// ModuleGraph lists discovered Gradle modules. Always populated
	// (possibly empty).
	ModuleGraph *module.ModuleGraph
	// ModuleIndex is the per-module index of files/symbols for rules
	// that declare NeedsModuleIndex. Nil when no such rule is active.
	ModuleIndex *module.PerModuleIndex
	// AndroidProject is the detected Android project layout (manifest,
	// resources, gradle). Nil when no Android manifest is found or no
	// rule needs Android data.
	AndroidProject *android.AndroidProject
	// Cache is the incremental analysis cache. Nil when --no-cache or
	// the cache file cannot be opened.
	Cache *cache.Cache
	// CacheResult holds the lookup result for the current paths.
	// CachedPaths lists files that hit the cache and can skip
	// per-file dispatch; CachedColumns holds the findings to be
	// re-merged into the final output.
	CacheResult *cache.CacheResult
	// RuleHash is the hash of active rules + config used as the cache
	// key; carried so Fixup can update the cache with the same hash.
	RuleHash string
	// CacheFilePath is the resolved cache file location, empty when
	// caching is disabled.
	CacheFilePath string
}

// DispatchResult is the output of Dispatch and the input of CrossFile.
// It adds the per-file findings (already filtered through each file's
// SuppressionIndex by the dispatcher) and the rule run statistics.
type DispatchResult struct {
	IndexResult
	// Findings are the per-file findings produced by the dispatcher,
	// already suppression-filtered.
	Findings scanner.FindingColumns
	// Stats captures per-rule CPU time, walk time, and any panics.
	Stats rules.RunStats
}

// CrossFileResult is the output of CrossFile and the input of Fixup.
// Cross-file, module-aware, and Android rule findings are appended to
// Findings after being run through each finding's target-file
// SuppressionIndex — fixing the bug where cross-file rules bypassed
// suppression in the pre-refactor code.
type CrossFileResult struct {
	DispatchResult
}

// FixupResult is the output of Fixup and the input of Output.
type FixupResult struct {
	CrossFileResult
	// AppliedFixes is the count of findings whose fix was applied to
	// disk (text + binary combined). Zero when --fix is not set.
	AppliedFixes int
	// TextApplied is the count of text fixes applied to disk. Zero when
	// Apply was false.
	TextApplied int
	// BinaryApplied is the count of binary fixes applied (or, when
	// DryRunBinary is true, the count that would be applied). Zero when
	// ApplyBinary was false.
	BinaryApplied int
	// StrippedByLevel is the number of text fixes dropped because their
	// rule's fix level exceeded MaxFixLevel. Zero when MaxFixLevel is 0
	// (no cap).
	StrippedByLevel int
	// FixableCount is the number of rows that still carry a text fix
	// after the MaxFixLevel filter ran. Callers use this to decide
	// whether there is anything to apply / report as available.
	FixableCount int
	// ModifiedFiles lists the files touched by Fixup, in stable order.
	ModifiedFiles []string
	// FixErrors captures non-fatal errors raised while applying text or
	// binary fixes. Callers surface these to stderr; Fixup itself does
	// not fail on per-file fix errors.
	FixErrors []error
	// BinaryErrors captures the subset of FixErrors that originated from
	// binary fix application. Callers that distinguish text vs binary in
	// their stderr output can use this slice directly.
	BinaryErrors []error
}

// OutputInput is the entry value for the Output phase. Output is the
// only phase whose side effect isn't a value return — it writes
// formatted findings to the supplied Writer.
type OutputInput struct {
	FixupResult
	// Writer is where the formatted output lands (os.Stdout for CLI,
	// a buffer for tests and MCP).
	Writer io.Writer
	// Format is one of "json", "plain", "sarif", "checkstyle", or
	// empty to auto-detect based on whether Writer is a terminal.
	Format string
	// BaselinePath, when non-empty, is the path to a detekt-format
	// baseline XML used to suppress known findings.
	BaselinePath string
	// DiffRef, when non-empty, is a git ref (e.g. "main",
	// "origin/main") used to restrict findings to files changed
	// since that ref.
	DiffRef string
	// BasePath is the base for relative paths in the output; empty
	// means "first scan path".
	BasePath string
	// ShowPerf, when true, causes Output to append a --perf summary
	// to Writer.
	ShowPerf bool
	// StartTime is when the overall run began; used by Output to
	// compute total duration in the JSON header.
	StartTime time.Time
	// Version is the krit version string embedded in JSON/SARIF output.
	Version string
	// ExperimentNames are the active experiment flag names, echoed in
	// JSON output.
	ExperimentNames []string
	// PerfTimings, when non-nil, are forwarded into FormatJSONColumns
	// so the JSON header includes a --perf timing summary.
	PerfTimings []perf.TimingEntry
	// CacheStats, when non-nil, are forwarded into FormatJSONColumns so
	// the JSON header includes cache hit/miss counters.
	CacheStats *cache.CacheStats
	// WarningsAsErrors, when true, promotes warning-severity findings
	// to errors before format dispatch.
	WarningsAsErrors bool
	// MinConfidence, when >0, drops findings whose confidence is below
	// the threshold before format dispatch.
	MinConfidence float64
	// ActiveRulesV1, when non-nil, overrides the v2→v1 conversion that
	// Output would otherwise derive from FixupResult.ActiveRules. Main
	// already has the v1 rule slice handy, so it can skip the conversion.
	ActiveRulesV1 []rules.Rule
}

// OutputResult captures post-Output metadata.
type OutputResult struct {
	// FinalFindings is the set that was actually written (after any
	// baseline/diff filters). Empty when Output ran in a mode that
	// filters everything out.
	FinalFindings scanner.FindingColumns
	// Timings is the full phase timing set including Output.
	Timings PhaseTimings
}
