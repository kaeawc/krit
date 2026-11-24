package rules

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Shared evidence for the checks ported from the Lint detectors bundled with AndroidX Compose runtime 1.12.1
// (runtime-lint, runtime-retain-lint, runtime-saveable-lint). Lint resolves every symbol; these helpers only
// accept what the file proves through imports, fully qualified names and same-file declarations.

const (
	acrRuntimePackage  = "androidx.compose.runtime"
	acrComposableFQN   = acrRuntimePackage + ".Composable"
	acrComposableShort = "Composable"
)

// registerAcrRule registers a Compose runtime Lint port. Lint's scope for these issues includes test sources, so
// unlike the AOSP Lint ports they also run on tests. The confidence is high unless impl declares Confidence().
func registerAcrRule(rule BaseRule, impl any, nodeTypes []string, languages []scanner.Language, check func(*api.Context)) {
	confidence := api.ConfidenceHigh
	if c, ok := impl.(interface{ Confidence() float64 }); ok {
		confidence = c.Confidence()
	}
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Desc, Sev: api.Severity(rule.Sev),
		NodeTypes: nodeTypes, Languages: languages, Confidence: confidence, Implementation: impl, Check: check,
	})
}

// acrNameResolvesTo reports whether the simple name refers to pkg.name in file: an explicit import (alias-aware)
// always wins; otherwise a star import of pkg, or the file living in pkg, binds it unless the file declares a
// top-level symbol with the same name.
func acrNameResolvesTo(file *scanner.File, pkg, name string) bool {
	if file == nil || name == "" {
		return false
	}
	fqn := pkg + "." + name
	starImported := false
	if facts := fileFactsCache().Imports(file); facts != nil {
		if target, ok := facts.Aliases[name]; ok {
			return target == fqn
		}
		if _, aliased := facts.FQNs[fqn]; aliased {
			// Imported only under another alias: the simple name is not bound to it.
			return false
		}
		_, starImported = facts.Wildcards[pkg+".*"]
	}
	if !starImported && filePackageName(file) != pkg {
		return false
	}
	return !acrFileDeclaresTopLevel(file, name)
}

// acrCalleeResolvesTo reports whether a call at node whose simple callee name is `name` targets pkg.name: no
// enclosing class, object or function body declares a function of that name (members and local functions win
// over imports), and the name resolves to pkg.name at file level.
func acrCalleeResolvesTo(file *scanner.File, node uint32, pkg, name string) bool {
	return !acrFunctionNameShadowed(file, node, name) && acrNameResolvesTo(file, pkg, name)
}

// acrFunctionNameShadowed reports whether a class body or statement block enclosing node declares a function
// with the given name.
func acrFunctionNameShadowed(file *scanner.File, node uint32, name string) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_body", "statements":
			for child := file.FlatFirstChild(p); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "function_declaration" && acrDeclarationName(file, child) == name {
					return true
				}
			}
		case "source_file":
			return false
		}
	}
	return false
}

// acrFileDeclaresTopLevel reports whether the file declares a top-level class, object, function, property or
// type alias with the given name.
func acrFileDeclaresTopLevel(file *scanner.File, name string) bool {
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "class_declaration", "object_declaration", "function_declaration", "type_alias":
			if acrDeclarationName(file, child) == name {
				return true
			}
		case "property_declaration":
			if v, ok := file.FlatFindChild(child, "variable_declaration"); ok &&
				file.FlatChildTextOrEmpty(v, "simple_identifier") == name {
				return true
			}
		}
	}
	return false
}

// acrDeclarationName returns the unquoted name of a class, object, function or type alias declaration.
func acrDeclarationName(file *scanner.File, decl uint32) string {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier", "type_identifier":
			return strings.Trim(file.FlatNodeText(child), "`")
		case "function_body", "class_body", "enum_class_body", "function_value_parameters":
			return ""
		}
	}
	return ""
}

// acrNameNode returns the name identifier of a function declaration.
func acrNameNode(file *scanner.File, decl uint32) uint32 {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			return child
		case "function_value_parameters", "function_body":
			return 0
		}
	}
	return 0
}

