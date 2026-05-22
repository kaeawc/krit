package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// PrecompileDuplicateDeclarationRule flags two top-level declarations in
// the same file that share a name and (for functions) a matching parameter
// type signature. Mirrors kotlinc's CONFLICTING_OVERLOADS / REDECLARATION
// diagnostics, restricted to the file-local subset:
//
//   - Only direct children of source_file are considered.
//   - Two `fun foo(x: Int)` declarations conflict; `fun foo(x: Int)` and
//     `fun foo(x: String)` do not (overloads).
//   - Two top-level `class Foo` declarations conflict.
//   - Two top-level `val foo` declarations conflict.
//   - Cross-file redeclaration is out of scope (requires NeedsCrossFile).
//   - Conflicts across different declaration kinds (e.g. class vs val) are
//     not flagged here; that case needs broader semantics.
//
// Parameter type comparison is textual against the tree-sitter type node.
// This is conservative: aliases that resolve to the same type read as
// distinct here, so this rule will sometimes miss real conflicts that a
// resolver-backed pass would catch. It does not produce false positives.
type PrecompileDuplicateDeclarationRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrecompileDuplicateDeclarationRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *PrecompileDuplicateDeclarationRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	seen := map[string]uint32{}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		var kind, name, sig string
		switch file.FlatType(child) {
		case "function_declaration":
			kind = "fun"
			name = overrideEnclosingClassName(file, child)
			sig = duplicateDeclFunctionSignature(file, child)
		case "class_declaration":
			kind = "class"
			name = overrideEnclosingClassName(file, child)
		case "property_declaration":
			kind = "val"
			name = duplicateDeclPropertyName(file, child)
		default:
			continue
		}
		if name == "" {
			continue
		}
		key := kind + "|" + name + "|" + sig
		if first, dup := seen[key]; dup {
			ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
				fmt.Sprintf("Duplicate top-level %s declaration `%s`; already declared on line %d.",
					kind, name, file.FlatRow(first)+1))
			continue
		}
		seen[key] = child
	}
}

// duplicateDeclPropertyName extracts the bound name from a property
// declaration. Destructuring declarations (`val (a, b) = pair`) bind
// multiple names and are skipped to avoid ambiguous keys.
func duplicateDeclPropertyName(file *scanner.File, idx uint32) string {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "variable_declaration":
			for sub := file.FlatFirstChild(child); sub != 0; sub = file.FlatNextSib(sub) {
				if file.FlatType(sub) == "simple_identifier" {
					return strings.TrimSpace(file.FlatNodeText(sub))
				}
			}
		case "multi_variable_declaration":
			return ""
		}
	}
	return ""
}

// duplicateDeclFunctionSignature returns a normalized text of the
// parameter type list of a function declaration. Receiver type, return
// type, default values, and parameter names are ignored — only the
// ordered parameter type texts determine overload identity.
func duplicateDeclFunctionSignature(file *scanner.File, idx uint32) string {
	params, ok := file.FlatFindChild(idx, "function_value_parameters")
	if !ok {
		return "()"
	}
	var parts []string
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		parts = append(parts, duplicateDeclParameterType(file, child))
	}
	return "(" + strings.Join(parts, ",") + ")"
}

// duplicateDeclParameterType returns the trimmed, whitespace-normalized
// text of a parameter's type node. Returns "?" for unparseable
// parameters so that two such parameters do not collide by accident.
func duplicateDeclParameterType(file *scanner.File, paramIdx uint32) string {
	seenColon := false
	for sub := file.FlatFirstChild(paramIdx); sub != 0; sub = file.FlatNextSib(sub) {
		if file.FlatType(sub) == ":" {
			seenColon = true
			continue
		}
		if !seenColon {
			continue
		}
		if !file.FlatIsNamed(sub) {
			continue
		}
		text := strings.TrimSpace(file.FlatNodeText(sub))
		return strings.Join(strings.Fields(text), "")
	}
	return "?"
}
