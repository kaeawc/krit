package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestAndroidLintPerformanceFileParity checks per-file Performance ports against Android Lint 32.4.1.
//
// Each directory under testdata/android_lint_performance_files holds Kotlin/Java inputs, checks.txt (the issue
// IDs) and lint-32.4.1.txt: what the Lint 32.4.1 CLI reported for those IDs on the same inputs (kotlin-stdlib 2.2.0
// and compiled stubs of the android.support.v7 / androidx RecyclerView classes on the classpath, a JDK home), one
// `path:line:column: ID: message` line per finding. known-false-negatives.txt, when present, lists Lint findings the
// Go path cannot prove from source (a lint-32.4.1.txt line, then ` # reason`); krit must report exactly the remaining
// findings and nothing else.
func TestAndroidLintPerformanceFileParity(t *testing.T) {
	root := filepath.Join("testdata", "android_lint_performance_files")
	cases, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if !c.IsDir() {
			continue
		}
		dir := filepath.Join(root, c.Name())
		t.Run(c.Name(), func(t *testing.T) {
			checks := strings.Split(strings.TrimSpace(readParityFile(t, filepath.Join(dir, "checks.txt"))), ",")
			knownFN := map[string]bool{}
			for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "known-false-negatives.txt"))) {
				finding, _, _ := strings.Cut(line, " # ")
				knownFN[strings.TrimSpace(finding)] = true
			}
			var want []string
			lintSet := map[string]bool{}
			for _, line := range parityLines(readParityFile(t, filepath.Join(dir, "lint-32.4.1.txt"))) {
				lintSet[line] = true
				if !knownFN[line] {
					want = append(want, line)
				}
			}
			for fn := range knownFN {
				if !lintSet[fn] {
					t.Errorf("known-false-negatives.txt lists %q, which Lint does not report", fn)
				}
			}
			got := runParityChecks(t, dir, checks)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from Lint 32.4.1 (minus known false negatives)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
}

func TestSyntheticAccessor_LocalLookalikes(t *testing.T) {
	kotlin := `package p

class Outer {
    private fun helper() = 1

    inner class Shadow {
        private fun helper() = 2
        fun x() = helper()
    }

    inner class LocalFun {
        fun x(): Int {
            fun helper() = 3
            return helper()
        }
    }

    inner class Scoped {
        fun x(sb: StringBuilder) = sb.run { helper() }
    }

    fun lambda() = listOf(1).map { helper() }

    companion object {
        fun fromCompanion(o: Outer) = o.helper()
    }
}
`
	if got := runRuleByNameOnFile(t, "SyntheticAccessor", parseKotlinSource(t, "Outer.kt", kotlin)); len(got) != 0 {
		t.Errorf("Kotlin lookalikes reported: %+v", got)
	}
	java := `package p;

public class Outer {
    private int count;
    private void bump() {}

    class Shadow {
        int count;
        void x() { count++; }
    }

    void captured() {
        int count = 0;
        Runnable r = new Runnable() {
            public void run() { int c = count; }
        };
        Runnable l = () -> bump();
    }

    class Local {
        void x(int count) { int c = count; }
    }
}
`
	if got := runRuleByNameOnFile(t, "SyntheticAccessor", parseJavaSource(t, "Outer.java", java)); len(got) != 0 {
		t.Errorf("Java lookalikes reported: %+v", got)
	}
}

func parseKotlinSource(t *testing.T, name, src string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func parseJavaSource(t *testing.T, name, src string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseJavaFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// Regressions from comparing the port with Lint on kotlinx.coroutines, leakcanary and element-x (and this source
// with the Lint CLI): a companion object's call to a private constructor is skipped like its method calls; past a
// class that extends a class from another file, a distinctive camelCase name still resolves outward while a
// framework-like one does not; a private extension (generic or not) is resolved from the receiver's declared type,
// also when that type is a class of the file without such a member.
func TestSyntheticAccessor_CorpusRegressions(t *testing.T) {
	src := `package p

import java.util.concurrent.atomic.AtomicLong

class Names private constructor(val ids: IntArray) {
    companion object {
        fun create() = Names(IntArray(0))
    }
}

class Scheduler : AtomicLong(0) {
    fun schedule(delta: Long) = addClamping(get(), delta) + computeDeadlineMillis(delta)
    fun later() = start()
}

private fun addClamping(a: Long, b: Long) = a + b
private fun computeDeadlineMillis(d: Long) = d
private fun start() = 0L

class Heap<T> { val size = 0 }
private fun <T> Heap<T>.none(): Boolean = size == 0
private fun <T> java.util.PriorityQueue<T>.drained(): Boolean = isEmpty()
private fun Heap<Int>.total(): Int = size
class User {
    fun idle(heap: Heap<Int>, queue: java.util.PriorityQueue<Int>) = heap.none() && queue.drained() && heap.total() > 0
}
`
	var got []string
	for _, f := range runRuleByNameOnFile(t, "SyntheticAccessor", parseKotlinSource(t, "Sched.kt", src)) {
		got = append(got, f.Message)
	}
	sort.Strings(got)
	// Lint also reports addClamping and start here; krit stops at the AtomicLong supertype for names starting like
	// framework members (add..., start), which a superclass from elsewhere could declare.
	want := []string{
		"Access to `private` method `computeDeadlineMillis` of class `SchedKt` requires synthetic accessor",
		"Access to `private` method `drained` of class `SchedKt` requires synthetic accessor",
		"Access to `private` method `none` of class `SchedKt` requires synthetic accessor",
		"Access to `private` method `total` of class `SchedKt` requires synthetic accessor",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
