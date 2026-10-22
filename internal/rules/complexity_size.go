package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// LongMethodRule detects functions exceeding a line count.
type LongMethodRule struct {
	FlatDispatchBase
	BaseRule
	AllowedLines int
}

// Description implements DescriptionProvider.
func (*LongMethodRule) Description() string {
	return "Flags functions that exceed the configured line limit. Long functions are harder to test and understand — consider extracting logical sub-steps into named helpers."
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *LongMethodRule) Confidence() float64 { return api.ConfidenceMedium }

// longMethodDeclarationLineFlat returns the 1-based line number of the `fun`
// keyword within a function_declaration node. It walks the node's direct
// children looking for the anonymous "fun" token, which is structurally
// reliable for annotated functions (e.g. @Override\nsuspend fun) where the
// node's start row is the annotation row, not the fun keyword row.
func longMethodDeclarationLineFlat(file *scanner.File, idx uint32) int {
	if file == nil {
		return 1
	}
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatNodeTextEquals(c, "fun") {
			return file.FlatRow(c) + 1
		}
	}
	return file.FlatRow(idx) + 1
}

// LargeClassRule detects classes exceeding a line count.
type LargeClassRule struct {
	FlatDispatchBase
	BaseRule
	AllowedLines int
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *LargeClassRule) Confidence() float64 { return api.ConfidenceMedium }

// ComplexInterfaceRule detects interfaces with too many members.
type ComplexInterfaceRule struct {
	FlatDispatchBase
	BaseRule
	AllowedDefinitions         int
	IncludePrivateDeclarations bool
	IncludeStaticDeclarations  bool
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *ComplexInterfaceRule) Confidence() float64 { return api.ConfidenceMedium }

// LongParameterListRule detects functions/constructors with too many parameters.
type LongParameterListRule struct {
	FlatDispatchBase
	BaseRule
	AllowedFunctionParameters    int
	AllowedConstructorParameters int
	IgnoreDefaultParameters      bool     // if true, parameters with defaults are not counted
	IgnoreDataClasses            bool     // if true, data class constructors are skipped
	IgnoreAnnotatedParameter     []string // annotations that exclude a parameter from counting
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *LongParameterListRule) Confidence() float64 { return api.ConfidenceMedium }

// paramHasIgnoredAnnotationFlat reports whether the parameter at idx
// carries any annotation whose simple name matches one of the names in
// `annotations`. Used to honor LongParameterListRule.IgnoreAnnotatedParameter
// so that, e.g., `@Composable`-annotated parameters can be excluded from
// the parameter count.
//
// Tree-sitter Kotlin attaches annotations on function parameters to a
// `parameter_modifiers` sibling that precedes the `parameter` node, while
// class-constructor parameters carry their annotations as children of the
// `class_parameter` node. The helper consults both shapes so callers can
// pass either node kind opaquely.
func paramHasIgnoredAnnotationFlat(file *scanner.File, idx uint32, annotations []string) bool {
	if len(annotations) == 0 || file == nil || idx == 0 {
		return false
	}
	// 1) Annotations attached as children (class_parameter case, and any
	//    parameter shape that nests a `modifiers` child).
	for _, name := range annotations {
		if name == "" {
			continue
		}
		if flatHasAnnotationNamed(file, idx, name) {
			return true
		}
	}
	// 2) Function-parameter case: annotations live on the preceding
	//    `parameter_modifiers` sibling within `function_value_parameters`.
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	var prevModifiers uint32
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if child == idx {
			break
		}
		if file.FlatType(child) == "parameter_modifiers" {
			prevModifiers = child
		} else if file.FlatType(child) == "parameter" || file.FlatType(child) == "class_parameter" {
			// Only the parameter_modifiers immediately preceding `idx`
			// belongs to it; reset on every other parameter node.
			prevModifiers = 0
		}
	}
	if prevModifiers == 0 {
		return false
	}
	for _, name := range annotations {
		if name == "" {
			continue
		}
		if flatAnnotationListContains("", file.FlatNodeText(prevModifiers), name) {
			return true
		}
	}
	return false
}

