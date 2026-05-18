package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIndentSizeTabResolvedAfterTabWidth verifies that when a section sets
// both indent_size = tab and tab_width, the resulting IndentSize uses the
// section's tab_width regardless of the order properties appear in the file
// (and regardless of the random map-iteration order applyProps observes).
func TestIndentSizeTabResolvedAfterTabWidth(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantIndent int
		wantTab    int
	}{
		{
			name: "tab_width=8 same section",
			content: `root = true
[*.kt]
indent_size = tab
tab_width = 8
`,
			wantIndent: 8,
			wantTab:    8,
		},
		{
			name: "tab_width=4 same section",
			content: `root = true
[*.kt]
indent_size = tab
tab_width = 4
`,
			wantIndent: 4,
			wantTab:    4,
		},
		{
			// tab_width listed before indent_size: must also resolve correctly.
			name: "tab_width listed first",
			content: `root = true
[*.kt]
tab_width = 6
indent_size = tab
`,
			wantIndent: 6,
			wantTab:    6,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ".editorconfig"), []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			ec := LoadEditorConfig(dir)
			if ec.IndentSize != tc.wantIndent {
				t.Errorf("IndentSize = %d, want %d", ec.IndentSize, tc.wantIndent)
			}
			if ec.TabWidth != tc.wantTab {
				t.Errorf("TabWidth = %d, want %d", ec.TabWidth, tc.wantTab)
			}
		})
	}
}

// TestIndentSizeTabDoesNotLeakAcrossFiles verifies that the resolution of
// `indent_size = tab` does not depend on stale TabWidth state from earlier
// processing in the same merge. Each per-section apply must use the
// tab_width declared in that section.
func TestIndentSizeTabDoesNotLeakAcrossFiles(t *testing.T) {
	// Parent .editorconfig sets a large tab_width but child overrides.
	parent := t.TempDir()
	child := filepath.Join(parent, "sub")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	parentContent := `root = true
[*.kt]
tab_width = 12
indent_size = 2
`
	childContent := `[*.kt]
indent_size = tab
tab_width = 4
`
	if err := os.WriteFile(filepath.Join(parent, ".editorconfig"), []byte(parentContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, ".editorconfig"), []byte(childContent), 0o644); err != nil {
		t.Fatal(err)
	}

	ec := LoadEditorConfig(child)
	if ec.IndentSize != 4 {
		t.Errorf("IndentSize = %d, want 4 (closest tab_width)", ec.IndentSize)
	}
	if ec.TabWidth != 4 {
		t.Errorf("TabWidth = %d, want 4", ec.TabWidth)
	}
}

func TestMatchesKotlin(t *testing.T) {
	cases := []struct {
		section string
		want    bool
	}{
		// Should match
		{"*", true},
		{"*.kt", true},
		{"*.kts", true},
		{"*.{kt,kts}", true},
		{"*.{kt, kts}", true},
		{"{*.kt,*.kts}", true},
		{"*.{java,kt,kts}", true},
		{"*.{kt,java}", true},

		// Should NOT match — substring lookalikes
		{"*.kt.tmpl", false},
		{"hot_keys.cfg", false},
		{"*.ktx", false},
		{"*.cfg", false},
		{"Makefile", false},
		{"*.java", false},
		{"*.xml", false},
		{"package.json", false},
		{"*.{json,yml}", false},
		{"*.ktm", false},
		{"*.gradle.kts.bak", false},
	}

	for _, tc := range cases {
		t.Run(tc.section, func(t *testing.T) {
			got := matchesKotlin(tc.section)
			if got != tc.want {
				t.Errorf("matchesKotlin(%q) = %v, want %v", tc.section, got, tc.want)
			}
		})
	}
}

// TestEditorConfigIgnoresLookalikeSections verifies that lookalike section
// headers do not apply their properties to Kotlin files.
func TestEditorConfigIgnoresLookalikeSections(t *testing.T) {
	dir := t.TempDir()
	content := `root = true
[*.kt.tmpl]
indent_size = 99
max_line_length = 999
[hot_keys.cfg]
indent_size = 77
[*.kt]
indent_size = 4
`
	if err := os.WriteFile(filepath.Join(dir, ".editorconfig"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ec := LoadEditorConfig(dir)
	if ec.IndentSize != 4 {
		t.Errorf("IndentSize = %d, want 4 (lookalike sections must not apply)", ec.IndentSize)
	}
	if ec.MaxLineLength != 0 {
		t.Errorf("MaxLineLength = %d, want 0 (lookalike sections must not apply)", ec.MaxLineLength)
	}
}

// TestEditorConfigCompoundKotlinSection verifies that `[*.{kt,kts}]` applies.
func TestEditorConfigCompoundKotlinSection(t *testing.T) {
	dir := t.TempDir()
	content := `root = true
[*.{kt,kts}]
indent_size = 4
max_line_length = 120
`
	if err := os.WriteFile(filepath.Join(dir, ".editorconfig"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ec := LoadEditorConfig(dir)
	if ec.IndentSize != 4 {
		t.Errorf("IndentSize = %d, want 4", ec.IndentSize)
	}
	if ec.MaxLineLength != 120 {
		t.Errorf("MaxLineLength = %d, want 120", ec.MaxLineLength)
	}
}
