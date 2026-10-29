package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const brokerWait = 5 * time.Second

// checkingDaemon is a resident daemon that can carry the pass's check: it
// counts the module compiles it runs, one per request, as krit-fir does.
type checkingDaemon struct {
	stubDaemon
	matches        bool
	compiles       atomic.Int32
	checkResponse  []byte
	withCheckCalls atomic.Int32
}

func (c *checkingDaemon) AnalyzeAll() (*oracle.Data, error) {
	c.compiles.Add(1)
	return c.stubDaemon.AnalyzeAll()
}

func (c *checkingDaemon) AnalyzeAllWithCheck(rider *oracle.CheckRider) (*oracle.Data, error) {
	c.withCheckCalls.Add(1)
	c.compiles.Add(1) // one compile answers both
	if rider.Sent != nil {
		rider.Sent()
	}
	if _, deliver := rider.Prepare(); deliver != nil && c.checkResponse != nil {
		deliver(c.checkResponse)
	}
	return &oracle.Data{Version: 1, Files: map[string]*oracle.File{}}, nil
}

func (c *checkingDaemon) MatchesRepoSettings(string, []string, gradlemodel.KotlinSettings, ...string) bool {
	return c.matches
}

// armedBroker returns a broker holding a real pass's check, and that pass's
// check response for the oracle to deliver.
func armedBroker(t *testing.T) (*firchecks.Broker, []byte) {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "src", "main", "kotlin")
	jar := filepath.Join(root, "krit-fir.jar")
	a := filepath.Join(src, "A.kt")
	for path, text := range map[string]string{a: "fun a() = run(Dispatchers.IO)\n", jar: "jar"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	checker := &firchecks.ProductionFirChecker{JarPath: jar, SourceDirs: []string{src}, Classpath: []string{jar}, NoCache: true}
	prefetch := firchecks.NewPrefetch(firchecks.PassOptions{
		Enabled: true, Checker: checker, ActiveRules: []*api.Rule{{ID: "InjectDispatcher"}},
		Config: config.NewConfig(), KotlinPaths: []string{a}, SourceDirs: checker.SourceDirs, Classpath: checker.Classpath,
	})
	if prefetch == nil {
		t.Fatal("no prefetch")
	}
	broker := firchecks.NewBrokerWithTimeouts(time.Minute, time.Minute)
	broker.Arm(prefetch, func(string, []string, gradlemodel.KotlinSettings, []string) bool { return true })
	response, _ := json.Marshal(map[string]any{
		"id": 1, "succeeded": 1, "skipped": 0, "rules": []string{"InjectDispatcher"},
		"findings": []any{}, "crashed": map[string]string{}, "errorFiles": map[string]string{},
	})
	return broker, response
}

func waitsFor(t *testing.T, broker *firchecks.Broker) <-chan string {
	t.Helper()
	out := make(chan string, 1)
	go func() { out <- broker.Wait() }()
	return out
}

func mustResolve(t *testing.T, out <-chan string) string {
	t.Helper()
	select {
	case reason := <-out:
		return reason
	case <-time.After(brokerWait):
		t.Fatal("the pass is still waiting on the oracle: the broker was never resolved")
		return ""
	}
}

// The oracle compiles once and the pass's check comes back from that compile.
func TestAnalyzeAllCarryingCheckCompilesOnce(t *testing.T) {
	broker, response := armedBroker(t)
	d := &checkingDaemon{matches: true, checkResponse: response}
	waiting := waitsFor(t, broker)

	if data, err := analyzeAllCarryingCheck(IndexInput{FIRCheckBroker: broker}, d); err != nil || data == nil {
		t.Fatalf("data=%v err=%v", data, err)
	}
	broker.Done("oracle daemon analysis finished")
	mustResolve(t, waiting)

	if d.compiles.Load() != 1 || d.withCheckCalls.Load() != 1 || d.allCalls != 0 {
		t.Fatalf("compiles=%d withCheck=%d plain=%d, want one combined compile", d.compiles.Load(), d.withCheckCalls.Load(), d.allCalls)
	}
	// The response arrived; "shared" is only claimed once the pass used it.
	if broker.Outcome() != "delivered, not used" {
		t.Fatalf("outcome = %q", broker.Outcome())
	}
}

