package rules

import (
	"bytes"
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/lazyre"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// InjectOnAbstractClassRule detects @Inject primary constructors on abstract
// classes. Dagger cannot instantiate an abstract class, so the binding is
// unreachable.
type InjectOnAbstractClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *InjectOnAbstractClassRule) Confidence() float64 { return api.ConfidenceMedium }

// ProviderInsteadOfLazyRule detects constructor parameters typed Provider<T>
// whose `.get()` is called exactly once across the class body. Lazy<T> matches
// the same intent and is cheaper.
type ProviderInsteadOfLazyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *ProviderInsteadOfLazyRule) Confidence() float64 { return api.ConfidenceMediumLow }

// LazyInsteadOfDirectRule detects constructor parameters typed Lazy<T> whose
// `.get()` is called unconditionally at class-init time. Direct injection is
// cheaper because the value is already eagerly required.
type LazyInsteadOfDirectRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *LazyInsteadOfDirectRule) Confidence() float64 { return api.ConfidenceMediumLow }

// classParameterNameAndType returns the simple identifier name and the
// last-segment unqualified type name of a primary-constructor `class_parameter`.
func classParameterNameAndType(file *scanner.File, param uint32) (name, typeName string) {
	if file == nil || param == 0 {
		return "", ""
	}
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			if name == "" {
				name = strings.TrimSpace(file.FlatNodeText(child))
			}
		case "user_type", "nullable_type":
			if typeName == "" {
				typeName = lastTypeSegment(file.FlatNodeText(child))
			}
		}
	}
	return name, typeName
}

// callExpressionGetReceiver returns the receiver text when `idx` is a
// call_expression of the form `<receiver>.get()`. It returns ""
// otherwise.
func callExpressionGetReceiver(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return ""
	}
	callee := file.FlatFirstChild(idx)
	for callee != 0 && !file.FlatIsNamed(callee) {
		callee = file.FlatNextSib(callee)
	}
	if callee == 0 || file.FlatType(callee) != "navigation_expression" {
		return ""
	}
	text := strings.TrimSpace(file.FlatNodeText(callee))
	if !strings.HasSuffix(text, ".get") {
		return ""
	}
	return strings.TrimSpace(text[:len(text)-len(".get")])
}

// countGetCallsInClassBody counts call_expressions of the form `name.get()`
// inside the class body (not including nested classes). It also reports whether
// any such call appears inside a property initializer or `init` block.
func countGetCallsInClassBody(file *scanner.File, classIdx uint32, name string) (count int, atInit bool) {
	if file == nil || classIdx == 0 || name == "" {
		return 0, false
	}
	body, ok := file.FlatFindChild(classIdx, "class_body")
	if !ok || body == 0 {
		return 0, false
	}
	var visit func(node uint32, inInitContext bool)
	visit = func(node uint32, inInitContext bool) {
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			t := file.FlatType(child)
			// Skip nested type declarations to keep scope bounded.
			if t == "class_declaration" || t == "object_declaration" {
				continue
			}
			childInit := inInitContext
			switch t {
			case "anonymous_initializer", "property_declaration":
				childInit = true
			case "function_declaration", "secondary_constructor", "getter", "setter":
				childInit = false
			}
			if t == "call_expression" {
				if rcv := callExpressionGetReceiver(file, child); rcv == name {
					count++
					if inInitContext {
						atInit = true
					}
				}
			}
			visit(child, childInit)
		}
	}
	visit(body, false)
	return count, atInit
}

// BindsInsteadOfProvidesRule detects `@Provides` functions with a single
// parameter whose body simply returns that parameter unchanged. The function
// can be expressed as a cheaper `@Binds` abstract method.
type BindsInsteadOfProvidesRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *BindsInsteadOfProvidesRule) Confidence() float64 { return api.ConfidenceMediumLowPlus }

// BindsReturnTypeMatchesParamRule detects `@Binds` functions whose parameter
// type equals the return type — a no-op binding that Dagger rejects.
type BindsReturnTypeMatchesParamRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *BindsReturnTypeMatchesParamRule) Confidence() float64 { return api.ConfidenceMedium }

