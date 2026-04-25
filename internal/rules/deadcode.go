package rules

import (
	"fmt"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// DeadCodeRule detects public/internal symbols that are never referenced from any other file.
// This is a cross-file rule that requires the CodeIndex to be populated.
type DeadCodeRule struct {
	BaseRule
	// IgnoreCommentReferences: if true (default), a symbol referenced only in comments
	// is still considered dead code. If false, comment references count as usage.
	IgnoreCommentReferences bool
}

// Cross-file and parsed-files rules are identified structurally now —
// callers type-assert to anonymous interfaces describing just the
// CheckCrossFile / CheckParsedFiles method sets. See v2.Rule.Needs
// (NeedsCrossFile / NeedsParsedFiles) for the canonical form.

// DeadCode is advertised as not-fixable because the symbol-deletion
// span (line range, leading KDoc, surrounding blank lines) is
// non-trivial to compute from the cross-file index alone and the
// current Check() path never populates a Fix. Removing a dead symbol
// remains a manual operation. Left in fixes.go history for when the
// per-symbol deletion pipeline lands.
func (r *DeadCodeRule) IsFixable() bool { return false }

// Confidence reports a tier-2 (medium) base confidence. The rule
// relies on the cross-file code index to detect unreferenced symbols
// and then runs shouldSkipSymbol to filter framework entry points,
// overrides, tests, and well-known lifecycle methods. It does NOT
// recognize DI annotations (@Provides, @Binds, @Inject,
// @ContributesBinding, @IntoSet, etc.) — a Dagger/Hilt/Kotlin-Inject
// binding is "unreferenced" by the code index but wired up at
// compile time by the DI framework. This is the false-positive
// pattern called out in roadmap/17; medium confidence keeps the
// rule honest until DI awareness lands.
func (r *DeadCodeRule) Confidence() float64 { return 0.75 }

// check runs against the full code index.
func (r *DeadCodeRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex

	unused := index.UnusedSymbols(r.IgnoreCommentReferences)
	for _, sym := range unused {
		// Skip common false positives
		if shouldSkipSymbol(sym) {
			continue
		}

		kindLabel := sym.Kind

		// Check if it's referenced in comments only
		hasCommentRef := false
		if r.IgnoreCommentReferences {
			hasCommentRef = index.IsReferencedOutsideFile(sym.Name, sym.File) &&
				!index.IsReferencedOutsideFileExcludingComments(sym.Name, sym.File)
		}

		var msg string
		if hasCommentRef {
			msg = fmt.Sprintf("%s %s '%s' appears to be unused. It is only referenced in comments, not in code.",
				strings.Title(sym.Visibility), kindLabel, sym.Name)
		} else {
			msg = fmt.Sprintf("%s %s '%s' appears to be unused. It is not referenced from any other file.",
				strings.Title(sym.Visibility), kindLabel, sym.Name)
		}

		f := scanner.Finding{
			File:     sym.File,
			Line:     sym.Line,
			Col:      1,
			RuleSet:  r.RuleSetName,
			Rule:     r.RuleName,
			Severity: r.Sev,
			Message:  msg,
		}

		// Fix: delete the declaration
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   sym.StartByte,
			EndByte:     sym.EndByte,
			Replacement: "",
		}

		ctx.Emit(f)
	}
}

func shouldSkipSymbol(sym scanner.Symbol) bool {
	// Skip overrides (called by framework/parent)
	if sym.IsOverride {
		return true
	}

	// Skip test classes and functions (invoked by test runner, not code)
	if sym.IsTest {
		return true
	}
	if strings.Contains(sym.File, "/test/") || strings.Contains(sym.File, "/androidTest/") ||
		strings.Contains(sym.File, "/benchmark/") || strings.Contains(sym.File, "/canary/") {
		return true
	}

	// Skip companion objects
	if sym.Kind == "object" && sym.Name == "Companion" {
		return true
	}

	// Skip Android lifecycle and framework entry points
	frameworkEntryPoints := map[string]bool{
		"main": true, "onCreate": true, "onDestroy": true, "onStart": true,
		"onStop": true, "onResume": true, "onPause": true, "onCreateView": true,
		"onViewCreated": true, "onBind": true, "onReceive": true, "invoke": true,
		"onAttach": true, "onDetach": true, "onDestroyView": true, "onActivityCreated": true,
		"onCreateOptionsMenu": true, "onOptionsItemSelected": true, "onSaveInstanceState": true,
		"onRestoreInstanceState": true, "onNewIntent": true, "onActivityResult": true,
		"onRequestPermissionsResult": true, "onConfigurationChanged": true,
		"onCreateDialog": true, "onDismiss": true, "onCancel": true,
	}
	if frameworkEntryPoints[sym.Name] {
		return true
	}

	// Skip serialization/reflection hooks
	if sym.Name == "serialVersionUID" || sym.Name == "CREATOR" {
		return true
	}

	// Skip @Preview/@Composable functions (used by Android Studio, not code)
	if strings.Contains(sym.Name, "Preview") {
		return true
	}

	// Skip classes that are likely referenced from XML or framework entry points.
	if isLikelyFrameworkEntryTypeName(sym.Name) {
		return true
	}

	return false
}
