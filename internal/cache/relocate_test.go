package cache

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// relocationTempDir is a temp dir whose path holds no rule path marker:
// t.TempDir() is named after the test, and "test" is one.
func relocationTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "krit-reloc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if rootAffectsRuleVerdicts(dir) {
		t.Skipf("temp dir %s contains a rule path marker", dir)
	}
	return dir
}

// seedRelocationRepo writes a two-file repository at root and an analysis
// cache for it under root/.krit/cache, as a scan there would leave it.
func seedRelocationRepo(t *testing.T, root string) (cachePath string, files []string) {
	t.Helper()
	return seedRelocationRepoMarked(t, root, true)
}

// seedRelocationRepoMarked is seedRelocationRepo with the project marker
// (settings.gradle.kts) optional.
func seedRelocationRepoMarked(t *testing.T, root string, marked bool) (cachePath string, files []string) {
	t.Helper()
	if marked {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte("rootProject.name = \"repo\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, body := range map[string]string{"app/src/A.kt": "class A\n", "app/src/B.kt": "class B\n"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	c := &Cache{RuleHash: "rh", ScanPaths: []string{root}, Files: map[string]FileEntry{}}
	for _, p := range files {
		cols := scanner.CollectFindings([]scanner.Finding{{File: p, Line: 1, Col: 1, Rule: "R", RuleSet: "s", Severity: "warning", Message: "m"}})
		c.UpdateEntryColumns(p, &cols)
	}
	cachePath = FilePath(DefaultDir(root), []string{root})
	if err := c.Save(cachePath); err != nil {
		t.Fatal(err)
	}
	return cachePath, files
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// #1662: a .krit/cache restored into a checkout at another absolute path
// (another CI agent's workspace) hits for unchanged files, and its findings
// name the new checkout.
func TestAnalysisCacheHitsAfterRelocatingCheckout(t *testing.T) {
	base := relocationTempDir(t)
	rootA, rootB := filepath.Join(base, "agent-1", "repo"), filepath.Join(base, "agent-2", "repo")
	cacheA, _ := seedRelocationRepo(t, rootA)
	_, filesB := seedRelocationRepo(t, rootB) // the same checkout at B ...
	cacheB := FilePath(DefaultDir(rootB), []string{rootB})
	if filepath.Base(cacheA) != filepath.Base(cacheB) {
		t.Fatalf("cache file names differ across roots: %s vs %s", filepath.Base(cacheA), filepath.Base(cacheB))
	}
	copyFile(t, cacheA, cacheB) // ... with agent 1's cache restored

	c := Load(cacheB)
	res := c.CheckFiles(filesB, "rh", rootB)
	if res.TotalCached != len(filesB) {
		t.Fatalf("relocated cache served %d of %d files", res.TotalCached, len(filesB))
	}
	for _, f := range res.CachedColumns.Findings() {
		if !strings.HasPrefix(f.File, rootB) {
			t.Fatalf("a cached finding names %s, not the new checkout", f.File)
		}
	}
}

// Rules apply path heuristics to absolute paths, so a cache must not follow
// a checkout into (or out of) a root where one could match.
func TestAnalysisCacheDiscardedWhenMovedUnderAHeuristicRoot(t *testing.T) {
	base := relocationTempDir(t)
	rootA, rootB := filepath.Join(base, "agent-1", "repo"), filepath.Join(base, "samples", "repo")
	cacheA, _ := seedRelocationRepo(t, rootA)
	seedRelocationRepo(t, rootB) // the same checkout under samples/
	cacheB := FilePath(DefaultDir(rootB), []string{rootB})
	copyFile(t, cacheA, cacheB)
	if c := Load(cacheB); len(c.Files) != 0 {
		t.Fatalf("a cache moved under samples/ kept %d entries", len(c.Files))
	}
}

// Without a project marker, rules read absolute paths (the root is only the
// scanned directory), so the cache keeps absolute paths and a moved copy
// misses instead of serving verdicts computed at the old path.
func TestAnalysisCacheNotRelocatedWithoutProjectMarker(t *testing.T) {
	base := relocationTempDir(t)
	rootA, rootB := filepath.Join(base, "agent-1", "repo"), filepath.Join(base, "agent-2", "repo")
	cacheA, _ := seedRelocationRepoMarked(t, rootA, false)
	_, filesB := seedRelocationRepoMarked(t, rootB, false)
	cacheB := FilePath(DefaultDir(rootB), []string{rootB})
	copyFile(t, cacheA, cacheB)
	if res := Load(cacheB).CheckFiles(filesB, "rh", rootB); res.TotalCached != 0 {
		t.Fatalf("an unmarked checkout's cache served %d files after a move", res.TotalCached)
	}
}

// A cache written before the project was marked holds verdicts computed on
// absolute paths; once a marker makes rules read root-relative paths, it is
// discarded rather than served (#1834). Without a marker it keeps hitting.
func TestAnalysisCacheDiscardedWhenRootBecomesMarked(t *testing.T) {
	root := filepath.Join(relocationTempDir(t), "repo")
	cachePath, files := seedRelocationRepoMarked(t, root, false)
	if res := Load(cachePath).CheckFiles(files, "rh", root); res.TotalCached != len(files) {
		t.Fatalf("unmarked cache served %d of %d files at its own unmarked root", res.TotalCached, len(files))
	}

	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte("rootProject.name = \"repo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if marked := FilePath(DefaultDir(root), []string{root}); marked != cachePath {
		copyFile(t, cachePath, marked) // the same file under whatever name the marked root uses
		cachePath = marked
	}
	if c := Load(cachePath); len(c.Files) != 0 {
		t.Fatalf("a cache written before the root was marked kept %d entries", len(c.Files))
	}
}

// A cache outside the repository's default directory (--cache-dir) keeps
// absolute paths, as before.
func TestAnalysisCacheElsewhereKeepsAbsolutePaths(t *testing.T) {
	if got := relocationRoot(filepath.Join(t.TempDir(), "custom", "krit.cache")); got != "" {
		t.Fatalf("relocationRoot of a --cache-dir file = %q, want \"\"", got)
	}
}

// Every path segment a built-in rule tests a file path for must hit
// rulePathMarkers, so a root holding it is never relocated. Adding a rule
// with a new path heuristic fails here: add a marker that covers it.
func TestRulePathMarkersCoverRules(t *testing.T) {
	pathLine := regexp.MustCompile(`(?i)path|\.File\b|filename`)
	literal := regexp.MustCompile(`"([-/a-zA-Z0-9_.]{2,40})"`)
	marker := regexp.MustCompile(`^(/|-)?[-a-z0-9_.]+(/[-a-z0-9_.]+)*/?$`)
	checked := 0
	// The rules, plus the shared path classifiers they call.
	shared := map[string]bool{
		filepath.Join("..", "scanner", "testpath.go"):      true,
		filepath.Join("..", "fileignore", "docsample.go"):  true,
		filepath.Join("..", "fileignore", "fileignore.go"): true,
	}
	err := filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		if !shared[p] && !strings.HasPrefix(p, filepath.Join("..", "rules")+string(filepath.Separator)) {
			return nil
		}
		if p == filepath.Join("..", "rules", "api", "docs.go") { // builds docs URLs, not path heuristics
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if !pathLine.MatchString(line) {
				continue
			}
			// A heuristic reading the project-relative path (projectPath,
			// held in a variable named inProject) cannot see the root's
			// folders, so its segments need no marker.
			if strings.Contains(line, "projectPath(") || strings.Contains(line, "inProject") {
				continue
			}
			// LastIndex finds the innermost segment (res/<folder>/ of a
			// resource file), which the folders above the root cannot supply;
			// len("...") is the offset arithmetic that follows it.
			if strings.Contains(line, "LastIndex(") || strings.Contains(line, "len(\"") {
				continue
			}
			for _, m := range literal.FindAllStringSubmatch(line, -1) {
				lit := strings.ToLower(m[1])
				// An unanchored literal counts only as a folder-name suffix
				// ("Main/" matches jvmMain/); "path/filepath" is an import.
				if !strings.HasPrefix(lit, "/") && !strings.HasPrefix(lit, "-") && !strings.HasSuffix(lit, "/") {
					continue
				}
				// A marker names a directory; a file name (".xml", ".kt") is
				// matched against the end of a path, never a root.
				if !marker.MatchString(lit) || strings.Count(lit, "/") < 1 || filepath.Ext(strings.TrimSuffix(lit, "/")) != "" && !strings.HasSuffix(lit, "/") {
					continue
				}
				if lit == "/" || lit == "//" || lit == "./" {
					continue
				}
				checked++
				probe := "/x/" + strings.Trim(lit, "/") + "/y"
				if !strings.HasPrefix(lit, "/") && !strings.HasPrefix(lit, "-") {
					probe = "/x/prefix" + strings.Trim(lit, "/") + "/y" // a suffix of a folder name
				}
				if !rootAffectsRuleVerdicts(probe) {
					t.Errorf("%s: path heuristic segment %q matches no rulePathMarkers entry", p, m[1])
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("found no path heuristic segments: the scan no longer recognizes the rules' heuristics")
	}
}

// A rule's path heuristic must read projectPath(ctx, file.Path), never the
// raw file.Path: the raw path holds the folders above the project root, so
// a checkout under /work/debug/ or /work/test/ would change the verdict,
// and a relocated cache entry would disagree with a fresh scan (#1814).
// The scan flags every call in internal/rules that passes a raw source or
// manifest path (file.Path, ctx.File.Path, m.Path, or a local holding one
// of them after filepath.ToSlash, strings.ToLower, ...) to a path
// heuristic: scanner.IsTestFile, a strings.Contains-style test against a
// folder literal, or a rules function whose body runs either on a string
// parameter.
func TestRulePathHeuristicsReadProjectPath(t *testing.T) {
	fset := token.NewFileSet()
	type parsed struct {
		dir  string
		file *ast.File
	}
	var files []parsed
	consts := map[string]string{} // package-level string constants, by package dir + "." + name
	err := filepath.WalkDir(filepath.Join("..", "rules"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "testdata" || name == "ruletest" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		dir := filepath.Dir(p)
		files = append(files, parsed{dir, f})
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if s, err := strconv.Unquote(lit.Value); err == nil {
							consts[dir+"."+name.Name] = s
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("found no internal/rules sources")
	}

	// folderLiteral reports whether e is a string naming a folder
	// ("/debug/", "-test", "/src/test/", "/spotless/"), not a file name,
	// an extension or a bare separator.
	folderLiteral := func(dir string, e ast.Expr) bool {
		var s string
		switch v := e.(type) {
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return false
			}
			u, err := strconv.Unquote(v.Value)
			if err != nil {
				return false
			}
			s = u
		case *ast.Ident:
			c, ok := consts[dir+"."+v.Name]
			if !ok {
				return false
			}
			s = c
		default:
			return false
		}
		lit := strings.ToLower(s)
		if strings.Trim(lit, "/.") == "" {
			return false
		}
		if !strings.HasPrefix(lit, "/") && !strings.HasPrefix(lit, "-") && !strings.HasSuffix(lit, "/") {
			return false
		}
		// A file name or extension is matched against a path's end.
		return strings.HasSuffix(lit, "/") || strings.HasPrefix(lit, "/.") || filepath.Ext(lit) == ""
	}
	calleeName := func(call *ast.CallExpr) string {
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			return fn.Name
		case *ast.SelectorExpr:
			if x, ok := fn.X.(*ast.Ident); ok {
				return x.Name + "." + fn.Sel.Name
			}
		}
		return ""
	}
	methodName := func(call *ast.CallExpr) string {
		if fn, ok := call.Fun.(*ast.SelectorExpr); ok {
			return fn.Sel.Name
		}
		return ""
	}
	stringsTests := map[string]bool{
		"strings.Contains": true, "strings.HasPrefix": true, "strings.HasSuffix": true,
		"strings.Index": true, "strings.LastIndex": true, "strings.Count": true,
	}
	// Wrappers keep their argument's folders.
	wrappers := map[string]bool{
		"filepath.ToSlash": true, "filepath.FromSlash": true, "filepath.Clean": true, "filepath.Dir": true,
		"filepath.Abs": true, "strings.ToLower": true, "strings.ReplaceAll": true, "strings.TrimPrefix": true,
		"path.Clean": true, "path.Dir": true, "scanner.LowerPath": true, "scanner.LowerSlashPath": true,
		"strings.Split": true, "strings.Fields": true,
	}
	external := map[string]bool{
		"scanner.IsTestFile": true, "fileignore.DocSamplePath": true, "fileignore.TestDataPath": true,
		"fileignore.DefaultPrunedWithinRepo": true,
	}
	// funcs and methods hold the rules functions that test folder names,
	// by package dir + "." + name.
	funcs, methods := map[string]bool{}, map[string]bool{}
	isHeuristicCall := func(dir string, call *ast.CallExpr) bool {
		name := calleeName(call)
		if external[name] || funcs[dir+"."+name] || methods[dir+"."+methodName(call)] {
			return true
		}
		if stringsTests[name] {
			for _, a := range call.Args {
				if folderLiteral(dir, a) {
					return true
				}
			}
		}
		return false
	}
	// carries reports whether e holds a value of locals, or one isSource
	// accepts, possibly lowered, slash-converted or trimmed.
	var carries func(e ast.Expr, locals map[string]bool, isSource func(ast.Expr) bool) bool
	carries = func(e ast.Expr, locals map[string]bool, isSource func(ast.Expr) bool) bool {
		if isSource(e) {
			return true
		}
		switch v := e.(type) {
		case *ast.ParenExpr:
			return carries(v.X, locals, isSource)
		case *ast.Ident:
			return locals[v.Name]
		case *ast.BinaryExpr: // "/" + path
			return v.Op == token.ADD && (carries(v.X, locals, isSource) || carries(v.Y, locals, isSource))
		case *ast.IndexExpr: // segments[i]
			return carries(v.X, locals, isSource)
		case *ast.SliceExpr: // path[idx+5:]
			return carries(v.X, locals, isSource)
		case *ast.CallExpr:
			if wrappers[calleeName(v)] && len(v.Args) > 0 {
				return carries(v.Args[0], locals, isSource)
			}
		}
		return false
	}
	// heuristicArgs calls report for every argument of a heuristic call in
	// body that carries a value of locals (updated through assignments) or
	// one isSource accepts.
	heuristicArgs := func(dir string, body *ast.BlockStmt, locals map[string]bool, isSource func(ast.Expr) bool, report func(call *ast.CallExpr, arg ast.Expr)) {
		ast.Inspect(body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.RangeStmt: // for _, seg := range segments
				if id, ok := v.Value.(*ast.Ident); ok {
					locals[id.Name] = carries(v.X, locals, isSource)
				}
			case *ast.AssignStmt:
				if len(v.Rhs) == len(v.Lhs) {
					for i, lhs := range v.Lhs {
						if id, ok := lhs.(*ast.Ident); ok {
							locals[id.Name] = carries(v.Rhs[i], locals, isSource)
						}
					}
				}
			case *ast.CallExpr:
				if isHeuristicCall(dir, v) {
					for _, a := range v.Args {
						if carries(a, locals, isSource) {
							report(v, a)
						}
					}
				}
			}
			return true
		})
	}
	noSource := func(ast.Expr) bool { return false }

	// A function that runs a heuristic on one of its string parameters is
	// one; repeat until no more are found (printlnNonProductionPath calls
	// scanner.IsTestFile on its path).
	for changed := true; changed; {
		changed = false
		for _, pf := range files {
			for _, decl := range pf.file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				params := stringParams(fd)
				if len(params) == 0 {
					continue
				}
				set := funcs
				if fd.Recv != nil {
					set = methods
				}
				key := pf.dir + "." + fd.Name.Name
				if set[key] {
					continue
				}
				heuristicArgs(pf.dir, fd.Body, params, noSource, func(*ast.CallExpr, ast.Expr) {
					if !set[key] {
						set[key] = true
						changed = true
					}
				})
			}
		}
	}
	if len(funcs) == 0 {
		t.Fatal("found no path heuristic helpers: the scan no longer recognizes them")
	}

	// rawPath reports whether e is a source or manifest path as the
	// dispatcher holds it: absolute, folders above the root included.
	rawPath := func(e ast.Expr) bool {
		v, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		owner := ""
		switch x := v.X.(type) {
		case *ast.Ident:
			owner = x.Name
		case *ast.SelectorExpr:
			owner = x.Sel.Name
		}
		owner = strings.ToLower(owner)
		switch v.Sel.Name {
		case "Path": // file.Path, ctx.File.Path, cls.file.Path, m.Path
			return strings.HasSuffix(owner, "file") || owner == "f" || owner == "m" || strings.HasSuffix(owner, "manifest")
		case "File": // an index symbol's or finding's path (sym.File); ctx.File is the parsed file
			return owner != "ctx"
		}
		return false
	}
	for _, pf := range files {
		for _, decl := range pf.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			heuristicArgs(pf.dir, fd.Body, map[string]bool{}, rawPath, func(call *ast.CallExpr, arg ast.Expr) {
				t.Errorf("%s: %s reads folder names from the raw path %s, which holds the folders above the project root; pass projectPath(ctx, file.Path) instead, threading ctx into the helper if it lacks one",
					fset.Position(arg.Pos()), exprString(fset, call.Fun), exprString(fset, arg))
			})
		}
	}
}

// stringParams returns the names of fd's string parameters.
func stringParams(fd *ast.FuncDecl) map[string]bool {
	params := map[string]bool{}
	for _, field := range fd.Type.Params.List {
		if id, ok := field.Type.(*ast.Ident); ok && id.Name == "string" {
			for _, name := range field.Names {
				params[name.Name] = true
			}
		}
	}
	return params
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, e); err != nil {
		return "?"
	}
	return b.String()
}

