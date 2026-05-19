package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type countingDecompiler struct {
	calls int32
	out   string
	err   error
}

func (c *countingDecompiler) Decompile(jarPath, fqn string) (string, error) {
	atomic.AddInt32(&c.calls, 1)
	return c.out, c.err
}

func TestDecompileCacheHitsDiskOnSecondCall(t *testing.T) {
	tmp := t.TempDir()
	jar := filepath.Join(tmp, "fake.jar")
	if err := os.WriteFile(jar, []byte("PK\x03\x04 fake jar bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := &countingDecompiler{out: "package x\nclass Y\n"}
	cache := NewDecompileCache(filepath.Join(tmp, "cache"), src)

	got1, err := cache.Get(jar, "x.Y")
	if err != nil {
		t.Fatal(err)
	}
	if got1 != src.out {
		t.Fatalf("got %q want %q", got1, src.out)
	}
	got2, err := cache.Get(jar, "x.Y")
	if err != nil {
		t.Fatal(err)
	}
	if got2 != src.out {
		t.Fatalf("second call got %q want %q", got2, src.out)
	}
	if atomic.LoadInt32(&src.calls) != 1 {
		t.Fatalf("expected 1 underlying decompile call, got %d", src.calls)
	}
}

func TestDecompileCacheMissingJAR(t *testing.T) {
	tmp := t.TempDir()
	src := &countingDecompiler{out: "stub"}
	cache := NewDecompileCache(filepath.Join(tmp, "cache"), src)

	got, err := cache.Get("/no/such.jar", "a.B")
	if err != nil {
		t.Fatal(err)
	}
	if got != "stub" {
		t.Fatalf("got %q want stub", got)
	}
	if !cache.JARMissing("/no/such.jar") {
		t.Fatal("expected JARMissing to be true")
	}
}

func TestRenderSignatureStubFromOracleClass(t *testing.T) {
	cls := &Class{
		FQN:        "kotlinx.coroutines.CoroutineScope",
		Kind:       "interface",
		Visibility: "public",
		Members: []*Member{
			{Name: "coroutineContext", Kind: "property", ReturnType: "kotlin.coroutines.CoroutineContext", Visibility: "public", IsAbstract: true},
		},
	}
	out := RenderSignatureStub("/cache/kotlinx-coroutines-core-1.7.3.jar", cls.FQN, cls)
	for _, want := range []string{
		"package kotlinx.coroutines",
		"interface CoroutineScope",
		"abstract val coroutineContext: kotlin.coroutines.CoroutineContext",
		"// Source: /cache/kotlinx-coroutines-core-1.7.3.jar",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stub missing %q\n--\n%s", want, out)
		}
	}
}

func TestRenderSignatureStubUnknownFQN(t *testing.T) {
	out := RenderSignatureStub("", "com.example.Mystery", nil)
	if !strings.Contains(out, "package com.example") {
		t.Errorf("missing package: %s", out)
	}
	if !strings.Contains(out, "class Mystery") {
		t.Errorf("missing class header: %s", out)
	}
	if !strings.Contains(out, "(unresolved classpath entry)") {
		t.Errorf("missing unresolved marker: %s", out)
	}
}

func TestSignatureStubDecompiler(t *testing.T) {
	d := &SignatureStubDecompiler{
		Lookup: func(fqn string) *Class {
			if fqn == "p.Q" {
				return &Class{FQN: fqn, Kind: "class"}
			}
			return nil
		},
	}
	out, err := d.Decompile("/x.jar", "p.Q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "class Q") {
		t.Errorf("missing class Q: %s", out)
	}
}

func TestWriteDecompiledSourceAtomicCreatesParentDirAndWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "p", "Q.kt")
	if err := writeDecompiledSourceAtomic(path, []byte("class Q\n")); err != nil {
		t.Fatalf("writeDecompiledSourceAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "class Q\n" {
		t.Errorf("content = %q, want %q", string(data), "class Q\n")
	}
	// fsutil.WriteFileAtomic renames the tempfile into place atomically;
	// after a successful return no dot-prefixed temp entries should
	// remain alongside the target.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") && e.Name() != filepath.Base(path) {
			t.Errorf("leftover temp entry after atomic write: %s", e.Name())
		}
	}
}

func TestWriteDecompiledSourceAtomicOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Q.kt")
	if err := os.WriteFile(path, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeDecompiledSourceAtomic(path, []byte("fresh\n")); err != nil {
		t.Fatalf("writeDecompiledSourceAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fresh\n" {
		t.Errorf("overwrite content = %q, want %q", string(data), "fresh\n")
	}
}

func TestFallbackDecompilerFallsBackOnPrimaryError(t *testing.T) {
	primary := &countingDecompiler{err: os.ErrNotExist}
	fallback := &countingDecompiler{out: "fallback"}
	d := FallbackDecompiler{Primary: primary, Fallback: fallback}

	got, err := d.Decompile("/x.jar", "p.Q")
	if err != nil {
		t.Fatal(err)
	}
	if got != "fallback" {
		t.Fatalf("got %q", got)
	}
	if atomic.LoadInt32(&primary.calls) != 1 || atomic.LoadInt32(&fallback.calls) != 1 {
		t.Fatalf("unexpected calls: primary=%d fallback=%d", primary.calls, fallback.calls)
	}
}
