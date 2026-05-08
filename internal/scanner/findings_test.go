package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

func TestFindingCollectorRoundTrip(t *testing.T) {
	original := []Finding{
		{
			File:     "src/Foo.kt",
			Line:     12,
			Col:      3,
			RuleSet:  "style",
			Rule:     "UseLet",
			Severity: "warning",
			Message:  "prefer let",
			Fix: &Fix{
				StartLine:   12,
				EndLine:     12,
				Replacement: "value?.let { it.length }",
			},
			Confidence: 0.95,
		},
		{
			File:     "res/icon.png",
			Line:     1,
			Col:      1,
			RuleSet:  "android",
			Rule:     "ConvertToWebp",
			Severity: "error",
			Message:  "convert image asset",
			BinaryFix: &BinaryFix{
				Type:         BinaryFixConvertWebP,
				SourcePath:   "res/icon.png",
				TargetPath:   "res/icon.webp",
				Description:  "convert to webp",
				DeleteSource: true,
				Content:      []byte{1, 2, 3},
				MinSdk:       21,
			},
			Confidence: 0.5,
		},
		{
			File:     "src/Foo.kt",
			Line:     24,
			Col:      7,
			RuleSet:  "style",
			Rule:     "UseLet",
			Severity: "info",
			Message:  "prefer let",
		},
	}

	columns := CollectFindings(original)

	if columns.Len() != len(original) {
		t.Fatalf("expected %d rows, got %d", len(original), columns.Len())
	}
	if len(columns.Files) != 2 {
		t.Fatalf("expected 2 unique files, got %d", len(columns.Files))
	}
	if len(columns.RuleSets) != 2 {
		t.Fatalf("expected 2 unique rule sets, got %d", len(columns.RuleSets))
	}
	if len(columns.Rules) != 2 {
		t.Fatalf("expected 2 unique rules, got %d", len(columns.Rules))
	}
	if len(columns.Messages) != 2 {
		t.Fatalf("expected 2 unique messages, got %d", len(columns.Messages))
	}

	roundTrip := columns.Findings()
	if !reflect.DeepEqual(roundTrip, original) {
		t.Fatalf("round-trip mismatch:\nwant: %#v\ngot:  %#v", original, roundTrip)
	}
}

func TestFindingColumnsJSONRoundTrip_UsesStableLowercaseSchema(t *testing.T) {
	original := CollectFindings([]Finding{
		{
			File:     "src/Foo.kt",
			Line:     12,
			Col:      3,
			RuleSet:  "style",
			Rule:     "UseLet",
			Severity: "warning",
			Message:  "prefer let",
			Fix: &Fix{
				StartLine:   12,
				EndLine:     12,
				Replacement: "value?.let { it.length }",
			},
			Confidence: 0.95,
		},
		{
			File:     "res/icon.png",
			Line:     1,
			Col:      1,
			RuleSet:  "android",
			Rule:     "ConvertToWebp",
			Severity: "error",
			Message:  "convert image asset",
			BinaryFix: &BinaryFix{
				Type:         BinaryFixConvertWebP,
				SourcePath:   "res/icon.png",
				TargetPath:   "res/icon.webp",
				Description:  "convert to webp",
				DeleteSource: true,
				Content:      []byte{1, 2, 3},
				MinSdk:       21,
			},
			Confidence: 0.5,
		},
	})

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if strings.Contains(string(data), `"Files"`) {
		t.Fatalf("expected lowercase JSON field names, got %s", string(data))
	}
	if !strings.Contains(string(data), `"files"`) {
		t.Fatalf("expected lowercase files field in JSON, got %s", string(data))
	}

	var roundTrip FindingColumns
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(roundTrip.Findings(), original.Findings()) {
		t.Fatalf("JSON round-trip mismatch:\nwant: %#v\ngot:  %#v", original.Findings(), roundTrip.Findings())
	}
}

func TestFindingColumnsUnmarshalJSON_AcceptsLegacyExportedFieldNames(t *testing.T) {
	data := []byte(`{
  "Files": ["src/Foo.kt"],
  "RuleSets": ["style"],
  "Rules": ["UseLet"],
  "Messages": ["prefer let"],
  "FixPool": [{"StartLine": 1, "EndLine": 1, "Replacement": "fixed()"}],
  "BinaryFixPool": [{"Type": 2, "Content": "cGF5bG9hZA=="}],
  "FileIdx": [0],
  "Line": [1],
  "Col": [2],
  "RuleSetIdx": [0],
  "RuleIdx": [0],
  "SeverityID": [1],
  "MessageIdx": [0],
  "Confidence": [95],
  "FixStart": [1],
  "BinaryFixStart": [1],
  "N": 1
}`)

	var columns FindingColumns
	if err := json.Unmarshal(data, &columns); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	want := []Finding{{
		File:       "src/Foo.kt",
		Line:       1,
		Col:        2,
		RuleSet:    "style",
		Rule:       "UseLet",
		Severity:   "warning",
		Message:    "prefer let",
		Confidence: 0.95,
		Fix: &Fix{
			StartLine:   1,
			EndLine:     1,
			Replacement: "fixed()",
		},
		BinaryFix: &BinaryFix{
			Type:    BinaryFixCreateFile,
			Content: []byte("payload"),
		},
	}}

	if !reflect.DeepEqual(columns.Findings(), want) {
		t.Fatalf("legacy JSON mismatch:\nwant: %#v\ngot:  %#v", want, columns.Findings())
	}
}

