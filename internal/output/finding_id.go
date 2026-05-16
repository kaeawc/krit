package output

import "fmt"

// FindingID returns the deterministic, transport-independent identifier
// for a JSON finding. The format is "rule:file:line:column" using the
// exact field values present in the JSON report; no normalization is
// performed so the id round-trips through cold/warm runs that use the
// same report.
//
// Used by `krit apply-suggestion` (and any future LSP/IDE flow that
// needs to refer to a finding without round-tripping through scanner
// internals) to look up findings by id.
func FindingID(f JSONFinding) string {
	return fmt.Sprintf("%s:%s:%d:%d", f.Rule, f.File, f.Line, f.Column)
}
