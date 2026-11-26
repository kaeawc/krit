package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
)

// Lint runs these checks only when the project depends on the artifact whose AAR ships them, so krit reports
// nothing without a declared dependency, even on a plain framework WebViewClient.
func TestAndroidxWebkitRulesRequireTheirDependency(t *testing.T) {
	webkit := librarymodel.ProjectProfile{Dependencies: []librarymodel.Dependency{{Group: "androidx.webkit", Name: "webkit", Version: "1.18.0"}}}
	uiautomator := librarymodel.ProjectProfile{Dependencies: []librarymodel.Dependency{
		{Group: "androidx.test.uiautomator", Name: "uiautomator", Version: "2.4.1"}}}
	for _, tc := range []struct {
		rule, src string
		profile   librarymodel.ProjectProfile
	}{
		{"MissingOnRenderProcessGone", `package com.example

import android.webkit.WebViewClient

class BrowserClient : WebViewClient()
`, webkit},
		{"CrossOriginIsolatedAllowlist", `package com.example

import androidx.webkit.Profile

fun allow(profile: Profile) {
    profile.setCrossOriginIsolatedAllowlist(setOf("https://example.com"))
}
`, webkit},
		{"DontUseAccessibilityNodeInfoGetText", `package com.example

import androidx.test.uiautomator.uiAutomator

fun login() = uiAutomator {
    onElement { text == "Login" }.click()
}
`, uiautomator},
	} {
		files := map[string]string{"src/main/java/com/example/A.kt": tc.src}
		if got := activityProjectFindings(t, tc.rule, "src/main/java/com/example/A.kt", files, tc.profile); len(got) != 1 {
			t.Errorf("%s with the dependency: got %q, want one finding", tc.rule, got)
		}
		if got := activityProjectFindings(t, tc.rule, "src/main/java/com/example/A.kt", files, librarymodel.ProjectProfile{}); len(got) != 0 {
			t.Errorf("%s without the dependency: got %q, want none", tc.rule, got)
		}
	}
}

// A named-argument label is not a read of AccessibilityNodeInfo.text; Lint's visitor only sees name references.
func TestDontUseAccessibilityNodeInfoGetTextSkipsArgumentLabels(t *testing.T) {
	uiautomator := librarymodel.ProjectProfile{Dependencies: []librarymodel.Dependency{
		{Group: "androidx.test.uiautomator", Name: "uiautomator", Version: "2.4.1"}}}
	src := `package com.example

import androidx.test.uiautomator.uiAutomator

fun matches(text: String): Boolean = text.isNotEmpty()

fun login() = uiAutomator {
    onElement { matches(text = "Hello") }.click()
    onElement { matches(text = text.toString()) }.click()
}
`
	files := map[string]string{"src/main/java/com/example/A.kt": src}
	got := activityProjectFindings(t, "DontUseAccessibilityNodeInfoGetText", "src/main/java/com/example/A.kt", files, uiautomator)
	if want := "9:32 Do not use AccessibilityNodeInfo#getText in a UiAutomatorTestScope."; len(got) != 1 || got[0] != want {
		t.Fatalf("got %q, want only the read in the argument value, %q", got, want)
	}
}

// UiAutomator is usually a test dependency; a test configuration still declares it.
func TestUiautomatorTestDependencyCounts(t *testing.T) {
	profile := librarymodel.ProfileFromGradleContent("build.gradle",
		"dependencies {\n    androidTestImplementation 'androidx.test.uiautomator:uiautomator:2.4.1'\n}\n")
	if !profile.HasAnyDependency(librarymodel.Coordinate{Group: "androidx.test.uiautomator", Name: "uiautomator"}) {
		t.Fatalf("androidTestImplementation uiautomator not in profile: %+v", profile.Dependencies)
	}
}
