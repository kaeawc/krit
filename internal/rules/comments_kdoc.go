package rules

import (
	"regexp"
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// kdocContentLines returns the KDoc lines without the comment decoration.
func kdocContentLines(raw string) []string {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "/**")
	raw = strings.TrimSuffix(raw, "*/")
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*"))
	}
	return lines
}

// kdocDefaultSectionText returns the KDoc text before the first block tag, trimmed.
func kdocDefaultSectionText(raw string) string {
	var kept []string
	for _, line := range kdocContentLines(raw) {
		if strings.HasPrefix(line, "@") {
			break
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// kdocStartsWithHTML reports text that opens with an HTML tag on its first line.
func kdocStartsWithHTML(text string) bool {
	if !strings.HasPrefix(text, "<") {
		return false
	}
	first, _, _ := strings.Cut(text, "\n")
	return strings.LastIndexByte(first, '>') > 1
}

// kdocURLSchemes are the schemes java.net.URL accepts.
var kdocURLSchemes = map[string]bool{"http": true, "https": true, "ftp": true, "file": true, "jar": true, "jrt": true, "mailto": true}

// kdocLastWordIsURL reports a last whitespace-separated word that is a well-formed URL.
func kdocLastWordIsURL(text string) bool {
	words := strings.Fields(text)
	if len(words) == 0 {
		return false
	}
	scheme, rest, ok := strings.Cut(words[len(words)-1], ":")
	if !ok || rest == "" || !kdocURLSchemes[strings.ToLower(scheme)] {
		return false
	}
	return kdocURIChars(rest)
}

// kdocURIChars reports that every character may appear in a java.net.URI.
func kdocURIChars(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 0x80:
		case c == '%':
			if i+2 >= len(s) || !isHexByte(s[i+1]) || !isHexByte(s[i+2]) {
				return false
			}
			i += 2
		case strings.IndexByte("<>\"{}|\\^` ", c) >= 0 || c < 0x21:
			return false
		}
	}
	return true
}

func isHexByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

type outdatedKind int

const (
	outdatedParam outdatedKind = iota
	outdatedProperty
	outdatedAny
)

type outdatedDecl struct {
	name string
	kind outdatedKind
}

var kdocSubjectTagRe = regexp.MustCompile(`^@(param|property)[ \t]+\[?([A-Za-z_][A-Za-z0-9_]*)`)

// outdatedDocDecls lists the @param and @property subjects of the KDoc in order.
func outdatedDocDecls(raw string) []outdatedDecl {
	var out []outdatedDecl
	for _, line := range kdocContentLines(raw) {
		m := kdocSubjectTagRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		kind := outdatedParam
		if m[1] == "property" {
			kind = outdatedProperty
		}
		out = append(out, outdatedDecl{name: m[2], kind: kind})
	}
	return out
}

// outdatedDeclsMatch compares documented and declared names as detekt does.
func outdatedDeclsMatch(doc, code []outdatedDecl, ordered bool) bool {
	if len(doc) != len(code) {
		return false
	}
	if !ordered {
		doc, code = sortedOutdated(doc), sortedOutdated(code)
	}
	for i := range doc {
		if doc[i].name != code[i].name {
			return false
		}
		if doc[i].kind != outdatedAny && code[i].kind != outdatedAny && doc[i].kind != code[i].kind {
			return false
		}
	}
	return true
}

func sortedOutdated(in []outdatedDecl) []outdatedDecl {
	out := append([]outdatedDecl(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func (r *OutdatedDocumentationRule) typeParamDecls(file *scanner.File, idx uint32) []outdatedDecl {
	if !r.MatchTypeParameters {
		return nil
	}
	var out []outdatedDecl
	for _, name := range outdatedDocCollectTypeParameterNamesFlat(file, idx) {
		out = append(out, outdatedDecl{name: name, kind: outdatedParam})
	}
	return out
}

func valueParamDecls(file *scanner.File, idx uint32) []outdatedDecl {
	var out []outdatedDecl
	if params, ok := file.FlatFindChild(idx, "function_value_parameters"); ok {
		for _, p := range functionParamEntries(file, params, nil) {
			out = append(out, outdatedDecl{name: paramName(file, p.node), kind: outdatedParam})
		}
	}
	return out
}

// ctorParamDecls lists primary constructor parameters. Public val/var parameters are properties.
func (r *OutdatedDocumentationRule) ctorParamDecls(file *scanner.File, ctor uint32) []outdatedDecl {
	var out []outdatedDecl
	for _, p := range classParamEntries(file, ctor, nil) {
		name := paramName(file, p.node)
		if name == "" {
			continue
		}
		kind := outdatedParam
		if file.FlatHasChildOfType(p.node, "binding_pattern_kind") && !file.FlatHasModifier(p.node, "private") {
			kind = outdatedProperty
			if r.AllowParamOnConstructorProperties {
				kind = outdatedAny
			}
		}
		out = append(out, outdatedDecl{name: name, kind: kind})
	}
	return out
}

func withoutParams(in []outdatedDecl) []outdatedDecl {
	var out []outdatedDecl
	for _, d := range in {
		if d.kind != outdatedParam {
			out = append(out, d)
		}
	}
	return out
}

// classOutdated mirrors detekt: an internal or private constructor also accepts docs for properties only.
func (r *OutdatedDocumentationRule) classOutdated(file *scanner.File, idx uint32, doc []outdatedDecl) bool {
	code := r.typeParamDecls(file, idx)
	ctor, hasCtor := file.FlatFindChild(idx, "primary_constructor")
	if hasCtor {
		code = append(code, r.ctorParamDecls(file, ctor)...)
	}
	if outdatedDeclsMatch(doc, code, r.MatchDeclarationsOrder) {
		return false
	}
	restricted := hasCtor && (file.FlatHasModifier(ctor, "internal") || file.FlatHasModifier(ctor, "private"))
	return !restricted || !outdatedDeclsMatch(doc, withoutParams(code), r.MatchDeclarationsOrder)
}

func (r *OutdatedDocumentationRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	kdoc, ok := flatPrecedingKDoc(file, idx)
	if !ok {
		return
	}
	doc := outdatedDocDecls(file.FlatNodeText(kdoc))
	if len(doc) == 0 {
		return
	}
	var outdated bool
	switch file.FlatType(idx) {
	case "class_declaration":
		outdated = r.classOutdated(file, idx, doc)
	case "secondary_constructor":
		outdated = !outdatedDeclsMatch(doc, valueParamDecls(file, idx), r.MatchDeclarationsOrder)
	default:
		code := append(r.typeParamDecls(file, idx), valueParamDecls(file, idx)...)
		outdated = !outdatedDeclsMatch(doc, code, r.MatchDeclarationsOrder)
	}
	if !outdated {
		return
	}
	if file.FlatType(idx) == "secondary_constructor" {
		// A constructor has no name, so detekt reports at the start of its KDoc.
		ctx.EmitAt(file.FlatRow(kdoc)+1, file.FlatCol(kdoc)+1, "Documentation of <no name> is outdated")
		return
	}
	ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Documentation of "+extractIdentifierFlat(file, idx)+" is outdated")
}

// kdocIsOrphan reports a KDoc that documents nothing: it ends a function body, lambda or class body, or
// precedes a non-declaration statement. detekt only reads KDocs attached to declarations.
func kdocIsOrphan(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	parentType := file.FlatType(parent)
	switch parentType {
	case "statements", "function_body", "lambda_literal", "class_body", "enum_class_body":
	default:
		return false
	}
	next, ok := nextNonLineComment(file, idx)
	if ok && file.FlatType(next) == "statements" {
		next = file.FlatFirstChild(next)
		for next != 0 && file.FlatType(next) == "line_comment" {
			next = file.FlatNextSib(next)
		}
		ok = next != 0
	}
	if !ok {
		return true
	}
	switch file.FlatType(next) {
	case "class_declaration", "object_declaration", "function_declaration", "property_declaration",
		"type_alias", "secondary_constructor", "enum_entry", "companion_object", "anonymous_initializer",
		"multiline_comment", "getter", "setter":
		return false
	}
	return parentType != "class_body" && parentType != "enum_class_body" || file.FlatType(next) == "}"
}

// nextNonLineComment returns the sibling after idx, skipping line comments.
func nextNonLineComment(file *scanner.File, idx uint32) (uint32, bool) {
	next, ok := file.FlatNextSibling(idx)
	for ok && file.FlatType(next) == "line_comment" {
		next, ok = file.FlatNextSibling(next)
	}
	return next, ok
}
