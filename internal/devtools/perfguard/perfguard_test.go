package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const reportJSON = `{
  "durationMs": 1000,
  "perfTiming": [
    {"name": "parse", "durationMs": 100},
    {"name": "indexPhaseRun", "durationMs": 40, "children": [{"name": "typeIndex", "durationMs": 30}]},
    {"name": "indexPhaseRun", "durationMs": 20, "children": [{"name": "typeIndex", "durationMs": 10}]},
    {"name": "androidPhase", "durationMs": 600, "children": [
      {"name": "manifestAnalysis", "durationMs": 590, "children": [{"name": "manifestSourceRuleChecks", "durationMs": 580}]}
    ]}
  ],
  "perfRuleStats": [
    {"rule": "UnusedVariable", "durationMs": 120, "sharePct": 40},
    {"rule": "MagicNumber", "durationMs": 30, "sharePct": 10}
  ]
}`

func TestParseSampleFlattensRepeatedPhases(t *testing.T) {
	s, err := parseSample([]byte(reportJSON))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{
		"parse":                         100,
		"indexPhaseRun":                 40,
		"indexPhaseRun/typeIndex":       30,
		"indexPhaseRun#2":               20,
		"indexPhaseRun#2/typeIndex":     10,
		"androidPhase":                  600,
		"androidPhase/manifestAnalysis": 590,
		"androidPhase/manifestAnalysis/manifestSourceRuleChecks": 580,
	}
	for k, v := range want {
		if s.Phases[k] != v {
			t.Errorf("phase %s = %v, want %v", k, s.Phases[k], v)
		}
	}
	if len(s.Phases) != len(want) {
		t.Errorf("got %d phases, want %d: %v", len(s.Phases), len(want), s.Phases)
	}
	if !s.Leaves["androidPhase/manifestAnalysis/manifestSourceRuleChecks"] || s.Leaves["androidPhase"] {
		t.Errorf("leaves wrong: %v", s.Leaves)
	}
	if s.TotalMs != 1000 || s.Rules["UnusedVariable"] != 120 || s.Shares["UnusedVariable"] != 40 {
		t.Errorf("total/rules wrong: %+v", s)
	}
}

func TestParseSampleRequiresPerfTiming(t *testing.T) {
	if _, err := parseSample([]byte(`{"durationMs": 5}`)); err == nil || !strings.Contains(err.Error(), "--perf") {
		t.Fatalf("want a --perf hint, got %v", err)
	}
}

func TestSummarizeTakesMediansAndCountsMissingAsZero(t *testing.T) {
	mk := func(total, phase float64, withRule bool) sample {
		s := sample{TotalMs: total, Phases: map[string]float64{"p": phase}, Leaves: map[string]bool{"p": true}, Rules: map[string]float64{}, Shares: map[string]float64{}}
		if withRule {
			s.Rules["R"] = 50
		}
		return s
	}
	sum := summarize(series{mk(100, 10, true), mk(300, 30, true), mk(200, 1000, false)})
	if sum.TotalMs != 200 || sum.Phases["p"] != 30 {
		t.Errorf("medians: total %v phase %v", sum.TotalMs, sum.Phases["p"])
	}
	// R ran in two of three samples: median of {50, 50, 0} is 50.
	if sum.Rules["R"] != 50 {
		t.Errorf("rule median = %v, want 50", sum.Rules["R"])
	}
}

func summ(total float64, phases, rules, shares map[string]float64) summary {
	leaves := map[string]bool{}
	for k := range phases {
		leaves[k] = true
	}
	if rules == nil {
		rules = map[string]float64{}
	}
	if shares == nil {
		shares = map[string]float64{}
	}
	return summary{TotalMs: total, Phases: phases, Leaves: leaves, Rules: rules, Shares: shares}
}

func kinds(regs []regression) []string {
	var out []string
	for _, r := range regs {
		out = append(out, r.Kind+":"+r.Key)
	}
	sort.Strings(out)
	return out
}

