package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

type oracleFlatLookup interface {
	LookupCallTargetFlat(file *scanner.File, idx uint32) string
	LookupCallTargetSuspendFlat(file *scanner.File, idx uint32) (bool, bool)
	LookupCallTargetAnnotationsFlat(file *scanner.File, idx uint32) []string
	LookupDiagnosticsForFlatRange(file *scanner.File, idx uint32) []oracle.Diagnostic
}

type oracleFlatExpressionLookup interface {
	LookupExpressionFlat(file *scanner.File, idx uint32) *typeinfer.ResolvedType
}

// oracleLookupExpressionFlat returns the compiler type of the node's own byte range, so a receiver
// that starts at the same position as a call does not answer for it.
func oracleLookupExpressionFlat(lookup oracle.Lookup, file *scanner.File, idx uint32) *typeinfer.ResolvedType {
	if lookup == nil || file == nil {
		return nil
	}
	if flat, ok := lookup.(oracleFlatExpressionLookup); ok {
		return flat.LookupExpressionFlat(file, idx)
	}
	return lookup.LookupExpression(file.Path, file.FlatRow(idx)+1, file.FlatCol(idx)+1)
}

func oracleLookupCallTargetFlat(lookup oracle.Lookup, file *scanner.File, idx uint32) string {
	if lookup == nil || file == nil {
		return ""
	}
	if flat, ok := lookup.(oracleFlatLookup); ok {
		return flat.LookupCallTargetFlat(file, idx)
	}
	return lookup.LookupCallTarget(file.Path, file.FlatRow(idx)+1, file.FlatCol(idx)+1)
}

func oracleLookupCallTargetSuspendFlat(lookup oracle.Lookup, file *scanner.File, idx uint32) (bool, bool) {
	if lookup == nil || file == nil {
		return false, false
	}
	if flat, ok := lookup.(oracleFlatLookup); ok {
		return flat.LookupCallTargetSuspendFlat(file, idx)
	}
	return lookup.LookupCallTargetSuspend(file.Path, file.FlatRow(idx)+1, file.FlatCol(idx)+1)
}

func oracleLookupCallTargetAnnotationsFlat(lookup oracle.Lookup, file *scanner.File, idx uint32) []string {
	if lookup == nil || file == nil {
		return nil
	}
	if flat, ok := lookup.(oracleFlatLookup); ok {
		return flat.LookupCallTargetAnnotationsFlat(file, idx)
	}
	return lookup.LookupCallTargetAnnotations(file.Path, file.FlatRow(idx)+1, file.FlatCol(idx)+1)
}

func oracleLookupDiagnosticsForFlatRange(lookup oracle.Lookup, file *scanner.File, idx uint32) []oracle.Diagnostic {
	if lookup == nil || file == nil {
		return nil
	}
	if flat, ok := lookup.(oracleFlatLookup); ok {
		return flat.LookupDiagnosticsForFlatRange(file, idx)
	}
	return lookup.LookupDiagnostics(file.Path)
}