// bindsNonKeyAnnotations never qualify a binding key, so they may sit on a @Binds function alone.
var bindsNonKeyAnnotations = map[string]bool{
	"Binds": true, "Suppress": true, "JvmSuppressWildcards": true, "Deprecated": true,
	"JvmStatic": true, "Throws": true, "OptIn": true, "SingleIn": true,
}

// bindsIsMultibinding reports a @Binds contribution into a set or map, which is never a no-op.
func bindsIsMultibinding(file *scanner.File, fn uint32) bool {
	return hasAnnotationFlat(file, fn, "IntoSet") || hasAnnotationFlat(file, fn, "IntoMap") ||
		hasAnnotationFlat(file, fn, "ElementsIntoSet")
}

// bindsKeyAnnotationsDiffer reports whether the function and its parameter carry different annotations
// as text, other than ones that never qualify a key, so the binding may alias one qualified key to
// another rather than bind a key to itself.
func bindsKeyAnnotationsDiffer(file *scanner.File, fn uint32) bool {
	fnKeys := bindsKeyAnnotationTexts(annotationNodeTextsFlat(file, fn))
	var paramTexts []string
	if params, ok := file.FlatFindChild(fn, "function_value_parameters"); ok {
		for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "parameter" {
				break
			}
			if file.FlatType(child) != "parameter_modifiers" {
				continue
			}
			for ann := file.FlatFirstChild(child); ann != 0; ann = file.FlatNextSib(ann) {
				if file.FlatType(ann) == "annotation" {
					paramTexts = append(paramTexts, file.FlatNodeText(ann))
				}
			}
		}
	}
	return !slices.Equal(fnKeys, bindsKeyAnnotationTexts(paramTexts))
}

// bindsKeyAnnotationTexts returns the sorted annotation texts that may qualify a key, without
// whitespace and with the annotation's name reduced to its simple name.
func bindsKeyAnnotationTexts(texts []string) []string {
	var keys []string
	for _, text := range texts {
		compact := strings.Join(strings.Fields(text), "")
		ref := strings.TrimPrefix(compact, "@")
		if colon := strings.Index(ref, ":"); colon >= 0 && !strings.ContainsAny(ref[:colon], ".([") {
			ref = ref[colon+1:]
		}
		end := 0
		for end < len(ref) && (isIdentByte(ref[end]) || ref[end] == '.') {
			end++
		}
		name := ref[:end]
		if dot := strings.LastIndex(name, "."); dot >= 0 {
			name = name[dot+1:]
		}
		if bindsNonKeyAnnotations[name] || slices.Contains(bindingScopeAnnotationNames, name) {
			continue
		}
		keys = append(keys, name+ref[end:])
	}
	slices.Sort(keys)
	return keys
}

// firstFunctionParameterNameAndType returns the (name, type) of the first
// parameter of `fn` (a function_declaration). Returns ("", "") if absent.
func firstFunctionParameterNameAndType(file *scanner.File, fn uint32) (string, string) {
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return "", ""
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		name := ""
		typeName := ""
		for sub := file.FlatFirstChild(child); sub != 0; sub = file.FlatNextSib(sub) {
			switch file.FlatType(sub) {
			case "simple_identifier":
				if name == "" {
					name = strings.TrimSpace(file.FlatNodeText(sub))
				}
			case "user_type", "nullable_type", "function_type", "parenthesized_type":
				if typeName == "" {
					typeName = strings.TrimSpace(file.FlatNodeText(sub))
				}
			}
		}
		return name, typeName
	}
	return "", ""
}

// expressionBodyReturnsIdentifier returns the simple-identifier text when fn
// has an expression body of the form `= identifier` (no further calls), or ""
// otherwise.
func expressionBodyReturnsIdentifier(file *scanner.File, fn uint32) string {
	body, ok := file.FlatFindChild(fn, "function_body")
	if !ok || body == 0 {
		return ""
	}
	var named uint32
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if named != 0 {
			return ""
		}
		named = child
	}
	if named == 0 || file.FlatType(named) != "simple_identifier" {
		return ""
	}
	return strings.TrimSpace(file.FlatNodeText(named))
}

