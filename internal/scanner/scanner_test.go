package scanner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

func TestLineOffsets_SingleLine(t *testing.T) {
	f := &File{Content: []byte("hello")}
	offsets := f.LineOffsets()
	if len(offsets) != 1 {
		t.Fatalf("expected 1 offset, got %d", len(offsets))
	}
	if offsets[0] != 0 {
		t.Fatalf("expected offset[0]=0, got %d", offsets[0])
	}
}

func TestLineOffsets_MultipleLines(t *testing.T) {
	f := &File{Content: []byte("line1\nline2\nline3")}
	offsets := f.LineOffsets()
	// "line1\n" = 6 bytes, "line2\n" = 6 bytes, "line3" = 5 bytes
	expected := []int{0, 6, 12}
	if len(offsets) != len(expected) {
		t.Fatalf("expected %d offsets, got %d", len(expected), len(offsets))
	}
	for i, exp := range expected {
		if offsets[i] != exp {
			t.Errorf("offset[%d]: expected %d, got %d", i, exp, offsets[i])
		}
	}
}

func TestLineOffsets_CRLF(t *testing.T) {
	f := &File{Content: []byte("line1\r\nline2\r\nline3")}
	offsets := f.LineOffsets()
	// '\n' at index 6, so line2 starts at 7; '\n' at index 13, line3 starts at 14
	expected := []int{0, 7, 14}
	if len(offsets) != len(expected) {
		t.Fatalf("expected %d offsets, got %d", len(expected), len(offsets))
	}
	for i, exp := range expected {
		if offsets[i] != exp {
			t.Errorf("offset[%d]: expected %d, got %d", i, exp, offsets[i])
		}
	}
}

func TestLineOffsets_EmptyContent(t *testing.T) {
	f := &File{Content: []byte{}}
	offsets := f.LineOffsets()
	if len(offsets) != 1 {
		t.Fatalf("expected 1 offset for empty content, got %d", len(offsets))
	}
	if offsets[0] != 0 {
		t.Fatalf("expected offset[0]=0, got %d", offsets[0])
	}
}

func TestLineOffsets_TrailingNewline(t *testing.T) {
	f := &File{Content: []byte("line1\nline2\n")}
	offsets := f.LineOffsets()
	// line1 at 0, line2 at 6, empty line at 12
	expected := []int{0, 6, 12}
	if len(offsets) != len(expected) {
		t.Fatalf("expected %d offsets, got %d", len(expected), len(offsets))
	}
	for i, exp := range expected {
		if offsets[i] != exp {
			t.Errorf("offset[%d]: expected %d, got %d", i, exp, offsets[i])
		}
	}
}

func TestLineOffsets_Cached(t *testing.T) {
	f := &File{Content: []byte("a\nb\nc")}
	offsets1 := f.LineOffsets()
	offsets2 := f.LineOffsets()
	// Should be the same slice (cached)
	if &offsets1[0] != &offsets2[0] {
		t.Fatal("expected LineOffsets to return cached result")
	}
}

func TestLineOffset_InRange(t *testing.T) {
	f := &File{Content: []byte("abc\ndef\nghi")}
	if got := f.LineOffset(0); got != 0 {
		t.Errorf("LineOffset(0): expected 0, got %d", got)
	}
	if got := f.LineOffset(1); got != 4 {
		t.Errorf("LineOffset(1): expected 4, got %d", got)
	}
	if got := f.LineOffset(2); got != 8 {
		t.Errorf("LineOffset(2): expected 8, got %d", got)
	}
}

func TestLineOffset_OutOfRange(t *testing.T) {
	f := &File{Content: []byte("abc\ndef")}
	if got := f.LineOffset(100); got != len(f.Content) {
		t.Errorf("LineOffset(100): expected %d, got %d", len(f.Content), got)
	}
}

func parseTestKotlin(t *testing.T, src string) *sitter.Node {
	t.Helper()
	content := []byte(src)
	parser := sitter.NewParser()
	parser.SetLanguage(kotlin.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		t.Fatalf("failed to parse Kotlin: %v", err)
	}
	return tree.RootNode()
}

// --- ParseFile tests ---

func TestParseFile_ValidKotlinFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Test.kt")
	src := "fun main() {\n    println(\"hello\")\n}\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	f, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if f.FlatTree == nil {
		t.Fatal("expected non-nil FlatTree")
	}
	if got := f.FlatType(0); got != "source_file" {
		t.Errorf("expected root node type 'source_file', got %q", got)
	}
	if len(f.Lines) != 4 {
		t.Errorf("expected 4 lines (trailing newline creates empty last element), got %d", len(f.Lines))
	}
	if f.Path != path {
		t.Errorf("expected Path=%q, got %q", path, f.Path)
	}
}

