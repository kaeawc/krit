package ruleslinter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// BuildInfoHelpers are the rules-package functions that read build files for a res directory: module build scripts,
// the version catalogs their aliases resolve through, settings scripts and manifests. A resource rule whose check
// methods reach one has findings that depend on those files, so it must implement rules.ResourceBuildInfoReader for
// its cache keys to cover them (#1883, #1943). AnalyzeBuildInfoReach checks that and fails when a listed helper no
// longer exists, so a rename or removal updates this list too. This is the one list of them; a new helper that reads
// build files for a resource rule joins it.
var BuildInfoHelpers = []string{
	// The module build script that owns a manifest or res path, and the catalogs its aliases resolve through.
	"lookupManifestGradleInfo",
	// A directory's build script and its catalogs.
	"readManifestGradleInfo",
	// Version catalogs: plugin aliases (catalogPluginIDs) and dependency aliases.
	"catalogPluginIDs",
	"catalogDeclaresDependency",
	// The module minSdk of a res file: its build script, then the manifest beside the res directory.
	"resourceModuleMinSdk",
	// Namespace, applicationId and plugins of the module owning a res directory: build script, catalogs, manifests.
	"readModuleIdentity",
	// minSdk and useSupportLibrary of a module's build script.
	"readModuleVectorSettings",
	// The Gradle path of a module, from the nearest settings script, and the settings scripts it searches.
	"gradleModulePath",
	"gradleSettingsCandidates",
	// A parsed manifest file.
	"manifestRoot",
	// The module manifest TranslucentOrientation reads for a res directory.
	"translucentManifestPath",
	// The manifest and build scripts beside a res root that the unused-resource graph scans for references.
	"unusedResourceRootFiles",
}

// BuildInfoCovered are rules-package functions that read build files only where every res directory's fingerprint
// already hashes them, so a rule that reaches a helper through them needs no ResourceBuildInfoReader. The call graph
// does not walk into them. Each states what covers it; the gate fails when one no longer exists.
var BuildInfoCovered = map[string]string{
	// It reads the res directory's source-set manifest and the module's main one (manifestRoot), which
	// resDirManifestFingerprint (internal/pipeline/android_fingerprint.go) hashes into the res-dir fingerprint.
	"wearIsWearResDir": "resDirManifestFingerprint hashes the source-set and main manifests it reads",
}

// BuildInfoReaderInterface is the rules-package interface a resource rule that reads build files implements.
const BuildInfoReaderInterface = "ResourceBuildInfoReader"

// The dispatch scopes the build-info gate checks, as api.Scope.String() names them. Other scopes are skipped.
const (
	BuildInfoScopeResource       = "resource"
	BuildInfoScopeResourceSource = "resource-source"
	BuildInfoScopeIcons          = "icons"
)

// buildInfoLimit states what the call graph does not follow, so a clean gate is read for what it proves.
const buildInfoLimit = "the gate follows static calls, method calls, closures, functions named in reachable code " +
	"and interface calls to every implementation in the package; calls through function values stored elsewhere " +
	"(fields, parameters, maps) are not followed"

// BuildInfoRoot is a registered rule the build-info gate checks: its ID, dispatch scope, the type of its
// Implementation in the analyzed package ("" when none is), and the functions the dispatcher calls (Check, FileGate,
// and an Aggregate's Collect and Finalize).
type BuildInfoRoot struct {
	Rule  string
	Scope string
	Impl  string
	Funcs []RuntimeFunc
}

// RuntimeFunc identifies a function value as runtime.FuncForPC reports it: its name relative to the package
// ("(*FooRule).check-fm", "checkFoo", "init.func3"), and the file and line of its entry, which locate a closure.
type RuntimeFunc struct {
	Name string
	File string
	Line int
}

