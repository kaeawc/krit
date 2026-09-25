package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These cases pin the declaration relationship that annotation consumers need.
func TestTopLevelAnnotationDeclarationShapes(t *testing.T) {
	cases := []struct {
		name, source, kind, annotation, rawShape string
	}{
		{"last declaration control", "package p\n@Deprecated(\"x\")\nfun old() {}\n", "function_declaration", "Deprecated", "attached"},
		{"following function", "package p\n@Deprecated(\"x\")\nfun old() {}\nfun use() {}\n", "function_declaration", "Deprecated", "split"},
		{"following property", "package p\n@Deprecated(\"x\")\nfun old() {}\nval v = 1\n", "function_declaration", "Deprecated", "split"},
		{"stacked", "package p\n@A(\"x\")\n@B(\"y\")\nfun f() {}\nfun g() {}\n", "function_declaration", "A", "stacked"},
		{"stacked second", "package p\n@A(\"x\")\n@B(\"y\")\nfun f() {}\nfun g() {}\n", "function_declaration", "B", "stacked"},
		{"other modifier", "package p\n@Suppress(\"X\") private fun f() {}\nfun g() {}\n", "function_declaration", "Suppress", "attached"},
		{"KDoc between", "package p\n@A(\"x\")\n/** docs */\nfun f() {}\nfun g() {}\n", "function_declaration", "A", "split"},
		{"class", "package p\n@A(\"x\")\nclass C\nfun g() {}\n", "class_declaration", "A", "attached"},
		{"object", "package p\n@A(\"x\")\nobject O\nfun g() {}\n", "object_declaration", "A", "split"},
		{"interface", "package p\n@A(\"x\")\ninterface I\nfun g() {}\n", "class_declaration", "A", "attached"},
		{"typealias", "package p\n@A(\"x\")\ntypealias Alias = String\nfun g() {}\n", "type_alias", "A", "attached"},
		{"property", "package p\n@A(\"x\")\nval v = 1\nfun g() {}\n", "property_declaration", "A", "attached"},
		{"use site", "package p\n@get:JvmName(\"x\")\nval v = 1\nfun g() {}\n", "property_declaration", "get:JvmName", "attached"},
		{"multi argument control", "package p\n@A(1, 2)\nfun f() {}\nfun g() {}\n", "function_declaration", "A", "attached"},
		{"named arguments control", "package p\n@A(a = 1, b = 2)\nfun f() {}\nfun g() {}\n", "function_declaration", "A", "attached"},
		{"member control", "package p\nclass C { @A(\"x\") fun f() {} }\nfun g() {}\n", "function_declaration", "A", "attached"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, content := parseKotlin(t, tc.source)
			raw := root.String()
			switch tc.rawShape {
			case "split":
				if !strings.Contains(raw, "(prefix_expression (annotation (user_type") || strings.Contains(raw, "(anonymous_function (ERROR") {
					t.Fatalf("unexpected raw split shape: %s", raw)
				}
			case "stacked":
				if !strings.Contains(raw, "(prefix_expression (annotation (constructor_invocation") || !strings.Contains(raw, "(anonymous_function (ERROR (simple_identifier))") {
					t.Fatalf("unexpected raw stacked shape: %s", raw)
				}
			case "attached":
				if strings.Contains(raw, "(prefix_expression (annotation") || !strings.Contains(raw, "(modifiers (annotation") {
					t.Fatalf("unexpected raw attached shape: %s", raw)
				}
			}
			flat := flattenTree(root)
			found := false
			seen := false
			FlatWalkNodes(flat, tc.kind, func(idx uint32) {
				if seen {
					return
				}
				seen = true
				mods, ok := FlatFindChild(flat, idx, "modifiers")
				if ok && strings.Contains(FlatNodeText(flat, mods, content), "@"+tc.annotation) {
					found = true
					wantStart := uint32(strings.Index(tc.source, "@"))
					if flat.StartBytes[idx] != wantStart || flat.StartBytes[mods] != wantStart {
						t.Errorf("normalized declaration/modifiers start = %d/%d, want %d", flat.StartBytes[idx], flat.StartBytes[mods], wantStart)
					}
				}
			})
			if !found {
				t.Errorf("%s annotation is not attached to %s modifiers", tc.annotation, tc.kind)
			}
			if tc.rawShape != "attached" {
				for child := flat.FirstChildren[0]; child != 0; child = flat.NextSibs[child] {
					if nodeTypeName(flat.Types[child]) == "prefix_expression" {
						t.Error("normalized tree still exposes orphan prefix_expression")
					}
				}
			}
		})
	}
}

