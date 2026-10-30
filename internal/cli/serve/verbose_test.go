package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/pipeline"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestAnalyzeProject_VerboseOutputIsRequestScoped(t *testing.T) {
	for _, strict := range []bool{false, true} {
		t.Run(map[bool]string{false: "streaming", true: "strict-verify"}[strict], func(t *testing.T) {
			state := newDaemonState(t.TempDir())
			t.Cleanup(state.workspace.FlushBackgroundSaves)
			state.strictVerify = strict
			writeKotlinFile(t, state.root, "Sample.kt", "package demo\n\nclass Sample\n")
			var got daemon.AnalyzeProjectResult
			for _, verbose := range []bool{true, false, true, false} {
				args := daemon.AnalyzeProjectArgs{Verbose: verbose, NoFir: true, NoTypeOracle: true, NoCache: true}
				request, err := json.Marshal(args)
				if err != nil {
					t.Fatal(err)
				}
				response, err := handleAnalyzeProject(context.Background(), state, request)
				if err != nil {
					t.Fatal(err)
				}
				var wire bytes.Buffer
				if err := response.(daemon.RawResponseWriter).WriteRawResponse(context.Background(), &wire); err != nil {
					t.Fatal(err)
				}
				handled, err := daemon.ScanAnalyzeProjectResponse(wire.Bytes(), &got)
				if err != nil {
					t.Fatal(err)
				}
				if !handled {
					t.Fatalf("response missed fast decoder: %s", &wire)
				}
				if verbose && !strings.Contains(got.VerboseOutput, "verbose:") {
					t.Fatalf("missing phase diagnostics: %q", got.VerboseOutput)
				}
				if verbose {
					assertVerbosePhasesOnce(t, got.VerboseOutput)
				}
				if !verbose && got.VerboseOutput != "" {
					t.Fatalf("diagnostics leaked into non-verbose request: %q", got.VerboseOutput)
				}
				if !json.Valid(got.Findings) || bytes.Contains(got.Findings, []byte("verbose:")) {
					t.Fatalf("invalid or contaminated findings: %s", got.Findings)
				}
			}
		})
	}
}

func assertVerbosePhasesOnce(t *testing.T, output string) {
	t.Helper()
	for _, phase := range []string{"verbose: Parsed ", "verbose: Cross-file analysis in "} {
		if count := strings.Count(output, phase); count != 1 {
			t.Errorf("phase %q appeared %d times, want once: %s", phase, count, output)
		}
	}
}

func TestVerboseReporterForwardsWarnings(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		reporter := newVerboseReporter(enabled)
		reporter.Verbosef("verbose: phase\n")
		reporter.Warnf("warning: %s\n", "test warning")
		var output bytes.Buffer
		if err := writeVerboseOutput(&output, reporter); err != nil {
			t.Fatal(err)
		}
		if !enabled {
			if output.Len() != 0 {
				t.Fatalf("disabled reporter emitted diagnostics: %s", &output)
			}
			continue
		}
		var got daemon.AnalyzeProjectResult
		if err := json.Unmarshal([]byte(`{"findings":[],"stats":{}`+output.String()+`}`), &got); err != nil {
			t.Fatal(err)
		}
		if got.VerboseOutput != "verbose: phase\nwarning: test warning\n" {
			t.Fatalf("diagnostics = %q", got.VerboseOutput)
		}
	}
}

func TestFirStartFindingsPassCapturesVerboseSummary(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "verbose"}[verbose], func(t *testing.T) {
			buffer := &verboseBuffer{}
			reporter := &diag.Reporter{Verbose: buffer}
			hook := firStartFindingsPass(daemon.AnalyzeProjectArgs{Fir: true, Verbose: verbose},
				[]string{t.TempDir()}, config.NewConfig(), firchecks.NewBroker(), nil, nil, reporter)
			pending, err := hook(context.Background(), pipeline.ParseResult{
				ActiveRules: []*api.Rule{{ID: "InjectDispatcher"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer pending.Cancel()
			if _, err := pending.Finish(nil); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := writeVerboseOutput(&output, reporter); err != nil {
				t.Fatal(err)
			}
			if verbose && !strings.Contains(output.String(), "verbose: FIR checker in") {
				t.Fatalf("FIR summary not captured: %s", &output)
			}
			if !verbose && output.Len() != 0 {
				t.Fatalf("non-verbose FIR emitted diagnostics: %s", &output)
			}
		})
	}
}

func TestWriteVerboseOutputEscapesAndOmitsEmpty(t *testing.T) {
	var output bytes.Buffer
	reporter := &diag.Reporter{Verbose: &verboseBuffer{}}
	if err := writeVerboseOutput(&output, reporter); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatal("empty output added a wire field")
	}
	message := "verbose: phase \"parse\"\nverbose: path \\source\n"
	reporter.Verbosef("%s", message)
	if err := writeVerboseOutput(&output, reporter); err != nil {
		t.Fatal(err)
	}
	var got daemon.AnalyzeProjectResult
	if err := json.Unmarshal([]byte(`{"findings":[],"stats":{}`+output.String()+`}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.VerboseOutput != message {
		t.Fatalf("output=%q, want %q", got.VerboseOutput, message)
	}
}
