package parity_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// Property-based tests over the FIR checker pipeline. Each generator enumerates
// a bounded grid across the axes that decide a rule's verdict — a systematic,
// reproducible sweep of the input space rather than a handful of examples. For
// every generated snippet the generator also records the expected verdict
// (shouldFlag), which serves as the oracle for three properties per rule:
//
//   - Expectation: FIR flags a snippet iff shouldFlag.
//   - Determinism: two runs over the same batch produce identical findings.
//   - Differential: the Go rule agrees with FIR on the same snippet (the
//     parity contract, extended from the 6 golden fixtures to the whole grid).
//
// All snippets for a rule run through a single krit-fir invocation (one JVM
// subprocess), so a grid of dozens of cases stays cheap. Each snippet is its
// own package to avoid conflicting top-level declarations in the shared compile.

type genCase struct {
	name       string // also the source file / package suffix
	source     string
	shouldFlag bool
	axes       string // human-readable axis values, for failure messages
}

// ---- Compose: remember capture × keyed ----

func composeCases() []genCase {
	var cases []genCase
	captures := []struct {
		key  string
		calc func(keyed bool) string // the remember(...) call
	}{
		{"noCapture", func(keyed bool) string {
			if keyed {
				return "remember(seed) { 42 }"
			}
			return "remember { 42 }"
		}},
		{"lambdaParam", func(keyed bool) string {
			if keyed {
				return "remember(seed) { seed + 1 }"
			}
			return "remember { seed + 1 }"
		}},
		{"callableRefParam", func(keyed bool) string {
			if keyed {
				return "remember(seed, seed::toString)"
			}
			return "remember(seed::toString)"
		}},
	}
	for _, c := range captures {
		for _, keyed := range []bool{false, true} {
			name := fmt.Sprintf("compose_%s_keyed%v", c.key, keyed)
			capturing := c.key != "noCapture"
			shouldFlag := !keyed && capturing
			src := fmt.Sprintf(`package %s
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
@Composable
fun Screen(seed: Int) {
    val v = %s
    use(v)
}
fun use(x: Any?) {}
`, name, c.calc(keyed))
			cases = append(cases, genCase{name + ".kt", src, shouldFlag, fmt.Sprintf("capture=%s keyed=%v", c.key, keyed)})
		}
	}
	return cases
}

// ---- Flow: lifecycle method × wrapper ----

func flowCases() []genCase {
	var cases []genCase
	methods := []string{"onCreate", "onStart", "onViewCreated", "onResume", "observe"}
	lifecycle := map[string]bool{"onCreate": true, "onStart": true, "onViewCreated": true}
	wrappers := []struct {
		key         string
		open, close string
	}{
		{"none", "", ""},
		{"repeatOnLifecycle", "repeatOnLifecycle(Lifecycle.State.STARTED) {", "}"},
		{"launchWhenStarted", "lifecycleScope.launchWhenStarted {", "}"},
	}
	for _, m := range methods {
		for _, w := range wrappers {
			name := fmt.Sprintf("flow_%s_%s", m, w.key)
			shouldFlag := lifecycle[m] && w.key != "repeatOnLifecycle"
			src := fmt.Sprintf(`package %s
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.launch
import kotlinx.coroutines.launchWhenStarted
import kotlinx.coroutines.flow.MutableStateFlow
class Screen : AppCompatActivity() {
    private val state = MutableStateFlow(0)
    fun %s() {
        lifecycleScope.launch {
            %s
            state.collect { }
            %s
        }
    }
}
`, name, m, w.open, w.close)
			cases = append(cases, genCase{name + ".kt", src, shouldFlag, fmt.Sprintf("method=%s wrapper=%s", m, w.key)})
		}
	}
	return cases
}

// ---- InjectDispatcher: owner × dispatcher ----

