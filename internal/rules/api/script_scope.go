package api

import "github.com/kaeawc/krit/internal/scanner"

// ScriptScope declares which Kotlin script (.kts) files a rule runs on. It
// only gates Kotlin-language dispatch of .kts files; Gradle-language rules
// (NeedsGradle) read build scripts through their own phase and ignore it.
type ScriptScope uint8

const (
	// ScriptsDefault resolves the scope from the rule's category and the
	// override table in this file. It is the zero value.
	ScriptsDefault ScriptScope = iota
	// ScriptsAll runs on Gradle build scripts and standalone scripts.
	ScriptsAll
	// ScriptsStandalone runs on standalone scripts (*.main.kts and other
	// non-Gradle .kts files) only.
	ScriptsStandalone
	// ScriptsGradle runs on Gradle build scripts (*.gradle.kts) only.
	ScriptsGradle
	// ScriptsNone never runs on a .kts file.
	ScriptsNone
)

// String returns the config spelling of the scope.
func (s ScriptScope) String() string {
	switch s {
	case ScriptsAll:
		return "all"
	case ScriptsStandalone:
		return "standalone"
	case ScriptsGradle:
		return "gradle"
	case ScriptsNone:
		return "none"
	default:
		return "default"
	}
}

// Applies reports whether a rule with this scope runs on a script of kind k.
// Regular Kotlin files always pass. An unresolved ScriptsDefault passes, so
// callers resolve the scope with RuleScriptScope first.
func (s ScriptScope) Applies(k scanner.ScriptKind) bool {
	if k == scanner.ScriptNone {
		return true
	}
	switch s {
	case ScriptsNone:
		return false
	case ScriptsStandalone:
		return k == scanner.ScriptStandalone
	case ScriptsGradle:
		return k == scanner.ScriptGradle
	default:
		return true
	}
}

// defaultScriptScopeByCategory is the script policy for whole rule sets.
// Scripts have no Android framework classes, no Compose or DI graph, no
// module public API and no test sources, so the rule sets built on those
// are off. Everything else matches detekt, which analyzes .kts like .kt.
// A category missing here runs on scripts.
var defaultScriptScopeByCategory = map[string]ScriptScope{
	"a11y":            ScriptsNone,
	"android-lint":    ScriptsNone,
	"architecture":    ScriptsNone,
	"compose":         ScriptsNone,
	"database":        ScriptsNone,
	"dead-code":       ScriptsNone,
	"di-hygiene":      ScriptsNone,
	"i18n":            ScriptsNone,
	"libraries":       ScriptsNone,
	"observability":   ScriptsNone,
	"precompile":      ScriptsNone,
	"privacy":         ScriptsNone,
	"resource-cost":   ScriptsNone,
	"testing-quality": ScriptsNone,
}

// scriptScopeOverrides adjusts single rules inside categories that run on
// scripts. Each entry follows what detekt 1.23.8 does on a script or a
// decision about entry points; the comment names the reason.
var scriptScopeOverrides = map[string]ScriptScope{
	// detekt excludes **/*.kts in its default config.
	"MagicNumber":               ScriptsNone,
	"MissingPackageDeclaration": ScriptsNone,
	// Script top-level code is not a declaration of the file: detekt reads
	// script properties as local variables and skips these file-shape rules.
	"MatchingDeclarationName": ScriptsNone,
	"MayBeConstant":           ScriptsNone,
	"NestedClassesVisibility": ScriptsNone,
	"TopLevelPropertyNaming":  ScriptsNone,
	// A script has no main entry point, and its top level may exit or throw.
	"ExitOutsideMain":         ScriptsNone,
	"ThrowingExceptionInMain": ScriptsNone,
	// Console output is the interface of a script.
	"PrintlnInProduction":         ScriptsNone,
	"PrintStackTraceInProduction": ScriptsNone,
	// Script blocks use !! on listFiles() and findProperty(), and cast Gradle's extra properties and
	// delegates, with no shorter alternative.
	"UnsafeCallOnNullableType": ScriptsNone,
	"ImplicitDefaultLocale":    ScriptsNone,
	// Build scripts throw for build errors, cast extra properties and chain long DSL calls. A standalone
	// script keeps these rules.
	"TooGenericExceptionThrown": ScriptsStandalone,
	"UnsafeCast":                ScriptsStandalone,
	"MaxLineLength":             ScriptsStandalone,
}

// RuleScriptScope resolves the script scope of r: its own Scripts field, then
// the per-rule override table, then the category default, then ScriptsAll.
func RuleScriptScope(r *Rule) ScriptScope {
	if r == nil {
		return ScriptsAll
	}
	if r.Scripts != ScriptsDefault {
		return r.Scripts
	}
	if s, ok := scriptScopeOverrides[r.ID]; ok {
		return s
	}
	if s, ok := defaultScriptScopeByCategory[r.Category]; ok {
		return s
	}
	return ScriptsAll
}

// RuleAppliesToScript reports whether r runs on a script of kind k under the
// built-in policy. It is true for ScriptNone.
func RuleAppliesToScript(r *Rule, k scanner.ScriptKind) bool {
	return RuleScriptScope(r).Applies(k)
}

// ScriptScopeOverrides returns a copy of the per-rule script scope table.
func ScriptScopeOverrides() map[string]ScriptScope {
	out := make(map[string]ScriptScope, len(scriptScopeOverrides))
	for id, s := range scriptScopeOverrides {
		out[id] = s
	}
	return out
}

// ScriptScopeCategoryDefaults returns a copy of the per-category script scope table.
func ScriptScopeCategoryDefaults() map[string]ScriptScope {
	out := make(map[string]ScriptScope, len(defaultScriptScopeByCategory))
	for c, s := range defaultScriptScopeByCategory {
		out[c] = s
	}
	return out
}
