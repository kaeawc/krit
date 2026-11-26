package prepush

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installRepo is a throwaway repository with the install script and a stub
// .githooks/pre-push that records the refs it was given.
func installRepo(t *testing.T) (repo, log string) {
	t.Helper()
	root := repoRoot(t)
	repo = t.TempDir()
	log = filepath.Join(t.TempDir(), "hooks.log")
	runIn(t, repo, nil, "git", "init", "-q")
	script, err := os.ReadFile(filepath.Join(root, "scripts", "install-hooks.sh"))
	if err != nil {
		t.Fatal(err)
	}
	writeExec(t, filepath.Join(repo, "scripts", "install-hooks.sh"), string(script))
	writeExec(t, filepath.Join(repo, ".githooks", "pre-push"),
		"#!/bin/sh\nwhile read -r ref rest; do echo \"repo $ref\" >> \"$HOOK_LOG\"; done\n")
	return repo, log
}

func writeExec(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runIn(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	out, err := tryIn(dir, env, "", name, args...)
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return out
}

func tryIn(dir string, env []string, stdin, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"), env...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInstallHooksChainsExistingHook(t *testing.T) {
	requireBashAndGit(t)
	repo, log := installRepo(t)
	hooksDir := filepath.Join(repo, ".git", "hooks")
	// A hook some other tool installed before krit's.
	writeExec(t, filepath.Join(hooksDir, "pre-push"),
		"#!/bin/sh\nwhile read -r ref rest; do echo \"local $ref\" >> \"$HOOK_LOG\"; done\n")

	runIn(t, repo, nil, "bash", "scripts/install-hooks.sh")
	if _, err := os.Stat(filepath.Join(hooksDir, "pre-push.local")); err != nil {
		t.Fatalf("existing hook was not kept as pre-push.local: %v", err)
	}
	// Idempotent: a second install keeps the same .local and one wrapper.
	runIn(t, repo, nil, "bash", "scripts/install-hooks.sh")
	if out, err := tryIn(repo, nil, "", "git", "config", "--get", "core.hooksPath"); err == nil {
		t.Fatalf("install set core.hooksPath: %s", out)
	}

	refs := "refs/heads/a 1111 refs/heads/a 0000\nrefs/heads/b 2222 refs/heads/b 0000\n"
	out, err := tryIn(repo, []string{"HOOK_LOG=" + log}, refs, filepath.Join(hooksDir, "pre-push"), "origin", "url")
	if err != nil {
		t.Fatalf("wrapper failed: %v\n%s", err, out)
	}
	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := "local refs/heads/a\nlocal refs/heads/b\nrepo refs/heads/a\nrepo refs/heads/b\n"
	if string(got) != want {
		t.Errorf("hooks saw\n%s\nwant (existing hook first, both get every ref)\n%s", got, want)
	}

	runIn(t, repo, nil, "bash", "scripts/install-hooks.sh", "--uninstall")
	restored, err := os.ReadFile(filepath.Join(hooksDir, "pre-push"))
	if err != nil || strings.Contains(string(restored), "krit-hook-wrapper") {
		t.Errorf("uninstall did not restore the original hook: %v\n%s", err, restored)
	}
	if _, err := os.Stat(filepath.Join(hooksDir, "pre-push.local")); !os.IsNotExist(err) {
		t.Errorf("uninstall left pre-push.local: %v", err)
	}
}

func TestInstallHooksFailingLocalHookStopsThePush(t *testing.T) {
	requireBashAndGit(t)
	repo, log := installRepo(t)
	hooksDir := filepath.Join(repo, ".git", "hooks")
	writeExec(t, filepath.Join(hooksDir, "pre-push"), "#!/bin/sh\ncat >/dev/null\nexit 7\n")
	runIn(t, repo, nil, "bash", "scripts/install-hooks.sh")
	if _, err := tryIn(repo, []string{"HOOK_LOG=" + log}, "refs/heads/a 1 refs/heads/a 0\n", filepath.Join(hooksDir, "pre-push")); err == nil {
		t.Fatal("a failing pre-existing hook must still block the push")
	}
}

func TestInstallHooksRefusesWithHooksPath(t *testing.T) {
	requireBashAndGit(t)
	repo, _ := installRepo(t)
	runIn(t, repo, nil, "git", "config", "core.hooksPath", ".husky")
	out, err := tryIn(repo, nil, "", "bash", "scripts/install-hooks.sh")
	if err == nil || !strings.Contains(out, "core.hooksPath is set") {
		t.Fatalf("want a refusal naming core.hooksPath, got err=%v:\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "hooks", "pre-push")); !os.IsNotExist(err) {
		t.Error("installed a hook git would ignore")
	}
}