func injectCases() []genCase {
	var cases []genCase
	owners := []struct {
		key     string
		flagged bool
		wrap    func(body string) string
	}{
		{"member", true, func(b string) string { return "class C {\n    suspend fun m() { " + b + " }\n}" }},
		{"object", true, func(b string) string { return "object O {\n    suspend fun m() { " + b + " }\n}" }},
		{"companion", true, func(b string) string {
			return "class C {\n    companion object {\n        suspend fun m() { " + b + " }\n    }\n}"
		}},
		{"topLevel", false, func(b string) string { return "suspend fun m() { " + b + " }" }},
		{"extension", false, func(b string) string { return "suspend fun String.m() { " + b + " }" }},
	}
	dispatchers := []string{"IO", "Default", "Unconfined", "Main"}
	for _, o := range owners {
		for _, d := range dispatchers {
			name := fmt.Sprintf("inject_%s_%s", o.key, d)
			shouldFlag := o.flagged && d != "Main"
			body := fmt.Sprintf("withContext(Dispatchers.%s) { }", d)
			src := fmt.Sprintf(`package %s
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
%s
`, name, o.wrap(body))
			cases = append(cases, genCase{name + ".kt", src, shouldFlag, fmt.Sprintf("owner=%s dispatcher=%s", o.key, d)})
		}
	}
	return cases
}

// ---- UnsafeCastWhenNullable: operand × op × target nullability ----

func castCases() []genCase {
	var cases []genCase
	operands := []struct {
		key       string
		decl      string // declares `val a`, empty when using a literal directly
		expr      string // the operand expression
		subtypeOf bool   // operand is a subtype of String? (so a cast to String? always succeeds)
	}{
		{"anyNullable", "val a: Any? = any()", "a", false},
		{"nonNullString", "val a: String = \"\"", "a", true},
		{"nullLiteral", "", "null", true},
	}
	ops := []struct {
		key   string
		token string
		isAs  bool
	}{
		{"as", "as", true},
		{"asSafe", "as?", false},
	}
	targets := []struct {
		key      string
		typ      string
		nullable bool
	}{
		{"nullable", "String?", true},
		{"nonNull", "String", false},
	}
	for _, operand := range operands {
		for _, op := range ops {
			for _, tgt := range targets {
				// `as?` to a non-null target is the idiomatic safe cast; `as?`
				// to a nullable target (`as? String?`) is not valid Kotlin, so
				// only emit `as?` with a non-null target.
				if !op.isAs && tgt.nullable {
					continue
				}
				name := fmt.Sprintf("cast_%s_%s_%s", operand.key, op.key, tgt.key)
				shouldFlag := op.isAs && tgt.nullable && !operand.subtypeOf
				decl := operand.decl
				if decl != "" {
					decl += "\n    "
				}
				src := fmt.Sprintf(`package %s
fun any(): Any? = null
fun f() {
    %sval y = %s %s %s
    use(y)
}
fun use(x: Any?) {}
`, name, decl, operand.expr, op.token, tgt.typ)
				cases = append(cases, genCase{name + ".kt", src, shouldFlag, fmt.Sprintf("operand=%s op=%s target=%s", operand.key, op.key, tgt.key)})
			}
		}
	}
	return cases
}

// launchWhenStartedStub is needed by the Flow grid; the parity stubs omit it.
const launchWhenStartedStub = `package kotlinx.coroutines
fun CoroutineScope.launchWhenStarted(block: suspend CoroutineScope.() -> Unit) {}
`

// firFlagCounts runs one krit-fir invocation over all cases (plus any extra
// stub files) and returns the flag count per case file for the given rule.
func firFlagCounts(t *testing.T, ruleCheckerName, ruleID string, cases []genCase, extraStubs map[string]string) map[string]int {
	t.Helper()
	sources := map[string]string{}
	for k, v := range extraStubs {
		sources[k] = v
	}
	for _, c := range cases {
		sources[c.name] = c.source
	}
	byFile := firCheck(t, []string{ruleCheckerName}, sources)
	counts := map[string]int{}
	for _, c := range cases {
		counts[c.name] = rulesOf(byFile[c.name], ruleID)
	}
	return counts
}

func assertExpectation(t *testing.T, cases []genCase, counts map[string]int) {
	t.Helper()
	for _, c := range cases {
		got := counts[c.name]
		if c.shouldFlag && got == 0 {
			t.Errorf("[%s] expected a finding (%s) but got none — false negative", c.name, c.axes)
		}
		if !c.shouldFlag && got != 0 {
			t.Errorf("[%s] expected no finding (%s) but got %d — false positive", c.name, c.axes, got)
		}
	}
}

