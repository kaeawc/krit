package rules

import (
	"bytes"
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const diHygieneRuleSet = "di-hygiene"

var (
	anvilContributesToToken      = []byte("ContributesTo")
	anvilContributesBindingToken = []byte("ContributesBinding")
	anvilMergeComponentToken     = []byte("MergeComponent")
)

func anvilMergeComponentMayMatch(content []byte) bool {
	return bytes.Contains(content, anvilContributesToToken) ||
		bytes.Contains(content, anvilContributesBindingToken) ||
		bytes.Contains(content, anvilMergeComponentToken)
}

func anvilScopeDeclarationCandidates(file *scanner.File) []uint32 {
	var matches []uint32
	if file == nil || file.FlatTree == nil {
		return matches
	}
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		if nodeType != "class_declaration" && nodeType != "object_declaration" {
			return
		}
		if anvilModifiersMayMatchFlat(file, idx) {
			matches = append(matches, idx)
		}
	})
	return matches
}

func anvilModifiersMayMatchFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	segment := file.Content[int(file.FlatStartByte(mods)):int(file.FlatEndByte(mods))]
	return bytes.Contains(segment, anvilContributesToToken) ||
		bytes.Contains(segment, anvilContributesBindingToken) ||
		bytes.Contains(segment, anvilMergeComponentToken)
}

// BindsMismatchedArityRule detects @Binds functions that do not declare exactly
// one parameter, which Dagger rejects during code generation.
type BindsMismatchedArityRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule. Detection uses annotation and import patterns for
// Dagger/Hilt/Anvil; project-specific DI aliases are not followed.
// Classified per roadmap/17.
func (r *BindsMismatchedArityRule) Confidence() float64 { return 0.75 }

func anvilContributedInterfaceScopesFlat(file *scanner.File) map[string]string {
	scopes := make(map[string]string)
	if file == nil || file.FlatTree == nil {
		return scopes
	}
	file.FlatWalkNodes(0, "class_declaration", func(idx uint32) {
		if _, ok := file.FlatFindChild(idx, "interface"); !ok {
			return
		}
		scope := anvilAnnotationScopeFlat(file, idx, "ContributesTo")
		if scope == "" {
			return
		}
		name := extractIdentifierFlat(file, idx)
		if name == "" {
			return
		}
		scopes[name] = scope
	})
	return scopes
}

func anvilAnnotationScopeFlat(file *scanner.File, idx uint32, annotationName string) string {
	annotationText := findAnnotationTextFlat(file, idx, annotationName)
	if annotationText == "" {
		return ""
	}

	openIdx := strings.Index(annotationText, "(")
	closeIdx := strings.LastIndex(annotationText, ")")
	if openIdx < 0 || closeIdx <= openIdx {
		return ""
	}

	scopeExpr := strings.TrimSpace(annotationText[openIdx+1 : closeIdx])
	if eqIdx := strings.Index(scopeExpr, "="); eqIdx >= 0 {
		scopeExpr = strings.TrimSpace(scopeExpr[eqIdx+1:])
	}
	if commaIdx := strings.Index(scopeExpr, ","); commaIdx >= 0 {
		scopeExpr = scopeExpr[:commaIdx]
	}
	if classIdx := strings.Index(scopeExpr, "::class"); classIdx >= 0 {
		scopeExpr = scopeExpr[:classIdx]
	}
	scopeExpr = strings.TrimSpace(scopeExpr)
	if dotIdx := strings.LastIndex(scopeExpr, "."); dotIdx >= 0 {
		scopeExpr = scopeExpr[dotIdx+1:]
	}
	return scopeExpr
}

func findAnnotationTextFlat(file *scanner.File, idx uint32, annotationName string) string {
	if file == nil || idx == 0 {
		return ""
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		if text := findAnnotationTextInFlatParent(file, mods, annotationName); text != "" {
			return text
		}
	}
	return findAnnotationTextInFlatParent(file, idx, annotationName)
}

func findAnnotationTextInFlatParent(file *scanner.File, parent uint32, annotationName string) string {
	target := "@" + annotationName
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
			if name == annotationName || strings.HasPrefix(name, annotationName+".") || strings.Contains(text, target) {
				return text
			}
		}
	}
	return ""
}

