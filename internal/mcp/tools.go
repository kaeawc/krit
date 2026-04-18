package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// toolDefinitions returns the list of MCP tool definitions.
func toolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "analyze",
			Description: "Analyze Kotlin source code for issues. Returns findings with severity, rule, message, and suggested fixes.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code": map[string]interface{}{
						"type":        "string",
						"description": "Kotlin source code to analyze",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path (for context like package detection)",
					},
					"rules": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Specific rules to check (empty = all)",
					},
					"severity": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"error", "warning", "info"},
						"description": "Minimum severity to report",
					},
				},
				"required": []string{"code"},
			},
		},
		{
			Name:        "suggest_fixes",
			Description: "Get auto-fix suggestions for Kotlin code. Returns fixes with safety levels (cosmetic/idiomatic/semantic) and the exact code transformations.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code": map[string]interface{}{
						"type":        "string",
						"description": "Kotlin source code to analyze",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path (for context like package detection)",
					},
					"fix_level": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"cosmetic", "idiomatic", "semantic", "all"},
						"default":     "idiomatic",
						"description": "Maximum fix safety level to include",
					},
				},
				"required": []string{"code"},
			},
		},
		{
			Name:        "explain_rule",
			Description: "Get detailed explanation of a krit rule: what it checks, why it matters, auto-fix availability.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"rule": map[string]interface{}{
						"type":        "string",
						"description": "Rule name (e.g., 'MagicNumber', 'FragmentConstructor')",
					},
				},
				"required": []string{"rule"},
			},
		},
		{
			Name:        "inspect_types",
			Description: "Query type information for Kotlin code: classes, hierarchy, imports, function signatures, sealed variants, enum entries.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code": map[string]interface{}{
						"type":        "string",
						"description": "Kotlin source code to analyze",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path (for context)",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"classes", "hierarchy", "imports", "sealed_variants", "enum_entries", "function_signatures"},
						"description": "Type of information to retrieve",
					},
				},
				"required": []string{"code", "query"},
			},
		},
		{
			Name:        "find_references",
			Description: "Search for symbol references across Kotlin, Java, and XML files in a project.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Symbol name to search for",
					},
					"project_paths": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Directories to search in",
					},
					"include_java": map[string]interface{}{
						"type":        "boolean",
						"description": "Include .java files in search (default false)",
					},
					"include_xml": map[string]interface{}{
						"type":        "boolean",
						"description": "Include .xml files in search (default false)",
					},
				},
				"required": []string{"name", "project_paths"},
			},
		},
		{
			Name:        "analyze_android",
			Description: "Analyze Android project files: manifests, resources, Gradle build files, and icons. Returns findings from Android-specific rules.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_path": map[string]interface{}{
						"type":        "string",
						"description": "Root path to the Android project (or module) to scan",
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"manifest", "resources", "gradle", "icons", "all"},
						"default":     "all",
						"description": "Analysis scope: manifest, resources, gradle, icons, or all",
					},
				},
				"required": []string{"project_path"},
			},
		},
		{
			Name:        "inspect_modules",
			Description: "Discover Gradle module structure from settings.gradle.kts / settings.gradle. Returns module list, source roots, and dependencies.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_root": map[string]interface{}{
						"type":        "string",
						"description": "Root directory of the Gradle project (containing settings.gradle.kts or settings.gradle)",
					},
					"module": map[string]interface{}{
						"type":        "string",
						"description": "Optional specific module path to inspect (e.g. ':app', ':core:util')",
					},
				},
				"required": []string{"project_root"},
			},
		},
		{
			Name:        "analyze_project",
			Description: "Full project analysis with summary: total files, total findings, top rules, per-file breakdown.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"paths": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Directories or files to analyze",
					},
					"config": map[string]interface{}{
						"type":        "string",
						"description": "Path to krit.yml config file (optional)",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"summary", "detailed"},
						"default":     "summary",
						"description": "Output format: summary or detailed per-file breakdown",
					},
				},
				"required": []string{"paths"},
			},
		},
	}
}