func TestFirProperty_Expectation(t *testing.T) {
	rules := []struct {
		checker, id string
		cases       []genCase
		stubs       map[string]string
	}{
		{"ComposeRememberWithoutKey", "ComposeRememberWithoutKey", composeCases(), nil},
		{"FlowCollectInOnCreate", "CollectInOnCreateWithoutLifecycle", flowCases(), map[string]string{"ZLaunchWhenStub.kt": launchWhenStartedStub}},
		{"InjectDispatcher", "InjectDispatcher", injectCases(), nil},
		{"UnsafeCastWhenNullable", "UnsafeCastWhenNullable", castCases(), nil},
	}
	for _, r := range rules {
		t.Run(r.checker, func(t *testing.T) {
			counts := firFlagCounts(t, r.checker, r.id, r.cases, r.stubs)
			assertExpectation(t, r.cases, counts)
		})
	}
}

func TestFirProperty_Determinism(t *testing.T) {
	// Same batch, two independent krit-fir runs -> identical per-case findings.
	cases := append(append(append(composeCases(), flowCases()...), injectCases()...), castCases()...)
	stubs := map[string]string{"ZLaunchWhenStub.kt": launchWhenStartedStub}
	sources := map[string]string{}
	for k, v := range stubs {
		sources[k] = v
	}
	for _, c := range cases {
		sources[c.name] = c.source
	}
	fingerprint := func() string {
		by := firCheck(t, []string{
			"ComposeRememberWithoutKey", "FlowCollectInOnCreate", "InjectDispatcher", "UnsafeCastWhenNullable",
		}, sources)
		var keys []string
		for f, fs := range by {
			for _, fd := range fs {
				keys = append(keys, fmt.Sprintf("%s:%d:%d:%s:%s", f, fd.Line, fd.Col, fd.Rule, fd.Message))
			}
		}
		sort.Strings(keys)
		return strings.Join(keys, "\n")
	}
	a := fingerprint()
	b := fingerprint()
	if a != b {
		t.Errorf("krit-fir findings are not deterministic across runs\nrun1:\n%s\nrun2:\n%s", a, b)
	}
	if a == "" {
		t.Error("expected some findings across the generated grid; got none (harness/stub problem?)")
	}
}

// runGoRuleCountOnSource writes source to a temp file, runs the single Go rule
// (source-inference resolver, no oracle — the same configuration TestFirPilotParity
// uses), and returns the finding count for ruleID.
func runGoRuleCountOnSource(t *testing.T, ruleID, source string) (int, bool) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "Src.kt")
	if err := os.WriteFile(p, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), p)
	if err != nil {
		t.Fatalf("parse generated source: %v", err)
	}
	for _, rule := range api.Registry {
		if rule.ID != ruleID {
			continue
		}
		var resolver typeinfer.TypeResolver
		if rule.Needs.Has(api.NeedsResolver) {
			r := typeinfer.NewResolver()
			r.IndexFilesParallel([]*scanner.File{file}, 1)
			resolver = r
		}
		cols, stats := rules.NewDispatcher([]*api.Rule{rule}, resolver).RunWithStats(file)
		if len(stats.Errors) > 0 {
			// A rule that errors during dispatch emits no findings; without this
			// the differential would read that as "Go says no flag" — a vacuous
			// pass. Surface it instead.
			t.Fatalf("Go rule %q errored on generated source: %v", ruleID, stats.Errors)
		}
		findings := cols.Findings()
		n := 0
		for _, f := range findings {
			if f.Rule == ruleID {
				n++
			}
		}
		return n, true
	}
	return 0, false // no Go rule with this ID (FIR-only rule)
}

