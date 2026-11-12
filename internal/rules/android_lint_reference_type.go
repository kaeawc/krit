package rules

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/android"
)

// This file is the element half of Lint's ReferenceType (DuplicateResourceDetector.TYPE_MISMATCH): a <string>, <color>,
// <dimen>, <bool>, <integer>, <drawable>, <fraction> or <attr> whose text is a reference to a resource of another type.
// The `<item name= type=>` half works on the resource index in ReferenceTypeRule.

// referenceTagTypes maps a values element to the resource type its text must reference.
var referenceTagTypes = map[string]string{
	"string": "string", "color": "color", "dimen": "dimen", "bool": "bool", "integer": "integer",
	"drawable": "drawable", "fraction": "fraction", "attr": "attr",
}

// referenceURLRe is ResourceUrl.parse for `@[+][*][package:]type/name`.
var referenceURLRe = regexp.MustCompile(`^@(\+?)\*?(?:[A-Za-z_][\w.]*:)?([a-z]+)/([^/\s]+)$`)

// resourceURLType is the type of a resource reference, or "" when the text is not a valid one.
func resourceURLType(text string) string {
	m := referenceURLRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	if m[1] != "" && m[2] != "id" {
		return ""
	}
	if _, ok := cycleDisplayNames[m[2]]; !ok {
		return ""
	}
	return m[2]
}

// referenceTypeFinding is one mismatching element.
type referenceTypeFinding struct {
	file      string
	line, col int
	msg       string
}

// hasTextReference reports whether any element text starts with '@' after optional whitespace.
func hasTextReference(data []byte) bool {
	for rest := data; ; {
		i := bytes.IndexByte(rest, '>')
		if i < 0 {
			return false
		}
		rest = rest[i+1:]
		j := 0
		for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t' || rest[j] == '\n' || rest[j] == '\r') {
			j++
		}
		if j < len(rest) && rest[j] == '@' {
			return true
		}
	}
}

// referenceTypeFileFindings checks the elements of one parsed values file.
func referenceTypeFileFindings(path string, root *android.XMLNode) []referenceTypeFinding {
	if root.Tag != "resources" {
		return nil
	}
	var out []referenceTypeFinding
	for _, el := range root.Children {
		typ := referenceTagTypes[el.Tag]
		if typ == "" || xmlToolsIgnores(el, "ReferenceType") || el.Text == "" {
			continue
		}
		text := strings.TrimLeft(el.Text, " \t\r\n")
		if !strings.HasPrefix(text, "@") || strings.HasPrefix(text[1:], el.Tag) {
			continue
		}
		got := resourceURLType(strings.TrimSpace(text))
		if got == "" || got == typ || (typ == "drawable" && (got == "color" || got == "mipmap")) {
			continue
		}
		line, col := el.Line, el.Col
		if len(el.TextNodes) > 0 {
			line, col = el.TextNodes[0].Line, el.TextNodes[0].Col
		}
		out = append(out, referenceTypeFinding{file: path, line: line, col: col,
			msg: fmt.Sprintf("Unexpected resource reference type; expected value of type `@%s/`", typ)})
	}
	return out
}