func TestFindingColumnsUnmarshalJSON_RejectsMismatchedRowLengths(t *testing.T) {
	var columns FindingColumns
	err := json.Unmarshal([]byte(`{
  "files": ["src/Foo.kt"],
  "fileIdx": [0],
  "line": [1, 2],
  "col": [1],
  "ruleSetIdx": [0],
  "ruleIdx": [0],
  "severityID": [1],
  "messageIdx": [0],
  "confidence": [95],
  "fixStart": [0],
  "binaryFixStart": [0]
}`), &columns)
	if err == nil {
		t.Fatal("expected mismatched row lengths to fail")
	}
}

func TestFindingColumnsUnmarshalJSON_RejectsMissingRowSlicesWhenNIsSet(t *testing.T) {
	var columns FindingColumns
	err := json.Unmarshal([]byte(`{
  "files": ["src/Foo.kt"],
  "ruleSets": ["style"],
  "rules": ["UseLet"],
  "messages": ["prefer let"],
  "fileIdx": [0],
  "ruleSetIdx": [0],
  "ruleIdx": [0],
  "severityID": [1],
  "messageIdx": [0],
  "confidence": [95],
  "fixStart": [0],
  "binaryFixStart": [0],
  "n": 1
}`), &columns)
	if err == nil {
		t.Fatal("expected missing line/col row slices to fail")
	}
}

func TestFindingColumnsUnmarshalJSON_RejectsOutOfRangeInternedIndexes(t *testing.T) {
	var columns FindingColumns
	err := json.Unmarshal([]byte(`{
  "files": ["src/Foo.kt"],
  "ruleSets": ["style"],
  "rules": ["UseLet"],
  "messages": ["prefer let"],
  "fileIdx": [1],
  "line": [1],
  "col": [2],
  "ruleSetIdx": [0],
  "ruleIdx": [0],
  "severityID": [1],
  "messageIdx": [0],
  "confidence": [95],
  "fixStart": [0],
  "binaryFixStart": [0]
}`), &columns)
	if err == nil {
		t.Fatal("expected invalid fileIdx to fail")
	}
}

func TestFindingColumnsUnmarshalJSON_RejectsOutOfRangeFixReferences(t *testing.T) {
	var columns FindingColumns
	err := json.Unmarshal([]byte(`{
  "files": ["src/Foo.kt"],
  "ruleSets": ["style"],
  "rules": ["UseLet"],
  "messages": ["prefer let"],
  "fixPool": [],
  "binaryFixPool": [],
  "fileIdx": [0],
  "line": [1],
  "col": [2],
  "ruleSetIdx": [0],
  "ruleIdx": [0],
  "severityID": [1],
  "messageIdx": [0],
  "confidence": [95],
  "fixStart": [1],
  "binaryFixStart": [1]
}`), &columns)
	if err == nil {
		t.Fatal("expected invalid fix references to fail")
	}
}

