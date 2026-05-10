package rules

import (
	"path/filepath"
	"testing"
)

func TestIsPlausibleCommentedKotlin(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"prose", "// just describing what this does", false},
		{"non comment", "fun foo() = 1", false},
		{"empty after slashes", "//   ", false},
		{"trailing brace", "// fun foo() {", true},
		{"close brace", "// }", true},
		{"semicolon", "// doStuff();", true},
		{"val keyword", "// val x = 1", true},
		{"var keyword", "// var x = 1", true},
		{"fun keyword", "// fun bar(x: Int)", true},
		{"if keyword", "// if (x > 0) doStuff()", true},
		{"when keyword", "// when (x) { 0 -> y }", true},
		{"return keyword", "// return result", true},
		{"assignment", "// foo = 42", true},
		{"equality is not assignment", "// x == y means equal", false},
		{"call expression", "// doStuff(arg)", true},
		{"non-greedy regex on prose", "// see http://example.com for details", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPlausibleCommentedKotlin(tc.line); got != tc.want {
				t.Fatalf("isPlausibleCommentedKotlin(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestConventionPluginID(t *testing.T) {
	root := filepath.FromSlash("/repo/build-logic/src/main/kotlin")
	cases := []struct {
		name string
		path string
		want string
	}{
		{"flat .gradle.kts", filepath.Join(root, "android.application.gradle.kts"), "android.application"},
		{"nested .gradle.kts", filepath.Join(root, "ci", "lint.gradle.kts"), "ci.lint"},
		{"flat .gradle", filepath.Join(root, "java.library.gradle"), "java.library"},
		{"non gradle file", filepath.Join(root, "some", "Code.kt"), ""},
		{"empty after trim", filepath.Join(root, ".gradle.kts"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := conventionPluginID(root, tc.path); got != tc.want {
				t.Fatalf("conventionPluginID(%q, %q) = %q, want %q", root, tc.path, got, tc.want)
			}
		})
	}
}

func TestIsGradleBuildScript(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"build.gradle", true},
		{"foo/build.gradle.kts", true},
		{"foo/settings.gradle.kts", false},
		{"foo/Bar.kt", false},
		{"build.gradle.kts.bak", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isGradleBuildScript(tc.path); got != tc.want {
				t.Fatalf("isGradleBuildScript(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestIsEnvironmentConfigCallName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"", false},
		{"loadEnvironment", true},
		{"buildConfig", true},
		{"setupEnv", true},
		{"unrelated", false},
		{"populateUser", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isEnvironmentConfigCallName(tc.name); got != tc.want {
				t.Fatalf("isEnvironmentConfigCallName(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestHardcodedEnvironmentLiteral(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		label bool
		want  string
	}{
		{"plain prod", `"prod"`, false, "prod"},
		{"upper case staging", `"STAGING"`, false, "STAGING"},
		{"unknown", `"production-east"`, false, ""},
		{"with label", `name = "qa"`, true, "qa"},
		{"with label but no =", `"qa"`, true, "qa"},
		{"unquoted", `prod`, false, ""},
		{"empty", "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hardcodedEnvironmentLiteral(tc.text, tc.label); got != tc.want {
				t.Fatalf("hardcodedEnvironmentLiteral(%q, %v) = %q, want %q", tc.text, tc.label, got, tc.want)
			}
		})
	}
}

func TestStripBlockComments(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		inBlock   bool
		want      string
		stillOpen bool
	}{
		{"no comment", "val x = 1", false, "val x = 1", false},
		{"open and close on same line", "val x /* commented */ = 1", false, "val x  = 1", false},
		{"open without close", "val x /* unclosed", false, "val x ", true},
		{"continuation closes", " still in block */ val y = 2", true, " val y = 2", false},
		{"continuation no close", " still in block", true, "", true},
		{"multiple blocks", "/* a */ val /* b */ x = 1", false, " val  x = 1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, open := stripBlockComments(tc.line, tc.inBlock)
			if got != tc.want || open != tc.stillOpen {
				t.Fatalf("stripBlockComments(%q, %v) = (%q, %v), want (%q, %v)",
					tc.line, tc.inBlock, got, open, tc.want, tc.stillOpen)
			}
		})
	}
}

func TestIsDebugSourceFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"app/src/debug/java/Foo.kt", true},
		{"app/src/main/java/Foo.kt", false},
		{"a/debug/Foo.kt", true},
		{"main/Foo.kt", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isDebugSourceFile(tc.path); got != tc.want {
				t.Fatalf("isDebugSourceFile(%q) = %v", tc.path, got)
			}
		})
	}
}

