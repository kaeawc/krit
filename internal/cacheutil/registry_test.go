package cacheutil_test

import (
	"errors"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
)

type fakeCache struct {
	name    string
	clearFn func(cacheutil.ClearContext) error
}

func (f *fakeCache) Name() string                              { return f.name }
func (f *fakeCache) Clear(ctx cacheutil.ClearContext) error    { return f.clearFn(ctx) }

func TestMain(m *testing.M) {
	cacheutil.ClearRegistryForTesting()
	m.Run()
}

func TestClearAll_CallsBothEvenIfFirstErrors(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	errA := errors.New("cache A failed")
	errB := errors.New("cache B failed")

	calledA := false
	calledB := false

	cacheutil.Register(&fakeCache{name: "a", clearFn: func(cacheutil.ClearContext) error {
		calledA = true
		return errA
	}})
	cacheutil.Register(&fakeCache{name: "b", clearFn: func(cacheutil.ClearContext) error {
		calledB = true
		return errB
	}})

	err := cacheutil.ClearAll(cacheutil.ClearContext{})
	if !calledA {
		t.Error("Clear() was not called on cache A")
	}
	if !calledB {
		t.Error("Clear() was not called on cache B")
	}
	if err == nil {
		t.Fatal("expected non-nil error from ClearAll")
	}
	if !errors.Is(err, errA) {
		t.Errorf("expected errA in joined error, got: %v", err)
	}
	if !errors.Is(err, errB) {
		t.Errorf("expected errB in joined error, got: %v", err)
	}
}

func TestClearAll_ThreadsRepoDir(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	var got string
	cacheutil.Register(&fakeCache{name: "a", clearFn: func(ctx cacheutil.ClearContext) error {
		got = ctx.RepoDir
		return nil
	}})

	if err := cacheutil.ClearAll(cacheutil.ClearContext{RepoDir: "/some/repo"}); err != nil {
		t.Fatalf("ClearAll: %v", err)
	}
	if got != "/some/repo" {
		t.Errorf("RepoDir not threaded: got %q, want %q", got, "/some/repo")
	}
}

func TestRegister_IdempotentByName(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	first := &fakeCache{name: "myCache", clearFn: func(cacheutil.ClearContext) error { return errors.New("first") }}
	second := &fakeCache{name: "myCache", clearFn: func(cacheutil.ClearContext) error { return nil }}

	cacheutil.Register(first)
	cacheutil.Register(second)

	all := cacheutil.AllRegistered()
	if len(all) != 1 {
		t.Fatalf("expected 1 entry in registry, got %d", len(all))
	}

	if err := all[0].Clear(cacheutil.ClearContext{}); err != nil {
		t.Errorf("expected nil from second registration, got: %v", err)
	}
}

func TestAllRegistered_ReturnsSnapshot(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	cacheutil.Register(&fakeCache{name: "x", clearFn: func(cacheutil.ClearContext) error { return nil }})

	snap := cacheutil.AllRegistered()
	if len(snap) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(snap))
	}

	snap[0] = &fakeCache{name: "injected", clearFn: func(cacheutil.ClearContext) error { return nil }}

	snap2 := cacheutil.AllRegistered()
	if len(snap2) != 1 {
		t.Fatalf("expected 1 entry after mutation, got %d", len(snap2))
	}
	if snap2[0].Name() != "x" {
		t.Errorf("registry was mutated; got name %q, want %q", snap2[0].Name(), "x")
	}
}
