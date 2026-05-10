package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRunProject_AnalysisCacheLookup_ByteEqualAcrossRuns is the
// byte-equal contract test #126 calls out: a second RunProject call
// with lookup enabled must produce output indistinguishable from the
// first (cold) call. This is the safety net for the daemon-resident
// AnalysisCache lookup path — if cache hits + cache misses don't
// reassemble identically, the daemon's output silently drifts from
// the CLI's.
func TestRunProject_AnalysisCacheLookup_ByteEqualAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".krit", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "krit.cache")

	// Write a few synthetic files so cache lookup has something to
	// actually classify.
	for _, name := range []string{"A.kt", "B.kt", "C.kt"} {
		if err := os.WriteFile(filepath.Join(dir, name),
			[]byte("package test\n\nclass "+name[:1]+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	run := func(t *testing.T) []byte {
		t.Helper()
		loaded := cache.Load(cachePath)
		if loaded == nil {
			t.Fatal("cache.Load returned nil")
		}
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{rule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				AnalysisCache:         loaded,
				AnalysisCacheFilePath: cachePath,
				AnalysisCacheLookup:   true,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return stripFindingTimestamps(res.JSON)
	}

	first := run(t)
	second := run(t)
	if string(first) != string(second) {
		t.Errorf("byte-equal contract failed:\n--- first ---\n%s\n--- second ---\n%s",
			first, second)
	}
}

// TestRunProject_AnalysisCacheLookup_DisabledByDefault verifies that
// without AnalysisCacheLookup=true, RunProject keeps the pre-#126
// write-only behavior — every file is dispatched on every call,
// regardless of cache contents.
func TestRunProject_AnalysisCacheLookup_DisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "krit.cache")
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"),
		[]byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	loaded := cache.Load(cachePath)
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
		},
		Host: ProjectHostState{
			AnalysisCache:         loaded,
			AnalysisCacheFilePath: cachePath,
			// AnalysisCacheLookup intentionally false.
		},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	if res.FindingsCount != 1 {
		t.Errorf("FindingsCount = %d, want 1 (rule must dispatch when lookup disabled)", res.FindingsCount)
	}
}

// stripFindingTimestamps drops timing-related JSON fields that change
// across runs so byte-equal comparisons exercise just the findings,
// not the wall-clock fluctuations.
func stripFindingTimestamps(jsonBytes []byte) []byte {
	// Wall durations and timestamps differ across runs even with
	// identical findings. JSON output embeds `"durationMs":...` and
	// `"timestamp":"..."` near the top. We strip naively by line —
	// JSON output is pretty-printed in this codebase, so per-field
	// stripping is robust enough for the byte-equal contract.
	return scrubLines(jsonBytes, []string{
		"\"durationMs\"",
		"\"timestamp\"",
		"\"wallSeconds\"",
		"\"startTime\"",
		"\"endTime\"",
	})
}

func scrubLines(data []byte, dropTokens []string) []byte {
	const newline = byte('\n')
	out := make([]byte, 0, len(data))
	start := 0
	for i := 0; i <= len(data); i++ {
		if i != len(data) && data[i] != newline {
			continue
		}
		line := data[start:i]
		drop := false
		for _, tok := range dropTokens {
			if bytesContains(line, []byte(tok)) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, line...)
			if i != len(data) {
				out = append(out, newline)
			}
		}
		start = i + 1
	}
	return out
}

func bytesContains(hay, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		match := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
