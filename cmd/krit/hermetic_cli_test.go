package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type scopeFinding struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Rule   string `json:"rule"`
}

// scanFindingKeys runs a Go-only scan of the Android playground and returns
// its findings as sorted "rule file:line:col" keys.
func scanFindingKeys(t *testing.T, extra ...string) []string {
	t.Helper()
	args := append([]string{"--no-daemon", "--no-cache", "--no-type-oracle", "-f", "json", "-q"}, extra...)
	args = append(args, "../../playground/android-app/")
	stdout, stderr, code := runKrit(t, args...)
	if code > 1 {
		t.Fatalf("krit %v exited %d: %s", extra, code, stderr)
	}
	var report struct {
		Findings []scopeFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON for %v: %v", extra, err)
	}
	keys := make([]string, len(report.Findings))
	for i, f := range report.Findings {
		keys[i] = fmt.Sprintf("%s %s:%d:%d", f.Rule, f.File, f.Line, f.Column)
	}
	sort.Strings(keys)
	return keys
}

// ruleScopes maps every rule ID to its scope group from --list-rules.
func ruleScopes(t *testing.T) map[string]string {
	t.Helper()
	stdout, stderr, code := runKrit(t, "--list-rules", "--format", "json")
	if code != 0 {
		t.Fatalf("--list-rules exited %d: %s", code, stderr)
	}
	var doc struct {
		Rules []struct {
			ID    string `json:"id"`
			Scope string `json:"scope"`
		} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, r := range doc.Rules {
		out[r.ID] = r.Scope
	}
	return out
}

// A file-scope shard plus a project-scope shard report exactly the findings
// of an unsharded scan, and each shard reports only its own rules.
func TestScopeShardsPartitionFindings(t *testing.T) {
	full := scanFindingKeys(t)
	fileShard := scanFindingKeys(t, "--only-scope=file")
	projectShard := scanFindingKeys(t, "--only-scope=project")
	if len(fileShard) == 0 || len(projectShard) == 0 {
		t.Fatalf("file=%d project=%d findings, want both non-empty on the playground", len(fileShard), len(projectShard))
	}

	scopes := ruleScopes(t)
	for shard, keys := range map[string][]string{"file": fileShard, "project": projectShard} {
		for _, k := range keys {
			rule := strings.SplitN(k, " ", 2)[0]
			if scopes[rule] != shard {
				t.Errorf("--only-scope=%s reported %s, a %q rule", shard, k, scopes[rule])
			}
		}
	}

	merged := append(append([]string{}, fileShard...), projectShard...)
	sort.Strings(merged)
	if strings.Join(merged, "\n") != strings.Join(full, "\n") {
		t.Errorf("file+project shards (%d findings) differ from the full scan (%d findings)", len(merged), len(full))
	}
}

func TestScopeFlagRejectsUnknownScope(t *testing.T) {
	_, stderr, code := runKrit(t, "--no-daemon", "--only-scope=modules", "../../playground/android-app/")
	if code != 2 || !strings.Contains(stderr, "unknown rule scope") {
		t.Fatalf("exit %d, stderr %q; want exit 2 naming the unknown scope", code, stderr)
	}
}

// copyTree copies the regular files under src into a new temp directory.
func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
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
	return dst
}

// reproducibleScan runs a --reproducible JSON scan of dir with its own state
// directory and returns stdout.
func reproducibleScan(t *testing.T, dir, stateDir string, extra ...string) []byte {
	t.Helper()
	args := append([]string{"--no-fir", "--no-daemon", "--no-type-oracle", "--reproducible", "-f", "json"}, extra...)
	cmd := exec.Command(binPath, append(args, dir)...)
	cmd.Env = append(os.Environ(), "KRIT_STATE_DIR="+stateDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatalf("krit %v: %v\n%s", extra, err, stderr.String())
		}
	}
	return stdout.Bytes()
}

// --reproducible JSON is byte-identical across a cold and a warm cache, a
// different worker count, --perf, and a relocated copy of the tree.
func TestReproducibleJSONIsByteIdentical(t *testing.T) {
	src := "../../playground/kotlin-webservice"
	first, second := copyTree(t, src), copyTree(t, src)
	state := t.TempDir()

	cold := reproducibleScan(t, first, state, "-j", "1")
	warm := reproducibleScan(t, first, state, "-j", "4", "--perf")
	moved := reproducibleScan(t, second, t.TempDir())

	var report map[string]any
	if err := json.Unmarshal(cold, &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if findings, _ := report["findings"].([]any); len(findings) == 0 {
		t.Fatal("expected findings on the playground")
	}
	for _, key := range []string{"cache", "caches", "cacheBudget", "perfTiming"} {
		if _, ok := report[key]; ok {
			t.Errorf("reproducible report has %q", key)
		}
	}
	if !bytes.Equal(cold, warm) {
		t.Errorf("cold and warm reproducible reports differ")
	}
	if !bytes.Equal(cold, moved) {
		t.Errorf("reproducible report changed when the tree moved")
	}
}
