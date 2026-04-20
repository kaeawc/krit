package rules

// Android Resource XML rules — shared infrastructure.
// These rules analyze Android resource XML files (layouts, values) via the
// ResourceIndex provided by the android package's resource scanner.
//
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/
//
// Rule implementations are split across files by category:
//   android_resource_layout.go  — layout structure rules
//   android_resource_a11y.go    — accessibility + i18n rules
//   android_resource_style.go   — dimension/style rules
//   android_resource_ids.go     — ID/reference/namespace rules
//   android_resource_rtl.go     — RTL rules
//   android_resource_values.go  — value/string rules

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// Resource-rule marker types. These are empty structs embedded by
// resource rule implementations. Android dependency metadata is surfaced
// onto v2.Rule.AndroidDeps by codegen in zz_registry_gen.go, which reads
// it back from the AndroidDependencies() methods below.

type ResourceBase struct{}

func (ResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepValues | AndroidDepLayout
}

type LayoutResourceBase struct{ ResourceBase }

func (LayoutResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepLayout
}

type ValuesResourceBase struct{ ResourceBase }

func (ValuesResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepValues
}

type ValuesStringsResourceBase struct{ ResourceBase }

func (ValuesStringsResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepValuesStrings
}

type ValuesPluralsResourceBase struct{ ResourceBase }

func (ValuesPluralsResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepValuesPlurals
}

type ValuesArraysResourceBase struct{ ResourceBase }

func (ValuesArraysResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepValuesArrays
}

type ValuesExtraTextResourceBase struct{ ResourceBase }

func (ValuesExtraTextResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepValuesExtraText
}

// ---------------------------------------------------------------------------
// Helper to create a resource finding
// ---------------------------------------------------------------------------

func resourceFinding(path string, line int, rule BaseRule, msg string) scanner.Finding {
	return scanner.Finding{
		File:     path,
		Line:     line,
		Col:      1,
		RuleSet:  rule.RuleSetName,
		Rule:     rule.RuleName,
		Severity: rule.Sev,
		Message:  msg,
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// walkViews calls fn for every view in the tree (pre-order).
func walkViews(v *android.View, fn func(*android.View)) {
	if v == nil {
		return
	}
	fn(v)
	for _, child := range v.Children {
		walkViews(child, fn)
	}
}

// walkViewsWithParent calls fn for every view in the tree with its parent.
func walkViewsWithParent(v *android.View, parent *android.View, fn func(v, parent *android.View)) {
	if v == nil {
		return
	}
	fn(v, parent)
	for _, child := range v.Children {
		walkViewsWithParent(child, v, fn)
	}
}

// truncate shortens a string for display in messages.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