// SingletonOnMutableClassRule detects @Singleton (or other application-scoped)
// classes whose body declares unprotected mutable state — `var` properties or
// `val` properties initialised with a mutable collection factory.
type SingletonOnMutableClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *SingletonOnMutableClassRule) Confidence() float64 { return api.ConfidenceMedium }

var singletonScopeAnnotationNames = []string{
	"Singleton",
	"ApplicationScoped",
}

var singletonMutableCollectionFactories = map[string]struct{}{
	"mutableListOf": {},
	"mutableSetOf":  {},
	"mutableMapOf":  {},
	"arrayListOf":   {},
	"hashMapOf":     {},
	"hashSetOf":     {},
	"linkedMapOf":   {},
	"linkedSetOf":   {},
	"ArrayList":     {},
	"HashMap":       {},
	"HashSet":       {},
	"LinkedHashMap": {},
	"LinkedHashSet": {},
}

func singletonScopeAnnotationFlat(file *scanner.File, idx uint32) string {
	for _, name := range singletonScopeAnnotationNames {
		if hasAnnotationFlat(file, idx, name) {
			return name
		}
	}
	return ""
}

// singletonPropertyInitCallee returns the simple-identifier callee of the
// property's initializer expression, or "" if the initializer is not a
// call expression.
func singletonPropertyInitCallee(file *scanner.File, prop uint32) string {
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "call_expression" {
			continue
		}
		// First named child of call_expression is the callee.
		callee := file.FlatFirstChild(child)
		for callee != 0 && !file.FlatIsNamed(callee) {
			callee = file.FlatNextSib(callee)
		}
		if callee == 0 {
			return ""
		}
		switch file.FlatType(callee) {
		case "simple_identifier", "identifier":
			return file.FlatNodeString(callee, nil)
		case "navigation_expression":
			text := strings.TrimSpace(file.FlatNodeText(callee))
			if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
				return text[dotIdx+1:]
			}
			return text
		}
		return ""
	}
	return ""
}

// MetroFactoryDeclarationShapeRule detects Metro factory annotations on
// concrete or sealed declarations. Metro factory implementations are generated.
type MetroFactoryDeclarationShapeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *MetroFactoryDeclarationShapeRule) Confidence() float64 { return api.ConfidenceMedium }

func hasMetroFactoryAnnotationFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	return findAnnotationTextFlat(file, idx, "DependencyGraph.Factory") != "" ||
		findAnnotationTextFlat(file, idx, "GraphExtension.Factory") != ""
}

// ScopeOnParameterizedClassRule detects class-keyed scopes and CDI definition
// errors on generic classes.
type ScopeOnParameterizedClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *ScopeOnParameterizedClassRule) Confidence() float64 { return api.ConfidenceMedium }

// bindingScopeAnnotationNames lists annotations that mark a DI scope. The list mirrors
// commonly used Hilt/Dagger scopes plus Anvil/Metro variants.
var bindingScopeAnnotationNames = []string{
	"Singleton",
	"Reusable",
	"ApplicationScoped",
	"ActivityScoped",
	"ActivityRetainedScoped",
	"FragmentScoped",
	"ViewScoped",
	"ViewModelScoped",
	"ServiceScoped",
	"SessionScoped",
	"RequestScoped",
	"UserScoped",
}

func firstScopeAnnotationFlat(file *scanner.File, idx uint32) string {
	for _, name := range bindingScopeAnnotationNames {
		if hasAnnotationFlat(file, idx, name) {
			return name
		}
	}
	return ""
}

// parameterizedScopeKind distinguishes shared class-keyed scopes from CDI definition errors.
type parameterizedScopeKind uint8

const (
	parameterizedScopeShared parameterizedScopeKind = iota
	parameterizedScopeCDI
)

