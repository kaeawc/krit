package parity_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// firCheck runs the krit-fir checkers over the given sources (on top of the
// compiler-tests stub library) and returns findings keyed by the caller's file
// names. It skips when the jar, stdlib, or javac is unavailable.
func firCheck(t *testing.T, rules []string, sources map[string]string) map[string][]scanner.Finding {
	t.Helper()
	tmp := t.TempDir()
	var files []string
	byName := map[string]string{}
	for name, body := range sources {
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
		abs, _ := filepath.Abs(p)
		byName[name] = abs
	}

	res := firInvoke(t, rules, files)
	if len(res.Crashed) > 0 {
		t.Fatalf("krit-fir crashed: %v", res.Crashed)
	}
	if len(res.ErrorFiles) > 0 {
		t.Fatalf("krit-fir hit compiler errors: %v", res.ErrorFiles)
	}

	out := map[string][]scanner.Finding{}
	for _, f := range res.Findings {
		out[f.File] = append(out[f.File], f)
	}
	// Re-key by the caller's short names.
	byShort := map[string][]scanner.Finding{}
	for name, abs := range byName {
		byShort[name] = out[abs]
	}
	return byShort
}

func rulesOf(findings []scanner.Finding, rule string) int {
	n := 0
	for _, f := range findings {
		if f.Rule == rule {
			n++
		}
	}
	return n
}

// TestComposeRememberWithoutKey_KeylessNoCapture pins that a keyless
// `remember { ... }` whose calculation lambda captures nothing external is
// NOT flagged. `remember { mutableStateOf(0) }` is the canonical, correct
// Compose idiom; the Go rule (ComposeRememberWithoutKeyRule) only fires when
// the lambda references an enclosing parameter, and the FIR checker must
// match that precision rather than flag every keyless remember.
func TestComposeRememberWithoutKey_KeylessNoCapture(t *testing.T) {
	sources := map[string]string{
		"NoCapture.kt": `package sample

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.runtime.mutableStateOf

@Composable
fun Counter() {
    val state = remember { mutableStateOf(0) }
    use(state.value)
}

fun use(x: Int) {}
`,
		"CaptureInLambda.kt": `package sample2

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

fun Box(content: () -> Unit) { content() }

@Composable
fun Screen(seed: Int) {
    Box {
        val v = remember { compute2(seed) }
        use2b(v)
    }
}

fun compute2(x: Int): Int = x
fun use2b(x: Int) {}
`,
		"Capture.kt": `package sample

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Derived(seed: Int) {
    val v = remember { compute(seed) }
    use2(v)
}

fun compute(x: Int): Int = x
fun use2(x: Int) {}
`,
	}
	findings := firCheck(t, []string{"ComposeRememberWithoutKey"}, sources)

	if n := rulesOf(findings["NoCapture.kt"], "ComposeRememberWithoutKey"); n != 0 {
		t.Errorf("keyless remember { mutableStateOf(0) } (no capture) flagged %d time(s); want 0 — this is the canonical correct idiom", n)
	}
	if n := rulesOf(findings["Capture.kt"], "ComposeRememberWithoutKey"); n != 1 {
		t.Errorf("keyless remember capturing enclosing param `seed` flagged %d time(s); want 1", n)
	}
	// The dominant real-world shape: remember inside a content lambda
	// (Column/Box/...). The enclosing *named* function is still the composable,
	// so a captured param must be detected through the intervening lambda.
	if n := rulesOf(findings["CaptureInLambda.kt"], "ComposeRememberWithoutKey"); n != 1 {
		t.Errorf("keyless remember inside a content lambda capturing `seed` flagged %d time(s); want 1", n)
	}
}

// flowCollectSources builds a project exercising CollectInOnCreateWithoutLifecycle across
// lifecycle callbacks and coroutine builders. The Go rule covers onCreate,
// onStart, and onViewCreated, and treats only repeatOnLifecycle as safe
// (launchWhenStarted/launchWhenResumed only suspend the collector, leaving the
// upstream flow active — the leak repeatOnLifecycle fixes).
func flowCollectSources() map[string]string {
	screen := func(pkg, method, wrapperOpen, wrapperClose string) string {
		return `package ` + pkg + `

import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch

class Screen(private val owner: LifecycleOwner) {
    private val state = MutableStateFlow(0)
    fun ` + method + `() {
        ` + wrapperOpen + `
            state.collect { }
        ` + wrapperClose + `
    }
}
`
	}
	launch, end := "owner.lifecycleScope.launch {", "}"
	return map[string]string{
		// onStart is in the Go rule's callback set; a bare collect here must flag.
		"OnStart.kt": screen("onstart", "onStart", launch, end),
		// onViewCreated is also in the set.
		"OnViewCreated.kt": screen("onview", "onViewCreated", launch, end),
		// onResume is deliberately NOT in the set; must not flag.
		"OnResume.kt": screen("onresume", "onResume", launch, end),
		// launchWhenStarted only suspends the collector; Go still flags it.
		"LaunchWhen.kt": screen("launchwhen", "onCreate", "owner.lifecycleScope.launchWhenStarted {", end),
		// repeatOnLifecycle is the one safe wrapper; must not flag.
		"RepeatOn.kt": screen("repeaton", "onCreate", launch+" owner.repeatOnLifecycle(Lifecycle.State.STARTED) {", "} }"),
	}
}