func TestFindingColumnsSortByFileLine(t *testing.T) {
	columns := CollectFindings([]Finding{
		{File: "b.kt", Line: 20, Col: 1, RuleSet: "style", Rule: "B", Severity: "warning", Message: "third"},
		{File: "a.kt", Line: 20, Col: 4, RuleSet: "style", Rule: "D", Severity: "warning", Message: "second"},
		{File: "a.kt", Line: 10, Col: 3, RuleSet: "style", Rule: "C", Severity: "warning", Message: "first"},
		{File: "a.kt", Line: 20, Col: 2, RuleSet: "style", Rule: "A", Severity: "warning", Message: "tie-break"},
	})

	columns.SortByFileLine()
	got := columns.Findings()

	want := []Finding{
		{File: "a.kt", Line: 10, Col: 3, RuleSet: "style", Rule: "C", Severity: "warning", Message: "first"},
		{File: "a.kt", Line: 20, Col: 2, RuleSet: "style", Rule: "A", Severity: "warning", Message: "tie-break"},
		{File: "a.kt", Line: 20, Col: 4, RuleSet: "style", Rule: "D", Severity: "warning", Message: "second"},
		{File: "b.kt", Line: 20, Col: 1, RuleSet: "style", Rule: "B", Severity: "warning", Message: "third"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted findings mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestFindingColumnsSortByFileLine_UsesLexicalTieBreakersWithInternedTables(t *testing.T) {
	columns := CollectFindings([]Finding{
		{
			File:     "z.kt",
			Line:     4,
			Col:      9,
			RuleSet:  "zeta",
			Rule:     "Zulu",
			Severity: "warning",
			Message:  "zeta first inserted",
			Fix: &Fix{
				StartLine:   4,
				EndLine:     4,
				Replacement: "zeta()",
			},
		},
		{
			File:     "z.kt",
			Line:     4,
			Col:      9,
			RuleSet:  "alpha",
			Rule:     "Beta",
			Severity: "warning",
			Message:  "alpha should sort first",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("alpha"),
			},
		},
		{
			File:     "z.kt",
			Line:     4,
			Col:      9,
			RuleSet:  "alpha",
			Rule:     "Alpha",
			Severity: "warning",
			Message:  "alpha rule should sort first",
		},
	})

	columns.SortByFileLine()
	got := columns.Findings()

	if got[0].RuleSet != "alpha" || got[0].Rule != "Alpha" {
		t.Fatalf("expected lexical alpha/Alpha first, got %#v", got[0])
	}
	if got[1].RuleSet != "alpha" || got[1].Rule != "Beta" {
		t.Fatalf("expected lexical alpha/Beta second, got %#v", got[1])
	}
	if got[2].RuleSet != "zeta" || got[2].Rule != "Zulu" {
		t.Fatalf("expected lexical zeta/Zulu third, got %#v", got[2])
	}
	if got[1].BinaryFix == nil || string(got[1].BinaryFix.Content) != "alpha" {
		t.Fatalf("expected binary fix to stay attached to second row, got %#v", got[1].BinaryFix)
	}
	if got[2].Fix == nil || got[2].Fix.Replacement != "zeta()" {
		t.Fatalf("expected text fix to stay attached to third row, got %#v", got[2].Fix)
	}
}

func TestFindingColumnsSortedRowOrderByFileLine(t *testing.T) {
	columns := CollectFindings([]Finding{
		{File: "b.kt", Line: 20, Col: 1, RuleSet: "style", Rule: "B", Severity: "warning", Message: "third"},
		{File: "a.kt", Line: 20, Col: 4, RuleSet: "style", Rule: "D", Severity: "warning", Message: "second"},
		{File: "a.kt", Line: 10, Col: 3, RuleSet: "style", Rule: "C", Severity: "warning", Message: "first"},
		{File: "a.kt", Line: 20, Col: 2, RuleSet: "style", Rule: "A", Severity: "warning", Message: "tie-break"},
	})

	order := columns.SortedRowOrderByFileLine()
	want := []int{2, 3, 1, 0}

	if !reflect.DeepEqual(order, want) {
		t.Fatalf("sorted row order mismatch:\nwant: %#v\ngot:  %#v", want, order)
	}

	if got := columns.Findings(); !reflect.DeepEqual(got, []Finding{
		{File: "b.kt", Line: 20, Col: 1, RuleSet: "style", Rule: "B", Severity: "warning", Message: "third"},
		{File: "a.kt", Line: 20, Col: 4, RuleSet: "style", Rule: "D", Severity: "warning", Message: "second"},
		{File: "a.kt", Line: 10, Col: 3, RuleSet: "style", Rule: "C", Severity: "warning", Message: "first"},
		{File: "a.kt", Line: 20, Col: 2, RuleSet: "style", Rule: "A", Severity: "warning", Message: "tie-break"},
	}) {
		t.Fatalf("SortedRowOrderByFileLine should not mutate columns, got %#v", got)
	}
}

func TestFindingColumnsSortedRowOrderByFileLine_PreservesOriginalOrderForEqualKeys(t *testing.T) {
	columns := CollectFindings([]Finding{
		{File: "a.kt", Line: 10, Col: 2, RuleSet: "style", Rule: "Same", Severity: "warning", Message: "first"},
		{File: "a.kt", Line: 10, Col: 2, RuleSet: "style", Rule: "Same", Severity: "error", Message: "second"},
		{File: "a.kt", Line: 10, Col: 2, RuleSet: "style", Rule: "Same", Severity: "info", Message: "third"},
	})

	order := columns.SortedRowOrderByFileLine()
	want := []int{0, 1, 2}

	if !reflect.DeepEqual(order, want) {
		t.Fatalf("stable tie order mismatch:\nwant: %#v\ngot:  %#v", want, order)
	}
}

func TestFindingColumnsVisitSortedByFileLine_MatchesSortedRowOrder(t *testing.T) {
	columns := CollectFindings([]Finding{
		{File: "b.kt", Line: 20, Col: 1, RuleSet: "style", Rule: "B", Severity: "warning", Message: "third"},
		{File: "a.kt", Line: 20, Col: 4, RuleSet: "style", Rule: "D", Severity: "warning", Message: "second"},
		{File: "a.kt", Line: 10, Col: 3, RuleSet: "style", Rule: "C", Severity: "warning", Message: "first"},
		{File: "a.kt", Line: 20, Col: 2, RuleSet: "style", Rule: "A", Severity: "warning", Message: "tie-break"},
	})

	var visited []int
	columns.VisitSortedByFileLine(func(row int) {
		visited = append(visited, row)
	})

	if want := columns.SortedRowOrderByFileLine(); !reflect.DeepEqual(visited, want) {
		t.Fatalf("visit order mismatch:\nwant: %#v\ngot:  %#v", want, visited)
	}
}

func TestFindingColumnsSortByFileLine_MatchesLegacyComparatorOnSyntheticCorpus(t *testing.T) {
	original := syntheticFindings(2048)
	want := append([]Finding(nil), original...)
	sortFindingsByFileLine(want)

	columns := CollectFindings(original)
	columns.SortByFileLine()
	got := columns.Findings()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("columnar sort mismatch against legacy comparator")
	}
}

func TestApplyIndexedPermutationInPlace(t *testing.T) {
	values := []int{10, 20, 30, 40, 50}
	destBySrc := []int{2, 0, 4, 1, 3}

	applyIndexedPermutationInPlace(destBySrc, func(i, j int) {
		swapSlice(values, i, j)
	})

	want := []int{20, 40, 10, 50, 30}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("permuted values mismatch:\nwant: %#v\ngot:  %#v", want, values)
	}
	if !reflect.DeepEqual(destBySrc, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("expected permutation scratch to normalize to identity, got %#v", destBySrc)
	}
}

