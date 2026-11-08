package scan

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

type verboseDaemonClient struct {
	args   daemon.AnalyzeProjectArgs
	result daemon.AnalyzeProjectResult
}

func (c *verboseDaemonClient) AnalyzeProject(args daemon.AnalyzeProjectArgs) (daemon.AnalyzeProjectResult, error) {
	c.args = args
	return c.result, nil
}

func TestRunDaemonAnalyzeForwardsVerboseOutput(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "verbose"}[verbose], func(t *testing.T) {
			message := "verbose: phase \"parse\" completed\nverbose: phase dispatch completed\n"
			client := &verboseDaemonClient{result: daemon.AnalyzeProjectResult{
				Findings: []byte(`{"rules":[],"line":[]}`), VerboseOutput: message,
			}}
			root := newRoot(t)
			f := freshScanFlags(t)
			*f.Verbose = verbose
			stdout := redirectStdout(t)
			original := os.Stderr
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				os.Stderr = original
				_ = reader.Close()
				_ = writer.Close()
			})
			os.Stderr = writer
			handled, code := runDaemonAnalyze(client, f, []string{root})
			if client.args.Verbose != verbose {
				t.Fatalf("request verbose=%v, want %v", client.args.Verbose, verbose)
			}
			_ = writer.Close()
			stderr, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if !handled || code != 0 {
				t.Fatalf("handled=%v code=%d", handled, code)
			}
			want := "info: using daemon\n"
			if verbose {
				want += message
			}
			if !strings.HasPrefix(string(stderr), want) {
				t.Fatalf("stderr=%q, want prefix %q", stderr, want)
			}
			if summary := strings.TrimPrefix(string(stderr), want); !strings.HasPrefix(summary, "info: Found 0 issue(s)") || strings.Contains(summary, "verbose:") {
				t.Fatalf("unexpected summary: %q", summary)
			}
			if got := stdout(); got != `{"rules":[],"line":[]}` || strings.Contains(got, "verbose:") {
				t.Fatalf("findings changed: %q", got)
			}
		})
	}
}

type streamingVerboseClient struct {
	verboseDaemonClient
}

func (c *streamingVerboseClient) AnalyzeProjectStream(args daemon.AnalyzeProjectArgs, onVerbose func(string)) (daemon.AnalyzeProjectResult, error) {
	c.args = args
	onVerbose("verbose: live\n")
	return c.result, nil
}

func TestRunDaemonAnalyzeStreamsVerboseBeforeSummary(t *testing.T) {
	client := &streamingVerboseClient{verboseDaemonClient{result: daemon.AnalyzeProjectResult{
		Findings: []byte(`{"rules":[],"line":[]}`), VerboseOutput: "verbose: rest\n",
	}}}
	f := freshScanFlags(t)
	*f.Verbose = true
	redirectStdout(t)
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Stderr = original; _ = reader.Close(); _ = writer.Close() })
	os.Stderr = writer
	handled, _ := runDaemonAnalyze(client, f, []string{newRoot(t)})
	_ = writer.Close()
	stderr, _ := io.ReadAll(reader)
	if !handled || !strings.HasPrefix(string(stderr), "info: using daemon\nverbose: live\nverbose: rest\ninfo: Found") {
		t.Fatalf("handled=%v stderr=%q", handled, stderr)
	}
}
