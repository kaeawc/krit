package rules

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/ruleslinter"
)

// rulesPkgPath is this package's import path, as runtime function names spell it.
var rulesPkgPath = reflect.TypeOf(LibraryCustomViewRule{}).PkgPath()

// TestRulesPackageReadersDeclareBuildInfo is the build-info reader gate of make lint-rules (#1943): a resource rule
// whose check functions reach a build-file helper (ruleslinter.BuildInfoHelpers) implements ResourceBuildInfoReader,
// so the resource cache keys cover the build files it reads; every reader reaches one; and no resource-source or icon
// rule reaches one. It runs here, in the test binary make lint-rules links anyway, because the rules' scopes and
// check functions come from api.Registry; ruleslinter.AnalyzeBuildInfoReach does the static analysis.
func TestRulesPackageReadersDeclareBuildInfo(t *testing.T) {
	t.Parallel()
	_, thisFile, _, _ := runtime.Caller(0)
	rulesDir := filepath.Dir(thisFile)
	start := time.Now()
	roots := registryBuildInfoRoots(t)
	violations, err := ruleslinter.AnalyzeBuildInfoReach(rulesDir, roots)
	if err != nil {
		t.Fatalf("AnalyzeBuildInfoReach(%q): %v", rulesDir, err)
	}
	scopes := map[string]int{}
	for _, root := range roots {
		scopes[root.Scope]++
	}
	t.Logf("build-info gate: %d rules (%v) in %v", len(roots), scopes, time.Since(start).Round(time.Millisecond))
	if len(violations) == 0 {
		return
	}
	var b strings.Builder
	for _, v := range violations {
		b.WriteString(v.String())
		b.WriteByte('\n')
	}
	t.Fatalf("ruleslinter found %d build-info reader violation(s):\n%s", len(violations), b.String())
}

// registryBuildInfoRoots lists the registered resource, resource-source and icon rules with the functions the
// dispatcher calls on them, as the runtime names them.
func registryBuildInfoRoots(t *testing.T) []ruleslinter.BuildInfoRoot {
	t.Helper()
	var roots []ruleslinter.BuildInfoRoot
	for _, r := range api.Registry {
		scope := RuleScope(r).String()
		switch scope {
		case ruleslinter.BuildInfoScopeResource, ruleslinter.BuildInfoScopeResourceSource, ruleslinter.BuildInfoScopeIcons:
		default:
			continue
		}
		root := ruleslinter.BuildInfoRoot{Rule: r.ID, Scope: scope}
		if typ := reflect.TypeOf(r.Implementation); typ != nil {
			if typ.Kind() == reflect.Pointer {
				typ = typ.Elem()
			}
			if typ.PkgPath() == rulesPkgPath {
				root.Impl = typ.Name()
			}
		}
		if _, ok := r.Implementation.(ResourceBuildInfoReader); ok && root.Impl == "" {
			t.Fatalf("rule %s implements ResourceBuildInfoReader through %T, outside package rules; the gate cannot check it", r.ID, r.Implementation)
		}
		fns := []any{r.Check, r.FileGate}
		if r.Aggregate != nil {
			fns = append(fns, r.Aggregate.Collect, r.Aggregate.Finalize)
		}
		for _, fn := range fns {
			v := reflect.ValueOf(fn)
			if v.IsNil() {
				continue
			}
			f := runtime.FuncForPC(v.Pointer())
			name, ok := strings.CutPrefix(f.Name(), rulesPkgPath+".")
			if !ok {
				t.Fatalf("rule %s: %s is not a function of package rules; the gate cannot follow it", r.ID, f.Name())
			}
			file, line := f.FileLine(f.Entry())
			root.Funcs = append(root.Funcs, ruleslinter.RuntimeFunc{Name: name, File: file, Line: line})
		}
		roots = append(roots, root)
	}
	if len(roots) == 0 {
		t.Fatal("no resource, resource-source or icon rule is registered")
	}
	return roots
}
