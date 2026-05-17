package di

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestExportJSON(t *testing.T) {
	graph := testDIGraph(t)
	var buf bytes.Buffer
	if err := graph.ExportJSON(&buf, ""); err != nil {
		t.Fatal(err)
	}
	var out ExportGraph
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Bindings) != 2 {
		t.Fatalf("bindings = %d, want 2", len(out.Bindings))
	}
	if out.Bindings[0].FQN != "test.Api" || out.Bindings[0].Deps[0].Target != "test.Disk" {
		t.Fatalf("unexpected export: %+v", out.Bindings[0])
	}
}

func TestExportDOTAndMermaid(t *testing.T) {
	graph := testDIGraph(t)
	var dot, mermaid bytes.Buffer
	if err := graph.ExportDOT(&dot, ""); err != nil {
		t.Fatal(err)
	}
	if err := graph.ExportMermaid(&mermaid, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dot.String(), `"test.Api" -> "test.Disk"`) {
		t.Fatalf("DOT missing edge:\n%s", dot.String())
	}
	if !strings.Contains(mermaid.String(), "-->") {
		t.Fatalf("Mermaid missing edge:\n%s", mermaid.String())
	}
}

func testDIGraph(t *testing.T) *Graph {
	t.Helper()
	file := parseDIFile(t, `
package test
class Api @Inject constructor(val disk: Disk)
class Disk @Inject constructor()
`)
	return BuildGraph([]*scanner.File{file}, nil)
}

func parseDIFile(t *testing.T, content string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Test.kt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return file
}
