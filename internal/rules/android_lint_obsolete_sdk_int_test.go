package rules_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// The fixtures under testdata/obsolete_sdk_int and their .lint files come from running Android Lint 32.4.1 (minSdk 21)
// on the same sources; the .lint files hold its ObsoleteSdkInt reports as "line:col message".
const obsoleteSdkIntTestdata = "testdata/obsolete_sdk_int"

func readObsoleteSdkIntFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(obsoleteSdkIntTestdata, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// obsoleteSdkIntModule writes a module with the given minSdk declaration and source file and returns the file path.
func obsoleteSdkIntModule(t *testing.T, gradle, rel, source string) string {
	t.Helper()
	root := t.TempDir()
	if gradle != "" {
		if err := os.WriteFile(filepath.Join(root, "build.gradle.kts"), []byte(gradle), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// obsoleteSdkIntReports runs the source rule on a file and lists its findings as sorted "line:col message".
func obsoleteSdkIntReports(t *testing.T, path string) []string {
	t.Helper()
	var file *scanner.File
	var err error
	if strings.HasSuffix(path, ".java") {
		file, err = scanner.ParseJavaFile(context.Background(), path)
	} else {
		file, err = scanner.ParseFile(context.Background(), path)
	}
	if err != nil {
		t.Fatal(err)
	}
	findings := runRuleByNameOnFile(t, "ObsoleteSdkInt", file)
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, fmt.Sprintf("%d:%d %s", f.Line, f.Col, f.Message))
	}
	sortReports(out)
	return out
}

// sortReports orders "line:col message" entries by line, then column, then message.
func sortReports(reports []string) {
	key := func(s string) (line, col int, msg string) {
		pos, msg, _ := strings.Cut(s, " ")
		_, _ = fmt.Sscanf(pos, "%d:%d", &line, &col)
		return line, col, msg
	}
	sort.SliceStable(reports, func(i, j int) bool {
		li, ci, mi := key(reports[i])
		lj, cj, mj := key(reports[j])
		if li != lj {
			return li < lj
		}
		if ci != cj {
			return ci < cj
		}
		return mi < mj
	})
}

func diffReports(got, want []string) string {
	have := map[string]int{}
	for _, s := range want {
		have[s]++
	}
	var extra, missing []string
	for _, s := range got {
		if have[s] > 0 {
			have[s]--
		} else {
			extra = append(extra, s)
		}
	}
	for _, s := range want {
		if have[s] > 0 {
			have[s]--
			missing = append(missing, s)
		}
	}
	return fmt.Sprintf("only krit: %q\nonly Lint: %q", extra, missing)
}

func TestObsoleteSdkIntSourceLintParity(t *testing.T) {
	for _, name := range []string{
		"basic.kt", "basic.java", "matrix.kt", "contexts.kt", "annotations.kt", "annotations.java", "operands.kt",
		"operands.java", "unknown.kt", "whens.kt", "nested.kt", "nested.java", "negation.kt", "negation.java",
	} {
		t.Run(name, func(t *testing.T) {
			dir := "kotlin"
			if strings.HasSuffix(name, ".java") {
				dir = "java"
			}
			path := obsoleteSdkIntModule(t, readObsoleteSdkIntFile(t, "build.gradle.kts"), filepath.Join("src/main", dir, "p", name),
				readObsoleteSdkIntFile(t, name))
			got := obsoleteSdkIntReports(t, path)
			want := strings.Split(strings.TrimSpace(readObsoleteSdkIntFile(t, name+".lint")), "\n")
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("ObsoleteSdkInt differs from Lint on %s\n%s", name, diffReports(got, want))
			}
		})
	}
}

// Without a minSdk only checks that need none can report: an annotation lower than the one of the declaration around
// it. A plain comparison stays silent because the module's level is unknown.
func TestObsoleteSdkIntNeedsMinSdkForPlainChecks(t *testing.T) {
	src := `package p

import android.os.Build
import androidx.annotation.RequiresApi

@RequiresApi(23)
class A {
    @RequiresApi(19)
    fun f() {
        if (Build.VERSION.SDK_INT >= 19) println()
        if (Build.VERSION.SDK_INT >= 23) println()
    }
}
`
	path := obsoleteSdkIntModule(t, "", "src/main/kotlin/p/A.kt", src)
	got := obsoleteSdkIntReports(t, path)
	want := []string{"8:5 Unnecessary; `SDK_INT` is always >= 23 from outer annotation (`@RequiresApi(23)`)"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("reports differ\n%s", diffReports(got, want))
	}
}

// The Kotlin grammar groups `!a || b` as `!(a || b)`; the rule reads the negation as belonging to the first operand.
func TestObsoleteSdkIntRepairsMisgroupedNegation(t *testing.T) {
	src := `package p

import android.os.Build

fun x() = true

fun f() {
    if (!x() || Build.VERSION.SDK_INT >= 19) println()
    if ((!x()) || Build.VERSION.SDK_INT >= 19) println()
    if (!(Build.VERSION.SDK_INT >= 23) || Build.VERSION.SDK_INT < 24) println()
}
`
	path := obsoleteSdkIntModule(t, readObsoleteSdkIntFile(t, "build.gradle.kts"), "src/main/kotlin/p/A.kt", src)
	got := obsoleteSdkIntReports(t, path)
	want := []string{
		"8:17 Unnecessary; `SDK_INT` is always >= 21",
		"9:19 Unnecessary; `SDK_INT` is always >= 21",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("reports differ\n%s", diffReports(got, want))
	}
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// lintProjectReports lists the golden lines of project.lint whose path starts with prefix, as "path:line:col message".
func lintProjectReports(t *testing.T, prefix string) []string {
	t.Helper()
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(readObsoleteSdkIntFile(t, "project.lint")), "\n") {
		if strings.HasPrefix(l, prefix) {
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}

func TestObsoleteSdkIntResourceLintParity(t *testing.T) {
	project := filepath.Join(t.TempDir(), "module")
	copyTree(t, filepath.Join(obsoleteSdkIntTestdata, "project"), project)
	idx, err := android.ScanResourceDir(filepath.Join(project, "res"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range runResourceRule(findResourceRule(t, "ObsoleteSdkIntResource"), idx) {
		rel, _ := filepath.Rel(project, f.File)
		got = append(got, fmt.Sprintf("%s:%d:%d %s", filepath.ToSlash(rel), f.Line, f.Col, f.Message))
	}
	sort.Strings(got)
	want := lintProjectReports(t, "res/")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ObsoleteSdkIntResource differs from Lint\n%s", diffReports(got, want))
	}
}

func TestObsoleteSdkIntManifestLintParity(t *testing.T) {
	xml := readObsoleteSdkIntFile(t, "project/AndroidManifest.xml")
	var got []string
	for _, f := range runManifestRule(findManifestRule(t, "ObsoleteSdkIntManifest"), parseManifestForTest(t, xml)) {
		got = append(got, fmt.Sprintf("AndroidManifest.xml:%d:%d %s", f.Line, f.Col, f.Message))
	}
	sort.Strings(got)
	want := lintProjectReports(t, "AndroidManifest.xml:")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ObsoleteSdkIntManifest differs from Lint\n%s", diffReports(got, want))
	}
}
