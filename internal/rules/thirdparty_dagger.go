package rules

// Ports of slack-lints' DaggerIssuesDetector (slackhq/slack-lints 0.11.1,
// slack-lint-checks/src/main/java/slack/lint/DaggerIssuesDetector.kt).
//
// Lint runs DaggerIssuesDetector as one detector that reports at most one
// issue per @Binds/@Provides method and returns after the first. daggerIssueOf
// mirrors that control flow, including the early returns, and every
// registered rule emits only when the verdict names its own ID. Two of the
// detector's IDs are not separate krit rules: BindsWrongParameterCount and
// RedundantBinds are Lint aliases of the existing BindsMismatchedArity and
// BindsReturnTypeMatchesParam rules. The pass still evaluates them so their
// early returns hide the same later checks as in Lint.

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	daggerBindsMustBeAbstract       = "BindsMustBeAbstract"
	daggerBindingReceiverParameter  = "BindingReceiverParameter"
	daggerBindingReturnType         = "BindingReturnType"
	daggerBindsWrongParameterCount  = "BindsWrongParameterCount"
	daggerMustBeInModule            = "MustBeInModule"
	daggerProvidesMustNotBeAbstract = "ProvidesMustNotBeAbstract"

	fqcnDaggerBinds    = "dagger.Binds"
	fqcnDaggerProvides = "dagger.Provides"
	fqcnDaggerModule   = "dagger.Module"
	// Lint accepts only dagger.Module; krit also accepts producer modules, which take @Binds and @Provides too.
	fqcnDaggerProducerModule = "dagger.producers.ProducerModule"
)

// daggerIssue is the one finding DaggerIssuesDetector reports for a method.
type daggerIssue struct {
	id   string
	node uint32 // the node whose start is the finding's location
}

// daggerClassKind is the JVM class kind of a method's containing class.
type daggerClassKind int

const (
	daggerKindOther daggerClassKind = iota // enum or annotation class
	daggerKindClass                        // class, object, companion object or anonymous object
	daggerKindInterface
)

var (
	daggerBindsName    = []byte("Binds")
	daggerProvidesName = []byte("Provides")
)

// daggerFileGate skips files that cannot name either annotation.
func daggerFileGate(file *scanner.File) bool {
	return sourceMayNameAny(file, "Binds", "Provides")
}

// daggerMayNameAnnotation reports whether modifier text can name dagger.Binds or dagger.Provides: by name, or by an
// alias the file imports one of them under (`import dagger.Provides as P`).
func daggerMayNameAnnotation(file *scanner.File, text []byte) bool {
	if bytes.Contains(text, daggerBindsName) || bytes.Contains(text, daggerProvidesName) {
		return true
	}
	for _, alias := range daggerImportAliases(file) {
		if bytes.Contains(text, alias) {
			return true
		}
	}
	return false
}

// daggerImportAliases returns the names other than Binds and Provides that the file imports those annotations as.
func daggerImportAliases(file *scanner.File) [][]byte {
	return filefacts.FileFact(fileFactsCache(), file, slotDaggerAliases, func() [][]byte {
		var aliases [][]byte
		for simple, full := range lintImportsOf(file).bySimple {
			if (full == fqcnDaggerBinds || full == fqcnDaggerProvides) && simple != full[len("dagger."):] {
				aliases = append(aliases, []byte("@"+simple))
			}
		}
		return aliases
	})
}

const slotDaggerAliases = "daggerAliases"

