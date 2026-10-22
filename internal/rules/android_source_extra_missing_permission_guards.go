package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

func missingPermissionHasStructuralGuard(ctx *api.Context, call uint32, req missingPermissionRequirement) bool {
	// SecurityException catches do not name a specific permission, so they
	// cover the requirement regardless of arity.
	if missingPermissionEnclosingCatchesSecurityException(ctx, call) {
		return true
	}
	return req.satisfied(func(perm string) bool {
		return missingPermissionEnclosingPermissionCondition(ctx, call, perm) ||
			missingPermissionHasPrecedingGrantedReturnGuard(ctx, call, perm) ||
			missingPermissionEnclosingAnnotationRequires(ctx, call, perm)
	})
}

type missingPermissionGuardState int

const (
	missingPermissionGuardUnknown missingPermissionGuardState = iota
	missingPermissionGuardGranted
	missingPermissionGuardDenied
)

func missingPermissionEnclosingPermissionCondition(ctx *api.Context, call uint32, perm string) bool {
	file := ctx.File
	for body, ok := file.FlatParent(call); ok; body, ok = file.FlatParent(body) {
		if file.FlatType(body) != "control_structure_body" {
			continue
		}
		parent, ok := file.FlatParent(body)
		if !ok || file.FlatType(parent) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := missingPermissionIfParts(file, parent)
		if cond == 0 || body != thenBody && body != elseBody {
			continue
		}
		state := missingPermissionConditionPermissionState(ctx, cond, perm)
		if state == missingPermissionGuardGranted && body == thenBody {
			return true
		}
		if state == missingPermissionGuardDenied && body == elseBody {
			return true
		}
	}
	return false
}

func missingPermissionConditionPermissionState(ctx *api.Context, cond uint32, perm string) missingPermissionGuardState {
	if ctx.File == nil || cond == 0 {
		return missingPermissionGuardUnknown
	}
	file := ctx.File
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "equality_expression":
		return missingPermissionEqualityPermissionState(ctx, cond, perm)
	case "conjunction_expression":
		for child := file.FlatFirstChild(cond); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if missingPermissionConditionPermissionState(ctx, child, perm) == missingPermissionGuardGranted {
				return missingPermissionGuardGranted
			}
		}
	}
	return missingPermissionGuardUnknown
}

func missingPermissionEqualityPermissionState(ctx *api.Context, expr uint32, perm string) missingPermissionGuardState {
	file := ctx.File
	if file.FlatType(expr) != "equality_expression" || file.FlatChildCount(expr) < 3 {
		return missingPermissionGuardUnknown
	}
	left := flatUnwrapParenExpr(file, file.FlatChild(expr, 0))
	op := strings.TrimSpace(file.FlatNodeText(file.FlatChild(expr, 1)))
	right := flatUnwrapParenExpr(file, file.FlatChild(expr, file.FlatChildCount(expr)-1))
	if left == 0 || op == "" || right == 0 {
		return missingPermissionGuardUnknown
	}

	if missingPermissionCheckSelfPermissionCall(ctx, left, perm) {
		return missingPermissionComparePermissionResult(file, op, right)
	}
	if missingPermissionCheckSelfPermissionCall(ctx, right, perm) {
		return missingPermissionComparePermissionResult(file, op, left)
	}
	return missingPermissionGuardUnknown
}

func missingPermissionComparePermissionResult(file *scanner.File, op string, result uint32) missingPermissionGuardState {
	granted, ok := missingPermissionPermissionResultConstant(file, result)
	if !ok {
		return missingPermissionGuardUnknown
	}
	switch op {
	case "==":
		if granted {
			return missingPermissionGuardGranted
		}
		return missingPermissionGuardDenied
	case "!=":
		if granted {
			return missingPermissionGuardDenied
		}
		return missingPermissionGuardGranted
	default:
		return missingPermissionGuardUnknown
	}
}

func missingPermissionPermissionResultConstant(file *scanner.File, expr uint32) (granted bool, ok bool) {
	expr = flatUnwrapParenExpr(file, expr)
	path := missingPermissionIdentifierPath(file, expr)
	switch missingPermissionSimpleName(path) {
	case "PERMISSION_GRANTED", "GRANTED":
		return true, true
	case "PERMISSION_DENIED", "DENIED":
		return false, true
	default:
		return false, false
	}
}

