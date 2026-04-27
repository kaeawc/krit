package rules

import (
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
)

type oracleFlatLookup interface {
	LookupCallTargetFlat(file *scanner.File, idx uint32) string
	LookupCallTargetSuspendFlat(file *scanner.File, idx uint32) (bool, bool)
	LookupCallTargetAnnotationsFlat(file *scanner.File, idx uint32) []string
	LookupDiagnosticsForFlatRange(file *scanner.File, idx uint32) []oracle.OracleDiagnostic
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

func oracleLookupDiagnosticsForFlatRange(lookup oracle.Lookup, file *scanner.File, idx uint32) []oracle.OracleDiagnostic {
	if lookup == nil || file == nil {
		return nil
	}
	if flat, ok := lookup.(oracleFlatLookup); ok {
		return flat.LookupDiagnosticsForFlatRange(file, idx)
	}
	return lookup.LookupDiagnostics(file.Path)
}
