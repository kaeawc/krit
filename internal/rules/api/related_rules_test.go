package api

import (
	"errors"
	"testing"
)

// rule constructs a minimal *Rule fixture for relation-validation tests.
// It bypasses Register so the global Registry stays untouched.
func relatedRule(id string, related ...string) *Rule {
	return &Rule{
		ID:           id,
		Description:  id + " description",
		Sev:          SeverityInfo,
		RelatedRules: related,
	}
}

func TestValidateRelations_TriangleIsAsymmetric(t *testing.T) {
	// Asymmetric chain: A → B, B → C, C → (nothing). The issue calls out
	// that relations are directional advisory hints — symmetry is not
	// required.
	a := relatedRule("A", "B")
	b := relatedRule("B", "C")
	c := relatedRule("C")
	if err := ValidateRelations([]*Rule{a, b, c}); err != nil {
		t.Fatalf("ValidateRelations on asymmetric chain returned %v; want nil", err)
	}
}

func TestValidateRelations_UnknownIDIsRejected(t *testing.T) {
	a := relatedRule("A", "Ghost")
	err := ValidateRelations([]*Rule{a})
	if err == nil {
		t.Fatalf("ValidateRelations on dangling reference returned nil; want error")
	}
	var rel *RelationError
	if !errors.As(err, &rel) {
		t.Fatalf("ValidateRelations returned %T; want *RelationError", err)
	}
	if rel.Rule != "A" || rel.Reference != "Ghost" {
		t.Fatalf("RelationError = %+v; want rule=A reference=Ghost", rel)
	}
}

func TestValidateRelations_SelfReferenceIsRejected(t *testing.T) {
	a := relatedRule("A", "A")
	err := ValidateRelations([]*Rule{a})
	if err == nil {
		t.Fatalf("ValidateRelations on self-reference returned nil; want error")
	}
}

func TestValidateRelations_EmptyRegistryIsOK(t *testing.T) {
	if err := ValidateRelations(nil); err != nil {
		t.Fatalf("ValidateRelations(nil) = %v; want nil", err)
	}
}

func TestValidateRelations_NilRulesAreSkipped(t *testing.T) {
	a := relatedRule("A", "B")
	b := relatedRule("B")
	if err := ValidateRelations([]*Rule{nil, a, nil, b}); err != nil {
		t.Fatalf("ValidateRelations with nil entries returned %v; want nil", err)
	}
}
