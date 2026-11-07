package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// Expected lines come from running Android Lint 32.4.1 on the same sources.
func TestUnspecifiedImmutableFlagJavaLintParity(t *testing.T) {
	got := findingLines(runRuleByNameOnJava(t, "UnspecifiedImmutableFlag", `package p;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import static android.app.PendingIntent.FLAG_IMMUTABLE;
import static android.app.PendingIntent.FLAG_UPDATE_CURRENT;
public class C {
  static final int K = PendingIntent.FLAG_MUTABLE;
  static final int NOK = 0;
  void f(Context c, Intent i, boolean b) {
    int flags = PendingIntent.FLAG_UPDATE_CURRENT;
    PendingIntent.getActivity(c, 0, i, flags);
    int flags2 = 0;
    if (b) flags2 |= FLAG_IMMUTABLE;
    PendingIntent.getActivity(c, 0, i, flags2);
    PendingIntent.getActivity(c, 0, i, K);
    PendingIntent.getActivity(c, 0, i, NOK);
    PendingIntent.getActivity(c, 0, i, FLAG_UPDATE_CURRENT);
    PendingIntent.getActivity(c, 0, i, FLAG_IMMUTABLE | FLAG_UPDATE_CURRENT);
    PendingIntent.getActivity(c, 0, i, b ? FLAG_IMMUTABLE : 0);
    int fl = 0;
    PendingIntent.getActivity(c, 0, i, fl);
    PendingIntent.getBroadcast(c, 0, i, PendingIntent.FLAG_ONE_SHOT);
    PendingIntent.getForegroundService(c, 0, i, 0);
    PendingIntent.getActivities(c, 0, new Intent[]{i}, 0);
    android.app.PendingIntent.getActivity(c, 0, i, 0);
    Other.getActivity(c, 0, i, 0);
  }
}
class Other { static void getActivity(Context c, int a, Intent i, int f) {} }
`))
	if want := "17,22,23,25,26"; got != want {
		t.Fatalf("lines = %s, want %s", got, want)
	}
}

func TestUnspecifiedImmutableFlagKotlinLintParity(t *testing.T) {
	got := findingLines(runRuleByName(t, "UnspecifiedImmutableFlag", `package p
import android.app.PendingIntent
import android.app.PendingIntent.FLAG_IMMUTABLE
import android.content.Context
import android.content.Intent
const val MY = PendingIntent.FLAG_IMMUTABLE
const val NOTHING = 0
class D {
  val pi = PendingIntent.getActivity(null, 0, Intent(), 0)
  fun f(c: Context, i: Intent, b: Boolean, unknown: Int) {
    PendingIntent.getActivity(c, 0, i, MY)
    PendingIntent.getActivity(c, 0, i, NOTHING)
    PendingIntent.getActivity(c, 0, i, FLAG_IMMUTABLE)
    PendingIntent.getActivity(c, 0, i, if (b) FLAG_IMMUTABLE else 0)
    val fl = 0
    PendingIntent.getActivity(c, 0, i, fl)
    var v = 0
    v = v or FLAG_IMMUTABLE
    PendingIntent.getActivity(c, 0, i, v)
    PendingIntent.getActivity(c, 0, i, unknown)
    PendingIntent.getActivity(c, 0, i, flags = 0, requestCode = 0)
    PendingIntent.getBroadcast(c, 0, i, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
    PendingIntent.getBroadcast(c, 0, i, PendingIntent.FLAG_NO_CREATE)
  }
}
`))
	if want := "9,12,16,21,23"; got != want {
		t.Fatalf("lines = %s, want %s", got, want)
	}
}

// Lint analyzes test sources only for detectors that ask for them, and UnspecifiedImmutableFlag does not.
func TestLintPortsSkipTestSources(t *testing.T) {
	src := `package p;
import android.app.PendingIntent;
public class A {
  void f(android.content.Context c, android.content.Intent i) { PendingIntent.getActivity(c, 0, i, 0); }
}
`
	if got := findingLines(runRuleByNameOnJavaPath(t, "UnspecifiedImmutableFlag", "app/src/test/java/p/A.java", src)); got != "" {
		t.Errorf("test source lines = %s, want none", got)
	}
	if got := findingLines(runRuleByNameOnJavaPath(t, "UnspecifiedImmutableFlag", "app/src/main/java/p/A.java", src)); got != "4" {
		t.Errorf("main source lines = %s, want 4", got)
	}
}

// Lint gates on the module's target SDK, or its minSdk when none is declared: below 23 nothing, below 31 a warning.
func TestUnspecifiedImmutableFlagUsesTheModuleTargetSdk(t *testing.T) {
	src := `package p;
import android.app.PendingIntent;
public class A {
  void f(android.content.Context c, android.content.Intent i) {
    PendingIntent.getActivity(c, 0, i, 0);
    PendingIntent.getActivity(c, 0, i, 0);
  }
}
`
	run := func(gradle string) []scanner.Finding {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "src/main/java/p"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(gradle), 0o644); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "src/main/java/p/A.java")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseJavaFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		return runRuleByNameOnFile(t, "UnspecifiedImmutableFlag", file)
	}
	cases := []struct {
		gradle   string
		count    int
		severity string
	}{
		{"android {\n defaultConfig {\n minSdk = 21\n targetSdk = 22 } }", 0, ""},
		{"android {\n defaultConfig { minSdk = 21\n targetSdk = 28 } }", 2, "warning"},
		{"android {\n defaultConfig { minSdk = 21\n targetSdk = 34 } }", 2, "error"},
		{"android {\n defaultConfig {\n minSdk = 29 } }", 2, "warning"},
		{"android {\n defaultConfig {\n minSdk = 21 } }", 0, ""},
	}
	for _, tc := range cases {
		got := run(tc.gradle)
		if len(got) != tc.count {
			t.Errorf("%s: %d findings, want %d", tc.gradle, len(got), tc.count)
			continue
		}
		for _, f := range got {
			if f.Severity != tc.severity {
				t.Errorf("%s: severity %q, want %q", tc.gradle, f.Severity, tc.severity)
			}
		}
	}
}