// acrTypeRefersTo reports whether a user_type (or its text) names pkg.name, either fully qualified or through a
// simple name that resolves to it. Type arguments are ignored.
func acrTypeRefersTo(file *scanner.File, typeNode uint32, pkg, name string) bool {
	if typeNode == 0 {
		return false
	}
	text := file.FlatNodeText(typeNode)
	if lt := strings.IndexByte(text, '<'); lt >= 0 {
		text = text[:lt]
	}
	text = strings.Join(strings.Fields(text), "")
	if text == pkg+"."+name {
		return true
	}
	return acrSimpleNameRefersTo(file, text, pkg, name)
}

// acrSimpleNameRefersTo reports whether the simple name used in file refers to pkg.name, through an import alias
// (`import pkg.name as Alias`) or through the name itself.
func acrSimpleNameRefersTo(file *scanner.File, used, pkg, name string) bool {
	if used == "" || strings.Contains(used, ".") {
		return false
	}
	if used != name {
		facts := fileFactsCache().Imports(file)
		return facts != nil && facts.Aliases[used] == pkg+"."+name
	}
	return acrNameResolvesTo(file, pkg, name)
}

// acrAnnotationIsComposable reports whether an annotation node is androidx.compose.runtime.Composable.
func acrAnnotationIsComposable(file *scanner.File, annotation uint32) bool {
	for child := file.FlatFirstChild(annotation); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type":
			return acrTypeRefersTo(file, child, acrRuntimePackage, acrComposableShort)
		case "constructor_invocation":
			if ut, ok := file.FlatFindChild(child, "user_type"); ok {
				return acrTypeRefersTo(file, ut, acrRuntimePackage, acrComposableShort)
			}
			return false
		case "use_site_target":
			return false
		}
	}
	return false
}

// acrHasComposableAnnotation reports whether the modifiers (or type_modifiers) directly under node carry
// @Composable.
func acrHasComposableAnnotation(file *scanner.File, node uint32) bool {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		if t != "modifiers" && t != "type_modifiers" {
			continue
		}
		for m := file.FlatFirstChild(child); m != 0; m = file.FlatNextSib(m) {
			if file.FlatType(m) == "annotation" && acrAnnotationIsComposable(file, m) {
				return true
			}
		}
	}
	return false
}

// acrFunctionIsComposable reports whether a function declaration is annotated @Composable.
func acrFunctionIsComposable(file *scanner.File, fn uint32) bool {
	return file.FlatType(fn) == "function_declaration" && acrHasComposableAnnotation(file, fn)
}

// acrFunctionHasModifier reports whether a declaration's modifiers contain the keyword.
func acrFunctionHasModifier(file *scanner.File, decl uint32, keyword string) bool {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return false
	}
	found := false
	file.FlatWalkAllNodes(mods, func(n uint32) {
		if !found && file.FlatChildCount(n) == 0 && file.FlatNodeTextEquals(n, keyword) {
			found = true
		}
	})
	return found
}

// acrJavaImports reports whether a Java file binds the simple name to pkg.name: a single-type import, or an
// on-demand import of pkg that no same-file class shadows.
func acrJavaImports(file *scanner.File, pkg, name string) bool {
	single, onDemand := false, false
	file.FlatWalkNodes(0, "import_declaration", func(node uint32) {
		text := strings.TrimSpace(file.FlatNodeText(node))
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "import"), ";"))
		if strings.HasPrefix(text, "static ") {
			return
		}
		text = strings.Join(strings.Fields(text), "")
		switch text {
		case pkg + "." + name:
			single = true
		case pkg + ".*":
			onDemand = true
		}
	})
	if single {
		return true
	}
	if !onDemand {
		return false
	}
	shadowed := false
	file.FlatWalkNodes(0, "class_declaration", func(node uint32) {
		if file.FlatChildTextOrEmpty(node, "identifier") == name {
			shadowed = true
		}
	})
	return !shadowed
}

// acrReturnKind classifies a function's declared return type.
type acrReturnKind int

