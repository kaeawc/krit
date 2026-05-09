package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// OverrideSignatureMismatchRule — flags `override fun X(...)` declarations
// whose parameter count does not match any same-named function on a supertype
// the resolver can see. Mimics kotlinc's NOTHING_TO_OVERRIDE diagnostic for
// the case the resolver can prove from source: same-workspace supertypes
// with a same-named member but a different parameter count.
//
// The rule is intentionally narrow:
//   - Only fires when at least one supertype known to the resolver
//     contains a function with the same name. If no supertype has the
//     name, the rule is silent (could be a library override, deferred
//     to classpath-aware resolution).
//   - Compares parameter COUNTS only. Type comparison is deferred until
//     parameter types are reliably resolved across files (today,
//     library-typed parameters are TypeUnknown and would false-positive).
//   - Generic functions and library supertypes are out of scope.
//
// ---------------------------------------------------------------------------
type OverrideSignatureMismatchRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *OverrideSignatureMismatchRule) Confidence() float64 { return 0.85 }

func (r *OverrideSignatureMismatchRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) != "function_declaration" {
		return
	}
	if !file.FlatHasModifier(idx, "override") {
		return
	}
	if ctx.Resolver == nil {
		return
	}
	classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration")
	if !ok {
		return
	}
	className := overrideEnclosingClassName(file, classDecl)
	if className == "" {
		return
	}
	classInfo := ctx.Resolver.ClassHierarchy(className)
	if classInfo == nil || len(classInfo.Supertypes) == 0 {
		return
	}
	funcName := overrideEnclosingClassName(file, idx)
	if funcName == "" {
		return
	}
	thisMember := overrideFindMember(classInfo.Members, funcName)
	if thisMember == nil || thisMember.Kind != "function" {
		return
	}
	// Walk supertypes transitively. If any reachable supertype has a
	// same-name function with a matching parameter count, the override is
	// fine. Cycles are guarded against via a seen set.
	var nearestMismatch *typeinfer.MemberInfo
	matched := false
	seen := map[string]bool{}
	queue := append([]string(nil), classInfo.Supertypes...)
	for len(queue) > 0 && !matched {
		st := queue[0]
		queue = queue[1:]
		if seen[st] {
			continue
		}
		seen[st] = true
		super := ctx.Resolver.ClassHierarchy(st)
		if super == nil {
			continue
		}
		queue = append(queue, super.Supertypes...)
		for i := range super.Members {
			sm := &super.Members[i]
			if sm.Kind != "function" || sm.Name != funcName {
				continue
			}
			if len(sm.Params) == len(thisMember.Params) {
				matched = true
				break
			}
			if nearestMismatch == nil {
				nearestMismatch = sm
			}
		}
	}
	if matched {
		return
	}
	if nearestMismatch == nil {
		// No supertype the resolver can see has this name; silent (could
		// resolve through classpath we don't see yet).
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Function '%s' is marked override but no supertype declares a matching parameter count: this declaration has %d, supertype has %d.",
			funcName, len(thisMember.Params), len(nearestMismatch.Params)))
}

func overrideEnclosingClassName(file *scanner.File, decl uint32) string {
	if file == nil || decl == 0 {
		return ""
	}
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "type_identifier" || file.FlatType(child) == "simple_identifier" {
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func overrideFindMember(members []typeinfer.MemberInfo, name string) *typeinfer.MemberInfo {
	for i := range members {
		if members[i].Name == name {
			return &members[i]
		}
	}
	return nil
}
