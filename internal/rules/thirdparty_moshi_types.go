package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// moshiAnnotation is one annotation on a Kotlin or Java declaration.
type moshiAnnotation struct {
	node     uint32
	nameNode uint32 // the annotation's type reference
	name     string // the type reference as written, e.g. "Json" or "com.squareup.moshi.Json"
	site     uint32 // Kotlin use-site target (`@field:`), or 0
	args     uint32 // value_arguments (Kotlin) or annotation_argument_list (Java), or 0
}

// moshiAnnotationsOf returns the annotations in the modifiers of decl.
func moshiAnnotationsOf(file *scanner.File, decl uint32) []moshiAnnotation {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return nil
	}
	var out []moshiAnnotation
	file.FlatForEachChild(mods, func(child uint32) {
		switch file.FlatType(child) {
		case "annotation", "marker_annotation":
			if ann, ok := moshiReadAnnotation(file, child); ok {
				out = append(out, ann)
			}
		}
	})
	return out
}

func moshiReadAnnotation(file *scanner.File, idx uint32) (moshiAnnotation, bool) {
	ann := moshiAnnotation{node: idx}
	file.FlatForEachChild(idx, func(child uint32) {
		switch file.FlatType(child) {
		case "use_site_target":
			ann.site = child
		case "constructor_invocation":
			ann.nameNode, _ = file.FlatFindChild(child, "user_type")
			ann.args, _ = file.FlatFindChild(child, "value_arguments")
		case "user_type", "identifier", "scoped_identifier":
			if ann.nameNode == 0 {
				ann.nameNode = child
			}
		case "annotation_argument_list":
			ann.args = child
		}
	})
	if ann.nameNode == 0 {
		return ann, false
	}
	ann.name = strings.Join(strings.Fields(file.FlatNodeText(ann.nameNode)), "")
	return ann, true
}

// moshiFindAnnotation returns the first annotation in anns that names fqn.
func moshiFindAnnotation(file *scanner.File, anns []moshiAnnotation, fqn string) *moshiAnnotation {
	for i := range anns {
		if moshiRefIs(file, anns[i].name, fqn) {
			return &anns[i]
		}
	}
	return nil
}

// moshiRefIs reports whether the type reference text, as written in file,
// names the class fqn: spelled in full, imported (directly, by alias, or on
// demand), or declared in the file's own package.
func moshiRefIs(file *scanner.File, text, fqn string) bool {
	if text == fqn {
		return true
	}
	head, _, _ := strings.Cut(text, ".")
	if _, imported := lintImportsOf(file).bySimple[head]; imported {
		return lintTypeRefMatches(file, text, fqn)
	}
	if lintTypeRefMatches(file, text, fqn) {
		return true
	}
	dot := strings.LastIndex(fqn, ".")
	return dot > 0 && !strings.Contains(text, ".") && text == fqn[dot+1:] &&
		moshiFilePackage(file) == fqn[:dot] && lintSameFileClass(file, text) == 0
}

// moshiFilePackage returns the package the file declares, or "".
func moshiFilePackage(file *scanner.File) string {
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "package_header", "package_declaration":
			for id := file.FlatFirstChild(child); id != 0; id = file.FlatNextSib(id) {
				switch file.FlatType(id) {
				case "identifier", "scoped_identifier":
					return strings.Join(strings.Fields(file.FlatNodeText(id)), "")
				}
			}
			return ""
		}
	}
	return ""
}

// moshiAnnotationArg returns the value expression of the annotation argument
// called name, or, in Kotlin, the unnamed argument at position pos. Java
// annotations take an unnamed argument only for "value".
func moshiAnnotationArg(file *scanner.File, ann moshiAnnotation, name string, pos int) uint32 {
	if ann.args == 0 {
		return 0
	}
	if file.FlatType(ann.args) == "value_arguments" {
		arg := flatNamedValueArgument(file, ann.args, name)
		if arg == 0 && pos >= 0 {
			arg = flatPositionalValueArgument(file, ann.args, pos)
		}
		return flatValueArgumentExpression(file, arg)
	}
	var value uint32
	for child := file.FlatFirstChild(ann.args); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) != "element_value_pair" {
			if name == "value" {
				value = child
			}
			continue
		}
		key, _ := file.FlatFindChild(child, "identifier")
		if key != 0 && file.FlatNodeText(key) == name {
			for v := file.FlatFirstChild(child); v != 0; v = file.FlatNextSib(v) {
				if file.FlatIsNamed(v) && v != key {
					value = v
				}
			}
		}
	}
	return value
}

// moshiAnnotationStringArg returns the string value of an annotation
// argument, or def when the argument is absent. known is false when the
// argument is not a plain string literal.
func moshiAnnotationStringArg(file *scanner.File, ann moshiAnnotation, name string, pos int, def string) (value string, known bool) {
	expr := moshiAnnotationArg(file, ann, name, pos)
	if expr == 0 {
		return def, true
	}
	return moshiStringLiteral(file, expr)
}

// moshiStringLiteral returns the value of a string literal without
// templates or escapes.
func moshiStringLiteral(file *scanner.File, idx uint32) (string, bool) {
	if idx == 0 || file.FlatType(idx) != "string_literal" {
		return "", false
	}
	var b strings.Builder
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "string_content", "string_fragment":
			b.WriteString(file.FlatNodeText(child))
		case "\"", "\"\"\"":
		default:
			return "", false
		}
	}
	if file.FlatChildCount(idx) == 0 {
		text := file.FlatNodeText(idx)
		if text != `""` && text != `""""""` {
			return "", false
		}
	}
	return b.String(), true
}

// moshiBooleanLiteral returns the value of a Kotlin or Java boolean literal.
func moshiBooleanLiteral(file *scanner.File, idx uint32) (value, ok bool) {
	if idx == 0 {
		return false, false
	}
	switch file.FlatType(idx) {
	case "boolean_literal":
		if isBooleanLiteralTrue(file, idx) {
			return true, true
		}
		return false, isBooleanLiteralFalse(file, idx)
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}
