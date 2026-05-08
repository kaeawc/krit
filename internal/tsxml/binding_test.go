package tsxml

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestGetLanguage(t *testing.T) {
	lang := GetLanguage()
	if lang == nil {
		t.Fatal("expected GetLanguage() to return non-nil language")
	}
}

func TestXMLParser_AcceptsAsciiNameForms(t *testing.T) {
	parser := sitter.NewParser()
	parser.SetLanguage(GetLanguage())

	cases := []struct {
		name string
		src  string
	}{
		{name: "basic", src: "<root/>"},
		{name: "dash", src: "<a-name/>"},
		{name: "dot", src: "<a.name/>"},
		{name: "digit-after-letter", src: "<a1/>"},
		{name: "underscore", src: "<_name/>"},
		{name: "colon", src: "<ns:name/>"},
		{name: "dot-and-dash", src: "<a-1.2/>"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := parseXML(t, parser, tc.src)
			if err != nil {
				t.Fatalf("expected parse to succeed: %v", err)
			}
			if root.HasError() {
				t.Fatalf("expected XML to parse without errors for %q", tc.src)
			}
		})
	}
}

func parseXML(t *testing.T, parser *sitter.Parser, src string) (*sitter.Node, error) {
	t.Helper()

	tree, err := parser.ParseCtx(context.Background(), nil, []byte(src))
	if err != nil {
		return nil, err
	}
	return tree.RootNode(), nil
}