// DeadBindingsRule detects @Provides/@Binds functions whose return type is not
// requested by any @Inject site or component exposure anywhere in the project.
type DeadBindingsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (lower) base confidence. Reachability is approximated
// from source-visible @Inject sites and component method exposures; project-specific
// DI machinery (assisted factories, multibindings, generated code) is not modeled.
func (r *DeadBindingsRule) Confidence() float64 { return 0.5 }

var deadBindingComponentAnnotations = []string{
	"Component", "Subcomponent", "MergeComponent", "MergeSubcomponent",
	"DefineComponent", "EntryPoint", "EarlyEntryPoint", "GraphExtension",
	"DependencyGraph", "ContributesSubcomponent", "ComponentExtension",
}

func (r *DeadBindingsRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}
	files := index.Files
	if len(files) == 0 {
		return
	}

	type bindingCandidate struct {
		file    *scanner.File
		idx     uint32
		name    string
		retType string
	}
	var bindings []bindingCandidate
	demand := make(map[string]struct{})

	addDemandTypes := func(types []string) {
		for _, t := range types {
			if t == "" {
				continue
			}
			demand[t] = struct{}{}
		}
	}

	for _, file := range files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			switch file.FlatType(idx) {
			case "function_declaration":
				isProvides := hasAnnotationFlat(file, idx, "Provides")
				isBinds := hasAnnotationFlat(file, idx, "Binds")
				inComponent := isInsideComponentInterfaceFlat(file, idx)
				if isProvides || isBinds {
					ret := lastTypeSegment(extractFunctionReturnTypeNameFlat(file, idx))
					if ret != "" {
						bindings = append(bindings, bindingCandidate{
							file:    file,
							idx:     idx,
							name:    extractIdentifierFlat(file, idx),
							retType: ret,
						})
					}
					addDemandTypes(extractFunctionParameterTypeNamesFlat(file, idx))
				}
				if inComponent {
					if ret := lastTypeSegment(extractFunctionReturnTypeNameFlat(file, idx)); ret != "" {
						demand[ret] = struct{}{}
					}
					addDemandTypes(extractFunctionParameterTypeNamesFlat(file, idx))
				}
			case "primary_constructor":
				if hasAnnotationFlat(file, idx, "Inject") {
					addDemandTypes(extractClassParameterTypeNamesFlat(file, idx))
				}
			case "secondary_constructor":
				if hasAnnotationFlat(file, idx, "Inject") {
					addDemandTypes(extractFunctionParameterTypeNamesFlat(file, idx))
				}
			case "property_declaration":
				if hasAnnotationFlat(file, idx, "Inject") {
					if name := lastTypeSegment(extractPropertyTypeNameFlat(file, idx)); name != "" {
						demand[name] = struct{}{}
					}
				}
			}
		})
	}

	for _, b := range bindings {
		if _, ok := demand[b.retType]; ok {
			continue
		}
		name := b.name
		if name == "" {
			name = "binding"
		}
		ctx.Emit(r.Finding(
			b.file,
			b.file.FlatRow(b.idx)+1,
			1,
			fmt.Sprintf("@Provides/@Binds function '%s' returning '%s' is not requested by any @Inject site or component exposure in the project.", name, b.retType),
		))
	}
}

func extractFunctionReturnTypeNameFlat(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	seenColon := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		if t == ":" {
			seenColon = true
			continue
		}
		if !seenColon {
			continue
		}
		switch t {
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			return strings.TrimSpace(file.FlatNodeText(child))
		case "function_body", "=":
			return ""
		}
	}
	return ""
}

func extractPropertyTypeNameFlat(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "variable_declaration":
			if name := extractParameterTypeNameFlat(file, child); name != "" {
				return name
			}
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func extractParameterTypeNameFlat(file *scanner.File, param uint32) string {
	if file == nil || param == 0 {
		return ""
	}
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func extractFunctionParameterTypeNamesFlat(file *scanner.File, fn uint32) []string {
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return nil
	}
	var names []string
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		if name := lastTypeSegment(extractParameterTypeNameFlat(file, child)); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func extractClassParameterTypeNamesFlat(file *scanner.File, ctor uint32) []string {
	if file == nil || ctor == 0 {
		return nil
	}
	var names []string
	for child := file.FlatFirstChild(ctor); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "class_parameter" {
			continue
		}
		if name := lastTypeSegment(extractParameterTypeNameFlat(file, child)); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func lastTypeSegment(typeText string) string {
	s := strings.TrimSpace(typeText)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, "<"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSuffix(s, "?")
	s = strings.TrimSpace(s)
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		s = s[idx+1:]
	}
	return strings.TrimSpace(s)
}

func isInsideComponentInterfaceFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) != "class_declaration" {
			continue
		}
		if !file.FlatHasChildOfType(parent, "interface") {
			continue
		}
		for _, ann := range deadBindingComponentAnnotations {
			if hasAnnotationFlat(file, parent, ann) {
				return true
			}
		}
	}
	return false
}

