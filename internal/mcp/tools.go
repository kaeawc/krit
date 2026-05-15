package mcp

import "github.com/kaeawc/krit/internal/jsonschema"

// This file defines the public MCP tool surface. krit exposes six tools, each
// using a discriminator field (mode / operation / query) to select among
// related sub-operations.
//
// The split follows the rules-vs-facts paradigm:
//
//   Rules side (tools that judge code):
//     - analyze:   run rules and return findings
//     - fix:       act on findings (suggest / preview / apply / suppress / baseline)
//     - rules:     registry metadata (explain / search / categories / configure)
//
//   Facts side (tools that describe code):
//     - symbols:   cross-file symbol facts (outline / find / references / usages / dead / rename)
//     - types:     semantic type facts (classes / hierarchy / imports / sealed_variants /
//                  enum_entries / function_signatures / resolve / class_info / nullable)
//     - structure: project structural facts (modules / profile / dependencies / api_surface /
//                  abi_hash / hotspots / breadth / leaky / pkg_drift / layer_violations)
//     - metrics:   historical finding counts persisted by `krit metrics log`
//
// Each tool's handler lives in its own tool_*.go file. Shared helpers live in
// tool_helpers.go.

// toolDefinitions returns the list of MCP tool definitions.
func toolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		analyzeToolDef(),
		fixToolDef(),
		rulesToolDef(),
		metricsToolDef(),
		symbolsToolDef(),
		typesToolDef(),
		structureToolDef(),
		snapshotToolDef(),
	}
}

func metricsToolDef() ToolDefinition {
	return ToolDefinition{
		Name:        "metrics",
		Description: "Query rule-level finding count history written by `krit metrics log`. `operation=query` returns timestamp/count/delta rows as JSON.",
		InputSchema: jsonschema.Object(map[string]*jsonschema.Schema{
			"operation": jsonschema.StringEnum([]string{"query"}, "query: read metrics JSONL and return per-rule time-series rows").WithDefault("query"),
			"path":      jsonschema.String("Metrics JSONL path; defaults to .krit/metrics.jsonl"),
			"rule":      jsonschema.String("Rule name, for example LongMethod"),
			"since":     jsonschema.String("Optional lower bound as YYYY-MM-DD or RFC3339"),
		}).WithRequired("rule"),
	}
}

// analyzeToolDef returns the analyze tool definition.
func analyzeToolDef() ToolDefinition {
	return ToolDefinition{
		Name: "analyze",
		Description: "Run krit's static-analysis rules and return findings. " +
			"`mode` selects scope: `code` (single buffer), `project` (multi-path aggregate with summary or per-file detail), " +
			"`android` (manifest/resources/gradle/icons), `impact` (count findings per rule before flipping it on/off). " +
			"Filterable by `rules[]` and `severity`. " +
			"Reach for non-obvious questions like: \"How noisy would enabling MagicNumber be before I commit to turning it on?\" (`impact`); " +
			"\"Which rules generate the most findings, ranked?\" (`project` + summary).",
		InputSchema: jsonschema.Object(map[string]*jsonschema.Schema{
			"mode": jsonschema.StringEnum(analyzeModes,
				"code: single buffer; project: multi-path aggregate; android: manifest/resources/gradle/icons; impact: count findings per rule").
				WithDefault(modeCode),
			"code":         jsonschema.String("Required for mode=code/impact (when paths absent)"),
			"path":         jsonschema.String("File path for context (mode=code)"),
			"paths":        jsonschema.Array(jsonschema.String(""), "Required for mode=project; optional for impact"),
			"project_path": jsonschema.String("Required for mode=android"),
			"scope":        jsonschema.StringEnum([]string{"manifest", "resources", "gradle", "icons", "all"}, "Android scope (mode=android)").WithDefault("all"),
			"format":       jsonschema.StringEnum([]string{"summary", "detailed"}, "Output format (mode=project)").WithDefault("summary"),
			"config":       jsonschema.String("Path to krit.yml (mode=project)"),
			"rules":        jsonschema.Array(jsonschema.String(""), "Filter to specific rule names"),
			"severity":     jsonschema.StringEnum([]string{"error", "warning", "info"}, "Minimum severity to include"),
		}),
	}
}