func missingPermissionCheckSelfPermissionCall(ctx *api.Context, call uint32, perm string) bool {
	if ctx.File == nil || ctx.File.FlatType(call) != "call_expression" {
		return false
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || target.CalleeName != "checkSelfPermission" {
		return false
	}
	for _, arg := range target.Arguments {
		if missingPermissionExprMatchesPermission(ctx, arg.Node, perm) {
			return true
		}
	}
	return false
}

func missingPermissionHasPrecedingGrantedReturnGuard(ctx *api.Context, call uint32, perm string) bool {
	if ctx.File == nil || call == 0 {
		return false
	}
	file := ctx.File
	scope := enclosingAncestor(file, call, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal")
	if scope == 0 {
		return false
	}
	targetStart := file.FlatStartByte(call)
	guarded := false
	file.FlatWalkNodes(scope, "if_expression", func(ifExpr uint32) {
		if guarded || file.FlatStartByte(ifExpr) >= targetStart {
			return
		}
		ifStmt, ifContainer := missingPermissionContainingStatement(file, ifExpr)
		callStmt, callContainer := missingPermissionContainingStatement(file, call)
		if ifContainer == 0 || ifContainer != callContainer || file.FlatEndByte(ifStmt) > file.FlatStartByte(callStmt) {
			return
		}
		cond, thenBody, _ := missingPermissionIfParts(file, ifExpr)
		if cond == 0 || thenBody == 0 {
			return
		}
		if missingPermissionConditionPermissionState(ctx, cond, perm) != missingPermissionGuardDenied {
			return
		}
		if missingPermissionBodyTerminates(file, thenBody) {
			guarded = true
		}
	})
	return guarded
}

func missingPermissionContainingStatement(file *scanner.File, idx uint32) (stmt uint32, container uint32) {
	if file == nil || idx == 0 {
		return 0, 0
	}
	for cur, ok := idx, true; ok; cur, ok = file.FlatParent(cur) {
		parent, hasParent := file.FlatParent(cur)
		if !hasParent {
			return cur, 0
		}
		switch file.FlatType(parent) {
		case "statements", "function_body", "control_structure_body", "lambda_literal", "anonymous_initializer":
			return cur, parent
		}
	}
	return 0, 0
}

func missingPermissionBodyTerminates(file *scanner.File, body uint32) bool {
	if file == nil || body == 0 {
		return false
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if missingPermissionNodeTerminates(file, child) {
			return true
		}
	}
	return missingPermissionNodeTerminates(file, body)
}

func missingPermissionNodeTerminates(file *scanner.File, node uint32) bool {
	switch file.FlatType(node) {
	case "jump_expression":
		text := strings.TrimSpace(file.FlatNodeText(node))
		return strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw")
	case "call_expression":
		target, ok := semantics.ResolveCallTarget(&api.Context{File: file}, node)
		return ok && (target.CalleeName == "error" || target.CalleeName == "TODO")
	default:
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return missingPermissionNodeTerminates(file, child)
			}
		}
		return false
	}
}

func missingPermissionEnclosingAnnotationRequires(ctx *api.Context, call uint32, perm string) bool {
	for owner, ok := ctx.File.FlatParent(call); ok; owner, ok = ctx.File.FlatParent(owner) {
		switch ctx.File.FlatType(owner) {
		case "function_declaration", "class_declaration", "object_declaration":
			if required, ok := missingPermissionRequiredPermissionAnnotation(ctx, owner); ok && required.covers(perm) {
				return true
			}
		}
	}
	return false
}

func missingPermissionRequiredPermissionAnnotation(ctx *api.Context, node uint32) (missingPermissionRequirement, bool) {
	file := ctx.File
	if mods, ok := file.FlatFindChild(node, "modifiers"); ok {
		if req, ok := missingPermissionAnnotationContainerRequirement(ctx, mods); ok {
			return req, true
		}
	}
	for prev, ok := file.FlatPrevSibling(node); ok; prev, ok = file.FlatPrevSibling(prev) {
		switch file.FlatType(prev) {
		case "prefix_expression", "annotation", "modifiers":
			if req, ok := missingPermissionAnnotationContainerRequirement(ctx, prev); ok {
				return req, true
			}
		default:
			if strings.TrimSpace(file.FlatNodeText(prev)) != "" {
				return missingPermissionRequirement{}, false
			}
		}
	}
	return missingPermissionRequirement{}, false
}

func missingPermissionAnnotationContainerRequirement(ctx *api.Context, container uint32) (missingPermissionRequirement, bool) {
	file := ctx.File
	if !missingPermissionAnnotationContainerHasRequiresPermission(file, container) {
		return missingPermissionRequirement{}, false
	}
	// Inspect named arguments first so anyOf/allOf semantics win over the
	// fall-back scan that would otherwise grab the first permission literal.
	var named missingPermissionRequirement
	file.FlatWalkNodes(container, "value_argument", func(arg uint32) {
		if !named.empty() {
			return
		}
		name := missingPermissionValueArgumentName(file, arg)
		if name != "anyOf" && name != "allOf" {
			return
		}
		perms := missingPermissionCollectPerms(ctx, arg)
		if len(perms) == 0 {
			return
		}
		kind := missingPermissionRequirementAnyOf
		if name == "allOf" {
			kind = missingPermissionRequirementAllOf
		}
		named = missingPermissionRequirement{kind: kind, perms: perms}
	})
	if !named.empty() {
		return named, true
	}
	var perm string
	file.FlatWalkAllNodes(container, func(candidate uint32) {
		if perm != "" {
			return
		}
		if p, ok := missingPermissionExprPermissionName(ctx, candidate); ok {
			perm = p
		}
	})
	if perm == "" {
		return missingPermissionRequirement{}, false
	}
	return singlePermRequirement(perm), true
}