// analyzeArgs are the arguments for the analyze tool.
type analyzeArgs struct {
	Code     string   `json:"code"`
	Path     string   `json:"path"`
	Rules    []string `json:"rules"`
	Severity string   `json:"severity"`
}

// toolAnalyze parses Kotlin code with the dispatcher and returns findings as JSON.
func (s *Server) toolAnalyze(arguments json.RawMessage) ToolResult {
	var args analyzeArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if args.Code == "" {
		return errorResult("'code' argument is required")
	}

	columns, err := s.parseAndAnalyzeColumns(args.Code, args.Path)
	if err != nil {
		return errorResult(err.Error())
	}

	// Filter by specific rules if requested
	if len(args.Rules) > 0 {
		ruleSet := make(map[string]bool, len(args.Rules))
		for _, r := range args.Rules {
			ruleSet[r] = true
		}
		columns = filterFindingColumns(&columns, func(columns *scanner.FindingColumns, row int) bool {
			return ruleSet[columns.RuleAt(row)]
		})
	}

	// Filter by severity if requested
	if args.Severity != "" {
		minLevel := severityLevel(args.Severity)
		columns = filterFindingColumns(&columns, func(columns *scanner.FindingColumns, row int) bool {
			return severityLevel(columns.SeverityAt(row)) <= minLevel
		})
	}

	return findingsToResultColumns(&columns)
}

// suggestFixesArgs are the arguments for the suggest_fixes tool.
type suggestFixesArgs struct {
	Code     string `json:"code"`
	Path     string `json:"path"`
	FixLevel string `json:"fix_level"`
}

// toolSuggestFixes returns fixable findings with fix details.
func (s *Server) toolSuggestFixes(arguments json.RawMessage) ToolResult {
	var args suggestFixesArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if args.Code == "" {
		return errorResult("'code' argument is required")
	}

	columns, err := s.parseAndAnalyzeColumns(args.Code, args.Path)
	if err != nil {
		return errorResult(err.Error())
	}

	// Filter to only fixable findings
	columns = filterFindingColumns(&columns, func(columns *scanner.FindingColumns, row int) bool {
		return columns.HasFix(row)
	})

	// Filter by fix level if not "all"
	if args.FixLevel != "" && args.FixLevel != "all" {
		maxLevel, ok := rules.ParseFixLevel(args.FixLevel)
		if !ok {
			return errorResult("invalid fix_level: " + args.FixLevel)
		}
		columns = filterFindingColumns(&columns, func(columns *scanner.FindingColumns, row int) bool {
			r := findRule(columns.RuleAt(row))
			if r == nil {
				return false
			}
			return rules.GetFixLevel(r) <= maxLevel
		})
	}

	return fixableToResultColumns(&columns)
}

// explainRuleArgs are the arguments for the explain_rule tool.
type explainRuleArgs struct {
	Rule string `json:"rule"`
}

// toolExplainRule looks up a rule in the registry and returns its metadata.
func (s *Server) toolExplainRule(arguments json.RawMessage) ToolResult {
	var args explainRuleArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if args.Rule == "" {
		return errorResult("'rule' argument is required")
	}

	r := findRule(args.Rule)
	if r == nil {
		return errorResult("unknown rule: " + args.Rule)
	}

	fixable := false
	if fr, ok := r.(rules.FixableRule); ok {
		fixable = fr.IsFixable()
	}

	fixLevel := ""
	if fixable {
		fixLevel = rules.GetFixLevel(r).String()
	}

	active := rules.IsDefaultActive(r.Name())

	info := map[string]interface{}{
		"name":        r.Name(),
		"description": r.Description(),
		"ruleSet":     r.RuleSet(),
		"severity":    r.Severity(),
		"active":      active,
		"fixable":     fixable,
	}
	if fixLevel != "" {
		info["fixLevel"] = fixLevel
	}

	data, _ := json.MarshalIndent(info, "", "  ")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(data)}},
	}
}

// findRule looks up a rule by name in the registry.
func findRule(name string) rules.Rule {
	for _, r := range rules.Registry {
		if r.Name() == name {
			return r
		}
	}
	return nil
}