// scopeAnnotationFrameworks maps scope names to their defining packages and framework semantics.
var scopeAnnotationFrameworks = map[string]map[string]parameterizedScopeKind{
	"Single": {
		"org.koin.core.annotation": parameterizedScopeShared,
	},
	"Singleton": {
		"org.koin.core.annotation": parameterizedScopeShared,
		"javax.ejb":                parameterizedScopeShared, "jakarta.ejb": parameterizedScopeShared,
	},
	"ApplicationScoped": {
		"javax.faces.bean": parameterizedScopeShared, "jakarta.faces.bean": parameterizedScopeShared,
		"javax.enterprise.context": parameterizedScopeCDI, "jakarta.enterprise.context": parameterizedScopeCDI,
	},
	"SessionScoped": {
		"javax.faces.bean": parameterizedScopeShared, "jakarta.faces.bean": parameterizedScopeShared,
		"javax.enterprise.context": parameterizedScopeCDI, "jakarta.enterprise.context": parameterizedScopeCDI,
	},
	"RequestScoped": {
		"javax.faces.bean": parameterizedScopeShared, "jakarta.faces.bean": parameterizedScopeShared,
		"javax.enterprise.context": parameterizedScopeCDI, "jakarta.enterprise.context": parameterizedScopeCDI,
	},
	"ViewScoped": {
		"javax.faces.bean": parameterizedScopeShared, "jakarta.faces.bean": parameterizedScopeShared,
		"javax.faces.view": parameterizedScopeCDI, "jakarta.faces.view": parameterizedScopeCDI,
	},
	"FlowScoped": {
		"javax.faces.flow": parameterizedScopeCDI, "jakarta.faces.flow": parameterizedScopeCDI,
	},
	"TransactionScoped": {
		"javax.transaction": parameterizedScopeCDI, "jakarta.transaction": parameterizedScopeCDI,
	},
	"ConversationScoped": {
		"javax.enterprise.context": parameterizedScopeCDI, "jakarta.enterprise.context": parameterizedScopeCDI,
	},
}

// scopeOnParameterizedClassScopeFlat resolves annotations through qualified references,
// explicit imports, then same-file declarations; unresolved packages remain silent.
func scopeOnParameterizedClassScopeFlat(file *scanner.File, idx uint32) (string, parameterizedScopeKind) {
	var references []string
	for _, annotation := range annotationNodesFlat(file, idx) {
		for child := file.FlatFirstChild(annotation); child != 0; child = file.FlatNextSib(child) {
			userType := child
			if file.FlatType(child) == "constructor_invocation" {
				userType, _ = file.FlatFindChild(child, "user_type")
			}
			if userType != 0 && file.FlatType(userType) == "user_type" {
				references = append(references, file.FlatNodeText(userType))
			}
		}
	}
	for _, ref := range references {
		name, pkg := ref, ""
		if dot := strings.LastIndex(ref, "."); dot >= 0 {
			name, pkg = ref[dot+1:], ref[:dot]
		} else if importedRef, imported := diImportedAnnotationReference(file, name); imported {
			dot := strings.LastIndex(importedRef, ".")
			name, pkg = importedRef[dot+1:], importedRef[:dot]
		} else {
			if !localAnnotationClassHeader(file, name) {
				continue
			}
			pkg = filePackageName(file)
		}
		if kind, ok := scopeAnnotationFrameworks[name][pkg]; ok {
			return name, kind
		}
	}
	return "", parameterizedScopeShared
}

// annotationNodeTextsFlat returns the text of each annotation node associated with idx.
func annotationNodeTextsFlat(file *scanner.File, idx uint32) []string {
	var texts []string
	for _, annotation := range annotationNodesFlat(file, idx) {
		texts = append(texts, file.FlatNodeText(annotation))
	}
	return texts
}

// annotationNodesFlat returns annotation nodes in idx's modifier list and preceding modifier or annotation siblings.
func annotationNodesFlat(file *scanner.File, idx uint32) []uint32 {
	var annotations []uint32
	addModifiers := func(mods uint32) {
		for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "annotation" {
				annotations = append(annotations, child)
			}
		}
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		addModifiers(mods)
	}
	for prev, ok := file.FlatPrevSibling(idx); ok; prev, ok = file.FlatPrevSibling(prev) {
		switch file.FlatType(prev) {
		case "modifiers":
			addModifiers(prev)
			continue
		case "annotation":
			annotations = append(annotations, prev)
			continue
		}
		break
	}
	return annotations
}

