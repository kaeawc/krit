package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type snapshotRecordingBundleStore struct {
	saved []*scanner.FindingColumns
}

func (s *snapshotRecordingBundleStore) Load(string, scanner.RunFingerprint) (*scanner.FindingColumns, bool) {
	return nil, false
}

func (s *snapshotRecordingBundleStore) Save(_ string, _ scanner.RunFingerprint, cols *scanner.FindingColumns) error {
	s.saved = append(s.saved, cols)
	return nil
}

// TestRunProjectAnalysis_BundleSaveDoesNotAliasResult is the regression test
// for #770: the deferred findings-bundle save and the resident mirror used to
// hold &crossFileResult.Findings, whose backing arrays the returned result
// shares. OutputPhase then sorted those columns in place while the daemon's
// background-save goroutine was still gob-encoding them. Mutating the returned
// findings before draining the parked save must leave both the saved and the
// resident columns untouched, and both must already be in file/line order.
func TestRunProjectAnalysis_BundleSaveDoesNotAliasResult(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"A.kt": "package test\n\nclass A1\n\nclass A2\n",
		"B.kt": "package test\n\nclass B1\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
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

	store := &snapshotRecordingBundleStore{}
	resident := map[string]*scanner.FindingColumns{}
	var parked []func()
	res, err := RunProjectAnalysis(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
			Version:     "test",
		},
		Host: ProjectHostState{
			FindingsBundleStore:     store,
			FindingsBundleCacheRoot: t.TempDir(),
			DaemonCaches: DaemonCaches{
				StoreResidentBundle: func(key string, cols *scanner.FindingColumns) { resident[key] = cols },
				BackgroundSave:      func(fn func()) { parked = append(parked, fn) },
			},
		},
	})
	if err != nil {
		t.Fatalf("RunProjectAnalysis: %v", err)
	}
	findings := &res.CrossFileResult.Findings
	if findings.Len() != 3 {
		t.Fatalf("findings = %d, want 3", findings.Len())
	}
	if len(resident) != 1 {
		t.Fatalf("resident bundles stashed = %d, want 1", len(resident))
	}

	// Stand in for OutputPhase and any other in-place consumer: scribble
	// over every row of the returned columns before the parked save runs.
	for i := range findings.Line {
		findings.Line[i] = 999
	}
	for _, fn := range parked {
		fn()
	}
	if len(store.saved) != 1 {
		t.Fatalf("bundle saves = %d, want 1", len(store.saved))
	}

	for label, cols := range map[string]*scanner.FindingColumns{
		"saved":    store.saved[0],
		"resident": resident[scanner.FindingsBundleKey(res.RunFP)],
	} {
		if cols == nil {
			t.Fatalf("%s bundle missing", label)
		}
		if cols.Len() != 3 {
			t.Fatalf("%s bundle rows = %d, want 3", label, cols.Len())
		}
		for row := 0; row < cols.Len(); row++ {
			if cols.Line[row] == 999 {
				t.Fatalf("%s bundle row %d shares storage with the returned findings", label, row)
			}
		}
		sorted := cols.Clone()
		sorted.SortByFileLine()
		for row := 0; row < cols.Len(); row++ {
			if cols.FileAt(row) != sorted.FileAt(row) || cols.Line[row] != sorted.Line[row] {
				t.Fatalf("%s bundle is not in file/line order at row %d", label, row)
			}
		}
	}
}