func TestTopLevelAnnotationCrossFileIndexTestRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Roots.kt")
	if err := os.WriteFile(path, []byte("@Test(\"unit\")\nfun checked() {}\nfun next() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	symbols, _ := indexFile(file)
	for _, symbol := range symbols {
		if symbol.Name == "checked" {
			if !symbol.IsTest {
				t.Error("cross-file index lost @Test from top-level function")
			}
			return
		}
	}
	t.Fatal("checked function absent from index")
}

func TestTopLevelAnnotationFileTargetStaysStandalone(t *testing.T) {
	root, content := parseKotlin(t, "@file:Suppress(\"MagicNumber\")\npackage p\nfun f() {}\nfun g() {}\n")
	if !strings.Contains(root.String(), "(file_annotation") || strings.Contains(root.String(), "(prefix_expression (annotation") {
		t.Fatalf("unexpected raw file-target shape: %s", root.String())
	}
	flat := flattenTree(root)
	var fileAnnotation bool
	FlatWalkNodes(flat, "file_annotation", func(idx uint32) {
		fileAnnotation = strings.Contains(FlatNodeText(flat, idx, content), "@file:Suppress")
	})
	if !fileAnnotation {
		t.Error("file-target annotation must remain a file_annotation")
	}
	FlatWalkNodes(flat, "function_declaration", func(idx uint32) {
		if mods, ok := FlatFindChild(flat, idx, "modifiers"); ok && strings.Contains(FlatNodeText(flat, mods, content), "@file:Suppress") {
			t.Error("file-target annotation must not attach to a function")
		}
	})
}

func TestTopLevelAnnotationNormalizedArgumentAndKDocShape(t *testing.T) {
	src := "package p\n@Deprecated(\"use new\")\n/** docs */\nfun old() {}\nfun next() {}\n"
	root, content := parseKotlin(t, src)
	flat := flattenTree(root)
	var decl uint32
	FlatWalkNodes(flat, "function_declaration", func(idx uint32) {
		if decl == 0 {
			decl = idx
		}
	})
	if decl == 0 {
		t.Fatal("normalized declaration missing")
	}
	mods, ok := FlatFindChild(flat, decl, "modifiers")
	if !ok {
		t.Fatal("modifiers missing")
	}
	ann, ok := FlatFindChild(flat, mods, "annotation")
	if !ok {
		t.Fatal("annotation missing")
	}
	call, ok := FlatFindChild(flat, ann, "constructor_invocation")
	if !ok {
		t.Fatal("constructor_invocation missing")
	}
	args, ok := FlatFindChild(flat, call, "value_arguments")
	if !ok || FlatNodeText(flat, args, content) != "(\"use new\")" {
		t.Fatalf("value arguments = %q", FlatNodeText(flat, args, content))
	}
	arg, ok := FlatFindChild(flat, args, "value_argument")
	if !ok || FlatNodeText(flat, arg, content) != "\"use new\"" {
		t.Fatalf("value argument = %q", FlatNodeText(flat, arg, content))
	}
	comment, ok := FlatFindChild(flat, mods, "multiline_comment")
	if !ok || FlatNodeText(flat, comment, content) != "/** docs */" {
		t.Fatal("KDoc was not retained in the normalized modifiers span")
	}
	if flat.EndBytes[mods] != flat.EndBytes[comment] || flat.EndBytes[decl] != uint32(strings.Index(src, "fun next")-1) {
		t.Error("normalized source ranges changed the declaration or KDoc boundary")
	}
}

func TestTopLevelAnnotationStackedParseErrorRemoved(t *testing.T) {
	root, _ := parseKotlin(t, "@A(\"x\")\n@B(\"y\")\nfun f() {}\nfun g() {}\n")
	if !strings.Contains(root.String(), "(ERROR (simple_identifier))") {
		t.Fatal("raw tree-sitter stacked-annotation recovery shape changed")
	}
	flat := flattenTree(root)
	if flat.Flags[0]&flatNodeFlagError != 0 {
		t.Fatal("normalized root still contains the recovered parser error")
	}
	FlatWalkNodes(flat, "ERROR", func(uint32) { t.Error("normalized tree still exposes parser recovery node") })
}
