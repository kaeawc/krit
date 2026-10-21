package rules

import (
	"fmt"

	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// NoNameShadowingRule flags inner declarations that shadow outer ones.
// NoNameShadowing behavior:
//   - Skips underscore "_" names
//   - Non-inner class bodies reset scope (outer names not inherited)
//   - Object/companion object declarations reset scope
//   - Class member function params do NOT shadow class constructor params
//     (accessible via this.name)
type NoNameShadowingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule is highly accurate per-shadow but extremely noisy on real
// codebases (~1,785 findings on pocket-android-app per roadmap/17).
// Many shadows are intentional (scoping functions, kotlinx.coroutines
// flow collectors, builder DSLs). Medium confidence keeps it in
// --min-confidence=medium pipelines but lets strict pipelines filter
// it out of their default gate.
func (r *NoNameShadowingRule) Confidence() float64 { return api.ConfidenceMedium }

type noNameShadowFindingKey struct {
	line int
	name string
}

// shadowScanCtx carries the per-scan state threaded through walkScopeFlat and
// reportIfShadowedFlat. `visible` and `blocked` vary per scope, so they stay
// as walkScopeFlat parameters; `file`, `findings`, and `seen` are constant for
// the whole scan.
type shadowScanCtx struct {
	file     *scanner.File
	findings *[]scanner.Finding
	seen     map[noNameShadowFindingKey]bool
}

func (r *NoNameShadowingRule) walkScopeFlat(node uint32, ctx *shadowScanCtx, visible map[string]int, blocked map[string]bool) {
	file := ctx.file
	if visible == nil {
		visible = make(map[string]int, 16)
	}
	var localNames map[string]bool
	addLocalName := func(name string) {
		if localNames == nil {
			localNames = make(map[string]bool, 8)
		}
		if localNames[name] {
			return
		}
		localNames[name] = true
		visible[name]++
	}
	defer func() {
		for name := range localNames {
			if visible[name] <= 1 {
				delete(visible, name)
				continue
			}
			visible[name]--
		}
	}()

	var collectNames func(n uint32)
	collectNames = func(n uint32) {
		skipNextLambdaAsClassBody := false
		for i := 0; i < file.FlatNamedChildCount(n); i++ {
			child := file.FlatNamedChild(n, i)
			childType := file.FlatType(child)

			if childType == "type_alias" {
				continue
			}
			if noNameShadowIsScopeBarrierFlat(file, child) {
				r.walkScopeFlat(child, ctx, nil, nil)
				continue
			}
			if skipNextLambdaAsClassBody && childType == "lambda_literal" {
				skipNextLambdaAsClassBody = false
				r.walkScopeClassBodyLambda(child, ctx)
				continue
			}
			if childType == "property_declaration" || childType == "variable_declaration" {
				setSkip := r.collectVarDeclName(ctx, child, childType, visible, localNames, blocked, addLocalName)
				if setSkip {
					skipNextLambdaAsClassBody = true
				}
				continue
			}
			if childType == "parameter" {
				name := extractIdentifierFlat(file, child)
				if name != "" && name != "_" {
					r.collectParameterName(ctx, n, child, name, visible, localNames, blocked, addLocalName)
				}
				continue
			}
			if childType == "class_parameter" {
				name := extractIdentifierFlat(file, child)
				if name != "" && name != "_" {
					addLocalName(name)
				}
				continue
			}
			if noNameShadowIsScopeNodeFlat(file, child) {
				switch childType {
				case "function_declaration", "secondary_constructor":
					r.walkScopeFlat(child, ctx, visible, blocked)
				case "class_body":
					if blocked == nil {
						blocked = make(map[string]bool, 8)
					}
					addedBlocked := noNameShadowPushBlockedNamesFlat(file, child, blocked)
					r.walkScopeFlat(child, ctx, visible, blocked)
					for _, name := range addedBlocked {
						delete(blocked, name)
					}
				default:
					r.walkScopeFlat(child, ctx, visible, blocked)
				}
			} else {
				if experiment.Enabled("no-name-shadowing-prune") && !noNameShadowMayContainDeclarationsFlat(file, child) {
					continue
				}
				collectNames(child)
			}
		}
	}
	collectNames(node)
}

func (r *NoNameShadowingRule) walkScopeClassBodyLambda(child uint32, ctx *shadowScanCtx) {
	file := ctx.file
	lambdaBlocked := make(map[string]bool, 16)
	var pushProps func(x uint32)
	pushProps = func(x uint32) {
		for j := 0; j < file.FlatNamedChildCount(x); j++ {
			c := file.FlatNamedChild(x, j)
			if file.FlatType(c) == "property_declaration" {
				if name := extractIdentifierFlat(file, c); name != "" {
					lambdaBlocked[name] = true
				}
			}
			if file.FlatType(c) == "statements" {
				pushProps(c)
			}
		}
	}
	pushProps(child)
	r.walkScopeFlat(child, ctx, nil, lambdaBlocked)
}

func (r *NoNameShadowingRule) collectDestructuredNames(ctx *shadowScanCtx, multi uint32, visible map[string]int, localNames map[string]bool, blocked map[string]bool, addLocalName func(string)) {
	file := ctx.file
	for j := 0; j < file.FlatChildCount(multi); j++ {
		compDecl := file.FlatChild(multi, j)
		if file.FlatType(compDecl) != "variable_declaration" {
			continue
		}
		compName := extractIdentifierFlat(file, compDecl)
		if compName != "" && compName != "_" {
			r.reportIfShadowedFlat(ctx, compName, compDecl, visible, localNames, blocked)
			addLocalName(compName)
		}
	}
}

func noNameShadowVarDeclSuppressed(file *scanner.File, child uint32, name string) bool {
	if noNameShadowIsLambdaParameterFlat(file, child) {
		return true
	}
	if noNameShadowIsWhenSubjectBindingFlat(file, child) {
		return true
	}
	if file.FlatHasAncestorOfType(child, "catch_block") {
		return true
	}
	if noNameShadowHasLocalSuppression(file, child) {
		return true
	}
	if isNullNarrowingSelfShadowFlat(file, child, name) {
		return true
	}
	if isScopeFunctionSelfConfigurationShadowFlat(file, child, name) {
		return true
	}
	if isSimpleSelfAliasShadowFlat(file, child, name) {
		return true
	}
	if noNameShadowIsConstructorBackedClassPropertyFlat(file, child, name) {
		return true
	}
	if noNameShadowIsSetterAssignPatternFlat(file, child, name) {
		return true
	}
	if noNameShadowIsMutableDerivedCopyFlat(file, child, name) {
		return true
	}
	return false
}

func (r *NoNameShadowingRule) collectParameterName(ctx *shadowScanCtx, n, child uint32, name string, visible map[string]int, localNames map[string]bool, blocked map[string]bool, addLocalName func(string)) {
	file := ctx.file
	if noNameShadowIsLambdaParameterFlat(file, child) {
		addLocalName(name)
		return
	}
	enclosingFn := n
	if file.FlatType(enclosingFn) != "function_declaration" {
		for p, ok := file.FlatParent(enclosingFn); ok; p, ok = file.FlatParent(p) {
			enclosingFn = p
			if file.FlatType(enclosingFn) == "function_declaration" {
				break
			}
		}
	}
	if file.FlatType(enclosingFn) == "function_declaration" &&
		file.FlatHasModifier(enclosingFn, "override") {
		addLocalName(name)
	} else if noNameShadowIsSetterParamFlat(file, enclosingFn, name) {
		addLocalName(name)
	} else {
		r.reportIfShadowedFlat(ctx, name, child, visible, localNames, blocked)
		addLocalName(name)
	}
}

func (r *NoNameShadowingRule) collectVarDeclName(ctx *shadowScanCtx, child uint32, childType string, visible map[string]int, localNames map[string]bool, blocked map[string]bool, addLocalName func(string)) (setSkipLambda bool) {
	file := ctx.file
	if parent, ok := file.FlatParent(child); ok && file.FlatType(parent) == "source_file" {
		return false
	}
	if childType == "property_declaration" {
		if ut, ok := file.FlatFindChild(child, "user_type"); ok {
			if file.FlatNodeTextEquals(ut, "constructor") {
				return true
			}
		}
	}
	multi, _ := file.FlatFindChild(child, "multi_variable_declaration")
	if multi != 0 {
		r.collectDestructuredNames(ctx, multi, visible, localNames, blocked, addLocalName)
		return false
	}
	name := extractIdentifierFlat(file, child)
	if name != "" && name != "_" {
		if noNameShadowVarDeclSuppressed(file, child, name) {
			addLocalName(name)
		} else {
			r.reportIfShadowedFlat(ctx, name, child, visible, localNames, blocked)
			addLocalName(name)
		}
	}
	return false
}

// reportIfShadowed checks if name shadows an outer declaration and appends a
// finding if so, with deduplication.
func (r *NoNameShadowingRule) reportIfShadowedFlat(ctx *shadowScanCtx, name string, child uint32, visible map[string]int, localNames map[string]bool, blocked map[string]bool) {
	file := ctx.file
	findings := ctx.findings
	seen := ctx.seen
	if blocked != nil && blocked[name] {
		return
	}
	// Skip destructuring components. `val (a, b) = pair` and lambda
	// destructuring `{ (view, day) -> ... }` bind names that refer to a
	// specific element of the source expression — the names are not
	// freely choosable without changing semantics.
	if isInsideDestructuringFlat(file, child) {
		return
	}
	if localNames == nil || !localNames[name] {
		if visible[name] == 0 {
			return
		}
		line := file.FlatRow(child) + 1
		// Use the declaration's actual column so multiple shadowing
		// declarations on the same line (e.g. two function parameters)
		// produce distinct finding keys.
		col := file.FlatCol(child) + 1
		key := noNameShadowFindingKey{line: line, name: name}
		if !seen[key] {
			seen[key] = true
			*findings = append(*findings, r.Finding(file, line, col,
				fmt.Sprintf("Name '%s' shadows an outer declaration", name)))
		}
	}
}

func isInsideDestructuringFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "multi_variable_declaration":
			return true
		case "function_declaration", "lambda_literal", "class_body", "function_body", "statements", "source_file":
			return false
		}
	}
	return false
}

func noNameShadowIsLambdaParameterFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "statements":
			return false
		case "lambda_literal":
			return true
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return false
		}
	}
	return false
}

func noNameShadowIsWhenSubjectBindingFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "when_subject":
			return true
		case "when_entry", "function_declaration", "class_declaration", "object_declaration", "source_file":
			return false
		}
	}
	return false
}

func noNameShadowPushBlockedNamesFlat(file *scanner.File, classBody uint32, blocked map[string]bool) []string {
	var added []string
	for i := 0; i < file.FlatNamedChildCount(classBody); i++ {
		child := file.FlatNamedChild(classBody, i)
		if file.FlatType(child) == "property_declaration" {
			name := extractIdentifierFlat(file, child)
			if name != "" && !blocked[name] {
				blocked[name] = true
				added = append(added, name)
			}
		}
	}

	classDecl, ok := file.FlatParent(classBody)
	if ok {
		for i := 0; i < file.FlatNamedChildCount(classDecl); i++ {
			child := file.FlatNamedChild(classDecl, i)
			switch file.FlatType(child) {
			case "primary_constructor", "class_parameter":
				noNameShadowCollectParamNamesFlat(file, child, func(name string) {
					if name == "" || blocked[name] {
						return
					}
					blocked[name] = true
					added = append(added, name)
				})
			}
		}
	}
	return added
}

func noNameShadowMayContainDeclarationsFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "source_file", "statements", "block", "function_body", "class_body", "lambda_literal",
		"for_statement", "while_statement", "do_while_statement", "if_expression",
		"when_expression", "when_entry", "try_expression", "catch_block", "finally_block",
		"property_declaration", "variable_declaration", "multi_variable_declaration",
		"parameter", "class_parameter", "function_declaration", "secondary_constructor",
		"class_declaration", "object_declaration", "companion_object":
		return true
	}
	return file.FlatNamedChildCount(idx) > 0 && file.FlatHasAncestorOfType(idx, "source_file")
}

func noNameShadowIsScopeBarrierFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "class_declaration":
		return !file.FlatHasModifier(idx, "inner")
	case "object_declaration", "companion_object":
		return true
	}
	return false
}

func noNameShadowIsScopeNodeFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "function_declaration", "secondary_constructor", "function_body", "lambda_literal",
		"for_statement", "while_statement", "do_while_statement", "if_expression", "when_expression",
		"try_expression", "catch_block", "finally_block", "class_body", "control_structure_body",
		"anonymous_initializer":
		return true
	}
	return false
}

func noNameShadowCollectParamNamesFlat(file *scanner.File, idx uint32, visit func(string)) {
	switch file.FlatType(idx) {
	case "class_parameter", "parameter":
		if name := extractIdentifierFlat(file, idx); name != "" {
			visit(name)
		}
		return
	}
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		noNameShadowCollectParamNamesFlat(file, file.FlatNamedChild(idx, i), visit)
	}
}
