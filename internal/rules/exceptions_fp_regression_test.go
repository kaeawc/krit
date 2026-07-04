package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ruleByIDForExceptionsTest returns the registered rule with the given ID.
func ruleByIDForExceptionsTest(t *testing.T, id string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("rule %q not registered", id)
	return nil
}

// runExceptionsRuleOnSource parses an inline Kotlin/Java source and returns the
// findings produced by the named rule.
func runExceptionsRuleOnSource(t *testing.T, id, ext, src string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "Fixture"+ext)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var (
		file *scanner.File
		err  error
	)
	if ext == ".java" {
		file, err = scanner.ParseJavaFile(context.Background(), p)
	} else {
		file, err = scanner.ParseFile(context.Background(), p)
	}
	if err != nil {
		t.Fatalf("parse %s: %v", p, err)
	}
	rule := ruleByIDForExceptionsTest(t, id)
	if rule.Needs.Has(api.NeedsResolver) {
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		cols := rules.NewDispatcher([]*api.Rule{rule}, resolver).Run(file)
		return cols.Findings()
	}
	cols := rules.NewDispatcher([]*api.Rule{rule}, nil).Run(file)
	return cols.Findings()
}

// TestInstanceOfCheckForExceptionCaughtVariableNarrowing locks the fix that
// narrowing an already-caught exception (the catch parameter, a `when (e)`
// dispatch, or a local unwrapped from the caught variable's cause) is NOT
// flagged, while a type-check on an unrelated value still is.
func TestInstanceOfCheckForExceptionCaughtVariableNarrowing(t *testing.T) {
	cases := []struct {
		name string
		ext  string
		src  string
		want int
	}{
		{
			name: "kotlin catch-param narrowing not flagged",
			ext:  ".kt",
			src: `package p
fun f() {
  try { work() } catch (e: IllegalStateException) {
    if (e is java.io.IOException) { throw RuntimeException(e) } else { throw e }
  }
}
fun work() {}`,
			want: 0,
		},
		{
			name: "kotlin when-dispatch on caught var not flagged",
			ext:  ".kt",
			src: `package p
fun f(): String {
  return try { work() } catch (e: Exception) {
    when (e) {
      is java.io.IOException -> "io"
      is IllegalStateException -> "state"
      else -> "other"
    }
  }
}
fun work(): String = "x"`,
			want: 0,
		},
		{
			name: "kotlin cause-unwrap narrowing not flagged",
			ext:  ".kt",
			src: `package p
fun f() {
  try { work() } catch (e: RuntimeException) {
    val cause = e.cause
    if (cause is InterruptedException) { throw cause }
  }
}
fun work() {}`,
			want: 0,
		},
		{
			name: "kotlin type-check on non-caught value flagged",
			ext:  ".kt",
			src: `package p
fun f(failure: Throwable) {
  try { work() } catch (e: Exception) {
    if (failure is java.io.IOException) { return }
  }
}
fun work() {}`,
			want: 1,
		},
		{
			name: "java catch-param narrowing not flagged",
			ext:  ".java",
			src: `class C {
  void m() throws java.io.IOException {
    try { work(); } catch (java.io.IOException e) {
      if (e instanceof java.net.SocketException) { return; }
      throw e;
    }
  }
  void work() throws java.io.IOException {}
}`,
			want: 0,
		},
		{
			name: "java cause-unwrap narrowing not flagged",
			ext:  ".java",
			src: `class C {
  void m() {
    try { work(); } catch (Exception e) {
      Throwable cause = e.getCause();
      if (cause instanceof java.io.IOException) { return; }
    }
  }
  void work() {}
}`,
			want: 0,
		},
		{
			name: "java type-check on non-caught value flagged",
			ext:  ".java",
			src: `class C {
  void m(Throwable failure) {
    try { work(); } catch (Exception e) {
      if (failure instanceof java.net.SocketException) { return; }
    }
  }
  void work() {}
}`,
			want: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runExceptionsRuleOnSource(t, "InstanceOfCheckForException", tc.ext, tc.src)
			if len(findings) != tc.want {
				t.Errorf("got %d findings, want %d", len(findings), tc.want)
				for _, f := range findings {
					t.Logf("  %d:%d %s", f.Line, f.Col, f.Message)
				}
			}
		})
	}
}

