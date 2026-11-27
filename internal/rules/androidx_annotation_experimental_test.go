package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// A file-level opt-in covers every use in the file, and a same-named local function is not the marked API.
func TestUnsafeOptInUsageFileOptInAndLookalike(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"src/main/kotlin/a/Marker.kt": `package a

import androidx.annotation.RequiresOptIn

@RequiresOptIn
annotation class Exp

@Exp
fun marked() = 1
`,
		"src/main/kotlin/b/FileOptIn.kt": `@file:androidx.annotation.OptIn(markerClass = [a.Exp::class])
package b

import a.marked

fun use() = marked()
`,
		"src/main/kotlin/c/Lookalike.kt": `package c

fun marked() = 2

fun use() = marked()
`,
		"src/main/kotlin/d/Unopted.kt": `package d

import a.marked

fun use() = marked()
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
		f, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	for _, r := range api.Registry {
		if r.ID != "UnsafeOptInUsageError" {
			continue
		}
		collector := scanner.NewFindingCollector(0)
		r.Check(&api.Context{ParsedFiles: files, Collector: collector, Rule: r})
		got := collector.Columns().Findings()
		if len(got) != 1 || filepath.Base(got[0].File) != "Unopted.kt" {
			t.Fatalf("want only d/Unopted.kt, got %v", got)
		}
		return
	}
	t.Fatal("UnsafeOptInUsageError not registered")
}
