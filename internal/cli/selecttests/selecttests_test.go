package selecttests

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestSelectTests(t *testing.T) {
	root := t.TempDir()
	files := parseFixture(t, root, map[string]string{
		"src/main/kotlin/com/example/Foo.kt": `
package com.example

class Foo {
    fun load(): String = "loaded"
}
`,
		"src/main/kotlin/com/example/Bar.kt": `
package com.example

class Bar {
    fun render(): String = "rendered"
}
`,
		"src/test/kotlin/com/example/FooTest.kt": `
package com.example

class FooTest {
    fun loads() {
        Foo().load()
    }
}
`,
		"src/test/kotlin/com/example/BarTest.kt": `
package com.example

class BarTest {
    fun renders() {
        Bar().render()
    }
}
`,
		"src/test/kotlin/com/example/UnrelatedTest.kt": `
package com.example

class UnrelatedTest {
    fun runs() = "ok"
}
`,
	})

	got := SelectTests(root, files, []string{"src/main/kotlin/com/example/Foo.kt"})
	want := []string{"src/test/kotlin/com/example/FooTest.kt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SelectTests() = %v, want %v", got, want)
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