// AnalyzeBuildInfoReach type-checks the Go package in dir and walks its call graph (callGraph) from each root's
// functions.
// It reports a resource-scope rule that reaches a BuildInfoHelpers function without implementing
// ResourceBuildInfoReader, one that implements it and reaches none (the reads it declares go through a helper missing
// from BuildInfoHelpers, or it reads nothing), and a resource-source or icon rule that reaches one at all: their cache
// keys take no build info, whatever the rule implements.
func AnalyzeBuildInfoReach(dir string, roots []BuildInfoRoot) ([]Violation, error) {
	prog, err := loadTypedPackage(dir)
	if err != nil {
		return nil, err
	}
	return analyzeBuildInfoReach(prog, roots, BuildInfoHelpers, BuildInfoCovered)
}

func analyzeBuildInfoReach(prog *typedPackage, roots []BuildInfoRoot, helperNames []string, coveredNames map[string]string) ([]Violation, error) {
	helpers := map[*types.Func]string{}
	var missing []string
	for _, name := range helperNames {
		if fn := prog.lookupFunc(name); fn != nil {
			helpers[fn] = name
		} else {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("ruleslinter: BuildInfoHelpers lists %s, not functions of package %s; update the list", strings.Join(missing, ", "), prog.pkg.Name())
	}
	covered := map[*types.Func]bool{}
	for name := range coveredNames {
		fn := prog.lookupFunc(name)
		if fn == nil {
			return nil, fmt.Errorf("ruleslinter: BuildInfoCovered lists %s, not a function of package %s; update the list", name, prog.pkg.Name())
		}
		covered[fn] = true
	}
	reader, _ := prog.pkg.Scope().Lookup(BuildInfoReaderInterface).(*types.TypeName)
	if reader == nil || !types.IsInterface(reader.Type()) {
		return nil, fmt.Errorf("ruleslinter: package %s declares no interface %s", prog.pkg.Name(), BuildInfoReaderInterface)
	}
	iface := reader.Type().Underlying().(*types.Interface)

	g := newCallGraph(prog)
	var out []Violation
	for _, root := range roots {
		if root.Scope != BuildInfoScopeResource && root.Scope != BuildInfoScopeResourceSource && root.Scope != BuildInfoScopeIcons {
			continue
		}
		var starts []callNode
		for _, f := range root.Funcs {
			nodes, err := prog.resolveRuntimeFunc(f)
			if err != nil {
				return nil, fmt.Errorf("ruleslinter: rule %s: %w", root.Rule, err)
			}
			starts = append(starts, nodes...)
		}
		if len(starts) == 0 {
			continue
		}
		helper, path := g.firstHelper(starts, helpers, covered)
		if msg := buildInfoMessage(root.Scope, helper, path, prog.implements(root.Impl, iface)); msg != "" {
			out = append(out, Violation{RuleID: root.Rule, Position: g.position(starts[0]), Message: msg})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RuleID < out[j].RuleID })
	return out, nil
}

// buildInfoMessage is the violation of a rule in scope that reaches helper through path ("" for none) and implements
// ResourceBuildInfoReader or not; "" when it has none.
func buildInfoMessage(scope, helper string, path []string, implements bool) string {
	switch {
	case helper != "" && scope != BuildInfoScopeResource:
		return fmt.Sprintf("%s rule reaches build-file helper %s (%s), but %s cache keys take no build info: "+
			"ResourceBuildInfoReader is only read for resource-scope rules, so key that scope on what %s reads first (%s)",
			scope, helper, strings.Join(path, " -> "), scope, helper, buildInfoLimit)
	case helper != "" && !implements:
		return fmt.Sprintf("reaches build-file helper %s (%s) without implementing %s: "+
			"implement rules.ResourceBuildInfoReader so the build-info fingerprint is in this rule's cache keys (%s)",
			helper, strings.Join(path, " -> "), BuildInfoReaderInterface, buildInfoLimit)
	case helper == "" && implements:
		return fmt.Sprintf("implements %s but reaches none of ruleslinter.BuildInfoHelpers: add the helper it reads "+
			"build files through to that list, or drop the interface if it reads none (%s)", BuildInfoReaderInterface, buildInfoLimit)
	}
	return ""
}

// lookupFunc returns the package-level function name, or nil.
func (p *typedPackage) lookupFunc(name string) *types.Func {
	fn, _ := p.pkg.Scope().Lookup(name).(*types.Func)
	return fn
}

// implements reports whether the package type named impl, or a pointer to it, implements iface.
func (p *typedPackage) implements(impl string, iface *types.Interface) bool {
	tn, ok := p.pkg.Scope().Lookup(impl).(*types.TypeName)
	return ok && impl != "" && (types.Implements(tn.Type(), iface) || types.Implements(types.NewPointer(tn.Type()), iface))
}

// typedPackage is a parsed, type-checked package.
type typedPackage struct {
	fset  *token.FileSet
	files []*ast.File
	pkg   *types.Package
	info  *types.Info
	// decls maps the package's functions and methods to their declarations.
	decls map[*types.Func]*ast.FuncDecl
	// funcVars maps package-level variables initialized with a function literal to it.
	funcVars map[*types.Var]*ast.FuncLit
}

// loadTypedPackage parses the non-test Go files of dir that match the build context and type-checks them against
// the export data of its dependencies, which `go list -export` builds (or finds in the build cache) in the module
// that contains dir, while the files are parsed.
func loadTypedPackage(dir string) (*typedPackage, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	type listed struct {
		exports map[string]string
		err     error
	}
	exportsc := make(chan listed, 1)
	go func() {
		exports, err := exportFiles(dir)
		exportsc <- listed{exports, err}
	}()
	p := &typedPackage{fset: token.NewFileSet()}
	parseErr := p.parseDir(dir)
	listing := <-exportsc
	if parseErr != nil {
		return nil, parseErr
	}
	if listing.err != nil {
		return nil, listing.err
	}
	imp := importer.ForCompiler(p.fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := listing.exports[path]
		if !ok || file == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(file)
	})
	p.info = &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}}
	conf := types.Config{Importer: imp}
	if p.pkg, err = conf.Check(filepath.Base(dir), p.fset, p.files, p.info); err != nil {
		return nil, fmt.Errorf("ruleslinter: type-check %s: %w", dir, err)
	}
	p.indexDecls()
	return p, nil
}

