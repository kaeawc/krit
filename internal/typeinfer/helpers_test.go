package typeinfer

import "testing"

// --- extractVisibility ---

func TestExtractVisibility_Private(t *testing.T) {
	if got := extractVisibility("private fun foo()"); got != "private" {
		t.Errorf("expected 'private', got %q", got)
	}
}

func TestExtractVisibility_Internal(t *testing.T) {
	if got := extractVisibility("internal class Bar"); got != "internal" {
		t.Errorf("expected 'internal', got %q", got)
	}
}

func TestExtractVisibility_Protected(t *testing.T) {
	if got := extractVisibility("protected val x: Int"); got != "protected" {
		t.Errorf("expected 'protected', got %q", got)
	}
}

func TestExtractVisibility_PublicDefault(t *testing.T) {
	if got := extractVisibility("fun foo()"); got != "public" {
		t.Errorf("expected 'public', got %q", got)
	}
}

func TestExtractVisibility_ExplicitPublic(t *testing.T) {
	if got := extractVisibility("public fun foo()"); got != "public" {
		t.Errorf("expected 'public', got %q", got)
	}
}

func TestExtractVisibility_EmptyString(t *testing.T) {
	if got := extractVisibility(""); got != "public" {
		t.Errorf("expected 'public' for empty string, got %q", got)
	}
}