// TestFirProperty_Differential checks the parity contract across the whole grid:
// the FIR checker and the Go rule reach the same verdict (flag / no-flag) on
// each generated snippet. The Go rule runs with source inference only (no
// oracle), matching TestFirPilotParity. Cases where the Go rule provably needs
// the oracle to match are listed in goOracleDependent and checked only against
// the generator's own expectation, not against FIR/Go agreement.
func TestFirProperty_Differential(t *testing.T) {
	rules := []struct {
		checker, id string
		cases       []genCase
		stubs       map[string]string
	}{
		{"ComposeRememberWithoutKey", "ComposeRememberWithoutKey", composeCases(), nil},
		{"FlowCollectInOnCreate", "CollectInOnCreateWithoutLifecycle", flowCases(), map[string]string{"ZLaunchWhenStub.kt": launchWhenStartedStub}},
		{"InjectDispatcher", "InjectDispatcher", injectCases(), nil},
		{"UnsafeCastWhenNullable", "UnsafeCastWhenNullable", castCases(), nil},
	}
	// Documented boundaries where FIR is intentionally more precise than the
	// Go rule. FIR resolves a bound callable reference's captured receiver
	// (remember(seed::toString)); the Go rule matches only lambda captures.
	// These are asserted against the generator's own expectation instead.
	knownDivergences := map[string]string{
		"compose_callableRefParam_keyedfalse.kt": "FIR resolves bound callable-reference captures; the Go rule matches only lambda captures",
	}
	for _, r := range rules {
		t.Run(r.checker, func(t *testing.T) {
			firCounts := firFlagCounts(t, r.checker, r.id, r.cases, r.stubs)
			for _, c := range r.cases {
				firFlag := firCounts[c.name] > 0
				goCount, hasGoRule := runGoRuleCountOnSource(t, r.id, c.source)
				if !hasGoRule {
					// FIR-only rule (no Go counterpart): the expectation test
					// already validates FIR; assert FIR matches expectation here too.
					if firFlag != c.shouldFlag {
						t.Errorf("[%s] FIR-only rule verdict wrong (%s): FIR=%v want=%v", c.name, c.axes, firFlag, c.shouldFlag)
					}
					continue
				}
				if reason, ok := knownDivergences[c.name]; ok {
					if firFlag != c.shouldFlag {
						t.Errorf("[%s] documented-divergence case has wrong FIR verdict (%s): FIR=%v want=%v", c.name, c.axes, firFlag, c.shouldFlag)
					}
					if firFlag == (goCount > 0) {
						t.Errorf("[%s] no longer diverges (FIR=Go=%v) — remove it from knownDivergences", c.name, firFlag)
					}
					t.Logf("[%s] known FIR/Go boundary (FIR=%v Go=%v): %s", c.name, firFlag, goCount > 0, reason)
					continue
				}
				if firFlag != (goCount > 0) {
					t.Errorf("[%s] FIR/Go disagree (%s): FIR=%v Go=%v (generator expected flag=%v)",
						c.name, c.axes, firFlag, goCount > 0, c.shouldFlag)
				}
			}
		})
	}
}

// allPropertyCases concatenates every rule's generated grid.
func allPropertyCases() []genCase {
	return append(append(append(composeCases(), flowCases()...), injectCases()...), castCases()...)
}

// fuzzWrappers apply source-level edits that keep the snippet compiling but
// push it off the exact grid shapes, so the fuzzer explores beyond the
// enumerated cases.
var fuzzWrappers = []func(string) string{
	func(s string) string { return s },
	func(s string) string { return s + "\nfun zExtra1() {}\n" },
	func(s string) string { return "\n" + s },
	func(s string) string { return s + "\n// trailing\n" },
}

// modIndex maps any int (including math.MinInt, where negation overflows) into
// [0, n) without panicking.
func modIndex(x, n int) int {
	i := x % n
	if i < 0 {
		i += n
	}
	return i
}

// FuzzFirCheckers drives the krit-fir pipeline over generated-and-mutated
// snippets and asserts it never crashes and never returns a Go-side error.
// Each snippet is built from a grid case (always compiling) plus a textual
// wrapper, so inputs stay valid Kotlin while ranging beyond the exact grid.
// During `go test` only the seed corpus runs (cheap); `go test -fuzz` explores.
func FuzzFirCheckers(f *testing.F) {
	all := allPropertyCases()
	// A small seed corpus keeps `go test` cheap (each exec is one JVM run);
	// `go test -fuzz` explores the index/wrapper space further.
	for _, seed := range []int{0, len(all) / 2, len(all) - 1} {
		f.Add(seed, 1)
	}
	f.Fuzz(func(t *testing.T, idx, wrapSel int) {
		if len(all) == 0 {
			t.Skip("no generated cases")
		}
		c := all[modIndex(idx, len(all))]
		src := fuzzWrappers[modIndex(wrapSel, len(fuzzWrappers))](c.source)

		sources := map[string]string{c.name: src, "ZLaunchWhenStub.kt": launchWhenStartedStub}
		// firCheck skips when the jar/stdlib is absent, and t.Fatalf's on a
		// crash or invoke error — exactly the no-crash property. Enable all four
		// checkers so any of them may run on the mutated snippet.
		_ = firCheck(t, allCheckerNames, sources)
	})
}

var allCheckerNames = []string{
	"ComposeRememberWithoutKey", "FlowCollectInOnCreate", "InjectDispatcher", "UnsafeCastWhenNullable",
}
