package api

import "fmt"

// FixMode classifies how a rule exposes fixes. Exactly one mode applies
// to a registered rule; mixed declarations are rejected at Register
// time. See Rule.FixMode for the derivation from Fix and SuggestedFixes.
type FixMode int

const (
	// FixModeNone marks a diagnostic-only rule. The rule never emits
	// any fix payload alongside its findings.
	FixModeNone FixMode = iota
	// FixModeAutofix marks a rule that emits a single automatic fix
	// per finding. The fix safety tier is carried by Rule.Fix.
	FixModeAutofix
	// FixModeSuggested marks a rule that emits one or more ordered,
	// user-selectable suggestions per finding. The slice order in
	// Rule.SuggestedFixes is the recommended display and application
	// order.
	FixModeSuggested
)

// String returns the canonical token for a FixMode. Unknown values
// fall back to "none" to match the FixLevel / Maturity stringer style.
func (m FixMode) String() string {
	switch m {
	case FixModeAutofix:
		return "autofix"
	case FixModeSuggested:
		return "suggested"
	default:
		return "none"
	}
}

// SuggestedFix declares one entry in a rule's ordered suggested-fix
// list. Position in Rule.SuggestedFixes is the recommended display and
// application order; safer or stronger fixes should be listed first.
type SuggestedFix struct {
	// ID is a stable, rule-scoped identifier for this suggestion, e.g.
	// "useApply" or "addOptIn". Must be non-empty and unique within a
	// rule's SuggestedFixes.
	ID string

	// Title is a short human-readable description shown in IDE quick-fix
	// menus, e.g. "Replace with apply { ... }". Must be non-empty.
	Title string

	// Level is the safety tier of this suggestion. FixNone is invalid;
	// every suggestion must declare a non-zero tier so consumers can
	// apply the same risk filtering they use for autofixes.
	Level FixLevel
}

// AutofixRule is an optional marker interface that a rule's
// Implementation may satisfy to make its autofix capability visible at
// compile time. Mutually exclusive with SuggestedFixRule; an
// Implementation must not satisfy both.
type AutofixRule interface {
	AutofixLevel() FixLevel
}

// SuggestedFixRule is an optional marker interface that a rule's
// Implementation may satisfy to advertise ordered user-selectable
// suggestions. Mutually exclusive with AutofixRule.
type SuggestedFixRule interface {
	SuggestedFixes() []SuggestedFix
}

// FixMode returns the fix mode advertised by this rule, derived from
// Fix and SuggestedFixes. Register guarantees a registered rule is in
// exactly one mode, so the result is well-defined for any rule that
// passed Register.
func (r *Rule) FixMode() FixMode {
	if r == nil {
		return FixModeNone
	}
	if len(r.SuggestedFixes) > 0 {
		return FixModeSuggested
	}
	if r.Fix != FixNone {
		return FixModeAutofix
	}
	return FixModeNone
}

// FixModeErrorKind classifies a FixModeError so callers can branch on
// the specific invariant that was violated without parsing the message.
type FixModeErrorKind int

const (
	// FixModeErrorMixedDeclaration: rule sets both Fix and SuggestedFixes.
	FixModeErrorMixedDeclaration FixModeErrorKind = iota + 1
	// FixModeErrorMixedInterface: Implementation satisfies both
	// AutofixRule and SuggestedFixRule.
	FixModeErrorMixedInterface
	// FixModeErrorEmptyID: a SuggestedFix entry has an empty ID.
	FixModeErrorEmptyID
	// FixModeErrorEmptyTitle: a SuggestedFix entry has an empty Title.
	FixModeErrorEmptyTitle
	// FixModeErrorLevelNone: a SuggestedFix entry has Level == FixNone.
	FixModeErrorLevelNone
	// FixModeErrorDuplicateID: two SuggestedFix entries share an ID.
	FixModeErrorDuplicateID
)

// FixModeError describes an invalid fix-mode declaration surfaced by
// Rule.ValidateFixMode. The struct mirrors RelationError so callers can
// distinguish error kinds with errors.As instead of substring matching.
type FixModeError struct {
	Rule  string
	Kind  FixModeErrorKind
	Index int    // index into SuggestedFixes when relevant, -1 otherwise
	ID    string // suggestion ID when relevant
}

func (e *FixModeError) Error() string {
	switch e.Kind {
	case FixModeErrorMixedDeclaration:
		return fmt.Sprintf("rule %q declares both an autofix and suggested fixes; a rule must pick exactly one fix mode", e.Rule)
	case FixModeErrorMixedInterface:
		return fmt.Sprintf("rule %q Implementation satisfies both AutofixRule and SuggestedFixRule; pick exactly one", e.Rule)
	case FixModeErrorEmptyID:
		return fmt.Sprintf("rule %q SuggestedFixes[%d]: ID is empty", e.Rule, e.Index)
	case FixModeErrorEmptyTitle:
		return fmt.Sprintf("rule %q SuggestedFixes[%d] (%s): Title is empty", e.Rule, e.Index, e.ID)
	case FixModeErrorLevelNone:
		return fmt.Sprintf("rule %q SuggestedFixes[%d] (%s): Level is FixNone; suggestions must declare a non-zero safety tier", e.Rule, e.Index, e.ID)
	case FixModeErrorDuplicateID:
		return fmt.Sprintf("rule %q SuggestedFixes: duplicate ID %q", e.Rule, e.ID)
	default:
		return fmt.Sprintf("rule %q: unknown fix-mode error", e.Rule)
	}
}

// ValidateFixMode reports whether the rule's fix-mode declaration is
// well-formed. Invoked from Register; tests may call it directly.
// Errors are *FixModeError values so callers can branch with errors.As.
func (r *Rule) ValidateFixMode() error {
	if r == nil {
		return nil
	}
	hasAutofix := r.Fix != FixNone
	hasSuggested := len(r.SuggestedFixes) > 0
	if hasAutofix && hasSuggested {
		return &FixModeError{Rule: r.ID, Kind: FixModeErrorMixedDeclaration, Index: -1}
	}
	if hasSuggested {
		seen := make(map[string]bool, len(r.SuggestedFixes))
		for i, s := range r.SuggestedFixes {
			if s.ID == "" {
				return &FixModeError{Rule: r.ID, Kind: FixModeErrorEmptyID, Index: i}
			}
			if s.Title == "" {
				return &FixModeError{Rule: r.ID, Kind: FixModeErrorEmptyTitle, Index: i, ID: s.ID}
			}
			if s.Level == FixNone {
				return &FixModeError{Rule: r.ID, Kind: FixModeErrorLevelNone, Index: i, ID: s.ID}
			}
			if seen[s.ID] {
				return &FixModeError{Rule: r.ID, Kind: FixModeErrorDuplicateID, Index: -1, ID: s.ID}
			}
			seen[s.ID] = true
		}
	}
	if impl := r.Implementation; impl != nil {
		_, isAutofix := impl.(AutofixRule)
		_, isSuggested := impl.(SuggestedFixRule)
		if isAutofix && isSuggested {
			return &FixModeError{Rule: r.ID, Kind: FixModeErrorMixedInterface, Index: -1}
		}
	}
	return nil
}
