package mcp

import (
	"context"
	"encoding/json"
	"os"
	"sort"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// analyzeArgs are the arguments for the analyze tool. Mode dispatches among
// code (single buffer), project (path-based aggregate), android (Android
// project files), and impact (rule noise count).
type analyzeArgs struct {
	Mode string `json:"mode"`

	// Common filters
	Rules    []string `json:"rules"`
	Severity string   `json:"severity"`

	// mode=code/impact (single buffer)
	Code string `json:"code"`
	Path string `json:"path"`

	// mode=project
	Paths  []string `json:"paths"`
	Config string   `json:"config"`
	Format string   `json:"format"`

	// mode=android
	ProjectPath string `json:"project_path"`
	Scope       string `json:"scope"`
}

func (s *Server) toolAnalyze(arguments json.RawMessage) ToolResult {
	var args analyzeArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	mode := args.Mode
	if mode == "" {
		mode = modeCode
	}

	switch mode {
	case modeCode:
		return s.analyzeCode(args)
	case modeProject:
		return s.analyzeProject(args)
	case modeAndroid:
		return s.analyzeAndroid(args)
	case modeImpact:
		return s.analyzeImpact(args)
	default:
		return errorResult("unknown mode: " + mode + "; valid: " + formatList(analyzeModes))
	}
}

// analyzeCode runs rules against a single in-memory buffer.
func (s *Server) analyzeCode(args analyzeArgs) ToolResult {
	if args.Code == "" {
		return errorResult("'code' argument is required for mode=code")
	}

	columns, err := s.parseAndAnalyzeColumns(args.Code, args.Path)
	if err != nil {
		return errorResult(err.Error())
	}

	if len(args.Rules) > 0 {
		ruleSet := make(map[string]bool, len(args.Rules))
		for _, r := range args.Rules {
			ruleSet[r] = true
		}
		columns = filterFindingColumns(&columns, func(columns *scanner.FindingColumns, row int) bool {
			return ruleSet[columns.RuleAt(row)]
		})
	}

	if args.Severity != "" {
		minLevel := severityLevel(args.Severity)
		columns = filterFindingColumns(&columns, func(columns *scanner.FindingColumns, row int) bool {
			return severityLevel(columns.SeverityAt(row)) <= minLevel
		})
	}

	return findingsToResultColumns(&columns)
}

// analyzeProject runs rules against multiple paths and returns a summary.
func (s *Server) analyzeProject(args analyzeArgs) ToolResult {
	if len(args.Paths) == 0 {
		return errorResult("'paths' argument is required for mode=project")
	}

	format := args.Format
	if format == "" {
		format = "summary"
	}

	ktFiles, err := scanner.CollectKotlinFiles(args.Paths, nil)
	if err != nil {
		return errorResult("collecting files: " + err.Error())
	}

	if len(ktFiles) == 0 {
		return errorResult("no Kotlin files found in the specified paths")
	}

	files, _ := scanner.ScanFiles(context.Background(), ktFiles, 4)

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

	ruleCounts := make(map[string]int)
	for row := 0; row < allColumns.Len(); row++ {
		ruleCounts[allColumns.RuleAt(row)]++
	}

	type ruleCount struct {
		Rule  string `json:"rule"`
		Count int    `json:"count"`
	}
	topRules := make([]ruleCount, 0, len(ruleCounts))
	for rule, count := range ruleCounts {
		topRules = append(topRules, ruleCount{Rule: rule, Count: count})
	}
	sort.SliceStable(topRules, func(i, j int) bool {
		if topRules[i].Count != topRules[j].Count {
			return topRules[i].Count > topRules[j].Count
		}
		return topRules[i].Rule < topRules[j].Rule
	})
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

	return jsonResult(summary)
}

func collectManifestFindings(dispatcher *rules.Dispatcher, collector *scanner.FindingCollector, manifestPaths []string) {
	for _, manifestPath := range manifestPaths {
		parsed, err := android.ParseManifest(manifestPath)
		if err != nil {
			continue
		}
		rManifest := pipeline.ConvertManifestForRules(android.ConvertManifest(parsed, manifestPath))
		file := &scanner.File{
			Path:     manifestPath,
			Language: scanner.LangXML,
			Metadata: rManifest,
		}
		cols := dispatcher.RunManifest(file, rManifest)
		collector.AppendColumns(&cols)
	}
}

func collectResourceFindings(dispatcher *rules.Dispatcher, collector *scanner.FindingCollector, resDirs []string) {
	for _, resDir := range resDirs {
		idx, err := android.ScanResourceDir(resDir)
		if err != nil {
			continue
		}
		file := &scanner.File{
			Path:     resDir,
			Language: scanner.LangXML,
			Metadata: idx,
		}
		cols := dispatcher.RunResource(file, idx)
		collector.AppendColumns(&cols)
	}
}

func collectGradleFindings(dispatcher *rules.Dispatcher, collector *scanner.FindingCollector, gradlePaths []string) {
	for _, gradlePath := range gradlePaths {
		content, err := os.ReadFile(gradlePath)
		if err != nil {
			continue
		}
		cfg, err := android.ParseBuildGradleContent(string(content))
		if err != nil {
			continue
		}
		file := &scanner.File{
			Path:     gradlePath,
			Language: scanner.LangGradle,
			Content:  content,
			Metadata: cfg,
		}
		cols := dispatcher.RunGradle(file, cfg)
		collector.AppendColumns(&cols)
	}
}

func collectIconFindings(dispatcher *rules.Dispatcher, collector *scanner.FindingCollector, resDirs []string) {
	for _, resDir := range resDirs {
		idx, err := android.ScanIconDirs(resDir)
		if err != nil {
			continue
		}
		file := &scanner.File{
			Path:     resDir,
			Language: scanner.LangXML,
			Metadata: idx,
		}
		cols := dispatcher.RunIcons(file, idx)
		collector.AppendColumns(&cols)
	}
}

// analyzeAndroid runs Android-specific rules against an Android project.
func (s *Server) analyzeAndroid(args analyzeArgs) ToolResult {
	if args.ProjectPath == "" {
		return errorResult("'project_path' argument is required for mode=android")
	}

	scope := args.Scope
	if scope == "" {
		scope = "all"
	}

	info, err := os.Stat(args.ProjectPath)
	if err != nil {
		return errorResult("cannot access project path: " + err.Error())
	}
	if !info.IsDir() {
		return errorResult("project_path must be a directory")
	}

	proj := android.DetectProject([]string{args.ProjectPath})
	if proj.IsEmpty() {
		return errorResult("no Android project files found in " + args.ProjectPath)
	}

	collector := scanner.NewFindingCollector(len(proj.ManifestPaths)*4 + len(proj.ResDirs)*8 + len(proj.GradlePaths)*4)

	libraryFacts := librarymodel.FactsForProfile(librarymodel.ProfileFromGradlePaths(proj.GradlePaths))
	dispatcher := rules.NewDispatcher(api.Registry)
	dispatcher.SetLibraryFacts(libraryFacts)

	if scope == "manifest" || scope == "all" {
		collectManifestFindings(dispatcher, collector, proj.ManifestPaths)
	}
	if scope == "resources" || scope == "all" {
		collectResourceFindings(dispatcher, collector, proj.ResDirs)
	}
	if scope == "gradle" || scope == "all" {
		collectGradleFindings(dispatcher, collector, proj.GradlePaths)
	}
	if scope == "icons" || scope == "all" {
		collectIconFindings(dispatcher, collector, proj.ResDirs)
	}

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

	return jsonResult(result)
}

// analyzeImpact counts findings per rule for either a buffer or a path set.
// Useful for "how loud would enabling rule X be?" before flipping config.
func (s *Server) analyzeImpact(args analyzeArgs) ToolResult {
	var allColumns scanner.FindingColumns

	switch {
	case args.Code != "":
		cols, err := s.parseAndAnalyzeColumns(args.Code, args.Path)
		if err != nil {
			return errorResult(err.Error())
		}
		allColumns = cols

	case len(args.Paths) > 0:
		ktFiles, err := scanner.CollectKotlinFiles(args.Paths, nil)
		if err != nil {
			return errorResult("collecting files: " + err.Error())
		}
		if len(ktFiles) == 0 {
			return errorResult("no Kotlin files found in the specified paths")
		}
		files, _ := scanner.ScanFiles(context.Background(), ktFiles, 4)
		collector := scanner.NewFindingCollector(len(files) * 8)
		for _, f := range files {
			cols, _ := s.analyzer.Dispatcher.RunColumnsWithStats(f)
			collector.AppendColumns(&cols)
		}
		allColumns = *collector.Columns()

	default:
		return errorResult("mode=impact requires either 'code' or 'paths'")
	}

	// Optional rule filter
	ruleSet := map[string]bool{}
	for _, r := range args.Rules {
		ruleSet[r] = true
	}

	counts := make(map[string]int)
	files := make(map[string]map[string]struct{})
	for row := 0; row < allColumns.Len(); row++ {
		rule := allColumns.RuleAt(row)
		if len(ruleSet) > 0 && !ruleSet[rule] {
			continue
		}
		counts[rule]++
		if files[rule] == nil {
			files[rule] = map[string]struct{}{}
		}
		files[rule][allColumns.FileAt(row)] = struct{}{}
	}

	type impactRow struct {
		Rule          string `json:"rule"`
		Findings      int    `json:"findings"`
		FilesAffected int    `json:"filesAffected"`
		Active        bool   `json:"active"`
	}

	rows := make([]impactRow, 0, len(counts))
	for rule, count := range counts {
		rows = append(rows, impactRow{
			Rule:          rule,
			Findings:      count,
			FilesAffected: len(files[rule]),
			Active:        rules.IsDefaultActive(rule),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Findings != rows[j].Findings {
			return rows[i].Findings > rows[j].Findings
		}
		return rows[i].Rule < rows[j].Rule
	})

	type impactResult struct {
		TotalFindings int         `json:"totalFindings"`
		Rules         []impactRow `json:"rules"`
	}

	return jsonResult(impactResult{
		TotalFindings: allColumns.Len(),
		Rules:         rows,
	})
}