// InjectOnAbstractClassRule detects @Inject primary constructors on abstract
// classes. Dagger cannot instantiate an abstract class, so the binding is
// unreachable.
type InjectOnAbstractClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *InjectOnAbstractClassRule) Confidence() float64 { return 0.75 }

// ProviderInsteadOfLazyRule detects constructor parameters typed Provider<T>
// whose `.get()` is called exactly once across the class body. Lazy<T> matches
// the same intent and is cheaper.
type ProviderInsteadOfLazyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *ProviderInsteadOfLazyRule) Confidence() float64 { return 0.6 }

// LazyInsteadOfDirectRule detects constructor parameters typed Lazy<T> whose
// `.get()` is called unconditionally at class-init time. Direct injection is
// cheaper because the value is already eagerly required.
type LazyInsteadOfDirectRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *LazyInsteadOfDirectRule) Confidence() float64 { return 0.6 }

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

// SubcomponentNotInstalledRule detects `@Subcomponent` declarations that are
// never returned from any parent `@Component` or parent `@Subcomponent`
// method, leaving the subcomponent orphaned.
type SubcomponentNotInstalledRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (lower) base confidence. Detection follows
// return-type text only; cross-module install graphs are not modeled.
func (r *SubcomponentNotInstalledRule) Confidence() float64 { return 0.5 }

func (r *SubcomponentNotInstalledRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	type subDecl struct {
		file *scanner.File
		idx  uint32
		name string
	}
	var subcomponents []subDecl
	type ref struct {
		ownerName string
	}
	referencesByName := make(map[string][]ref)

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Component")) &&
			!bytes.Contains(file.Content, []byte("Subcomponent")) {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "class_declaration" {
				return
			}
			isSubcomponent := hasAnnotationFlat(file, idx, "Subcomponent") ||
				hasAnnotationFlat(file, idx, "MergeSubcomponent")
			isComponent := hasAnnotationFlat(file, idx, "Component") ||
				hasAnnotationFlat(file, idx, "MergeComponent")
			if isSubcomponent {
				name := extractIdentifierFlat(file, idx)
				if name != "" {
					subcomponents = append(subcomponents, subDecl{file: file, idx: idx, name: name})
				}
			}
			if isSubcomponent || isComponent {
				ownerName := extractIdentifierFlat(file, idx)
				returns := collectComponentReturnTypeNames(file, idx)
				for n := range returns {
					referencesByName[n] = append(referencesByName[n], ref{ownerName: ownerName})
				}
			}
		})
	}

	for _, sub := range subcomponents {
		installed := false
		for _, rf := range referencesByName[sub.name] {
			if rf.ownerName != sub.name {
				installed = true
				break
			}
		}
		if installed {
			continue
		}
		ctx.Emit(r.Finding(
			sub.file,
			sub.file.FlatRow(sub.idx)+1,
			1,
			fmt.Sprintf("@Subcomponent '%s' is never returned from any parent @Component or @Subcomponent method; the subcomponent is orphaned.", sub.name),
		))
	}
}

// collectComponentReturnTypeNames walks the body of a component class
// declaration and returns the set of last-segment names referenced by every
// method or property return type (including generic args and nested types).
func collectComponentReturnTypeNames(file *scanner.File, classIdx uint32) map[string]struct{} {
	out := make(map[string]struct{})
	body, ok := file.FlatFindChild(classIdx, "class_body")
	if !ok || body == 0 {
		return out
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_declaration":
			addTypeNameSegments(extractFunctionReturnTypeNameFlat(file, child), out)
		case "property_declaration":
			addTypeNameSegments(extractPropertyTypeNameFlat(file, child), out)
		}
	}
	return out
}