// daggerIssueOf returns the issue DaggerIssuesDetector reports for the Kotlin
// function or Java method fn, or false when it reports none or the source
// cannot prove the verdict.
func daggerIssueOf(file *scanner.File, fn uint32) (daggerIssue, bool) {
	// Most methods carry neither name (nor an import alias of one) in their modifiers; reject them before
	// resolving annotations.
	mods, ok := file.FlatFindChild(fn, "modifiers")
	if !ok {
		return daggerIssue{}, false
	}
	if !daggerMayNameAnnotation(file, file.FlatNodeBytes(mods)) {
		return daggerIssue{}, false
	}
	anns := moshiAnnotationsOf(file, fn)
	isBinds := moshiFindAnnotation(file, anns, fqcnDaggerBinds) != nil
	isProvides := moshiFindAnnotation(file, anns, fqcnDaggerProvides) != nil
	if !isBinds && !isProvides {
		return daggerIssue{}, false
	}
	java := file.Language == scanner.LangJava
	container, kind, topLevel, ok := daggerContainingClass(file, fn)
	if !ok {
		// A local function is not a UMethod, so Lint never visits it.
		return daggerIssue{}, false
	}
	// A top-level Kotlin function belongs to its file facade class, which is never a module.
	if topLevel || !daggerIsModule(file, daggerModuleClass(file, container)) {
		return daggerIssue{daggerMustBeInModule, fn}, true
	}
	body := daggerBody(file, fn)
	switch {
	case isBinds && kind == daggerKindInterface:
		if body != 0 {
			return daggerIssue{daggerBindsMustBeAbstract, daggerBodyLocation(file, fn, body)}, true
		}
	case kind == daggerKindClass:
		isAbstract := file.FlatFindModifierNode(fn, "abstract") != 0
		if isBinds && !isAbstract {
			return daggerIssue{daggerBindsMustBeAbstract, fn}, true
		}
		if isProvides && isAbstract {
			return daggerIssue{daggerProvidesMustNotBeAbstract, fn}, true
		}
	case kind == daggerKindInterface && isProvides:
		// Deliberate divergence: Lint also reports a static Java method, which Dagger accepts in an interface module.
		if !java || file.FlatFindModifierNode(fn, "static") == 0 {
			return daggerIssue{daggerProvidesMustNotBeAbstract, fn}, true
		}
	}

	params, list := daggerParameters(file, fn)
	if isBinds && params != 1 {
		at := fn
		if params > 0 {
			at = list
		}
		return daggerIssue{daggerBindsWrongParameterCount, at}, true
	}
	if java {
		if ret, ok := file.FlatFindChild(fn, "void_type"); ok {
			return daggerIssue{daggerBindingReturnType, ret}, true
		}
		return daggerIssue{}, false
	}
	// Lint reports a Kotlin return type at the function: the light method has no return type element.
	if daggerKotlinReturnsUnit(file, fn, body) {
		return daggerIssue{daggerBindingReturnType, fn}, true
	}
	if receiver := daggerKotlinReceiverType(file, fn); receiver != 0 {
		return daggerIssue{daggerBindingReceiverParameter, receiver}, true
	}
	// RedundantBinds and BindsTypeMismatch need the parameter and return types; not ported here.
	return daggerIssue{}, false
}

// daggerContainingClass returns the class declaration that contains fn and its
// kind. topLevel reports a top-level Kotlin function; ok is false when fn is
// not a member of a class or file (a local function).
func daggerContainingClass(file *scanner.File, fn uint32) (container uint32, kind daggerClassKind, topLevel, ok bool) {
	parent, _ := file.FlatParent(fn)
	switch file.FlatType(parent) {
	case "source_file":
		return 0, daggerKindOther, true, true
	case "class_body", "enum_class_body", "interface_body", "annotation_type_body":
	case "enum_body_declarations":
		parent, _ = file.FlatParent(parent)
	default:
		return 0, daggerKindOther, false, false
	}
	container, _ = file.FlatParent(parent)
	switch file.FlatType(container) {
	case "class_declaration":
		if file.FlatHasChildOfType(container, "interface") {
			return container, daggerKindInterface, false, true
		}
		// Test the body, not the `enum` keyword: tree-sitter moves the keyword out of the modifiers after an annotation.
		if file.FlatType(parent) == "enum_class_body" {
			return container, daggerKindOther, false, true
		}
		return container, daggerKindClass, false, true
	case "object_declaration", "companion_object", "object_literal", "object_creation_expression", "record_declaration":
		return container, daggerKindClass, false, true
	case "interface_declaration":
		return container, daggerKindInterface, false, true
	case "enum_declaration", "annotation_type_declaration", "enum_entry", "enum_constant":
		return container, daggerKindOther, false, true
	}
	return 0, daggerKindOther, false, false
}

