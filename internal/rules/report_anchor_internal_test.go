package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestSymbolAnchorLandsOnDeclarationName(t *testing.T) {
	src := "package demo\n\n@Stable\n@Suppress(\"unused\")\n    class Holder(val a: Int)\n\n@Stable\nfun helper() = 1\n"
	file := parseInlineForInternalTest(t, src)
	class := firstFlatNodeOfType(t, file, "class_declaration", "")
	fn := firstFlatNodeOfType(t, file, "function_declaration", "")
	cases := []struct {
		name      string
		sym       scanner.Symbol
		line, col int
	}{
		{"class", scanner.Symbol{Name: "Holder", Kind: "class", Line: 3, StartByte: int(file.FlatStartByte(class)), EndByte: int(file.FlatEndByte(class))}, 5, 11},
		{"function", scanner.Symbol{Name: "helper", Kind: "function", Line: 7, StartByte: int(file.FlatStartByte(fn)), EndByte: int(file.FlatEndByte(fn))}, 8, 5},
		{"unknown symbol keeps its line", scanner.Symbol{Name: "x", Kind: "class", Line: 4}, 4, 1},
	}
	for _, c := range cases {
		line, col := symbolAnchor(file, c.sym)
		if line != c.line || col != c.col {
			t.Errorf("%s: got %d:%d, want %d:%d", c.name, line, col, c.line, c.col)
		}
	}
}

func TestCommentOffsetPositionTracksColumn(t *testing.T) {
	src := "package demo\n\n/**\n * See [Missing] here\n */\nfun f() {}\n"
	file := parseInlineForInternalTest(t, src)
	comment := firstFlatNodeOfType(t, file, "multiline_comment", "")
	text := file.FlatNodeText(comment)
	offset := len(text[:indexOf(text, "Missing")])
	line, col := commentOffsetPosition(file, comment, offset)
	if line != 4 || col != 9 {
		t.Fatalf("got %d:%d, want 4:9", line, col)
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