// filterFindings filters findings using a predicate.
func filterFindings(findings []scanner.Finding, keep func(scanner.Finding) bool) []scanner.Finding {
	var result []scanner.Finding
	for _, f := range findings {
		if keep(f) {
			result = append(result, f)
		}
	}
	return result
}

// filterFindingColumns filters columnar findings using a row predicate.
func filterFindingColumns(columns *scanner.FindingColumns, keep func(*scanner.FindingColumns, int) bool) scanner.FindingColumns {
	if columns == nil || columns.Len() == 0 || keep == nil {
		return scanner.FindingColumns{}
	}
	return columns.FilterRows(func(row int) bool {
		return keep(columns, row)
	})
}

// severityLevel maps severity strings to numeric levels (lower = more severe).
func severityLevel(sev string) int {
	switch strings.ToLower(sev) {
	case "error":
		return 1
	case "warning":
		return 2
	case "info":
		return 3
	default:
		return 4
	}
}

// findingsToResult converts findings to a ToolResult with JSON text.
func findingsToResult(findings []scanner.Finding) ToolResult {
	columns := scanner.CollectFindings(findings)
	return findingsToResultColumns(&columns)
}

func findingsToResultColumns(columns *scanner.FindingColumns) ToolResult {
	type findingJSON struct {
		File     string `json:"file"`
		Line     int    `json:"line"`
		Col      int    `json:"col"`
		Rule     string `json:"rule"`
		RuleSet  string `json:"ruleSet"`
		Severity string `json:"severity"`
		Message  string `json:"message"`
		Fixable  bool   `json:"fixable"`
	}

	items := make([]findingJSON, 0, columns.Len())
	for row := 0; row < columns.Len(); row++ {
		items = append(items, findingJSON{
			File:     columns.FileAt(row),
			Line:     columns.LineAt(row),
			Col:      columns.ColumnAt(row),
			Rule:     columns.RuleAt(row),
			RuleSet:  columns.RuleSetAt(row),
			Severity: columns.SeverityAt(row),
			Message:  columns.MessageAt(row),
			Fixable:  columns.HasFix(row),
		})
	}

	data, _ := json.MarshalIndent(items, "", "  ")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(data)}},
	}
}

// fixableToResult converts fixable findings to a ToolResult with fix details.
func fixableToResult(findings []scanner.Finding) ToolResult {
	columns := scanner.CollectFindings(findings)
	return fixableToResultColumns(&columns)
}

func fixableToResultColumns(columns *scanner.FindingColumns) ToolResult {
	type fixJSON struct {
		File        string `json:"file"`
		Line        int    `json:"line"`
		Rule        string `json:"rule"`
		Severity    string `json:"severity"`
		Message     string `json:"message"`
		FixLevel    string `json:"fixLevel"`
		Replacement string `json:"replacement"`
	}

	items := make([]fixJSON, 0, columns.Len())
	for row := 0; row < columns.Len(); row++ {
		if !columns.HasFix(row) {
			continue
		}
		level := "semantic"
		ruleName := columns.RuleAt(row)
		if r := findRule(ruleName); r != nil {
			level = rules.GetFixLevel(r).String()
		}
		fix := columns.FixAt(row)
		items = append(items, fixJSON{
			File:        columns.FileAt(row),
			Line:        columns.LineAt(row),
			Rule:        ruleName,
			Severity:    columns.SeverityAt(row),
			Message:     columns.MessageAt(row),
			FixLevel:    level,
			Replacement: fix.Replacement,
		})
	}

	if len(items) == 0 {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "No auto-fixes available."}},
		}
	}

	data, _ := json.MarshalIndent(items, "", "  ")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(data)}},
	}
}

// inspectTypesArgs are the arguments for the inspect_types tool.
type inspectTypesArgs struct {
	Code  string `json:"code"`
	Path  string `json:"path"`
	Query string `json:"query"`
}

