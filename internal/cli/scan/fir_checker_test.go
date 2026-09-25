package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestRunFIRCheckerPassDisabledIsNoOp(t *testing.T) {
	base := []scanner.Finding{{Rule: "X"}}
	got := runFIRCheckerPass(firCheckerOpts{Enabled: false}, base)
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("got %v; want %v (base unchanged when disabled)", got, base)
	}
}

// Every active rule ID reaches the jar: discovery there decides which rules
// have a checker, and the response's advertised rules scope the verdict.
func TestRunFIRCheckerPassUnknownRuleInvokesChecker(t *testing.T) {
	checker := firchecks.NewFakeFirChecker()
	checker.Rules = []string{}
	base := []scanner.Finding{{Rule: "X"}}
	got := runFIRCheckerPass(firCheckerOpts{
		Enabled:     true,
		Checker:     checker,
		ActiveRules: []*api.Rule{{ID: "NotAFirRule"}},
		Tracker:     perf.New(false),
	}, base)
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("got %v; want %v (base unchanged when no FIR rules active)", got, base)
	}
	if len(checker.Called) != 1 {
		t.Fatalf("checker.Check should receive unknown rule ID; got %d invocations", len(checker.Called))
	}
}

// Active rules' configured options reach the FIR checker as ruleConfigs;
// krit-interpreted keys (active, excludes) and option-less rules do not.
func TestRunFIRCheckerPassSendsActiveRuleOptions(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("coroutines", "InjectDispatcher", "active", true)
	cfg.Set("coroutines", "InjectDispatcher", "excludes", []interface{}{"**/gen/**"})
	cfg.Set("coroutines", "InjectDispatcher", "dispatcherNames", []interface{}{"IO"})
	cfg.Set("style", "MagicNumber", "active", true)
	cfg.Set("style", "Inactive", "threshold", 9) // not an active rule
	checker := firchecks.NewFakeFirChecker()
	runFIRCheckerPass(firCheckerOpts{
		Enabled: true,
		Checker: checker,
		Config:  cfg,
		ActiveRules: []*api.Rule{
			{ID: "InjectDispatcher", Category: "coroutines"},
			{ID: "MagicNumber", Category: "style"},
		},
		Tracker: perf.New(false),
	}, nil)
	if len(checker.CalledRuleConfigs) != 1 {
		t.Fatalf("expected one Check call, got %d", len(checker.CalledRuleConfigs))
	}
	want := firchecks.RuleConfigs{"InjectDispatcher": {"dispatcherNames": []interface{}{"IO"}}}
	if got := checker.CalledRuleConfigs[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ruleConfigs = %#v; want %#v", got, want)
	}
}

// The --fir checker compiles against the oracle's JVM-scoped source roots
// and the configured classpath; without them cross-file and library
// references stay unresolved and every file is gated.
func TestNewFIRCheckerUsesOracleCompileContext(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"src/main/kotlin", "src/jsMain/kotlin"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	lib := filepath.Join(root, "lib.jar")
	cfg := config.NewConfigFromData(map[string]interface{}{
		"oracle": map[string]interface{}{"classpath": []interface{}{lib}},
	})
	t.Setenv("CLASSPATH", "")

	checker := NewFIRChecker([]string{root}, cfg, false, false)

	if want := []string{filepath.Join(root, "src/main/kotlin")}; !reflect.DeepEqual(checker.SourceDirs, want) {
		t.Fatalf("SourceDirs = %v, want %v (JVM roots only)", checker.SourceDirs, want)
	}
	if want := []string{lib}; !reflect.DeepEqual(checker.Classpath, want) {
		t.Fatalf("Classpath = %v, want %v", checker.Classpath, want)
	}
}
