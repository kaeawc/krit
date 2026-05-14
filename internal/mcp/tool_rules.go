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
	Query     string   `json:"query"`
	Category  string   `json:"category"`
	Precision string   `json:"precision"`
	Needs     []string `json:"needs"`
	Without   []string `json:"without"`

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

	desc, _ := rules.MetaForRule(r)

	info := map[string]interface{}{
		"name":         r.ID,
		"description":  r.Description,
		"ruleSet":      r.Category,
		"severity":     string(r.Sev),
		"active":       active,
		"fixable":      fixable,
		"precision":    rules.V2RulePrecision(r).String(),
		"cost":         rules.CostFor(r).String(),
		"capabilities": r.CapabilitiesList(),
		"owners":       desc.Owners,
		"maintainedBy": "Maintained by " + strings.Join(desc.Owners, ", "),
	}
	if fixLevel != "" {
		info["fixLevel"] = fixLevel
	}
	if related := resolveRelatedRules(r); len(related) > 0 {
		info["relatedRules"] = related
	}

	return jsonResult(info)
}

// resolveRelatedRules returns the rule IDs listed in r.RelatedRules,
// filtered to those that still resolve in the registry. ValidateRelations
// already rejects dangling references at NewDispatcher time; this filter
// is a defensive guard for callers that introspect the registry without
// having constructed a dispatcher.
func resolveRelatedRules(r *api.Rule) []string {
	if r == nil || len(r.RelatedRules) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.RelatedRules))
	for _, id := range r.RelatedRules {
		if findRule(id) != nil {
			out = append(out, id)
		}
	}
	return out
}

// rulesSearch performs a case-insensitive substring match over rule name,
// description, and category. Results are ranked by where the match landed
// (name > description > category) then alphabetical.
func (s *Server) rulesSearch(args rulesArgs) ToolResult {
	capabilityFilter := api.CapabilityFilter{Require: args.Needs, Exclude: args.Without}
	if args.Query == "" && args.Precision == "" && capabilityFilter.IsZero() {
		return errorResult("'query', 'precision', 'needs', or 'without' argument is required for operation=search")
	}

	if _, unknown := api.ParseCapabilities(args.Needs); len(unknown) > 0 {
		return errorResult("unknown capability label(s) in 'needs': " + strings.Join(unknown, ", "))
	}
	if _, unknown := api.ParseCapabilities(args.Without); len(unknown) > 0 {
		return errorResult("unknown capability label(s) in 'without': " + strings.Join(unknown, ", "))
	}

	q := strings.ToLower(args.Query)
	categoryFilter := strings.ToLower(args.Category)

	var precisionFilter api.Precision
	if args.Precision != "" {
		p, ok := api.ParsePrecision(args.Precision)
		if !ok {
			return errorResult("unknown precision: " + args.Precision +
				"; valid: heuristic/text-backed, ast-backed, project-structure-aware, type-aware, policy")
		}
		precisionFilter = p
	}

	type hit struct {
		Name         string   `json:"name"`
		Category     string   `json:"ruleSet"`
		Severity     string   `json:"severity"`
		Active       bool     `json:"active"`
		Fixable      bool     `json:"fixable"`
		Precision    string   `json:"precision"`
		Cost         string   `json:"cost"`
		Capabilities []string `json:"capabilities"`
		Description  string   `json:"description"`
		Score        int      `json:"-"`
	}

	hits := make([]hit, 0, 32)
	for _, r := range api.Registry {
		if categoryFilter != "" && !strings.EqualFold(r.Category, args.Category) {
			continue
		}

		precision := rules.V2RulePrecision(r)
		if precisionFilter != api.PrecisionUnset && precision != precisionFilter {
			continue
		}

		if !capabilityFilter.MatchRule(r) {
			continue
		}

		score := 0
		nameMatch := strings.Contains(strings.ToLower(r.ID), q)
		descMatch := strings.Contains(strings.ToLower(r.Description), q)
		catMatch := strings.Contains(strings.ToLower(r.Category), q)

		switch {
		case q == "":
			// No query text: any of precision / needs / without acts as the
			// filter. We've already applied those above, so anything that
			// reaches here matches.
			score = 1
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
			Name:         r.ID,
			Category:     r.Category,
			Severity:     string(r.Sev),
			Active:       rules.IsDefaultActive(r.ID),
			Fixable:      fixable,
			Precision:    precision.String(),
			Cost:         rules.CostFor(r).String(),
			Capabilities: r.CapabilitiesList(),
			Description:  r.Description,
			Score:        score,
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

// rulesCategories returns each category with its rule count, plus a
// per-precision-tier breakdown of the registry.
func (s *Server) rulesCategories(_ rulesArgs) ToolResult {
	type bucketRow struct {
		Name      string `json:"name"`
		RuleCount int    `json:"ruleCount"`
		Active    int    `json:"activeByDefault"`
	}

	counts := map[string]int{}
	active := map[string]int{}
	precCounts := map[api.Precision]int{}
	precActive := map[api.Precision]int{}
	for _, r := range api.Registry {
		counts[r.Category]++
		isActive := rules.IsDefaultActive(r.ID)
		if isActive {
			active[r.Category]++
		}
		p := rules.V2RulePrecision(r)
		precCounts[p]++
		if isActive {
			precActive[p]++
		}
	}

	rows := make([]bucketRow, 0, len(counts))
	for name, count := range counts {
		rows = append(rows, bucketRow{
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

	tierOrder := []api.Precision{
		api.PrecisionPolicy,
		api.PrecisionTypeAware,
		api.PrecisionProjectStructure,
		api.PrecisionASTBacked,
		api.PrecisionHeuristicTextBacked,
	}
	precisions := make([]bucketRow, 0, len(tierOrder))
	for _, p := range tierOrder {
		if precCounts[p] == 0 {
			continue
		}
		precisions = append(precisions, bucketRow{
			Name:      p.String(),
			RuleCount: precCounts[p],
			Active:    precActive[p],
		})
	}

	type categoriesResult struct {
		Total      int         `json:"total"`
		Categories []bucketRow `json:"categories"`
		Precisions []bucketRow `json:"precisions"`
	}

	return jsonResult(categoriesResult{
		Total:      len(rows),
		Categories: rows,
		Precisions: precisions,
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
