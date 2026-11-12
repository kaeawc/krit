package serve

import (
	"bufio"
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
				[]string{t.TempDir()}, config.NewConfig(), firchecks.NewBroker(), nil, nil, reporter, nil)
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

func TestVerboseBufferStreamsUntilSealed(t *testing.T) {
	var wire bytes.Buffer
	sink := daemon.NewStreamSink(bufio.NewWriter(&wire))
	reporter := newVerboseReporter(true)
	reporter.Verbosef("early\n")
	attachSink := reporter.Verbose.(*verboseBuffer)
	attachSink.attachSink(sink)
	reporter.Verbosef("live\n")
	sink.Seal()
	reporter.Verbosef("late\n")

	want := "{\"stream\":\"early\\n\"}\n{\"stream\":\"live\\n\"}\n"
	if wire.String() != want {
		t.Fatalf("streamed=%q, want %q", wire.String(), want)
	}
	var out bytes.Buffer
	if err := writeVerboseOutput(&out, reporter); err != nil {
		t.Fatal(err)
	}
	if out.String() != `,"verbose_output":"late\n"` {
		t.Fatalf("remainder=%q", out.String())
	}
}

func TestAnalyzeProject_StreamsVerboseOutputBeforeEnvelope(t *testing.T) {
	state := newDaemonState(t.TempDir())
	t.Cleanup(state.workspace.FlushBackgroundSaves)
	writeKotlinFile(t, state.root, "Sample.kt", "package demo\n\nclass Sample\n")
	var wire bytes.Buffer
	writer := bufio.NewWriter(&wire)
	ctx := daemon.WithStreamSink(context.Background(), daemon.NewStreamSink(writer))
	request, err := json.Marshal(daemon.AnalyzeProjectArgs{Verbose: true, StreamVerbose: true, NoFir: true, NoTypeOracle: true, NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	response, err := handleAnalyzeProject(ctx, state, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.(daemon.RawResponseWriter).WriteRawResponse(ctx, writer); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSuffix(wire.Bytes(), []byte("\n")), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("expected stream frames plus a response, got %q", wire.String())
	}
	streamed := ""
	for _, line := range lines[:len(lines)-1] {
		var frame struct {
			Stream string `json:"stream"`
		}
		if err := json.Unmarshal(line, &frame); err != nil || frame.Stream == "" {
			t.Fatalf("bad frame %q: %v", line, err)
		}
		streamed += frame.Stream
	}
	assertVerbosePhasesOnce(t, streamed)
	var got daemon.AnalyzeProjectResult
	if handled, err := daemon.ScanAnalyzeProjectResponse(lines[len(lines)-1], &got); err != nil || !handled {
		t.Fatalf("final line not a response: handled=%v err=%v", handled, err)
	}
	// Each phase line reaches the client once across the frames and the response.
	assertVerbosePhasesOnce(t, streamed+got.VerboseOutput)
}