// fixToolDef returns the fix tool definition.
func fixToolDef() ToolDefinition {
	return ToolDefinition{
		Name: "fix",
		Description: "Get or apply auto-fixes derived from analysis. `operation`: `suggest` (list fixable findings with safety levels), " +
			"`suppress` (emit the exact `@Suppress(\"RuleName\")` annotation and insertion line for one finding — no manual editing). " +
			"Always gate writes with `fix_level`: cosmetic / idiomatic / semantic. " +
			"Reach for: \"Silence this single finding without rewriting the code\" (`suppress`); " +
			"\"List every cosmetic fix available in this file\" (`suggest` + `cosmetic`).",
		InputSchema: jsonschema.Object(map[string]*jsonschema.Schema{
			"operation": jsonschema.StringEnum(fixOperations,
				"suggest: list fixable findings; suppress: emit @Suppress annotation").
				WithDefault(opFixSuggest),
			"code":      jsonschema.String("Kotlin source code"),
			"path":      jsonschema.String("File path for context"),
			"fix_level": jsonschema.StringEnum([]string{"cosmetic", "idiomatic", "semantic", "all"}, "Maximum fix safety level (operation=suggest)").WithDefault("idiomatic"),
			"rule":      jsonschema.String("Rule name to suppress (operation=suppress)"),
			"line":      jsonschema.Integer("Line number to suppress on (operation=suppress)"),
			"scope":     jsonschema.StringEnum([]string{"file", "declaration"}, "Suppression scope (operation=suppress)").WithDefault("declaration"),
		}),
	}
}

// rulesToolDef returns the rules tool definition.
func rulesToolDef() ToolDefinition {
	return ToolDefinition{
		Name: "rules",
		Description: "Discover, explain, and configure krit's rules across all categories. `operation`: " +
			"`explain` (one rule by exact name), `search` (full-text by concept — use this when you don't know the rule name), " +
			"`categories` (list all categories with rule counts), `configure` (generate the krit.yml stanza for a rule with active/severity overrides). " +
			"Reach for: \"What rules cover null safety?\" (`search`); " +
			"\"Which categories have the most rules?\" (`categories`); " +
			"\"Give me the YAML to disable MagicNumber and downgrade LongMethod to warning\" (`configure`).",
		InputSchema: jsonschema.Object(map[string]*jsonschema.Schema{
			"operation": jsonschema.StringEnum(rulesOperations,
				"explain: rule metadata; search: full-text concept search; categories: rule counts per category; configure: krit.yml YAML").
				WithDefault(opRulesExplain),
			"rule":      jsonschema.String("Rule name (operation=explain/configure)"),
			"query":     jsonschema.String("Free-text search query (operation=search)"),
			"category":  jsonschema.String("Filter to one category (operation=search)"),
			"precision": jsonschema.StringEnum([]string{"heuristic/text-backed", "ast-backed", "project-structure-aware", "type-aware", "policy"}, "Filter by precision tier (operation=search)"),
			"needs":     jsonschema.Array(jsonschema.String(""), "Require these capability labels — every label must be present (operation=search). Use 'oracle' as a shorthand for any oracle:* bit."),
			"without":   jsonschema.Array(jsonschema.String(""), "Exclude rules that declare any of these capability labels (operation=search). 'without: [oracle]' filters out every NeedsOracle* rule."),
			"active":    jsonschema.Boolean("Override active flag (operation=configure)"),
			"severity":  jsonschema.StringEnum([]string{"error", "warning", "info"}, "Override severity (operation=configure)"),
		}),
	}
}

// symbolsToolDef returns the symbols tool definition.
func symbolsToolDef() ToolDefinition {
	return ToolDefinition{
		Name: "symbols",
		Description: "Cross-file symbol navigation, backed by krit's bloom-filter-accelerated CodeIndex. `operation`: " +
			"`outline` (all decls in a file), " +
			"`references` (raw usage list across .kt/.java/.xml). " +
			"Reach for: \"What classes/functions are declared in this file?\" (`outline`); " +
			"\"Find every usage of `getUserId` across Kotlin, Java, and XML\" (`references`).",
		InputSchema: jsonschema.Object(map[string]*jsonschema.Schema{
			"operation": jsonschema.StringEnum(symbolsOperations,
				"outline: file decls; references: cross-file usage search").
				WithDefault(opSymbolsReferences),
			"code":          jsonschema.String("Required for operation=outline"),
			"path":          jsonschema.String("File path (operation=outline)"),
			"name":          jsonschema.String("Symbol name (operation=references)"),
			"project_paths": jsonschema.Array(jsonschema.String(""), "Directories to search (operation=references)"),
			"include_java":  jsonschema.Boolean("Include .java files (operation=references)"),
			"include_xml":   jsonschema.Boolean("Include .xml files (operation=references)"),
		}),
	}
}

