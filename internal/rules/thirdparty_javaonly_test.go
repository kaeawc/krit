package rules_test

import (
	"strings"
	"testing"
)

// An annotation named JavaOnly from another package is not slack-lint-annotations', and Java sources are only read,
// never checked (Lint's ErrorProne analogue covers them).
func TestJavaOnlyDetectorLookalikesAndJavaCallers(t *testing.T) {
	got := runSlackAnnotationRule(t, "JavaOnlyDetector", map[string]string{
		"Fake.kt": `package com.example.fake

annotation class JavaOnly

class Api {
  @JavaOnly fun legacy() = 1
}

fun use(api: Api) = api.legacy()
`,
		"Real.kt": `package com.example

import slack.lint.annotations.JavaOnly

class Api {
  @JavaOnly fun legacy() = 1
}

fun use(api: Api) = api.legacy()
`,
	})
	if strings.Join(got, " ") != "Real.kt:9" {
		t.Errorf("want only the slack-lint-annotations call (Real.kt:9), got %v", got)
	}
}