func TestCollectInOnCreateWithoutLifecycle_LifecycleCoverage(t *testing.T) {
	f := firCheck(t, []string{"CollectInOnCreateWithoutLifecycle"}, flowCollectSources())
	rule := "CollectInOnCreateWithoutLifecycle"
	want := map[string]int{
		"OnStart.kt":       1,
		"OnViewCreated.kt": 1,
		"OnResume.kt":      0,
		"LaunchWhen.kt":    1,
		"RepeatOn.kt":      0,
	}
	for file, exp := range want {
		if got := rulesOf(f[file], rule); got != exp {
			t.Errorf("%s: %s fired %d time(s); want %d", file, rule, got, exp)
		}
	}
}

func TestInjectDispatcher_OwnerAndIdiom(t *testing.T) {
	sources := map[string]string{
		// Member function of a class: injectable via constructor -> flag.
		"Member.kt": `package inj
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
class Repo {
    suspend fun load() { withContext(Dispatchers.IO) { } }
}
`,
		// Top-level function: nothing to inject into -> no flag.
		"TopLevel.kt": `package inj
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
suspend fun loadTop() { withContext(Dispatchers.IO) { } }
`,
		// Extension function: receiver is fixed by the call site -> no flag.
		"Extension.kt": `package inj
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
suspend fun String.loadExt() { withContext(Dispatchers.IO) { } }
`,
		// Constructor default value IS the injection point -> no flag.
		"CtorDefault.kt": `package inj
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
class Repo2(val d: CoroutineDispatcher = Dispatchers.IO)
`,
		// Dispatchers.Main is excluded by design -> no flag.
		"MainExcluded.kt": `package inj
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
class Repo3 {
    suspend fun x() { withContext(Dispatchers.Main) { } }
}
`,
	}
	f := firCheck(t, []string{"InjectDispatcher"}, sources)
	rule := "InjectDispatcher"
	want := map[string]int{
		"Member.kt":       1,
		"TopLevel.kt":     0,
		"Extension.kt":    0,
		"CtorDefault.kt":  0,
		"MainExcluded.kt": 0,
	}
	for file, exp := range want {
		if got := rulesOf(f[file], rule); got != exp {
			t.Errorf("%s: %s fired %d time(s); want %d", file, rule, got, exp)
		}
	}
}

func TestUnsafeCastWhenNullable_Cases(t *testing.T) {
	sources := map[string]string{
		// Real unsafe downcast: Any? is not a subtype of String?, the cast
		// can throw ClassCastException -> flag.
		"Downcast.kt": `package cast1
fun any(): Any? = null
fun f() {
    val a: Any? = any()
    val s = a as String?
    use(s)
}
fun use(x: String?) {}
`,
		// null literal: type Nothing? is a subtype of String?, always safe -> no flag.
		"TypedNull.kt": `package cast2
fun f() {
    val s = null as String?
    use(s)
}
fun use(x: String?) {}
`,
		// Redundant widening: String is a subtype of String?, always
		// succeeds (USELESS_CAST, not unsafe) -> no flag.
		"Widening.kt": `package cast3
fun f(x: String) {
    val s = x as String?
    use(s)
}
fun use(x: String?) {}
`,
		// Safe cast operator: not the AS operation -> no flag.
		"SafeCast.kt": `package cast4
fun any(): Any? = null
fun f() {
    val a: Any? = any()
    val s = a as? String
    use(s)
}
fun use(x: String?) {}
`,
	}
	f := firCheck(t, []string{"UnsafeCastWhenNullable"}, sources)
	rule := "UnsafeCastWhenNullable"
	want := map[string]int{
		"Downcast.kt":  1,
		"TypedNull.kt": 0,
		"Widening.kt":  0,
		"SafeCast.kt":  0,
	}
	for file, exp := range want {
		if got := rulesOf(f[file], rule); got != exp {
			t.Errorf("%s: %s fired %d time(s); want %d", file, rule, got, exp)
		}
	}
	for _, fd := range f["Downcast.kt"] {
		if fd.Rule == rule && fd.Severity != "warning" {
			t.Errorf("UnsafeCastWhenNullable severity = %q; want warning (code smell, not a compiler error)", fd.Severity)
		}
	}
}