// toolInspectTypes parses Kotlin code, runs type inference, and returns requested type info.
func (s *Server) toolInspectTypes(arguments json.RawMessage) ToolResult {
	var args inspectTypesArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if args.Code == "" {
		return errorResult("'code' argument is required")
	}
	if args.Query == "" {
		return errorResult("'query' argument is required")
	}

	path := args.Path
	if path == "" {
		path = "input.kt"
	}

	file, err := parseKotlinCode(args.Code, path)
	if err != nil {
		return errorResult(err.Error())
	}

	info := typeinfer.IndexFileParallel(file)
	if info == nil {
		return errorResult("type inference returned no results")
	}

	var result interface{}

	switch args.Query {
	case "classes":
		type classJSON struct {
			Name       string   `json:"name"`
			FQN        string   `json:"fqn,omitempty"`
			Kind       string   `json:"kind"`
			Supertypes []string `json:"supertypes,omitempty"`
			IsSealed   bool     `json:"isSealed,omitempty"`
			IsData     bool     `json:"isData,omitempty"`
			IsAbstract bool     `json:"isAbstract,omitempty"`
			IsOpen     bool     `json:"isOpen,omitempty"`
		}
		var classes []classJSON
		for _, ci := range info.Classes {
			classes = append(classes, classJSON{
				Name:       ci.Name,
				FQN:        ci.FQN,
				Kind:       ci.Kind,
				Supertypes: ci.Supertypes,
				IsSealed:   ci.IsSealed,
				IsData:     ci.IsData,
				IsAbstract: ci.IsAbstract,
				IsOpen:     ci.IsOpen,
			})
		}
		if classes == nil {
			classes = []classJSON{}
		}
		result = classes

	case "hierarchy":
		type hierarchyEntry struct {
			Name       string   `json:"name"`
			Supertypes []string `json:"supertypes"`
		}
		var entries []hierarchyEntry
		for _, ci := range info.Classes {
			entries = append(entries, hierarchyEntry{
				Name:       ci.Name,
				Supertypes: ci.Supertypes,
			})
		}
		if entries == nil {
			entries = []hierarchyEntry{}
		}
		result = entries

	case "imports":
		type importsJSON struct {
			Explicit map[string]string `json:"explicit"`
			Wildcard []string          `json:"wildcard"`
			Aliases  map[string]string `json:"aliases,omitempty"`
		}
		imp := importsJSON{
			Explicit: make(map[string]string),
			Wildcard: []string{},
		}
		if info.ImportTable != nil {
			if info.ImportTable.Explicit != nil {
				imp.Explicit = info.ImportTable.Explicit
			}
			if info.ImportTable.Wildcard != nil {
				imp.Wildcard = info.ImportTable.Wildcard
			}
			if len(info.ImportTable.Aliases) > 0 {
				imp.Aliases = info.ImportTable.Aliases
			}
		}
		result = imp

	case "function_signatures":
		type funcJSON struct {
			Name       string `json:"name"`
			ReturnType string `json:"returnType,omitempty"`
		}
		var funcs []funcJSON
		for name, rt := range info.Functions {
			retType := ""
			if rt != nil {
				retType = rt.Name
			}
			funcs = append(funcs, funcJSON{
				Name:       name,
				ReturnType: retType,
			})
		}
		if funcs == nil {
			funcs = []funcJSON{}
		}
		result = funcs

	case "sealed_variants":
		if info.SealedSubs == nil {
			result = map[string][]string{}
		} else {
			result = info.SealedSubs
		}

	case "enum_entries":
		if info.EnumEntries == nil {
			result = map[string][]string{}
		} else {
			result = info.EnumEntries
		}

	default:
		return errorResult("unknown query type: " + args.Query + "; valid: classes, hierarchy, imports, function_signatures, sealed_variants, enum_entries")
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(data)}},
	}
}

// refMatch represents a single reference match in a file.
type refMatch struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// findReferencesArgs are the arguments for the find_references tool.
type findReferencesArgs struct {
	Name         string   `json:"name"`
	ProjectPaths []string `json:"project_paths"`
	IncludeJava  bool     `json:"include_java"`
	IncludeXML   bool     `json:"include_xml"`
}

