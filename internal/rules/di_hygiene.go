package rules

import (
	"bytes"
	"fmt"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

const diHygieneRuleSet = "di-hygiene"

// AnvilMergeComponentEmptyScopeRule detects @MergeComponent scopes that have no
// matching @ContributesTo/@ContributesBinding declarations anywhere in the project.
type AnvilMergeComponentEmptyScopeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule. Detection uses annotation and import patterns for
// Dagger/Hilt/Anvil; project-specific DI aliases are not followed.
// Classified per roadmap/17.
func (r *AnvilMergeComponentEmptyScopeRule) Confidence() float64 { return 0.75 }

func (r *AnvilMergeComponentEmptyScopeRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}
	files := index.Files
	if len(files) == 0 {
		return
	}

	contributedScopes := make(map[string]struct{})
	type mergeComponentCandidate struct {
		file  *scanner.File
		idx   uint32
		scope string
	}
	var mergeComponents []mergeComponentCandidate

	for _, file := range files {
		if file == nil || file.FlatTree == nil || !anvilMergeComponentMayMatch(file.Content) {
			continue
		}
		for _, idx := range anvilScopeDeclarationCandidates(file) {
			if !anvilModifiersMayMatchFlat(file, idx) {
				continue
			}

			if scope := anvilAnnotationScopeFlat(file, idx, "ContributesTo"); scope != "" {
				contributedScopes[scope] = struct{}{}
			}
			if scope := anvilAnnotationScopeFlat(file, idx, "ContributesBinding"); scope != "" {
				contributedScopes[scope] = struct{}{}
			}
			if scope := anvilAnnotationScopeFlat(file, idx, "MergeComponent"); scope != "" {
				mergeComponents = append(mergeComponents, mergeComponentCandidate{
					file:  file,
					idx:   idx,
					scope: scope,
				})
			}
		}
	}

	for _, candidate := range mergeComponents {
		if _, ok := contributedScopes[candidate.scope]; ok {
			continue
		}

		name := extractIdentifierFlat(candidate.file, candidate.idx)
		if name == "" {
			name = "merged component"
		}

		ctx.Emit(r.Finding(
			candidate.file,
			candidate.file.FlatRow(candidate.idx)+1,
			1,
			fmt.Sprintf("@MergeComponent(%s::class) on '%s' has no matching @ContributesTo or @ContributesBinding scope in the project, so the merged component will be empty.", candidate.scope, name),
		))
	}
}

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

// AnvilContributesBindingWithoutScopeRule detects a same-file mismatch between
// @ContributesBinding(scope) and the @ContributesTo(scope) on the bound interface.
type AnvilContributesBindingWithoutScopeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule. Detection uses annotation and import patterns for
// Dagger/Hilt/Anvil; project-specific DI aliases are not followed.
// Classified per roadmap/17.
func (r *AnvilContributesBindingWithoutScopeRule) Confidence() float64 { return 0.75 }

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

// HiltEntryPointOnNonInterfaceRule detects Hilt entry points declared as a
// class or object instead of an interface.
type HiltEntryPointOnNonInterfaceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule. Detection uses annotation and import patterns for
// Dagger/Hilt/Anvil; project-specific DI aliases are not followed.
// Classified per roadmap/17.
func (r *HiltEntryPointOnNonInterfaceRule) Confidence() float64 { return 0.75 }

func hiltEntryPointDeclarationFlat(file *scanner.File, idx uint32) (kind, name string, line int, ok bool) {
	if file == nil || !hasAnnotationFlat(file, idx, "EntryPoint") {
		return "", "", 0, false
	}

	switch file.FlatType(idx) {
	case "class_declaration", "object_declaration":
		return hiltEntryPointDeclKindFlat(file, idx), extractIdentifierFlat(file, idx), file.FlatRow(idx) + 1, true
	case "prefix_expression":
		target := hiltEntryPointAnnotatedTargetFlat(file, idx)
		if target == 0 {
			return "", "", 0, false
		}
		switch file.FlatType(target) {
		case "class_declaration", "object_declaration":
			return hiltEntryPointDeclKindFlat(file, target), extractIdentifierFlat(file, target), file.FlatRow(idx) + 1, true
		case "infix_expression":
			return hiltEntryPointInfixDeclFlat(file, target)
		}
	}

	return "", "", 0, false
}

func hiltEntryPointAnnotatedTargetFlat(file *scanner.File, idx uint32) uint32 {
	current := idx
	for file != nil && file.FlatType(current) == "prefix_expression" {
		if file.FlatNamedChildCount(current) < 2 {
			return 0
		}
		current = file.FlatNamedChild(current, 1)
	}
	return current
}

func hiltEntryPointInfixDeclFlat(file *scanner.File, idx uint32) (kind, name string, line int, ok bool) {
	if file == nil || file.FlatNamedChildCount(idx) < 2 {
		return "", "", 0, false
	}

	kind = file.FlatNodeText(file.FlatNamedChild(idx, 0))
	if kind != "class" && kind != "interface" {
		return "", "", 0, false
	}

	name = file.FlatNodeText(file.FlatNamedChild(idx, 1))
	return kind, name, file.FlatRow(idx) + 1, true
}

func hiltEntryPointDeclKindFlat(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "object_declaration":
		return "object"
	case "class_declaration":
		if file.FlatHasChildOfType(idx, "interface") {
			return "interface"
		}
		return "class"
	default:
		return "class"
	}
}

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

func (r *DeadBindingsRule) check(ctx *v2.Context) {
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
