package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

func flatNavigationExpressionLastIdentifier(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	last := ""
	var walk func(uint32)
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "simple_identifier":
			last = file.FlatNodeString(n, nil)
			return
		case "value_arguments", "value_argument", "call_suffix", "lambda_literal", "string_literal":
			return
		}
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				walk(child)
			}
		}
	}
	walk(idx)
	return last
}

func flatNavigationExpressionLastIdentifierEquals(file *scanner.File, idx uint32, want string) bool {
	if file == nil || idx == 0 || want == "" {
		return false
	}
	var last uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "navigation_suffix":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatIsNamed(gc) && file.FlatType(gc) == "simple_identifier" {
					last = gc
				}
			}
		case "simple_identifier":
			last = child
		}
	}
	return last != 0 && file.FlatNodeTextEquals(last, want)
}

// finalSimpleIdentifier returns the identifier text at the end of a
// navigation chain or directly_assignable_expression — the rightmost
// simple_identifier reachable by walking named children. For
// `w.settings.javaScriptEnabled` this returns `javaScriptEnabled`.
func finalSimpleIdentifier(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	// Walk children looking for the last simple_identifier (direct or in a
	// navigation_suffix).
	last := ""
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			last = file.FlatNodeText(child)
		case "navigation_suffix":
			if inner, ok := file.FlatFindChild(child, "simple_identifier"); ok {
				last = file.FlatNodeText(inner)
			}
		case "navigation_expression", "directly_assignable_expression":
			if nested := finalSimpleIdentifier(file, child); nested != "" {
				last = nested
			}
		}
	}
	return last
}

// flatNavigationChainIdentifiers returns the dotted segments of a
// navigation_expression as a slice of identifier names. For
// `R.layout.foo`, returns ["R", "layout", "foo"]. For a call-result
// navigation (`foo().bar`), the first segment is "" because the
// receiver isn't a bare identifier — callers should check len and
// segments[0].
func flatNavigationChainIdentifiers(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 || file.FlatType(idx) != "navigation_expression" {
		return nil
	}
	var walk func(uint32) []string
	walk = func(n uint32) []string {
		if n == 0 {
			return nil
		}
		switch file.FlatType(n) {
		case "simple_identifier":
			return []string{file.FlatNodeText(n)}
		case "navigation_expression":
			var out []string
			for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "simple_identifier":
					out = append(out, file.FlatNodeText(c))
				case "navigation_expression":
					out = append(out, walk(c)...)
				case "navigation_suffix":
					if ident, ok := file.FlatFindChild(c, "simple_identifier"); ok {
						out = append(out, file.FlatNodeText(ident))
					}
				}
			}
			return out
		}
		return nil
	}
	return walk(idx)
}