// toolFindReferences searches for symbol references across project files.
func (s *Server) toolFindReferences(arguments json.RawMessage) ToolResult {
	var args findReferencesArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if args.Name == "" {
		return errorResult("'name' argument is required")
	}
	if len(args.ProjectPaths) == 0 {
		return errorResult("'project_paths' argument is required")
	}

	var refs []refMatch

	// Collect file paths
	var filePaths []string
	for _, p := range args.ProjectPaths {
		ktFiles, err := scanner.CollectKotlinFiles([]string{p}, nil)
		if err != nil {
			return errorResult("collecting Kotlin files: " + err.Error())
		}
		filePaths = append(filePaths, ktFiles...)

		if args.IncludeJava {
			javaFiles, err := scanner.CollectJavaFiles([]string{p}, nil)
			if err != nil {
				return errorResult("collecting Java files: " + err.Error())
			}
			filePaths = append(filePaths, javaFiles...)
		}

		if args.IncludeXML {
			xmlFiles, err := collectXMLFiles(p)
			if err != nil {
				return errorResult("collecting XML files: " + err.Error())
			}
			filePaths = append(filePaths, xmlFiles...)
		}
	}

	// Search each file for the symbol name
	for _, fp := range filePaths {
		matches, err := searchFileForSymbol(fp, args.Name)
		if err != nil {
			continue // skip unreadable files
		}
		refs = append(refs, matches...)
	}

	if refs == nil {
		refs = []refMatch{}
	}

	data, _ := json.MarshalIndent(refs, "", "  ")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(data)}},
	}
}

// collectXMLFiles finds all .xml files under a directory path.
func collectXMLFiles(root string) ([]string, error) {
	var files []string
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(root, ".xml") {
			return []string{root}, nil
		}
		return nil, nil
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "build" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".xml") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// searchFileForSymbol searches a file for lines containing the symbol name.
func searchFileForSymbol(path, name string) ([]refMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []refMatch

	sc := bufio.NewScanner(f)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Text()
		if strings.Contains(line, name) {
			results = append(results, refMatch{
				File: path,
				Line: lineNum,
				Text: strings.TrimSpace(line),
			})
		}
	}
	return results, sc.Err()
}

// analyzeProjectArgs are the arguments for the analyze_project tool.
type analyzeProjectArgs struct {
	Paths  []string `json:"paths"`
	Config string   `json:"config"`
	Format string   `json:"format"`
}

// toolAnalyzeProject runs a full project analysis and returns a summary.
func (s *Server) toolAnalyzeProject(arguments json.RawMessage) ToolResult {
	var args analyzeProjectArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if len(args.Paths) == 0 {
		return errorResult("'paths' argument is required")
	}

	format := args.Format
	if format == "" {
		format = "summary"
	}

	// Collect all Kotlin files
	ktFiles, err := scanner.CollectKotlinFiles(args.Paths, nil)
	if err != nil {
		return errorResult("collecting files: " + err.Error())
	}

	if len(ktFiles) == 0 {
		return errorResult("no Kotlin files found in the specified paths")
	}

	// Parse all files
	files, parseErrs := scanner.ScanFiles(ktFiles, 4)
	_ = parseErrs // ignore individual parse errors

	// Run dispatcher on each file
	type fileResult struct {
		File     string `json:"file"`
		Findings int    `json:"findings"`
	}

	collector := scanner.NewFindingCollector(len(files) * 8)
	var perFile []fileResult

	for _, f := range files {
		fileColumns, _ := s.analyzer.Dispatcher.RunColumnsWithStats(f)
		collector.AppendColumns(&fileColumns)
		if format == "detailed" || fileColumns.Len() > 0 {
			perFile = append(perFile, fileResult{
				File:     f.Path,
				Findings: fileColumns.Len(),
			})
		}
	}
	allColumns := collector.Columns()

	// Count findings per rule
	ruleCounts := make(map[string]int)
	for row := 0; row < allColumns.Len(); row++ {
		ruleCounts[allColumns.RuleAt(row)]++
	}

	// Build top rules (sorted by count, top 10)
	type ruleCount struct {
		Rule  string `json:"rule"`
		Count int    `json:"count"`
	}
	var topRules []ruleCount
	for rule, count := range ruleCounts {
		topRules = append(topRules, ruleCount{Rule: rule, Count: count})
	}
	// Simple insertion sort for small lists
	for i := 1; i < len(topRules); i++ {
		for j := i; j > 0 && topRules[j].Count > topRules[j-1].Count; j-- {
			topRules[j], topRules[j-1] = topRules[j-1], topRules[j]
		}
	}
	if len(topRules) > 10 {
		topRules = topRules[:10]
	}

	type summaryJSON struct {
		TotalFiles    int          `json:"totalFiles"`
		TotalFindings int          `json:"totalFindings"`
		TopRules      []ruleCount  `json:"topRules"`
		PerFile       []fileResult `json:"perFile,omitempty"`
	}

	summary := summaryJSON{
		TotalFiles:    len(files),
		TotalFindings: allColumns.Len(),
		TopRules:      topRules,
	}

	if format == "detailed" {
		summary.PerFile = perFile
	}

	if summary.TopRules == nil {
		summary.TopRules = []ruleCount{}
	}

	data, _ := json.MarshalIndent(summary, "", "  ")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(data)}},
	}
}

