package prepush

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// rulesSourceGates lists, as "<package>:<Test>", the Test functions outside
// internal/rules that read internal/rules' source files: a filepath.Join of
// ".." segments ending in "rules", or a "../rules" literal. They are static
// gates on the rules' code, so a rule change can break them while their own
// package is unchanged (#1896).
func rulesSourceGates(t *testing.T, root string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var gates []string
	internal := filepath.Join(root, "internal")
	err := filepath.WalkDir(internal, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// internal/rules runs in full on a rule change.
			if d.Name() == "testdata" || p == filepath.Join(internal, "rules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			if readsRulesSource(fn.Body) {
				gates = append(gates, filepath.Base(filepath.Dir(p))+":"+fn.Name.Name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(gates)
	return gates
}

func readsRulesSource(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch v := n.(type) {
		case *ast.BasicLit:
			if s, err := strconv.Unquote(v.Value); err == nil && v.Kind == token.STRING &&
				(s == "../rules" || strings.HasPrefix(s, "../rules/")) {
				found = true
			}
		case *ast.CallExpr:
			sel, ok := v.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Join" {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); !ok || (x.Name != "filepath" && x.Name != "path") {
				return true
			}
			ups := 0
			for _, a := range v.Args {
				lit, ok := a.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					break
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					break
				}
				if s == ".." {
					ups++
					continue
				}
				if s == "rules" && ups > 0 {
					found = true
				}
				break
			}
		}
		return true
	})
	return found
}

// TestRuleChangePlansRulesSourceGates fails when a test outside
// internal/rules scans the rules' sources but the pre-push plan for a rule
// change doesn't run it, so only CI's `go test ./...` would (#1896).
func TestRuleChangePlansRulesSourceGates(t *testing.T) {
	requireBashAndGit(t)
	root := repoRoot(t)
	gates := rulesSourceGates(t, root)
	if len(gates) == 0 {
		t.Fatal("found no tests outside internal/rules that read its sources; internal/cache's TestRulePathHeuristicsReadProjectPath should be one, so readsRulesSource no longer recognizes it")
	}
	cmd := exec.Command("bash", filepath.Join(root, "scripts", "pre-push.sh"), "--dry-run")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"KRIT_PREPUSH_FILES=internal/rules/accessibility.go",
		"KRIT_PREPUSH_SKIP=winvet",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pre-push.sh --dry-run: %v\n%s", err, out)
	}
	planned := map[string]bool{}
	for _, line := range trimLines(string(out)) {
		if rest, ok := strings.CutPrefix(line, "test "); ok {
			for _, target := range strings.Fields(rest) {
				planned[target] = true
			}
		}
	}
	for _, gate := range gates {
		if !planned[gate] {
			pkg, _, _ := strings.Cut(gate, ":")
			t.Errorf("%s reads internal/rules sources, but a rule change's pre-push plan doesn't run it: in scripts/pre-push.sh, print %q from test_targets under the internal/rules match, and give run_test a %s: case if it has none", gate, gate, pkg)
		}
	}
}
