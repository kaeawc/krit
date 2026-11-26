package firchecks

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// The settings-group sessions' verdicts replace the merged ones when they
// answer a file or report a missing opt-in; any other error they report for a
// file the merged session answered keeps the merged verdict.
func TestPassAppliesSettingsGroupVerdicts(t *testing.T) {
	p := isolationProject(t)
	merged := NewFakeFirChecker()
	merged.Rules = []string{verdictRule}
	// Gated by a flag only its own source set enables.
	merged.ErrorFiles = map[string]string{p.abs(isolateTestFile): "'when' expression must be exhaustive."}
	var gotFiles []string
	var gotID int64
	splitter := func(_ context.Context, _ string, files []string, request []byte) ([]byte, []string, error) {
		gotFiles = files
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(request, &req); err != nil {
			t.Fatal(err)
		}
		gotID = req.ID
		finding := p.fir(isolateTestFile, "Dispatchers.IO", verdictRule)
		resp, _ := json.Marshal(map[string]any{
			"id":    req.ID,
			"rules": []string{verdictRule},
			"findings": []FirFinding{{Path: finding.File, Line: finding.Line, Col: finding.Col,
				StartByte: finding.StartByte, EndByte: finding.EndByte, Rule: verdictRule, Message: finding.Message, Confidence: 1}},
			"errorFiles": map[string]string{
				// The real build of this source set reports the missing opt-in.
				p.abs(isolateAndroidFile): "This declaration needs opt-in. Its usage must be marked with '@x.Exp' or '@OptIn(x.Exp::class)'",
				// A group session lacking context must not hide a merged answer.
				p.abs(isolateMainFile): "Unresolved reference 'x'.",
			},
		})
		return resp, []string{p.abs(isolateAndroidFile), p.abs(isolateMainFile), p.abs(isolateTestFile)}, nil
	}
	opts := isolationOptions(p, merged, nil, nil)
	var verbose strings.Builder
	opts.Verbose, opts.VerboseOut = true, &verbose
	opts.SettingsSplitter = splitter
	got := RunPass(opts, isolationGoFindings(p))

	if gotID != settingsSplitRequestID || len(gotFiles) != 3 {
		t.Fatalf("splitter got request %d with files %v; want id %d and every checked file", gotID, gotFiles, settingsSplitRequestID)
	}
	want := describe([]scanner.Finding{
		p.at(isolateTestFile, "Dispatchers.IO", verdictRule, "go: test"),
		p.at(isolateAndroidFile, "Dispatchers.IO", verdictRule, "go: androidTest"),
	})
	if d := describe(got); !reflect.DeepEqual(d, want) {
		t.Fatalf("verdicts after the split\ngot:  %v\nwant: %v", d, want)
	}
	for _, line := range []string{
		"verbose: FIR settings: rechecked 3 files with their own source set's settings",
		"1 gated files now authoritative, 1 now gated on a missing opt-in, 0 on their own JVM target, 1 kept the merged verdict",
		"FIR gated: " + p.abs(isolateAndroidFile) + ": This declaration needs opt-in.",
	} {
		if !strings.Contains(verbose.String(), line) {
			t.Fatalf("verbose output lacks %q:\n%s", line, verbose.String())
		}
	}
}

func TestPassKeepsMergedVerdictWhenTheSettingsSplitFails(t *testing.T) {
	p := isolationProject(t)
	for name, splitter := range map[string]SettingsSplitter{
		"error": func(context.Context, string, []string, []byte) ([]byte, []string, error) {
			return nil, nil, errors.New("worker failed")
		},
		"one session": func(context.Context, string, []string, []byte) ([]byte, []string, error) {
			return nil, nil, nil
		},
		"no checkers": func(context.Context, string, []string, []byte) ([]byte, []string, error) {
			return []byte(`{"id":3,"rules":[]}`), []string{p.abs(isolateTestFile)}, nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			opts := isolationOptions(p, redeclarationChecker(p), nil, nil)
			opts.SettingsSplitter = splitter
			got := RunPass(opts, isolationGoFindings(p))
			want := describe([]scanner.Finding{
				p.at(isolateTestFile, "Dispatchers.IO", verdictRule, "go: test"),
				p.at(isolateAndroidFile, "Dispatchers.IO", verdictRule, "go: androidTest"),
			})
			if d := describe(got); !reflect.DeepEqual(d, want) {
				t.Fatalf("merged verdicts should stand\ngot:  %v\nwant: %v", d, want)
			}
		})
	}
}

