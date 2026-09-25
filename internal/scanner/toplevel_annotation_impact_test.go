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
		{"line comment between", "package p\n@Deprecated(\"x\") // TODO\nfun old() {}\nfun use() {}\n", "function_declaration", "Deprecated", "split"},
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

func TestTopLevelAnnotationUnrelatedSyntaxErrorRetained(t *testing.T) {
	src := "@A(\"x\")\nfun f() {}\nfun g() {}\nfun broken( {}\n"
	root, _ := parseKotlin(t, src)
	if !strings.Contains(root.String(), "(prefix_expression (annotation") {
		t.Fatalf("expected split annotation, raw tree: %s", root.String())
	}
	flat := flattenTree(root)
	var recovery uint32
	for idx := range flat.Types {
		if flat.Node(uint32(idx)).IsErrorNode() && flat.ChildCounts[idx] == 0 {
			recovery = uint32(idx)
			break
		}
	}
	if recovery == 0 {
		t.Fatalf("expected leaf ERROR/MISSING node, raw tree: %s", root.String())
	}
	if !flat.Node(recovery).HasError() {
		t.Errorf("leaf recovery node %s at byte %d lost HasError", flat.Node(recovery).TypeName(), flat.StartBytes[recovery])
	}
	if !flat.Node(0).HasError() {
		t.Error("source_file lost HasError from unrelated syntax error")
	}
}

func TestTopLevelAnnotationBareOuterParenthesizedInner(t *testing.T) {
	src := "@A\n@B(\"y\")\nfun f() {}\nfun g() {}\n"
	root, content := parseKotlin(t, src)
	raw := root.String()
	const wantRaw = "(source_file (prefix_expression (annotation (user_type (type_identifier))) (prefix_expression (annotation (user_type (type_identifier))) (parenthesized_expression (string_literal (string_content))))) (function_declaration (simple_identifier) (function_value_parameters) (function_body)) (function_declaration (simple_identifier) (function_value_parameters) (function_body)))"
	if raw != wantRaw {
		t.Fatalf("unexpected raw stacked split shape: %s", raw)
	}
	flat := flattenTree(root)
	var decl uint32
	FlatWalkNodes(flat, "function_declaration", func(idx uint32) {
		if decl == 0 {
			decl = idx
		}
	})
	if decl == 0 {
		t.Fatal("normalized function declaration missing")
	}
	mods, ok := FlatFindChild(flat, decl, "modifiers")
	if !ok {
		t.Fatal("normalized modifiers missing")
	}
	var anns []uint32
	for child := flat.FirstChildren[mods]; child != 0; child = flat.NextSibs[child] {
		if nodeTypeName(flat.Types[child]) == "annotation" {
			anns = append(anns, child)
		}
	}
	if len(anns) != 2 {
		t.Fatalf("normalized annotation count = %d, want 2; raw tree: %s", len(anns), raw)
	}
	if got := FlatNodeText(flat, anns[0], content); got != "@A" {
		t.Errorf("outer annotation = %q, want bare @A", got)
	}
	if _, ok := FlatFindChild(flat, anns[0], "constructor_invocation"); ok {
		t.Error("bare @A unexpectedly has a constructor invocation")
	}
	if got := FlatNodeText(flat, anns[1], content); got != "@B(\"y\")" {
		t.Errorf("inner annotation = %q, want @B(\"y\")", got)
	}
	call, ok := FlatFindChild(flat, anns[1], "constructor_invocation")
	if !ok {
		t.Fatal("@B constructor invocation missing")
	}
	args, ok := FlatFindChild(flat, call, "value_arguments")
	if !ok || FlatNodeText(flat, args, content) != "(\"y\")" {
		t.Errorf("@B value arguments = %q, want (\"y\")", FlatNodeText(flat, args, content))
	}
}

func TestTopLevelAnnotationNestedFunctionRecoveryLimit(t *testing.T) {
	for _, tc := range []struct{ name, first string }{
		{"receiver", "user_type"},
		{"type parameters", "type_parameters"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decl := &annotationTreeNode{typ: "anonymous_function", children: []*annotationTreeNode{
				{typ: "fun"},
				{typ: tc.first},
				{typ: "ERROR", children: []*annotationTreeNode{{typ: "simple_identifier"}}},
			}}
			if repairTopLevelAnonymousFunction(decl) {
				t.Error("repaired unsupported anonymous_function recovery shape")
			}
			if decl.typ != "anonymous_function" || decl.children[1].typ != tc.first {
				t.Error("unsupported recovery shape was mutated")
			}
			if target, _ := topLevelAnnotationTarget(nil, 0, decl); target != nil {
				t.Error("unsupported nested recovery matched a declaration target")
			}
		})
	}
}
