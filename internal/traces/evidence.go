package traces

// IsObservedSymbol reports whether any resolved runtime state pins
// the given static symbol FQN. Rules that declare
// NeedsRuntimeEvidence should call this with a candidate dead-code
// symbol and skip / downgrade the finding when the answer is true.
//
// Empty stores answer false — the absence of trace evidence is not
// itself evidence the symbol is dead. Callers should also fall back
// to static-only analysis when the store has no sources at all.
func (s *Store) IsObservedSymbol(fqn string) bool {
	if s == nil || fqn == "" {
		return false
	}
	for _, st := range s.States {
		if st.TopSymbol == fqn {
			return true
		}
	}
	return false
}

// HasEvidence reports whether the store carries any ingested
// runtime data. Rules use this to decide whether to apply the
// confidence adjustment at all: an empty store cannot disprove a
// static finding, so the rule must fall back to its static behavior.
func (s *Store) HasEvidence() bool {
	if s == nil {
		return false
	}
	return len(s.Sources) > 0 || len(s.States) > 0
}

// AdjustConfidence is the canonical confidence-downgrade pattern for
// rules that consume runtime evidence. When the store has evidence
// and the symbol is observed, the rule's base confidence is
// multiplied by adjustment (typically <= 0.5) — making the finding
// less likely to surface, but never eliminating it outright (the
// static signal may still be correct, e.g. when traces are stale).
//
// Returns base unchanged when the store has no evidence or the
// symbol is not observed.
func (s *Store) AdjustConfidence(base float64, fqn string, adjustment float64) float64 {
	if !s.HasEvidence() || !s.IsObservedSymbol(fqn) {
		return base
	}
	return base * adjustment
}
