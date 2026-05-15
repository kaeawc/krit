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
	Maturity  string   `json:"maturity"`
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
	meta, _ := rules.MetaForRule(r)

	info := map[string]interface{}{
		"name":         r.ID,
		"description":  r.Description,
		"ruleSet":      r.Category,
		"severity":     string(r.Sev),
		"active":       active,
		"fixable":      fixable,
		"precision":    rules.V2RulePrecision(r).String(),
		"effort":       rules.V2RuleEffort(r).String(),
		"stability":    rules.V2RuleStability(r).String(),
		"maturity":     r.Maturity.String(),
		"noisiness":    rules.V2RuleNoisiness(r).String(),
		"cost":         rules.CostFor(r).String(),
		"capabilities": r.CapabilitiesList(),
		"owners":       meta.Owners,
		"maintainedBy": "Maintained by " + strings.Join(meta.Owners, ", "),
	}
	if docs := api.RuleDocsURL(r); docs != "" {
		info["docsURL"] = docs
	}
	if fixLevel != "" {
		info["fixLevel"] = fixLevel
	}
	if len(meta.KnownLimitations) > 0 {
		info["caveats"] = meta.KnownLimitations
	}
	if related := resolveRelatedRules(r); len(related) > 0 {
		info["relatedRules"] = related
	}
	if meta.IntroducedIn != "" {
		info["introducedIn"] = meta.IntroducedIn
	}
	if meta.EnabledByDefaultSince != "" {
		info["enabledByDefaultSince"] = meta.EnabledByDefaultSince
	}
	if lifecycle := formatLifecycle(meta.IntroducedIn, meta.EnabledByDefaultSince); lifecycle != "" {
		info["lifecycle"] = lifecycle
	}
	if r.Deprecated != nil {
		dep := map[string]interface{}{}
		if r.Deprecated.Since != "" {
			dep["since"] = r.Deprecated.Since
		}
		if r.Deprecated.ReplacedBy != "" {
			dep["replacedBy"] = r.Deprecated.ReplacedBy
		}
		if r.Deprecated.Reason != "" {
			dep["reason"] = r.Deprecated.Reason
		}
		info["deprecation"] = dep
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

// formatLifecycle renders a human-readable summary of when a rule first
// shipped and (optionally) when it became default-active, e.g.
// "Introduced in 0.2.0; default since 0.4.0".
func formatLifecycle(introducedIn, enabledByDefaultSince string) string {
	switch {
	case introducedIn == "" && enabledByDefaultSince == "":
		return ""
	case introducedIn == "":
		return "Default since " + enabledByDefaultSince
	case enabledByDefaultSince == "":
		return "Introduced in " + introducedIn
	default:
		return "Introduced in " + introducedIn + "; default since " + enabledByDefaultSince
	}
}

// parseSearchFilters validates the precision/maturity filter arguments.
// Returns a non-nil ToolResult pointer when validation fails.
func parseSearchFilters(args rulesArgs) (api.Precision, api.Maturity, bool, *ToolResult) {
	var precisionFilter api.Precision
	if args.Precision != "" {
		p, ok := api.ParsePrecision(args.Precision)
		if !ok {
			r := errorResult("unknown precision: " + args.Precision +
				"; valid: heuristic/text-backed, ast-backed, project-structure-aware, type-aware, policy")
			return 0, 0, false, &r
		}
		precisionFilter = p
	}

	var maturityFilter api.Maturity
	maturityFilterSet := false
	if args.Maturity != "" {
		m, ok := api.ParseMaturity(args.Maturity)
		if !ok {
			r := errorResult("unknown maturity: " + args.Maturity + "; valid: stable, experimental, deprecated")
			return 0, 0, false, &r
		}
		maturityFilter = m
		maturityFilterSet = true
	}
	return precisionFilter, maturityFilter, maturityFilterSet, nil
}

// validateCapabilityArgs returns a non-nil ToolResult error when 'needs' or
// 'without' contains an unknown capability label.
func validateCapabilityArgs(needs, without []string) *ToolResult {
	if _, unknown := api.ParseCapabilities(needs); len(unknown) > 0 {
		r := errorResult("unknown capability label(s) in 'needs': " + strings.Join(unknown, ", "))
		return &r
	}
	if _, unknown := api.ParseCapabilities(without); len(unknown) > 0 {
		r := errorResult("unknown capability label(s) in 'without': " + strings.Join(unknown, ", "))
		return &r
	}
	return nil
}

// rulesSearch performs a case-insensitive substring match over rule name,
// description, and category. Results are ranked by where the match landed
// (name > description > category) then alphabetical.
func (s *Server) rulesSearch(args rulesArgs) ToolResult {
	capabilityFilter := api.CapabilityFilter{Require: args.Needs, Exclude: args.Without}
	if args.Query == "" && args.Precision == "" && args.Maturity == "" && capabilityFilter.IsZero() {
		return errorResult("'query', 'precision', 'maturity', 'needs', or 'without' argument is required for operation=search")
	}

	if errResult := validateCapabilityArgs(args.Needs, args.Without); errResult != nil {
		return *errResult
	}

	q := strings.ToLower(args.Query)
	categoryFilter := strings.ToLower(args.Category)

	precisionFilter, maturityFilter, maturityFilterSet, errResult := parseSearchFilters(args)
	if errResult != nil {
		return *errResult
	}

	type hit struct {
		Name         string   `json:"name"`
		Category     string   `json:"ruleSet"`
		Severity     string   `json:"severity"`
		Active       bool     `json:"active"`
		Fixable      bool     `json:"fixable"`
		Precision    string   `json:"precision"`
		Maturity     string   `json:"maturity"`
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

		if maturityFilterSet && r.Maturity != maturityFilter {
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
			// No query text: any of precision / maturity / needs / without acts
			// as the filter. We've already applied those above, so anything that
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
			Maturity:     r.Maturity.String(),
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
	matCounts := map[api.Maturity]int{}
	matActive := map[api.Maturity]int{}
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
		matCounts[r.Maturity]++
		if isActive {
			matActive[r.Maturity]++
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

	maturityOrder := []api.Maturity{api.MaturityStable, api.MaturityExperimental, api.MaturityDeprecated}
	maturities := make([]bucketRow, 0, len(maturityOrder))
	for _, m := range maturityOrder {
		maturities = append(maturities, bucketRow{
			Name:      m.String(),
			RuleCount: matCounts[m],
			Active:    matActive[m],
		})
	}

	type categoriesResult struct {
		Total      int         `json:"total"`
		Categories []bucketRow `json:"categories"`
		Precisions []bucketRow `json:"precisions"`
		Maturities []bucketRow `json:"maturities"`
	}

	return jsonResult(categoriesResult{
		Total:      len(rows),
		Categories: rows,
		Precisions: precisions,
		Maturities: maturities,
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
