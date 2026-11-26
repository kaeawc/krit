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

// slackCacheEdit rewrites one file of the project before the next pair of runs.
type slackCacheEdit struct {
	label, file, body string
}

// checkSlackWarmMatchesNoCache writes files into a project, then after each edit checks that a warm (cached) run
// reports what --no-cache does for rules. Each edit touches only the file that declares a class hierarchy another
// file's findings depend on, the #1778 stale-cache shape.
func checkSlackWarmMatchesNoCache(t *testing.T, files map[string]string, rules []string, edits []slackCacheEdit) {
	t.Helper()
	bin := buildVerdictKrit(t, repoRoot(t))
	project := t.TempDir()
	src := filepath.Join(project, "src", "main", "java", "p")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range files {
		write(name, body)
	}
	home := t.TempDir()
	wanted := map[string]bool{}
	for _, r := range rules {
		wanted[r] = true
	}
	run := func(useCache bool) []string {
		t.Helper()
		args := []string{"--no-fir", "--no-type-oracle", "--no-daemon", "--no-fir-daemon", "-f", "json", "-q",
			"--enable-rules", strings.Join(rules, ",")}
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
			if wanted[f.Rule] {
				out = append(out, fmt.Sprintf("%s:%d %s", filepath.Base(f.File), f.Line, f.Rule))
			}
		}
		sort.Strings(out)
		return out
	}
	if cold := run(true); len(cold) == 0 {
		t.Fatal("the cold run reported nothing: the fixture no longer exercises the rules")
	}
	for _, e := range edits {
		write(e.file, e.body)
		if warm, truth := run(true), run(false); strings.Join(warm, "\n") != strings.Join(truth, "\n") {
			t.Fatalf("%s: warm %v, --no-cache %v", e.label, warm, truth)
		}
	}
}

// Review of #1805 (P1): CastingViewContextToActivity proves a View receiver, explicit or implicit, through a class
// hierarchy declared in another file; editing only that file must not leave a stale finding in the warm run.
func TestCastingViewContextToActivityFollowsOtherFilesAcrossWarmRuns(t *testing.T) {
	const (
		remote    = "package p\n\nimport android.content.Context\nimport android.widget.FrameLayout\n\nopen class CustomRemote(c: Context) : FrameLayout(c)\n"
		notRemote = "package p\n\nimport android.content.Context\n\nopen class CustomRemote(c: Context)\n"
		base      = "package p\n\nimport android.content.Context\nimport android.widget.FrameLayout\n\nopen class Base(c: Context) : FrameLayout(c)\n"
		notBase   = "package p\n\nimport android.content.Context\n\nopen class Base(c: Context)\n"
	)
	checkSlackWarmMatchesNoCache(t, map[string]string{
		"CustomRemote.kt": remote,
		"Base.kt":         base,
		"Use.kt":          "package p\n\nimport android.app.Activity\n\nfun host(cv: CustomRemote): Activity {\n  return cv.getContext() as Activity\n}\n",
		"Sub.kt":          "package p\n\nimport android.app.Activity\nimport android.content.Context\n\nclass Sub(c: Context) : Base(c) {\n  fun h() = getContext() as Activity\n}\n",
	}, []string{"CastingViewContextToActivity"}, []slackCacheEdit{
		{"CustomRemote stops extending FrameLayout (explicit receiver)", "CustomRemote.kt", notRemote},
		{"CustomRemote extends FrameLayout again", "CustomRemote.kt", remote},
		{"Base stops extending FrameLayout (implicit receiver)", "Base.kt", notBase},
		{"Base extends FrameLayout again", "Base.kt", base},
	})
}
