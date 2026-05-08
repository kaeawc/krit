package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// fixArgs are the arguments for the fix tool.
type fixArgs struct {
	Operation string `json:"operation"`

	// operation=suggest
	Code     string `json:"code"`
	Path     string `json:"path"`
	FixLevel string `json:"fix_level"`

	// operation=suppress
	Rule  string `json:"rule"`
	Line  int    `json:"line"`
	Scope string `json:"scope"`
}

func (s *Server) toolFix(arguments json.RawMessage) ToolResult {
	var args fixArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	op := args.Operation
	if op == "" {
		op = opFixSuggest
	}

	switch op {
	case opFixSuggest:
		return s.fixSuggest(args)
	case opFixSuppress:
		return s.fixSuppress(args)
	default:
		return errorResult("unknown operation: " + op + "; valid: " + formatList(fixOperations))
	}
}

// fixSuggest returns fixable findings filtered by safety level.
func (s *Server) fixSuggest(args fixArgs) ToolResult {
	if args.Code == "" {
		return errorResult("'code' argument is required for operation=suggest")
	}

	columns, err := s.parseAndAnalyzeColumns(args.Code, args.Path)
	if err != nil {
		return errorResult(err.Error())
	}

	columns = filterFindingColumns(&columns, func(columns *scanner.FindingColumns, row int) bool {
		return columns.HasFix(row)
	})

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
			lvl, _ := rules.GetV2FixLevel(r)
			return lvl <= maxLevel
		})
	}

	return fixableToResultColumns(&columns)
}

// fixSuppress emits the @Suppress annotation text and the line on which to
// insert it. Does not modify any file — the agent applies the change.
func (s *Server) fixSuppress(args fixArgs) ToolResult {
	if args.Rule == "" {
		return errorResult("'rule' argument is required for operation=suppress")
	}
	if findRule(args.Rule) == nil {
		return errorResult("unknown rule: " + args.Rule)
	}

	scope := args.Scope
	if scope == "" {
		scope = "declaration"
	}

	annotation := fmt.Sprintf(`@Suppress("%s")`, args.Rule)

	type suppressionResult struct {
		Rule       string `json:"rule"`
		Annotation string `json:"annotation"`
		Scope      string `json:"scope"`
		Line       int    `json:"line"`
		Note       string `json:"note"`
	}

	note := "Insert the annotation on the declaration immediately containing the finding."
	if scope == "file" {
		note = "Insert `@file:Suppress(\"...\")` at the top of the file, above any package or import statements."
		annotation = fmt.Sprintf(`@file:Suppress("%s")`, args.Rule)
	}

	return jsonResult(suppressionResult{
		Rule:       args.Rule,
		Annotation: annotation,
		Scope:      scope,
		Line:       args.Line,
		Note:       note,
	})
}
