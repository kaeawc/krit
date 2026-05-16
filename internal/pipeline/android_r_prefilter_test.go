package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestMayReferenceAndroidResources pins the byte-pattern semantics of
// the resource-source-rule prefilter. False matches (e.g. "PARSER." or
// strings inside comments) are intentional — the rule layer remains
// the authoritative check; the prefilter only exists to skip files
// that demonstrably can't have an Android-R reference (no "R." pair
// anywhere in their content).
func TestMayReferenceAndroidResources(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", false},
		{"plain text no R", "fun greet() = \"hello\"", false},
		{"reference to R.string", "val s = R.string.foo", true},
		{"reference to R.layout", "setContentView(R.layout.main)", true},
		{"reference to R.drawable", "imageView.setImageResource(R.drawable.icon)", true},
		{"import statement only", "import com.app.R\n", false},
		{"import + use", "import com.app.R\nval s = R.string.foo", true},
		{"false-positive via identifier", "val PARSER. = 1\n", true},
		{"comment mentioning R.", "// uses R.string.foo somewhere\nfun greet() = \"\"", true},
		{"unicode without R", "val π = 3.14", false},
		{"multiline Kotlin source no R", "package com.demo\nfun a() = 1\nfun b() = 2\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &scanner.File{Path: "Foo.kt", Content: []byte(tt.content)}
			if got := mayReferenceAndroidResources(file); got != tt.want {
				t.Errorf("mayReferenceAndroidResources(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

// TestMayReferenceAndroidResources_NilSafety guards the common
// "scanner.File without content" case the dispatcher uses for
// skeleton entries.
func TestMayReferenceAndroidResources_NilSafety(t *testing.T) {
	if mayReferenceAndroidResources(nil) {
		t.Errorf("nil file: want false")
	}
	if mayReferenceAndroidResources(&scanner.File{Path: "Foo.kt"}) {
		t.Errorf("file with nil Content: want false")
	}
}
