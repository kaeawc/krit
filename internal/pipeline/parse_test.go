package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// writeKt is a small helper that writes a .kt file at path with content
// and fails the test on I/O error.
func writeKt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func TestParsePhase_Name(t *testing.T) {
	if got := (ParsePhase{}).Name(); got != "parse" {
		t.Errorf("Name() = %q, want %q", got, "parse")
	}
}

func TestParsePhase_Run_SingleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Foo.kt")
	writeKt(t, path, "class Foo {}\n")

	in := ParseInput{
		Paths: []string{dir},
	}
	out, err := (ParsePhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(out.KotlinFiles) != 1 {
		t.Fatalf("KotlinFiles = %d, want 1", len(out.KotlinFiles))
	}
	if len(out.ParseErrors) != 0 {
		t.Errorf("ParseErrors = %v, want empty", out.ParseErrors)
	}
	if !strings.HasSuffix(out.KotlinFiles[0].Path, "Foo.kt") {
		t.Errorf("KotlinFiles[0].Path = %q, want suffix Foo.kt", out.KotlinFiles[0].Path)
	}
}

func TestParsePhase_Run_GeneratedFilesSkipped(t *testing.T) {
	dir := t.TempDir()
	genPath := filepath.Join(dir, "generated", "Foo.kt")
	realPath := filepath.Join(dir, "Bar.kt")
	writeKt(t, genPath, "class Foo {}\n")
	writeKt(t, realPath, "class Bar {}\n")

	in := ParseInput{
		Paths:            []string{dir},
		IncludeGenerated: false,
	}
	out, err := (ParsePhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(out.KotlinFiles) != 1 {
		t.Fatalf("KotlinFiles = %d, want 1 (generated must be dropped)", len(out.KotlinFiles))
	}
	if !strings.HasSuffix(out.KotlinFiles[0].Path, "Bar.kt") {
		t.Errorf("kept file = %q, want the non-generated Bar.kt", out.KotlinFiles[0].Path)
	}
}

func TestParsePhase_Run_GeneratedFilesKept_WhenFlagSet(t *testing.T) {
	dir := t.TempDir()
	genPath := filepath.Join(dir, "generated", "Foo.kt")
	realPath := filepath.Join(dir, "Bar.kt")
	writeKt(t, genPath, "class Foo {}\n")
	writeKt(t, realPath, "class Bar {}\n")

	in := ParseInput{
		Paths:            []string{dir},
		IncludeGenerated: true,
	}
	out, err := (ParsePhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(out.KotlinFiles) != 2 {
		t.Fatalf("KotlinFiles = %d, want 2 (generated must be kept with flag set)", len(out.KotlinFiles))
	}
}

func TestParsePhase_Run_LPTSortDescending(t *testing.T) {
	dir := t.TempDir()
	// Build three files with clearly distinct content sizes.
	short := "class A {}\n"                                    // ~11 bytes
	medium := strings.Repeat("class B { val x = 1 }\n", 3)    // ~66 bytes
	long := strings.Repeat("class C { val x = 1 }\n", 10)     // ~220 bytes
	writeKt(t, filepath.Join(dir, "A.kt"), short)
	writeKt(t, filepath.Join(dir, "B.kt"), medium)
	writeKt(t, filepath.Join(dir, "C.kt"), long)

	in := ParseInput{
		Paths: []string{dir},
	}
	out, err := (ParsePhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(out.KotlinFiles) != 3 {
		t.Fatalf("KotlinFiles = %d, want 3", len(out.KotlinFiles))
	}
	for i := 0; i < len(out.KotlinFiles)-1; i++ {
		if len(out.KotlinFiles[i].Content) < len(out.KotlinFiles[i+1].Content) {
			t.Errorf("LPT order broken at %d: len(%d)=%d < len(%d)=%d",
				i,
				i, len(out.KotlinFiles[i].Content),
				i+1, len(out.KotlinFiles[i+1].Content))
		}
	}
	// Sanity check: first is the "long" file, last is the "short" file.
	if !strings.HasSuffix(out.KotlinFiles[0].Path, "C.kt") {
		t.Errorf("KotlinFiles[0] = %q, want suffix C.kt (largest)", out.KotlinFiles[0].Path)
	}
	if !strings.HasSuffix(out.KotlinFiles[len(out.KotlinFiles)-1].Path, "A.kt") {
		t.Errorf("KotlinFiles[last] = %q, want suffix A.kt (smallest)", out.KotlinFiles[len(out.KotlinFiles)-1].Path)
	}
}

func TestParsePhase_Run_BuildsSuppressionIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Foo.kt")
	content := "@Suppress(\"FooRule\")\nclass X {\n    val y = 1\n}\n"
	writeKt(t, path, content)

	in := ParseInput{
		Paths: []string{dir},
	}
	out, err := (ParsePhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(out.KotlinFiles) != 1 {
		t.Fatalf("KotlinFiles = %d, want 1", len(out.KotlinFiles))
	}
	f := out.KotlinFiles[0]
	if f.SuppressionIdx == nil {
		t.Fatal("SuppressionIdx must not be nil after Parse runs")
	}
	// Pick a byte offset inside the class body. "val y = 1" lives after the
	// "class X {\n    " prefix; binding to the literal "val y" offset makes
	// the assertion robust against minor whitespace changes.
	valOffset := bytes.Index([]byte(content), []byte("val y"))
	if valOffset < 0 {
		t.Fatal("fixture missing expected substring")
	}
	if !f.SuppressionIdx.IsSuppressed(valOffset, "FooRule", "") {
		t.Errorf("IsSuppressed(%d, FooRule) = false, want true", valOffset)
	}
}

// crossFileRule is a minimal v2.Rule that declares NeedsCrossFile. Used by
// the Java-collection tests to exercise the conditional Java parse path.
func crossFileRule() *v2.Rule {
	return &v2.Rule{
		ID:          "X",
		Description: "t",
		NodeTypes:   nil,
		Needs:       v2.NeedsCrossFile,
		Check:       func(*v2.Context) {},
	}
}

func TestParsePhase_Run_CollectsJavaFiles_WhenNeedsCrossFile(t *testing.T) {
	dir := t.TempDir()
	writeKt(t, filepath.Join(dir, "Foo.kt"), "class Foo {}\n")
	javaPath := filepath.Join(dir, "Bar.java")
	if err := os.WriteFile(javaPath, []byte("class Bar {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", javaPath, err)
	}

	in := ParseInput{
		Paths:       []string{dir},
		ActiveRules: []*v2.Rule{crossFileRule()},
	}
	out, err := (ParsePhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(out.JavaFiles) != 1 {
		t.Fatalf("JavaFiles = %d, want 1", len(out.JavaFiles))
	}
	if !strings.HasSuffix(out.JavaFiles[0].Path, "Bar.java") {
		t.Errorf("JavaFiles[0].Path = %q, want suffix Bar.java", out.JavaFiles[0].Path)
	}
}

func TestParsePhase_Run_SkipsJavaFiles_WhenNoCrossFileRule(t *testing.T) {
	dir := t.TempDir()
	writeKt(t, filepath.Join(dir, "Foo.kt"), "class Foo {}\n")
	javaPath := filepath.Join(dir, "Bar.java")
	if err := os.WriteFile(javaPath, []byte("class Bar {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", javaPath, err)
	}

	in := ParseInput{
		Paths:       []string{dir},
		ActiveRules: nil,
	}
	out, err := (ParsePhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if out.JavaFiles != nil {
		t.Errorf("JavaFiles = %v, want nil when no rule needs cross-file data", out.JavaFiles)
	}
}

func TestParsePhase_Run_LoggerReceivesVerboseLine(t *testing.T) {
	dir := t.TempDir()
	writeKt(t, filepath.Join(dir, "Foo.kt"), "class Foo {}\n")

	var lines []string
	in := ParseInput{
		Paths: []string{dir},
		Logger: func(format string, args ...any) {
			lines = append(lines, fmt.Sprintf(format, args...))
		},
	}
	if _, err := (ParsePhase{}).Run(context.Background(), in); err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, l := range lines {
		if strings.Contains(l, "verbose: Parsed") && strings.Contains(l, "files in") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Logger never received the Parsed-files line. Got: %v", lines)
	}
}

func TestParsePhase_Run_NilLoggerIsNoOp(t *testing.T) {
	dir := t.TempDir()
	writeKt(t, filepath.Join(dir, "Foo.kt"), "class Foo {}\n")

	in := ParseInput{
		Paths:  []string{dir},
		Logger: nil,
	}
	// Must not panic when Logger is nil.
	if _, err := (ParsePhase{}).Run(context.Background(), in); err != nil {
		t.Fatalf("Run with nil Logger: %v", err)
	}
}

func TestParsePhase_Run_TrackerRecordsParseSubPhase(t *testing.T) {
	dir := t.TempDir()
	writeKt(t, filepath.Join(dir, "Foo.kt"), "class Foo {}\n")

	tracker := perf.New(true)
	in := ParseInput{
		Paths:   []string{dir},
		Tracker: tracker,
	}
	if _, err := (ParsePhase{}).Run(context.Background(), in); err != nil {
		t.Fatalf("Run: %v", err)
	}
	entries := tracker.GetTimings()
	found := false
	for _, e := range entries {
		if e.Name == "parse" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Tracker missing 'parse' sub-phase. Got: %+v", entries)
	}
}

func TestParsePhase_Run_NilTrackerIsNoOp(t *testing.T) {
	dir := t.TempDir()
	writeKt(t, filepath.Join(dir, "Foo.kt"), "class Foo {}\n")

	in := ParseInput{
		Paths:   []string{dir},
		Tracker: nil,
	}
	// Must not panic with a nil Tracker interface value.
	if _, err := (ParsePhase{}).Run(context.Background(), in); err != nil {
		t.Fatalf("Run with nil Tracker: %v", err)
	}
}

func TestParsePhase_Run_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	writeKt(t, filepath.Join(dir, "Foo.kt"), "class Foo {}\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invocation

	in := ParseInput{
		Paths: []string{dir},
	}
	_, err := runPhase[ParseInput, ParseResult](ctx, ParsePhase{}, in)
	if err == nil {
		t.Fatal("expected error from pre-cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	var pe *PhaseError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PhaseError, got %T", err)
	}
	if pe.Phase != "parse" {
		t.Errorf("Phase = %q, want parse", pe.Phase)
	}
}