const (
	acrReturnUnknown acrReturnKind = iota
	acrReturnUnit
	acrReturnValue
)

// acrFunctionReturnKind reports whether the function returns Unit, a value, or a type the source cannot prove (an
// expression body without a declared type).
func acrFunctionReturnKind(file *scanner.File, fn uint32) acrReturnKind {
	afterParams := false
	sawColon := false
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch t := file.FlatType(child); {
		case t == "function_value_parameters":
			afterParams = true
		case !afterParams:
			continue
		case t == ":":
			sawColon = true
		case t == "function_body":
			if sawColon {
				return acrReturnUnknown
			}
			if first := file.FlatFirstChild(child); first != 0 && file.FlatType(first) == "=" {
				return acrExpressionBodyReturnKind(file, child, 0)
			}
			return acrReturnUnit
		case sawColon && (t == "user_type" || t == "nullable_type" || t == "function_type" || t == "parenthesized_type"):
			text := strings.Join(strings.Fields(file.FlatNodeText(child)), "")
			if text == "Unit" || text == "kotlin.Unit" {
				return acrReturnUnit
			}
			return acrReturnValue
		case t == "type_constraints":
			continue
		}
	}
	if sawColon {
		return acrReturnUnknown
	}
	return acrReturnUnit
}

// acrExpressionBodyReturnKind infers the type of an untyped expression body only when it is `Unit` or a call to
// the single same-file top-level function of that name, whose own return kind is then used.
func acrExpressionBodyReturnKind(file *scanner.File, body uint32, depth int) acrReturnKind {
	expr := flatLastNamedChild(file, body)
	if expr == 0 || depth > 3 {
		return acrReturnUnknown
	}
	switch file.FlatType(expr) {
	case "simple_identifier":
		if file.FlatNodeTextEquals(expr, "Unit") && !acrFileDeclaresTopLevel(file, "Unit") {
			return acrReturnUnit
		}
	case "call_expression":
		callee := file.FlatFirstChild(expr)
		if file.FlatType(callee) != "simple_identifier" {
			return acrReturnUnknown
		}
		target := acrSingleTopLevelFunction(file, file.FlatNodeText(callee))
		if target == 0 {
			return acrReturnUnknown
		}
		if fb, ok := file.FlatFindChild(target, "function_body"); ok && !acrFunctionDeclaresReturnType(file, target) {
			if first := file.FlatFirstChild(fb); first != 0 && file.FlatType(first) == "=" {
				return acrExpressionBodyReturnKind(file, fb, depth+1)
			}
		}
		return acrFunctionReturnKind(file, target)
	}
	return acrReturnUnknown
}

// acrSingleTopLevelFunction returns the only top-level function declaration with the given name, or 0 when there
// is none or it is overloaded.
func acrSingleTopLevelFunction(file *scanner.File, name string) uint32 {
	var found uint32
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "function_declaration" && acrDeclarationName(file, child) == name {
			if found != 0 {
				return 0
			}
			found = child
		}
	}
	return found
}

// acrFunctionDeclaresReturnType reports whether a function declaration has an explicit `: Type`.
func acrFunctionDeclaresReturnType(file *scanner.File, fn uint32) bool {
	afterParams := false
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_value_parameters":
			afterParams = true
		case ":":
			if afterParams {
				return true
			}
		case "function_body":
			return false
		}
	}
	return false
}

// acrIsLocalDeclaration reports whether a declaration sits inside a function body, lambda or accessor, where
// Lint's UAST does not model it as a member method or field.
func acrIsLocalDeclaration(file *scanner.File, decl uint32) bool {
	for p, ok := file.FlatParent(decl); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_body", "lambda_literal", "anonymous_function", "getter", "setter", "anonymous_initializer",
			"control_structure_body", "statements":
			return true
		case "class_body", "enum_class_body", "source_file":
			return false
		}
	}
	return false
}

// acrFirstRuneIsUpper mirrors Kotlin's Char.isUpperCase on the first character of name.
func acrFirstRuneIsUpper(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return r != utf8.RuneError && unicode.IsUpper(r)
}
