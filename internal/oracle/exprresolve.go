package oracle

import (
	"encoding/json"
	"fmt"

	"github.com/kaeawc/krit/internal/typeinfer"
)

// ExpressionPosition identifies a single oracle expression-type query
// by 1-based line and column. Mirrors api.ExpressionPosition (the
// rule-side equivalent) so callers can convert by struct literal.
// Kept independent of v2 here to avoid an oracle → rules-package
// dependency direction.
type ExpressionPosition struct {
	Line int
	Col  int
}

// resolveExpressionTypesResponse mirrors the JSON shape produced by
// DaemonSession.handleResolveExpressionTypes (krit-types Main.kt). The
// outer envelope ("id", "result") is unwrapped by sendResult; here we
// only need the result body.
type resolveExpressionTypesResponse struct {
	Types  map[string]map[string]resolvedExpressionFact `json:"types"`
	Errors map[string]string                            `json:"errors,omitempty"`
}

// resolvedExpressionFact is the wire shape for a single fact. Kind is
// recomputed Go-side from name (mirrors how typeinfer.ResolvedType is
// constructed elsewhere when only fqn + nullable arrive over the wire).
type resolvedExpressionFact struct {
	Name     string `json:"name"`
	FQN      string `json:"fqn"`
	Nullable bool   `json:"nullable"`
}

// ResolveExpressionTypes sends a batched resolveExpressionTypes RPC to
// the daemon and returns the resolved facts keyed by (file path,
// position). Positions that did not resolve to a fact are silently
// omitted — the caller treats absence as "no fact at this position."
//
// On RPC error (timeout, daemon dead, malformed response), returns
// (nil, err); the caller should fall through to source-only inference.
func (d *Daemon) ResolveExpressionTypes(positions map[string][]ExpressionPosition) (map[string]map[ExpressionPosition]*typeinfer.ResolvedType, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	wirePositions := make(map[string][]map[string]int, len(positions))
	for path, list := range positions {
		entries := make([]map[string]int, len(list))
		for i, pos := range list {
			entries[i] = map[string]int{"line": pos.Line, "col": pos.Col}
		}
		wirePositions[path] = entries
	}

	params := map[string]interface{}{"expressionPositions": wirePositions}
	rawResult, err := d.sendResult("resolveExpressionTypes", params)
	if err != nil {
		return nil, err
	}
	if rawResult == nil {
		return map[string]map[ExpressionPosition]*typeinfer.ResolvedType{}, nil
	}

	var resp resolveExpressionTypesResponse
	if err := json.Unmarshal(*rawResult, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal resolveExpressionTypes response: %w", err)
	}

	out := make(map[string]map[ExpressionPosition]*typeinfer.ResolvedType, len(resp.Types))
	for path, byKey := range resp.Types {
		fileMap := make(map[ExpressionPosition]*typeinfer.ResolvedType, len(byKey))
		for posKey, fact := range byKey {
			pos, ok := parseExpressionPositionKey(posKey)
			if !ok {
				continue
			}
			fileMap[pos] = factToResolvedType(fact)
		}
		out[path] = fileMap
	}
	return out, nil
}

// SetExpressionFact injects a single resolved expression fact into the
// oracle's expression map. Intended to be called from the targeted-
// resolution pre-pass, between Oracle.Load and the start of dispatch —
// there are no concurrent readers of expressions during that window.
// Calling this concurrently with rule dispatch is undefined.
//
// A nil type is silently ignored so callers can pass through "no fact"
// markers from the resolver without checking each entry.
func (o *Oracle) SetExpressionFact(filePath string, line, col int, t *typeinfer.ResolvedType) {
	if o == nil || t == nil {
		return
	}
	fileExprs, ok := o.expressions[filePath]
	if !ok {
		fileExprs = make(map[uint64]*typeinfer.ResolvedType)
		o.expressions[filePath] = fileExprs
	}
	fileExprs[packLineCol(line, col)] = t
}

// parseExpressionPositionKey turns the wire key "L:C" into an
// ExpressionPosition. Malformed keys are dropped (returns false) so
// one bad entry doesn't fail the whole batch.
func parseExpressionPositionKey(key string) (ExpressionPosition, bool) {
	var line, col int
	n, err := fmt.Sscanf(key, "%d:%d", &line, &col)
	if err != nil || n != 2 {
		return ExpressionPosition{}, false
	}
	return ExpressionPosition{Line: line, Col: col}, true
}

// factToResolvedType reshapes the wire fact into the typeinfer struct
// rules consume. Kind is derived from the simple name using the same
// rule set as makeResolvedType in oracle.go (primitive / Unit /
// Nothing / nullable / class) so callers see the same shape regardless
// of how the fact arrived.
func factToResolvedType(fact resolvedExpressionFact) *typeinfer.ResolvedType {
	kind := typeinfer.TypeClass
	if _, ok := typeinfer.PrimitiveTypes[fact.Name]; ok {
		kind = typeinfer.TypePrimitive
	}
	if fact.Name == "Unit" {
		kind = typeinfer.TypeUnit
	}
	if fact.Name == "Nothing" {
		kind = typeinfer.TypeNothing
	}
	if fact.Nullable {
		kind = typeinfer.TypeNullable
	}
	return &typeinfer.ResolvedType{
		Name:     fact.Name,
		FQN:      fact.FQN,
		Kind:     kind,
		Nullable: fact.Nullable,
	}
}
