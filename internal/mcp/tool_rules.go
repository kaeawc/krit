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
	// Taxonomy filters rules whose SecurityTaxonomy lists this ID
	// (case-insensitive). Accepts CWE/OWASP/SEI-CERT/MITRE IDs.
	Taxonomy string `json:"taxonomy"`

	// operation=configure
	Active   *bool  `json:"active"`
	Severity string `json:"severity"`

	// operation=search: optional LanguageSupport filter. Empty Language
	// disables the filter; an empty Status list with a non-empty Language
	// keeps every rule with a classification for that language.
	LanguageSupport *languageSupportArg `json:"languageSupport,omitempty"`
}

// languageSupportArg mirrors rules.LanguageSupportFilter on the wire.
type languageSupportArg struct {
	Language string   `json:"language"`
	Status   []string `json:"status,omitempty"`
	Negate   bool     `json:"negate,omitempty"`
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
	if mapping := securityMapping(r.Security); mapping != nil {
		info["security"] = mapping
	}

	return jsonResult(info)
}

// securityMapping renders the rule's security taxonomy as a per-axis
// map for MCP `explain` output, or nil when the rule carries no
// published taxonomy IDs.
func securityMapping(sec *api.SecurityTaxonomy) map[string][]string {
	if sec == nil || sec.IsEmpty() {
		return nil
	}
	mapping := map[string][]string{}
	if len(sec.CWE) > 0 {
		mapping["cwe"] = sec.CWE
	}
	if len(sec.OWASP) > 0 {
		mapping["owasp"] = sec.OWASP
	}
	if len(sec.SEICert) > 0 {
		mapping["sei-cert"] = sec.SEICert
	}
	if len(sec.Mitre) > 0 {
		mapping["mitre"] = sec.Mitre
	}
	return mapping
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
// shipped and (optionally) when it became default-active.
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

// parseSearchFilters validates the precision/maturity/taxonomy filter
// arguments. Returns a non-nil ToolResult pointer when validation fails.
func parseSearchFilters(args rulesArgs) (api.Precision, api.Maturity, bool, api.TaxonomyMatcher, *ToolResult) {
	var precisionFilter api.Precision
	if args.Precision != "" {
		p, ok := api.ParsePrecision(args.Precision)
		if !ok {
			r := errorResult("unknown precision: " + args.Precision +
				"; valid: heuristic/text-backed, ast-backed, project-structure-aware, type-aware, policy")
			return 0, 0, false, api.TaxonomyMatcher{}, &r
		}
		precisionFilter = p
	}

	var maturityFilter api.Maturity
	maturityFilterSet := false
	if args.Maturity != "" {
		m, ok := api.ParseMaturity(args.Maturity)
		if !ok {
			r := errorResult("unknown maturity: " + args.Maturity + "; valid: stable, experimental, deprecated")
			return 0, 0, false, api.TaxonomyMatcher{}, &r
		}
		maturityFilter = m
		maturityFilterSet = true
	}

	var taxonomyMatcher api.TaxonomyMatcher
	if args.Taxonomy != "" {
		taxonomyMatcher = api.TaxonomyMatcher{IDs: []string{args.Taxonomy}}
	}
	return precisionFilter, maturityFilter, maturityFilterSet, taxonomyMatcher, nil
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

// buildSupportFilter converts a rulesArgs.LanguageSupport payload into a
// validated rules.LanguageSupportFilter. Returns a non-nil ToolResult only
// when validation fails.
func buildSupportFilter(args rulesArgs) (rules.LanguageSupportFilter, *ToolResult) {
	if args.LanguageSupport == nil || args.LanguageSupport.Language == "" {
		return rules.LanguageSupportFilter{}, nil
	}
	statuses := make([]api.LanguageSupportStatus, 0, len(args.LanguageSupport.Status))
	for _, s := range args.LanguageSupport.Status {
		statuses = append(statuses, api.LanguageSupportStatus(s))
	}
	f := rules.LanguageSupportFilter{
		Language: args.LanguageSupport.Language,
		Status:   statuses,
		Negate:   args.LanguageSupport.Negate,
	}
	if err := f.Validate(); err != nil {
		errRes := errorResult(err.Error())
		return rules.LanguageSupportFilter{}, &errRes
	}
	return f, nil
}

type ruleHit struct {
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

type searchOpts struct {
	query             string
	category          string
	precisionFilter   api.Precision
	maturityFilter    api.Maturity
	maturityFilterSet bool
	taxonomyMatcher   api.TaxonomyMatcher
	supportFilter     rules.LanguageSupportFilter
	hasSupportFilter  bool
	capabilityFilter  api.CapabilityFilter
	hasCapabilityFilt bool
}

func (o searchOpts) hasTaxonomyFilter() bool {
	return len(o.taxonomyMatcher.IDs) > 0
}

// scoreRuleForSearch returns (score, keep). Scoring favors name > description
// > category matches; a missing query is allowed when another filter is set.
func scoreRuleForSearch(r *api.Rule, opts searchOpts) (int, bool) {
	if opts.query == "" {
		if opts.precisionFilter == api.PrecisionUnset && !opts.maturityFilterSet && !opts.hasTaxonomyFilter() && !opts.hasSupportFilter && !opts.hasCapabilityFilt {
			return 0, false
		}
		return 1, true
	}
	switch {
	case strings.Contains(strings.ToLower(r.ID), opts.query):
		return 3, true
	case strings.Contains(strings.ToLower(r.Description), opts.query):
		return 2, true
	case strings.Contains(strings.ToLower(r.Category), opts.query):
		return 1, true
	}
	return 0, false
}

func collectRuleHits(opts searchOpts) []ruleHit {
	hits := make([]ruleHit, 0, 32)
	for _, r := range api.Registry {
		if opts.category != "" && !strings.EqualFold(r.Category, opts.category) {
			continue
		}
		precision := rules.V2RulePrecision(r)
		if opts.precisionFilter != api.PrecisionUnset && precision != opts.precisionFilter {
			continue
		}
		if opts.maturityFilterSet && r.Maturity != opts.maturityFilter {
			continue
		}
		if opts.hasTaxonomyFilter() && !opts.taxonomyMatcher.Matches(r.Security) {
			continue
		}
		if opts.hasSupportFilter && !opts.supportFilter.Matches(r) {
			continue
		}
		if !opts.capabilityFilter.MatchRule(r) {
			continue
		}
		score, keep := scoreRuleForSearch(r, opts)
		if !keep {
			continue
		}
		_, fixable := rules.GetV2FixLevel(r)
		hits = append(hits, ruleHit{
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
	return hits
}

// rulesSearch performs a case-insensitive substring match over rule name,
// description, and category. Results are ranked by where the match landed
// (name > description > category) then alphabetical.
func (s *Server) rulesSearch(args rulesArgs) ToolResult {
	hasSupportFilter := args.LanguageSupport != nil && args.LanguageSupport.Language != ""
	capabilityFilter := api.CapabilityFilter{Require: args.Needs, Exclude: args.Without}
	hasCapabilityFilt := !capabilityFilter.IsZero()

	if args.Query == "" && args.Precision == "" && args.Maturity == "" && args.Taxonomy == "" && !hasSupportFilter && !hasCapabilityFilt {
		return errorResult("'query', 'precision', 'maturity', 'taxonomy', 'languageSupport', 'needs', or 'without' argument is required for operation=search")
	}

	if errResult := validateCapabilityArgs(args.Needs, args.Without); errResult != nil {
		return *errResult
	}

	precisionFilter, maturityFilter, maturityFilterSet, taxonomyMatcher, errResult := parseSearchFilters(args)
	if errResult != nil {
		return *errResult
	}

	supportFilter, errResult := buildSupportFilter(args)
	if errResult != nil {
		return *errResult
	}

	hits := collectRuleHits(searchOpts{
		query:             strings.ToLower(args.Query),
		category:          args.Category,
		precisionFilter:   precisionFilter,
		maturityFilter:    maturityFilter,
		maturityFilterSet: maturityFilterSet,
		taxonomyMatcher:   taxonomyMatcher,
		supportFilter:     supportFilter,
		hasSupportFilter:  hasSupportFilter,
		capabilityFilter:  capabilityFilter,
		hasCapabilityFilt: hasCapabilityFilt,
	})

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Name < hits[j].Name
	})

	type searchResult struct {
		Query string    `json:"query"`
		Total int       `json:"total"`
		Hits  []ruleHit `json:"hits"`
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
