package scanner

// SuggestedFix is an ordered, rule-emitted suggestion attached to a Finding.
// Distinct from Fix / BinaryFix: the autofix slot drives the `fixable` and
// `fixLevel` JSON fields and is bound by the rule's declared FixLevel safety
// tier, whereas suggestions are advisory and may be machine-applicable
// (non-empty Edits), application-resolved (non-empty ApplicationToken), or
// purely informational.
type SuggestedFix struct {
	ID               string
	Title            string
	Detail           string
	Edits            []SuggestedEdit
	ApplicationToken string
}

// SuggestedEdit mirrors Fix's line/byte-mode replacement shape so consumers
// can apply suggestions through the same code path as autofixes.
type SuggestedEdit struct {
	TargetFile  string
	StartLine   int
	EndLine     int
	Replacement string
	StartByte   int
	EndByte     int
	ByteMode    bool
}

func cloneSuggestedFix(fix SuggestedFix) SuggestedFix {
	if len(fix.Edits) > 0 {
		edits := make([]SuggestedEdit, len(fix.Edits))
		copy(edits, fix.Edits)
		fix.Edits = edits
	} else {
		fix.Edits = nil
	}
	return fix
}
