package scanner

import (
	"context"
	"strings"
	"testing"
)

func parseSynthetic(t *testing.T, src string) (*FlatTree, []byte) {
	t.Helper()
	parser := GetKotlinParser()
	defer PutKotlinParser(parser)
	content := []byte(src)
	f, err := ParseKotlinFile(context.Background(), parser, "Synthetic.kt", content)
	if err != nil {
		t.Fatal(err)
	}
	return f.FlatTree, content
}

func topLevelTypes(ft *FlatTree) []string {
	var out []string
	for c := ft.FirstChildren[0]; c != 0; c = ft.NextSibs[c] {
		out = append(out, nodeTypeName(ft.Types[c]))
	}
	return out
}

func childOfType(ft *FlatTree, parent uint32, typ string) uint32 {
	for c := ft.FirstChildren[parent]; c != 0; c = ft.NextSibs[c] {
		if nodeTypeName(ft.Types[c]) == typ {
			return c
		}
	}
	return 0
}

func topLevelOfType(ft *FlatTree, typ string, n int) uint32 {
	for c := ft.FirstChildren[0]; c != 0; c = ft.NextSibs[c] {
		if nodeTypeName(ft.Types[c]) == typ {
			if n == 0 {
				return c
			}
			n--
		}
	}
	return 0
}

func hasMisparse(ft *FlatTree) bool {
	for c := ft.FirstChildren[0]; c != 0; c = ft.NextSibs[c] {
		switch nodeTypeName(ft.Types[c]) {
		case "prefix_expression", "infix_expression", "ERROR":
			return true
		}
	}
	return false
}

func TestCtorOnNextLineParsesClassBody(t *testing.T) {
	cases := map[string]string{
		"inject":           "class Foo\n@Inject\nconstructor(private val a: A, private val b: B) : Base() {\n  fun x() {}\n}\n",
		"optIn":            "class Foo\n@OptIn(\n  ExperimentalA::class,\n  ExperimentalB::class,\n)\nconstructor(private val a: A) : Base() {\n  fun x() {}\n}\n",
		"private":          "class Foo\nprivate constructor(private val a: A) : Base() {\n  fun x() {}\n}\n",
		"annotatedPrivate": "class Foo\n@Inject\nprivate constructor(private val a: A) : Base() {\n  fun x() {}\n}\n",
		"data":             "data class Foo\n@OptIn(ExperimentalA::class)\nconstructor(val a: A) {\n  fun x() {}\n}\n",
		"generic":          "class Foo<T : Any>\n@Inject\nconstructor(val a: T) {\n  fun x() {}\n}\n",
		"blank":            "class Foo\n\n@Inject\nconstructor(val a: A) {\n  fun x() {}\n}\n",
		"crlf":             "class Foo\r\n@Inject\r\nconstructor(val a: A) {\r\n  fun x() {}\r\n}\r\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			src := body + "\n@Composable\ninternal fun Later() {}\n"
			ft, content := parseSynthetic(t, src)
			if got := topLevelTypes(ft); strings.Join(got, ",") != "class_declaration,function_declaration" {
				t.Fatalf("top level = %v", got)
			}
			cls := topLevelOfType(ft, "class_declaration", 0)
			if childOfType(ft, cls, "class_body") == 0 {
				t.Fatal("class has no class_body")
			}
			fn := topLevelOfType(ft, "function_declaration", 0)
			mods := childOfType(ft, fn, "modifiers")
			if mods == 0 || childOfType(ft, mods, "annotation") == 0 || childOfType(ft, mods, "visibility_modifier") == 0 {
				t.Fatal("later function lost its modifiers")
			}
			off := strings.Index(src, "@Composable")
			if int(ft.StartBytes[fn]) != off {
				t.Fatalf("function start byte %d, want %d", ft.StartBytes[fn], off)
			}
			wantRow := strings.Count(src[:off], "\n")
			if int(ft.StartRows[fn]) != wantRow || ft.StartCols[fn] != 0 {
				t.Fatalf("function at %d:%d, want %d:0", ft.StartRows[fn], ft.StartCols[fn], wantRow)
			}
			if string(content[ft.StartBytes[cls]:ft.EndBytes[cls]]) != strings.TrimSpace(body) && !strings.HasPrefix(body, string(content[ft.StartBytes[cls]:ft.EndBytes[cls]])) {
				t.Fatalf("class span does not match source")
			}
		})
	}
}

