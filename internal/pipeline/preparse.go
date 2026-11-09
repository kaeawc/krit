package pipeline

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// PreparseFuture parses the project's sources, and builds the cross-file code index from them,
// in the background while a slow producer that does not read them (the oracle JVM) runs. The
// parse and index phases take the results when their own input is identical, so a cold scan
// stops paying for them after the JVM finishes. Any mismatch or failure runs the normal phase,
// so findings never depend on it.
type PreparseFuture struct {
	parseIn ParseInput
	index   *IndexInput // nil when no code index is prebuilt
	phase   ParsePhase

	once    sync.Once
	started atomic.Bool

	parseDone chan struct{}
	parsed    ParseResult
	parseErr  error
	taken     bool

	indexDone chan struct{}
	built     prebuiltCodeIndex
}

// prebuiltCodeIndex is the code index the background goroutine built.
type prebuiltCodeIndex struct {
	ok          bool
	result      IndexResult
	timings     []perf.TimingEntry
	kotlinFiles []*scanner.File
	javaFiles   []*scanner.File
}

// StartPreparse starts parsing the files args names and, when a code index will be needed, building
// it. It returns nil when the project cannot be parsed ahead of the oracle: the caller must have
// collected the Kotlin paths and must scan every file (no analysis cache serves any of them).
func StartPreparse(ctx context.Context, args ProjectArgs, host ProjectHostState) *PreparseFuture {
	if args.KotlinPaths == nil || args.TargetedResolution {
		return nil
	}
	parseIn := projectParseInput(args, host, args.KotlinPaths, args.JavaPaths, false)
	f := &PreparseFuture{
		parseIn:   parseIn,
		phase:     ParsePhase{Workers: args.Workers},
		parseDone: make(chan struct{}),
		indexDone: make(chan struct{}),
	}
	if hasIndexBacked, _, _ := ClassifyCrossFileNeeds(args.ActiveRules); hasIndexBacked {
		f.index = &IndexInput{
			CrossFileCacheDir:   host.CrossFileCacheDir,
			CrossFileJobsFlag:   args.Workers,
			CrossFileJavaPaths:  args.JavaPaths,
			GeneratedSourceDirs: args.GeneratedSourceDirs,
			IncludeGenerated:    args.IncludeGenerated,
			ParseCache:          host.ParseCache,
			Reporter:            host.Reporter,
			Verbose:             host.Reporter.VerboseEnabled(),
		}
	}
	f.start(ctx)
	return f
}

func (f *PreparseFuture) start(ctx context.Context) {
	f.once.Do(func() {
		f.started.Store(true)
		go f.run(ctx)
	})
}

func (f *PreparseFuture) run(ctx context.Context) {
	defer close(f.indexDone)
	f.parsePhase(ctx)
	if f.parseErr != nil || f.index == nil {
		return
	}
	f.buildIndex(ctx)
}

func (f *PreparseFuture) parsePhase(ctx context.Context) {
	defer close(f.parseDone)
	defer func() {
		if r := recover(); r != nil {
			f.parseErr = fmt.Errorf("preparse panic: %v", r)
		}
	}()
	f.parsed, f.parseErr = f.phase.Run(ctx, f.parseIn)
}

func (f *PreparseFuture) buildIndex(ctx context.Context) {
	defer func() { _ = recover() }() // the index phase builds it normally
	in := *f.index
	in.ParseResult = f.parsed
	tracker := perf.New(true)
	in.CrossFileParentTracker = tracker
	var result IndexResult
	IndexPhase{}.runCodeIndexBuild(ctx, in, &result)
	f.built = prebuiltCodeIndex{
		ok:          result.CodeIndex != nil,
		result:      result,
		timings:     tracker.GetTimings(),
		kotlinFiles: f.parsed.KotlinFiles,
		javaFiles:   f.parsed.JavaFiles,
	}
}

// Take waits for the background parse and returns its result when in asks for the same parse. It
// returns false when the future is nil, was never started, was already taken, failed, or was built
// for a different input.
func (f *PreparseFuture) Take(in ParseInput) (ParseResult, bool) {
	if f == nil || !f.started.Load() || f.taken || !f.parseIn.sameParseAs(in) {
		return ParseResult{}, false
	}
	<-f.parseDone
	if f.parseErr != nil {
		return ParseResult{}, false
	}
	// The index build reads the parsed files, and the phases after the parse start to use them: wait
	// for it so the two never run at once.
	<-f.indexDone
	f.taken = true
	return f.parsed, true
}

// takeCodeIndex copies the prebuilt code index into result when in would build the same one from
// the parse result this future produced. It reports whether it did.
func (f *PreparseFuture) takeCodeIndex(in IndexInput, result *IndexResult) bool {
	if f == nil || f.index == nil || !f.taken {
		return false
	}
	want := f.index
	if in.CrossFileCacheDir != want.CrossFileCacheDir ||
		in.CrossFileJobsFlag != want.CrossFileJobsFlag ||
		in.CrossFileWorkers != 0 ||
		in.IncludeGenerated != want.IncludeGenerated ||
		in.ParseCache != want.ParseCache ||
		in.CodeIndexSnapshotLoader != nil || in.CodeIndexSnapshotSaver != nil || in.XMLFilesLoader != nil ||
		(in.CrossFileJavaPaths == nil) != (want.CrossFileJavaPaths == nil) ||
		!slices.Equal(in.CrossFileJavaPaths, want.CrossFileJavaPaths) ||
		!slices.Equal(in.GeneratedSourceDirs, want.GeneratedSourceDirs) {
		return false
	}
	b := f.built
	if !b.ok || !sameFiles(in.KotlinFiles, b.kotlinFiles) || !sameFiles(in.JavaFiles, b.javaFiles) {
		return false
	}
	result.CodeIndex = b.result.CodeIndex
	result.JavaFiles = b.result.JavaFiles
	for _, e := range b.timings {
		perf.AddEntry(in.CrossFileParentTracker, e.Name, time.Duration(e.DurationMs)*time.Millisecond)
	}
	f.index = nil // the index is built once
	return true
}

// sameFiles reports whether a and b are the same slice of files.
func sameFiles(a, b []*scanner.File) bool {
	if len(a) != len(b) {
		return false
	}
	return len(a) == 0 || &a[0] == &b[0]
}

// sameParseAs reports whether in would produce the same parse as p.
func (p ParseInput) sameParseAs(in ParseInput) bool {
	return in.Config == p.Config &&
		in.ParseCache == p.ParseCache &&
		in.ResidentFiles == nil && p.ResidentFiles == nil &&
		in.IncludeGenerated == p.IncludeGenerated &&
		in.SkipJavaCollection == p.SkipJavaCollection &&
		in.Workers == p.Workers &&
		slices.Equal(in.Paths, p.Paths) &&
		slices.Equal(in.Excludes, p.Excludes) &&
		slices.Equal(in.GeneratedSourceDirs, p.GeneratedSourceDirs) &&
		slices.Equal(in.IncludeGeneratedAllowlist, p.IncludeGeneratedAllowlist) &&
		in.KotlinPaths != nil && p.KotlinPaths != nil && slices.Equal(in.KotlinPaths, p.KotlinPaths) &&
		(in.JavaPaths == nil) == (p.JavaPaths == nil) && slices.Equal(in.JavaPaths, p.JavaPaths) &&
		slices.Equal(in.ActiveRules, p.ActiveRules)
}
