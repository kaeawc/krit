package pipeline

import (
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// TargetedResolutionInput bundles every dependency RunTargetedResolutionPass
// needs. Pulled out as a struct so the call site is one named field per
// dependency and tests can construct it without threading nine arguments.
type TargetedResolutionInput struct {
	// ActiveRules is the rule set whose ExprPositions selectors will be
	// invoked. Rules without a selector contribute nothing.
	ActiveRules []*api.Rule
	// Files is the parsed source set the selectors walk.
	Files []*scanner.File
	// Resolver dispatches the batched RPC. nil disables the pass.
	// Production wires this to a Daemon-backed adapter; tests inject
	// a fake.
	Resolver api.ExpressionTypeResolver
	// Sink writes resolved facts into the oracle. nil disables the pass.
	Sink api.ExpressionFactSink
}

// RunTargetedResolutionPass orchestrates the on-demand expression-fact
// resolution pre-pass. It walks active rules' ExprPositions selectors
// over the parsed file set, batches the union of requested positions
// to the resolver, and writes returned facts to the sink — populating
// the oracle's expression map before dispatch begins.
//
// Returns nil with no work when no rule supplies a selector or when
// the resolver/sink are missing. A resolver error is returned without
// applying any partial results — callers should log and continue with
// source-only inference.
func RunTargetedResolutionPass(in TargetedResolutionInput) error {
	if in.Resolver == nil || in.Sink == nil {
		return nil
	}
	positions := api.CollectExpressionPositions(in.ActiveRules, in.Files)
	if len(positions) == 0 {
		return nil
	}
	results, err := in.Resolver.Resolve(positions)
	if err != nil {
		return err
	}
	api.ApplyResolvedExpressions(in.Sink, results)
	return nil
}

// DaemonExpressionResolver adapts an *oracle.Daemon to api.ExpressionTypeResolver.
// Converts between api.ExpressionPosition (rule-side) and
// oracle.ExpressionPosition (RPC-side) — the two have identical field
// shapes but live in separate packages to avoid an oracle → rules
// dependency direction.
type DaemonExpressionResolver struct {
	Daemon *oracle.Daemon
}

// Resolve implements api.ExpressionTypeResolver. Per-position conversion
// is straight literal copies; the Daemon does the actual RPC.
func (r DaemonExpressionResolver) Resolve(positions map[string][]api.ExpressionPosition) (map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType, error) {
	if r.Daemon == nil {
		return nil, nil
	}
	requested := make(map[string][]oracle.ExpressionPosition, len(positions))
	for path, list := range positions {
		converted := make([]oracle.ExpressionPosition, len(list))
		for i, p := range list {
			converted[i] = oracle.ExpressionPosition{Line: p.Line, Col: p.Col}
		}
		requested[path] = converted
	}
	resolved, err := r.Daemon.ResolveExpressionTypes(requested)
	if err != nil {
		return nil, err
	}
	out := make(map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType, len(resolved))
	for path, byPos := range resolved {
		fileMap := make(map[api.ExpressionPosition]*typeinfer.ResolvedType, len(byPos))
		for pos, rt := range byPos {
			fileMap[api.ExpressionPosition{Line: pos.Line, Col: pos.Col}] = rt
		}
		out[path] = fileMap
	}
	return out, nil
}