// localAnnotationClassHeader reports whether the file declares its own `annotation class name`.
// It reads declaration nodes, so a commented-out declaration or one inside a string does not count.
func localAnnotationClassHeader(file *scanner.File, name string) bool {
	if !bytes.Contains(file.Content, []byte("class "+name)) {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "class_declaration", func(decl uint32) {
		if found || !file.FlatHasModifier(decl, "annotation") || extractIdentifierFlat(file, decl) != name {
			return
		}
		found = true
	})
	return found
}

// diImportedPackage returns the package the file imports name from, by its own name or an import
// alias, and whether it imports name at all. It reads the parsed import headers, which sit before the
// first declaration, so an import written in a comment does not count.
func diImportedPackage(file *scanner.File, name string) (string, bool) {
	ref, imported := diImportedAnnotationReference(file, name)
	if !imported {
		return "", false
	}
	dot := strings.LastIndex(ref, ".")
	if ref[dot+1:] != name {
		return "", true
	}
	return ref[:dot], true
}

// diImportedAnnotationReference resolves an explicit annotation import, including aliases,
// without guessing packages from wildcard imports.
func diImportedAnnotationReference(file *scanner.File, name string) (string, bool) {
	ref, imported := "", false
	visit := func(header uint32) bool {
		path, alias, wildcard := getInstanceImportHeaderParts(file, header)
		dot := strings.LastIndex(path, ".")
		if wildcard || dot < 0 {
			return false
		}
		simple := path[dot+1:]
		if alias != "" {
			if alias != name {
				return false
			}
			ref, imported = path, true
			return true
		}
		if simple == name {
			ref, imported = path, true
			return true
		}
		return false
	}
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "import_list":
			for header := file.FlatFirstChild(child); header != 0; header = file.FlatNextSib(header) {
				if file.FlatType(header) == "import_header" && visit(header) {
					return ref, imported
				}
			}
		case "import_header":
			if visit(child) {
				return ref, imported
			}
		case "class_declaration", "object_declaration", "function_declaration", "property_declaration", "type_alias":
			return "", false
		}
	}
	return "", false
}

// fileHeaderLine reports whether a trimmed line can sit in the file header, before the first
// declaration: blank, a comment, a file annotation, or the package or an import directive.
func fileHeaderLine(trimmed string) bool {
	return trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "@file:") ||
		strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "import ")
}

// Packages declaring the DI annotation each binding rule means, as its FIR checker matches by class id.
var (
	daggerOrMetroBindsPackages   = []string{"dagger", "dev.zacsweers.metro"}
	diProvidesPackages           = []string{"dagger", "dev.zacsweers.metro", "me.tatarka.inject.annotations"}
	daggerOrMetroIntoMapPackages = []string{"dagger.multibindings", "dev.zacsweers.metro"}
	diIntoSetPackages            = []string{"dagger.multibindings", "dev.zacsweers.metro", "me.tatarka.inject.annotations"}
)

// hasDIAnnotationFlat is hasAnnotationFlat for a DI annotation declared in one of packages. An
// unqualified use does not count when the file imports name from another package or declares its own
// annotation class name without importing it, since then the annotation is not the DI framework's.
func hasDIAnnotationFlat(file *scanner.File, idx uint32, name string, packages []string) bool {
	// The simple name also matches a qualified use, so this rejects most functions in one scan.
	if !hasAnnotationFlat(file, idx, name) {
		return false
	}
	for _, pkg := range packages {
		if hasAnnotationFlat(file, idx, pkg+"."+name) {
			return true
		}
	}
	if pkg, imported := diImportedPackage(file, name); imported {
		return slices.Contains(packages, pkg)
	}
	if slices.Contains(packages, filePackageName(file)) {
		return true
	}
	local := localAnnotationClassHeader(file, name)
	return !local
}

// filePackageName returns the file's package, or "" for the default package.
func filePackageName(file *scanner.File) string {
	for _, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if !fileHeaderLine(trimmed) || strings.HasPrefix(trimmed, "import ") {
			break
		}
		if fields := strings.Fields(trimmed); len(fields) >= 2 && fields[0] == "package" {
			return strings.TrimSuffix(fields[1], ";")
		}
	}
	return ""
}

