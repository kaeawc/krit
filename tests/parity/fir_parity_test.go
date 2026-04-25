package parity_test

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

type parityCase struct {
	name        string
	goRule      string
	fixture     string
	packageName string
}

func TestFirPilotParity(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar` to enable FIR parity")
	}
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache; run `cd tools/krit-fir && ./gradlew :compiler-tests:test`")
	}

	cases := []parityCase{
		{
			name:        "FlowCollectInOnCreate_Positive",
			goRule:      "CollectInOnCreateWithoutLifecycle",
			fixture:     "tests/fixtures/positive/coroutines/CollectInOnCreateWithoutLifecycle.kt",
			packageName: "parity.flow.positive",
		},
		{
			name:        "FlowCollectInOnCreate_Negative",
			goRule:      "CollectInOnCreateWithoutLifecycle",
			fixture:     "tests/fixtures/negative/coroutines/CollectInOnCreateWithoutLifecycle.kt",
			packageName: "parity.flow.negative",
		},
		{
			name:        "ComposeRememberWithoutKey_Positive",
			goRule:      "ComposeRememberWithoutKey",
			fixture:     "tests/fixtures/positive/compose/ComposeRememberWithoutKey.kt",
			packageName: "parity.compose.positive",
		},
		{
			name:        "ComposeRememberWithoutKey_Negative",
			goRule:      "ComposeRememberWithoutKey",
			fixture:     "tests/fixtures/negative/compose/ComposeRememberWithoutKey.kt",
			packageName: "parity.compose.negative",
		},
		{
			name:        "InjectDispatcher_Positive",
			goRule:      "InjectDispatcher",
			fixture:     "tests/fixtures/positive/coroutines/InjectDispatcher.kt",
			packageName: "parity.inject.positive",
		},
		{
			name:        "InjectDispatcher_Negative",
			goRule:      "InjectDispatcher",
			fixture:     "tests/fixtures/negative/coroutines/InjectDispatcher.kt",
			packageName: "parity.inject.negative",
		},
	}

	tmp := t.TempDir()
	files := writeParityStubs(t, tmp)
	tempPathByName := make(map[string]string, len(cases))
	for i, tc := range cases {
		path := writeCaseSource(t, root, tmp, i, tc)
		files = append(files, path)
		tempPathByName[tc.name] = path
	}

	firResult, err := firchecks.InvokeCached(
		jar,
		files,
		nil,
		[]string{stdlib},
		[]string{"FlowCollectInOnCreate", "ComposeRememberWithoutKey", "InjectDispatcher"},
		"",
		false,
		false,
	)
	if err != nil {
		t.Fatalf("running FIR parity check: %v", err)
	}
	if len(firResult.Crashed) > 0 {
		t.Fatalf("FIR parity check crashed: %v", firResult.Crashed)
	}

	firByPath := map[string][]scanner.Finding{}
	for _, f := range firResult.Findings {
		firByPath[f.File] = append(firByPath[f.File], f)
	}
	allFirFindings := summarizeFindings(firResult.Findings)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			goFindings := runGoRule(t, root, tc.goRule, tc.fixture)
			firFindings := firByPath[tempPathByName[tc.name]]

			goKeys := findingLineCounts(goFindings, tc.goRule)
			firKeys := findingLineCounts(firFindings, tc.goRule)
			if !sameCounts(goKeys, firKeys) {
				t.Fatalf("FIR parity mismatch for %s\nGo:  %v\nFIR: %v\nGo findings:  %s\nFIR findings: %s\nAll FIR findings: %s",
					tc.name, goKeys, firKeys, summarizeFindings(goFindings), summarizeFindings(firFindings), allFirFindings)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("could not find repository root")
		}
		wd = parent
	}
}

func isExecutableJar(path string) bool {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "META-INF/MANIFEST.MF" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return false
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return false
		}
		return strings.Contains(string(data), "Main-Class: dev.krit.fir.MainKt")
	}
	return false
}

func findKotlinStdlib() string {
	if path := os.Getenv("KOTLIN_STDLIB_JAR"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".gradle", "caches", "modules-2", "files-2.1", "org.jetbrains.kotlin", "kotlin-stdlib", "*", "*", "kotlin-stdlib-*.jar"))
	sort.Strings(matches)
	for i := len(matches) - 1; i >= 0; i-- {
		name := filepath.Base(matches[i])
		if strings.Contains(name, "sources") || strings.Contains(name, "javadoc") {
			continue
		}
		return matches[i]
	}
	return ""
}

func writeParityStubs(t *testing.T, dir string) []string {
	t.Helper()
	stubs := map[string]string{
		"Coroutines.kt": `package kotlinx.coroutines

open class CoroutineDispatcher
class CoroutineScope

object Dispatchers {
    val IO: CoroutineDispatcher = CoroutineDispatcher()
    val Default: CoroutineDispatcher = CoroutineDispatcher()
    val Unconfined: CoroutineDispatcher = CoroutineDispatcher()
    val Main: CoroutineDispatcher = CoroutineDispatcher()
}

suspend fun <T> withContext(context: CoroutineDispatcher, block: () -> T): T = block()
fun CoroutineScope.launch(block: () -> Unit) { block() }
fun CoroutineScope.launch(context: CoroutineDispatcher, block: () -> Unit) { block() }
`,
		"Flow.kt": `package kotlinx.coroutines.flow

interface Flow<T> {
    fun collect(action: (T) -> Unit) {}
}
class MutableStateFlow<T>(initial: T) : Flow<T>

fun <T> Flow<T>.collect(action: (T) -> Unit) {}
`,
		"Compose.kt": `package androidx.compose.runtime

annotation class Composable

fun <T> remember(calculation: () -> T): T = calculation()
fun <T> remember(key1: Any?, calculation: () -> T): T = calculation()
`,
		"AndroidOs.kt": `package android.os

class Bundle
`,
		"AppCompat.kt": `package androidx.appcompat.app

import android.os.Bundle
import androidx.lifecycle.Lifecycle

open class AppCompatActivity {
    val lifecycle: Lifecycle = Lifecycle()
    open fun onCreate(savedInstanceState: Bundle?) {}
}
`,
		"Lifecycle.kt": `package androidx.lifecycle

import androidx.appcompat.app.AppCompatActivity
import kotlinx.coroutines.CoroutineScope

class Lifecycle {
    enum class State { STARTED }
}

val AppCompatActivity.lifecycleScope: CoroutineScope
    get() = CoroutineScope()

fun repeatOnLifecycle(state: Lifecycle.State, block: () -> Unit) { block() }
fun Lifecycle.repeatOnLifecycle(state: Lifecycle.State, block: () -> Unit) { block() }
`,
	}

	paths := make([]string, 0, len(stubs))
	for name, source := range stubs {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func writeCaseSource(t *testing.T, root, dir string, index int, tc parityCase) string {
	t.Helper()
	sourcePath := filepath.Join(root, tc.fixture)
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	source := rewritePackage(string(data), tc.packageName)
	if strings.Contains(tc.fixture, "/compose/") {
		source += `

fun buildSeries(dataset: List<Int>): List<Int> = dataset
fun Render(series: List<Int>) {}
`
	}
	path := filepath.Join(dir, fmt.Sprintf("%02d_%s.kt", index, strings.ReplaceAll(tc.name, "-", "_")))
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func rewritePackage(source, packageName string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "package ") {
			lines[i] = "package " + packageName
			return strings.Join(lines, "\n")
		}
	}
	return "package " + packageName + "\n\n" + source
}

func runGoRule(t *testing.T, root, ruleName, fixture string) []scanner.Finding {
	t.Helper()
	file, err := scanner.ParseFile(filepath.Join(root, fixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range v2rules.Registry {
		if rule.ID != ruleName {
			continue
		}
		if rule.Needs.Has(v2rules.NeedsResolver) {
			resolver := typeinfer.NewResolver()
			resolver.IndexFilesParallel([]*scanner.File{file}, 1)
			cols := rules.NewDispatcherV2([]*v2rules.Rule{rule}, resolver).Run(file)
			return cols.Findings()
		}
		cols := rules.NewDispatcherV2([]*v2rules.Rule{rule}).Run(file)
		return cols.Findings()
	}
	t.Fatalf("rule %q not found", ruleName)
	return nil
}

func findingLineCounts(findings []scanner.Finding, ruleName string) map[int]int {
	out := map[int]int{}
	for _, f := range findings {
		if f.Rule != ruleName {
			continue
		}
		out[f.Line]++
	}
	return out
}

func sameCounts(a, b map[int]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if b[k] != av {
			return false
		}
	}
	return true
}

func summarizeFindings(findings []scanner.Finding) string {
	if len(findings) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		parts = append(parts, fmt.Sprintf("%s:%d:%d:%s", filepath.Base(f.File), f.Line, f.Col, f.Rule))
	}
	sort.Strings(parts)
	return "[" + strings.Join(parts, ", ") + "]"
}
