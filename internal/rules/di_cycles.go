package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/di"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type DiCycleDetectionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *DiCycleDetectionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DiCycleDetectionRule) check(ctx *api.Context) {
	if len(ctx.ParsedFiles) == 0 {
		return
	}
	graph := di.BuildGraph(ctx.ParsedFiles, nil)
	for _, cycle := range graph.FindCycles() {
		if len(cycle.Bindings) == 0 || cycle.Bindings[0] == nil {
			continue
		}
		first := cycle.Bindings[0]
		ctx.Emit(scanner.Finding{
			File:     first.File,
			Line:     first.Line,
			Col:      1,
			RuleSet:  r.RuleSetName,
			Rule:     r.RuleName,
			Severity: r.Sev,
			Message:  fmt.Sprintf("DI binding cycle detected: %s.", formatDICycle(cycle)),
		})
	}
}

func formatDICycle(cycle di.Cycle) string {
	names := make([]string, 0, len(cycle.Bindings)+1)
	for _, b := range cycle.Bindings {
		if b != nil {
			names = append(names, b.FQN)
		}
	}
	if len(names) > 0 {
		names = append(names, names[0])
	}
	return strings.Join(names, " -> ")
}
