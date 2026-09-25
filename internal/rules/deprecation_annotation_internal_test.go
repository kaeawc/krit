package rules

import "testing"

func TestDeprecationAnnotationTypeName(t *testing.T) {
	cases := map[string]string{
		`@Deprecated("x")`:                               "Deprecated",
		`@kotlin.Deprecated("x")`:                        "kotlin.Deprecated",
		`@java.lang.Deprecated`:                          "java.lang.Deprecated",
		`@field:Deprecated("x")`:                         "Deprecated",
		`@Deprecated("see: other")`:                      "Deprecated",
		`@OptIn(DeprecatedForRemovalCompilerApi::class)`: "OptIn",
		`@DeprecatedMarker`:                              "DeprecatedMarker",
	}
	for in, want := range cases {
		if got := deprecationAnnotationTypeName(in); got != want {
			t.Errorf("deprecationAnnotationTypeName(%q) = %q, want %q", in, got, want)
		}
	}
}

// These cases declare the annotated function as a class member. tree-sitter-kotlin
// misparses a top-level declaration whose annotation has a single
// expression-shaped argument (`@Deprecated("x")`, `@OptIn(X::class)`) as a
// prefix_expression statement when another declaration follows it, which would
// leave no modifiers to inspect and make a negative case pass vacuously.
func TestDeprecation_SourcePathMatchesAnnotationNameExactly(t *testing.T) {
	optIn := `package p
@RequiresOptIn
annotation class DeprecatedForRemovalCompilerApi
annotation class DeprecatedMarker
class C {
    @OptIn(DeprecatedForRemovalCompilerApi::class)
    fun helper() {}
    @DeprecatedMarker
    fun marked() {}
    fun use() {
        helper()
        marked()
    }
}
`
	if got := runDeprecation(t, optIn, nil); len(got) != 0 {
		t.Fatalf("annotations that merely contain \"Deprecated\" flagged: %#v", got)
	}

	for _, ann := range []string{`@kotlin.Deprecated("x")`, `@java.lang.Deprecated`, `@Deprecated("x")`, `@Deprecated(message = "x")`} {
		code := "package p\nclass C {\n    " + ann + "\n    fun old() {}\n    fun use() { old() }\n}\n"
		if got := runDeprecation(t, code, nil); len(got) != 1 {
			t.Errorf("%s: findings = %d, want 1", ann, len(got))
		}
	}
}

func TestDeprecation_MessageComesFromTheDeprecatedAnnotationOnly(t *testing.T) {
	code := `package p
annotation class Doc(val message: String)
class C {
    @Doc(message = "documentation text")
    @Deprecated("the real reason")
    fun old() {}
    fun use() { old() }
}
`
	got := runDeprecation(t, code, nil)
	if len(got) != 1 {
		t.Fatalf("findings = %#v, want 1", got)
	}
	if got[0].Message != "'old' is deprecated: the real reason" {
		t.Fatalf("message = %q; the message must come from @Deprecated, not @Doc", got[0].Message)
	}
}
