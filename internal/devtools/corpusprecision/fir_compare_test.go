package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCompareFIRNoReadyCorporaPreservesReport(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "missing report"
		if existing {
			name = "existing report"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			corpusRoot := filepath.Join(root, "corpus")
			if err := os.MkdirAll(corpusRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			reportPath := filepath.Join(root, "docs", "fir-validation.md")
			original := []byte("existing FIR validation report\n")
			if existing {
				if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(reportPath, original, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			selected := []availableCorpus{{corpus: corpus{Name: "test"}, ScanPath: corpusRoot}}
			var out bytes.Buffer
			if err := compareFIR(root, selected, true, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "No corpora with exported Gradle models were available.") {
				t.Fatalf("missing skip message: %q", out.String())
			}
			got, err := os.ReadFile(reportPath)
			if !existing {
				if !os.IsNotExist(err) {
					t.Fatalf("report was created: contents=%q, err=%v", got, err)
				}
				return
			}
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("report changed: contents=%q, err=%v", got, err)
			}
		})
	}
}

func TestParseFIRVerdict(t *testing.T) {
	const summary = "verbose: FIR verdict: 3 authoritative files, 2 gated (compiler error or crash), 1 excluded (scripts or not in a JVM source set), 1 rule errors (checker threw; Go kept for that rule and file), 1 gated (generated sources)\n"
	t.Run("zero rules", func(t *testing.T) {
		got, rows, err := parseFIRVerdict(summary)
		if err != nil || got != (firSummary{3, 2, 1, 1, 1}) || len(rows) != 0 {
			t.Fatalf("got %+v, %+v, %v", got, rows, err)
		}
	})
	t.Run("rule error and special name", func(t *testing.T) {
		verbose := summary + "verbose: FIR verdict unusual rule [x]: confirmed=2 go-dropped=1 fir-added=3 enriched-with-fix=1 suppressed=4 files-gated=3 rule-errors=1\n"
		_, rows, err := parseFIRVerdict(verbose)
		if err != nil || len(rows) != 1 || rows[0].Rule != "unusual rule [x]" || rows[0].FilesGated != 3 || rows[0].RuleErrors != 1 || rows[0].Confirmed != 2 || rows[0].GoDropped != 1 || rows[0].FirAdded != 3 {
			t.Fatalf("rows=%+v err=%v", rows, err)
		}
	})
	if _, _, err := parseFIRVerdict("verbose: unrelated\n"); err == nil {
		t.Fatal("missing summary accepted")
	}
}

func TestJoinFIRFindings(t *testing.T) {
	const goJSON = `{"findings":[{"rule":"Ported","relPath":"src/A.kt","line":1,"col":2,"lineHash":"aaaa"},{"rule":"Ported","relPath":"src/A.kt","line":2,"col":3,"lineHash":"bbbb"},{"rule":"Other","relPath":"src/A.kt","line":4,"col":1,"lineHash":"dddd"}]}`
	const firJSON = `{"findings":[{"rule":"Ported","relPath":"src/A.kt","line":1,"col":2,"lineHash":"aaaa"},{"rule":"Ported","relPath":"src/A.kt","line":3,"col":4,"lineHash":"cccc"},{"rule":"Other","relPath":"src/A.kt","line":5,"col":1,"lineHash":"eeee"}]}`
	decode := func(s string) []normalizedFinding {
		var v struct {
			Findings []normalizedFinding `json:"findings"`
		}
		if err := json.NewDecoder(strings.NewReader(s)).Decode(&v); err != nil {
			t.Fatal(err)
		}
		return v.Findings
	}
	joined := joinFIRFindings(decode(goJSON), decode(firJSON), map[string]bool{"Ported": true})
	if len(joined) != 1 {
		t.Fatalf("joined=%+v", joined)
	}
	want := firJoin{Confirmed: 1, GoDropped: []normalizedFinding{{Rule: "Ported", RelPath: "src/A.kt", Line: 2, Col: 3, LineHash: "bbbb"}}, FirAdded: []normalizedFinding{{Rule: "Ported", RelPath: "src/A.kt", Line: 3, Col: 4, LineHash: "cccc"}}}
	if !reflect.DeepEqual(joined["Ported"], want) {
		t.Fatalf("got=%+v want=%+v", joined["Ported"], want)
	}
}

func TestJoinFIRRawJSON(t *testing.T) {
	root := t.TempDir()
	corpusRoot := filepath.Join(root, "corpus")
	if err := os.MkdirAll(filepath.Join(corpusRoot, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corpusRoot, "src", "A.kt"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	decode := func(data string) []normalizedFinding {
		var raw rawReport
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			t.Fatal(err)
		}
		findings, err := normalizeFindings(root, corpusRoot, raw.Findings)
		if err != nil {
			t.Fatal(err)
		}
		return findings
	}
	goFindings := decode(`{"findings":[{"file":"corpus/src/A.kt","line":1,"column":2,"rule":"Ported"},{"file":"corpus/src/A.kt","line":2,"column":3,"rule":"Ported"}]}`)
	firFindings := decode(`{"findings":[{"file":"corpus/src/A.kt","line":1,"column":2,"rule":"Ported"},{"file":"corpus/src/A.kt","line":3,"column":4,"rule":"Ported"}]}`)
	joined := joinFIRFindings(goFindings, firFindings, map[string]bool{"Ported": true})["Ported"]
	if joined.Confirmed != 1 || len(joined.GoDropped) != 1 || len(joined.FirAdded) != 1 {
		t.Fatalf("joined=%+v", joined)
	}
	if joined.GoDropped[0].RelPath != "src/A.kt" || joined.GoDropped[0].LineHash == "" {
		t.Fatalf("signature=%+v", joined.GoDropped[0])
	}
}
