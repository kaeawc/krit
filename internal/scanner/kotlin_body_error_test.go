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

// A class-like body nested in another body and closed on the same line as its
// last member made the grammar's error recovery swallow the enclosing class and
// every declaration after it into one ERROR node, so the output filter dropped
// every finding in the file (#1839).
func TestOneLineNestedClassBodiesHaveNoErrorNodes(t *testing.T) {
	tests := []struct {
		name, line, memberType, memberText string
	}{
		{"companion object property", "class H { companion object { val X = 1 } }", "property_declaration", "val X = 1"},
		{"object const property", "class H { object O { const val X = 1 } }", "property_declaration", "const val X = 1"},
		{"annotated companion property", "class H { companion object { @JvmField val X = 1 } }", "property_declaration", "@JvmField val X = 1"},
		{"companion function", "class H { companion object { fun g() {} } }", "function_declaration", "fun g() {}"},
		{"two companion members", "class H { companion object { val X = 1; val Y = 2 } }", "property_declaration", "val Y = 2"},
		{"nested class", "class H { class I { val x = 1 } }", "property_declaration", "val x = 1"},
		{"nested interface", "class H { interface I { fun f() } }", "function_declaration", "fun f()"},
		{"nested interface property", "class H { interface I { val x: Int } }", "property_declaration", "val x: Int"},
		{"enum body member", "class H { enum class E { A, B; fun g() = 1 } }", "function_declaration", "fun g() = 1"},
		{"enum entry body", "class H { enum class E { A { override fun f() = 1 } } }", "function_declaration", "override fun f() = 1"},
		{"three levels", "class A { class B { class C { val x = 1 } } }", "property_declaration", "val x = 1"},
		{"four levels", "class A { class B { class C { object D { val x = 1 } } } }", "property_declaration", "val x = 1"},
		{"annotated outer class", "@Suppress(\"X\") class H { companion object { fun g() = 1 } }", "function_declaration", "fun g() = 1"},
		{"object literal in a member", "class H { val o = object { val x = 1 } }", "property_declaration", "val x = 1"},
		{"comment before the brace", "class H { companion object { val X = 1 /* c */ } }", "property_declaration", "val X = 1"},
		{"adjacent closing braces", "class H { companion object {val X = 1 }}", "property_declaration", "val X = 1"},
		{"outer header on its own line", "class H {\n  companion object { val X = 1 } }", "property_declaration", "val X = 1"},
		{"trailing line comment", "class H { companion object { val X = 1 } } // c", "property_declaration", "val X = 1"},
		// The inner '}' sits alone in an ERROR node whose member is the node
		// before it, outside that ERROR (#1875).
		{"';' and a member after the body", "class H { companion object { val X = 1 }; val y = 2 }", "property_declaration", "val X = 1"},
		{"' ; ' and a member after the body", "class H { companion object { val X = 1 } ; val y = 2 }", "property_declaration", "val y = 2"},
		{"';' and two members after the body", "class H { companion object { val X = 1 }; val y = 2; val z = 3 }", "property_declaration", "val z = 3"},
		{"';' and a function after the body", "class H { companion object { val X = 1 }; fun f() {} }", "function_declaration", "fun f() {}"},
		{"';' and an init block after the body", "class H { companion object { val X = 1 }; init {} }", "anonymous_initializer", "init {}"},
		{"member after the body with no ';'", "class H { companion object { val X = 1 } fun f() {} }", "function_declaration", "fun f() {}"},
		{"two objects separated by ';'", "class H { object O { val X = 1 }; object P { val Y = 2 } }", "property_declaration", "val Y = 2"},
		{"';' after bodies at three levels", "class A { class B { class C { val x = 1 }; val y = 2 }; val z = 3 }", "property_declaration", "val y = 2"},
		{"';' after a companion in an enum body", "enum class E { A, B; companion object { val X = 1 }; fun g() = 1 }", "function_declaration", "fun g() = 1"},
		{"';' after an object literal", "class H { val o = object { val x = 1 }; val y = 2 }", "property_declaration", "val y = 2"},
		{"';' and no member after the body", "class H { companion object { val X = 1 }; }", "property_declaration", "val X = 1"},
		{"comment holding '}' before ';'", "class H { companion object { val X = 1 } /* } */; val y = 2 }", "property_declaration", "val y = 2"},
		{"';' and a string holding ' }'", "class H { companion object { val X = 1 }; val s = \" }\"; val y = 2 }", "property_declaration", "val s = \" }\""},
		// No byte before the closing brace: a ';' is inserted and the tree is
		// mapped back to the source's offsets (#1866).
		{"no space before either brace", "class H { companion object { val X = 1}}", "property_declaration", "val X = 1"},
		{"no space before the inner brace", "class H { companion object { val X = 1} }", "property_declaration", "val X = 1"},
		{"no space before the outer brace", "class H { companion object { val X = 1 }}", "property_declaration", "val X = 1"},
		{"no space, object const property", "class H { object O { const val X = 1}}", "property_declaration", "const val X = 1"},
		{"no space, companion function", "class H { companion object { fun g() {}}}", "function_declaration", "fun g() {}"},
		{"no space, call initializer", "class H { companion object { val X = listOf(1)}}", "property_declaration", "val X = listOf(1)"},
		{"no space, string initializer holding '}'", "class H { companion object { val s = \"}\"}}", "property_declaration", "val s = \"}\""},
		{"no space, two members", "class H { companion object { val X = 1; val Y = 2}}", "property_declaration", "val Y = 2"},
		{"no space, three levels", "class A { class B { class C { val x = 1}}}", "property_declaration", "val x = 1"},
		{"no space, four levels", "class A { class B { class C { object D { val x = 1}}}}", "property_declaration", "val x = 1"},
		{"mixed spacing, three levels", "class A { class B { class C { val x = 1} }}", "property_declaration", "val x = 1"},
		{"no space, nested interface", "class H { interface I { fun f()}}", "function_declaration", "fun f()"},
		{"no space, enum body member", "class H { enum class E { A, B; fun g() = 1}}", "function_declaration", "fun g() = 1"},
		{"no space, object literal in a member", "class H { val o = object { val x = 1}}", "property_declaration", "val x = 1"},
		{"no space, outer object", "object O { object P { val x = 1}}", "property_declaration", "val x = 1"},
		{"no space, outer interface", "interface I { companion object { val X = 1}}", "property_declaration", "val X = 1"},
		{"no space, comment before the brace", "class H { companion object { val X = 1 /* c */}}", "property_declaration", "val X = 1"},
		{"no space, ';' and a member", "class H { companion object { val X = 1}; val y = 2 }", "property_declaration", "val y = 2"},
		{"no space, ';' and a member, no space at the end", "class H { companion object { val X = 1}; val y = 2}", "property_declaration", "val y = 2"},
		{"no space, ';' at three levels", "class A { class B { class C { val x = 1}; val y = 2}; val z = 3}", "property_declaration", "val z = 3"},
		{"no space, ';' in an enum body", "enum class E { A, B; companion object { val X = 1}; fun g() = 1}", "function_declaration", "fun g() = 1"},
		{"no space, two objects separated by ';'", "class H { object O { val X = 1}; object P { val Y = 2}}", "property_declaration", "val Y = 2"},
	}
	for _, tc := range tests {
		// A later function with a body is what made recovery swallow the
		// rest of the file for the ';' shapes (#1875).
		for _, tail := range []string{"\nval m = 42\n", "\nfun after() { val unused = 1 }\n", ""} {
			name := tc.name
			switch {
			case tail == "":
				name += "/at EOF"
			case strings.HasPrefix(tail, "\nfun"):
				name += "/before a function"
			}
			t.Run(name, func(t *testing.T) {
				src := tc.line + tail
				f := parseScriptForTest(t, "a.kt", src)
				assertNoErrorNodes(t, f)

				// The outer declaration is a direct child of the file and ends
				// where the original line does.
				var outer uint32
				f.FlatForEachChild(0, func(c uint32) {
					if typ := f.FlatType(c); outer == 0 && (typ == "class_declaration" || typ == "object_declaration") {
						outer = c
					}
				})
				if outer == 0 {
					t.Fatalf("no top-level class or object declaration; top level = %v", scriptTopLevelTypes(f))
				}
				lineEnd := strings.Index(tc.line, " // c")
				if lineEnd < 0 {
					lineEnd = len(tc.line)
				}
				if got := f.FlatNodeText(outer); got != tc.line[:lineEnd] {
					t.Fatalf("outer class text = %q, want %q", got, tc.line[:lineEnd])
				}
				if tail != "" {
					want := "property_declaration"
					if strings.HasPrefix(tail, "\nfun") {
						want = "function_declaration"
					}
					if got := scriptTopLevelTypes(f); got[len(got)-1] != want {
						t.Fatalf("trailing declaration lost; top level = %v", got)
					}
				}

				// The member sits inside the outer class at its original
				// offsets, with no rewrite byte in its text.
				var member uint32
				f.FlatWalkNodes(outer, tc.memberType, func(idx uint32) {
					if member == 0 && f.FlatNodeText(idx) == tc.memberText {
						member = idx
					}
				})
				if member == 0 {
					t.Fatalf("no %s %q inside the outer class", tc.memberType, tc.memberText)
				}
				if start := f.FlatStartByte(member); !strings.HasPrefix(src[start:], tc.memberText) {
					t.Fatalf("member start %d does not point at %q", start, tc.memberText)
				}

				// Any ';' token not in the source is a zero-width separator.
				for idx := range f.FlatTree.Types {
					i := uint32(idx)
					if f.FlatType(i) == ";" && f.FlatEndByte(i) > f.FlatStartByte(i) && src[f.FlatStartByte(i)] != ';' {
						t.Fatalf("inserted ';' at %d is visible in the tree", f.FlatStartByte(i))
					}
				}
				if f.FlatEndByte(0) != uint32(len(src)) {
					t.Fatalf("root ends at %d, source has %d bytes", f.FlatEndByte(0), len(src))
				}
				for i := range f.FlatTree.StartBytes {
					off := f.FlatTree.StartBytes[i]
					row := strings.Count(src[:off], "\n")
					col := off - uint32(strings.LastIndex(src[:off], "\n")+1)
					if int(f.FlatTree.StartRows[i]) != row || uint32(f.FlatTree.StartCols[i]) != col {
						t.Fatalf("node %d at %d has row/col %d:%d, want %d:%d", i, off, f.FlatTree.StartRows[i], f.FlatTree.StartCols[i], row, col)
					}
				}
			})
		}
	}
}

