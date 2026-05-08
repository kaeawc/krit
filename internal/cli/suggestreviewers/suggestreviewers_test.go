package suggestreviewers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/proc"
)

func TestBuildSuggestions_AggregatesCallerOwnersAndDirectFileOwners(t *testing.T) {
	root := t.TempDir()

	// Three callers of UserRepository.save: two owned by @team-core, one by
	// @team-extra. The change touches UserRepository.kt itself (direct
	// owner: @team-data) and LoginScreen.kt (direct owner: @team-ui).
	writeKt(t, root, "core/UserRepository.kt", `
package com.acme.core
class UserRepository {
    fun save() {}
}
`)
	writeKt(t, root, "core/LoginScreen.kt", `
package com.acme.core
import com.acme.core.UserRepository
class LoginScreen(private val repo: UserRepository) {
    fun submit() { repo.save() }
}
`)
	writeKt(t, root, "billing/Checkout.kt", `
package com.acme.billing
import com.acme.core.UserRepository
class Checkout(private val repo: UserRepository) {
    fun confirm() { repo.save() }
}
`)
	writeKt(t, root, "extra/Audit.kt", `
package com.acme.extra
import com.acme.core.UserRepository
class Audit(private val repo: UserRepository) {
    fun record() { repo.save() }
}
`)

	// Per CODEOWNERS semantics, the LAST matching rule wins, so the
	// per-file overrides are listed after the directory rules.
	co := ParseCodeowners(`
*                          @org/everyone
core/                      @team-core
billing/                   @team-core
extra/                     @team-extra
core/UserRepository.kt     @team-data
core/LoginScreen.kt        @team-ui
`)

	changed := []string{"core/UserRepository.kt", "core/LoginScreen.kt"}
	got := buildSuggestions(root, co, changed)

	teamScores := map[string]int{}
	for _, t := range got.Teams {
		teamScores[t.Team] = t.Score
	}

	// @team-core owns the LoginScreen.kt and Checkout.kt callers via the
	// core/ and billing/ rules. (LoginScreen.kt has a per-file override to
	// @team-ui, so it doesn't contribute to @team-core's caller count.)
	if teamScores["@team-core"] == 0 {
		t.Errorf("expected @team-core to score caller hits, got %v", teamScores)
	}

	// @team-extra owns 1 caller (extra/Audit.kt).
	if teamScores["@team-extra"] == 0 {
		t.Errorf("expected @team-extra to score, got %v", teamScores)
	}

	// Direct file owners must appear too.
	if teamScores["@team-data"] == 0 {
		t.Errorf("expected @team-data (direct owner of UserRepository.kt) to appear, got %v", teamScores)
	}
	if teamScores["@team-ui"] == 0 {
		t.Errorf("expected @team-ui (direct owner of LoginScreen.kt) to appear, got %v", teamScores)
	}

	// Sorted by score descending.
	for i := 1; i < len(got.Teams); i++ {
		if got.Teams[i-1].Score < got.Teams[i].Score {
			t.Errorf("teams not sorted by score: %+v", got.Teams)
		}
	}

	// Best-recommendation kind for @team-core should be caller-based (it
	// has more callers than direct files); @team-data is direct only.
	for _, ts := range got.Teams {
		switch ts.Team {
		case "@team-core":
			if ts.Best.Kind != "callers" {
				t.Errorf("@team-core best kind = %q, want callers", ts.Best.Kind)
			}
			if !strings.Contains(ts.Best.Symbol, "UserRepository") {
				t.Errorf("@team-core best symbol = %q, want to mention UserRepository", ts.Best.Symbol)
			}
		case "@team-data":
			if ts.Best.Kind != "direct" {
				t.Errorf("@team-data best kind = %q, want direct", ts.Best.Kind)
			}
			if ts.Best.File != "core/UserRepository.kt" {
				t.Errorf("@team-data file = %q", ts.Best.File)
			}
		}
	}
}

func TestRunWith_TextOutput(t *testing.T) {
	root := t.TempDir()
	writeKt(t, root, "core/Repo.kt", `
package com.acme.core
class Repo { fun save() {} }
`)
	writeKt(t, root, "feature/UseRepo.kt", `
package com.acme.feature
import com.acme.core.Repo
class UseRepo(private val r: Repo) { fun go() { r.save() } }
`)
	mustWrite(t, filepath.Join(root, "CODEOWNERS"), `
* @org/everyone
core/    @team-core
feature/ @team-feature
`)

	runner := proc.NewFake().OnExact(
		"git",
		[]string{"diff", "--name-only", "--diff-filter=ACMR", "main"},
		proc.Response{Result: proc.Result{Stdout: []byte("core/Repo.kt\n")}},
	)

	out, err := captureStdout(t, func(stdout, stderr *os.File) int {
		return runWith(runner, root, stdout, stderr, []string{"--base", "main"})
	})
	if err != nil {
		t.Fatalf("runWith: %v", err)
	}
	if !strings.Contains(out, "Suggested reviewers:") {
		t.Errorf("missing header in output: %s", out)
	}
	if !strings.Contains(out, "@team-feature") {
		t.Errorf("expected @team-feature (owns caller of Repo.save) in output: %s", out)
	}
	if !strings.Contains(out, "@team-core") {
		t.Errorf("expected @team-core (direct owner of Repo.kt) in output: %s", out)
	}
}

func TestRunWith_NoCodeownersErrors(t *testing.T) {
	root := t.TempDir()
	runner := proc.NewFake()
	rc := runWith(runner, root, os.Stdout, os.Stderr, nil)
	if rc != 1 {
		t.Errorf("rc = %d, want 1 when CODEOWNERS missing", rc)
	}
	if calls := runner.Calls(); len(calls) != 0 {
		t.Errorf("git should not be called when CODEOWNERS missing, got %d calls", len(calls))
	}
}

func TestRunWith_GitFailureSurfaced(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CODEOWNERS"), "* @x\n")
	runner := proc.NewFake().OnExact(
		"git",
		[]string{"diff", "--name-only", "--diff-filter=ACMR", "bad-ref"},
		proc.Response{Result: proc.Result{ExitCode: 128, Stderr: []byte("fatal: bad revision\n")}},
	)
	rc := runWith(runner, root, os.Stdout, os.Stderr, []string{"--base", "bad-ref"})
	if rc != 1 {
		t.Errorf("rc = %d, want 1 on git failure", rc)
	}
}

func writeKt(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// captureStdout runs fn with a temporary stdout pipe and returns the
// captured text. stderr is discarded into a temp file so test output
// stays clean.
func captureStdout(t *testing.T, fn func(stdout, stderr *os.File) int) (string, error) {
	t.Helper()
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer stdoutR.Close()
	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr-*.log")
	if err != nil {
		stdoutW.Close()
		return "", err
	}
	defer stderrFile.Close()

	done := make(chan []byte, 1)
	go func() {
		var buf []byte
		chunk := make([]byte, 4096)
		for {
			n, err := stdoutR.Read(chunk)
			if n > 0 {
				buf = append(buf, chunk[:n]...)
			}
			if err != nil {
				break
			}
		}
		done <- buf
	}()

	rc := fn(stdoutW, stderrFile)
	stdoutW.Close()
	out := <-done
	if rc != 0 {
		return string(out), &runError{rc: rc}
	}
	return string(out), nil
}

type runError struct{ rc int }

func (e *runError) Error() string { return "non-zero exit" }