func addTypeNameSegments(typeText string, out map[string]struct{}) {
	if typeText == "" {
		return
	}
	if name := lastTypeSegment(typeText); name != "" {
		out[name] = struct{}{}
	}
	if open := strings.Index(typeText, "<"); open >= 0 && strings.HasSuffix(strings.TrimSpace(typeText), ">") {
		end := strings.LastIndex(typeText, ">")
		inner := typeText[open+1 : end]
		for _, part := range strings.Split(inner, ",") {
			addTypeNameSegments(strings.TrimSpace(part), out)
		}
	}
	// Nested type reference like `UserSubcomponent.Factory` — also add the
	// outer class so the parent-side reference satisfies the subcomponent
	// declaration.
	if dot := strings.Index(typeText, "."); dot >= 0 {
		head := strings.TrimSpace(typeText[:dot])
		if head != "" {
			out[head] = struct{}{}
		}
	}
}

// BindsInsteadOfProvidesRule detects `@Provides` functions with a single
// parameter whose body simply returns that parameter unchanged. The function
// can be expressed as a cheaper `@Binds` abstract method.
type BindsInsteadOfProvidesRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *BindsInsteadOfProvidesRule) Confidence() float64 { return 0.7 }

// BindsReturnTypeMatchesParamRule detects `@Binds` functions whose parameter
// type equals the return type — a no-op binding that Dagger rejects.
type BindsReturnTypeMatchesParamRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *BindsReturnTypeMatchesParamRule) Confidence() float64 { return 0.75 }

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

// ComponentMissingModuleRule detects `@Component(modules = [...])`
// declarations whose listed modules do not transitively cover every binding
// reachable through the component's exposed methods.
type ComponentMissingModuleRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (lower) base confidence. Detection follows
// @Provides/@Binds return types and @Inject constructor parameters in source;
// generated code, multibindings, and assisted factories are not modeled.
func (r *ComponentMissingModuleRule) Confidence() float64 { return 0.5 }

type componentMissingModuleProvider struct {
	moduleName string
	paramTypes []string
}

type componentMissingModuleComponent struct {
	file        *scanner.File
	idx         uint32
	name        string
	modules     map[string]struct{}
	returnTypes []string
}

func (r *ComponentMissingModuleRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	providersByType := make(map[string][]componentMissingModuleProvider)
	injectCtorParams := make(map[string][]string)
	var components []componentMissingModuleComponent

	for _, file := range index.Files {
		if !componentMissingModuleFileMayMatch(file) {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			t := file.FlatType(idx)
			if t != "class_declaration" && t != "object_declaration" {
				return
			}
			if hasAnnotationFlat(file, idx, "Module") {
				collectModuleProviders(file, idx, providersByType)
			}
			if hasAnnotationFlat(file, idx, "Component") || hasAnnotationFlat(file, idx, "MergeComponent") {
				components = append(components, collectComponentInfo(file, idx))
			}
			if name := extractIdentifierFlat(file, idx); name != "" && hasAnyInjectPrimaryConstructor(file, idx) {
				ctor, _ := file.FlatFindChild(idx, "primary_constructor")
				injectCtorParams[name] = extractClassParameterTypeNamesFlat(file, ctor)
			}
		})
	}

	for _, ci := range components {
		r.emitMissingModulesForComponent(ctx, ci, providersByType, injectCtorParams)
	}
}

func componentMissingModuleFileMayMatch(file *scanner.File) bool {
	if file == nil || file.FlatTree == nil {
		return false
	}
	return bytes.Contains(file.Content, []byte("Component")) ||
		bytes.Contains(file.Content, []byte("Module")) ||
		bytes.Contains(file.Content, []byte("Inject"))
}

func collectModuleProviders(file *scanner.File, classIdx uint32, providersByType map[string][]componentMissingModuleProvider) {
	moduleName := extractIdentifierFlat(file, classIdx)
	body, ok := file.FlatFindChild(classIdx, "class_body")
	if !ok || body == 0 {
		return
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_declaration":
			recordModuleProvider(file, child, moduleName, providersByType)
		case "companion_object", "object_declaration":
			recordCompanionProviders(file, child, moduleName, providersByType)
		}
	}
}