// A group session compiled at its source set's own JVM target reports the
// errors its real build reports at that target; they replace the merged
// verdict like a missing opt-in (#1648). Other errors do not.
func TestJvmTargetErrorRecognizesTargetDependentDiagnostics(t *testing.T) {
	for reason, want := range map[string]bool{
		"Using @JvmRecord is only allowed with -jvm-target 16 or later (or -jvm-target 15 with the -Xjvm-enable-preview flag enabled).":              true,
		"Cannot inline bytecode built with JVM target 17 into bytecode that is being built with JVM target 11. Specify proper '-jvm-target' option.": true,
		"Unresolved reference 'jvmTarget'.":     false,
		"This declaration needs opt-in.":        false,
		"'when' expression must be exhaustive.": false,
	} {
		if got := jvmTargetError(reason); got != want {
			t.Errorf("jvmTargetError(%q) = %v, want %v", reason, got, want)
		}
	}
}

// A group session that gates a file the merged session answered, on an error
// at the source set's own JVM target, replaces the merged verdict: Go's
// finding stands for the gated file. The perf entry and the verbose summary
// count it (#1648).
func TestPassCountsJvmTargetGatedFiles(t *testing.T) {
	p := isolationProject(t)
	merged := NewFakeFirChecker()
	merged.Rules = []string{verdictRule}
	splitter := func(_ context.Context, _ string, _ []string, request []byte) ([]byte, []string, error) {
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(request, &req); err != nil {
			t.Fatal(err)
		}
		resp, _ := json.Marshal(map[string]any{
			"id": req.ID, "rules": []string{verdictRule}, "findings": []FirFinding{},
			"errorFiles": map[string]string{
				p.abs(isolateMainFile): "Using @JvmRecord is only allowed with -jvm-target 16 or later (or -jvm-target 15 with the -Xjvm-enable-preview flag enabled).",
			},
		})
		return resp, []string{p.abs(isolateMainFile)}, nil
	}
	var verbose strings.Builder
	opts := isolationOptions(p, merged, nil, &verbose)
	opts.SettingsSplitter = splitter
	tracker := perf.New(true)
	opts.Tracker = tracker
	got := describe(RunPass(opts, isolationGoFindings(p)))
	if want := describe([]scanner.Finding{p.at(isolateMainFile, "Dispatchers.IO", verdictRule, "go: main")}); !reflect.DeepEqual(got, want) {
		t.Fatalf("verdicts\ngot:  %v\nwant: %v", got, want)
	}
	if line := "0 now gated on a missing opt-in, 1 on their own JVM target, 0 kept the merged verdict"; !strings.Contains(verbose.String(), line) {
		t.Fatalf("verbose output lacks %q:\n%s", line, verbose.String())
	}
	var entry *perf.TimingEntry
	var find func([]perf.TimingEntry)
	find = func(entries []perf.TimingEntry) {
		for i := range entries {
			if entries[i].Name == "firSettingsSplit" {
				entry = &entries[i]
			}
			find(entries[i].Children)
		}
	}
	find(tracker.GetTimings())
	if entry == nil || entry.Metrics["targetGated"] != 1 || entry.Metrics["newlyGated"] != 0 {
		t.Fatalf("firSettingsSplit perf entry %+v; want targetGated=1", entry)
	}
}
