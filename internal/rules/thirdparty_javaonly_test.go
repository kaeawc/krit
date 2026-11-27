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

// #1851's review, fixed in #1881: a nested class implementing a @JavaOnly interface must be annotated too, and a
// call that resolves to an unannotated override is not a call to the @JavaOnly method (the override itself is
// reported). #1881's review (P3): an override is matched by parameter count, so a same-named overload is not one.
func TestJavaOnlyDetectorNestedSubtypesOverridesAndOverloads(t *testing.T) {
	got := runSlackAnnotationRule(t, "JavaOnlyDetector", map[string]string{
		"Api.kt": `package com.example

import slack.lint.annotations.JavaOnly

@JavaOnly
interface Callback {
  fun done()
}

open class Base {
  @JavaOnly open fun on() {}
  open fun on(x: Int) {}
}
`,
		"Use.kt": `package com.example

class Outer {
  class Nested : Callback {
    override fun done() {}
  }
}

class Sub : Base() {
  override fun on() {}
}

class Overloads : Base() {
  override fun on(x: Int) {}
}

fun use(s: Sub, b: Base, o: Overloads) {
  s.on()
  b.on()
  o.on(1)
}
`,
	})
	if strings.Join(got, " ") != "Use.kt:10 Use.kt:19 Use.kt:4" {
		t.Errorf("want the nested subtype (Use.kt:4), the unannotated override (Use.kt:10) and b.on() (Use.kt:19), got %v", got)
	}
}
