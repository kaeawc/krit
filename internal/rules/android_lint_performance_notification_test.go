package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestAndroidLintPerformanceParity checks the project-scope Performance ports against Android Lint 32.4.1.
//
// Each directory under testdata/android_lint_performance holds Kotlin/Java inputs, checks.txt (the issue IDs) and
// lint-32.4.1.txt: what the Lint 32.4.1 CLI reported for those IDs on the same inputs, one
// `path:line:column: ID: message` line per finding. The Lint project had kotlin-stdlib 2.2.0 and compiled stubs of
// the android.app, android.content and androidx.core.app classes the inputs use on its classpath, and a JDK home.
// The rules run once over all files of the directory, as the parsed-files phase runs them.
func TestAndroidLintPerformanceParity(t *testing.T) {
	root := filepath.Join("testdata", "android_lint_performance")
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
			want := parityLines(readParityFile(t, filepath.Join(dir, "lint-32.4.1.txt")))
			got := runProjectParityChecks(t, dir, checks)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from Lint 32.4.1\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
}

// runProjectParityChecks parses every Kotlin and Java file under dir and runs each parsed-files rule once over all
// of them, returning sorted `path:line:column: ID: message` lines with paths relative to dir.
func runProjectParityChecks(t *testing.T, dir string, checks []string) []string {
	t.Helper()
	files := parseParityDir(t, dir)
	seen := map[string]bool{}
	var out []string
	for _, check := range checks {
		for _, f := range runParsedFilesRule(t, check, files...) {
			rel, _ := filepath.Rel(dir, f.File)
			line := fmt.Sprintf("%s:%d:%d: %s: %s", filepath.ToSlash(rel), f.Line, f.Col, check, f.Message)
			// Lint reports a notification once per launch in the handler; its text report shows each location once.
			if !seen[line] {
				seen[line] = true
				out = append(out, line)
			}
		}
	}
	sort.Strings(out)
	return out
}

func parseParityDir(t *testing.T, dir string) []*scanner.File {
	t.Helper()
	var files []*scanner.File
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		var file *scanner.File
		switch filepath.Ext(path) {
		case ".kt":
			file, err = scanner.ParseFile(context.Background(), path)
		case ".java":
			file, err = scanner.ParseJavaFile(context.Background(), path)
		default:
			return nil
		}
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// notificationFindings writes the sources (relative path -> content) to a temporary directory and runs one rule
// over all of them.
func notificationFindings(t *testing.T, rule string, sources map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	for rel, src := range sources {
		path := filepath.Join(dir, "src", "main", "java", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out []string
	for _, f := range runParsedFilesRule(t, rule, parseParityDir(t, dir)...) {
		out = append(out, fmt.Sprintf("%s:%d:%d", filepath.Base(f.File), f.Line, f.Col))
	}
	sort.Strings(out)
	return out
}

const notificationJavaReceiver = `package p;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

public class Outer {
    public static class Inner extends BroadcastReceiver {
        @Override
        public void onReceive(Context context, Intent intent) {
            Runnable r = () -> context.startActivity(intent);
        }
    }
}
`

// Regression: a nested Java receiver written `Outer.Inner::class.java` (a navigation ending in `::class`, not a
// callable_reference) and the implicit receiver of `with(builder) { }` (whose arguments sit on the inner call).
func TestNotificationTrampoline_NestedReceiverAndWithScope(t *testing.T) {
	sources := map[string]string{
		"p/Outer.java": notificationJavaReceiver,
		"p/Notify.kt": `package p

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat

class Notify(private val context: Context) {
    fun nested(builder: NotificationCompat.Builder) {
        val intent = Intent(context, Outer.Inner::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, 0)
        with(builder) {
            setContentIntent(pending)
        }
    }
}
`,
	}
	if got := notificationFindings(t, "NotificationTrampoline", sources); strings.Join(got, ",") != "Notify.kt:13:13" {
		t.Fatalf("NotificationTrampoline = %q, want [Notify.kt:13:13]", got)
	}
	if got := notificationFindings(t, "LaunchActivityFromNotification", sources); len(got) != 0 {
		t.Fatalf("a found receiver is not LaunchActivityFromNotification, got %q", got)
	}
}

// Lint's class lookup fails for Kotlin receivers, so it reports LaunchActivityFromNotification (not
// NotificationTrampoline) for them, and nothing from addAction.
func TestNotificationTrampoline_KotlinReceiverIsLaunchActivity(t *testing.T) {
	sources := map[string]string{
		"p/Receiver.kt": `package p

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class Receiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        context.startActivity(intent)
    }
}
`,
		"p/Notify.kt": `package p

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat

class Notify(private val context: Context) {
    fun content() {
        val intent = Intent(context, Receiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, 0)
        NotificationCompat.Builder(context, "c").setContentIntent(pending).addAction(0, "a", pending)
    }
}
`,
	}
	if got := notificationFindings(t, "NotificationTrampoline", sources); len(got) != 0 {
		t.Fatalf("NotificationTrampoline = %q, want none", got)
	}
	if got := notificationFindings(t, "LaunchActivityFromNotification", sources); strings.Join(got, ",") != "Notify.kt:12:50" {
		t.Fatalf("LaunchActivityFromNotification = %q, want [Notify.kt:12:50]", got)
	}
}

func TestNotificationTrampoline_LookalikeBuilders(t *testing.T) {
	sources := map[string]string{
		"p/Notify.kt": `package p

import android.app.Notification
import android.app.PendingIntent
import android.content.Context
import android.content.Intent

class NotificationCompat {
    class Builder(c: Context, id: String) {
        fun setContentIntent(p: PendingIntent?) = this
    }
}

class Notify(private val context: Context) {
    fun platform() {
        val pending = PendingIntent.getBroadcast(context, 0, Intent("x"), 0)
        Notification.Builder(context, "c").setContentIntent(pending)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
        // androidx.core.app.NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }
}
`,
	}
	if got := notificationFindings(t, "LaunchActivityFromNotification", sources); len(got) != 0 {
		t.Fatalf("platform and same-file Builder lookalikes are not NotificationCompat.Builder, got %q", got)
	}
}
