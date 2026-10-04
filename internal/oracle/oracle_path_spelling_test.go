package oracle

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// The JVM helpers report facts under the absolute paths they were sent,
// while a scan of a relative root (`krit .`) hands rules relative File.Path
// values. Every per-file lookup must reach the same facts under either
// spelling: a miss here reads as "the oracle could not resolve this call",
// which silences every rule that gates on a resolved call target.
func TestOracleLookups_RelativePathReachesAbsoluteFacts(t *testing.T) {
	// Resolve symlinks first so the working directory and the fact keys
	// agree on platforms whose temp dir is a symlink.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	rel := filepath.Join("src", "Chart.kt")
	abs := filepath.Join(dir, rel)
	source := "fun chart(dataset: List<Int>) {\n    val series = remember { dataset }\n}\n"
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	o, err := LoadFromData(&Data{Version: 1, Files: map[string]*File{
		abs: {
			Package: "test",
			Expressions: map[string]*ExpressionType{
				"2:18": {
					Type:               "kotlin.collections.List<kotlin.Int>",
					StartByte:          49,
					EndByte:            69,
					CallTarget:         "androidx.compose.runtime.remember",
					CallTargetResolved: true,
					CallTargetSuspend:  true,
					Annotations:        []string{"androidx.compose.runtime.Composable"},
				},
			},
			Diagnostics: []*Diagnostic{{FactoryName: "UNUSED_VARIABLE", Severity: "WARNING", Line: 2, Col: 9}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{abs, rel} {
		if got := o.LookupCallTarget(path, 2, 18); got != "androidx.compose.runtime.remember" {
			t.Errorf("LookupCallTarget(%q) = %q", path, got)
		}
		if got := o.LookupExpression(path, 2, 18); got == nil {
			t.Errorf("LookupExpression(%q) = nil", path)
		}
		if suspend, ok := o.LookupCallTargetSuspend(path, 2, 18); !ok || !suspend {
			t.Errorf("LookupCallTargetSuspend(%q) = %v, %v", path, suspend, ok)
		}
		if got := o.LookupCallTargetAnnotations(path, 2, 18); len(got) != 1 {
			t.Errorf("LookupCallTargetAnnotations(%q) = %v", path, got)
		}
		if got := o.LookupDiagnostics(path); len(got) != 1 {
			t.Errorf("LookupDiagnostics(%q) = %v", path, got)
		}
		if got := o.BlobHash(path); got == BlobHash(nil) {
			t.Errorf("BlobHash(%q) is the empty-file hash", path)
		}
	}

	// The byte-range path rules take: a file parsed under its relative
	// spelling, as the scan of a relative root parses it.
	file, err := scanner.ParseFile(context.Background(), rel)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.IsAbs(file.Path) {
		t.Fatalf("parsed file path %q is absolute; the test needs the relative spelling", file.Path)
	}
	var call uint32
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if call == 0 {
			call = idx
		}
	})
	if call == 0 {
		t.Fatal("no call_expression in the parsed source")
	}
	if start, end := file.FlatStartByte(call), file.FlatEndByte(call); start != 49 || end != 69 {
		t.Fatalf("remember call spans [%d, %d); the fact above is keyed to [49, 69)", start, end)
	}
	if got := o.LookupCallTargetFlat(file, call); got != "androidx.compose.runtime.remember" {
		t.Errorf("LookupCallTargetFlat = %q", got)
	}
	if suspend, ok := o.LookupCallTargetSuspendFlat(file, call); !ok || !suspend {
		t.Errorf("LookupCallTargetSuspendFlat = %v, %v", suspend, ok)
	}
	if got := o.LookupExpressionFlat(file, call); got == nil {
		t.Error("LookupExpressionFlat = nil")
	}
}

// Facts keyed by a relative path (a daemon response re-keyed to the caller's
// spelling) are read as spelled, never re-resolved against the working
// directory.
func TestOracleLookups_RelativeFactsReadAsSpelled(t *testing.T) {
	t.Chdir(t.TempDir())
	rel := filepath.Join("src", "Chart.kt")
	o, err := LoadFromData(&Data{Version: 1, Files: map[string]*File{
		rel: {Expressions: map[string]*ExpressionType{
			"2:18": {Type: "kotlin.Int", CallTarget: "androidx.compose.runtime.remember"},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := o.LookupCallTarget(rel, 2, 18); got != "androidx.compose.runtime.remember" {
		t.Errorf("LookupCallTarget(%q) = %q", rel, got)
	}
	o.SetExpressionFact(rel, 3, 1, makeResolvedType("kotlin.String", false))
	if got := o.LookupExpression(rel, 3, 1); got == nil {
		t.Errorf("LookupExpression(%q) after SetExpressionFact = nil", rel)
	}
}