// analyzeAndroidArgs are the arguments for the analyze_android tool.
type analyzeAndroidArgs struct {
	ProjectPath string `json:"project_path"`
	Scope       string `json:"scope"`
}

// toolAnalyzeAndroid analyzes Android project files based on scope.
func (s *Server) toolAnalyzeAndroid(arguments json.RawMessage) ToolResult {
	var args analyzeAndroidArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if args.ProjectPath == "" {
		return errorResult("'project_path' argument is required")
	}

	scope := args.Scope
	if scope == "" {
		scope = "all"
	}

	// Validate the path exists
	info, err := os.Stat(args.ProjectPath)
	if err != nil {
		return errorResult("cannot access project path: " + err.Error())
	}
	if !info.IsDir() {
		return errorResult("project_path must be a directory")
	}

	// Detect Android project files
	proj := android.DetectAndroidProject([]string{args.ProjectPath})
	if proj.IsEmpty() {
		return errorResult("no Android project files found in " + args.ProjectPath)
	}

	collector := scanner.NewFindingCollector(len(proj.ManifestPaths)*4 + len(proj.ResDirs)*8 + len(proj.GradlePaths)*4)

	// Manifest analysis
	if scope == "manifest" || scope == "all" {
		for _, manifestPath := range proj.ManifestPaths {
			manifest, err := android.ParseManifest(manifestPath)
			if err != nil {
				continue
			}
			// Convert to rules.Manifest and run ManifestRules
			rManifest := convertManifest(manifestPath, manifest)
			for _, rule := range rules.ManifestRules {
				collector.AppendAll(rule.CheckManifest(rManifest))
			}
		}
	}

	// Resource analysis
	if scope == "resources" || scope == "all" {
		for _, resDir := range proj.ResDirs {
			idx, err := android.ScanResourceDir(resDir)
			if err != nil {
				continue
			}
			for _, rule := range rules.ResourceRules {
				collector.AppendAll(rule.CheckResources(idx))
			}
		}
	}

	// Gradle analysis
	if scope == "gradle" || scope == "all" {
		for _, gradlePath := range proj.GradlePaths {
			content, err := os.ReadFile(gradlePath)
			if err != nil {
				continue
			}
			cfg, err := android.ParseBuildGradleContent(string(content))
			if err != nil {
				continue
			}
			for _, rule := range rules.GradleRules {
				collector.AppendAll(rule.CheckGradle(gradlePath, string(content), cfg))
			}
		}
	}

	// Icon analysis
	if scope == "icons" || scope == "all" {
		for _, resDir := range proj.ResDirs {
			idx, err := android.ScanIconDirs(resDir)
			if err != nil {
				continue
			}
			collector.AppendAll(rules.RunAllIconChecks(idx))
		}
	}

	// Build result summary
	type findingCompact struct {
		File     string `json:"file"`
		Line     int    `json:"line"`
		Rule     string `json:"rule"`
		Severity string `json:"severity"`
		Message  string `json:"message"`
	}

	type androidResult struct {
		ProjectPath   string           `json:"projectPath"`
		Scope         string           `json:"scope"`
		ManifestCount int              `json:"manifestCount"`
		ResDirCount   int              `json:"resDirCount"`
		GradleCount   int              `json:"gradleCount"`
		TotalFindings int              `json:"totalFindings"`
		Findings      []findingCompact `json:"findings"`
	}

	allColumns := collector.Columns()
	compact := make([]findingCompact, 0, allColumns.Len())
	for row := 0; row < allColumns.Len(); row++ {
		compact = append(compact, findingCompact{
			File:     allColumns.FileAt(row),
			Line:     allColumns.LineAt(row),
			Rule:     allColumns.RuleAt(row),
			Severity: allColumns.SeverityAt(row),
			Message:  allColumns.MessageAt(row),
		})
	}

	result := androidResult{
		ProjectPath:   args.ProjectPath,
		Scope:         scope,
		ManifestCount: len(proj.ManifestPaths),
		ResDirCount:   len(proj.ResDirs),
		GradleCount:   len(proj.GradlePaths),
		TotalFindings: allColumns.Len(),
		Findings:      compact,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(data)}},
	}
}

