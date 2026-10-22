package scanner

import (
	"sort"
	"strings"
	"testing"
)

func propertyNodes(file *File) []uint32 {
	var out []uint32
	file.FlatWalkNodes(0, "property_declaration", func(idx uint32) { out = append(out, idx) })
	return out
}

// TestPropertyDeclarationShape pins the parser shape the name lookup relies
// on: the property name lives in variable_declaration, while a bare-identifier
// initializer is a direct simple_identifier child of property_declaration.
func TestPropertyDeclarationShape(t *testing.T) {
	cases := []struct{ src, name, initializer string }{
		{"val f: T = x\n", "f", "x"},
		{"val count = total\n", "count", "total"},
		{"var a = b\n", "a", "b"},
		{"class C {\n    val f: T = x\n    val count = total\n    var a = b\n}\n", "f", "x"},
	}
	for _, tc := range cases {
		file := parseKotlinSnippet(t, tc.src)
		props := propertyNodes(file)
		if len(props) == 0 {
			t.Fatalf("no property_declaration in %q", tc.src)
		}
		p := props[0]
		if _, ok := file.FlatFindChild(p, "variable_declaration"); !ok {
			t.Errorf("%q: expected variable_declaration child", tc.src)
		}
		if got := file.FlatChildTextOrEmpty(p, "simple_identifier"); got != tc.initializer {
			t.Errorf("%q: direct simple_identifier child = %q, want initializer %q", tc.src, got, tc.initializer)
		}
		if got := file.PropertyNameFlat(p); got != tc.name {
			t.Errorf("%q: PropertyNameFlat = %q, want %q", tc.src, got, tc.name)
		}
	}
}

func TestPropertyNameFlat_AllShapes(t *testing.T) {
	src := `val top1: T = x
val top2 = total
var top3 = b
val (d1, d2) = pair
val T.ext: Int get() = other
val lazyVal by lazy { 1 }
class C {
    val m1: T = x
    val m2 = total
    var m3 = b
}
`
	file := parseKotlinSnippet(t, src)
	var got []string
	for _, p := range propertyNodes(file) {
		got = append(got, file.PropertyNameFlat(p))
	}
	sort.Strings(got)
	want := []string{"", "ext", "lazyVal", "m1", "m2", "m3", "top1", "top2", "top3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("names = %q, want %q", got, want)
	}
}