// MethodOverloadingRule detects too many overloads of the same method name.
type MethodOverloadingRule struct {
	FlatDispatchBase
	BaseRule
	AllowedOverloads int
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *MethodOverloadingRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MethodOverloadingRule) checkScopeFlat(ctx *api.Context, node uint32) {
	file := ctx.File
	counts := make(map[string]int)
	firstLine := make(map[string]int)
	addFunction := func(fn uint32) {
		name := extractIdentifierFlat(file, fn)
		if name == "" {
			return
		}
		counts[name]++
		if _, ok := firstLine[name]; !ok {
			firstLine[name] = file.FlatRow(fn) + 1
		}
	}

	switch file.FlatType(node) {
	case "source_file":
		for i := 0; i < file.FlatChildCount(node); i++ {
			child := file.FlatChild(node, i)
			switch file.FlatType(child) {
			case "function_declaration":
				addFunction(child)
			case "class_declaration":
				r.checkScopeFlat(ctx, child)
			}
		}
	case "class_declaration":
		body, _ := file.FlatFindChild(node, "class_body")
		if body == 0 {
			return
		}
		forEachDirectClassBodyNodeFlat(file, body, func(child uint32) {
			switch file.FlatType(child) {
			case "function_declaration":
				addFunction(child)
			case "class_declaration":
				r.checkScopeFlat(ctx, child)
			}
		})
	default:
		return
	}
	for name, count := range counts {
		if count > r.AllowedOverloads {
			ctx.Emit(r.Finding(file, firstLine[name], 1,
				fmt.Sprintf("Method '%s' has %d overloads (allowed: %d).", name, count, r.AllowedOverloads)))
		}
	}
}

// TooManyFunctionsRule detects files or classes with too many functions.
type TooManyFunctionsRule struct {
	FlatDispatchBase
	BaseRule
	AllowedFunctionsPerFile      int
	AllowedFunctionsPerClass     int
	AllowedFunctionsPerInterface int
	AllowedFunctionsPerObject    int
	AllowedFunctionsPerEnum      int
	IgnorePrivate                bool
	IgnoreDeprecated             bool
	IgnoreInternal               bool
	IgnoreOverridden             bool
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *TooManyFunctionsRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *TooManyFunctionsRule) shouldCountFunctionFlat(fnNode uint32, file *scanner.File) bool {
	if r.IgnorePrivate && file.FlatHasModifier(fnNode, "private") {
		return false
	}
	if r.IgnoreInternal && file.FlatHasModifier(fnNode, "internal") {
		return false
	}
	if r.IgnoreOverridden && file.FlatHasModifier(fnNode, "override") {
		return false
	}
	if r.IgnoreDeprecated {
		if mods, ok := file.FlatFindChild(fnNode, "modifiers"); ok && strings.Contains(file.FlatNodeText(mods), "Deprecated") {
			return false
		}
	}
	return true
}

func (r *TooManyFunctionsRule) countFunctionsInClassFlat(cls uint32, file *scanner.File) int {
	body, _ := file.FlatFindChild(cls, "class_body")
	if body == 0 {
		return 0
	}
	count := 0
	forEachDirectClassBodyNodeFlat(file, body, func(child uint32) {
		if file.FlatType(child) == "function_declaration" && r.shouldCountFunctionFlat(child, file) {
			count++
		}
	})
	return count
}

func forEachDirectClassBodyNodeFlat(file *scanner.File, body uint32, fn func(uint32)) {
	if body == 0 || fn == nil {
		return
	}
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		switch file.FlatType(child) {
		case "function_declaration", "property_declaration", "class_declaration":
			fn(child)
		case "class_member_declarations":
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				switch file.FlatType(gc) {
				case "function_declaration", "property_declaration", "class_declaration":
					fn(gc)
				}
			}
		}
	}
}

// countComplexInterfaceMembersFlat tallies direct class-body declarations,
// honoring ComplexInterfaceRule's includePrivateDeclarations and
// includeStaticDeclarations options:
//
//   - includePrivate=false: direct private fun/val/var declarations are
//     skipped.
//   - includeStatic=true: companion-object members are also counted as if
//     declared directly on the interface.
//
// Kotlin interfaces don't have a true "static" concept; companion-object
// members are the closest analogue for this rule.
func countComplexInterfaceMembersFlat(file *scanner.File, body uint32, includePrivate, includeStatic bool) int {
	if body == 0 {
		return 0
	}
	members := 0
	countMember := func(member uint32) {
		t := file.FlatType(member)
		if t != "function_declaration" && t != "property_declaration" {
			return
		}
		if !includePrivate && file.FlatHasModifier(member, "private") {
			return
		}
		members++
	}
	visitChild := func(child uint32) {
		t := file.FlatType(child)
		if t == "function_declaration" || t == "property_declaration" {
			countMember(child)
			return
		}
		if includeStatic && t == "companion_object" {
			if companionBody, ok := file.FlatFindChild(child, "class_body"); ok {
				for j := 0; j < file.FlatChildCount(companionBody); j++ {
					countMember(file.FlatChild(companionBody, j))
				}
			}
		}
	}
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		visitChild(child)
		if file.FlatType(child) == "class_member_declarations" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				visitChild(file.FlatChild(child, j))
			}
		}
	}
	return members
}
