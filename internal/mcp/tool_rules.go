package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// rulesArgs are the arguments for the rules tool.
type rulesArgs struct {
	Operation string `json:"operation"`

	// operation=explain/configure
	Rule string `json:"rule"`

	// operation=search
	Query    string `json:"query"`
	Category string `json:"category"`

	// operation=configure
	Active   *bool  `json:"active"`
	Severity string `json:"severity"`
}

func (s *Server) toolRules(arguments json.RawMessage) ToolResult {
	var args rulesArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	op := args.Operation
	if op == "" {
		op = opRulesExplain
	}

	switch op {
	case opRulesExplain:
		return s.rulesExplain(args)
	case opRulesSearch:
		return s.rulesSearch(args)
	case opRulesCategories:
		return s.rulesCategories(args)
	case opRulesConfigure:
		return s.rulesConfigure(args)
	default:
		return errorResult("unknown operation: " + op + "; valid: " + formatList(rulesOperations))
	}
}

// rulesExplain returns metadata for one rule.
func (s *Server) rulesExplain(args rulesArgs) ToolResult {
	if args.Rule == "" {
		return errorResult("'rule' argument is required for operation=explain")
	}

	r := findRule(args.Rule)
	if r == nil {
		return errorResult("unknown rule: " + args.Rule)
	}

	fixLvl, fixable := rules.GetV2FixLevel(r)
	fixLevel := ""
	if fixable {
		fixLevel = fixLvl.String()
	}

	active := rules.IsDefaultActive(r.ID)

	info := map[string]interface{}{
		"name":        r.ID,
		"description": r.Description,
		"ruleSet":     r.Category,
		"severity":    string(r.Sev),
		"active":      active,
		"fixable":     fixable,
	}
	if fixLevel != "" {
		info["fixLevel"] = fixLevel
	}

	return jsonResult(info)
}

// rulesSearch performs a case-insensitive substring match over rule name,
// description, and category. Results are ranked by where the match landed
// (name > description > category) then alphabetical.
func (s *Server) rulesSearch(args rulesArgs) ToolResult {
	if args.Query == "" {
		return errorResult("'query' argument is required for operation=search")
	}

	q := strings.ToLower(args.Query)
	categoryFilter := strings.ToLower(args.Category)

	type hit struct {
		Name        string `json:"name"`
		Category    string `json:"ruleSet"`
		Severity    string `json:"severity"`
		Active      bool   `json:"active"`
		Fixable     bool   `json:"fixable"`
		Description string `json:"description"`
		Score       int    `json:"-"`
	}

	hits := make([]hit, 0, 32)
	for _, r := range api.Registry {
		if categoryFilter != "" && !strings.EqualFold(r.Category, args.Category) {
			continue
		}

		score := 0
		nameMatch := strings.Contains(strings.ToLower(r.ID), q)
		descMatch := strings.Contains(strings.ToLower(r.Description), q)
		catMatch := strings.Contains(strings.ToLower(r.Category), q)

		switch {
		case nameMatch:
			score = 3
		case descMatch:
			score = 2
		case catMatch:
			score = 1
		default:
			continue
		}

		_, fixable := rules.GetV2FixLevel(r)
		hits = append(hits, hit{
			Name:        r.ID,
			Category:    r.Category,
			Severity:    string(r.Sev),
			Active:      rules.IsDefaultActive(r.ID),
			Fixable:     fixable,
			Description: r.Description,
			Score:       score,
		})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Name < hits[j].Name
	})

	type searchResult struct {
		Query string `json:"query"`
		Total int    `json:"total"`
		Hits  []hit  `json:"hits"`
	}

	return jsonResult(searchResult{
		Query: args.Query,
		Total: len(hits),
		Hits:  hits,
	})
}

// rulesCategories returns each category with its rule count.
func (s *Server) rulesCategories(_ rulesArgs) ToolResult {
	type categoryRow struct {
		Name      string `json:"name"`
		RuleCount int    `json:"ruleCount"`
		Active    int    `json:"activeByDefault"`
	}

	counts := map[string]int{}
	active := map[string]int{}
	for _, r := range api.Registry {
		counts[r.Category]++
		if rules.IsDefaultActive(r.ID) {
			active[r.Category]++
		}
	}

	rows := make([]categoryRow, 0, len(counts))
	for name, count := range counts {
		rows = append(rows, categoryRow{
			Name:      name,
			RuleCount: count,
			Active:    active[name],
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].RuleCount != rows[j].RuleCount {
			return rows[i].RuleCount > rows[j].RuleCount
		}
		return rows[i].Name < rows[j].Name
	})

	type categoriesResult struct {
		Total      int           `json:"total"`
		Categories []categoryRow `json:"categories"`
	}

	return jsonResult(categoriesResult{
		Total:      len(rows),
		Categories: rows,
	})
}

// rulesConfigure generates a krit.yml YAML stanza with optional overrides.
func (s *Server) rulesConfigure(args rulesArgs) ToolResult {
	if args.Rule == "" {
		return errorResult("'rule' argument is required for operation=configure")
	}

	r := findRule(args.Rule)
	if r == nil {
		return errorResult("unknown rule: " + args.Rule)
	}

	active := rules.IsDefaultActive(r.ID)
	if args.Active != nil {
		active = *args.Active
	}

	severity := string(r.Sev)
	if args.Severity != "" {
		severity = args.Severity
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s:\n", r.Category)
	fmt.Fprintf(&sb, "  %s:\n", r.ID)
	fmt.Fprintf(&sb, "    active: %t\n", active)
	fmt.Fprintf(&sb, "    severity: %s\n", severity)

	type configureResult struct {
		Rule     string `json:"rule"`
		Category string `json:"ruleSet"`
		YAML     string `json:"yaml"`
	}

	return jsonResult(configureResult{
		Rule:     r.ID,
		Category: r.Category,
		YAML:     sb.String(),
	})
}