func TestFindingColumnsCloneIsIndependent(t *testing.T) {
	original := CollectFindings([]Finding{
		{
			File:     "src/Foo.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "android",
			Rule:     "ConvertToWebp",
			Severity: "warning",
			Message:  "asset issue",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("original"),
			},
		},
	})

	clone := original.Clone()
	clone.Files[0] = "other.kt"
	clone.BinaryFixPool[0].Content[0] = 'X'

	if original.Files[0] != "src/Foo.kt" {
		t.Fatalf("mutating clone changed original file table: %q", original.Files[0])
	}
	if string(original.BinaryFixPool[0].Content) != "original" {
		t.Fatalf("mutating clone changed original binary fix content: %q", string(original.BinaryFixPool[0].Content))
	}
}

func TestFindingColumnsClone_DoesNotWarmSortRankCaches(t *testing.T) {
	original := CollectFindings([]Finding{
		{File: "z.kt", Line: 3, Col: 1, RuleSet: "zeta", Rule: "Zulu", Severity: "warning", Message: "z"},
		{File: "a.kt", Line: 1, Col: 2, RuleSet: "alpha", Rule: "Alpha", Severity: "warning", Message: "a"},
		{File: "m.kt", Line: 2, Col: 3, RuleSet: "beta", Rule: "Beta", Severity: "warning", Message: "m"},
	})

	if original.fileLexRanks != nil || original.ruleSetLexRanks != nil || original.ruleLexRanks != nil {
		t.Fatal("expected sort rank caches to start empty")
	}

	clone := original.Clone()

	if original.fileLexRanks != nil || original.ruleSetLexRanks != nil || original.ruleLexRanks != nil {
		t.Fatal("expected Clone to leave source sort rank caches cold")
	}
	if clone.fileLexRanks != nil || clone.ruleSetLexRanks != nil || clone.ruleLexRanks != nil {
		t.Fatal("expected Clone to leave clone sort rank caches cold")
	}
}

func TestFindingColumnsClonePreservesWarmSortRankCaches(t *testing.T) {
	original := CollectFindings([]Finding{
		{File: "z.kt", Line: 3, Col: 1, RuleSet: "zeta", Rule: "Zulu", Severity: "warning", Message: "z"},
		{File: "a.kt", Line: 1, Col: 2, RuleSet: "alpha", Rule: "Alpha", Severity: "warning", Message: "a"},
		{File: "m.kt", Line: 2, Col: 3, RuleSet: "beta", Rule: "Beta", Severity: "warning", Message: "m"},
	})

	original.prepareSortRanks()
	clone := original.Clone()

	if original.fileLexRanks == nil || original.ruleSetLexRanks == nil || original.ruleLexRanks == nil {
		t.Fatal("expected warmed source sort rank caches to remain populated")
	}
	if !reflect.DeepEqual(clone.fileLexRanks, original.fileLexRanks) {
		t.Fatalf("file rank cache mismatch:\nwant: %#v\ngot:  %#v", original.fileLexRanks, clone.fileLexRanks)
	}
	if !reflect.DeepEqual(clone.ruleSetLexRanks, original.ruleSetLexRanks) {
		t.Fatalf("ruleset rank cache mismatch:\nwant: %#v\ngot:  %#v", original.ruleSetLexRanks, clone.ruleSetLexRanks)
	}
	if !reflect.DeepEqual(clone.ruleLexRanks, original.ruleLexRanks) {
		t.Fatalf("rule rank cache mismatch:\nwant: %#v\ngot:  %#v", original.ruleLexRanks, clone.ruleLexRanks)
	}

	clone.fileLexRanks[0] = 99
	clone.ruleSetLexRanks[0] = 88
	clone.ruleLexRanks[0] = 77
	if original.fileLexRanks[0] == 99 || original.ruleSetLexRanks[0] == 88 || original.ruleLexRanks[0] == 77 {
		t.Fatal("clone should own independent copies of sort rank caches")
	}
}

func TestFindingColumnsPromoteWarningsToErrors(t *testing.T) {
	columns := CollectFindings([]Finding{
		{File: "a.kt", Line: 1, Col: 1, RuleSet: "style", Rule: "Warn", Severity: "warning", Message: "warn"},
		{File: "a.kt", Line: 2, Col: 1, RuleSet: "style", Rule: "Info", Severity: "info", Message: "info"},
		{File: "a.kt", Line: 3, Col: 1, RuleSet: "style", Rule: "Err", Severity: "error", Message: "err"},
	})

	columns.PromoteWarningsToErrors()
	got := columns.Findings()

	if got[0].Severity != "error" {
		t.Fatalf("expected warning to become error, got %q", got[0].Severity)
	}
	if got[1].Severity != "info" {
		t.Fatalf("expected info to remain info, got %q", got[1].Severity)
	}
	if got[2].Severity != "error" {
		t.Fatalf("expected error to remain error, got %q", got[2].Severity)
	}
}

