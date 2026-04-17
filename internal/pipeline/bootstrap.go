package pipeline

import (
	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// DefaultActiveRules returns the v2 rules enabled by default for this
// build. The LSP server, MCP server, and CLI all derive their active
// rule list from this function so a rule that ships disabled-by-default
// stays disabled in every entry point without special-case filtering.
//
// It also ensures v2 rules have been bridged into the v1 Registry via
// rules.RegisterV2Rules (idempotent) so callers that still read the v1
// Registry (e.g. rule-audit, JSON output fix-level lookup) see the same
// set.
func DefaultActiveRules() []*v2.Rule {
	rules.RegisterV2Rules()
	active := make([]*v2.Rule, 0, len(v2.Registry))
	for _, r := range v2.Registry {
		if rules.IsDefaultActive(r.ID) {
			active = append(active, r)
		}
	}
	return active
}

// BuildDispatcher constructs a *rules.Dispatcher from the given v2 rules
// and an optional type resolver. Centralising this means the LSP and
// MCP servers no longer duplicate the rule-conversion and dispatcher
// construction logic (roadmap acceptance criterion: "LSP and MCP use
// no rule-dispatch logic of their own").
//
// Pass resolver=nil when no type-aware analysis is available; the
// dispatcher degrades gracefully.
func BuildDispatcher(activeRules []*v2.Rule, resolver typeinfer.TypeResolver) *rules.Dispatcher {
	v1 := v2RulesToV1(activeRules)
	if resolver == nil {
		return rules.NewDispatcher(v1)
	}
	return rules.NewDispatcher(v1, resolver)
}