// missingPermissionValueArgumentName returns the name of a Kotlin named value
// argument (e.g. "anyOf" in `anyOf = [...]`), or "" for positional arguments.
// Mirrors valueArgumentExpression's label handling but extracts the label
// rather than skipping it.
func missingPermissionValueArgumentName(file *scanner.File, arg uint32) string {
	if file == nil || arg == 0 || file.FlatType(arg) != "value_argument" {
		return ""
	}
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "value_argument_label":
			var name string
			file.FlatWalkAllNodes(child, func(node uint32) {
				if name == "" && file.FlatType(node) == "simple_identifier" {
					name = file.FlatNodeString(node, nil)
				}
			})
			return name
		case "simple_identifier":
			if next, ok := file.FlatNextSibling(child); ok && file.FlatType(next) == "=" {
				return file.FlatNodeString(child, nil)
			}
			return ""
		default:
			return ""
		}
	}
	return ""
}

// missingPermissionCollectPerms walks a value-argument subtree (typically a
// collection literal in anyOf/allOf) and returns each recognized permission
// constant in source order, de-duplicated.
func missingPermissionCollectPerms(ctx *api.Context, node uint32) []string {
	file := ctx.File
	if file == nil || node == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	file.FlatWalkAllNodes(node, func(candidate uint32) {
		p, ok := missingPermissionExprPermissionName(ctx, candidate)
		if !ok || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	})
	return out
}

func missingPermissionFileHasRequiresPermissionAnnotation(file *scanner.File) bool {
	if file == nil {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "annotation", func(annotation uint32) {
		if !found && missingPermissionAnnotationContainerHasRequiresPermission(file, annotation) {
			found = true
		}
	})
	return found
}

func missingPermissionAnnotationContainerHasRequiresPermission(file *scanner.File, container uint32) bool {
	if file == nil || container == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(container, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "user_type", "type_identifier", "simple_identifier", "navigation_expression":
			path := missingPermissionIdentifierPath(file, candidate)
			if path == "" {
				path = file.FlatNodeText(candidate)
			}
			if missingPermissionSimpleName(path) == "RequiresPermission" {
				found = true
			}
		}
	})
	return found
}

func missingPermissionEnclosingCatchesSecurityException(ctx *api.Context, call uint32) bool {
	for _, handler := range semantics.EnclosingCaughtExceptionHandlers(ctx, call) {
		if handler.TypeName == "SecurityException" || handler.TypeName == "java.lang.SecurityException" {
			return true
		}
	}
	return false
}

func missingPermissionExprMatchesPermission(ctx *api.Context, expr uint32, perm string) bool {
	if p, ok := missingPermissionExprPermissionName(ctx, expr); ok {
		return p == perm
	}
	if ctx.File == nil || expr == 0 {
		return false
	}
	found := false
	ctx.File.FlatWalkAllNodes(expr, func(node uint32) {
		if found {
			return
		}
		if p, ok := missingPermissionExprPermissionName(ctx, node); ok && p == perm {
			found = true
		}
	})
	return found
}

func missingPermissionExprPermissionName(ctx *api.Context, expr uint32) (string, bool) {
	if ctx.File == nil || expr == 0 {
		return "", false
	}
	if val, ok := semantics.EvalConst(ctx, expr); ok && val.Kind == "string" {
		return missingPermissionStringPermissionName(val.String)
	}
	file := ctx.File
	switch file.FlatType(expr) {
	case "simple_identifier", "navigation_expression", "user_type", "type_identifier":
		path := missingPermissionIdentifierPath(file, expr)
		if path == "" {
			return "", false
		}
		last := missingPermissionSimpleName(path)
		if !missingPermissionKnownPerms[last] {
			return "", false
		}
		if strings.Contains(path, ".permission.") || strings.Contains(path, "Manifest.permission.") || path == last {
			return last, true
		}
	}
	return "", false
}

func missingPermissionStringPermissionName(value string) (string, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "android.permission.")
	if missingPermissionKnownPerms[value] {
		return value, true
	}
	return "", false
}

func missingPermissionIdentifierPath(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	var parts []string
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier":
			parts = append(parts, file.FlatNodeString(candidate, nil))
		}
	})
	return strings.Join(parts, ".")
}

func missingPermissionSimpleName(name string) string {
	name = strings.TrimSpace(strings.Trim(name, "`"))
	if idx := strings.LastIndex(name, "."); idx >= 0 && idx+1 < len(name) {
		return name[idx+1:]
	}
	return name
}

func missingPermissionHasImport(file *scanner.File, fqn string) bool {
	found := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		if found {
			return
		}
		path := missingPermissionIdentifierPath(file, node)
		found = path == fqn
	})
	return found
}

func missingPermissionIfParts(file *scanner.File, idx uint32) (cond uint32, thenBody uint32, elseBody uint32) {
	foundElse := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "control_structure_body":
			if !foundElse && thenBody == 0 {
				thenBody = child
			} else if foundElse && elseBody == 0 {
				elseBody = child
			}
		case "else":
			foundElse = true
		default:
			if cond == 0 && file.FlatIsNamed(child) {
				cond = child
			}
		}
	}
	return cond, thenBody, elseBody
}