func recordModuleProvider(file *scanner.File, fn uint32, moduleName string, providersByType map[string][]componentMissingModuleProvider) {
	if !hasAnnotationFlat(file, fn, "Provides") && !hasAnnotationFlat(file, fn, "Binds") {
		return
	}
	ret := lastTypeSegment(extractFunctionReturnTypeNameFlat(file, fn))
	if ret == "" {
		return
	}
	providersByType[ret] = append(providersByType[ret], componentMissingModuleProvider{
		moduleName: moduleName,
		paramTypes: extractFunctionParameterTypeNamesFlat(file, fn),
	})
}

func recordCompanionProviders(file *scanner.File, comp uint32, moduleName string, providersByType map[string][]componentMissingModuleProvider) {
	for inner := file.FlatFirstChild(comp); inner != 0; inner = file.FlatNextSib(inner) {
		if file.FlatType(inner) != "class_body" {
			continue
		}
		for fn := file.FlatFirstChild(inner); fn != 0; fn = file.FlatNextSib(fn) {
			if file.FlatType(fn) != "function_declaration" {
				continue
			}
			recordModuleProvider(file, fn, moduleName, providersByType)
		}
	}
}

func collectComponentInfo(file *scanner.File, classIdx uint32) componentMissingModuleComponent {
	ci := componentMissingModuleComponent{
		file:    file,
		idx:     classIdx,
		name:    extractIdentifierFlat(file, classIdx),
		modules: parseComponentModulesAttr(file, classIdx),
	}
	body, ok := file.FlatFindChild(classIdx, "class_body")
	if !ok || body == 0 {
		return ci
	}
	for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "function_declaration":
			if ret := lastTypeSegment(extractFunctionReturnTypeNameFlat(file, c)); ret != "" {
				ci.returnTypes = append(ci.returnTypes, ret)
			}
		case "property_declaration":
			if ret := lastTypeSegment(extractPropertyTypeNameFlat(file, c)); ret != "" {
				ci.returnTypes = append(ci.returnTypes, ret)
			}
		}
	}
	return ci
}

func (r *ComponentMissingModuleRule) emitMissingModulesForComponent(
	ctx *api.Context,
	ci componentMissingModuleComponent,
	providersByType map[string][]componentMissingModuleProvider,
	injectCtorParams map[string][]string,
) {
	if len(ci.modules) == 0 {
		return
	}
	seen := make(map[string]struct{})
	queue := append([]string{}, ci.returnTypes...)
	emitted := make(map[string]struct{})
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		for _, p := range providersByType[t] {
			r.maybeEmitMissingModule(ctx, ci, p, emitted)
			queue = append(queue, p.paramTypes...)
		}
		queue = append(queue, injectCtorParams[t]...)
	}
}

func (r *ComponentMissingModuleRule) maybeEmitMissingModule(
	ctx *api.Context,
	ci componentMissingModuleComponent,
	p componentMissingModuleProvider,
	emitted map[string]struct{},
) {
	if p.moduleName == "" {
		return
	}
	if _, listed := ci.modules[p.moduleName]; listed {
		return
	}
	if _, already := emitted[p.moduleName]; already {
		return
	}
	emitted[p.moduleName] = struct{}{}
	compName := ci.name
	if compName == "" {
		compName = "component"
	}
	ctx.Emit(r.Finding(
		ci.file,
		ci.file.FlatRow(ci.idx)+1,
		1,
		fmt.Sprintf("@Component '%s' transitively requires bindings from '%s', which is not listed in `modules = [...]`.", compName, p.moduleName),
	))
}

// parseComponentModulesAttr returns the set of simple module names listed in
// `@Component(modules = [A::class, B::class])`. Empty if no modules attr.
func parseComponentModulesAttr(file *scanner.File, idx uint32) map[string]struct{} {
	out := make(map[string]struct{})
	text := findAnnotationTextFlat(file, idx, "Component")
	if text == "" {
		text = findAnnotationTextFlat(file, idx, "MergeComponent")
	}
	if text == "" {
		return out
	}
	openIdx := strings.Index(text, "[")
	closeIdx := strings.LastIndex(text, "]")
	if openIdx < 0 || closeIdx <= openIdx {
		return out
	}
	inner := text[openIdx+1 : closeIdx]
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		if i := strings.Index(part, "::"); i >= 0 {
			part = strings.TrimSpace(part[:i])
		}
		if dot := strings.LastIndex(part, "."); dot >= 0 {
			part = part[dot+1:]
		}
		if part != "" {
			out[part] = struct{}{}
		}
	}
	return out
}

