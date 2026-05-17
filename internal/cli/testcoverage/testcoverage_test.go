package testcoverage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestStaticFindings(t *testing.T) {
	root := t.TempDir()
	files := parseFixture(t, root, map[string]string{
		"src/main/kotlin/com/example/Foo.kt": `
package com.example

class Foo {
    fun load(): String = "loaded"
    fun save(): String = "saved"
}
`,
		"src/main/kotlin/com/example/Bar.kt": `
package com.example

class Bar {
    fun render(): String = "rendered"
}
`,
		"src/main/kotlin/com/example/GenericBox.kt": `
package com.example

class GenericBox<T>(val value: T)
`,
		"src/test/kotlin/com/example/FooTest.kt": `
package com.example

class FooTest {
    fun loads() {
        Foo().save()
    }

    fun saves() {
        Foo().save()
    }
}
`,
		"src/test/kotlin/com/example/BarTest.kt": `
package com.example

class BarTest {
    fun unrelated() {
        "nothing"
    }
}
`,
		"src/test/kotlin/com/example/GenericBoxTest.kt": `
package com.example

class GenericBoxTest {
    fun acceptsLambdaOnlyConstruction() {
        GenericBox(value = { "ok" })
    }
}
`,
	})

	findings := StaticFindings(root, files)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(findings), findings)
	}
	got := testCoverageFindingText(findings)
	if !strings.Contains(got, "FooTest.loads->Foo.load") {
		t.Fatalf("missing stale member finding: %s", got)
	}
	if !strings.Contains(got, "BarTest->Bar") {
		t.Fatalf("missing stale class finding: %s", got)
	}
	if strings.Contains(got, "GenericBoxTest") {
		t.Fatalf("generic class reference should be clean: %s", got)
	}
}

func parseFixture(t *testing.T, root string, files map[string]string) []*scanner.File {
	t.Helper()
	var parsed []*scanner.File
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, file)
	}
	return parsed
}

func testCoverageFindingText(findings []Finding) string {
	var b strings.Builder
	for _, finding := range findings {
		b.WriteString(finding.Test)
		b.WriteString("->")
		b.WriteString(finding.Target)
		b.WriteByte('\n')
	}
	return b.String()
}
