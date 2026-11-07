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

// classKindLabel names the declaration kind for messages: "Interface",
// "Object", "Enum class" or "Class".
// classIsEnum reports an `enum class`; the grammar keeps `enum` as a keyword child, not a modifier.
func classIsEnum(file *scanner.File, idx uint32) bool {
	return file.FlatHasChildOfType(idx, "enum") || file.FlatHasModifier(idx, "enum")
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
	IgnoreOverloaded           bool
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

// checkScopeFlat counts the functions declared directly in a file or in a class, object or enum entry body,
// keyed by name (extensions by `Receiver.name`), and reports each name that reaches the threshold.
func (r *MethodOverloadingRule) checkScopeFlat(ctx *api.Context, node uint32) {
	file := ctx.File
	counts := make(map[string]int)
	var order []string
	addFunction := func(fn uint32) {
		name := extractIdentifierFlat(file, fn)
		if name == "" {
			return
		}
		if recv := functionReceiverTypeText(file, fn); recv != "" {
			name = recv + "." + name
		}
		if counts[name] == 0 {
			order = append(order, name)
		}
		counts[name]++
	}
	var anchorRow, anchorCol int
	switch file.FlatType(node) {
	case "source_file":
		for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "function_declaration" {
				addFunction(c)
			}
		}
		anchorRow, anchorCol = 1, 1
		if hdr, ok := file.FlatFindChild(node, "package_header"); ok {
			anchorRow, anchorCol = file.FlatRow(hdr)+1, file.FlatCol(hdr)+1
		} else {
			for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "import_list", "import_header", "line_comment", "multiline_comment", "file_annotation":
					continue
				}
				anchorRow, anchorCol = file.FlatRow(c)+1, file.FlatCol(c)+1
				break
			}
		}
	case "class_declaration", "object_declaration", "companion_object":
		body, _ := file.FlatFindChild(node, "class_body")
		if body == 0 {
			body, _ = file.FlatFindChild(node, "enum_class_body")
		}
		if body == 0 {
			return
		}
		var collect func(b uint32)
		collect = func(b uint32) {
			for c := file.FlatFirstChild(b); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "function_declaration":
					addFunction(c)
				case "class_member_declarations":
					collect(c)
				}
			}
		}
		collect(body)
		anchorRow, anchorCol = declLine(file, node), declCol(file, node)
	default:
		return
	}
	for _, name := range order {
		if counts[name] > r.AllowedOverloads {
			ctx.EmitAt(anchorRow, anchorCol, fmt.Sprintf("The method '%s' is overloaded %d times.", name, counts[name]))
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
	IgnoreAnnotatedFunctions     []string
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
	if r.IgnoreDeprecated && flatHasAnnotationNamed(file, fnNode, "Deprecated") {
		return false
	}
	for _, name := range r.IgnoreAnnotatedFunctions {
		if name = strings.TrimPrefix(strings.TrimSpace(name), "@"); name != "" && flatHasAnnotationNamed(file, fnNode, name) {
			return false
		}
	}
	return true
}

func (r *TooManyFunctionsRule) countFunctionsInClassFlat(cls uint32, file *scanner.File) int {
	body, _ := file.FlatFindChild(cls, "class_body")
	if body == 0 {
		body, _ = file.FlatFindChild(cls, "enum_class_body")
	}
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

// countComplexInterfaceMembersFlat counts the properties and functions declared in an interface body as
// detekt's ComplexInterface does: private members only with includePrivate, companion members with
// includeStatic, and with ignoreOverloaded each function name once (extensions as `Receiver.name`).
func countComplexInterfaceMembersFlat(file *scanner.File, body uint32, includePrivate, includeStatic, ignoreOverloaded bool) int {
	if body == 0 {
		return 0
	}
	count := func(body uint32) int {
		props := 0
		seen := map[string]bool{}
		funcs := 0
		visit := func(member uint32) {
			t := file.FlatType(member)
			if t != "function_declaration" && t != "property_declaration" {
				return
			}
			if !includePrivate && file.FlatHasModifier(member, "private") {
				return
			}
			if t == "property_declaration" {
				props++
				return
			}
			if ignoreOverloaded {
				name := extractIdentifierFlat(file, member)
				if recv := functionReceiverTypeText(file, member); recv != "" {
					name = recv + "." + name
				}
				if seen[name] {
					return
				}
				seen[name] = true
			}
			funcs++
		}
		for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
			visit(c)
			if file.FlatType(c) == "class_member_declarations" {
				for g := file.FlatFirstChild(c); g != 0; g = file.FlatNextSib(g) {
					visit(g)
				}
			}
		}
		return props + funcs
	}
	members := count(body)
	if includeStatic {
		for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "companion_object" {
				if cb, ok := file.FlatFindChild(c, "class_body"); ok {
					members += count(cb)
				}
			}
		}
	}
	return members
}

// functionReceiverTypeText returns the receiver type text of an extension function, or "".
func functionReceiverTypeText(file *scanner.File, fn uint32) string {
	for c := file.FlatFirstChild(fn); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "simple_identifier":
			return ""
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			if n := file.FlatNextSib(c); n != 0 && file.FlatType(n) == "." {
				return file.FlatNodeText(c)
			}
		}
	}
	return ""
}