func TestParseFile_InternsPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Interned.kt")
	src := "fun value() = 1\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	firstPath := strings.Clone(path)
	secondPath := string([]byte(path))

	first, err := ParseFile(firstPath)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	second, err := ParseFile(secondPath)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if unsafe.StringData(first.Path) != unsafe.StringData(second.Path) {
		t.Fatalf("expected ParseFile to intern identical paths")
	}
}

func TestParseFile_NonexistentFile(t *testing.T) {
	_, err := ParseFile("/nonexistent/path/Foo.kt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// --- NodeText tests ---

func TestNodeText_ExtractsCorrectContent(t *testing.T) {
	src := "val greeting = \"hello\"\n"
	content := []byte(src)
	root := parseTestKotlin(t, src)

	var propNode *sitter.Node
	WalkNodes(root, "property_declaration", func(child *sitter.Node) {
		if propNode == nil {
			propNode = child
		}
	})
	if propNode == nil {
		t.Fatal("expected to find property_declaration node")
	}
	text := NodeText(propNode, content)
	if text != "val greeting = \"hello\"" {
		t.Errorf("expected %q, got %q", "val greeting = \"hello\"", text)
	}
}

func TestNodeBytes_ReturnsLiveView(t *testing.T) {
	src := "private val x = 1\n"
	content := []byte(src)
	root := parseTestKotlin(t, src)

	var propNode *sitter.Node
	WalkNodes(root, "property_declaration", func(n *sitter.Node) {
		propNode = n
	})
	if propNode == nil {
		t.Fatal("expected to find property_declaration")
	}

	modNode := FindModifierNode(propNode, content, "private")
	if modNode == nil {
		t.Fatal("expected to find private modifier node")
	}

	view := NodeBytes(modNode, content)
	if string(view) != "private" {
		t.Fatalf("expected %q, got %q", "private", string(view))
	}

	content[modNode.StartByte()] = 'P'
	if string(view) != "Private" {
		t.Fatalf("expected live view to reflect updated content, got %q", string(view))
	}
}

func TestNodeString_ReturnsStableInternedCopy(t *testing.T) {
	src := "private val x = 1\n"
	content := []byte(src)
	root := parseTestKotlin(t, src)

	var propNode *sitter.Node
	WalkNodes(root, "property_declaration", func(n *sitter.Node) {
		propNode = n
	})
	if propNode == nil {
		t.Fatal("expected to find property_declaration")
	}

	modNode := FindModifierNode(propNode, content, "private")
	if modNode == nil {
		t.Fatal("expected to find private modifier node")
	}

	pool := NewStringPool()
	got := NodeString(modNode, content, pool)
	content[modNode.StartByte()] = 'P'
	if got != "private" {
		t.Fatalf("expected interned string to remain stable after content mutation, got %q", got)
	}
}

// --- IsCommentLine tests ---

func TestIsCommentLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"// comment", true},
		{"  // indented comment", true},
		{"/* block comment */", true},
		{"  /* indented block */", true},
		{"* continuation line", true},
		{"  * indented continuation", true},
		{"val x = 1", false},
		{"", false},
		{"   ", false},
		{"fun foo() // trailing comment", false},
	}
	for _, tt := range tests {
		got := IsCommentLine(tt.line)
		if got != tt.want {
			t.Errorf("IsCommentLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

// --- WalkNodes tests ---

func TestWalkNodes_CountsMatchingNodes(t *testing.T) {
	src := "fun foo() {}\nfun bar() {}\nval x = 1\n"
	root := parseTestKotlin(t, src)

	var count int
	WalkNodes(root, "function_declaration", func(_ *sitter.Node) {
		count++
	})
	if count != 2 {
		t.Errorf("expected 2 function_declarations, got %d", count)
	}
}

func TestWalkNodes_NoMatches(t *testing.T) {
	src := "val x = 1\n"
	root := parseTestKotlin(t, src)

	var count int
	WalkNodes(root, "class_declaration", func(_ *sitter.Node) {
		count++
	})
	if count != 0 {
		t.Errorf("expected 0 class_declarations, got %d", count)
	}
}

func TestWalkAllNodes_VisitsEveryNode(t *testing.T) {
	src := "val x = 1\n"
	root := parseTestKotlin(t, src)

	var total int
	WalkAllNodes(root, func(_ *sitter.Node) {
		total++
	})
	if total < 3 {
		t.Errorf("expected at least 3 nodes (root + property + children), got %d", total)
	}
}

// --- FindChild tests ---

func TestFindChild_Found(t *testing.T) {
	src := "fun foo() {}\nval x = 1\n"
	root := parseTestKotlin(t, src)

	child := FindChild(root, "function_declaration")
	if child == nil {
		t.Fatal("expected to find function_declaration child")
	}
	if child.Type() != "function_declaration" {
		t.Errorf("expected type 'function_declaration', got %q", child.Type())
	}
}

func TestFindChild_NotFound(t *testing.T) {
	src := "val x = 1\n"
	root := parseTestKotlin(t, src)

	child := FindChild(root, "class_declaration")
	if child != nil {
		t.Errorf("expected nil when child type not found, got %q", child.Type())
	}
}

// --- HasAncestorOfType tests ---

func TestHasAncestorOfType_True(t *testing.T) {
	src := "fun foo() { val x = 1 }\n"
	root := parseTestKotlin(t, src)

	var propNode *sitter.Node
	WalkNodes(root, "property_declaration", func(n *sitter.Node) {
		propNode = n
	})
	if propNode == nil {
		t.Fatal("expected to find property_declaration inside function")
	}

	if !HasAncestorOfType(propNode, "function_declaration") {
		t.Error("expected property_declaration to have function_declaration ancestor")
	}
	if !HasAncestorOfType(propNode, "source_file") {
		t.Error("expected property_declaration to have source_file ancestor")
	}
}

func TestHasAncestorOfType_False(t *testing.T) {
	src := "val x = 1\n"
	root := parseTestKotlin(t, src)

	var propNode *sitter.Node
	WalkNodes(root, "property_declaration", func(n *sitter.Node) {
		propNode = n
	})
	if propNode == nil {
		t.Fatal("expected to find property_declaration")
	}

	if HasAncestorOfType(propNode, "class_declaration") {
		t.Error("expected property_declaration NOT to have class_declaration ancestor")
	}
}

// --- HasModifier tests ---

func TestHasModifier_Detected(t *testing.T) {
	src := "private val x = 1\n"
	content := []byte(src)
	root := parseTestKotlin(t, src)

	var propNode *sitter.Node
	WalkNodes(root, "property_declaration", func(n *sitter.Node) {
		propNode = n
	})
	if propNode == nil {
		t.Fatal("expected to find property_declaration")
	}

	if !HasModifier(propNode, content, "private") {
		t.Error("expected HasModifier to detect 'private'")
	}
}

func TestHasModifier_NotDetected(t *testing.T) {
	src := "val x = 1\n"
	content := []byte(src)
	root := parseTestKotlin(t, src)

	var propNode *sitter.Node
	WalkNodes(root, "property_declaration", func(n *sitter.Node) {
		propNode = n
	})
	if propNode == nil {
		t.Fatal("expected to find property_declaration")
	}

	if HasModifier(propNode, content, "private") {
		t.Error("expected HasModifier to return false for missing modifier")
	}
}

func TestHasModifier_Override(t *testing.T) {
	src := "class Foo { override fun bar() {} }\n"
	content := []byte(src)
	root := parseTestKotlin(t, src)

	var funcNode *sitter.Node
	WalkNodes(root, "function_declaration", func(n *sitter.Node) {
		funcNode = n
	})
	if funcNode == nil {
		t.Fatal("expected to find function_declaration")
	}

	if !HasModifier(funcNode, content, "override") {
		t.Error("expected HasModifier to detect 'override'")
	}
}

// --- CountNodes tests ---

func TestCountNodes_MultipleFunctions(t *testing.T) {
	src := "fun a() {}\nfun b() {}\nfun c() {}\n"
	root := parseTestKotlin(t, src)

	count := CountNodes(root, "function_declaration")
	if count != 3 {
		t.Errorf("expected 3 function_declarations, got %d", count)
	}
}

func TestCountNodes_Zero(t *testing.T) {
	src := "val x = 1\n"
	root := parseTestKotlin(t, src)

	count := CountNodes(root, "class_declaration")
	if count != 0 {
		t.Errorf("expected 0 class_declarations, got %d", count)
	}
}

// --- FindModifierNode tests ---

func TestFindModifierNode_Found(t *testing.T) {
	src := "private val x = 1\n"
	content := []byte(src)
	root := parseTestKotlin(t, src)

	var propNode *sitter.Node
	WalkNodes(root, "property_declaration", func(n *sitter.Node) {
		propNode = n
	})
	if propNode == nil {
		t.Fatal("expected to find property_declaration")
	}

	node := FindModifierNode(propNode, content, "private")
	if node == nil {
		t.Fatal("expected FindModifierNode to return non-nil for 'private'")
	}
	if NodeText(node, content) != "private" {
		t.Errorf("expected node text 'private', got %q", NodeText(node, content))
	}
}

func TestFindModifierNode_Override(t *testing.T) {
	src := "class Foo { override fun bar() {} }\n"
	content := []byte(src)
	root := parseTestKotlin(t, src)

	var funcNode *sitter.Node
	WalkNodes(root, "function_declaration", func(n *sitter.Node) {
		funcNode = n
	})
	if funcNode == nil {
		t.Fatal("expected to find function_declaration")
	}

	node := FindModifierNode(funcNode, content, "override")
	if node == nil {
		t.Fatal("expected FindModifierNode to return non-nil for 'override'")
	}
	if NodeText(node, content) != "override" {
		t.Errorf("expected node text 'override', got %q", NodeText(node, content))
	}
}

func TestFindModifierNode_NotFound(t *testing.T) {
	src := "val x = 1\n"
	content := []byte(src)
	root := parseTestKotlin(t, src)

	var propNode *sitter.Node
	WalkNodes(root, "property_declaration", func(n *sitter.Node) {
		propNode = n
	})
	if propNode == nil {
		t.Fatal("expected to find property_declaration")
	}

	node := FindModifierNode(propNode, content, "private")
	if node != nil {
		t.Errorf("expected FindModifierNode to return nil when no modifiers present, got %q", NodeText(node, content))
	}
}

func TestParseFile_MalformedKotlin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.kt")
	os.WriteFile(path, []byte("package test\nfun broken( { class }}} if while\n"), 0644)

	// Tree-sitter is error-recovering — it should parse without returning an error
	file, err := ParseFile(path)
	if err != nil {
		t.Fatalf("expected no error (tree-sitter recovers), got: %v", err)
	}
	if file == nil {
		t.Fatal("expected non-nil file")
	}
	if file.FlatTree == nil {
		t.Fatal("expected non-nil flat tree")
	}
	// Should still have lines
	if len(file.Lines) == 0 {
		t.Error("expected non-empty lines")
	}
}

func TestReadLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.kt")
	os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0644)

	lines, err := ReadLines(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestReadLines_NonexistentFile(t *testing.T) {
	_, err := ReadLines("/nonexistent/path.kt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestCollectKotlinFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.kt"), []byte("package test"), 0644)
	os.WriteFile(filepath.Join(dir, "b.kt"), []byte("package test"), 0644)
	os.WriteFile(filepath.Join(dir, "c.java"), []byte("package test;"), 0644)
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0755)
	os.WriteFile(filepath.Join(dir, ".hidden", "d.kt"), []byte("package test"), 0644)

	files, err := CollectKotlinFiles([]string{dir}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should find .kt files (hidden dirs may or may not be skipped depending on implementation)
	if len(files) < 2 {
		t.Fatalf("expected at least 2 .kt files, got %d", len(files))
	}
}

func TestCollectKotlinFilesRespectsGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".claude/worktrees/\ngenerated/\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	keep := filepath.Join(dir, "src", "main", "kotlin", "Keep.kt")
	ignoredClaude := filepath.Join(dir, ".claude", "worktrees", "nested", "Ignored.kt")
	ignoredGenerated := filepath.Join(dir, "generated", "IgnoredGenerated.kt")
	for _, path := range []string{keep, ignoredClaude, ignoredGenerated} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to create parent dir for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte("package test"), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	files, err := CollectKotlinFiles([]string{dir}, nil)
	if err != nil {
		t.Fatalf("CollectKotlinFiles returned error: %v", err)
	}
	if !hasPath(files, keep) {
		t.Fatalf("expected kept file %q in collected files: %#v", keep, files)
	}
	if hasPath(files, ignoredClaude) {
		t.Fatalf("expected .gitignored worktree file %q to be skipped: %#v", ignoredClaude, files)
	}
	if hasPath(files, ignoredGenerated) {
		t.Fatalf("expected .gitignored generated file %q to be skipped: %#v", ignoredGenerated, files)
	}
}