// The repair only keeps a reparse that shrinks the error region, so a real
// syntax error inside a one-line nested body is still reported, and source
// that already parsed cleanly keeps the tree the grammar gave it.
func TestOneLineNestedClassBodyRepairKeepsRealErrors(t *testing.T) {
	f := parseScriptForTest(t, "a.kt", "class A { object B { val x = 1 ) } }\nval m = 42\n")
	hasError := false
	for _, flags := range f.FlatTree.Flags {
		hasError = hasError || flags&flatNodeFlagIsError != 0
	}
	if !hasError {
		t.Fatal("broken source must keep its error node")
	}

	// The same with ';' and a member after the nested body (#1875).
	for _, src := range []string{
		"class A { object B { val x = 1 ) }; val y = 2 }\nval m = 42\n",
		"class A { object B { val x = 1 }; val y = ) }\nval m = 42\n",
	} {
		f := parseScriptForTest(t, "a.kt", src)
		hasError := false
		for _, flags := range f.FlatTree.Flags {
			hasError = hasError || flags&flatNodeFlagIsError != 0
		}
		if !hasError {
			t.Fatalf("broken source %q must keep its error node", src)
		}
	}

	// With no space before the braces the repair inserts a ';'; a real error
	// there is still an error (#1866).
	for _, src := range []string{
		"class A { object B { val x = 1 )}}\nval m = 42\n",
		"class A { object B { val x = 1 )}; val y = 2}\nfun after() { val unused = 1 }\n",
		"class A { object B { val x = 1}; val y = )}\nfun after() { val unused = 1 }\n",
	} {
		f := parseScriptForTest(t, "a.kt", src)
		hasError := false
		for _, flags := range f.FlatTree.Flags {
			hasError = hasError || flags&flatNodeFlagIsError != 0
		}
		if !hasError {
			t.Fatalf("broken source %q must keep its error node", src)
		}
	}

	// The no-space forms from #1866, which used to be a known limit, recover.
	for _, src := range []string{
		"class H { companion object { val X = 1}}\nval m = 42\n",
		"class H { companion object { val X = 1}; val y = 2 }\nfun after() { val unused = 1 }\n",
	} {
		h := parseScriptForTest(t, "c.kt", src)
		assertNoErrorNodes(t, h)
		if got := strings.Join(scriptTopLevelTypes(h), ","); !strings.HasPrefix(got, "class_declaration,") {
			t.Fatalf("no-space form %q parses as %v", src, got)
		}
	}

	// Clean source keeps its tree: every ';' node is a ';' byte of the source.
	for _, clean := range []string{
		"fun f() { run { val x = 1 } }\nclass H { init { val y = 2 } }\n",
		"fun f() { run { val x = 1 }; val y = 2 }\nclass H { init { val y = 2 }; val z = 1 }\n",
		"class H { val s = \" }\"; val t = \"\"\" } \"\"\"; val y = 2 }\nfun g() { if (true) { h() }; h() }\n",
		// Same-line '}' with no space before it in lambdas, blocks, `when`,
		// `if`/`else` and string templates.
		"fun f() { run { val x = 1}}\nclass H { init { val y = 2}}\n",
		"val l = { x: Int -> x}\nval s = \"${l(1)}\"\nval t = \"${\"}\"}\"\n",
		"fun g(x: Int) { if (x > 0) { h()} else { k()}; when (x) { 1 -> h(); else -> k()}}\n",
	} {
		g := parseScriptForTest(t, "b.kt", clean)
		assertNoErrorNodes(t, g)
		for idx := range g.FlatTree.Types {
			i := uint32(idx)
			if g.FlatType(i) == ";" && (g.FlatEndByte(i) != g.FlatStartByte(i)+1 || clean[g.FlatStartByte(i)] != ';') {
				t.Fatalf("clean source %q gained a ';' node at %d", clean, g.FlatStartByte(i))
			}
		}
	}
}