// Common CI workspace roots must stay relocatable.
func TestCommonCIRootsAreRelocatable(t *testing.T) {
	for _, root := range []string{
		"/home/runner/work/app/app", "/builds/group/app", "/workspaces/app",
		"/var/lib/jenkins/workspace/app", "/Users/runner/work/app/app", `C:\a\app\app`,
	} {
		if rootAffectsRuleVerdicts(root) {
			t.Errorf("%s is treated as a heuristic root", root)
		}
	}
}

// A path under the root is stored repository-relative with forward slashes
// and re-rooted on load; any other path is stored exactly as given, so an
// outside-root Windows path keeps its separators and still hits (#1806).
func TestRelocatePathRoundTrip(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "ci1", "app")
	inside := filepath.Join(root, "sub", "Foo.kt")
	if got := relocatePath(root, inside); got != "sub/Foo.kt" {
		t.Fatalf("relocatePath(inside) = %q, want sub/Foo.kt", got)
	}
	moved := filepath.Join(string(filepath.Separator), "ci2", "app@2")
	if got := rerootPath(moved, relocatePath(root, inside)); got != filepath.Join(moved, "sub", "Foo.kt") {
		t.Fatalf("re-rooted inside path = %q", got)
	}
	for _, outside := range []string{
		filepath.Join(string(filepath.Separator), "elsewhere", "lib", "Bar.kt"),
		root + "-sibling" + string(filepath.Separator) + "Baz.kt",
		filepath.Join("relative", "Qux.kt"),
	} {
		if got := relocatePath(root, outside); got != outside {
			t.Errorf("relocatePath(%q) = %q, want it unchanged", outside, got)
		}
		if got := rerootPath(moved, relocatePath(root, outside)); filepath.IsAbs(outside) && got != outside {
			t.Errorf("round trip of %q = %q, want it unchanged", outside, got)
		}
	}
}

// The cache file a build wrote under the absolute-path name is removed once
// the repository-relative name takes over, instead of lingering forever.
func TestResolveCacheDirRemovesLegacyCacheFile(t *testing.T) {
	root := relocationTempDir(t)
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte("rootProject.name = \"repo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := DefaultDir(root)
	legacy := legacyFilePath(dir, []string{root})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, current := ResolveCacheDir("", []string{root})
	if current == legacy {
		t.Skip("legacy and relocatable names coincide for this path")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy cache file %s still exists (stat err %v)", legacy, err)
	}
}
