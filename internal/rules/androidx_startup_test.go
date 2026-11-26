package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/manifest"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// An Initializer reached through a base class declared in another file is still an Initializer: the rules see every
// main-source class at once.
func TestStartupRulesFollowCrossFileBaseClass(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"src/main/kotlin/p/Base.kt": `package p

import androidx.startup.Initializer

abstract class BaseInitializer : Initializer<Unit>
`,
		"src/main/kotlin/p/Child.kt": `package p

import android.content.Context

class ChildInitializer(val x: Int) : BaseInitializer() {
    override fun create(context: Context) = Unit
    override fun dependencies() = emptyList<Class<out androidx.startup.Initializer<*>>>()
}
`,
	}
	var files []*scanner.File
	for rel, src := range sources {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	run := func(id string) []scanner.Finding {
		t.Helper()
		for _, r := range api.Registry {
			if r.ID == id {
				collector := scanner.NewFindingCollector(0)
				r.Check(&api.Context{ParsedFiles: files, Manifests: []*manifest.Manifest{}, Collector: collector, Rule: r})
				return collector.Columns().Findings()
			}
		}
		t.Fatalf("rule %s not registered", id)
		return nil
	}
	if got := run("EnsureInitializerNoArgConstr"); len(got) != 1 || filepath.Base(got[0].File) != "Child.kt" {
		t.Errorf("EnsureInitializerNoArgConstr: want ChildInitializer only, got %v", got)
	}
	// The abstract base class is reported too, as in Lint (only interfaces are skipped).
	if got := run("EnsureInitializerMetadata"); len(got) != 2 {
		t.Errorf("EnsureInitializerMetadata: want BaseInitializer and ChildInitializer, got %v", got)
	}
}

// A fully qualified class literal in dependencies() (a.b.Child::class.java parses as a navigation, not a
// callable reference) still makes the child reachable.
func TestStartupQualifiedDependencyLiteral(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src", "main", "kotlin", "p", "Inits.kt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `package p

import android.content.Context
import androidx.startup.Initializer

class Root : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = listOf(p.Child::class.java)
}

class Child : Initializer<Unit> {
    override fun create(context: Context) = Unit
    override fun dependencies(): List<Class<out Initializer<*>>> = emptyList()
}
`
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range api.Registry {
		if r.ID != "EnsureInitializerMetadata" {
			continue
		}
		collector := scanner.NewFindingCollector(0)
		r.Check(&api.Context{ParsedFiles: []*scanner.File{file}, Manifests: []*manifest.Manifest{}, Collector: collector, Rule: r})
		got := collector.Columns().Findings()
		// Root is listed nowhere; Child is reached through Root's qualified class literal.
		if len(got) != 1 || got[0].Line != 6 {
			t.Fatalf("want only Root (line 6), got %v", got)
		}
		return
	}
	t.Fatal("EnsureInitializerMetadata not registered")
}