func TestFindingColumnsStripTextFixes(t *testing.T) {
	columns := CollectFindings([]Finding{
		{
			File:     "a.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "KeepFix",
			Severity: "warning",
			Message:  "keep text fix",
			Fix: &Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "kept()",
			},
		},
		{
			File:     "a.kt",
			Line:     2,
			Col:      1,
			RuleSet:  "style",
			Rule:     "DropFix",
			Severity: "warning",
			Message:  "drop text fix",
			Fix: &Fix{
				StartLine:   2,
				EndLine:     2,
				Replacement: "dropped()",
			},
		},
		{
			File:     "a.kt",
			Line:     3,
			Col:      1,
			RuleSet:  "style",
			Rule:     "BinaryOnly",
			Severity: "warning",
			Message:  "keep binary fix",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("payload"),
			},
		},
	})

	if got := columns.CountTextFixes(); got != 2 {
		t.Fatalf("CountTextFixes before strip = %d, want 2", got)
	}

	stripped := columns.StripTextFixes(func(row int) bool {
		return columns.RuleAt(row) == "DropFix"
	})

	if stripped != 1 {
		t.Fatalf("StripTextFixes stripped %d rows, want 1", stripped)
	}
	if got := columns.CountTextFixes(); got != 1 {
		t.Fatalf("CountTextFixes after strip = %d, want 1", got)
	}
	if !columns.HasFix(0) {
		t.Fatal("expected first row fix to remain")
	}
	if columns.HasFix(1) {
		t.Fatal("expected second row fix to be stripped")
	}
	if columns.BinaryFixStart[2] == 0 {
		t.Fatal("expected binary fix to remain after stripping text fixes")
	}

	got := columns.Findings()
	if got[0].Fix == nil {
		t.Fatal("expected first finding fix to remain after round-trip")
	}
	if got[1].Fix != nil {
		t.Fatal("expected stripped finding fix to be nil after round-trip")
	}
	if got[2].BinaryFix == nil {
		t.Fatal("expected binary fix to survive round-trip")
	}
}

func TestFindingColumnsFindingsWithFixes(t *testing.T) {
	columns := CollectFindings([]Finding{
		{
			File:     "a.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "TextFix",
			Severity: "warning",
			Message:  "text fix",
			Fix: &Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "fixed()",
			},
		},
		{
			File:     "a.kt",
			Line:     2,
			Col:      1,
			RuleSet:  "style",
			Rule:     "NoFix",
			Severity: "warning",
			Message:  "no fix",
		},
		{
			File:     "b.png",
			Line:     1,
			Col:      1,
			RuleSet:  "android",
			Rule:     "BinaryFix",
			Severity: "error",
			Message:  "binary fix",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("payload"),
			},
		},
	})

	got := columns.FindingsWithFixes()
	want := []Finding{
		{
			File:     "a.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "TextFix",
			Severity: "warning",
			Message:  "text fix",
			Fix: &Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "fixed()",
			},
		},
		{
			File:     "b.png",
			Line:     1,
			Col:      1,
			RuleSet:  "android",
			Rule:     "BinaryFix",
			Severity: "error",
			Message:  "binary fix",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("payload"),
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindingsWithFixes mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}

	columns.BinaryFixPool[0].Content[0] = 'X'
	if string(got[1].BinaryFix.Content) != "payload" {
		t.Fatalf("FindingsWithFixes should clone binary fix content, got %q", string(got[1].BinaryFix.Content))
	}
}

func TestFilterColumnsByFilePaths_UsesAbsoluteMatchesAndPreservesRows(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore Chdir failed: %v", err)
		}
	})
	keepPath := filepath.Join("src", "Keep.kt")
	dropPath := filepath.Join("src", "Drop.kt")
	keepAbs, err := filepath.Abs(keepPath)
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}

	columns := CollectFindings([]Finding{
		{
			File:     keepPath,
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "UseLet",
			Severity: "warning",
			Message:  "keep text fix",
			Fix: &Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "kept()",
			},
		},
		{
			File:     dropPath,
			Line:     2,
			Col:      3,
			RuleSet:  "style",
			Rule:     "DropMe",
			Severity: "error",
			Message:  "drop this row",
		},
		{
			File:     keepPath,
			Line:     4,
			Col:      2,
			RuleSet:  "android",
			Rule:     "ConvertToWebp",
			Severity: "warning",
			Message:  "keep binary fix",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("keep"),
			},
		},
	})

	filtered := FilterColumnsByFilePaths(&columns, map[string]bool{keepAbs: true})

	want := []Finding{
		{
			File:     keepPath,
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "UseLet",
			Severity: "warning",
			Message:  "keep text fix",
			Fix: &Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "kept()",
			},
		},
		{
			File:     keepPath,
			Line:     4,
			Col:      2,
			RuleSet:  "android",
			Rule:     "ConvertToWebp",
			Severity: "warning",
			Message:  "keep binary fix",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("keep"),
			},
		},
	}

	if !reflect.DeepEqual(filtered.Findings(), want) {
		t.Fatalf("filtered findings mismatch:\nwant: %#v\ngot:  %#v", want, filtered.Findings())
	}
}