// MissingJvmSuppressWildcardsRule detects @Provides/@Binds functions taking
// Set<T> or Map<K, V> parameters without @JvmSuppressWildcards on the value type. Without
// the annotation, the Kotlin compiler emits Set<? extends T> in JVM signatures
// and Dagger fails to find matching multibindings.
type MissingJvmSuppressWildcardsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *MissingJvmSuppressWildcardsRule) Confidence() float64 { return api.ConfidenceMedium }

// fileImportsMetro reports whether the file imports the Metro DI runtime.
func fileImportsMetro(file *scanner.File) bool {
	for _, line := range file.Lines {
		if strings.HasPrefix(strings.TrimSpace(line), "import dev.zacsweers.metro") {
			return true
		}
	}
	return false
}

// functionValueParameterTypeTexts returns the declared type text of each value parameter of fn,
// including annotations written on the type itself.
func functionValueParameterTypeTexts(file *scanner.File, fn uint32) []string {
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return nil
	}
	var types []string
	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if file.FlatType(param) != "parameter" {
			continue
		}
		_, typ, found := strings.Cut(file.FlatNodeText(param), ":")
		if !found {
			continue
		}
		if eq := strings.Index(typ, "="); eq >= 0 && !strings.Contains(typ[:eq], "<") {
			typ = typ[:eq]
		}
		types = append(types, strings.TrimSpace(typ))
	}
	return types
}

var typeAnnotationPrefixRe = lazyre.New(`^(@[A-Za-z_][\w.]*(\([^)]*\))?\s+)+`)

// kotlinFinalTypes are final JVM types, so Kotlin emits no wildcard when one is a covariant type argument.
var kotlinFinalTypes = map[string]bool{
	"String": true, "Int": true, "Long": true, "Short": true, "Byte": true, "Char": true,
	"Boolean": true, "Float": true, "Double": true, "Unit": true, "Pair": true, "Triple": true,
}

// fileFinalClassNames returns the simple names of classes in the file that are final on the JVM:
// classes without open, abstract or sealed, and objects. Interfaces are excluded.
func fileFinalClassNames(file *scanner.File) map[string]bool {
	names := map[string]bool{}
	file.FlatWalkNodes(0, "class_declaration", func(decl uint32) {
		if file.FlatHasChildOfType(decl, "interface") || file.FlatHasModifier(decl, "open") ||
			file.FlatHasModifier(decl, "abstract") || file.FlatHasModifier(decl, "sealed") ||
			file.FlatHasModifier(decl, "annotation") {
			return
		}
		if name := extractIdentifierFlat(file, decl); name != "" {
			names[name] = true
		}
	})
	return names
}

// multibindingReturnNeedsJvmSuppress reports the first Set or Map in typeText, at any nesting depth,
// whose covariant value type argument would get a `? extends` wildcard in the JVM signature.
func multibindingReturnNeedsJvmSuppress(typeText string, finalTypes map[string]bool) (string, bool) {
	s := strings.TrimSpace(typeText)
	if s == "" {
		return "", false
	}
	// @JvmSuppressWildcards on the type itself covers every type argument.
	prefix := typeAnnotationPrefixRe.FindString(s)
	if strings.Contains(prefix, "@JvmSuppressWildcards") {
		return "", false
	}
	s = s[len(prefix):]
	for i := 0; i < len(s); i++ {
		if s[i] != '<' {
			continue
		}
		start := i
		for start > 0 && (isIdentByte(s[start-1])) {
			start--
		}
		bare := s[start:i]
		if bare != "Set" && bare != "MutableSet" && bare != "Map" && bare != "MutableMap" {
			continue
		}
		if start > 0 && s[start-1] != '.' && s[start-1] != '<' && s[start-1] != ',' && s[start-1] != ' ' {
			continue
		}
		args, ok := splitTypeArguments(s[i:])
		if !ok || len(args) == 0 {
			continue
		}
		value := strings.TrimSpace(args[0])
		if strings.HasSuffix(bare, "Map") {
			if len(args) < 2 {
				continue
			}
			value = strings.TrimSpace(args[len(args)-1])
		}
		if strings.Contains(value, "@JvmSuppressWildcards") {
			continue
		}
		if !strings.HasPrefix(value, "out ") && multibindingValueIsFinal(value, finalTypes) {
			continue
		}
		return bare, true
	}
	return "", false
}