func TestCtorOnNextLinePositionsOfInnerNodes(t *testing.T) {
	src := "class Foo\n@Inject\nconstructor(private val a: A) {\n  fun target() {}\n}\n"
	ft, content := parseSynthetic(t, src)
	cls := topLevelOfType(ft, "class_declaration", 0)
	body := childOfType(ft, cls, "class_body")
	fn := childOfType(ft, body, "function_declaration")
	if fn == 0 {
		t.Fatal("no member function")
	}
	if ft.StartRows[fn] != 3 || ft.StartCols[fn] != 2 {
		t.Fatalf("member at %d:%d, want 3:2", ft.StartRows[fn], ft.StartCols[fn])
	}
	if got := string(content[ft.StartBytes[fn]:ft.EndBytes[fn]]); got != "fun target() {}" {
		t.Fatalf("member span = %q", got)
	}
}

func TestCtorOnNextLineNestedClass(t *testing.T) {
	src := "class Outer {\n  class Inner\n  @Inject\n  constructor(val a: A) {\n    fun x() {}\n  }\n\n  fun after() {}\n}\n\nfun top() {}\n"
	ft, _ := parseSynthetic(t, src)
	if hasMisparse(ft) {
		t.Fatalf("top level = %v", topLevelTypes(ft))
	}
	if got := topLevelTypes(ft); strings.Join(got, ",") != "class_declaration,function_declaration" {
		t.Fatalf("top level = %v", got)
	}
}

func TestSecondaryConstructorInBodyUntouched(t *testing.T) {
	src := "class Foo(val a: Int) {\n  val x = 1\n  @Inject\n  constructor() : this(1)\n}\n"
	if got := ctorGapBreaks([]byte(src)); got != nil {
		t.Fatalf("unexpected rewrite %v", got)
	}
	src2 := "class Foo // trailing\n@Inject\nconstructor(val a: A) {}\n"
	if got := ctorGapBreaks([]byte(src2)); got != nil {
		t.Fatalf("comment gap rewritten %v", got)
	}
}

func TestRewriteKeepsLength(t *testing.T) {
	src := []byte("class Foo\r\n@Inject\r\nconstructor(val a: A) {}\r\n")
	breaks := ctorGapBreaks(src)
	if len(breaks) == 0 {
		t.Fatal("expected a rewrite")
	}
	if out := blankBreaks(src, breaks); len(out) != len(src) {
		t.Fatalf("length changed: %d -> %d", len(src), len(out))
	}
}

func TestMultiAnnotationPreviewFunctionsAreDeclarations(t *testing.T) {
	cases := []string{
		"@PreviewLightDark\n@Composable\ninternal fun PreviewOne() {\n  Body()\n}\n",
		"@Preview(name = \"a\")\n@Composable\nprivate fun PreviewOne() {\n  Body()\n}\n",
		"@A\n@B\n@C\ninternal fun PreviewOne() {\n  Body()\n}\n",
	}
	for _, fn := range cases {
		src := "import a.B\n\nfun first() {\n  call()\n}\n\n" + fn + "\nfun last() {}\n"
		ft, content := parseSynthetic(t, src)
		if got := strings.Join(topLevelTypes(ft), ","); got != "import_list,function_declaration,function_declaration,function_declaration" {
			t.Fatalf("%q: top level = %s", fn, got)
		}
		d := topLevelOfType(ft, "function_declaration", 1)
		mods := childOfType(ft, d, "modifiers")
		if mods == 0 || childOfType(ft, mods, "annotation") == 0 || childOfType(ft, mods, "visibility_modifier") == 0 {
			t.Fatalf("%q: modifiers incomplete", fn)
		}
		off := strings.Index(src, fn)
		if int(ft.StartBytes[d]) != off || int(ft.StartBytes[mods]) != off {
			t.Fatalf("%q: start bytes %d/%d, want %d", fn, ft.StartBytes[d], ft.StartBytes[mods], off)
		}
		if row := strings.Count(src[:off], "\n"); int(ft.StartRows[d]) != row {
			t.Fatalf("%q: row %d, want %d", fn, ft.StartRows[d], row)
		}
		if got := string(content[ft.EndBytes[d]-1 : ft.EndBytes[d]]); got != "}" {
			t.Fatalf("%q: declaration ends at %q", fn, got)
		}
		last := topLevelOfType(ft, "function_declaration", 2)
		if want := strings.Count(src[:strings.Index(src, "fun last")], "\n"); int(ft.StartRows[last]) != want {
			t.Fatalf("%q: later row %d, want %d", fn, ft.StartRows[last], want)
		}
	}
}

func TestSingleAnnotationFunctionUnchanged(t *testing.T) {
	ft, _ := parseSynthetic(t, "@Composable\ninternal fun One() {}\n")
	if got := strings.Join(topLevelTypes(ft), ","); got != "function_declaration" {
		t.Fatalf("top level = %s", got)
	}
}
