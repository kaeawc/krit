package scanner

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

func parseKotlin(t *testing.T, src string) (*sitter.Node, []byte) {
	t.Helper()
	content := []byte(src)
	parser := sitter.NewParser()
	parser.SetLanguage(kotlin.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		t.Fatalf("failed to parse Kotlin: %v", err)
	}
	return tree.RootNode(), content
}
