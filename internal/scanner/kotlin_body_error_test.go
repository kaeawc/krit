package scanner

import (
	"strings"
	"testing"
)

func classBodyOf(f *File) uint32 {
	var body uint32
	f.FlatWalkNodes(0, "class_body", func(idx uint32) {
		if body == 0 {
			body = idx
		}
	})
	return body
}

func TestOneLineClassBodiesHaveNoErrorNodes(t *testing.T) {
	tests := []struct {
		name, src, memberType, memberText string
	}{
		{"class property", "class A { val x = 1 }\n", "property_declaration", "val x = 1"},
		{"object function", "object S { fun f() {} }\n", "function_declaration", "fun f() {}"},
		{"interface function", "interface X { fun f() }\n", "function_declaration", "fun f()"},
		{"nested class", "class A { class B }\n", "class_declaration", "class B"},
		{"after other members", "class A {\n  val a = 1\n  fun g() = 2 }\n", "function_declaration", "fun g() = 2"},
		{"in a script", "class A { val x = 1 }\n", "property_declaration", "val x = 1"},
	}
	for _, tc := range tests {
		for _, path := range []string{"a.kt", "a.main.kts"} {
			t.Run(tc.name+"/"+path, func(t *testing.T) {
				f := parseScriptForTest(t, path, tc.src)
				assertNoErrorNodes(t, f)
				body := classBodyOf(f)
				if body == 0 {
					t.Fatal("no class_body")
				}
				var member uint32
				f.FlatForEachChild(body, func(c uint32) {
					if f.FlatType(c) == tc.memberType {
						member = c
					}
				})
				if member == 0 {
					t.Fatalf("%s is not a direct child of the class body", tc.memberType)
				}
				if got := f.FlatNodeText(member); got != tc.memberText {
					t.Fatalf("member text = %q, want %q", got, tc.memberText)
				}
				// Offsets stay those of the source.
				if start := f.FlatStartByte(member); tc.src[start:start+uint32(len(tc.memberText))] != tc.memberText {
					t.Fatalf("member start %d does not point at %q", start, tc.memberText)
				}
				if f.FlatEndByte(0) != uint32(len(tc.src)) {
					t.Fatalf("root ends at %d, source has %d bytes", f.FlatEndByte(0), len(tc.src))
				}
			})
		}
	}
}

func TestClassBodyErrorRepairKeepsRealErrors(t *testing.T) {
	// Stray tokens next to a member are a real syntax error and stay one.
	f := parseScriptForTest(t, "a.kt", "class A { val x = 1 ) ] }\n")
	hasError := false
	for _, flags := range f.FlatTree.Flags {
		hasError = hasError || flags&flatNodeFlagIsError != 0
	}
	if !hasError {
		t.Fatal("broken source must keep its error node")
	}
	if strings.Contains(f.FlatNodeText(0), "ERROR") {
		t.Fatal("unexpected text")
	}
}