func TestIsAndroidTestSupportArtifactSource(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"app/src/main/Foo.kt", false},
		{"app/test-fixtures/Foo.kt", true},
		{"app/src/testFixtures/Foo.kt", false}, // testFixtures is camelCase; isAndroidTestSupportArtifactSource lowercases first
		{"app/fakes/Foo.kt", true},
		{"app/Mocks/Foo.kt", true},
		{"app/idling-resources/Foo.kt", true},
		{"app/instrumentation-tests/Foo.kt", true},
		{"app/EspressoModule.kt", true},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := isAndroidTestSupportArtifactSource(tc.path)
			// for the testFixtures case, lowercased path becomes
			// "app/src/testfixtures/foo.kt", which IS in the marker list
			// ("/testfixtures/"). Adjust expectation.
			if tc.path == "app/src/testFixtures/Foo.kt" {
				tc.want = true
			}
			if got != tc.want {
				t.Fatalf("isAndroidTestSupportArtifactSource(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestCompactSourceReference(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"already compact", "Foo.bar", "Foo.bar"},
		{"trims edges", "  Foo.bar  ", "Foo.bar"},
		{"strips spaces", "Foo . bar", "Foo.bar"},
		{"strips tabs", "Foo\t.bar", "Foo.bar"},
		{"strips newlines", "Foo\n.bar\r\n.baz", "Foo.bar.baz"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compactSourceReference(tc.in); got != tc.want {
				t.Fatalf("compactSourceReference(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestQualifySourceName(t *testing.T) {
	cases := []struct {
		pkg, name, want string
	}{
		{"", "Foo", "Foo"},
		{"com.example", "Foo", "com.example.Foo"},
		{"com.example.sub", "Bar", "com.example.sub.Bar"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := qualifySourceName(tc.pkg, tc.name); got != tc.want {
				t.Fatalf("qualifySourceName(%q, %q) = %q, want %q", tc.pkg, tc.name, got, tc.want)
			}
		})
	}
}

func TestSimpleSourceName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"com.example.Foo", "Foo"},
		{"Foo", "Foo"},
		{"", ""},
		{"a.b.c.d", "d"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := simpleSourceName(tc.in); got != tc.want {
				t.Fatalf("simpleSourceName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsTestFixturePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"app/src/testFixtures/Foo.kt", true},
		{"app/src/main/Foo.kt", false},
		{filepath.FromSlash("app/src/testFixtures/Foo.kt"), true},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isTestFixturePath(tc.path); got != tc.want {
				t.Fatalf("isTestFixturePath(%q) = %v", tc.path, got)
			}
		})
	}
}

func TestIsGeneratedSourcePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"app/build/generated/foo.kt", true},
		{"app/generated/foo.kt", true},
		{"app/build/ksp/foo.kt", true},
		{"app/build/kapt/stubs/foo.kt", true},
		{"app/src/main/foo.kt", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isGeneratedSourcePath(tc.path); got != tc.want {
				t.Fatalf("isGeneratedSourcePath(%q) = %v", tc.path, got)
			}
		})
	}
}

func TestParseSourceImport(t *testing.T) {
	cases := []struct {
		name         string
		text         string
		wantQN       string
		wantLocal    string
		wantWildcard bool
	}{
		{"plain", "import com.example.Foo", "com.example.Foo", "Foo", false},
		{"plain trailing semicolon", "import com.example.Foo;", "com.example.Foo", "Foo", false},
		{"alias", "import com.example.Foo as Bar", "com.example.Foo", "Bar", false},
		{"wildcard", "import com.example.*", "com.example", "", true},
		{"java static", "import static com.example.Foo.bar", "com.example.Foo.bar", "bar", false},
		{"not import", "package com.example", "", "", false},
		{"empty body", "import   ", "", "", false},
		// Regression for #114: tree-sitter trailing trivia (a leading
		// block comment attached to the import_header node) must not
		// fool the import-keyword gate. Pre-fix returned ("", "", false).
		{"leading block comment trivia", "/* doc */\nimport com.example.Foo", "com.example.Foo", "Foo", false},
		// Pre-fix this passed because TrimSpace + TrimPrefix("import")
		// happened to handle the leading newline; included to lock the
		// behavior in.
		{"leading newline", "\nimport com.example.Foo", "com.example.Foo", "Foo", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qn, local, wild := parseSourceImport(tc.text)
			if qn != tc.wantQN || local != tc.wantLocal || wild != tc.wantWildcard {
				t.Fatalf("parseSourceImport(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.text, qn, local, wild, tc.wantQN, tc.wantLocal, tc.wantWildcard)
			}
		})
	}
}

func TestTimberSimpleName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"timber.log.Timber.d", "d"},
		{"timber.log.Timber#d", "d"},
		{"  Timber.plant  ", "plant"},
		{"plant", "plant"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := timberSimpleName(tc.in); got != tc.want {
				t.Fatalf("timberSimpleName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
