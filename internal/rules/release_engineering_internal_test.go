package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestDebugToastPrefixRegex(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{`"debug"`, true},
		{`"DEBUG: clicked"`, true},
		{`'test '`, true},
		{`"wip"`, true},
		{`"debug message"`, true},
		{`"debugger"`, false},
		{`"testing-fixture"`, false},
		{`"wipe"`, false},
		{`"Debugger attached"`, false},
		{`"savedMessage"`, false},
	}
	for _, tc := range cases {
		got := debugToastPrefixRe.MatchString(tc.text)
		if got != tc.want {
			t.Errorf("debugToastPrefixRe.MatchString(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

// TestHasLoggingImportPrefixAnchored regresses bug A: the previous
// implementation used strings.Contains for matching a logging-package
// prefix, so non-logging imports whose path happened to include "mu." or
// "java.util.logging." as a substring were incorrectly classified as
// logging imports.
func TestHasLoggingImportPrefixAnchored(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   bool
	}{
		{
			name: "java.util.logging positive",
			source: "package com.example\n" +
				"\n" +
				"import java.util.logging.Logger\n",
			want: true,
		},
		{
			name: "demu lookalike negative",
			source: "package com.example\n" +
				"\n" +
				"import com.example.demu.LoggerFacade\n",
			want: false,
		},
		{
			name: "foo.mu lookalike negative",
			source: "package com.example\n" +
				"\n" +
				"import foo.mu.X\n",
			want: false,
		},
		{
			name: "java.util.logging substring negative",
			source: "package com.example\n" +
				"\n" +
				"import org.example.java.util.logging.X\n",
			want: false,
		},
		{
			name: "timber positive",
			source: "package com.example\n" +
				"\n" +
				"import timber.log.Timber\n",
			want: true,
		},
		{
			name: "java import with semicolon positive",
			source: "package com.example;\n" +
				"\n" +
				"import java.util.logging.Logger;\n",
			want: true,
		},
		{
			name: "kotlin import with alias positive",
			source: "package com.example\n" +
				"\n" +
				"import java.util.logging.Logger as JLogger\n",
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := &scanner.File{Lines: strings.Split(tc.source, "\n")}
			got := hasLoggingImport(file)
			if got != tc.want {
				t.Fatalf("hasLoggingImport = %v, want %v\nsource:\n%s", got, tc.want, tc.source)
			}
		})
	}
}

// TestReadSourceModuleGradleInfoLexical regresses bug B: a commented-out
// plugin id or a string literal containing "applicationId" must not flip
// a library module's classification to "application".
func TestReadSourceModuleGradleInfoLexical(t *testing.T) {
	cases := []struct {
		name          string
		filename      string
		content       string
		isApplication bool
		isLibrary     bool
	}{
		{
			name:     "kts plugin id is application",
			filename: "build.gradle.kts",
			content: `plugins {
    id("com.android.application")
}
`,
			isApplication: true,
			isLibrary:     false,
		},
		{
			name:     "kts plugin id is library",
			filename: "build.gradle.kts",
			content: `plugins {
    id("com.android.library")
}
`,
			isApplication: false,
			isLibrary:     true,
		},
		{
			name:     "commented classpath does not classify as application",
			filename: "build.gradle",
			content: `// classpath 'com.android.application'

plugins {
    id 'com.android.library'
}
`,
			isApplication: false,
			isLibrary:     true,
		},
		{
			name:     "applicationId inside string description does not classify as application",
			filename: "build.gradle.kts",
			content: `plugins {
    id("com.android.library")
}

description = "applicationId is configured per flavor"
`,
			isApplication: false,
			isLibrary:     true,
		},
		{
			name:     "block-commented application plugin id does not classify as application",
			filename: "build.gradle.kts",
			content: `/*
 * plugins {
 *     id("com.android.application")
 * }
 */
plugins {
    id("com.android.library")
}
`,
			isApplication: false,
			isLibrary:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.filename), []byte(tc.content), 0o644); err != nil {
				t.Fatalf("write %s: %v", tc.filename, err)
			}
			info := readSourceModuleGradleInfo(dir)
			if !info.found {
				t.Fatalf("expected info.found=true for %s", tc.filename)
			}
			if info.isApplication != tc.isApplication {
				t.Errorf("isApplication = %v, want %v", info.isApplication, tc.isApplication)
			}
			if info.isLibrary != tc.isLibrary {
				t.Errorf("isLibrary = %v, want %v", info.isLibrary, tc.isLibrary)
			}
		})
	}
}