// multibindingValueIsFinal reports a value type that Kotlin never wraps in a wildcard.
func multibindingValueIsFinal(value string, finalTypes map[string]bool) bool {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "?"))
	if lt := strings.Index(value, "<"); lt >= 0 {
		value = value[:lt]
	}
	if dot := strings.LastIndex(value, "."); dot >= 0 {
		value = value[dot+1:]
	}
	return kotlinFinalTypes[value] || finalTypes[value]
}

// splitTypeArguments splits the top-level arguments of a type argument list that starts with `<`.
func splitTypeArguments(s string) ([]string, bool) {
	depth := 0
	start := 1
	var args []string
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<', '(':
			depth++
		case ')':
			depth--
		case '>':
			if i > 0 && s[i-1] == '-' {
				continue
			}
			depth--
			if depth == 0 {
				return append(args, s[start:i]), true
			}
		case ',':
			if depth == 1 {
				args = append(args, s[start:i])
				start = i + 1
			}
		}
	}
	return nil, false
}

// ModuleWithNonStaticProvidesRule detects @Module abstract class declarations
// that mix @Binds (abstract instance methods) with @Provides functions at the
// top level. The @Provides functions belong in a companion object so Dagger
// can call them statically.
type ModuleWithNonStaticProvidesRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *ModuleWithNonStaticProvidesRule) Confidence() float64 { return api.ConfidenceMedium }

// IntoMapMissingKeyRule detects @IntoMap @Provides/@Binds functions that lack
// a `@*Key(...)` annotation. Without a key annotation, Dagger fails at code
// generation.
type IntoMapMissingKeyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *IntoMapMissingKeyRule) Confidence() float64 { return api.ConfidenceMedium }

// hasMapKeyAnnotationFlat returns true when any annotation on idx is named
// `@*Key` (e.g. @StringKey, @ClassKey, @IntKey, custom @FooKey). Built-in
// non-key annotations whose names happen to end in "Key" are filtered out.
func hasMapKeyAnnotationFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		if mapKeyAnnotationInParentFlat(file, mods) {
			return true
		}
	}
	return mapKeyAnnotationInParentFlat(file, idx)
}

func mapKeyAnnotationInParentFlat(file *scanner.File, parent uint32) bool {
	for i := 0; i < file.FlatChildCount(parent); i++ {
		child := file.FlatChild(parent, i)
		switch file.FlatType(child) {
		case "annotation", "modifier":
			text := strings.TrimSpace(file.FlatNodeText(child))
			name := strings.TrimPrefix(text, "@")
			if parenIdx := strings.Index(name, "("); parenIdx >= 0 {
				name = name[:parenIdx]
			}
			if colonIdx := strings.Index(name, ":"); colonIdx >= 0 {
				name = name[:colonIdx]
			}
			name = strings.TrimSpace(name)
			if dotIdx := strings.LastIndex(name, "."); dotIdx >= 0 {
				name = name[dotIdx+1:]
			}
			if name == "" || name == "Key" || name == "MapKey" {
				continue
			}
			if strings.HasSuffix(name, "Key") {
				return true
			}
		}
	}
	return false
}

// IntoSetOnNonSetReturnRule detects @IntoSet @Provides functions whose return
// type is a collection wrapper (List/Set/Map/Collection/Iterable/Array). Dagger
// multibindings collect by return type, so wrapping the contribution in a
// collection drops the intended elements.
type IntoSetOnNonSetReturnRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *IntoSetOnNonSetReturnRule) Confidence() float64 { return api.ConfidenceMedium }

var intoSetCollectionWrapperTypes = map[string]struct{}{
	"List":              {},
	"MutableList":       {},
	"ArrayList":         {},
	"Set":               {},
	"MutableSet":        {},
	"HashSet":           {},
	"LinkedHashSet":     {},
	"Map":               {},
	"MutableMap":        {},
	"HashMap":           {},
	"LinkedHashMap":     {},
	"Collection":        {},
	"MutableCollection": {},
	"Iterable":          {},
	"MutableIterable":   {},
	"Array":             {},
}

