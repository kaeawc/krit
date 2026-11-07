package scanner

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
)

// TestMisparseCorpus measures parse quality over a Kotlin source tree. Set
// KRIT_MISPARSE_CORPUS to the root. It prints counts only:
//
//   - bad: files with a top-level prefix/infix expression that contains a
//     declaration;
//   - errFiles / missFiles: files with ERROR / MISSING nodes;
//   - nonDeclFiles: files with a top-level node that is not a declaration.
//
// KRIT_MISPARSE_LIST=<file> writes one "path:row detail" line per failing file
// (the first ERROR or non-declaration node). KRIT_MISPARSE_RAW=1 also measures
// the grammar without repairs.
func TestMisparseCorpus(t *testing.T) {
	root := os.Getenv("KRIT_MISPARSE_CORPUS")
	if root == "" {
		t.Skip("KRIT_MISPARSE_CORPUS not set")
	}
	var paths []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == "build" || d.Name() == ".git" || d.Name() == ".claude" || d.Name() == ".idea") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".kt") {
			paths = append(paths, p)
		}
		return nil
	})
	sort.Strings(paths)
	modes := []string{"fixed"}
	if os.Getenv("KRIT_MISPARSE_RAW") != "" {
		modes = []string{"raw", "fixed"}
	}
	for _, mode := range modes {
		runMisparseMode(mode, paths)
	}
}

type corpusFileResult struct {
	line                    string
	bad, err, miss, nonDecl bool
	shape                   string
	errCount, missingCount  int
	hasLine                 bool
}