func TestFindingColumnsFilterRows_PreservesFixPoolsAndCloneFallback(t *testing.T) {
	source := CollectFindings([]Finding{
		{
			File:     "src/Keep.kt",
			Line:     3,
			Col:      1,
			RuleSet:  "style",
			Rule:     "KeepRule",
			Severity: "warning",
			Message:  "keep me",
			Fix: &Fix{
				StartLine:   3,
				EndLine:     3,
				Replacement: "fixed()",
			},
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("payload"),
			},
			Confidence: 0.91,
		},
		{
			File:     "src/Drop.kt",
			Line:     8,
			Col:      2,
			RuleSet:  "style",
			Rule:     "DropRule",
			Severity: "error",
			Message:  "drop me",
		},
	})

	filtered := source.FilterRows(func(row int) bool {
		return source.RuleAt(row) == "KeepRule"
	})
	want := []Finding{{
		File:     "src/Keep.kt",
		Line:     3,
		Col:      1,
		RuleSet:  "style",
		Rule:     "KeepRule",
		Severity: "warning",
		Message:  "keep me",
		Fix: &Fix{
			StartLine:   3,
			EndLine:     3,
			Replacement: "fixed()",
		},
		BinaryFix: &BinaryFix{
			Type:    BinaryFixCreateFile,
			Content: []byte("payload"),
		},
		Confidence: 0.91,
	}}
	if got := filtered.Findings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered findings mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}

	source.BinaryFixPool[0].Content[0] = 'X'
	if got := string(filtered.BinaryFixPool[0].Content); got != "payload" {
		t.Fatalf("filtered binary fix should be independent, got %q", got)
	}

	cloned := source.FilterRows(nil)
	if !reflect.DeepEqual(cloned.Findings(), source.Findings()) {
		t.Fatalf("FilterRows(nil) should clone all rows:\nwant: %#v\ngot:  %#v", source.Findings(), cloned.Findings())
	}
	if &cloned.FileIdx[0] == &source.FileIdx[0] {
		t.Fatal("FilterRows(nil) should return an independent clone")
	}
}

func TestFilterByMinConfidence(t *testing.T) {
	source := CollectFindings([]Finding{
		{
			File: "src/High.kt", Line: 1, Col: 1,
			RuleSet: "style", Rule: "HighConf", Severity: "warning",
			Message: "high", Confidence: 0.95,
		},
		{
			File: "src/Mid.kt", Line: 2, Col: 1,
			RuleSet: "style", Rule: "MidConf", Severity: "warning",
			Message: "mid", Confidence: 0.60,
		},
		{
			File: "src/Low.kt", Line: 3, Col: 1,
			RuleSet: "style", Rule: "LowConf", Severity: "warning",
			Message: "low", Confidence: 0.20,
		},
		{
			File: "src/Unset.kt", Line: 4, Col: 1,
			RuleSet: "style", Rule: "UnsetConf", Severity: "warning",
			Message: "unset (Confidence=0)",
		},
	})

	// min=0 keeps everything (including unset).
	kept := source.FilterByMinConfidence(0)
	if kept.N != 4 {
		t.Errorf("min=0: expected 4 rows kept, got %d", kept.N)
	}

	// min=0.5 drops Low and Unset.
	half := source.FilterByMinConfidence(0.5)
	if half.N != 2 {
		t.Errorf("min=0.5: expected 2 rows kept, got %d", half.N)
	}
	halfRules := map[string]bool{}
	for _, f := range half.Findings() {
		halfRules[f.Rule] = true
	}
	if !halfRules["HighConf"] || !halfRules["MidConf"] {
		t.Errorf("min=0.5: expected HighConf+MidConf, got %v", halfRules)
	}

	// min=0.9 keeps only HighConf.
	nine := source.FilterByMinConfidence(0.9)
	if nine.N != 1 {
		t.Errorf("min=0.9: expected 1 row kept, got %d", nine.N)
	}
	if nine.Findings()[0].Rule != "HighConf" {
		t.Errorf("min=0.9: expected HighConf, got %s", nine.Findings()[0].Rule)
	}

	// min=1.0 drops everything.
	full := source.FilterByMinConfidence(1.0)
	if full.N != 0 {
		t.Errorf("min=1.0: expected 0 rows kept, got %d", full.N)
	}
}

