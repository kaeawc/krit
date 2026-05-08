package mcp

import "strings"

// Discriminator constants for each tool. These are the single source of truth
// for the mode/operation/query enum values: tool definitions in tools.go derive
// their schema enums from these slices, dispatchers switch on these constants,
// and error messages format the same list. Adding a new operation requires
// editing only one place per tool.

// analyze.mode values.
const (
	modeCode    = "code"
	modeProject = "project"
	modeAndroid = "android"
	modeImpact  = "impact"
)

var analyzeModes = []string{modeCode, modeProject, modeAndroid, modeImpact}

// fix.operation values.
const (
	opFixSuggest  = "suggest"
	opFixSuppress = "suppress"
)

var fixOperations = []string{opFixSuggest, opFixSuppress}

// rules.operation values.
const (
	opRulesExplain    = "explain"
	opRulesSearch     = "search"
	opRulesCategories = "categories"
	opRulesConfigure  = "configure"
)

var rulesOperations = []string{opRulesExplain, opRulesSearch, opRulesCategories, opRulesConfigure}

// symbols.operation values.
const (
	opSymbolsOutline    = "outline"
	opSymbolsReferences = "references"
)

var symbolsOperations = []string{opSymbolsOutline, opSymbolsReferences}

// types.query values.
const (
	queryTypesClasses        = "classes"
	queryTypesHierarchy      = "hierarchy"
	queryTypesImports        = "imports"
	queryTypesSealedVariants = "sealed_variants"
	queryTypesEnumEntries    = "enum_entries"
	queryTypesFunctionSigs   = "function_signatures"
)

var typesQueries = []string{
	queryTypesClasses,
	queryTypesHierarchy,
	queryTypesImports,
	queryTypesSealedVariants,
	queryTypesEnumEntries,
	queryTypesFunctionSigs,
}

// structure.operation values.
const (
	opStructureModules  = "modules"
	opStructureProfile  = "profile"
	opStructureHotspots = "hotspots"
	opStructureBreadth  = "breadth"
	opStructurePkgDrift = "pkg_drift"
)

var structureOperations = []string{
	opStructureModules,
	opStructureProfile,
	opStructureHotspots,
	opStructureBreadth,
	opStructurePkgDrift,
}

// formatList renders a slice as a comma-separated string suitable for error
// messages and schema descriptions.
func formatList(values []string) string { return strings.Join(values, ", ") }
