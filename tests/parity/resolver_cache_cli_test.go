package parity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// #1778 through the real CLI, findings bundle and delta/affected-set replay
// included: after each edit, a warm run must report what --no-cache does for
// the resolver-backed rules, in-process and through the daemon.
func TestResolverBackedFindingsFollowOtherFilesAcrossWarmRuns(t *testing.T) {
	bin := buildVerdictKrit(t, repoRoot(t))
	for _, daemon := range []bool{false, true} {
		name := "in-process"
		if daemon {
			name = "daemon"
		}
		t.Run(name, func(t *testing.T) { checkResolverCacheEdits(t, bin, daemon) })
	}
}

const (
	rcBaseView  = "package p\n\nimport android.content.Context\nimport android.view.View\n\nopen class BaseView(c: Context) : View(c)\n"
	rcPlainSwap = "package p\n\nimport android.content.Context\nimport android.view.View\n\nopen class BaseView(c: Context) : Any()\n\nopen class Plain(c: Context) : View(c)\n"
	rcNotView   = "package p\n\nimport android.content.Context\n\nopen class BaseView(c: Context)\n"
	rcMidView   = "package p\n\nimport android.content.Context\n\nopen class MidView(c: Context) : BaseView(c) {\n    fun helper(): Int = 1\n}\n"
	rcMyView    = "package p\n\nimport android.content.Context\nimport android.view.MotionEvent\n\nclass MyView(c: Context) : MidView(c) {\n    override fun onTouchEvent(event: MotionEvent): Boolean {\n        return true\n    }\n}\n"
	rcShape     = "package p\n\nsealed class Shape\n\nclass Circle : Shape()\n"
	rcUse       = "package p\n\nfun area(s: Shape): Int = when (s) {\n    is Circle -> 1\n    else -> 0\n}\n"
	rcSquare    = "package p\n\nclass Square : Shape()\n"
	rcUnrelated = "package p\n\nclass Unrelated\n"
)

func checkResolverCacheEdits(t *testing.T, bin string, daemon bool) {
	project := t.TempDir()
	src := filepath.Join(project, "src", "main", "kotlin", "p")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{
		"BaseView.kt": rcBaseView, "MidView.kt": rcMidView, "MyView.kt": rcMyView,
		"Shape.kt": rcShape, "Use.kt": rcUse, "Unrelated.kt": rcUnrelated,
	} {
		write(name, body)
	}
	home := t.TempDir()
	t.Cleanup(func() { stopDelegatedDaemon(t, bin, home, project) })
	run := func(useCache bool) []string {
		t.Helper()
		args := []string{"--no-fir", "--no-type-oracle", "-f", "json", "-q",
			"--enable-rules", "ClickableViewAccessibility,ElseCaseInsteadOfExhaustiveWhen"}
		if !daemon {
			args = append(args, "--no-daemon", "--no-fir-daemon")
		}
		if !useCache {
			args = append(args, "--no-cache")
		}
		cmd := exec.Command(bin, append(args, ".")...)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "HOME="+home)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		var exit *exec.ExitError
		if err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
			t.Fatalf("krit %v: %v\nstderr:\n%s", args, err, stderr.String())
		}
		var report struct {
			Findings []struct {
				File string `json:"file"`
				Line int    `json:"line"`
				Rule string `json:"rule"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("parse krit output: %v\n%s", err, stdout.String())
		}
		var out []string
		for _, f := range report.Findings {
			if f.Rule == "ClickableViewAccessibility" || f.Rule == "ElseCaseInsteadOfExhaustiveWhen" {
				out = append(out, fmt.Sprintf("%s:%d %s", filepath.Base(f.File), f.Line, f.Rule))
			}
		}
		sort.Strings(out)
		return out
	}
	check := func(label string) {
		t.Helper()
		warm, truth := run(true), run(false)
		if strings.Join(warm, "\n") != strings.Join(truth, "\n") {
			t.Fatalf("%s: warm %v, --no-cache %v", label, warm, truth)
		}
	}

	cold := run(true)
	if len(cold) == 0 {
		t.Fatal("the cold run reported none of the resolver-backed findings: the fixture no longer exercises them")
	}
	check("unchanged")
	write("BaseView.kt", rcPlainSwap)
	check("supertypes swapped inside BaseView.kt (delta path)")
	write("BaseView.kt", rcBaseView)
	check("swapped back")
	write("BaseView.kt", rcNotView)
	check("BaseView stops extending View (MyView : MidView : BaseView)")
	write("BaseView.kt", rcBaseView)
	check("BaseView extends View again")
	write("Square.kt", rcSquare)
	check("a new subclass of the sealed Shape")
	write("BaseView.kt", rcNotView)
	write("MidView.kt", strings.Replace(rcMidView, "= 1", "= 2", 1))
	check("BaseView supertype edit plus a MidView body edit")
	write("BaseView.kt", rcBaseView)
	write("MidView.kt", rcMidView)
	check("both reverted")
}