func hasAnyInjectPrimaryConstructor(file *scanner.File, idx uint32) bool {
	ctor, ok := file.FlatFindChild(idx, "primary_constructor")
	if !ok || ctor == 0 {
		return false
	}
	return hasAnnotationFlat(file, ctor, "Inject")
}

// firstMapKeyAnnotationLiteralFlat returns the literal argument of the first
// `@*Key(...)` annotation on idx (e.g. `"foo"` for `@StringKey("foo")`).
// Returns ("", false) if no key annotation or no parenthesized argument.
func firstMapKeyAnnotationLiteralFlat(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 {
		return "", false
	}
	parents := []uint32{idx}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		parents = append(parents, mods)
	}
	for _, parent := range parents {
		for i := 0; i < file.FlatChildCount(parent); i++ {
			child := file.FlatChild(parent, i)
			t := file.FlatType(child)
			if t != "annotation" && t != "modifier" {
				continue
			}
			text := strings.TrimSpace(file.FlatNodeText(child))
			name := strings.TrimPrefix(text, "@")
			parenIdx := strings.Index(name, "(")
			if parenIdx < 0 {
				continue
			}
			argText := name[parenIdx+1:]
			closeIdx := strings.LastIndex(argText, ")")
			if closeIdx < 0 {
				continue
			}
			argText = strings.TrimSpace(argText[:closeIdx])
			name = name[:parenIdx]
			if colonIdx := strings.Index(name, ":"); colonIdx >= 0 {
				name = name[:colonIdx]
			}
			if dotIdx := strings.LastIndex(name, "."); dotIdx >= 0 {
				name = name[dotIdx+1:]
			}
			name = strings.TrimSpace(name)
			if name == "" || name == "Key" || name == "MapKey" {
				continue
			}
			if !strings.HasSuffix(name, "Key") {
				continue
			}
			if eq := strings.Index(argText, "="); eq >= 0 {
				argText = strings.TrimSpace(argText[eq+1:])
			}
			if argText == "" {
				continue
			}
			return argText, true
		}
	}
	return "", false
}

// enclosingClassChainFlat returns the dot-joined name of the chain of class /
// object declarations enclosing idx (outermost first). Returns "" if the
// enclosing scope is the file (top-level).
func enclosingClassChainFlat(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	var names []string
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "class_declaration", "object_declaration":
			if name := extractIdentifierFlat(file, parent); name != "" {
				names = append([]string{name}, names...)
			}
		}
	}
	return strings.Join(names, ".")
}

// SingletonOnMutableClassRule detects @Singleton (or other application-scoped)
// classes whose body declares unprotected mutable state — `var` properties or
// `val` properties initialised with a mutable collection factory.
type SingletonOnMutableClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *SingletonOnMutableClassRule) Confidence() float64 { return 0.75 }

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
func (r *MetroFactoryDeclarationShapeRule) Confidence() float64 { return 0.75 }

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
func (r *ScopeOnParameterizedClassRule) Confidence() float64 { return 0.75 }

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
func (r *MissingJvmSuppressWildcardsRule) Confidence() float64 { return 0.75 }

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
func (r *ModuleWithNonStaticProvidesRule) Confidence() float64 { return 0.75 }

// IntoMapMissingKeyRule detects @IntoMap @Provides/@Binds functions that lack
// a `@*Key(...)` annotation. Without a key annotation, Dagger fails at code
// generation.
type IntoMapMissingKeyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *IntoMapMissingKeyRule) Confidence() float64 { return 0.75 }

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
func (r *IntoSetOnNonSetReturnRule) Confidence() float64 { return 0.75 }

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

func anvilImplementedTypesFlat(file *scanner.File, idx uint32) []string {
	var names []string
	if file == nil || idx == 0 {
		return names
	}
	file.FlatForEachChild(idx, func(child uint32) {
		if file.FlatType(child) != "delegation_specifier" {
			return
		}
		if name := extractSupertypeNameFlat(file, child); name != "" {
			names = append(names, name)
		}
	})
	return names
}
