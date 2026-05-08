package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// errorResult creates an error ToolResult.
func errorResult(msg string) ToolResult {
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %s", msg)}},
		IsError: true,
	}
}

// jsonResult marshals v as pretty JSON and wraps it in a ToolResult.
func jsonResult(v interface{}) ToolResult {
	data, _ := json.MarshalIndent(v, "", "  ")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(data)}},
	}
}

// ruleByID indexes the rule registry by rule ID. Built once on first access; the
// registry is immutable for the process lifetime so a sync.Once is safe.
// Avoids the O(rules) linear scan in fix.suggest's per-finding hot loop.
var (
	ruleByID     map[string]*api.Rule
	ruleByIDOnce sync.Once
)

func findRule(name string) *api.Rule {
	ruleByIDOnce.Do(func() {
		ruleByID = make(map[string]*api.Rule, len(api.Registry))
		for _, r := range api.Registry {
			ruleByID[r.ID] = r
		}
	})
	return ruleByID[name]
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

// findingsToResultColumns marshals findings into the standard JSON shape.
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

	return jsonResult(items)
}

// fixableToResultColumns marshals fix suggestions for the fix.suggest operation.
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
			if lvl, ok := rules.GetV2FixLevel(r); ok {
				level = lvl.String()
			}
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

	return jsonResult(items)
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
			return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
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

// refMatch represents a single reference match in a file.
type refMatch struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}
