package scanner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func BenchmarkPerFileAllocationHotspots(b *testing.B) {
	tree, content := benchmarkParseKotlinTree(b, benchmarkArenaSource(200))
	root := tree.RootNode()
	findings := syntheticFindings(5000)

	b.Run("flatten-tree", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(content)))
		for i := 0; i < b.N; i++ {
			flat := flattenTree(root)
			if len(flat.Nodes) == 0 {
				b.Fatal("expected flattened tree to contain nodes")
			}
		}
	})

	b.Run("collect-findings", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			columns := CollectFindings(findings)
			if columns.Len() != len(findings) {
				b.Fatalf("expected %d findings, got %d", len(findings), columns.Len())
			}
		}
	})

	b.Run("file-lifecycle", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			flat := flattenTree(root)
			columns := CollectFindings(findings)
			if len(flat.Nodes) == 0 {
				b.Fatal("expected flattened tree to contain nodes")
			}
			if columns.Len() != len(findings) {
				b.Fatalf("expected %d findings, got %d", len(findings), columns.Len())
			}
		}
	})
}

func benchmarkParseKotlinTree(b *testing.B, src string) (*sitter.Tree, []byte) {
	b.Helper()

	content := []byte(src)
	parser := GetKotlinParser()
	defer PutKotlinParser(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		b.Fatalf("failed to parse Kotlin benchmark source: %v", err)
	}
	return tree, content
}

func benchmarkArenaSource(methods int) string {
	var sb strings.Builder
	sb.Grow(methods * 180)
	sb.WriteString("package bench\n\n")
	sb.WriteString("class ArenaTarget {\n")
	for i := 0; i < methods; i++ {
		fmt.Fprintf(&sb, "    @Deprecated(\"legacy\")\n    fun method%d(input: String): String {\n", i)
		fmt.Fprintf(&sb, "        val values%d = listOf(input, input.trim(), input.lowercase())\n", i)
		fmt.Fprintf(&sb, "        return values%d.filter { it.isNotEmpty() }.joinToString(separator = \":\")\n", i)
		sb.WriteString("    }\n\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}
