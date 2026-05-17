package filefacts

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

const sampleKotlin = `package com.example

import android.util.Log
import kotlinx.coroutines.flow.* as F
import org.slf4j.LoggerFactory

class Greeter(private val name: String) {
    fun greet() {
        Log.d("tag", "hello ${'$'}name")
        val logger = LoggerFactory.getLogger(Greeter::class.java)
        logger.info("hi")
    }
}
`

func parseSample(t *testing.T) *scanner.File {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(path, []byte(sampleKotlin), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return file
}

func TestImports_PopulatesFQNsWildcardsAliases(t *testing.T) {
	file := parseSample(t)
	c := NewCache()
	facts := c.Imports(file)

	if _, ok := facts.FQNs["android.util.Log"]; !ok {
		t.Fatalf("expected android.util.Log in FQNs, got %v", facts.FQNs)
	}
	if _, ok := facts.FQNs["org.slf4j.LoggerFactory"]; !ok {
		t.Fatalf("expected org.slf4j.LoggerFactory in FQNs, got %v", facts.FQNs)
	}
	if _, ok := facts.Wildcards["kotlinx.coroutines.flow.*"]; !ok {
		t.Fatalf("expected kotlinx.coroutines.flow.* in Wildcards, got %v", facts.Wildcards)
	}
	if facts.Aliases["Log"] != "android.util.Log" {
		t.Fatalf("expected alias Log -> android.util.Log, got %q", facts.Aliases["Log"])
	}
	if facts.Aliases["LoggerFactory"] != "org.slf4j.LoggerFactory" {
		t.Fatalf("expected alias LoggerFactory -> org.slf4j.LoggerFactory, got %q", facts.Aliases["LoggerFactory"])
	}
}

func TestImports_HasFQNHandlesWildcards(t *testing.T) {
	file := parseSample(t)
	facts := NewCache().Imports(file)

	if !facts.HasFQN("android.util.Log") {
		t.Fatal("expected HasFQN(android.util.Log) to be true")
	}
	if !facts.HasFQN("kotlinx.coroutines.flow.MutableStateFlow") {
		t.Fatal("expected wildcard import to cover kotlinx.coroutines.flow.MutableStateFlow")
	}
	if facts.HasFQN("kotlinx.coroutines.channels.Channel") {
		t.Fatal("did not expect wildcard import to cover kotlinx.coroutines.channels.Channel")
	}
}

func TestImports_HasAnyPrefix(t *testing.T) {
	file := parseSample(t)
	facts := NewCache().Imports(file)

	if !facts.HasAnyPrefix("org.slf4j.") {
		t.Fatal("expected org.slf4j. prefix to match")
	}
	if !facts.HasAnyPrefix("kotlinx.coroutines.flow.") {
		t.Fatal("expected wildcard to match prefix")
	}
	if facts.HasAnyPrefix("io.reactivex.") {
		t.Fatal("unrelated prefix should not match")
	}
}

func TestImports_NilCacheRecomputes(t *testing.T) {
	file := parseSample(t)
	var c *Cache
	first := c.Imports(file)
	second := c.Imports(file)
	if first == second {
		t.Fatal("nil-cache should recompute, not return same pointer")
	}
	if !first.HasFQN("android.util.Log") || !second.HasFQN("android.util.Log") {
		t.Fatal("nil-cache results should be valid")
	}
}

func TestImports_CachedReturnsSamePointer(t *testing.T) {
	file := parseSample(t)
	c := NewCache()
	a := c.Imports(file)
	b := c.Imports(file)
	if a != b {
		t.Fatal("expected cached call to return identical pointer")
	}
}

func TestReferences_IndexesIdentifiersOutsideHeaders(t *testing.T) {
	file := parseSample(t)
	c := NewCache()
	facts := c.References(file, func(file *scanner.File, idx uint32) string {
		switch file.FlatType(idx) {
		case "simple_identifier", "type_identifier":
			return file.FlatNodeText(idx)
		}
		return ""
	})

	if !facts.NameReferenced("logger") {
		t.Fatal("expected logger local variable to be referenced")
	}
	if !facts.NameReferenced("name") {
		t.Fatal("expected name parameter to be referenced")
	}
	if facts.NameReferenced("DoesNotExist") {
		t.Fatal("did not expect DoesNotExist to be referenced")
	}
}

func TestFileFact_GenericMemoization(t *testing.T) {
	file := parseSample(t)
	c := NewCache()
	calls := 0
	compute := func() int {
		calls++
		return 42
	}
	a := FileFact(c, file, "answer", compute)
	b := FileFact(c, file, "answer", compute)
	if a != 42 || b != 42 {
		t.Fatalf("want 42, got %d / %d", a, b)
	}
	if calls != 1 {
		t.Fatalf("expected one compute, got %d", calls)
	}

	other := FileFact(c, file, "different", func() string { return "x" })
	if other != "x" {
		t.Fatalf("slot independence: got %q", other)
	}

	var nilCache *Cache
	calls = 0
	_ = FileFact(nilCache, file, "answer", compute)
	_ = FileFact(nilCache, file, "answer", compute)
	if calls != 2 {
		t.Fatalf("nil cache should recompute, got %d", calls)
	}
}

func TestStringFact_KeyedByStringAndSlot(t *testing.T) {
	c := NewCache()
	calls := 0
	compute := func() int {
		calls++
		return 99
	}
	a := StringFact(c, "/path/to/dir", "gradle", compute)
	b := StringFact(c, "/path/to/dir", "gradle", compute)
	if a != 99 || b != 99 {
		t.Fatalf("want 99, got %d / %d", a, b)
	}
	if calls != 1 {
		t.Fatalf("expected one compute, got %d", calls)
	}
	_ = StringFact(c, "/other/dir", "gradle", compute)
	if calls != 2 {
		t.Fatalf("different key should miss, got %d", calls)
	}
	_ = StringFact(c, "/path/to/dir", "manifest", compute)
	if calls != 3 {
		t.Fatalf("different slot should miss, got %d", calls)
	}
	var nilCache *Cache
	calls = 0
	_ = StringFact(nilCache, "k", "s", compute)
	_ = StringFact(nilCache, "k", "s", compute)
	if calls != 2 {
		t.Fatalf("nil cache should recompute, got %d", calls)
	}
}

func TestNodeFact_KeyedByIdxAndSlot(t *testing.T) {
	file := parseSample(t)
	c := NewCache()
	calls := 0
	compute := func() int {
		calls++
		return 7
	}
	a := NodeFact(c, file, 5, "metric", compute)
	b := NodeFact(c, file, 5, "metric", compute)
	if a != 7 || b != 7 {
		t.Fatalf("want 7, got %d / %d", a, b)
	}
	if calls != 1 {
		t.Fatalf("expected one compute per (file, idx, slot), got %d", calls)
	}
	_ = NodeFact(c, file, 6, "metric", compute)
	if calls != 2 {
		t.Fatalf("different idx should miss, got %d compute calls", calls)
	}
	_ = NodeFact(c, file, 5, "other", compute)
	if calls != 3 {
		t.Fatalf("different slot should miss, got %d compute calls", calls)
	}
}

func TestFunctionDecl_CachesPerNode(t *testing.T) {
	file := parseSample(t)
	c := NewCache()

	// pick any node id; we just exercise the cache lookup path.
	const fakeIdx uint32 = 1
	calls := 0
	compute := func() *FunctionDeclFact {
		calls++
		return &FunctionDeclFact{Name: "greet", Annotations: map[string]struct{}{}}
	}
	a := c.FunctionDecl(file, fakeIdx, compute)
	b := c.FunctionDecl(file, fakeIdx, compute)
	if calls != 1 {
		t.Fatalf("expected compute to run once, got %d", calls)
	}
	if a != b {
		t.Fatal("expected cached pointer reuse")
	}

	var nilCache *Cache
	calls = 0
	x := nilCache.FunctionDecl(file, fakeIdx, compute)
	y := nilCache.FunctionDecl(file, fakeIdx, compute)
	if calls != 2 {
		t.Fatalf("nil cache should recompute on each call, got %d compute runs", calls)
	}
	if x == y {
		t.Fatal("nil cache should return distinct pointers")
	}
}
