package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type RequiresAPIViolationRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *RequiresAPIViolationRule) Confidence() float64 { return api.ConfidenceHigher }

type requiresAPIIndex struct {
	levels map[string]int
}

func isRequiresAPIDeclType(typ string) bool {
	switch typ {
	case "function_declaration", "property_declaration", "class_declaration", "object_declaration":
		return true
	}
	return false
}

func buildRequiresAPIIndex(file *scanner.File) *requiresAPIIndex {
	idx := &requiresAPIIndex{levels: map[string]int{}}
	if file == nil {
		return idx
	}
	file.FlatWalkAllNodes(0, func(decl uint32) {
		if !isRequiresAPIDeclType(file.FlatType(decl)) {
			return
		}
		level := requiresAPILevelForDeclFlat(file, decl)
		if level <= 0 {
			// Tree-sitter Kotlin sometimes parses top-level `@RequiresApi(26)`
			// as a `prefix_expression` sibling rather than a modifier on the
			// next declaration.
			level = precedingPrefixAnnotationLevelFlat(file, decl)
			if level <= 0 {
				return
			}
		}
		name := extractIdentifierFlat(file, decl)
		if name == "" {
			return
		}
		if prior, ok := idx.levels[name]; !ok || level > prior {
			idx.levels[name] = level
		}
	})
	return idx
}

func precedingPrefixAnnotationLevelFlat(file *scanner.File, decl uint32) int {
	parent, ok := file.FlatParent(decl)
	if !ok {
		return 0
	}
	best := 0
	for c := file.FlatFirstChild(parent); c != 0 && c != decl; c = file.FlatNextSib(c) {
		typ := file.FlatType(c)
		if isRequiresAPIDeclType(typ) {
			best = 0
			continue
		}
		if typ != "prefix_expression" {
			continue
		}
		if level := prefixExpressionAnnotationLevel(file, c); level > best {
			best = level
		}
	}
	return best
}

func prefixExpressionAnnotationLevel(file *scanner.File, idx uint32) int {
	var annotationNode, parenNode uint32
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "annotation":
			annotationNode = c
		case "parenthesized_expression":
			parenNode = c
		}
	}
	if annotationNode == 0 {
		return 0
	}
	name := annotationFinalName(file, annotationNode)
	if name != "RequiresApi" && name != "TargetApi" {
		return 0
	}
	if level := requiresAPIAnnotationLevel(file, annotationNode); level > 0 {
		return level
	}
	if parenNode == 0 {
		return 0
	}
	for c := file.FlatFirstChild(parenNode); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "integer_literal" {
			if n, err := strconv.Atoi(strings.TrimSpace(file.FlatNodeText(c))); err == nil {
				return n
			}
		}
	}
	return parseRequiresAPIArgText(file.FlatNodeText(parenNode))
}

func requiresAPILevelForDeclFlat(file *scanner.File, decl uint32) int {
	best := 0
	if mods, ok := file.FlatFindChild(decl, "modifiers"); ok {
		if l := extractRequiresAPILevelFromModifiers(file, mods); l > best {
			best = l
		}
	}
	for p, ok := file.FlatParent(decl); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration":
			if mods, ok := file.FlatFindChild(p, "modifiers"); ok {
				if l := extractRequiresAPILevelFromModifiers(file, mods); l > best {
					best = l
				}
			}
		case "source_file":
			return best
		}
	}
	return best
}

func extractRequiresAPILevelFromModifiers(file *scanner.File, mods uint32) int {
	best := 0
	file.FlatWalkNodes(mods, "annotation", func(ann uint32) {
		name := annotationFinalName(file, ann)
		if name != "RequiresApi" && name != "TargetApi" {
			return
		}
		if level := requiresAPIAnnotationLevel(file, ann); level > best {
			best = level
		}
	})
	return best
}

func requiresAPIAnnotationLevel(file *scanner.File, ann uint32) int {
	if args, ok := file.FlatFindChild(ann, "value_arguments"); ok {
		for i := 0; i < file.FlatChildCount(args); i++ {
			arg := file.FlatChild(args, i)
			if file.FlatType(arg) != "value_argument" {
				continue
			}
			if n := parseRequiresAPIArgText(file.FlatNodeText(arg)); n > 0 {
				return n
			}
		}
		return 0
	}
	text := file.FlatNodeText(ann)
	open := strings.Index(text, "(")
	if open < 0 {
		return 0
	}
	end := strings.LastIndex(text, ")")
	if end <= open {
		return 0
	}
	return parseRequiresAPIArgText(text[open+1 : end])
}

func parseRequiresAPIArgText(text string) int {
	text = strings.TrimSpace(text)
	if eq := strings.Index(text, "="); eq >= 0 {
		text = strings.TrimSpace(text[eq+1:])
	}
	if comma := strings.Index(text, ","); comma >= 0 {
		text = strings.TrimSpace(text[:comma])
	}
	if n, err := strconv.Atoi(text); err == nil {
		return n
	}
	return 0
}

// projectMinSdk defaults to 1 so fixtures and editor/LSP single-file runs
// without Gradle metadata still trigger on @RequiresApi-annotated calls.
func projectMinSdk(ctx *api.Context) int {
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	if facts != nil && facts.Profile.MinSdkVersion > 0 {
		return facts.Profile.MinSdkVersion
	}
	return 1
}

func (r *RequiresAPIViolationRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file == nil || scanner.IsTestFile(file.Path) {
		return
	}
	if nodeHasAncestorTypeFlat(file, idx, "import_header") {
		return
	}

	nodeType := file.FlatType(idx)
	switch nodeType {
	case "navigation_expression":
		if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" {
			return
		}
	case "user_type":
		parent, ok := file.FlatParent(idx)
		if !ok {
			return
		}
		pt := file.FlatType(parent)
		if pt != "call_expression" && pt != "constructor_invocation" {
			return
		}
	}

	name := apiNodeNameFlat(file, idx)
	if name == "" {
		return
	}

	index := filefacts.FileFact(ctx.Facts, file, "requiresAPIIndex", func() *requiresAPIIndex {
		return buildRequiresAPIIndex(file)
	})
	level, ok := index.levels[name]
	if !ok || level <= 0 {
		return
	}

	minSdk := projectMinSdk(ctx)
	if level <= minSdk {
		return
	}
	if apiGuardedByVersionCheckFlat(file, idx) {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Call to '%s' requires API %d but module minSdk is %d; guard with Build.VERSION.SDK_INT or annotate the caller with @RequiresApi(%d).",
			name, level, minSdk, level))
}