// A line of declarations separated by ';', each with a one-line nested body
// followed by ';' and a member, parses into those declarations (#1875).
func TestOneLineNestedBodiesBeforeSemicolonOnOneLine(t *testing.T) {
	src := "class H { companion object { val X = 1 }; val y = 2 }; class J { object O { val Z = 1 }; val w = 3 }\nfun after() { val unused = 1 }\n"
	f := parseScriptForTest(t, "a.kt", src)
	assertNoErrorNodes(t, f)
	got := strings.Join(scriptTopLevelTypes(f), ",")
	if want := "class_declaration,class_declaration,function_declaration"; got != want {
		t.Fatalf("top level = %s, want %s", got, want)
	}
}

// spaceBeforeBraces puts a space before every '}' that directly follows a
// byte other than a space or '{'. The test lines hold no strings or comments.
func spaceBeforeBraces(line string) string {
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] == '}' && i > 0 && line[i-1] != ' ' && line[i-1] != '{' {
			b.WriteByte(' ')
		}
		b.WriteByte(line[i])
	}
	return b.String()
}

// A no-space one-line nested body parses into the same tree as the form with
// a space before each brace, which the repair already handled (#1866). The
// nodes on later lines sit at the spaced form's offsets shifted back by the
// added spaces, with the same rows and columns.
func TestOneLineNestedNoSpaceBodiesMatchSpacedForm(t *testing.T) {
	for _, line := range []string{
		"class H { companion object { val X = 1}}",
		"class H { companion object { val X = 1} }",
		"class A { class B { class C { val x = 1}}}",
		"class H { companion object { val X = 1}; val y = 2 }",
		"class H { companion object { val X = 1}; val y = 2}",
		"enum class E { A, B; companion object { val X = 1}; fun g() = 1}",
		"class H { val o = object { val x = 1}}",
		"interface I { companion object { val X = 1}}",
		"class H { companion object { val X = 1}}; class J { object O { val Z = 1}; val w = 3}",
	} {
		t.Run(line, func(t *testing.T) {
			tail := "\nfun after() { val unused = 1 }\n\nval m = 42\n"
			spaced := spaceBeforeBraces(line)
			if spaced == line {
				t.Fatalf("test line %q has no brace to space", line)
			}
			f := parseScriptForTest(t, "a.kt", line+tail)
			g := parseScriptForTest(t, "a.kt", spaced+tail)
			assertNoErrorNodes(t, g)
			assertNoErrorNodes(t, f)
			ft, gt := f.FlatTree, g.FlatTree
			if len(ft.Types) != len(gt.Types) {
				t.Fatalf("%d nodes, spaced form has %d", len(ft.Types), len(gt.Types))
			}
			shift := uint32(len(spaced) - len(line))
			for i := range ft.Types {
				idx := uint32(i)
				if f.FlatType(idx) != g.FlatType(idx) || ft.Parents[i] != gt.Parents[i] {
					t.Fatalf("node %d is %s under %d, spaced form has %s under %d", i, f.FlatType(idx), ft.Parents[i], g.FlatType(idx), gt.Parents[i])
				}
				nospace := func(s string) string { return strings.ReplaceAll(s, " ", "") }
				if got, want := f.FlatNodeText(idx), g.FlatNodeText(idx); nospace(got) != nospace(want) {
					t.Fatalf("node %d %s text = %q, want %q", i, f.FlatType(idx), got, want)
				}
				if ft.StartRows[i] == 0 {
					continue
				}
				if ft.StartBytes[i]+shift != gt.StartBytes[i] || ft.EndBytes[i]+shift != gt.EndBytes[i] {
					t.Fatalf("node %d %s at [%d,%d), spaced form at [%d,%d) with shift %d", i, f.FlatType(idx), ft.StartBytes[i], ft.EndBytes[i], gt.StartBytes[i], gt.EndBytes[i], shift)
				}
				if ft.StartRows[i] != gt.StartRows[i] || ft.StartCols[i] != gt.StartCols[i] {
					t.Fatalf("node %d %s at %d:%d, spaced form at %d:%d", i, f.FlatType(idx), ft.StartRows[i], ft.StartCols[i], gt.StartRows[i], gt.StartCols[i])
				}
			}
		})
	}
}