// convertManifest converts an android.Manifest to a rules.Manifest.
func convertManifest(path string, m *android.Manifest) *rules.Manifest {
	rm := &rules.Manifest{
		Path:    path,
		Package: m.Package,
	}

	// Parse version attributes
	rm.VersionCode = m.VersionCode
	rm.VersionName = m.VersionName

	// Parse SDK versions
	if m.UsesSdk.MinSdkVersion != "" {
		fmt.Sscanf(m.UsesSdk.MinSdkVersion, "%d", &rm.MinSDK)
	}
	if m.UsesSdk.TargetSdkVersion != "" {
		fmt.Sscanf(m.UsesSdk.TargetSdkVersion, "%d", &rm.TargetSDK)
	}

	// Uses-sdk element
	if m.UsesSdk.MinSdkVersion != "" || m.UsesSdk.TargetSdkVersion != "" {
		rm.UsesSdk = &rules.ManifestElement{Tag: "uses-sdk", Line: 1}
	}

	// Permissions
	for _, p := range m.UsesPermissions {
		rm.UsesPermissions = append(rm.UsesPermissions, p.Name)
	}
	for _, p := range m.Permissions {
		rm.Permissions = append(rm.Permissions, p.Name)
	}

	// Uses-features
	for _, f := range m.UsesFeatures {
		rm.UsesFeatures = append(rm.UsesFeatures, rules.ManifestUsesFeature{
			Name:     f.Name,
			Required: f.Required,
		})
	}

	// Application
	if m.Application.Name != "" || len(m.Application.Activities) > 0 || len(m.Application.Services) > 0 {
		app := &rules.ManifestApplication{
			Line:                  1,
			Icon:                  m.Application.Icon,
			NetworkSecurityConfig: m.Application.NetworkSecurityConfig,
			FullBackupContent:     m.Application.FullBackupContent,
			DataExtractionRules:   m.Application.DataExtractionRules,
		}
		if m.Application.AllowBackup != "" {
			v := m.Application.AllowBackup == "true"
			app.AllowBackup = &v
		}
		if m.Application.Debuggable != "" {
			v := m.Application.Debuggable == "true"
			app.Debuggable = &v
		}
		if m.Application.UsesCleartextTraffic != "" {
			v := m.Application.UsesCleartextTraffic == "true"
			app.UsesCleartextTraffic = &v
		}

		// Convert components
		for _, a := range m.Application.Activities {
			comp := rules.ManifestComponent{
				Tag:             "activity",
				Name:            a.Name,
				Line:            1,
				HasIntentFilter: len(a.IntentFilters) > 0,
				Permission:      a.Permission,
			}
			if a.Exported != "" {
				v := a.Exported == "true"
				comp.Exported = &v
			}
			for _, f := range a.IntentFilters {
				for _, action := range f.Actions {
					comp.IntentFilterActions = append(comp.IntentFilterActions, action.Name)
				}
				for _, cat := range f.Categories {
					comp.IntentFilterCategories = append(comp.IntentFilterCategories, cat.Name)
				}
			}
			app.Activities = append(app.Activities, comp)
		}
		for _, svc := range m.Application.Services {
			comp := rules.ManifestComponent{
				Tag:             "service",
				Name:            svc.Name,
				Line:            1,
				HasIntentFilter: len(svc.IntentFilters) > 0,
				Permission:      svc.Permission,
			}
			if svc.Exported != "" {
				v := svc.Exported == "true"
				comp.Exported = &v
			}
			app.Services = append(app.Services, comp)
		}
		for _, r := range m.Application.Receivers {
			comp := rules.ManifestComponent{
				Tag:             "receiver",
				Name:            r.Name,
				Line:            1,
				HasIntentFilter: len(r.IntentFilters) > 0,
				Permission:      r.Permission,
			}
			if r.Exported != "" {
				v := r.Exported == "true"
				comp.Exported = &v
			}
			app.Receivers = append(app.Receivers, comp)
		}
		for _, p := range m.Application.Providers {
			comp := rules.ManifestComponent{
				Tag:        "provider",
				Name:       p.Name,
				Line:       1,
				Permission: p.Permission,
			}
			if p.Exported != "" {
				v := p.Exported == "true"
				comp.Exported = &v
			}
			app.Providers = append(app.Providers, comp)
		}
		rm.Application = app
	}

	// Elements
	for _, e := range m.Elements {
		rm.Elements = append(rm.Elements, rules.ManifestElement{
			Tag:       e.Tag,
			Line:      e.Line,
			ParentTag: e.ParentTag,
		})
	}

	return rm
}