// parseDir parses the non-test Go files of dir that match the build context, across the CPUs.
func (p *typedPackage) parseDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("ruleslinter: read dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if ok, err := build.Default.MatchFile(dir, name); err == nil && ok {
			names = append(names, name)
		}
	}
	p.files = make([]*ast.File, len(names))
	errs := make([]error, len(names))
	var wg sync.WaitGroup
	next := make(chan int)
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for i := range next {
				p.files[i], errs[i] = parser.ParseFile(p.fset, filepath.Join(dir, names[i]), nil, parser.SkipObjectResolution)
			}
		})
	}
	for i := range names {
		next <- i
	}
	close(next)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("ruleslinter: parse %s: %w", dir, err)
	}
	return nil
}

// indexDecls maps the package's functions and methods to their declarations, and its package-level variables
// initialized with a function literal to it.
func (p *typedPackage) indexDecls() {
	p.decls = map[*types.Func]*ast.FuncDecl{}
	p.funcVars = map[*types.Var]*ast.FuncLit{}
	for _, f := range p.files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if fn, ok := p.info.Defs[d.Name].(*types.Func); ok {
					p.decls[fn] = d
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					p.indexFuncVars(spec)
				}
			}
		}
	}
}

func (p *typedPackage) indexFuncVars(spec ast.Spec) {
	vs, ok := spec.(*ast.ValueSpec)
	if !ok || len(vs.Values) != len(vs.Names) {
		return
	}
	for i, name := range vs.Names {
		if lit, ok := vs.Values[i].(*ast.FuncLit); ok {
			if v, ok := p.info.Defs[name].(*types.Var); ok {
				p.funcVars[v] = lit
			}
		}
	}
}

