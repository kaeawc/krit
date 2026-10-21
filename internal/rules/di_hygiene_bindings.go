package rules

import (
	"strings"

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

// ScopeOnParameterizedClassRule detects scope annotations on generic classes,
// where the type parameter is erased at runtime so the scope holds a single
// instance regardless of the type argument.
type ScopeOnParameterizedClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *ScopeOnParameterizedClassRule) Confidence() float64 { return api.ConfidenceMedium }

// scopeAnnotationNames lists annotations that mark a DI scope. The list mirrors
// commonly used Hilt/Dagger scopes plus Anvil/Metro variants.
var scopeAnnotationNames = []string{
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
	for _, name := range scopeAnnotationNames {
		if hasAnnotationFlat(file, idx, name) {
			return name
		}
	}
	return ""
}

// MissingJvmSuppressWildcardsRule detects @Provides/@Binds functions returning
// Set<T> or Map<K, V> without @JvmSuppressWildcards on the value type. Without
// the annotation, the Kotlin compiler emits Set<? extends T> in JVM signatures
// and Dagger fails to find matching multibindings.
type MissingJvmSuppressWildcardsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *MissingJvmSuppressWildcardsRule) Confidence() float64 { return api.ConfidenceMedium }

func multibindingReturnNeedsJvmSuppress(typeText string) (string, bool) {
	s := strings.TrimSpace(typeText)
	if s == "" {
		return "", false
	}
	s = strings.TrimSuffix(s, "?")
	s = strings.TrimSpace(s)
	openIdx := strings.Index(s, "<")
	if openIdx < 0 {
		return "", false
	}
	bare := strings.TrimSpace(s[:openIdx])
	if dotIdx := strings.LastIndex(bare, "."); dotIdx >= 0 {
		bare = bare[dotIdx+1:]
	}
	switch bare {
	case "Set", "MutableSet", "Map", "MutableMap":
	default:
		return "", false
	}
	if !strings.HasSuffix(s, ">") {
		return "", false
	}
	inner := s[openIdx+1 : len(s)-1]
	if strings.Contains(inner, "@JvmSuppressWildcards") {
		return "", false
	}
	return bare, true
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