func runMisparseMode(mode string, paths []string) {
	results := make([]corpusFileResult, len(paths))
	var wg sync.WaitGroup
	next := make(chan int, 64)
	start := time.Now()
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			parser := GetKotlinParser()
			defer PutKotlinParser(parser)
			for i := range next {
				results[i] = measureCorpusFile(mode, parser, paths[i])
			}
		}()
	}
	for i := range paths {
		next <- i
	}
	close(next)
	wg.Wait()
	elapsed := time.Since(start)

	var bad, errFiles, missFiles, nonDeclFiles, errNodes int
	shapes := map[string]int{}
	var lines []string
	for _, r := range results {
		if r.bad {
			bad++
			shapes[r.shape]++
		}
		if r.err {
			errFiles++
		}
		if r.miss {
			missFiles++
		}
		if r.nonDecl {
			nonDeclFiles++
		}
		errNodes += r.errCount
		if r.hasLine {
			lines = append(lines, r.line)
		}
	}
	fmt.Printf("MODE=%s files=%d bad=%d errFiles=%d errNodes=%d missFiles=%d nonDeclFiles=%d elapsed=%s\n",
		mode, len(paths), bad, errFiles, errNodes, missFiles, nonDeclFiles, elapsed)
	var keys []string
	for k := range shapes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  shape %q: %d\n", k, shapes[k])
	}
	if out := os.Getenv("KRIT_MISPARSE_LIST"); out != "" && mode == "fixed" {
		_ = os.WriteFile(out, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
}

func measureCorpusFile(mode string, parser *sitter.Parser, path string) corpusFileResult {
	var r corpusFileResult
	content, err := os.ReadFile(path)
	if err != nil {
		return r
	}
	var ft *FlatTree
	if mode == "raw" {
		tree, err := parser.ParseCtx(context.Background(), nil, content)
		if err != nil {
			return r
		}
		ft = flattenTree(tree.RootNode())
	} else {
		f, err := ParseKotlinFile(context.Background(), parser, path, content)
		if err != nil {
			return r
		}
		ft = f.FlatTree
	}
	e, m, nd := corpusErrorCounts(ft)
	r.errCount, r.missingCount = e, m
	r.err, r.miss, r.nonDecl = e > 0, m > 0, nd > 0
	for c := ft.FirstChildren[0]; c != 0; c = ft.NextSibs[c] {
		ty := nodeTypeName(ft.Types[c])
		if ty != "prefix_expression" && ty != "infix_expression" {
			continue
		}
		txt := content[ft.StartBytes[c]:ft.EndBytes[c]]
		if bytes.Contains(txt, []byte("fun ")) || bytes.Contains(txt, []byte("class ")) {
			prev := "<none>"
			if pc := ft.PrevSibs[c]; pc != 0 {
				prev = nodeTypeName(ft.Types[pc])
			}
			r.bad = true
			r.shape = prev + " -> " + ty
			break
		}
	}
	if r.err || r.miss || r.nonDecl || r.bad {
		r.hasLine = true
		r.line = fmt.Sprintf("%s err=%d miss=%d nondecl=%d %s", path, e, m, nd, firstProblem(ft, content))
	}
	return r
}

// firstProblem describes the first ERROR/MISSING node or non-declaration
// top-level node as "row:col type(parent) line-text".
func firstProblem(ft *FlatTree, content []byte) string {
	pick := uint32(0)
	for i := 1; i < len(ft.Types); i++ {
		if ft.Flags[i]&flatNodeFlagIsError == 0 || inExplicitBackingField(ft, uint32(i)) {
			continue
		}
		// Prefer the innermost recovery node: its text is where parsing broke.
		leaf := true
		for c := ft.FirstChildren[i]; c != 0; c = ft.NextSibs[c] {
			if ft.Flags[c]&flatNodeFlagIsError != 0 {
				leaf = false
				break
			}
		}
		if leaf {
			pick = uint32(i)
			break
		}
	}
	if pick == 0 {
		for c := ft.FirstChildren[0]; c != 0; c = ft.NextSibs[c] {
			if !corpusTopLevelOK[nodeTypeName(ft.Types[c])] {
				pick = c
				break
			}
		}
	}
	if pick == 0 {
		return ""
	}
	at := ft.StartBytes[pick]
	if ft.Flags[pick]&flatNodeFlagIsError != 0 && ft.StartBytes[pick] == ft.EndBytes[pick] && at > 0 {
		at-- // MISSING nodes sit after the token that precedes the gap
	}
	ls := bytes.LastIndexByte(content[:at], '\n') + 1
	le := bytes.IndexByte(content[ls:], '\n')
	if le < 0 {
		le = len(content) - ls
	}
	text := strings.TrimSpace(string(content[ls : ls+le]))
	if len(text) > 120 {
		text = text[:120]
	}
	parent := "<root>"
	if p := ft.Parents[pick]; p != 0 || pick != 0 {
		parent = nodeTypeName(ft.Types[ft.Parents[pick]])
	}
	return fmt.Sprintf("%d %s(%s) | %s", ft.StartRows[pick]+1, nodeTypeName(ft.Types[pick]), parent, text)
}

func inExplicitBackingField(ft *FlatTree, i uint32) bool {
	for n := i; n != 0; n = ft.Parents[n] {
		if nodeTypeName(ft.Types[n]) == ExplicitBackingFieldNodeType {
			return true
		}
	}
	return false
}

// corpusTopLevelOK lists top-level node types that are not misparsed
// expressions.
var corpusTopLevelOK = map[string]bool{
	"package_header": true, "import_list": true, "import_header": true,
	"class_declaration": true, "object_declaration": true, "function_declaration": true,
	"property_declaration": true, "type_alias": true, "typealias_declaration": true,
	"line_comment": true, "multiline_comment": true, "block_comment": true,
	"annotation": true, "file_annotation": true, "shebang_line": true,
	"secondary_constructor": true, "companion_object": true,
	"anonymous_initializer": true, "ERROR": true, "getter": true, "setter": true, ExplicitBackingFieldNodeType: true,
}

// corpusErrorCounts returns the ERROR node count, the MISSING node count and
// the number of top-level nodes that are neither declarations nor trivia.
// Nodes inside a recovered explicit backing field are expected and skipped.
func corpusErrorCounts(ft *FlatTree) (errs, missing, nonDecl int) {
	if len(ft.Types) == 0 {
		return
	}
	for i := range ft.Types {
		if ft.Flags[i]&flatNodeFlagIsError == 0 || inExplicitBackingField(ft, uint32(i)) {
			continue
		}
		if ft.StartBytes[i] == ft.EndBytes[i] {
			missing++
		} else {
			errs++
		}
	}
	for c := ft.FirstChildren[0]; c != 0; c = ft.NextSibs[c] {
		if !corpusTopLevelOK[nodeTypeName(ft.Types[c])] {
			nonDecl++
		}
	}
	return
}