// TestSwallowedExceptionUsageAndRecovery locks the fixes that a catch which
// passes/wraps/inspects the caught exception, or performs real recovery without
// touching it, is NOT a swallow — while a genuinely empty-but-for-a-comment or
// log-the-string-and-drop-the-cause catch still is.
func TestSwallowedExceptionUsageAndRecovery(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "passes whole exception to unknown function not swallowed",
			src: `package p
class C {
  fun f(): String {
    val out = StringBuilder()
    try { w() } catch (t: Throwable) {
      out.append(convertThrowableToString(t))
      return out.toString()
    }
    return "ok"
  }
  fun convertThrowableToString(t: Throwable): String = ""
  fun w() {}
}`,
			want: 0,
		},
		{
			name: "labeled return wrapping exception not swallowed",
			src: `package p
class C {
  fun f(): String = run {
    try { w() } catch (e: java.io.IOException) {
      return@run wrap(e)
    }
    "ok"
  }
  fun wrap(e: Throwable): String = ""
  fun w() {}
}`,
			want: 0,
		},
		{
			name: "when on e.cause as tail expression not swallowed",
			src: `package p
class C {
  fun f(): String {
    return run {
      try { w() } catch (e: java.util.concurrent.ExecutionException) {
        when (val cause = e.cause) {
          is java.io.IOException -> netErr(cause)
          else -> appErr(cause)
        }
      }
    }
  }
  fun netErr(c: Throwable?): String = ""
  fun appErr(c: Throwable?): String = ""
  fun w() {}
}`,
			want: 0,
		},
		{
			name: "fallback assignment recovery not swallowed",
			src: `package p
class C {
  var cached: String? = "x"
  fun f() { try { w() } catch (e: Exception) { cached = null } }
  fun w() {}
}`,
			want: 0,
		},
		{
			name: "early return recovery not swallowed",
			src: `package p
class C {
  fun f(): Boolean {
    try { w() } catch (e: java.io.IOException) { return false }
    return true
  }
  fun w() {}
}`,
			want: 0,
		},
		{
			name: "truly empty catch deferred to EmptyCatchBlock",
			src: `package p
class C {
  fun f() { try { w() } catch (e: IllegalArgumentException) {} }
  fun w() {}
}`,
			want: 0,
		},
		{
			// The outer catch performs recovery (a guarded retry) and never
			// touches its own `e`; the inner catch rethrows its own `e`. Neither
			// is a swallow, and the inner catch's `e` must not be counted
			// against the outer catch.
			name: "nested catch reference does not count for outer catch",
			src: `package p
class C {
  fun f(url: String) {
    try { launch() } catch (e: RuntimeException) {
      if (url == "intent") {
        try { launch() } catch (inner: RuntimeException) { throw inner }
      }
    }
  }
  fun launch() {}
}`,
			want: 0,
		},
		{
			name: "log-the-string-and-drop-the-cause still swallowed",
			src: `package p
class C {
  fun f() { try { w() } catch (e: java.io.IOException) { Log.w(TAG, "Cannot restore.") } }
  fun w() {}
  companion object { const val TAG = "T" }
}`,
			want: 1,
		},
		{
			name: "comment-only swallow still flagged",
			src: `package p
class C {
  fun f() {
    try { w() } catch (e: Exception) {
      // ignored
    }
  }
  fun w() {}
}`,
			want: 1,
		},
		{
			name: "throw new dropping cause still swallowed",
			src: `package p
class C {
  fun f() {
    try { w() } catch (e: IllegalStateException) {
      throw IllegalArgumentException(e.message)
    }
  }
  fun w() {}
}`,
			want: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runExceptionsRuleOnSource(t, "SwallowedException", ".kt", tc.src)
			if len(findings) != tc.want {
				t.Errorf("got %d findings, want %d", len(findings), tc.want)
				for _, f := range findings {
					t.Logf("  %d:%d %s", f.Line, f.Col, f.Message)
				}
			}
		})
	}
}