// inspectModulesArgs are the arguments for the inspect_modules tool.
type inspectModulesArgs struct {
	ProjectRoot string `json:"project_root"`
	Module      string `json:"module"`
}

// toolInspectModules discovers Gradle module structure.
func (s *Server) toolInspectModules(arguments json.RawMessage) ToolResult {
	var args inspectModulesArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if args.ProjectRoot == "" {
		return errorResult("'project_root' argument is required")
	}

	info, err := os.Stat(args.ProjectRoot)
	if err != nil {
		return errorResult("cannot access project root: " + err.Error())
	}
	if !info.IsDir() {
		return errorResult("project_root must be a directory")
	}

	graph, err := module.DiscoverModules(args.ProjectRoot)
	if err != nil {
		return errorResult("discovering modules: " + err.Error())
	}
	if graph == nil {
		return errorResult("no settings.gradle.kts or settings.gradle found in " + args.ProjectRoot)
	}

	type moduleJSON struct {
		Path        string   `json:"path"`
		Dir         string   `json:"dir"`
		SourceRoots []string `json:"sourceRoots,omitempty"`
		DependsOn   []string `json:"dependsOn,omitempty"`
	}

	// If a specific module is requested, filter to it
	if args.Module != "" {
		modPath := args.Module
		if !strings.HasPrefix(modPath, ":") {
			modPath = ":" + modPath
		}
		m, ok := graph.Modules[modPath]
		if !ok {
			return errorResult("module not found: " + args.Module)
		}
		var deps []string
		for _, d := range m.Dependencies {
			deps = append(deps, d.ModulePath)
		}
		result := moduleJSON{
			Path:        m.Path,
			Dir:         m.Dir,
			SourceRoots: m.SourceRoots,
			DependsOn:   deps,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: string(data)}},
		}
	}

	// Return all modules
	type graphJSON struct {
		RootDir     string       `json:"rootDir"`
		ModuleCount int          `json:"moduleCount"`
		Modules     []moduleJSON `json:"modules"`
	}

	modules := make([]moduleJSON, 0, len(graph.Modules))
	for _, m := range graph.Modules {
		var deps []string
		for _, d := range m.Dependencies {
			deps = append(deps, d.ModulePath)
		}
		modules = append(modules, moduleJSON{
			Path:        m.Path,
			Dir:         m.Dir,
			SourceRoots: m.SourceRoots,
			DependsOn:   deps,
		})
	}

	result := graphJSON{
		RootDir:     graph.RootDir,
		ModuleCount: len(graph.Modules),
		Modules:     modules,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(data)}},
	}
}

// errorResult creates an error ToolResult.
func errorResult(msg string) ToolResult {
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %s", msg)}},
		IsError: true,
	}
}
