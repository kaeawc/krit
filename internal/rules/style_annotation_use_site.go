package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnnecessaryAnnotationUseSiteTargetRule detects `@param:` on primary constructor parameters and
// `@property:` on properties that are not constructor parameters, where the target is already the default.
type UnnecessaryAnnotationUseSiteTargetRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. The default target depends on the annotation's
// declared targets, which the AST cannot see, so the rule mirrors detekt's purely syntactic check.
func (r *UnnecessaryAnnotationUseSiteTargetRule) Confidence() float64 { return api.ConfidenceMedium }

const (
	useSiteParamMessage    = "An annotation over a parameter doesn't need the use-site target @param."
	useSitePropertyMessage = "An annotation over a property, that it's not a parameter, doesn't need the use-site target @property."
)

// unnecessaryUseSiteTarget returns the target node and message for an annotation whose use-site target is redundant.
func unnecessaryUseSiteTarget(file *scanner.File, annotation uint32) (target uint32, message string, ok bool) {
	target, found := file.FlatFindChild(annotation, "use_site_target")
	if !found {
		return 0, "", false
	}
	modifiers, found := file.FlatParent(annotation)
	if !found || file.FlatType(modifiers) != "modifiers" {
		return 0, "", false
	}
	decl, found := file.FlatParent(modifiers)
	if !found {
		return 0, "", false
	}
	switch {
	case file.FlatHasChildOfType(target, "param") && isPrimaryConstructorParameter(file, decl):
		return target, useSiteParamMessage, true
	case file.FlatHasChildOfType(target, "property") && isSingleVariableProperty(file, decl):
		return target, useSitePropertyMessage, true
	}
	return 0, "", false
}

func isPrimaryConstructorParameter(file *scanner.File, decl uint32) bool {
	if file.FlatType(decl) != "class_parameter" {
		return false
	}
	parent, found := file.FlatParent(decl)
	return found && file.FlatType(parent) == "primary_constructor"
}

// isSingleVariableProperty excludes destructuring declarations, which detekt does not check.
func isSingleVariableProperty(file *scanner.File, decl uint32) bool {
	return file.FlatType(decl) == "property_declaration" && !file.FlatHasChildOfType(decl, "multi_variable_declaration")
}

func registerStyleUnnecessaryAnnotationUseSiteTarget() {
	r := &UnnecessaryAnnotationUseSiteTargetRule{BaseRule: BaseRule{RuleName: "UnnecessaryAnnotationUseSiteTarget", RuleSetName: "style", Sev: "warning", Desc: "Detects annotation use-site targets that are the default for the annotated declaration."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"annotation"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			file := ctx.File
			target, message, ok := unnecessaryUseSiteTarget(file, ctx.Idx)
			if !ok {
				return
			}
			// detekt reports once per annotation in a `@target:[A B]` group.
			for range max(1, groupedAnnotationCount(file, ctx.Idx)) {
				ctx.EmitAt(file.FlatRow(target)+1, file.FlatCol(target)+1, message)
			}
		},
	})
}

func groupedAnnotationCount(file *scanner.File, annotation uint32) int {
	count := 0
	for c := file.FlatFirstChild(annotation); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "user_type" || t == "constructor_invocation" {
			count++
		}
	}
	return count
}
