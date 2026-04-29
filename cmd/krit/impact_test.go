package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestSimpleName(t *testing.T) {
	cases := map[string]string{
		"com.acme.Foo.bar":      "bar",
		"com.acme.Foo":          "Foo",
		"bar":                   "bar",
		"com.acme.Foo.bar(Int)": "bar",
		"":                      "",
	}
	for in, want := range cases {
		if got := simpleName(in); got != want {
			t.Errorf("simpleName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitImpactArgs(t *testing.T) {
	pos, rest := splitImpactArgs([]string{"com.acme.Foo.bar", "--json", "--from-file", "x.kt", "com.acme.Baz"})
	wantPos := []string{"com.acme.Foo.bar", "com.acme.Baz"}
	if len(pos) != len(wantPos) {
		t.Fatalf("positional = %v, want %v", pos, wantPos)
	}
	for i, p := range pos {
		if p != wantPos[i] {
			t.Errorf("positional[%d] = %q, want %q", i, p, wantPos[i])
		}
	}
	wantRest := []string{"--json", "--from-file", "x.kt"}
	if len(rest) != len(wantRest) {
		t.Fatalf("rest = %v, want %v", rest, wantRest)
	}
	for i, r := range rest {
		if r != wantRest[i] {
			t.Errorf("rest[%d] = %q, want %q", i, r, wantRest[i])
		}
	}
}

func TestComputeImpact(t *testing.T) {
	dir := t.TempDir()
	writeKt(t, dir, "core/Repo.kt", `
package com.acme.core
class UserRepository {
    fun load(): String = "x"
}
`)
	writeKt(t, dir, "feature/ProfileViewModel.kt", `
package com.acme.feature
import com.acme.core.UserRepository
class ProfileViewModel(private val repo: UserRepository) {
    fun show() { repo.load() }
}
`)
	writeKt(t, dir, "feature/Unrelated.kt", `
package com.acme.feature
class Unrelated { fun nothing() = 0 }
`)

	paths, err := scanner.CollectKotlinFiles([]string{dir}, nil)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	files, _ := scanner.ScanFiles(paths, runtime.NumCPU())
	idx := scanner.BuildIndex(files, runtime.NumCPU())

	hits := computeImpact(idx, []string{"com.acme.core.UserRepository.load"})
	if len(hits) == 0 {
		t.Fatalf("expected at least one hit file, got none")
	}
	wantSuffix := filepath.Join("feature", "ProfileViewModel.kt")
	found := false
	for f := range hits {
		if filepath.Base(filepath.Dir(f)) == "feature" && filepath.Base(f) == "ProfileViewModel.kt" {
			found = true
		}
		if filepath.Base(f) == "Unrelated.kt" {
			t.Errorf("Unrelated.kt should not be in hits")
		}
	}
	if !found {
		t.Errorf("expected hit file ending in %s, got %v", wantSuffix, hits)
	}
}

func TestChangedFQNsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := writeKt(t, dir, "Foo.kt", `
package com.acme
class Foo {
    fun bar(): Int = 1
    class Baz
}
`)
	fqns, err := changedFQNsFromFile(path)
	if err != nil {
		t.Fatalf("changedFQNsFromFile: %v", err)
	}
	if len(fqns) == 0 {
		t.Fatalf("expected FQNs, got none")
	}
	hasBar := false
	for _, f := range fqns {
		if f == "com.acme.Foo.bar" {
			hasBar = true
		}
	}
	if !hasBar {
		t.Errorf("expected com.acme.Foo.bar in %v", fqns)
	}
}

func writeKt(t *testing.T, dir, rel, body string) string {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return full
}