func TestCompareGates(t *testing.T) {
	base := summ(2000,
		map[string]float64{"parse": 400, "tiny": 10, "manifest": 800},
		map[string]float64{"UnusedVariable": 200, "Hot": 100},
		map[string]float64{"UnusedVariable": 20, "Hot": 10})
	tests := []struct {
		name string
		head summary
		want []string
	}{
		{"unchanged", base, nil},
		{"noise under the floors",
			summ(2250, map[string]float64{"parse": 450, "tiny": 100, "manifest": 900},
				map[string]float64{"UnusedVariable": 250, "Hot": 120}, map[string]float64{"UnusedVariable": 20, "Hot": 10}),
			nil},
		{"total and phase regress",
			summ(2600, map[string]float64{"parse": 400, "tiny": 10, "manifest": 1400}, base.Rules, base.Shares),
			[]string{"phase:manifest", "total:durationMs"}},
		{"percent without the absolute floor is noise",
			summ(2000, map[string]float64{"parse": 400, "tiny": 150, "manifest": 800}, base.Rules, base.Shares),
			nil},
		{"rule regresses and newly dominates",
			summ(2100, base.Phases,
				map[string]float64{"UnusedVariable": 200, "Hot": 400}, map[string]float64{"UnusedVariable": 18, "Hot": 35}),
			[]string{"rule-share:Hot", "rule:Hot"}},
		{"new rule over budget",
			summ(2100, base.Phases,
				map[string]float64{"UnusedVariable": 200, "Hot": 100, "NewPort": 300, "CheapPort": 20}, base.Shares),
			[]string{"new-rule:NewPort"}},
		{"new phase over the floor",
			summ(2100, map[string]float64{"parse": 400, "tiny": 10, "manifest": 800, "newPhase": 250}, base.Rules, base.Shares),
			[]string{"phase:newPhase"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kinds(compareSummaries("c", base, tt.head, defaultGates))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteReportListsRegressionsAndNewRules(t *testing.T) {
	base := summ(1000, map[string]float64{"parse": 100}, map[string]float64{"A": 10}, map[string]float64{"A": 100})
	head := summ(1500, map[string]float64{"parse": 600}, map[string]float64{"A": 10, "B": 5}, map[string]float64{"A": 66, "B": 33})
	regs := compareSummaries("synthetic", base, head, defaultGates)
	var buf bytes.Buffer
	writeReport(&buf, []corpusResult{{Name: "synthetic", Base: base, Head: head, Rounds: 5}}, regs, defaultGates, "T")
	out := buf.String()
	for _, want := range []string{"## T", "2 regression(s)", "| synthetic | phase | `parse` |", "Rules new in head: `B` 5.0ms", "median of 5 interleaved runs"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
}

func TestCheckScaling(t *testing.T) {
	small := summ(1000, map[string]float64{"linear": 200, "quadratic": 300, "tiny": 1, "total": 1000}, map[string]float64{"R": 100}, nil)
	large := summ(14000, map[string]float64{"linear": 1250, "quadratic": 10800, "tiny": 400, "total": 14000}, map[string]float64{"R": 650}, nil)
	g := scaleGates{Factor: 6, Slack: 2, MinMs: 500}

	results, stale := checkScaling(small, large, g, knownSuperlinear{})
	var super []string
	for _, r := range results {
		if r.Super {
			super = append(super, r.Kind+":"+r.Key)
		}
	}
	// linear grew 6.25x and R 6.5x: within 12x. tiny stays under MinMs.
	// total is perfTiming's whole-run node and is never judged.
	if strings.Join(super, ",") != "phase:quadratic" {
		t.Errorf("super-linear = %v, want [phase:quadratic]", super)
	}
	if len(stale) != 0 {
		t.Errorf("stale = %v", stale)
	}
	var buf bytes.Buffer
	if !writeScaleReport(&buf, "s", "l", results, stale, g, knownSuperlinear{}) {
		t.Errorf("an unlisted super-linear phase must fail:\n%s", buf.String())
	}

	known := knownSuperlinear{"phase:quadratic": "#1658"}
	results, stale = checkScaling(small, large, g, known)
	buf.Reset()
	if writeScaleReport(&buf, "s", "l", results, stale, g, known) {
		t.Errorf("a listed super-linear phase must not fail:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "#1658") {
		t.Errorf("report should name the known issue:\n%s", buf.String())
	}

	// After the fix the phase scales linearly: the stale entry fails the
	// guard until its line is deleted.
	fixed := summ(8000, map[string]float64{"linear": 1250, "quadratic": 1900, "tiny": 400}, map[string]float64{"R": 650}, nil)
	results, stale = checkScaling(small, fixed, g, known)
	buf.Reset()
	if !writeScaleReport(&buf, "s", "l", results, stale, g, known) || len(stale) != 1 || !strings.Contains(buf.String(), "delete these lines") {
		t.Errorf("a stale allowlist entry must fail with a delete hint (stale=%v):\n%s", stale, buf.String())
	}
}

func TestLoadKnown(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	if err := os.WriteFile(good, []byte("# c\n\nphase:a/b #12\nrule:R #34 trailing words\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	k, err := loadKnown(good)
	if err != nil || k["phase:a/b"] != "#12" || k["rule:R"] != "#34" || len(k) != 2 {
		t.Fatalf("loadKnown = %v, %v", k, err)
	}
	bad := filepath.Join(dir, "bad.txt")
	if err := os.WriteFile(bad, []byte("phase:a/b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadKnown(bad); err == nil {
		t.Fatal("an entry without an issue must be rejected")
	}
}

// TestCheckedInAllowlistParses keeps known-superlinear.txt loadable and every
// entry tied to an issue.
func TestCheckedInAllowlistParses(t *testing.T) {
	k, err := loadKnown("known-superlinear.txt")
	if err != nil {
		t.Fatal(err)
	}
	for key := range k {
		if !strings.HasPrefix(key, "phase:") && !strings.HasPrefix(key, "rule:") {
			t.Errorf("entry %q must start with phase: or rule:", key)
		}
	}
}

func treeHash(t *testing.T, dir string) (string, int) {
	t.Helper()
	h := sha256.New()
	n := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write([]byte(filepath.ToSlash(rel)))
		h.Write(data)
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil)), n
}

func TestGenerateIsDeterministicAndShaped(t *testing.T) {
	spec := genSpec{Modules: 3, FilesPerModule: 4, ClassesPerFile: 6}
	a, b := t.TempDir(), t.TempDir()
	if err := generate(a, spec); err != nil {
		t.Fatal(err)
	}
	if err := generate(b, spec); err != nil {
		t.Fatal(err)
	}
	ha, na := treeHash(t, a)
	hb, _ := treeHash(t, b)
	if ha != hb {
		t.Fatal("generate is not deterministic")
	}
	// settings, root build, catalog, app (build, manifest, activity, strings,
	// layout) + per module: build, manifest, bases, strings, files, a layout
	// per 4 files.
	if want := 3 + 5 + 3*(4+4+1); na != want {
		t.Errorf("generated %d files, want %d", na, want)
	}
	manifest, err := os.ReadFile(filepath.Join(a, "feature1", "src", "main", "AndroidManifest.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<activity", "<service", "<receiver", "<provider"} {
		if !strings.Contains(string(manifest), want) {
			t.Errorf("module manifest lacks %s", want)
		}
	}
	src, err := os.ReadFile(filepath.Join(a, "feature1", "src", "main", "kotlin", "com", "example", "f1", "File0.kt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{": BaseActivity1()", ": BaseService1()", ": BaseReceiver1()", "@Composable", "data class Model", "class Repository"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("generated source lacks %q", want)
		}
	}
	if err := generate(t.TempDir(), genSpec{}); err == nil {
		t.Error("an empty spec must be rejected")
	}
}

// TestRunKritStartsCold runs a fake krit that fails if the corpus still has
// the .krit directory a previous run left behind.
func TestRunKritStartsCold(t *testing.T) {
	dir := t.TempDir()
	corpus := filepath.Join(dir, "corpus")
	if err := os.MkdirAll(filepath.Join(corpus, ".krit", "parse-cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "krit")
	script := "#!/bin/sh\nfor last; do :; done\n" +
		"if [ -e \"$last/.krit\" ]; then echo 'warm cache left behind' >&2; exit 3; fi\n" +
		"mkdir -p \"$last/.krit/parse-cache\"\n" +
		"echo '" + strings.ReplaceAll(reportJSON, "\n", " ") + "'\nexit 1\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		s, err := runKrit(t.Context(), fake, defaultScanArgs, corpus)
		if err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
		if s.TotalMs != 1000 {
			t.Fatalf("run %d: total %v", i+1, s.TotalMs)
		}
	}
}

// TestScalingFailsOnlyOnHeadWorseThanBase pins the base-vs-head scaling rule:
// a metric super-linear on base too is reported but doesn't fail, unless head
// grows clearly faster than base.
func TestScalingFailsOnlyOnHeadWorseThanBase(t *testing.T) {
	g := scaleGates{Factor: 6, Slack: 2, MinMs: 500}
	small := summ(1000, map[string]float64{"io": 30, "worse": 100, "fresh": 100}, nil, nil)
	large := summ(9000, map[string]float64{"io": 540, "worse": 3600, "fresh": 1300}, nil, nil)
	head, _ := checkScaling(small, large, g, knownSuperlinear{})

	// Base: io equally super-linear; worse super-linear but much less;
	// fresh linear.
	bSmall := summ(1000, map[string]float64{"io": 30, "worse": 100, "fresh": 100}, nil, nil)
	bLarge := summ(9000, map[string]float64{"io": 545, "worse": 1300, "fresh": 600}, nil, nil)
	base, _ := checkScaling(bSmall, bLarge, g, knownSuperlinear{})
	head = applyBase(head, base)

	got := map[string]bool{}
	for _, r := range head {
		got[r.Key] = r.Super && !r.PreExisting
	}
	if got["io"] {
		t.Error("a phase base scales as badly must not fail head")
	}
	if !got["worse"] {
		t.Error("a phase head scales clearly worse than base must fail")
	}
	if !got["fresh"] {
		t.Error("a phase only head scales super-linearly must fail")
	}
	var buf bytes.Buffer
	if !writeScaleReport(&buf, "s", "l", head, nil, g, knownSuperlinear{}) || !strings.Contains(buf.String(), "not caused by this change") {
		t.Errorf("report should fail and list the pre-existing metric separately:\n%s", buf.String())
	}
}

// The FIR corpus (#1729): deterministic, JVM-only, with a Gradle model whose
// feature source sets depend on core, so krit compiles them as modules.
func TestGenerateJVMIsDeterministicAndHasAModel(t *testing.T) {
	spec := genSpec{Modules: 3, FilesPerModule: 2, ClassesPerFile: 2}
	a := filepath.Join(t.TempDir(), "p")
	if err := generateJVM(a, spec); err != nil {
		t.Fatal(err)
	}
	ha, na := treeHash(t, a)
	if err := os.RemoveAll(a); err != nil {
		t.Fatal(err)
	}
	if err := generateJVM(a, spec); err != nil {
		t.Fatal(err)
	}
	if hb, _ := treeHash(t, a); ha != hb {
		t.Fatal("generateJVM is not deterministic")
	}
	// settings, model, core (build + 2 files), 3 features (build + 2 files).
	if want := 2 + 3 + 3*3; na != want {
		t.Errorf("generated %d files, want %d", na, want)
	}
	data, err := os.ReadFile(filepath.Join(a, ".krit", "gradle-model", "model.json"))
	if err != nil {
		t.Fatal(err)
	}
	var model struct {
		RootDir  string `json:"rootDir"`
		Projects []struct {
			Path       string `json:"path"`
			SourceSets []struct {
				ProjectDeps []string `json:"projectDeps"`
				SourceDirs  []string `json:"sourceDirs"`
			} `json:"sourceSets"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	if model.RootDir != a || len(model.Projects) != 4 {
		t.Fatalf("model root %q with %d projects", model.RootDir, len(model.Projects))
	}
	feature := model.Projects[2]
	if feature.Path != ":feature1" || len(feature.SourceSets) != 1 ||
		strings.Join(feature.SourceSets[0].ProjectDeps, ",") != ":core" ||
		feature.SourceSets[0].SourceDirs[0] != filepath.Join(a, "feature1", "src", "main", "kotlin") {
		t.Fatalf("feature1 = %+v", feature)
	}
	src, err := os.ReadFile(filepath.Join(a, "feature1", "src", "main", "kotlin", "com", "example", "f1", "Feature0.kt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"import com.example.core.*", ": Base0_0(name), Source0_0", "suspend fun refresh"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("feature source lacks %q", want)
		}
	}
	if err := generateJVM(t.TempDir(), genSpec{}); err == nil {
		t.Error("an empty spec must be rejected")
	}
}

// A FIR run keeps the corpus's model, runs with the side's environment and a
// fresh state directory of its own, and fails when FIR analyzed nothing.
func TestRunSideFIR(t *testing.T) {
	dir := t.TempDir()
	corpus := filepath.Join(dir, "corpus")
	for _, d := range []string{".krit/gradle-model", ".krit/fir-check-cache"} {
		if err := os.MkdirAll(filepath.Join(corpus, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	states := filepath.Join(dir, "states")
	report := strings.ReplaceAll(reportJSON, "\n", " ")
	fake := func(name, fir string) string {
		path := filepath.Join(dir, name)
		script := "#!/bin/sh\nfor last; do :; done\n" +
			"[ -d \"$last/.krit/gradle-model\" ] || { echo 'model removed' >&2; exit 3; }\n" +
			"[ -e \"$last/.krit/fir-check-cache\" ] && { echo 'cache left behind' >&2; exit 3; }\n" +
			"[ \"$KRIT_FIR_JAR\" = /jars/side.jar ] || { echo \"KRIT_FIR_JAR=$KRIT_FIR_JAR\" >&2; exit 3; }\n" +
			"[ -d \"$KRIT_STATE_DIR\" ] && [ -z \"$(ls -A \"$KRIT_STATE_DIR\")\" ] || { echo 'state dir not fresh' >&2; exit 3; }\n" +
			"echo \"$KRIT_STATE_DIR\" >> " + states + "\ntouch \"$KRIT_STATE_DIR/used\"\n" +
			"mkdir -p \"$last/.krit/fir-check-cache\"\n" +
			"echo '" + strings.Replace(report, `"perfRuleStats"`, `"summary": {"fir": {"authoritative": `+fir+`}}, "perfRuleStats"`, 1) + "'\nexit 1\n"
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	sd := side{Bin: fake("krit", "12"), Env: []string{"KRIT_FIR_JAR=/jars/side.jar"}, FIR: true}
	for i := 0; i < 2; i++ {
		if _, err := runSide(t.Context(), sd, firScanArgs, corpus); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	data, err := os.ReadFile(states)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Fields(string(data)); len(lines) != 2 || lines[0] == lines[1] {
		t.Fatalf("state dirs = %v, want two distinct ones", lines)
	}
	sd.Bin = fake("krit-no-fir", "0")
	if _, err := runSide(t.Context(), sd, firScanArgs, corpus); err == nil || !strings.Contains(err.Error(), "FIR analyzed no file") {
		t.Fatalf("a FIR run without FIR = %v", err)
	}
}

// Under -fir only the total and phases are gated, with second-scale floors:
// a rule's one-time cost moving between rules is not a regression.
func TestFIRGates(t *testing.T) {
	base := summ(20000, map[string]float64{"typeOracle": 18000, "ruleCallbacks": 1400},
		map[string]float64{"ViewTag": 14, "FragmentLiveDataObserve": 222}, map[string]float64{"ViewTag": 2, "FragmentLiveDataObserve": 25})
	blip := summ(20300, map[string]float64{"typeOracle": 18100, "ruleCallbacks": 1500},
		map[string]float64{"ViewTag": 272, "FragmentLiveDataObserve": 4}, map[string]float64{"ViewTag": 31, "FragmentLiveDataObserve": 1})
	if got := kinds(compareSummaries("c", base, blip, defaultFIRGates)); len(got) != 0 {
		t.Fatalf("A/A noise regressed under the FIR gates: %v", got)
	}
	if got := kinds(compareSummaries("c", base, blip, defaultGates)); len(got) == 0 {
		t.Fatal("the source-only gates should still see the rule blip (the reason FIR mode turns them off)")
	}
	slow := summ(24000, map[string]float64{"typeOracle": 23000, "ruleCallbacks": 1400}, base.Rules, base.Shares)
	if got := strings.Join(kinds(compareSummaries("c", base, slow, defaultFIRGates)), ","); got != "phase:typeOracle,total:durationMs" {
		t.Fatalf("a slower oracle compile = %s", got)
	}

	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	g := defaultGates
	fs.Float64Var(&g.TotalPct, "total-pct", g.TotalPct, "")
	fs.Float64Var(&g.PhaseMinMs, "phase-min-ms", g.PhaseMinMs, "")
	if err := fs.Parse([]string{"-total-pct", "8"}); err != nil {
		t.Fatal(err)
	}
	got := firGatesWith(fs, g)
	if got.TotalPct != 8 || got.PhaseMinMs != defaultFIRGates.PhaseMinMs || got.rulesGated() {
		t.Fatalf("firGatesWith = %+v", got)
	}
}
