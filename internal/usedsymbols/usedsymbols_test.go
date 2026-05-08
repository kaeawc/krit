package usedsymbols

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func parseSource(t *testing.T, src string) *scanner.File {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Foo.kt")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return f
}

func has(syms []Symbol, fqn string) bool {
	for _, s := range syms {
		if s.FQN == fqn {
			return true
		}
	}
	return false
}

func TestImportsAreEmittedAsSymbols(t *testing.T) {
	src := `package com.acme.feature

import com.acme.core.UserRepository
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

class Foo(private val repo: UserRepository) {
    fun load(): Flow<UserRepository.Result> = repo.load().map { it }
}
`
	f := parseSource(t, src)
	syms := Extract(f)

	want := []string{
		"com.acme.core.UserRepository",
		"com.acme.core.UserRepository.Result",
		"kotlinx.coroutines.flow.Flow",
		"kotlinx.coroutines.flow.map",
	}
	for _, w := range want {
		if !has(syms, w) {
			t.Errorf("missing symbol %q in %v", w, syms)
		}
	}
}

func TestSamePackageIsFiltered(t *testing.T) {
	src := `package com.acme.same

class Bar
class Baz {
    val b: Bar = Bar()
}
`
	f := parseSource(t, src)
	syms := Extract(f)
	for _, s := range syms {
		if s.FQN == "com.acme.same.Bar" {
			t.Errorf("same-package symbol leaked: %v", s)
		}
	}
}

func TestAliasResolves(t *testing.T) {
	src := `package com.acme.feature

import com.acme.core.UserRepository as Repo

class Foo(private val r: Repo)
`
	f := parseSource(t, src)
	syms := Extract(f)
	if !has(syms, "com.acme.core.UserRepository") {
		t.Errorf("alias not resolved: %v", syms)
	}
}

func TestAnnotationEmittedAsAnnotationKind(t *testing.T) {
	src := `package com.acme.feature

import javax.inject.Inject

class Foo @Inject constructor()
`
	f := parseSource(t, src)
	syms := Extract(f)
	found := false
	for _, s := range syms {
		if s.FQN == "javax.inject.Inject" {
			found = true
			if s.Kind != "annotation" && s.Kind != "class" {
				t.Errorf("unexpected kind for annotation: %v", s)
			}
		}
	}
	if !found {
		t.Errorf("annotation not extracted: %v", syms)
	}
}
