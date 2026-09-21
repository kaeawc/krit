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
	Message      func(ctx *api.Context, d oracle.Diagnostic) string
	Fix          func(ctx *api.Context, d oracle.Diagnostic) *scanner.Fix
	Confidence   float64
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
			!diagnosticProjectionOverlapsFlat(ctx.File, ctx.Idx, d) {
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

func diagnosticProjectionOverlapsFlat(file *scanner.File, idx uint32, d oracle.Diagnostic) bool {
	if file == nil || idx == 0 {
		return false
	}
	if d.EndByte > d.StartByte {
		return d.StartByte < int(file.FlatEndByte(idx)) && d.EndByte > int(file.FlatStartByte(idx))
	}
	if d.Line <= 0 || d.Col <= 0 {
		return false
	}
	offset := file.LineOffset(d.Line-1) + d.Col - 1
	return offset >= int(file.FlatStartByte(idx)) && offset < int(file.FlatEndByte(idx))
}
