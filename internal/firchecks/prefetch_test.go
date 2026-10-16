package firchecks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

type prefetchProject struct {
	src, jar string
	a, b     string
	checker  *ProductionFirChecker
	opts     PassOptions
}

func newPrefetchProject(t *testing.T) *prefetchProject {
	t.Helper()
	root := t.TempDir()
	p := &prefetchProject{src: filepath.Join(root, "src", "main", "kotlin"), jar: filepath.Join(root, "krit-fir.jar")}
	p.a, p.b = filepath.Join(p.src, "A.kt"), filepath.Join(p.src, "B.kt")
	for path, text := range map[string]string{p.a: "fun a() = run(Dispatchers.IO)\n", p.b: "fun b() {}\n", p.jar: "jar"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p.checker = &ProductionFirChecker{JarPath: p.jar, SourceDirs: []string{p.src}, Classpath: []string{p.jar}, NoCache: true}
	p.opts = PassOptions{Enabled: true, Checker: p.checker, ActiveRules: []*api.Rule{{ID: verdictRule}},
		Config: config.NewConfig(), KotlinPaths: []string{p.a, p.b}, SourceDirs: p.checker.SourceDirs, Classpath: p.checker.Classpath}
	return p
}

// response is a krit-fir check response to the prefetch request.
func (p *prefetchProject) response(rules ...string) []byte {
	data, _ := json.Marshal(map[string]any{
		"id": prefetchRequestID, "succeeded": 1, "skipped": 0, "rules": rules,
		"findings":   []map[string]any{{"path": p.a, "line": 1, "col": 16, "rule": verdictRule, "severity": "warning", "message": "inject it"}},
		"crashed":    map[string]string{},
		"errorFiles": map[string]string{p.b: "Unresolved reference 'x'."},
	})
	return data
}

// prefetched returns a checker whose prefetch holds the oracle's response.
func (p *prefetchProject) prefetched(t *testing.T, rules ...string) *Prefetch {
	t.Helper()
	prefetch := NewPrefetch(p.opts)
	if prefetch == nil {
		t.Fatal("NewPrefetch = nil")
	}
	p.checker.Prefetch = prefetch
	rider := prefetch.Rider(p.jar, []string{p.src}, []string{p.jar}, "")
	if rider == nil {
		t.Fatal("Rider = nil for the checker's own compile context")
	}
	_, deliver := rider.Prepare()
	deliver(p.response(rules...))
	return prefetch
}

// stubMisses fails the test unless want, and counts the checks it runs.
func stubMisses(t *testing.T, want bool) *int {
	t.Helper()
	old := runMissesForCache
	t.Cleanup(func() { runMissesForCache = old })
	calls := 0
	runMissesForCache = func(_ string, files, _, _, _ []string, _ RuleConfigs, _ FileFacts, _, _ bool, _ ...string) (*CheckResponse, error) {
		calls++
		if !want {
			t.Fatalf("the pass compiled on its own for %v", files)
		}
		return &CheckResponse{}, nil
	}
	return &calls
}

func (p *prefetchProject) check(t *testing.T, files []string, rules []string) *Result {
	t.Helper()
	plan, ok := planPass(p.opts)
	if !ok {
		t.Fatal("planPass: no pass")
	}
	res, err := p.checker.Check(files, p.checker.SourceDirs, p.checker.Classpath, rules, plan.configs, plan.facts)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestPrefetchRequestIsThePassRequest(t *testing.T) {
	p := newPrefetchProject(t)
	request, _ := NewPrefetch(p.opts).Rider(p.jar, []string{p.src}, []string{p.jar}, "").Prepare()
	var req firDaemonRequest
	if err := json.Unmarshal(request, &req); err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, ref := range req.Files {
		files = append(files, ref.Path)
	}
	if req.Command != "check" || !slices.Equal(files, []string{p.a, p.b}) || !slices.Equal(req.Rules, []string{verdictRule}) ||
		!slices.Equal(req.SourceDirs, []string{p.src}) || !slices.Equal(req.Classpath, []string{p.jar}) {
		t.Fatalf("request = %+v", req)
	}
}

func TestPrefetchedResponseServesThePass(t *testing.T) {
	p := newPrefetchProject(t)
	p.prefetched(t, verdictRule)
	stubMisses(t, false)

	res := p.check(t, []string{p.a, p.b}, []string{verdictRule})
	if len(res.Findings) != 1 || res.Findings[0].File != p.a || res.Findings[0].Rule != verdictRule {
		t.Fatalf("findings = %+v", res.Findings)
	}
	if _, gated := res.ErrorFiles[p.b]; !gated || !slices.Equal(res.Rules, []string{verdictRule}) {
		t.Fatalf("errorFiles = %v rules = %v", res.ErrorFiles, res.Rules)
	}

	// A subset of the prefetched files gets only its own verdicts.
	res = p.check(t, []string{p.b}, []string{verdictRule})
	if len(res.Findings) != 0 || len(res.ErrorFiles) != 1 {
		t.Fatalf("subset result = %+v", res)
	}
}

func TestPrefetchIsNotReusedForAnotherCompile(t *testing.T) {
	cases := map[string]func(p *prefetchProject) []string{
		"rules": func(*prefetchProject) []string { return nil },
		"edited source": func(p *prefetchProject) []string {
			if err := os.WriteFile(p.b, []byte("fun b() = 1\n"), 0o644); err != nil {
				panic(err)
			}
			return []string{p.a, p.b}
		},
		"added source": func(p *prefetchProject) []string {
			if err := os.WriteFile(filepath.Join(p.src, "C.kt"), []byte("fun c() {}\n"), 0o644); err != nil {
				panic(err)
			}
			return []string{p.a, p.b}
		},
		"unplanned file": func(p *prefetchProject) []string {
			extra := filepath.Join(filepath.Dir(p.src), "Extra.kt")
			if err := os.WriteFile(extra, []byte("fun e() {}\n"), 0o644); err != nil {
				panic(err)
			}
			return []string{p.a, extra}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			p := newPrefetchProject(t)
			p.prefetched(t, verdictRule)
			calls := stubMisses(t, true)
			files := change(p)
			rules := []string{verdictRule}
			if files == nil {
				files, rules = []string{p.a, p.b}, []string{verdictRule, "OtherRule"}
			}
			p.check(t, files, rules)
			if *calls != 1 {
				t.Fatalf("own checks = %d, want 1", *calls)
			}
		})
	}
}

func TestPrefetchRidesOnlyTheCheckersCompileContext(t *testing.T) {
	p := newPrefetchProject(t)
	prefetch := NewPrefetch(p.opts)
	for name, rider := range map[string]any{
		"jar":       prefetch.Rider(p.jar+".other", []string{p.src}, []string{p.jar}, ""),
		"sources":   prefetch.Rider(p.jar, []string{p.src, filepath.Dir(p.src)}, []string{p.jar}, ""),
		"classpath": prefetch.Rider(p.jar, []string{p.src}, nil, ""),
		"target":    prefetch.Rider(p.jar, []string{p.src}, []string{p.jar}, "17"),
	} {
		if fmt.Sprint(rider) != "<nil>" {
			t.Fatalf("%s: rider for another compile context", name)
		}
	}
	if NewPrefetch(PassOptions{Enabled: true, Checker: NewFakeFirChecker(), ActiveRules: p.opts.ActiveRules, KotlinPaths: p.opts.KotlinPaths}) != nil {
		t.Fatal("prefetch for a non-production checker")
	}
	disabled := p.opts
	disabled.Enabled = false
	if NewPrefetch(disabled) != nil {
		t.Fatal("prefetch for a disabled pass")
	}
	noRules := p.opts
	noRules.ActiveRules = nil
	if request, deliver := NewPrefetch(noRules).Rider(p.jar, []string{p.src}, []string{p.jar}, "").Prepare(); request != nil || deliver != nil {
		t.Fatal("prepared a request for a pass with no active rules")
	}
}

// A JVM whose AOT cache registers no checkers answers with no rules; the
// pass's own check carries the recovery for that.
func TestPrefetchIgnoresAResponseWithoutCheckers(t *testing.T) {
	p := newPrefetchProject(t)
	p.prefetched(t)
	calls := stubMisses(t, true)
	p.check(t, []string{p.a, p.b}, []string{verdictRule})
	if *calls != 1 {
		t.Fatalf("own checks = %d, want 1", *calls)
	}
}