// Whatever keeps the check from riding, the oracle still gets its plain
// analysis and the pass is released to compile on its own.
func TestAnalyzeAllCarryingCheckDeclines(t *testing.T) {
	t.Run("daemon compiles in another context", func(t *testing.T) {
		broker, response := armedBroker(t)
		d := &checkingDaemon{matches: false, checkResponse: response}
		waiting := waitsFor(t, broker)
		if _, err := analyzeAllCarryingCheck(IndexInput{FIRCheckBroker: broker}, d); err != nil {
			t.Fatal(err)
		}
		if reason := mustResolve(t, waiting); reason != "oracle daemon compiles in another context" {
			t.Fatalf("reason = %q", reason)
		}
		if d.allCalls != 1 || d.withCheckCalls.Load() != 0 {
			t.Fatalf("plain=%d withCheck=%d", d.allCalls, d.withCheckCalls.Load())
		}
	})
	t.Run("daemon cannot carry a check", func(t *testing.T) {
		broker, _ := armedBroker(t)
		d := &stubDaemon{}
		waiting := waitsFor(t, broker)
		if _, err := analyzeAllCarryingCheck(IndexInput{FIRCheckBroker: broker}, d); err != nil {
			t.Fatal(err)
		}
		mustResolve(t, waiting)
		if d.allCalls != 1 {
			t.Fatalf("plain=%d", d.allCalls)
		}
	})
	t.Run("no broker", func(t *testing.T) {
		d := &checkingDaemon{matches: true}
		if _, err := analyzeAllCarryingCheck(IndexInput{}, d); err != nil {
			t.Fatal(err)
		}
		if d.allCalls != 1 || d.withCheckCalls.Load() != 0 {
			t.Fatalf("plain=%d withCheck=%d", d.allCalls, d.withCheckCalls.Load())
		}
	})
}

