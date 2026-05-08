package codemod

import "testing"

func TestRecipeValidate(t *testing.T) {
	recipe := Recipe{
		Name:        "replace-timber",
		Language:    "kotlin",
		Match:       `((simple_identifier) @match (#eq? @match "Timber"))`,
		Replacement: "logger",
	}
	if err := recipe.Validate(); err != nil {
		t.Fatalf("expected valid recipe, got %v", err)
	}

	recipe.Language = "ruby"
	if err := recipe.Validate(); err == nil {
		t.Fatal("expected invalid language error")
	}
}
