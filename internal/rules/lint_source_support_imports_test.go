package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// lintImportsOf reads only the file header: an import-shaped line inside a raw string or comment in the body is not
// an import.
func TestLintImportsOfReadsOnlyTheHeader(t *testing.T) {
	for name, src := range map[string]string{
		"Body.kt": `package test

import dagger.Provides

val source = """
import dagger.Binds
"""

/*
import dagger.Module
*/
class Body
`,
		"Body.java": `package test;

import dagger.Provides;

class Body {
  String source = """
import dagger.Binds;
""";
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			var file *scanner.File
			var err error
			if filepath.Ext(name) == ".java" {
				file, err = scanner.ParseJavaFile(context.Background(), path)
			} else {
				file, err = scanner.ParseFile(context.Background(), path)
			}
			if err != nil {
				t.Fatal(err)
			}
			imp := lintImportsOf(file)
			if imp.bySimple["Provides"] != "dagger.Provides" {
				t.Errorf("header import missing: %v", imp.bySimple)
			}
			for _, simple := range []string{"Binds", "Module"} {
				if full, ok := imp.bySimple[simple]; ok {
					t.Errorf("body line read as an import: %s -> %s", simple, full)
				}
			}
		})
	}
}