// A partial reanalyze compiles for a few files only, so the pass compiles the
// module on its own and the broker says so at once.
func TestDaemonAnalyzePartialReanalyzeDeclinesTheCheck(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	scanPaths := []string{root}
	typesPath := oracle.CachePath(scanPaths)
	if err := os.MkdirAll(filepath.Dir(typesPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := oracle.WriteTypesJSON(typesPath, &oracle.Data{Version: 1, Files: map[string]*oracle.File{}}); err != nil {
		t.Fatal(err)
	}
	in := IndexInput{OracleBackend: oracle.BackendFIR, StaleOraclePaths: []string{filepath.Join(root, "A.kt")}}
	scope, ok := oracleTypesScope(in, scanPaths)
	if !ok {
		t.Fatal("no scope")
	}
	recordTypesScope(typesPath, scope, true, scanPaths, t.Logf)

	broker, response := armedBroker(t)
	in.FIRCheckBroker = broker
	d := &checkingDaemon{matches: true, checkResponse: response}
	waiting := waitsFor(t, broker)
	if got := daemonAnalyze(in, d, scanPaths, perf.New(false)); got == nil {
		t.Fatal("no data")
	}
	if reason := mustResolve(t, waiting); reason != "partial oracle reanalyze" {
		t.Fatalf("reason = %q", reason)
	}
	if d.filesCalls != 1 || d.withCheckCalls.Load() != 0 {
		t.Fatalf("files=%d withCheck=%d", d.filesCalls, d.withCheckCalls.Load())
	}
}

// The index phase resolves the broker when it does not run the oracle at all,
// so a pass joined after it never waits on a decision that was skipped.
func TestIndexPhaseResolvesTheBrokerWhenTheOracleIsSkipped(t *testing.T) {
	broker := firchecks.NewBrokerWithTimeouts(time.Hour, time.Hour)
	waiting := waitsFor(t, broker)
	if _, err := (IndexPhase{SkipAndroid: true, SkipModules: true, SkipResolverIndex: true}).Run(context.Background(), IndexInput{
		ParseResult:    ParseResult{Paths: []string{t.TempDir()}},
		BaseResolver:   nil,
		FIRCheckBroker: broker,
	}); err != nil {
		t.Fatal(err)
	}
	if reason := mustResolve(t, waiting); reason == "" {
		t.Fatal("resolved without a reason")
	}
}

func TestIndexPhaseResolvesTheBrokerOnTheNonDaemonOraclePath(t *testing.T) {
	dir := t.TempDir()
	oraclePath := filepath.Join(dir, "types.json")
	if err := os.WriteFile(oraclePath, []byte(`{"version":1,"files":{},"dependencies":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	broker := firchecks.NewBrokerWithTimeouts(time.Hour, time.Hour)
	waiting := waitsFor(t, broker)
	if _, err := (IndexPhase{SkipAndroid: true, SkipModules: true, SkipResolverIndex: true}).Run(context.Background(), IndexInput{
		ParseResult: ParseResult{
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{{ID: "OracleRule", Description: "d", Needs: api.NeedsOracle, Check: func(*api.Context) {}}},
		},
		OracleEnabled:  true,
		InputTypesPath: oraclePath,
		FIRCheckBroker: broker,
	}); err != nil {
		t.Fatal(err)
	}
	if mustResolve(t, waiting) == "" {
		t.Fatal("resolved without a reason")
	}
}

type waitingPass struct{}

func (w *waitingPass) Finish(f []scanner.Finding) ([]scanner.Finding, error) { return f, nil }
func (w *waitingPass) Cancel()                                               {}

// The whole analysis resolves the broker once the index phase has returned,
// and on the early returns that never reach it, so the pass joined at the end
// cannot deadlock behind an oracle that never answered.
func TestRunProjectAnalysisResolvesTheBroker(t *testing.T) {
	run := func(t *testing.T, ctx context.Context, withFiles bool) (*firchecks.Broker, <-chan string, error) {
		root := t.TempDir()
		if withFiles {
			if err := os.WriteFile(filepath.Join(root, "A.kt"), []byte("fun a() {}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		broker := firchecks.NewBrokerWithTimeouts(time.Hour, time.Hour)
		var waiting <-chan string
		_, err := RunProjectAnalysis(ctx, ProjectInput{
			Args: ProjectArgs{Config: config.NewConfig(), Paths: []string{root}, ActiveRules: []*api.Rule{findV2RuleForTest(t, "AbsentOrWrongFileLicense")}, Format: "json", Version: "test"},
			Host: ProjectHostState{
				FIRCheckBroker: broker,
				StartFindingsPass: func(context.Context, ParseResult) (PendingFindingsPass, error) {
					// The pass starts right after parsing and waits on the broker.
					waiting = waitsFor(t, broker)
					return &waitingPass{}, nil
				},
			},
		})
		return broker, waiting, err
	}

	t.Run("oracle not used", func(t *testing.T) {
		broker, waiting, err := run(t, context.Background(), true)
		if err != nil {
			t.Fatal(err)
		}
		if waiting == nil {
			t.Fatal("the pass never started")
		}
		mustResolve(t, waiting)
		if !broker.Resolved() {
			t.Fatal("broker unresolved")
		}
	})
	t.Run("error before the index phase", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		broker, waiting, err := run(t, ctx, true)
		if err == nil {
			t.Log("canceled analysis returned no error")
		}
		if waiting != nil {
			mustResolve(t, waiting)
		}
		// Whether or not the pass ever started, nobody is left waiting.
		if waiting != nil && !broker.Resolved() {
			t.Fatal("broker unresolved after an early return")
		}
	})
}
