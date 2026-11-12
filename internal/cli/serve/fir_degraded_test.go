package serve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules/api"
)

// A checker failure in a delegated scan must reach the request's degraded
// state, which the response carries back to the CLI.
func TestFirStartFindingsPassRecordsCheckerFailure(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "A.kt")
	if err := os.WriteFile(src, []byte("fun answer() = 42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", filepath.Join(t.TempDir(), "missing.jar"))
	var recorded *output.FirDegraded
	hook := firStartFindingsPass(daemon.AnalyzeProjectArgs{Fir: true, NoFirDaemon: true, NoCache: true},
		[]string{root}, config.NewConfig(), firchecks.NewBroker(), nil, nil, &diag.Reporter{},
		func(d *output.FirDegraded) { recorded = d })
	pending, err := hook(context.Background(), pipeline.ParseResult{
		ActiveRules: []*api.Rule{{ID: "InjectDispatcher"}, {ID: "NotAFirRule"}},
		KotlinPaths: []string{src},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pending.Cancel()
	if _, err := pending.Finish(nil); err != nil {
		t.Fatal(err)
	}
	if recorded == nil || !strings.HasPrefix(recorded.Reason, "checker failed:") {
		t.Fatalf("degraded state not recorded: %+v", recorded)
	}
	if len(recorded.FallbackRules) != 1 || recorded.FallbackRules[0] != "InjectDispatcher" {
		t.Fatalf("FallbackRules = %v", recorded.FallbackRules)
	}
}