func TestFindingCollectorAppendColumnsMergesInternTablesAndPools(t *testing.T) {
	left := CollectFindings([]Finding{
		{
			File:     "src/Foo.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "UseLet",
			Severity: "warning",
			Message:  "prefer let",
			Fix: &Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "left",
			},
		},
	})
	right := CollectFindings([]Finding{
		{
			File:     "src/Foo.kt",
			Line:     2,
			Col:      3,
			RuleSet:  "style",
			Rule:     "UseLet",
			Severity: "error",
			Message:  "prefer let",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("binary"),
			},
			Confidence: 0.83,
		},
		{
			File:     "src/Bar.kt",
			Line:     5,
			Col:      2,
			RuleSet:  "performance",
			Rule:     "AvoidAlloc",
			Severity: "warning",
			Message:  "avoid allocation",
		},
	})

	collector := NewFindingCollector(0)
	collector.AppendColumns(&left)
	collector.AppendColumns(&right)
	merged := collector.Columns()

	if merged.Len() != 3 {
		t.Fatalf("expected 3 rows, got %d", merged.Len())
	}
	if len(merged.Files) != 2 {
		t.Fatalf("expected 2 unique files, got %d", len(merged.Files))
	}
	if len(merged.RuleSets) != 2 {
		t.Fatalf("expected 2 unique rule sets, got %d", len(merged.RuleSets))
	}
	if len(merged.Rules) != 2 {
		t.Fatalf("expected 2 unique rules, got %d", len(merged.Rules))
	}
	if len(merged.Messages) != 2 {
		t.Fatalf("expected 2 unique messages, got %d", len(merged.Messages))
	}

	want := append(left.Findings(), right.Findings()...)
	got := merged.Findings()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged findings mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}

	right.BinaryFixPool[0].Content[0] = 'X'
	if string(merged.BinaryFixPool[0].Content) != "binary" {
		t.Fatalf("merged binary fix should be independent, got %q", string(merged.BinaryFixPool[0].Content))
	}
}

func TestFindingCollectorInternsStringTablesAcrossCollectors(t *testing.T) {
	left := CollectFindings([]Finding{{
		File:     strings.Clone("src/Foo.kt"),
		Line:     1,
		Col:      1,
		RuleSet:  strings.Clone("style"),
		Rule:     strings.Clone("UseLet"),
		Severity: "warning",
		Message:  strings.Clone("prefer let"),
	}})
	right := CollectFindings([]Finding{{
		File:     strings.Clone("src/Foo.kt"),
		Line:     2,
		Col:      4,
		RuleSet:  strings.Clone("style"),
		Rule:     strings.Clone("UseLet"),
		Severity: "error",
		Message:  strings.Clone("prefer let"),
	}})

	if unsafe.StringData(left.Files[0]) != unsafe.StringData(right.Files[0]) {
		t.Fatal("expected file table entries to share interned storage across collectors")
	}
	if unsafe.StringData(left.RuleSets[0]) != unsafe.StringData(right.RuleSets[0]) {
		t.Fatal("expected ruleset table entries to share interned storage across collectors")
	}
	if unsafe.StringData(left.Rules[0]) != unsafe.StringData(right.Rules[0]) {
		t.Fatal("expected rule table entries to share interned storage across collectors")
	}
	if unsafe.StringData(left.Messages[0]) != unsafe.StringData(right.Messages[0]) {
		t.Fatal("expected message table entries to share interned storage across collectors")
	}
}

func TestFindingCollectorAppendRowCopiesFixPools(t *testing.T) {
	source := CollectFindings([]Finding{
		{
			File:     "src/Foo.kt",
			Line:     7,
			Col:      2,
			RuleSet:  "style",
			Rule:     "UseLet",
			Severity: "warning",
			Message:  "prefer let",
			Fix: &Fix{
				StartLine:   7,
				EndLine:     7,
				Replacement: "value?.let { it.length }",
			},
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("payload"),
			},
			Confidence: 0.91,
		},
	})

	collector := NewFindingCollector(1)
	collector.AppendRow(&source, 0)
	got := collector.Columns()

	if !reflect.DeepEqual(got.Findings(), source.Findings()) {
		t.Fatalf("append row mismatch:\nwant: %#v\ngot:  %#v", source.Findings(), got.Findings())
	}

	source.BinaryFixPool[0].Content[0] = 'X'
	if string(got.BinaryFixPool[0].Content) != "payload" {
		t.Fatalf("copied binary fix should be independent, got %q", string(got.BinaryFixPool[0].Content))
	}
}

