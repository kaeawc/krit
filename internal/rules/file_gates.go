package rules

import (
	"bytes"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/scanner"
)

// sourceMayNameAny is a conservative FileGate building block: it reports
// whether the file's source can spell one of the identifiers. It is true when
// one of them occurs literally, and also whenever the source holds text that
// rule helpers normalize before comparing names, since normalizing could join
// two fragments into an identifier: helpers strip backticks from node text,
// so two adjacent backticks between fragments would vanish, and Java import
// facts drop block comments from import text without leaving a separator.
//
// Identifiers are matched as raw bytes, which also covers aliases (the import
// that introduces an alias spells the real name) and fully-qualified names.
func sourceMayNameAny(file *scanner.File, identifiers ...string) bool {
	if file == nil {
		return true
	}
	content := file.Content
	for _, id := range identifiers {
		if bytes.Contains(content, []byte(id)) {
			return true
		}
	}
	if bytes.Contains(content, []byte("``")) {
		return true
	}
	return file.Language == scanner.LangJava && bytes.Contains(content, []byte("*/"))
}

// compactTextHasSuffix reports whether text with every Unicode space removed —
// what strings.Join(strings.Fields(text), "") builds — ends with suffix,
// without building that string.
func compactTextHasSuffix(text, suffix string) bool {
	i, j := len(text), len(suffix)
	for j > 0 {
		if i == 0 {
			return false
		}
		r, size := utf8.DecodeLastRuneInString(text[:i])
		i -= size
		if unicode.IsSpace(r) {
			continue
		}
		want, wantSize := utf8.DecodeLastRuneInString(suffix[:j])
		if r != want {
			return false
		}
		j -= wantSize
	}
	return true
}