// daggerModuleClass returns the class whose @Module annotation counts for
// members of container: the enclosing class for a companion object.
func daggerModuleClass(file *scanner.File, container uint32) uint32 {
	if file.FlatType(container) != "companion_object" {
		return container
	}
	body, _ := file.FlatParent(container)
	outer, _ := file.FlatParent(body)
	return outer
}

func daggerIsModule(file *scanner.File, class uint32) bool {
	if class == 0 {
		return false
	}
	anns := moshiAnnotationsOf(file, class)
	return moshiFindAnnotation(file, anns, fqcnDaggerModule) != nil || moshiFindAnnotation(file, anns, fqcnDaggerProducerModule) != nil
}

// daggerBody returns fn's function_body (Kotlin) or block (Java), or 0.
func daggerBody(file *scanner.File, fn uint32) uint32 {
	if body, ok := file.FlatFindChild(fn, "function_body"); ok {
		return body
	}
	if body, ok := file.FlatFindChild(fn, "block"); ok {
		return body
	}
	return 0
}

// daggerBodyLocation is where Lint reports a body: at its opening brace, or at
// the function for a Kotlin expression body.
func daggerBodyLocation(file *scanner.File, fn, body uint32) uint32 {
	if file.FlatType(body) == "function_body" {
		if first := file.FlatFirstChild(body); first == 0 || file.FlatType(first) != "{" {
			return fn
		}
	}
	return body
}

// daggerParameters counts fn's JVM parameters, an extension receiver included,
// and returns its parameter list.
func daggerParameters(file *scanner.File, fn uint32) (count int, list uint32) {
	if params, ok := file.FlatFindChild(fn, "function_value_parameters"); ok {
		list = params
		walkFunctionParametersFlat(file, params, func(uint32) { count++ })
		if file.FlatHasChildOfType(fn, ".") {
			count++
		}
		return count, list
	}
	if params, ok := file.FlatFindChild(fn, "formal_parameters"); ok {
		list = params
		file.FlatForEachChild(params, func(child uint32) {
			switch file.FlatType(child) {
			case "formal_parameter", "spread_parameter":
				count++
			}
		})
	}
	return count, list
}

// daggerKotlinReturnsUnit reports whether the Kotlin function provably
// returns Unit: a declared Unit type, a block body or no body without a
// declared type, or an expression body that is `Unit` itself. Any other
// expression body abstains, since its type is not known from source.
func daggerKotlinReturnsUnit(file *scanner.File, fn, body uint32) bool {
	if ret := daggerKotlinReturnType(file, fn); ret != 0 {
		text := strings.Join(strings.Fields(file.FlatNodeText(ret)), "")
		return (text == "Unit" && lintSameFileClass(file, "Unit") == 0) || text == "kotlin.Unit"
	}
	if body == 0 {
		return true
	}
	first := file.FlatFirstChild(body)
	if file.FlatType(first) == "{" {
		return true
	}
	for child := first; child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			return file.FlatType(child) == "simple_identifier" && file.FlatNodeTextEquals(child, "Unit") &&
				lintSameFileClass(file, "Unit") == 0
		}
	}
	return false
}

// daggerKotlinReturnType returns the declared return type node of a Kotlin function, or 0.
func daggerKotlinReturnType(file *scanner.File, fn uint32) uint32 {
	afterColon := false
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch t := file.FlatType(child); {
		case t == ":":
			afterColon = true
		case t == "function_body":
			return 0
		case afterColon && file.FlatIsNamed(child) && t != "type_constraints":
			return child
		}
	}
	return 0
}

// daggerKotlinReceiverType returns the receiver type node of a Kotlin extension function, or 0.
func daggerKotlinReceiverType(file *scanner.File, fn uint32) uint32 {
	var prev uint32
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case ".":
			return prev
		case "simple_identifier", "function_value_parameters":
			return 0
		}
		if file.FlatIsNamed(child) {
			prev = child
		}
	}
	return 0
}
