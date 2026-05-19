package rules

import (
	"testing"
)

// TestGradleNameRegexReturnsRegisteredInstance proves the per-name
// regex registry is honoured: looking up the same name twice yields
// the same *regexp.Regexp pointer, so per-line scans of a Gradle file
// do not recompile the same pattern on every line.
func TestGradleNameRegexReturnsRegisteredInstance(t *testing.T) {
	first := gradleNameRegex(gradleBlockOpenerRegexes, "repositories", "block-opener")
	second := gradleNameRegex(gradleBlockOpenerRegexes, "repositories", "block-opener")
	if first != second {
		t.Fatalf("gradleNameRegex returned distinct instances for the same name; registry miss")
	}
}

func TestGradleNameRegexPanicsOnUnregisteredName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for unregistered name; got none")
		}
	}()
	gradleNameRegex(gradleBlockOpenerRegexes, "definitelyNotRegistered", "block-opener")
}

func TestGradleLineMentionsURLSetterDetectsBothForms(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"url-prop", `url "https://repo.example.com"`, true},
		{"setUrl-method", `setUrl("https://repo.example.com")`, true},
		{"unrelated", `name = "central"`, false},
		{"bare-prefix-only", `urls = [a, b]`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gradleLineMentionsURLSetter(tc.input); got != tc.want {
				t.Errorf("gradleLineMentionsURLSetter(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestGradleLineOpensNamedBlockMatchesKnownGradleNames(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		names []string
		want  bool
	}{
		{"repositories-open", `repositories {`, []string{"repositories"}, true},
		{"maven-with-args", `maven(url: "http://x") {`, []string{"maven", "ivy"}, true},
		{"unrelated-block", `tasks.register("foo") {`, []string{"repositories"}, false},
		{"name-substring-false-positive", `mavenCentralReleases {`, []string{"maven"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gradleLineOpensNamedBlock(tc.line, tc.names...); got != tc.want {
				t.Errorf("gradleLineOpensNamedBlock(%q, %v) = %v, want %v", tc.line, tc.names, got, tc.want)
			}
		})
	}
}

func TestGradleLineHasDirectRepositoryCallIgnoresFunDef(t *testing.T) {
	if gradleLineHasDirectRepositoryCall(`fun jcenter() = mavenCentral()`, "jcenter") {
		t.Errorf("fun-definition must not match a direct call")
	}
	if !gradleLineHasDirectRepositoryCall(`jcenter()`, "jcenter") {
		t.Errorf("direct call must match")
	}
}
