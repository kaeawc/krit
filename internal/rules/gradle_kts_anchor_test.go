package rules_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

const anchorBuildKts = `plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android") version "1.9.0"
    id("com.example.convention")
}

apply(plugin = "com.example.convention")

android {
    compileSdk = 34
    defaultConfig {
        minSdk = 14
        targetSdk = 28
        versionName = "1.0"
    }
}

repositories {
    // mavenLocal()
    mavenLocal()
    maven { url = uri("http://repo.example.com/maven") }
}

val libVersion = "1.0"

dependencies {
    // implementation("com.commented:dep:+")
    /* implementation("com.blockcomment:dep:+")
       implementation("com.blockcomment:dyn:1.+") */
    implementation("com.example:lib:+")
    implementation("androidx.appcompat:appcompat:1.0.0")
    implementation(
        "com.android.support:appcompat-v7:28.0.0"
    )
    implementation("com.example:snapshot:1.0-SNAPSHOT")
    implementation("com.example:dyn:1.+")
    implementation("com.example:tmpl:${'$'}libVersion")
    implementation("com.example:tmpl2:${'$'}{libVersion}-SNAPSHOT-not-literal")
    implementation(group = "com.example", name = "named", version = "2.+")
    classpath("com.android.tools.build:gradle:3.0.0")
}

configurations.all {
    resolutionStrategy.force("com.squareup.okhttp3:okhttp:4.12.0")
}
`

// gradleAnchorCases lists, per rule, the source text each expected finding sits
// on. Every other line (comments, templates) must stay quiet.
var gradleAnchorCases = map[string][]string{
	"RemoteVersion":               {`"com.example:lib:+"`},
	"DynamicVersion":              {`"com.example:dyn:1.+"`, `name = "named"`},
	"GradleDynamicVersion":        {`"com.example:lib:+"`, `"com.example:dyn:1.+"`, `name = "named"`}, // Lint counts a bare "+" too
	"OldTargetApi":                {`targetSdk = 28`},
	"ExpiredTargetSdkVersion":     {`targetSdk = 28`},
	"MinSdkTooLow":                {`minSdk = 14`},
	"DeprecatedDependency":        {`"com.android.support:appcompat-v7:28.0.0"`},
	"MavenLocal":                  {"    mavenLocal()"},
	"AndroidGradlePluginVersion":  {`gradle:3.0.0`},
	"NewerVersionAvailable":       {`appcompat:1.0.0`},
	"GradleDependency":            {`appcompat:1.0.0`},
	"DependencySnapshotInRelease": {`1.0-SNAPSHOT"`},
	"DependencyFromHttp":          {`http://repo.example.com`},
	"ApplyPluginTwice":            {`apply(plugin = "com.example.convention")`},
	"ConfigurationsAllSideEffect": {`configurations.all {`},
}

type layoutVariant struct {
	name string
	// shift is the number of lines the rewrite adds before the first original line.
	shift   int
	rewrite func(string) string
}

func layoutVariants() []layoutVariant {
	return []layoutVariant{
		{"lf", 0, func(s string) string { return s }},
		{"crlf", 0, func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }},
		{"tabs", 0, func(s string) string { return strings.ReplaceAll(s, "    ", "\t") }},
		{"header", 3, func(s string) string { return "// header\n/* plugins { id(\"x\") } */\n\n" + s }},
		{"crlf-tabs-header", 3, func(s string) string {
			return strings.ReplaceAll("// header\n// header\n\n"+strings.ReplaceAll(s, "    ", "\t"), "\n", "\r\n")
		}},
	}
}

// lineOf returns the 1-based line holding the first occurrence of token.
func lineOf(text, token string) int {
	i := strings.Index(text, token)
	if i < 0 {
		return -1
	}
	return strings.Count(text[:i], "\n") + 1
}

func TestGradleKtsRuleAnchors(t *testing.T) {
	for ruleID, tokens := range gradleAnchorCases {
		for _, v := range layoutVariants() {
			t.Run(ruleID+"/"+v.name, func(t *testing.T) {
				text := v.rewrite(anchorBuildKts)
				cfg, _ := android.ParseBuildGradleContent(text)
				var want []int
				for _, tok := range tokens {
					line := lineOf(anchorBuildKts, tok)
					if line < 0 {
						t.Fatalf("token %q missing from the base text", tok)
					}
					want = append(want, line+v.shift)
				}
				sort.Ints(want)
				var got []int
				for _, f := range runGradleRule(findGradleRule(t, ruleID), "build.gradle.kts", text, cfg) {
					got = append(got, f.Line)
					// Gradle rules anchor on the line of the offending statement, at column 1.
					if f.Col != 1 {
						t.Errorf("column = %d, want 1", f.Col)
					}
				}
				sort.Ints(got)
				if fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("%s findings on lines %v, want %v", ruleID, got, want)
				}
			})
		}
	}
}

const anchorSettingsKts = `pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenLocal()
    }
}
dependencyResolutionManagement {
    repositories {
        // maven { url = uri("http://commented.example.com") }
        maven { url = uri("http://repo.example.com/maven") }
    }
}
rootProject.name = "demo"
include(":app")
`

func TestGradleKtsSettingsRuleAnchors(t *testing.T) {
	for _, v := range layoutVariants() {
		t.Run(v.name, func(t *testing.T) {
			text := v.rewrite(anchorSettingsKts)
			cfg, _ := android.ParseBuildGradleContent(text)
			for ruleID, token := range map[string]string{
				"DependencyFromHttp": "http://repo.example.com",
				"MavenLocal":         "mavenLocal()",
			} {
				want := lineOf(anchorSettingsKts, token) + v.shift
				var got []int
				for _, f := range runGradleRule(findGradleRule(t, ruleID), "settings.gradle.kts", text, cfg) {
					got = append(got, f.Line)
				}
				if len(got) != 1 || got[0] != want {
					t.Errorf("%s findings on lines %v, want [%d]", ruleID, got, want)
				}
			}
		})
	}
}
