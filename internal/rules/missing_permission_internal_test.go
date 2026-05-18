package rules

import "testing"

func TestMissingPermissionRequirement_Helpers(t *testing.T) {
	t.Run("single covers exact perm only", func(t *testing.T) {
		r := singlePermRequirement("CAMERA")
		if !r.covers("CAMERA") {
			t.Fatal("single CAMERA should cover CAMERA")
		}
		if r.covers("RECORD_AUDIO") {
			t.Fatal("single CAMERA should not cover RECORD_AUDIO")
		}
	})
	t.Run("anyOf never covers a specific perm", func(t *testing.T) {
		r := missingPermissionRequirement{kind: missingPermissionRequirementAnyOf, perms: []string{"A", "B"}}
		if r.covers("A") || r.covers("B") {
			t.Fatal("anyOf must not be treated as a coverage guarantee for any specific permission")
		}
	})
	t.Run("allOf covers every listed perm", func(t *testing.T) {
		r := missingPermissionRequirement{kind: missingPermissionRequirementAllOf, perms: []string{"A", "B"}}
		if !r.covers("A") || !r.covers("B") {
			t.Fatal("allOf must cover every listed permission")
		}
		if r.covers("C") {
			t.Fatal("allOf must not cover an unrelated permission")
		}
	})
	t.Run("anyOf satisfied when any predicate holds", func(t *testing.T) {
		r := missingPermissionRequirement{kind: missingPermissionRequirementAnyOf, perms: []string{"A", "B"}}
		if !r.satisfied(func(p string) bool { return p == "B" }) {
			t.Fatal("anyOf should be satisfied when one alternative is guarded")
		}
		if r.satisfied(func(p string) bool { return false }) {
			t.Fatal("anyOf with no perm guarded should not be satisfied")
		}
	})
	t.Run("allOf satisfied only when every predicate holds", func(t *testing.T) {
		r := missingPermissionRequirement{kind: missingPermissionRequirementAllOf, perms: []string{"A", "B"}}
		if r.satisfied(func(p string) bool { return p == "A" }) {
			t.Fatal("allOf should not be satisfied when one perm is missing")
		}
		if !r.satisfied(func(p string) bool { return true }) {
			t.Fatal("allOf should be satisfied when all perms are guarded")
		}
	})
	t.Run("empty requirement is never satisfied", func(t *testing.T) {
		var r missingPermissionRequirement
		if !r.empty() {
			t.Fatal("zero requirement should report empty")
		}
		if r.satisfied(func(string) bool { return true }) {
			t.Fatal("empty requirement should never be satisfied")
		}
	})
	t.Run("describe joins anyOf with or and allOf with and", func(t *testing.T) {
		anyR := missingPermissionRequirement{kind: missingPermissionRequirementAnyOf, perms: []string{"A", "B"}}
		if got := anyR.describe(); got != "A or B" {
			t.Fatalf("anyOf describe: want %q, got %q", "A or B", got)
		}
		allR := missingPermissionRequirement{kind: missingPermissionRequirementAllOf, perms: []string{"A", "B"}}
		if got := allR.describe(); got != "A and B" {
			t.Fatalf("allOf describe: want %q, got %q", "A and B", got)
		}
		single := singlePermRequirement("CAMERA")
		if got := single.describe(); got != "CAMERA" {
			t.Fatalf("single describe: want %q, got %q", "CAMERA", got)
		}
	})
	t.Run("messageSuffix is plural for multi-perm requirements", func(t *testing.T) {
		if got := singlePermRequirement("CAMERA").messageSuffix(); got != "permission" {
			t.Fatalf("single suffix: want %q, got %q", "permission", got)
		}
		r := missingPermissionRequirement{kind: missingPermissionRequirementAnyOf, perms: []string{"A", "B"}}
		if got := r.messageSuffix(); got != "permissions" {
			t.Fatalf("multi suffix: want %q, got %q", "permissions", got)
		}
	})
}