// typesToolDef returns the types tool definition.
func typesToolDef() ToolDefinition {
	return ToolDefinition{
		Name: "types",
		Description: "Semantic type queries via krit's type inference — smart-cast aware, parallel-indexed. `query`: " +
			"`classes`, `hierarchy`, `imports`, `sealed_variants`, `enum_entries`, `function_signatures`. " +
			"Reach for: \"Give me every variant of this sealed class so I can write an exhaustive `when`\" (`sealed_variants`); " +
			"\"Show me the inheritance chain for every class in this file\" (`hierarchy`).",
		InputSchema: jsonschema.Object(map[string]*jsonschema.Schema{
			"query": jsonschema.StringEnum(typesQueries, "Type information to retrieve"),
			"code":  jsonschema.String("Kotlin source code"),
			"path":  jsonschema.String("File path for context"),
		}).WithRequired("code", "query"),
	}
}

// snapshotToolDef returns the snapshot tool definition.
func snapshotToolDef() ToolDefinition {
	return ToolDefinition{
		Name: "snapshot",
		Description: "Read structural snapshots captured by `krit snapshot capture` / `krit snapshot backfill`. " +
			"`operation`: `status` (list captured shas + manifests), `info` (one sha's manifest), " +
			"`timeline` (a scalar metric over captured shas, sparse across history), " +
			"`diff` (added/removed files, symbols, modules, edges, plus repo + module metric deltas between two captured shas). " +
			"Reach for: \"How did fan-in on :feature:checkout move over the last 50 commits?\" (`timeline` + scope=module); " +
			"\"What structural changes does this PR introduce vs. main?\" (`diff`).",
		InputSchema: jsonschema.Object(map[string]*jsonschema.Schema{
			"operation": jsonschema.StringEnum([]string{"status", "info", "timeline", "diff"},
				"status: list captured snapshots; info: one sha's manifest; timeline: scalar metric series; diff: structural delta between two shas").
				WithDefault("status"),
			"repo_root":  jsonschema.String("Repo root (default: cwd)"),
			"commit_sha": jsonschema.String("Commit sha or ref (operation=info)"),
			"scope":      jsonschema.StringEnum([]string{"repo", "module", "file"}, "Timeline scope (operation=timeline)").WithDefault("repo"),
			"target":     jsonschema.String("Module path (':app') or repo-relative file path; required for scope=module/file"),
			"metric":     jsonschema.String("loc|bytes|symbols|public_symbols|cyclomatic|files|fan_in|fan_out|modules (operation=timeline)"),
			"from":       jsonschema.String("From sha or ref (operation=diff)"),
			"to":         jsonschema.String("To sha or ref (operation=diff)"),
		}),
	}
}

// structureToolDef returns the structure tool definition.
func structureToolDef() ToolDefinition {
	return ToolDefinition{
		Name: "structure",
		Description: "Project-level structural facts — modules, profile, dependencies, architecture metrics. `operation`: " +
			"`modules` (Gradle module discovery: paths, source roots, dependency graph; pass `module` to scope to one), " +
			"`profile` (detect frameworks: Compose, Room, SQLDelight, Hilt, Coroutines from Gradle build files), " +
			"`hotspots` (project-wide symbol fan-in: which symbols have the most callers — high blast-radius for breaking changes), " +
			"`breadth` (files importing the most symbols — refactor candidates), " +
			"`pkg_drift` (files whose Kotlin package doesn't match their directory). " +
			"Reach for: \"What modules does :feature:checkout depend on transitively?\" (`modules` + module); " +
			"\"What symbols, if I changed them, would break the most code?\" (`hotspots`); " +
			"\"Does this project use Room or SQLDelight?\" (`profile`).",
		InputSchema: jsonschema.Object(map[string]*jsonschema.Schema{
			"operation": jsonschema.StringEnum(structureOperations,
				"modules: gradle graph; profile: framework detection; hotspots: fan-in ranking; breadth: import-count ranking; pkg_drift: package/dir mismatch").
				WithDefault(opStructureModules),
			"project_root": jsonschema.String("Project root (operation=modules/profile)"),
			"module":       jsonschema.String("Specific module path (operation=modules)"),
			"paths":        jsonschema.Array(jsonschema.String(""), "Directories to scan (operation=hotspots/breadth/pkg_drift)"),
			"threshold":    jsonschema.Integer("Min count threshold (operation=hotspots/breadth)"),
			"limit":        jsonschema.Integer("Max results to return (operation=hotspots/breadth/pkg_drift)"),
		}),
	}
}
