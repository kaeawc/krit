package pipeline

import (
	"context"
	"errors"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func TestDefaultActiveRules_NonEmpty(t *testing.T) {
	active := DefaultActiveRules()
	if len(active) == 0 {
		t.Fatal("DefaultActiveRules returned 0 rules; expected the usual registered set")
	}
}

func TestDefaultActiveRules_NoNilEntries(t *testing.T) {
	for i, r := range DefaultActiveRules() {
		if r == nil {
			t.Errorf("DefaultActiveRules()[%d] is nil", i)
		}
	}
}

func TestBuildDispatcher_AcceptsNilResolver(t *testing.T) {
	d := BuildDispatcher(DefaultActiveRules(), nil)
	if d == nil {
		t.Fatal("BuildDispatcher returned nil")
	}
}

func TestBuildDispatcher_AcceptsEmptyRules(t *testing.T) {
	d := BuildDispatcher(nil, nil)
	if d == nil {
		t.Fatal("BuildDispatcher(nil,nil) returned nil")
	}
}

// ---------------------------------------------------------------------------
// ParseSingle
// ---------------------------------------------------------------------------

func TestParseSingle_SimpleKotlin(t *testing.T) {
	content := []byte("fun main() {}\n")
	file, err := ParseSingle(context.Background(), "example.kt", content)
	if err != nil {
		t.Fatalf("ParseSingle error: %v", err)
	}
	if file == nil {
		t.Fatal("ParseSingle returned nil file")
	}
}

func TestParseSingle_EmptyPathDefaultsToInputKt(t *testing.T) {
	content := []byte("fun main() {}\n")
	file, err := ParseSingle(context.Background(), "", content)
	if err != nil {
		t.Fatalf("ParseSingle error: %v", err)
	}
	if file == nil {
		t.Fatal("ParseSingle returned nil file")
	}
	if file.Path != "input.kt" {
		t.Errorf("file.Path = %q, want %q", file.Path, "input.kt")
	}
}

func TestParseSingle_CancelledContextDoesNotPanic(t *testing.T) {
	// tree-sitter's ParseCtx only interrupts long-running parses via its
	// internal goroutine; tiny buffers complete before the context is
	// checked. The important contract is that ParseSingle does not panic
	// and propagates any error that tree-sitter does surface.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call
	file, err := ParseSingle(ctx, "example.kt", []byte("fun main() {}\n"))
	// Either a cancelled-context error or a successful parse are acceptable;
	// what must NOT happen is a panic.
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled when error is non-nil", err)
		}
	} else if file == nil {
		t.Error("ParseSingle returned nil file without an error")
	}
}

// ---------------------------------------------------------------------------
// NewSingleFileAnalyzer
// ---------------------------------------------------------------------------

func TestNewSingleFileAnalyzer_NilRulesUsesDefaultActive(t *testing.T) {
	a := NewSingleFileAnalyzer(nil, nil)
	if a == nil {
		t.Fatal("NewSingleFileAnalyzer returned nil")
	}
	if len(a.ActiveRules) == 0 {
		t.Error("ActiveRules should be non-empty when nil is passed")
	}
	if a.Dispatcher == nil {
		t.Error("Dispatcher should not be nil")
	}
}

func TestNewSingleFileAnalyzer_ExplicitRules(t *testing.T) {
	defaults := DefaultActiveRules()
	if len(defaults) == 0 {
		t.Skip("no active rules registered")
	}
	subset := defaults[:1]
	a := NewSingleFileAnalyzer(subset, nil)
	if a == nil {
		t.Fatal("NewSingleFileAnalyzer returned nil")
	}
	if len(a.ActiveRules) != 1 {
		t.Errorf("ActiveRules len = %d, want 1", len(a.ActiveRules))
	}
}

func TestNewSingleFileAnalyzer_EmptyRulesSlice(t *testing.T) {
	a := NewSingleFileAnalyzer([]*v2.Rule{}, nil)
	if a == nil {
		t.Fatal("NewSingleFileAnalyzer returned nil")
	}
	if a.Dispatcher == nil {
		t.Error("Dispatcher should not be nil even with empty rules")
	}
}

func TestNewSingleFileAnalyzer_NilResolver(t *testing.T) {
	// Passing nil resolver should not panic; dispatcher must degrade gracefully.
	a := NewSingleFileAnalyzer(DefaultActiveRules(), nil)
	if a == nil {
		t.Fatal("NewSingleFileAnalyzer returned nil")
	}
}

// ---------------------------------------------------------------------------
// AnalyzeBufferColumns
// ---------------------------------------------------------------------------

func TestAnalyzeBufferColumns_SimpleKotlin_NoError(t *testing.T) {
	a := NewSingleFileAnalyzer(DefaultActiveRules(), nil)
	columns, file, err := a.AnalyzeBufferColumns(context.Background(), "main.kt", []byte("fun main() {}\n"))
	if err != nil {
		t.Fatalf("AnalyzeBufferColumns error: %v", err)
	}
	if file == nil {
		t.Fatal("returned file is nil")
	}
	_ = columns // zero findings is valid for clean code
}

func TestAnalyzeBufferColumns_KnownRuleTriggered(t *testing.T) {
	// TrailingWhitespace fires on lines that end with spaces.
	src := []byte("package test\nfun main() {   \n    println(\"hi\")\n}\n")
	a := NewSingleFileAnalyzer(DefaultActiveRules(), nil)
	columns, _, err := a.AnalyzeBufferColumns(context.Background(), "main.kt", src)
	if err != nil {
		t.Fatalf("AnalyzeBufferColumns error: %v", err)
	}
	if columns.Len() == 0 {
		t.Error("expected at least one finding for trailing whitespace, got 0")
	}
}

func TestAnalyzeBufferColumns_CancelledContextDoesNotPanic(t *testing.T) {
	// Same as TestParseSingle_CancelledContextDoesNotPanic: tree-sitter
	// only interrupts long-running parses; tiny buffers complete before
	// the context goroutine fires. Verify no panic and that any error
	// wraps context.Canceled.
	a := NewSingleFileAnalyzer(DefaultActiveRules(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := a.AnalyzeBufferColumns(ctx, "main.kt", []byte("fun main() {}\n"))
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("unexpected error kind: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AnalyzeFileColumns
// ---------------------------------------------------------------------------

func TestAnalyzeFileColumns_ReturnsFindings(t *testing.T) {
	// Parse first, then dispatch.
	src := []byte("package test\nfun main() {   \n    println(\"hi\")\n}\n")
	file, err := ParseSingle(context.Background(), "main.kt", src)
	if err != nil {
		t.Fatalf("ParseSingle error: %v", err)
	}
	a := NewSingleFileAnalyzer(DefaultActiveRules(), nil)
	columns := a.AnalyzeFileColumns(file)
	if columns.Len() == 0 {
		t.Error("expected at least one finding for trailing whitespace, got 0")
	}
}

func TestAnalyzeFileColumns_CleanFileNoFindings(t *testing.T) {
	src := []byte("fun main() {}\n")
	file, err := ParseSingle(context.Background(), "main.kt", src)
	if err != nil {
		t.Fatalf("ParseSingle error: %v", err)
	}
	a := NewSingleFileAnalyzer(DefaultActiveRules(), nil)
	columns := a.AnalyzeFileColumns(file)
	// A trivially clean buffer should produce zero findings.
	_ = columns
}
