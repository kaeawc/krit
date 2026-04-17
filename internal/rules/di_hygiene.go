package rules

import (
	"bytes"
	"fmt"
	"strings"

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

func (r *AnvilMergeComponentEmptyScopeRule) CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding {
	if index == nil {
		return nil
	}
	return r.CheckParsedFiles(index.Files)
}

func (r *AnvilMergeComponentEmptyScopeRule) CheckParsedFiles(files []*scanner.File) []scanner.Finding {
	if len(files) == 0 {
		return nil
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

	findings := make([]scanner.Finding, 0, len(mergeComponents))
	for _, candidate := range mergeComponents {
		if _, ok := contributedScopes[candidate.scope]; ok {
			continue
		}

		name := extractIdentifierFlat(candidate.file, candidate.idx)
		if name == "" {
			name = "merged component"
		}

		findings = append(findings, r.Finding(
			candidate.file,
			candidate.file.FlatRow(candidate.idx)+1,
			1,
			fmt.Sprintf("@MergeComponent(%s::class) on '%s' has no matching @ContributesTo or @ContributesBinding scope in the project, so the merged component will be empty.", candidate.scope, name),
		))
	}

	return findings
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
	mods := file.FlatFindChild(idx, "modifiers")
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

func (r *AnvilContributesBindingWithoutScopeRule) NodeTypes() []string {
	return []string{"class_declaration", "object_declaration"}
}

func (r *AnvilContributesBindingWithoutScopeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	bindingScope := anvilAnnotationScopeFlat(file, idx, "ContributesBinding")
	if bindingScope == "" {
		return nil
	}

	interfaceScopes := anvilContributedInterfaceScopesFlat(file)
	if len(interfaceScopes) == 0 {
		return nil
	}

	for _, iface := range anvilImplementedTypesFlat(file, idx) {
		if iface == "" {
			continue
		}
		ifaceScope, ok := interfaceScopes[iface]
		if !ok || ifaceScope == "" || ifaceScope == bindingScope {
			continue
		}

		name := extractIdentifierFlat(file, idx)
		if name == "" {
			name = "binding"
		}

		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			1,
			fmt.Sprintf("@ContributesBinding(%s::class) on '%s' binds '%s', which is only contributed to %s::class.", bindingScope, name, iface, ifaceScope),
		)}
	}

	return nil
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

func (r *BindsMismatchedArityRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *BindsMismatchedArityRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !hasAnnotationFlat(file, idx, "Binds") {
		return nil
	}

	paramCount := 0
	if params := file.FlatFindChild(idx, "function_value_parameters"); params != 0 {
		walkFunctionParametersFlat(file, params, func(_ uint32) {
			paramCount++
		})
	}
	if paramCount == 1 {
		return nil
	}

	name := extractIdentifierFlat(file, idx)
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		1,
		fmt.Sprintf("@Binds function '%s' must declare exactly one parameter; found %d.", name, paramCount),
	)}
}

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

func (r *HiltEntryPointOnNonInterfaceRule) NodeTypes() []string {
	return []string{"class_declaration", "object_declaration", "prefix_expression"}
}

func (r *HiltEntryPointOnNonInterfaceRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	kind, name, line, ok := hiltEntryPointDeclarationFlat(file, idx)
	if !ok || kind == "interface" {
		return nil
	}
	if name == "" {
		name = "entry point"
	}

	return []scanner.Finding{r.Finding(
		file,
		line,
		1,
		fmt.Sprintf("@EntryPoint '%s' must be declared as an interface, not a %s.", name, kind),
	)}
}

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
		if file.FlatFindChild(idx, "interface") == 0 {
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
	if mods := file.FlatFindChild(idx, "modifiers"); mods != 0 {
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