// withInsertedSemicolons and dropInsertedBytes are inverses on offsets: a node
// ending after an inserted ';' ends before it, the ';' itself becomes zero
// width at the brace, and offsets past every insertion move back by all of
// them (#1866).
func TestInsertedSemicolonOffsetsMapBack(t *testing.T) {
	src := []byte("{ a}}x")
	inserts := []uint32{3, 4}
	buf := withInsertedSemicolons(src, inserts)
	if got, want := string(buf), "{ a;};}x"; got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
	if got := withInsertedSemicolons(src, nil); &got[0] != &src[0] {
		t.Fatal("no inserts must return src itself")
	}
	// Spans in buf: "a" [2,3), first ';' [3,4), "a;" [2,4), first '}' [4,5),
	// second ';' [5,6), second '}' [6,7), "x" [7,8), whole buffer [0,8).
	ft := &FlatTree{
		StartBytes: []uint32{2, 3, 2, 4, 5, 6, 7, 0},
		EndBytes:   []uint32{3, 4, 4, 5, 6, 7, 8, 8},
	}
	dropInsertedBytes(ft, inserts)
	wantStart := []uint32{2, 3, 2, 3, 4, 4, 5, 0}
	wantEnd := []uint32{3, 3, 3, 4, 4, 5, 6, 6}
	for i := range wantStart {
		if ft.StartBytes[i] != wantStart[i] || ft.EndBytes[i] != wantEnd[i] {
			t.Errorf("span %d = [%d,%d), want [%d,%d)", i, ft.StartBytes[i], ft.EndBytes[i], wantStart[i], wantEnd[i])
		}
	}
	if got := newInserts([]uint32{3, 9}, []uint32{9, 5, 3, 5, 1}); len(got) != 2 || got[0] != 1 || got[1] != 5 {
		t.Errorf("newInserts = %v, want [1 5]", got)
	}
}