// TestGradleStripCommentsFullPreservesStrings asserts that the
// comments-only stripper removes line and block comments but leaves
// string literal bodies intact — Gradle plugin ids live inside string
// literals, and the classifier still needs to see them.
func TestGradleStripCommentsFullPreservesStrings(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		keepToken string
		dropToken string
	}{
		{
			name:      "line comment is stripped, plugin string kept",
			input:     "// id 'com.android.application'\nid 'com.android.library'\n",
			keepToken: "com.android.library",
			dropToken: "com.android.application",
		},
		{
			name:      "block comment is stripped",
			input:     "/* id 'com.android.application' */\nid 'com.android.library'\n",
			keepToken: "com.android.library",
			dropToken: "com.android.application",
		},
		{
			name:      "string body is preserved",
			input:     "id 'com.android.library'\n",
			keepToken: "com.android.library",
			dropToken: "com.android.application",
		},
		{
			name:      "double-slash inside string is not a comment",
			input:     "val url = \"http://example.com\"\nid 'com.android.library'\n",
			keepToken: "http://example.com",
			dropToken: "TOKEN_NOT_PRESENT",
		},
		{
			name:      "block-comment terminator inside raw string does not close a non-existent comment",
			input:     "val s = \"\"\"*/keep_me*/\"\"\"\nid 'com.android.library'\n",
			keepToken: "keep_me",
			dropToken: "TOKEN_NOT_PRESENT",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gradleStripCommentsFull(tc.input)
			if !strings.Contains(got, tc.keepToken) {
				t.Errorf("expected %q to remain in stripped output, got %q", tc.keepToken, got)
			}
			if tc.dropToken != "TOKEN_NOT_PRESENT" && strings.Contains(got, tc.dropToken) {
				t.Errorf("expected %q to be removed from stripped output, got %q", tc.dropToken, got)
			}
		})
	}
}

// TestGradleStripCommentsAndStringBodiesFullBlanksLiterals asserts the
// stricter stripper used for identifier-style classifier tokens: both
// comments and string literal bodies are blanked out so a substring
// like "applicationId" only matches real DSL property references.
func TestGradleStripCommentsAndStringBodiesFullBlanksLiterals(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		keepToken string
		dropToken string
	}{
		{
			name:      "applicationId inside double-quoted string is blanked",
			input:     "description = \"applicationId is configured per flavor\"\napplicationId = \"com.example.app\"\n",
			keepToken: "description",
			dropToken: "configured",
		},
		{
			name:      "applicationId inside single-quoted string is blanked",
			input:     "description = 'applicationId is configured'\n",
			keepToken: "description",
			dropToken: "applicationId",
		},
		{
			name:      "applicationId inside triple-quoted string is blanked",
			input:     "val s = \"\"\"applicationId is configured\"\"\"\n",
			keepToken: "val s",
			dropToken: "applicationId",
		},
		{
			name:      "escaped quote keeps lexer inside literal",
			input:     "val s = \"applicationId is \\\"x\\\"\"\n",
			keepToken: "val s",
			dropToken: "applicationId",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gradleStripCommentsAndStringBodiesFull(tc.input)
			if !strings.Contains(got, tc.keepToken) {
				t.Errorf("expected %q to remain in stripped output, got %q", tc.keepToken, got)
			}
			if strings.Contains(got, tc.dropToken) {
				t.Errorf("expected %q to be removed from stripped output, got %q", tc.dropToken, got)
			}
		})
	}
}

// TestGradleContainsCodeToken covers the word-boundary lookup used to
// confirm an Android DSL property identifier appears outside string
// literals. Inputs are pre-stripped by
// gradleStripCommentsAndStringBodiesFull so this test feeds already-
// stripped content.
func TestGradleContainsCodeToken(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		needle string
		want   bool
	}{
		{"bare property assignment", "applicationId = \"x\"", "applicationId", true},
		{"trailing call paren", "applicationId(\"x\")", "applicationId", true},
		{"property at end of file", "applicationId", "applicationId", true},
		{"longer identifier rejected", "myApplicationIdSuffix", "applicationId", false},
		{"prefix-only identifier rejected", "applicationIdentity = 1", "applicationId", false},
		{"completely absent", "id = 1", "applicationId", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gradleContainsCodeToken(tc.input, tc.needle); got != tc.want {
				t.Fatalf("gradleContainsCodeToken(%q, %q) = %v, want %v", tc.input, tc.needle, got, tc.want)
			}
		})
	}
}
