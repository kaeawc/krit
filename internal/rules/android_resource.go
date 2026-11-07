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
	"strings"
)

// Resource-rule marker types. These are empty structs embedded by
// resource rule implementations. The rule registry source records Android
// dependency metadata on api.Rule.AndroidDeps.

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
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// walkViewsUnlessIgnored is walkViews but skips subtrees whose root has a
// tools:ignore naming one of issueIDs, matching lint's ancestor-scoped ignores.
func walkViewsUnlessIgnored(v *android.View, issueIDs []string, fn func(*android.View)) {
	if v == nil || resourceToolsIgnoreMatches(v, issueIDs...) {
		return
	}
	fn(v)
	for _, child := range v.Children {
		walkViewsUnlessIgnored(child, issueIDs, fn)
	}
}

// walkViewsWithParentUnlessIgnored is walkViewsWithParent with the same ignore handling.
func walkViewsWithParentUnlessIgnored(v *android.View, parent *android.View, issueIDs []string, fn func(v, parent *android.View)) {
	if v == nil || resourceToolsIgnoreMatches(v, issueIDs...) {
		return
	}
	fn(v, parent)
	for _, child := range v.Children {
		walkViewsWithParentUnlessIgnored(child, v, issueIDs, fn)
	}
}

// frameworkViewName returns the simple name of a layout tag that is an Android framework or
// AndroidX/Material view, or "" for app-defined views, so a custom com.app.ImageView is not mistaken for ImageView.
func frameworkViewName(tag string) string {
	if !strings.Contains(tag, ".") {
		return tag
	}
	for _, prefix := range []string{"android.", "androidx.", "com.google.android."} {
		if strings.HasPrefix(tag, prefix) {
			return tag[strings.LastIndex(tag, ".")+1:]
		}
	}
	return ""
}
