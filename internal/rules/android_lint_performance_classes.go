package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's ItemDecoratorDetector (DuplicateDivider) and SyntheticAccessorDetector
// (SyntheticAccessor).

func registerPerformanceClassChecks() {
	divider := &DuplicateDividerRule{AndroidRule: lintAndroidRule("DuplicateDivider", "Unnecessary Divider Copy",
		ALCPerformance, ALSWarning, 4)}
	registerPerformanceLintRule(divider.AndroidRule, divider, divider.Confidence(), divider.check,
		[]string{"class_declaration"}, []scanner.Language{scanner.LangKotlin, scanner.LangJava})
	synthetic := &SyntheticAccessorRule{AndroidRule: lintAndroidRule("SyntheticAccessor", "Synthetic Accessor",
		ALCPerformance, ALSWarning, 2)}
	registerPerformanceLintRule(synthetic.AndroidRule, synthetic, synthetic.Confidence(), synthetic.check,
		[]string{"source_file", "program"}, []scanner.Language{scanner.LangKotlin, scanner.LangJava})
}

// DuplicateDividerRule mirrors Lint's DuplicateDivider: a copy of the support-library sample DividerItemDecoration
// (a RecyclerView.ItemDecoration subclass named DividerItemDecoration with HORIZONTAL_LIST and VERTICAL_LIST
// fields), which android.support.v7.widget.DividerItemDecoration replaces.
type DuplicateDividerRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports high confidence: the name, supertype and fields are read from the declaration.
func (r *DuplicateDividerRule) Confidence() float64 { return api.ConfidenceHigh }

// supportItemDecoration is the only supertype Lint's detector registers; the androidx class is not matched.
const supportItemDecoration = "android.support.v7.widget.RecyclerView.ItemDecoration"

func (r *DuplicateDividerRule) check(ctx *api.Context) {
	file := ctx.File
	name := lintClassNameNode(file, ctx.Idx)
	if name == 0 || file.FlatNodeText(name) != "DividerItemDecoration" {
		return
	}
	if !classDeclaresField(file, ctx.Idx, "HORIZONTAL_LIST") || !classDeclaresField(file, ctx.Idx, "VERTICAL_LIST") {
		return
	}
	if _, nested := flatEnclosingAncestor(file, ctx.Idx, "class_body"); !nested && sourcePackageName(file) == "android.support.v7.widget" {
		return
	}
	// A class of the file named RecyclerView shadows the support-library import.
	if lintSameFileClass(file, "RecyclerView") != 0 || !lintClassExtends(ctx, ctx.Idx, supportItemDecoration, nil) {
		return
	}
	ctx.EmitAt(file.FlatRow(name)+1, file.FlatCol(name)+1, "Replace with `android.support.v7.widget.DividerItemDecoration`?")
}

// classDeclaresField reports whether a class declares a field called name: a Java field, or a Kotlin property of the
// class or of its companion object (which compiles to a static field of the class).
func classDeclaresField(file *scanner.File, cls uint32, name string) bool {
	found := false
	var scan func(container uint32)
	scan = func(container uint32) {
		for c := file.FlatFirstChild(container); c != 0 && !found; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "field_declaration":
				// Only the field's own declarators: an anonymous class in the initializer declares its own fields.
				for d := file.FlatFirstChild(c); d != 0; d = file.FlatNextSib(d) {
					if file.FlatType(d) != "variable_declarator" {
						continue
					}
					if id, ok := file.FlatFindChild(d, "identifier"); ok && file.FlatNodeText(id) == name {
						found = true
					}
				}
			case "property_declaration":
				if vd, ok := file.FlatFindChild(c, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
					found = true
				}
			case "companion_object":
				if inner, ok := file.FlatFindChild(c, "class_body"); ok {
					scan(inner)
				}
			}
		}
	}
	// A Kotlin class may have no body and declare its fields in the primary constructor.
	if body, ok := file.FlatFindChild(cls, "class_body"); ok {
		scan(body)
	}
	if !found && file.Language == scanner.LangKotlin {
		// Primary-constructor `val`/`var` parameters are fields too.
		if ctor, ok := file.FlatFindChild(cls, "primary_constructor"); ok {
			file.FlatWalkNodes(ctor, "class_parameter", func(p uint32) {
				if id, ok := file.FlatFindChild(p, "simple_identifier"); ok && file.FlatNodeText(id) == name &&
					kotlinClassParameterIsProperty(file, p) {
					found = true
				}
			})
		}
	}
	return found
}

func (r *DuplicateDividerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DuplicateDivider", RuleSet: "android-lint", DefaultActive: true}
}

// SyntheticAccessor is off by default in Lint: it matters for libraries and apps near the 64K method limit.
func (r *SyntheticAccessorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SyntheticAccessor", RuleSet: "android-lint", DefaultActive: false,
		OptInReason: api.OptInReasonProjectPolicy}
}

// kotlinClassParameterIsProperty reports a constructor parameter declared with val or var, which is a property.
func kotlinClassParameterIsProperty(file *scanner.File, p uint32) bool {
	if kind, ok := file.FlatFindChild(p, "binding_pattern_kind"); ok {
		k := strings.TrimSpace(file.FlatNodeText(kind))
		return k == "val" || k == "var"
	}
	return perfHasToken(file, p, "val") || perfHasToken(file, p, "var")
}