// exportFiles maps the import path of each package the package in dir imports to its export data file, from
// `go list -export` run there. Only direct imports are needed: a package's export data covers the types it uses from
// its own dependencies. The package itself is not listed with -export, which would build its export data too.
func exportFiles(dir string) (map[string]string, error) {
	listed, err := goList(dir, "-f", "{{join .Imports \"\\n\"}}", ".")
	if err != nil {
		return nil, err
	}
	var imports []string
	for _, path := range strings.Fields(listed) {
		if path != "C" && path != "unsafe" {
			imports = append(imports, path)
		}
	}
	out := map[string]string{}
	if len(imports) == 0 {
		return out, nil
	}
	listed, err = goList(dir, append([]string{"-export", "-f", "{{.ImportPath}}\t{{.Export}}"}, imports...)...)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimSpace(listed), "\n") {
		if path, file, ok := strings.Cut(line, "\t"); ok {
			out[path] = file
		}
	}
	return out, nil
}

func goList(dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "go", append([]string{"list"}, args...)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ruleslinter: go list %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(stdout), nil
}

var runtimeMethodName = regexp.MustCompile(`^\(\*?([A-Za-z_]\w*)(?:\[[^\]]*\])?\)\.([A-Za-z_]\w*)$`)

// resolveRuntimeFunc maps a runtime function name to the declarations it runs: a function or method by name, or the
// function literals that start on its entry line for a closure ("init.func3", "(*FooRule).register.func1").
func (p *typedPackage) resolveRuntimeFunc(f RuntimeFunc) ([]callNode, error) {
	name := strings.TrimSuffix(f.Name, "-fm")
	if m := runtimeMethodName.FindStringSubmatch(name); m != nil {
		tn, ok := p.pkg.Scope().Lookup(m[1]).(*types.TypeName)
		if !ok {
			return nil, fmt.Errorf("cannot resolve %s: no type %s", f.Name, m[1])
		}
		obj, _, _ := types.LookupFieldOrMethod(types.NewPointer(tn.Type()), true, p.pkg, m[2])
		fn, ok := obj.(*types.Func)
		if !ok {
			return nil, fmt.Errorf("cannot resolve %s: no method %s on %s", f.Name, m[2], m[1])
		}
		return []callNode{fn.Origin()}, nil
	}
	if !strings.Contains(name, ".") {
		if fn, ok := p.pkg.Scope().Lookup(name).(*types.Func); ok {
			return []callNode{fn}, nil
		}
		return nil, fmt.Errorf("cannot resolve %s: no function %s", f.Name, name)
	}
	var lits []callNode
	base := filepath.Base(f.File)
	for _, file := range p.files {
		tf := p.fset.File(file.Pos())
		if filepath.Base(tf.Name()) != base {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok && tf.Line(lit.Pos()) == f.Line {
				lits = append(lits, lit)
			}
			return true
		})
	}
	if len(lits) == 0 {
		return nil, fmt.Errorf("cannot resolve closure %s: no function literal at %s:%d", f.Name, base, f.Line)
	}
	return lits, nil
}

// callNode is a function the call graph walks: a *types.Func with a body in the package, the *ast.FuncLit of a
// closure root, or a package-level *types.Var initialized with a function literal.
type callNode any

// callGraph is a lazily built call graph over one package. A node's callees are the package functions and methods
// its body, closures included, names (a call, a method call on a concrete type, or a reference passed as a value) and
// the interface methods it calls. An interface method's callees are, by class hierarchy analysis, that method of
// every package type that implements the interface: no other package can call the unexported helpers.
type callGraph struct {
	p       *typedPackage
	callees map[callNode][]callNode
	// named are the package's named non-interface types, for class hierarchy analysis.
	named []*types.TypeName
}

func newCallGraph(p *typedPackage) *callGraph {
	g := &callGraph{p: p, callees: map[callNode][]callNode{}}
	for _, name := range p.pkg.Scope().Names() {
		if tn, ok := p.pkg.Scope().Lookup(name).(*types.TypeName); ok && !tn.IsAlias() && !types.IsInterface(tn.Type()) {
			g.named = append(g.named, tn)
		}
	}
	return g
}

// interfaceMethod returns the interface fn is a method of, or nil when fn has a concrete receiver or none.
func interfaceMethod(fn *types.Func) *types.Interface {
	sig, _ := fn.Type().(*types.Signature)
	if sig == nil || sig.Recv() == nil {
		return nil
	}
	iface, _ := sig.Recv().Type().Underlying().(*types.Interface)
	return iface
}