func TestCollectKotlinFilesRespectsGitignoreForExplicitPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("generated/\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	ignored := filepath.Join(dir, "generated", "Ignored.kt")
	if err := os.MkdirAll(filepath.Dir(ignored), 0755); err != nil {
		t.Fatalf("failed to create generated dir: %v", err)
	}
	if err := os.WriteFile(ignored, []byte("package test"), 0644); err != nil {
		t.Fatalf("failed to write ignored file: %v", err)
	}

	files, err := CollectKotlinFiles([]string{ignored}, nil)
	if err != nil {
		t.Fatalf("CollectKotlinFiles returned error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected explicit .gitignored file to be skipped, got %#v", files)
	}
}

func TestCollectJavaFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.kt"), []byte("package test"), 0644)
	os.WriteFile(filepath.Join(dir, "b.java"), []byte("package test;"), 0644)

	files, err := CollectJavaFiles([]string{dir}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 .java file, got %d", len(files))
	}
}

func TestCollectJavaFilesRespectsNestedGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	module := filepath.Join(dir, "module")
	if err := os.MkdirAll(filepath.Join(module, "generated"), 0755); err != nil {
		t.Fatalf("failed to create module dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(module, ".gitignore"), []byte("generated/\n"), 0644); err != nil {
		t.Fatalf("failed to write nested .gitignore: %v", err)
	}
	keep := filepath.Join(module, "Keep.java")
	ignored := filepath.Join(module, "generated", "Ignored.java")
	for _, path := range []string{keep, ignored} {
		if err := os.WriteFile(path, []byte("package test;"), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	files, err := CollectJavaFiles([]string{dir}, nil)
	if err != nil {
		t.Fatalf("CollectJavaFiles returned error: %v", err)
	}
	if !hasPath(files, keep) {
		t.Fatalf("expected kept Java file %q in collected files: %#v", keep, files)
	}
	if hasPath(files, ignored) {
		t.Fatalf("expected nested .gitignored Java file %q to be skipped: %#v", ignored, files)
	}
}

func hasPath(paths []string, want string) bool {
	wantAbs, _ := filepath.Abs(want)
	for _, path := range paths {
		gotAbs, _ := filepath.Abs(path)
		if gotAbs == wantAbs {
			return true
		}
	}
	return false
}

func TestPartitionIndexedPaths(t *testing.T) {
	paths := []string{"a.kt", "b.kt", "c.kt", "d.kt", "e.kt", "f.kt", "g.kt"}

	got := partitionIndexedPaths(paths, 3)
	want := [][]indexedPath{
		{
			{index: 0, path: "a.kt"},
			{index: 1, path: "b.kt"},
		},
		{
			{index: 2, path: "c.kt"},
			{index: 3, path: "d.kt"},
		},
		{
			{index: 4, path: "e.kt"},
			{index: 5, path: "f.kt"},
			{index: 6, path: "g.kt"},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partition mismatch:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestPartitionIndexedPaths_ClampsWorkers(t *testing.T) {
	paths := []string{"a.kt", "b.kt"}

	got := partitionIndexedPaths(paths, 8)
	want := [][]indexedPath{
		{{index: 0, path: "a.kt"}},
		{{index: 1, path: "b.kt"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partition mismatch when clamping workers:\nwant: %#v\ngot:  %#v", want, got)
	}

	got = partitionIndexedPaths(paths, 0)
	want = [][]indexedPath{
		{
			{index: 0, path: "a.kt"},
			{index: 1, path: "b.kt"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partition mismatch when normalizing worker count:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestScanFiles_PreservesInputOrder(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "First.kt")
	secondPath := filepath.Join(dir, "Second.kt")
	missingPath := filepath.Join(dir, "Missing.kt")

	if err := os.WriteFile(firstPath, []byte("fun first() = 1\n"), 0644); err != nil {
		t.Fatalf("failed to write first file: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("fun second() = 2\n"), 0644); err != nil {
		t.Fatalf("failed to write second file: %v", err)
	}

	files, errs := ScanFiles([]string{secondPath, missingPath, firstPath}, 2)
	if len(files) != 2 {
		t.Fatalf("expected 2 parsed files, got %d", len(files))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(errs))
	}

	gotPaths := []string{files[0].Path, files[1].Path}
	wantPaths := []string{secondPath, firstPath}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("parsed file order mismatch:\nwant: %#v\ngot:  %#v", wantPaths, gotPaths)
	}
	if !strings.Contains(errs[0].Error(), missingPath) {
		t.Fatalf("expected parse error to mention %q, got %q", missingPath, errs[0].Error())
	}
}
