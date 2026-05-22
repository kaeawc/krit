package rules

import (
	"fmt"
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// AbstractMemberNotImplementedRule — flags concrete classes that fail to
// implement at least one abstract member declared on a supertype the
// resolver can see. Mimics kotlinc's ABSTRACT_MEMBER_NOT_IMPLEMENTED for
// the source-resolvable case: same-workspace interfaces and abstract
// classes whose abstract members the subclass omitted.
//
// Scope:
//   - Interfaces, sealed interfaces, abstract classes, and expect/external
//     declarations are skipped (they don't have to implement anything).
//   - For interface supertypes, all function members are treated as
//     abstract for v1. Default-method interfaces (rare in Kotlin) will
//     false-positive against this rule until member-body presence is
//     tracked on MemberInfo.
//   - For non-interface supertypes, only members with the explicit
//     `abstract` modifier are required.
//   - Walks one level of supertypes. Transitive abstract requirements
//     (X : AbstractA : InterfaceB) are deferred.
//   - Implementations include same-named members in the class body and
//     `override val/var` properties declared in the primary constructor.
//
// ---------------------------------------------------------------------------
type AbstractMemberNotImplementedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AbstractMemberNotImplementedRule) Confidence() float64 { return api.ConfidenceMediumHigh }

func (r *AbstractMemberNotImplementedRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) != "class_declaration" {
		return
	}
	if file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "expect") || file.FlatHasModifier(idx, "external") {
		return
	}
	if abstractRuleIsInterface(file, idx) {
		return
	}
	if ctx.Resolver == nil {
		return
	}
	className := abstractRuleClassName(file, idx)
	if className == "" {
		return
	}
	classInfo := ctx.Resolver.ClassHierarchy(className)
	if classInfo == nil || len(classInfo.Supertypes) == 0 {
		return
	}
	implementations := abstractRuleImplementations(file, idx, classInfo)
	required, transitivelySatisfied := abstractRuleRequiredMembers(ctx.Resolver, classInfo.Supertypes)
	for name := range transitivelySatisfied {
		implementations[name] = true
	}
	var missing []string
	for name := range required {
		if implementations[name] {
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Class '%s' does not implement abstract supertype member(s): %s.",
			className, strings.Join(missing, ", ")))
}

func abstractRuleIsInterface(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "interface" {
			return true
		}
	}
	// Tree-sitter sometimes wraps `interface` in modifiers; conservative
	// fallback uses the prefix text.
	return strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(idx)), "interface")
}

func abstractRuleClassName(file *scanner.File, idx uint32) string {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "type_identifier" {
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

// abstractRuleImplementations returns the set of member names implemented
// by the class. Includes class_body members regardless of `override`
// modifier (any same-named member shadows the supertype contract enough
// to silence this rule), plus primary-constructor `override val/var`
// properties.
func abstractRuleImplementations(file *scanner.File, classIdx uint32, classInfo *typeinfer.ClassInfo) map[string]bool {
	out := map[string]bool{}
	for _, m := range classInfo.Members {
		out[m.Name] = true
	}
	// Walk primary_constructor for `override val/var` parameters.
	// Tree-sitter Kotlin emits class_parameter nodes as direct children
	// of primary_constructor (no class_parameters wrapper).
	primary, ok := file.FlatFindChild(classIdx, "primary_constructor")
	if !ok {
		return out
	}
	for child := file.FlatFirstChild(primary); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "class_parameter" {
			continue
		}
		if !classParameterHasOverrideModifier(file, child) {
			continue
		}
		if name := classParameterName(file, child); name != "" {
			out[name] = true
		}
	}
	return out
}

func classParameterHasOverrideModifier(file *scanner.File, paramIdx uint32) bool {
	for child := file.FlatFirstChild(paramIdx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "modifiers":
			for sub := file.FlatFirstChild(child); sub != 0; sub = file.FlatNextSib(sub) {
				if file.FlatNodeText(sub) == "override" {
					return true
				}
				for gc := file.FlatFirstChild(sub); gc != 0; gc = file.FlatNextSib(gc) {
					if file.FlatNodeText(gc) == "override" {
						return true
					}
				}
			}
		case "override":
			return true
		}
	}
	return false
}

func classParameterName(file *scanner.File, paramIdx uint32) string {
	for child := file.FlatFirstChild(paramIdx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

// abstractRuleRequiredMembers walks the transitive supertype closure
// and returns:
//   - required: member names that some interface or abstract class
//     declares abstractly somewhere up the chain.
//   - satisfied: member names that an intermediate concrete class
//     (or a non-abstract abstract-class member) provides, i.e. names
//     the subclass does not need to re-implement.
//
// The dual return lets the rule subtract satisfied from required so
// that `class FullChild : GreeterBase()` where GreeterBase already
// implements Greeter.greet does not emit a finding.
func abstractRuleRequiredMembers(resolver typeinfer.TypeResolver, supertypes []string) (required map[string]bool, satisfied map[string]bool) {
	required = map[string]bool{}
	satisfied = map[string]bool{}
	seen := map[string]bool{}
	queue := append([]string(nil), supertypes...)
	for len(queue) > 0 {
		st := queue[0]
		queue = queue[1:]
		if seen[st] {
			continue
		}
		seen[st] = true
		super := resolver.ClassHierarchy(st)
		if super == nil {
			continue
		}
		queue = append(queue, super.Supertypes...)
		isInterface := strings.Contains(super.Kind, "interface")
		isAbstractClass := super.IsAbstract && !isInterface
		for _, m := range super.Members {
			switch {
			case isInterface:
				// v1 simplification: every interface member is treated
				// as abstract. Default-method interfaces would
				// false-positive until member-body presence is tracked.
				required[m.Name] = true
			case isAbstractClass && m.IsAbstract:
				required[m.Name] = true
			default:
				// Concrete class member, or non-abstract member of an
				// abstract class. Satisfies the contract for any
				// further-down subclass.
				satisfied[m.Name] = true
			}
		}
	}
	return required, satisfied
}
