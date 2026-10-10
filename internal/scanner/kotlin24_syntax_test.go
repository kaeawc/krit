package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func parseKotlinSnippet(t *testing.T, src string) *File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "S.kt")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func errorSpans(f *File) []string {
	var spans []string
	src := string(f.Content)
	for idx, flags := range f.FlatTree.Flags {
		if flags&flatNodeFlagIsError != 0 {
			spans = append(spans, src[f.FlatTree.StartBytes[idx]:f.FlatTree.EndBytes[idx]])
		}
	}
	return spans
}

func hasNamedNode(f *File, nodeType, text string) bool {
	src := string(f.Content)
	for idx := range f.FlatTree.Types {
		if f.FlatType(uint32(idx)) == nodeType && src[f.FlatTree.StartBytes[idx]:f.FlatTree.EndBytes[idx]] == text {
			return true
		}
	}
	return false
}

// TestKotlin24SyntaxSupport records how the pinned tree-sitter Kotlin grammar
// (2024) handles syntax added up to Kotlin 2.4 (#785). Constructs it predates
// produce ERROR/MISSING nodes; krit then drops only findings anchored inside
// those spans (FilterColumnsByErrorRegions), so each case also asserts that
// the error stays local and the enclosing declaration is still indexed. When
// the grammar is upgraded and a case starts parsing cleanly, update its
// expectation and docs/kotlin-syntax-support.md.
func TestKotlin24SyntaxSupport(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		errors    []string // exact ERROR/MISSING spans, nil when clean
		survivors [][2]string
	}{
		{
			name:      "context receiver (pre-2.2)",
			src:       "class Logger\ncontext(Logger)\nfun log() { }\n",
			survivors: [][2]string{{"function_declaration", "fun log() { }"}},
		},
		{
			name:      "named context parameter",
			src:       "class Logger\ncontext(logger: Logger)\nfun log() { }\n",
			errors:    []string{": Logger"},
			survivors: [][2]string{{"function_declaration", "fun log() { }"}},
		},
		{
			name:      "anonymous context parameter",
			src:       "class Logger\ncontext(_: Logger)\nfun log() { }\n",
			errors:    []string{": Logger"},
			survivors: [][2]string{{"function_declaration", "fun log() { }"}},
		},
		{
			name:      "context parameter on a property",
			src:       "class Users\ncontext(users: Users)\nval first: Int get() = 1\n",
			errors:    []string{": Users"},
			survivors: [][2]string{{"property_declaration", "val first: Int get() = 1"}},
		},
		{
			name:      "explicit backing field",
			src:       "class Counter {\n  val items: List<Int>\n    field = mutableListOf()\n}\n",
			errors:    []string{""}, // the MISSING `val` of the recovered `field` line
			survivors: [][2]string{{"property_declaration", "val items: List<Int>"}},
		},
		{
			name:      "@all meta-target",
			src:       "annotation class Email\nclass User(@all:Email val email: String)\n",
			errors:    []string{":Email"},
			survivors: [][2]string{{"class_parameter", "@all:Email val email: String"}},
		},
		{
			name:      "@param use-site target",
			src:       "annotation class Email\nclass User(@param:Email val email: String)\n",
			survivors: [][2]string{{"annotation", "@param:Email"}},
		},
		{
			name:      "when guard",
			src:       "fun f(x: Any) = when (x) {\n  is Int if x > 0 -> 1\n  else -> 0\n}\n",
			errors:    []string{"if x > 0"},
			survivors: [][2]string{{"when_entry", "else -> 0"}},
		},
		{
			name: "nested type alias",
			src:  "class Outer {\n  typealias Id = String\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := parseKotlinSnippet(t, tc.src)
			got := errorSpans(f)
			if len(got) != len(tc.errors) {
				t.Fatalf("error spans = %q, want %q", got, tc.errors)
			}
			for i := range got {
				if got[i] != tc.errors[i] {
					t.Fatalf("error spans = %q, want %q", got, tc.errors)
				}
			}
			for _, s := range tc.survivors {
				if !hasNamedNode(f, s[0], s[1]) {
					t.Errorf("%s %q not indexed", s[0], s[1])
				}
			}
		})
	}
}

// The recovered `field = ...` line must not look like a property named
// `field`, or declaration rules report it (UndocumentedPublicProperty,
// DeadCode).
func TestExplicitBackingFieldIsNotAProperty(t *testing.T) {
	f := parseKotlinSnippet(t, "class Counter {\n  val items: List<Int>\n    field = mutableListOf()\n  val other = 1\n}\n")
	var props []string
	for _, idx := range f.FlatTree.NodesOfType(internNodeType("property_declaration")) {
		props = append(props, f.FlatNodeText(idx))
	}
	if len(props) != 2 || props[0] != "val items: List<Int>" || props[1] != "val other = 1" {
		t.Fatalf("property declarations = %q", props)
	}
	if n := len(f.FlatTree.NodesOfType(internNodeType(ExplicitBackingFieldNodeType))); n != 1 {
		t.Fatalf("explicit_backing_field nodes = %d, want 1", n)
	}
}

// Error recovery that only resembles a backing field keeps its shape: the
// name must be `field` and it must follow a property.
func TestBackingFieldNormalizationIsNarrow(t *testing.T) {
	f := parseKotlinSnippet(t, "class C {\n  fun f() = 1\n  other = 2\n}\n")
	if n := len(f.FlatTree.NodesOfType(internNodeType(ExplicitBackingFieldNodeType))); n != 0 {
		t.Fatalf("explicit_backing_field nodes = %d, want 0", n)
	}
}
