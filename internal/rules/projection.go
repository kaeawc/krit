package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// DiagnosticProjection describes how to turn a Kotlin compiler diagnostic
// already computed by the JVM oracle into a Krit finding. Krit owns the
// finding's message and fix; the projection layers Krit's presentation on top
// of the compiler's verdict rather than passing through the raw diagnostic.
type DiagnosticProjection struct {
	RuleID       string
	FactoryNames []string
	// Accept, when set, further filters a factory-matched, node-anchored
	// diagnostic before it is projected. It is for factories broader than the
	// rule (e.g. SENSELESS_COMPARISON covers every always-true/false condition,
	// but a null-check rule owns only the null-comparison direction). Returning
	// false skips this diagnostic and lets the caller's heuristic run.
	Accept     func(ctx *api.Context, d oracle.Diagnostic) bool
	Message    func(ctx *api.Context, d oracle.Diagnostic) string
	Fix        func(ctx *api.Context, d oracle.Diagnostic) *scanner.Fix
	Confidence float64
}

// projectDiagnostic looks up compiler diagnostics for ctx.Idx and, when a
// matching factory overlaps that node, emits one projected finding. It returns
// false when an oracle is unavailable or no matching diagnostic overlaps, so
// callers can run their existing source-level heuristic unchanged.
func projectDiagnostic(ctx *api.Context, spec DiagnosticProjection) bool {
	if ctx == nil || ctx.File == nil || ctx.Resolver == nil || spec.Message == nil {
		return false
	}
	resolver, ok := ctx.Resolver.(*oracle.CompositeResolver)
	if !ok || resolver.Oracle() == nil {
		return false
	}
	for _, d := range oracleLookupDiagnosticsForFlatRange(resolver.Oracle(), ctx.File, ctx.Idx) {
		if !diagnosticProjectionMatchesFactory(spec.FactoryNames, d.FactoryName) ||
			!diagnosticAnchoredAtNode(ctx.File, ctx.Idx, d) {
			continue
		}
		if spec.Accept != nil && !spec.Accept(ctx, d) {
			continue
		}
		line, col := d.Line, d.Col
		if line <= 0 || col <= 0 {
			line = ctx.File.FlatRow(ctx.Idx) + 1
			col = ctx.File.FlatCol(ctx.Idx) + 1
		}
		f := scanner.Finding{
			Line:       line,
			Col:        col,
			Message:    spec.Message(ctx, d),
			Confidence: spec.Confidence,
		}
		if spec.Fix != nil {
			f.Fix = spec.Fix(ctx, d)
		}
		ctx.Emit(f)
		return true
	}
	return false
}

func diagnosticProjectionMatchesFactory(factoryNames []string, factoryName string) bool {
	for _, want := range factoryNames {
		if factoryName == want {
			return true
		}
	}
	return false
}

// diagnosticAnchoredAtNode reports whether d belongs to ctx.Idx itself rather
// than to a same-typed node nested inside it. The compiler anchors a diagnostic
// on a specific node; a rule that dispatches on a node type T must claim the
// diagnostic only when ctx.Idx is the tightest T covering the diagnostic's span.
// Otherwise an inner useless elvis (or cast, etc.) nested in the right operand
// of an outer expression of the same type would be mis-attributed to the outer
// node — a false positive with a code-deleting fix. A plain byte-range overlap
// cannot make that distinction; requiring that no same-typed descendant also
// covers the span can. (Climbing from a byte-range descendant is unsound: a
// wrapper node sharing the inner node's exact span is the shallower match and
// would climb past the nested node to the enclosing one.)
func diagnosticAnchoredAtNode(file *scanner.File, idx uint32, d oracle.Diagnostic) bool {
	if file == nil || idx == 0 {
		return false
	}
	startByte, endByte, ok := diagnosticAnchorBytes(file, d)
	if !ok {
		return false
	}
	// The dispatched node must itself cover the diagnostic span.
	if startByte < file.FlatStartByte(idx) || endByte > file.FlatEndByte(idx) {
		return false
	}
	// Reject when a same-typed node nested inside ctx.Idx also covers the
	// span: the compiler anchored on that inner node, not on ctx.Idx.
	nested := false
	file.FlatWalkNodes(idx, file.FlatType(idx), func(child uint32) {
		if nested || child == idx {
			return
		}
		if file.FlatStartByte(child) <= startByte && file.FlatEndByte(child) >= endByte {
			nested = true
		}
	})
	return !nested
}

// diagnosticAnchorBytes returns the byte span the compiler attached d to: its
// explicit byte range when present, otherwise the zero-width point at its
// line/col. It reports false when neither is available.
func diagnosticAnchorBytes(file *scanner.File, d oracle.Diagnostic) (start, end uint32, ok bool) {
	if d.EndByte > d.StartByte {
		return uint32(d.StartByte), uint32(d.EndByte), true
	}
	if d.StartByte > 0 {
		return uint32(d.StartByte), uint32(d.StartByte), true
	}
	if d.Line <= 0 || d.Col <= 0 {
		return 0, 0, false
	}
	offset := file.LineOffset(d.Line-1) + d.Col - 1
	if offset < 0 {
		return 0, 0, false
	}
	return uint32(offset), uint32(offset), true
}