func intoSetReturnIsCollectionWrapper(typeText string) (string, bool) {
	s := strings.TrimSpace(typeText)
	if s == "" {
		return "", false
	}
	s = strings.TrimSuffix(s, "?")
	s = strings.TrimSpace(s)
	bare := s
	if idx := strings.Index(bare, "<"); idx >= 0 {
		bare = bare[:idx]
	}
	bare = strings.TrimSpace(bare)
	if dotIdx := strings.LastIndex(bare, "."); dotIdx >= 0 {
		bare = bare[dotIdx+1:]
	}
	if _, ok := intoSetCollectionWrapperTypes[bare]; ok {
		return bare, true
	}
	return "", false
}

var singletonThreadSafeMarkers = []string{
	"@Volatile", "@GuardedBy", "Synchronized", "by atomic", "Atomic", "Mutex", "MutableStateFlow", "MutableSharedFlow",
}

// singletonVarIsProtected reports whether a singleton var has an atomic or lock-based guard.
func singletonVarIsProtected(file *scanner.File, classIdx, prop uint32, propName string) bool {
	text := file.FlatNodeText(prop)
	for _, marker := range singletonThreadSafeMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return strings.Contains(text, "private") && mutableObjectPropertyReferencesAreSynchronized(file, classIdx, prop, propName)
}

var singletonThreadAnnotations = []string{"MainThread", "UiThread", "WorkerThread"}

var singletonConfinementParamNames = map[string]bool{
	"mainThreadHandler": true, "mainHandler": true, "uiHandler": true,
	"mainDispatcher": true, "uiDispatcher": true, "mainThreadDispatcher": true,
}

var singletonConfinementParamTypes = map[string]bool{"Handler": true, "MainCoroutineDispatcher": true}

// singletonClassIsThreadConfined reports a class whose constructor takes a main-thread handler or
// dispatcher, or that carries a thread annotation itself. Its state is assumed to be touched from one thread.
func singletonClassIsThreadConfined(file *scanner.File, classIdx uint32) bool {
	for _, a := range singletonThreadAnnotations {
		if hasAnnotationFlat(file, classIdx, a) {
			return true
		}
	}
	ctor, ok := file.FlatFindChild(classIdx, "primary_constructor")
	if !ok {
		return false
	}
	confined := false
	file.FlatWalkNodes(ctor, "class_parameter", func(param uint32) {
		name := extractIdentifierFlat(file, param)
		_, typ, _ := strings.Cut(file.FlatNodeText(param), ":")
		typ = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(typ), "?"))
		if i := strings.LastIndex(typ, "."); i >= 0 {
			typ = typ[i+1:]
		}
		if singletonConfinementParamNames[name] || singletonConfinementParamTypes[typ] {
			confined = true
		}
	})
	return confined
}

// singletonVarOnlyTouchedByThreadAnnotatedMembers reports a property that is referenced only inside
// functions annotated @MainThread, @UiThread or @WorkerThread.
func singletonVarOnlyTouchedByThreadAnnotatedMembers(file *scanner.File, classIdx, prop uint32, propName string) bool {
	seen := false
	all := true
	file.FlatWalkNodes(classIdx, "simple_identifier", func(ref uint32) {
		if !all || !file.FlatNodeTextEquals(ref, propName) || flatNodeHasAncestor(file, ref, prop) {
			return
		}
		seen = true
		fn, ok := flatEnclosingFunction(file, ref)
		if !ok {
			all = false
			return
		}
		for _, a := range singletonThreadAnnotations {
			if hasAnnotationFlat(file, fn, a) {
				return
			}
		}
		all = false
	})
	return seen && all
}

// singletonClassAssignsProperty reports whether the class body assigns the named property, as `name = x`,
// `this.name = x` or a compound assignment such as `name += x`.
func singletonClassAssignsProperty(file *scanner.File, classIdx uint32, propName string) bool {
	found := false
	file.FlatWalkNodes(classIdx, "assignment", func(assign uint32) {
		if found {
			return
		}
		target := file.FlatFirstChild(assign)
		if target == 0 {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(target))
		found = text == propName || text == "this."+propName
	})
	return found
}
