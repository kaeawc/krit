package rules

import (
	"bytes"
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// IntoMapDuplicateKeyRule detects two or more @IntoMap @Provides/@Binds
// functions in the same enclosing module/component that declare the same key
// literal. Duplicate map keys create conflicting contributions.
type IntoMapDuplicateKeyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *IntoMapDuplicateKeyRule) Confidence() float64 { return 0.7 }

// IntoSetDuplicateTypeRule detects two or more @IntoSet @Provides functions in
// the same enclosing module/component that return the same concrete impl
// expression — Dagger's set dedupes by value, so contributions are lost.
type IntoSetDuplicateTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (lower) base confidence. Detection compares the
// final returned-expression callee text; aliases and indirection are not
// followed.
func (r *IntoSetDuplicateTypeRule) Confidence() float64 { return 0.5 }

func (r *IntoMapDuplicateKeyRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	type entry struct {
		file *scanner.File
		idx  uint32
		name string
	}
	groups := make(map[string][]entry)
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("IntoMap")) {
			continue
		}
		pkg := sourcePackageName(file)
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "function_declaration" {
				return
			}
			if !hasAnnotationFlat(file, idx, "IntoMap") {
				return
			}
			if !hasAnnotationFlat(file, idx, "Provides") && !hasAnnotationFlat(file, idx, "Binds") {
				return
			}
			keyLit, ok := firstMapKeyAnnotationLiteralFlat(file, idx)
			if !ok {
				return
			}
			scope := pkg + "::" + enclosingClassChainFlat(file, idx)
			groupKey := scope + "::" + keyLit
			name := extractIdentifierFlat(file, idx)
			groups[groupKey] = append(groups[groupKey], entry{file: file, idx: idx, name: name})
		})
	}
	for groupKey, entries := range groups {
		if len(entries) < 2 {
			continue
		}
		keyLit := suffixAfterLastDoubleColon(groupKey)
		for _, e := range entries {
			n := e.name
			if n == "" {
				n = "binding"
			}
			ctx.Emit(r.Finding(
				e.file,
				e.file.FlatRow(e.idx)+1,
				1,
				fmt.Sprintf("@IntoMap function '%s' shares key %s with %d other contribution(s) in the same module/component; use a distinct map key or remove the duplicate contribution.", n, keyLit, len(entries)-1),
			))
		}
	}
}

func suffixAfterLastDoubleColon(s string) string {
	idx := strings.LastIndex(s, "::")
	if idx < 0 {
		return ""
	}
	return s[idx+2:]
}

func (r *IntoSetDuplicateTypeRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	type entry struct {
		file *scanner.File
		idx  uint32
		name string
	}
	groups := make(map[string][]entry)
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("IntoSet")) {
			continue
		}
		pkg := sourcePackageName(file)
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "function_declaration" {
				return
			}
			if !hasAnnotationFlat(file, idx, "IntoSet") {
				return
			}
			if !hasAnnotationFlat(file, idx, "Provides") {
				return
			}
			impl := intoSetReturnedExpressionCallee(file, idx)
			if impl == "" {
				return
			}
			scope := pkg + "::" + enclosingClassChainFlat(file, idx)
			groupKey := scope + "::" + impl
			name := extractIdentifierFlat(file, idx)
			groups[groupKey] = append(groups[groupKey], entry{file: file, idx: idx, name: name})
		})
	}
	for groupKey, entries := range groups {
		if len(entries) < 2 {
			continue
		}
		impl := suffixAfterLastDoubleColon(groupKey)
		for _, e := range entries {
			n := e.name
			if n == "" {
				n = "binding"
			}
			ctx.Emit(r.Finding(
				e.file,
				e.file.FlatRow(e.idx)+1,
				1,
				fmt.Sprintf("@IntoSet function '%s' contributes '%s' which is also contributed by %d other binding(s) in the same module/component; the set dedupes, so contributions are lost.", n, impl, len(entries)-1),
			))
		}
	}
}

// intoSetReturnedExpressionCallee returns the simple-identifier callee of the
// expression-body or single-return statement of `idx`. Used to compare two
// `@IntoSet` providers that construct the same concrete implementation.
func intoSetReturnedExpressionCallee(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	// Expression body: `= Foo()` — find call_expression sibling of `=`.
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "=" {
			continue
		}
		next := file.FlatNextSib(child)
		for next != 0 && !file.FlatIsNamed(next) {
			next = file.FlatNextSib(next)
		}
		if next == 0 {
			break
		}
		return calleeIdentifierText(file, next)
	}
	// Block body: look for the single return statement.
	body, ok := file.FlatFindChild(idx, "function_body")
	if !ok || body == 0 {
		return ""
	}
	var lone uint32
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		t := file.FlatType(child)
		if t == "{" || t == "}" {
			continue
		}
		if lone != 0 {
			return ""
		}
		lone = child
	}
	if lone == 0 {
		return ""
	}
	if file.FlatType(lone) == "jump_expression" {
		// `return Foo()`: walk down to call_expression
		for c := file.FlatFirstChild(lone); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				return calleeIdentifierText(file, c)
			}
		}
	}
	return calleeIdentifierText(file, lone)
}

func calleeIdentifierText(file *scanner.File, idx uint32) string {
	if file.FlatType(idx) != "call_expression" {
		return ""
	}
	callee := file.FlatFirstChild(idx)
	for callee != 0 && !file.FlatIsNamed(callee) {
		callee = file.FlatNextSib(callee)
	}
	if callee == 0 {
		return ""
	}
	switch file.FlatType(callee) {
	case "simple_identifier":
		return strings.TrimSpace(file.FlatNodeText(callee))
	case "navigation_expression":
		text := strings.TrimSpace(file.FlatNodeText(callee))
		if dot := strings.LastIndex(text, "."); dot >= 0 {
			return text[dot+1:]
		}
		return text
	}
	return ""
}