func TestFindingColumnsScalarAccessors(t *testing.T) {
	columns := CollectFindings([]Finding{
		{
			File:       "src/Foo.kt",
			Line:       12,
			Col:        3,
			RuleSet:    "style",
			Rule:       "UseLet",
			Severity:   "warning",
			Message:    "prefer let",
			Confidence: 0.95,
			Fix: &Fix{
				StartLine:   12,
				EndLine:     12,
				Replacement: "value?.let { it.length }",
			},
		},
		{
			File:     "src/Bar.kt",
			Line:     2,
			Col:      1,
			RuleSet:  "performance",
			Rule:     "AvoidAlloc",
			Severity: "info",
			Message:  "avoid allocation",
		},
	})

	if got := columns.FileAt(0); got != "src/Foo.kt" {
		t.Fatalf("FileAt(0) = %q, want %q", got, "src/Foo.kt")
	}
	if got := columns.LineAt(0); got != 12 {
		t.Fatalf("LineAt(0) = %d, want 12", got)
	}
	if got := columns.ColumnAt(0); got != 3 {
		t.Fatalf("ColumnAt(0) = %d, want 3", got)
	}
	if got := columns.RuleSetAt(0); got != "style" {
		t.Fatalf("RuleSetAt(0) = %q, want %q", got, "style")
	}
	if got := columns.RuleAt(0); got != "UseLet" {
		t.Fatalf("RuleAt(0) = %q, want %q", got, "UseLet")
	}
	if got := columns.SeverityAt(0); got != "warning" {
		t.Fatalf("SeverityAt(0) = %q, want %q", got, "warning")
	}
	if got := columns.MessageAt(0); got != "prefer let" {
		t.Fatalf("MessageAt(0) = %q, want %q", got, "prefer let")
	}
	if got := columns.ConfidenceAt(0); got != 0.95 {
		t.Fatalf("ConfidenceAt(0) = %v, want 0.95", got)
	}
	if !columns.HasFix(0) {
		t.Fatal("HasFix(0) = false, want true")
	}
	if columns.HasFix(1) {
		t.Fatal("HasFix(1) = true, want false")
	}
}

func TestFindingColumnsFixAccessorsReturnIndependentCopies(t *testing.T) {
	columns := CollectFindings([]Finding{
		{
			File:     "src/Foo.kt",
			Line:     7,
			Col:      2,
			RuleSet:  "style",
			Rule:     "UseLet",
			Severity: "warning",
			Message:  "prefer let",
			Fix: &Fix{
				StartLine:   7,
				EndLine:     7,
				Replacement: "fixed()",
			},
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("payload"),
			},
		},
		{
			File:     "src/Bar.kt",
			Line:     3,
			Col:      1,
			RuleSet:  "style",
			Rule:     "NoFix",
			Severity: "info",
			Message:  "no fix",
		},
	})

	textFix := columns.FixAt(0)
	if textFix == nil {
		t.Fatal("FixAt(0) = nil, want fix")
	}
	if got := textFix.Replacement; got != "fixed()" {
		t.Fatalf("FixAt(0).Replacement = %q, want %q", got, "fixed()")
	}
	if got := columns.FixAt(1); got != nil {
		t.Fatalf("FixAt(1) = %#v, want nil", got)
	}

	binaryFix := columns.BinaryFixAt(0)
	if binaryFix == nil {
		t.Fatal("BinaryFixAt(0) = nil, want fix")
	}
	if got := string(binaryFix.Content); got != "payload" {
		t.Fatalf("BinaryFixAt(0).Content = %q, want %q", got, "payload")
	}
	if got := columns.BinaryFixAt(1); got != nil {
		t.Fatalf("BinaryFixAt(1) = %#v, want nil", got)
	}

	textFix.Replacement = "changed()"
	binaryFix.Content[0] = 'X'
	if got := columns.FixPool[0].Replacement; got != "fixed()" {
		t.Fatalf("FixAt should return an independent copy, pool replacement = %q", got)
	}
	if got := string(columns.BinaryFixPool[0].Content); got != "payload" {
		t.Fatalf("BinaryFixAt should clone content, pool content = %q", got)
	}
}

func TestFindingColumnsVisitRowsWithFixes(t *testing.T) {
	columns := CollectFindings([]Finding{
		{
			File:     "src/Foo.kt",
			Line:     1,
			Col:      1,
			RuleSet:  "style",
			Rule:     "TextFix",
			Severity: "warning",
			Message:  "text fix",
			Fix: &Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "fixed()",
			},
		},
		{
			File:     "src/Foo.kt",
			Line:     2,
			Col:      1,
			RuleSet:  "style",
			Rule:     "BinaryFix",
			Severity: "warning",
			Message:  "binary fix",
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("payload"),
			},
		},
		{
			File:     "src/Foo.kt",
			Line:     3,
			Col:      1,
			RuleSet:  "style",
			Rule:     "BothFixes",
			Severity: "warning",
			Message:  "both fixes",
			Fix: &Fix{
				StartLine:   3,
				EndLine:     3,
				Replacement: "both()",
			},
			BinaryFix: &BinaryFix{
				Type:    BinaryFixCreateFile,
				Content: []byte("both"),
			},
		},
		{
			File:     "src/Foo.kt",
			Line:     4,
			Col:      1,
			RuleSet:  "style",
			Rule:     "NoFix",
			Severity: "warning",
			Message:  "no fix",
		},
	})

	var textRows []int
	columns.VisitRowsWithTextFixes(func(row int) {
		textRows = append(textRows, row)
	})
	if !reflect.DeepEqual(textRows, []int{0, 2}) {
		t.Fatalf("VisitRowsWithTextFixes mismatch:\nwant: %#v\ngot:  %#v", []int{0, 2}, textRows)
	}

	var binaryRows []int
	columns.VisitRowsWithBinaryFixes(func(row int) {
		binaryRows = append(binaryRows, row)
	})
	if !reflect.DeepEqual(binaryRows, []int{1, 2}) {
		t.Fatalf("VisitRowsWithBinaryFixes mismatch:\nwant: %#v\ngot:  %#v", []int{1, 2}, binaryRows)
	}
}
