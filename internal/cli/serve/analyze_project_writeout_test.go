package serve

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestAnalyzeProject_CreateBaselineMatchesInProcess pins the
// equivalence between the daemon-routed --create-baseline path and the
// in-process WriteBaselineColumns call. The daemon ships baseline IDs
// in AnalyzeProjectStats; the CLI hands them to
// scanner.WriteBaselineIDsXML. Both must produce byte-identical XML
// to the baseline written from the in-process FindingColumns.
func TestAnalyzeProject_CreateBaselineMatchesInProcess(t *testing.T) {
	socket, state := startServerForTest(t)

	writeKotlinFile(t, state.root, "Foo.kt",
		"package demo\n\nclass Foo {\n    fun greet() = println(\"hi\")\n}\n")
	writeKotlinFile(t, state.root, "Bar.kt",
		"package demo\n\nfun bar(unused: Int): Int { return 42 }\n")

	basePath, _ := filepath.Abs(state.root)

	// --- Direct path: build FindingColumns and write baseline -----
	cfg := config.NewConfig()
	rules.ApplyConfig(cfg)
	activeRules := rules.ActiveRulesV2(map[string]bool{}, map[string]bool{}, false, false, false)
	repoDir := oracle.FindRepoDir([]string{state.root})
	if repoDir == "" {
		repoDir = state.root
	}
	pc, err := scanner.NewParseCacheWithCap(repoDir, cacheutil.DefaultParseCacheCapBytes)
	if err != nil {
		t.Fatalf("ParseCache: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	directRes, err := pipeline.RunProject(context.Background(), pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:      cfg,
			Paths:       []string{state.root},
			ActiveRules: activeRules,
			Format:      "json",
			Version:     "test",
			BasePath:    basePath,
		},
		Host: pipeline.ProjectHostState{ParseCache: pc},
	})
	if err != nil {
		t.Fatalf("direct RunProject: %v", err)
	}
	directBaseline := filepath.Join(t.TempDir(), "direct.xml")
	if err := scanner.WriteBaselineColumns(directBaseline, &directRes.FinalFindings, basePath); err != nil {
		t.Fatalf("direct WriteBaselineColumns: %v", err)
	}
	directBytes, err := os.ReadFile(directBaseline)
	if err != nil {
		t.Fatalf("read direct baseline: %v", err)
	}

	// --- Daemon path: ship baseline IDs back, write locally -------
	var verbResult daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{
			Format:         "json",
			BasePath:       basePath,
			CreateBaseline: true,
		}, &verbResult); err != nil {
		t.Fatalf("verb call: %v", err)
	}
	verbBaseline := filepath.Join(t.TempDir(), "verb.xml")
	if err := scanner.WriteBaselineIDsXML(verbBaseline, verbResult.Stats.BaselineIDs); err != nil {
		t.Fatalf("verb WriteBaselineIDsXML: %v", err)
	}
	verbBytes, err := os.ReadFile(verbBaseline)
	if err != nil {
		t.Fatalf("read verb baseline: %v", err)
	}

	if string(directBytes) != string(verbBytes) {
		t.Errorf("baseline XML diverges between in-process and daemon path\n--- direct ---\n%s\n--- verb ---\n%s",
			string(directBytes), string(verbBytes))
	}

	// Sanity: at least one baseline ID — the fixture deliberately has
	// rule-firing content so an empty result would be a regression.
	if len(verbResult.Stats.BaselineIDs) == 0 {
		t.Fatal("verb returned 0 baseline IDs; fixture should produce findings")
	}
}

// TestAnalyzeProject_DryRunMatchesInProcess pins the equivalence
// between the daemon-routed --dry-run path and the in-process
// FixupPhase count-only run. The daemon ships fixable-file list and
// counts in AnalyzeProjectStats; the CLI replays them as stdout/stderr
// lines. Both must agree on the file set, fixable count, and
// stripped-by-level count for every fix-level cap.
func TestAnalyzeProject_DryRunMatchesInProcess(t *testing.T) {
	socket, state := startServerForTest(t)

	// Use content that triggers fixable rules. A magic number plus
	// some boilerplate is reliably picked up by built-in rules with
	// fixes attached.
	writeKotlinFile(t, state.root, "Demo.kt",
		"package demo\n\nfun foo() {\n    val x = 12345\n}\n")
	writeKotlinFile(t, state.root, "Other.kt",
		"package demo\n\nfun bar() {\n    val y = 67890\n}\n")

	cfg := config.NewConfig()
	rules.ApplyConfig(cfg)
	activeRules := rules.ActiveRulesV2(map[string]bool{}, map[string]bool{}, false, false, false)
	repoDir := oracle.FindRepoDir([]string{state.root})
	if repoDir == "" {
		repoDir = state.root
	}
	pc, err := scanner.NewParseCacheWithCap(repoDir, cacheutil.DefaultParseCacheCapBytes)
	if err != nil {
		t.Fatalf("ParseCache: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	for _, fixLevel := range []string{"", "cosmetic", "idiomatic", "semantic"} {
		t.Run("level="+fixLevel, func(t *testing.T) {
			maxFixLevel := rules.FixLevel(0)
			if fixLevel != "" {
				lvl, ok := rules.ParseFixLevel(fixLevel)
				if !ok {
					t.Fatalf("ParseFixLevel(%q): not ok", fixLevel)
				}
				maxFixLevel = lvl
			}

			directRes, err := pipeline.RunProject(context.Background(), pipeline.ProjectInput{
				Args: pipeline.ProjectArgs{
					Config:      cfg,
					Paths:       []string{state.root},
					ActiveRules: activeRules,
					Format:      "json",
					Version:     "test",
					DryRun:      true,
					MaxFixLevel: maxFixLevel,
				},
				Host: pipeline.ProjectHostState{ParseCache: pc},
			})
			if err != nil {
				t.Fatalf("direct RunProject: %v", err)
			}

			directFiles := collectFixableFiles(&directRes.Fixup.Findings)

			var verbResult daemon.AnalyzeProjectResult
			if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
				daemon.AnalyzeProjectArgs{
					Format:   "json",
					DryRun:   true,
					FixLevel: fixLevel,
				}, &verbResult); err != nil {
				t.Fatalf("verb call: %v", err)
			}

			if !reflect.DeepEqual(directFiles, verbResult.Stats.DryRunFiles) {
				t.Errorf("DryRunFiles mismatch:\n direct: %v\n   verb: %v", directFiles, verbResult.Stats.DryRunFiles)
			}
			if directRes.Fixup.FixableCount != verbResult.Stats.DryRunFixableCount {
				t.Errorf("DryRunFixableCount mismatch: direct=%d verb=%d",
					directRes.Fixup.FixableCount, verbResult.Stats.DryRunFixableCount)
			}
			if directRes.Fixup.StrippedByLevel != verbResult.Stats.DryRunStrippedByLevel {
				t.Errorf("DryRunStrippedByLevel mismatch: direct=%d verb=%d",
					directRes.Fixup.StrippedByLevel, verbResult.Stats.DryRunStrippedByLevel)
			}
		})
	}
}