// implementations are the methods named like the interface method fn of the package types that implement its
// interface, through a pointer receiver or not.
func (g *callGraph) implementations(fn *types.Func, iface *types.Interface) []callNode {
	var out []callNode
	for _, tn := range g.named {
		for _, t := range []types.Type{tn.Type(), types.NewPointer(tn.Type())} {
			if !types.Implements(t, iface) {
				continue
			}
			obj, _, _ := types.LookupFieldOrMethod(t, true, g.p.pkg, fn.Name())
			if m, ok := obj.(*types.Func); ok && m.Pkg() == g.p.pkg {
				out = append(out, m.Origin())
			}
			break
		}
	}
	return out
}

func (g *callGraph) body(n callNode) ast.Node {
	switch n := n.(type) {
	case *types.Func:
		if d := g.p.decls[n]; d != nil && d.Body != nil {
			return d.Body
		}
	case *ast.FuncLit:
		return n.Body
	case *types.Var:
		if lit := g.p.funcVars[n]; lit != nil {
			return lit.Body
		}
	}
	return nil
}

func (g *callGraph) edges(n callNode) []callNode {
	if out, ok := g.callees[n]; ok {
		return out
	}
	if fn, ok := n.(*types.Func); ok {
		if iface := interfaceMethod(fn); iface != nil {
			out := g.implementations(fn, iface)
			g.callees[n] = out
			return out
		}
	}
	var out []callNode
	seen := map[callNode]bool{}
	if body := g.body(n); body != nil {
		ast.Inspect(body, func(node ast.Node) bool {
			id, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			var callee callNode
			switch obj := g.p.info.Uses[id].(type) {
			case *types.Func:
				if obj.Pkg() == g.p.pkg || interfaceMethod(obj) != nil {
					callee = obj.Origin()
				}
			case *types.Var:
				if g.p.funcVars[obj] != nil {
					callee = obj
				}
			}
			if callee != nil && !seen[callee] {
				seen[callee] = true
				out = append(out, callee)
			}
			return true
		})
	}
	g.callees[n] = out
	return out
}

// firstHelper walks breadth-first from starts and returns the first helper reached, by name, and the shortest call
// path to it; "" when none is reachable. Helpers are not walked into: the gate asks whether a rule reads build files,
// not how. Nor are BuildInfoCovered functions.
func (g *callGraph) firstHelper(starts []callNode, helpers map[*types.Func]string, covered map[*types.Func]bool) (string, []string) {
	parent := map[callNode]callNode{}
	queue := append([]callNode(nil), starts...)
	for _, s := range starts {
		parent[s] = nil
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if fn, ok := n.(*types.Func); ok {
			if name, ok := helpers[fn]; ok {
				var path []string
				for c := n; c != nil; c = parent[c] {
					path = append([]string{g.name(c)}, path...)
				}
				return name, path
			}
			if covered[fn] {
				continue
			}
		}
		for _, c := range g.edges(n) {
			if _, ok := parent[c]; !ok {
				parent[c] = n
				queue = append(queue, c)
			}
		}
	}
	return "", nil
}

func (g *callGraph) name(n callNode) string {
	switch n := n.(type) {
	case *types.Func:
		if sig, ok := n.Type().(*types.Signature); ok && sig.Recv() != nil {
			recv := types.TypeString(sig.Recv().Type(), func(*types.Package) string { return "" })
			return "(" + recv + ")." + n.Name()
		}
		return n.Name()
	case *types.Var:
		return n.Name()
	case *ast.FuncLit:
		pos := g.p.fset.Position(n.Pos())
		return fmt.Sprintf("func@%s:%d", filepath.Base(pos.Filename), pos.Line)
	}
	return "?"
}

func (g *callGraph) position(n callNode) token.Position {
	switch n := n.(type) {
	case *types.Func:
		if d := g.p.decls[n]; d != nil {
			return g.p.fset.Position(d.Pos())
		}
		return g.p.fset.Position(n.Pos())
	case *types.Var:
		return g.p.fset.Position(n.Pos())
	case *ast.FuncLit:
		return g.p.fset.Position(n.Pos())
	}
	return token.Position{}
}
