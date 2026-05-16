// Package ruleslinter statically verifies that each rule which calls
// ctx.Resolver or (*oracle.CompositeResolver).Oracle() declares a
// matching capability in its api.Rule registration. The accepted
// declarations are NeedsResolver, NeedsOracle, or the unified
// NeedsTypeInfo (which subsumes both). The runtime dispatcher only
// wires the resolver / oracle when those bits are set, so a missing
// declaration silently drops findings. This gate catches the mistake
// at build time.
//
// The same gate also enforces the NeedsConcurrent capability: a rule body
// that manages its own worker-local
// finding-collector concurrency (calls scanner.MergeCollectors, spawns
// goroutines with `go`, or uses sync.WaitGroup) must declare
// NeedsConcurrent in Meta(); conversely, a rule declaring
// NeedsConcurrent whose body contains none of those signals is flagged
// as declared-but-unused. The signal set is intentionally narrow — see
// scanBodyUsage below — and errs toward false negatives over false
// positives.
//
// A third gate enforces Fix-declaration fidelity: a rule that declares
// Fix: api.FixSemantic / FixIdiomatic / FixCosmetic must populate a
// Fix on at least one finding it emits (either directly in the Check
// body or in a same-package helper / rule-receiver method that the
// body transitively calls). The check exists because a Fix declaration
// flows into SARIF "fixable" metadata and into `krit fix` UX — a rule
// that says it can be fixed but never produces a fix corrupts both.
//
// Scope is intentionally conservative:
//   - Analyzes one Go package (typically internal/rules) at a time.
//   - Resolves the Check expression to a single function body: a FuncLit,
//     a package-level FuncDecl, or a method on the rule type bound in
//     the surrounding { r := &FooRule{...}; api.Register(...) } block.
//   - Scans that body (and bodies of any same-package helpers it calls)
//     for ctx.Resolver selector usage, zero-arg .Oracle() method calls,
//     and the concurrent-state signals described above. No generics
//     tracking, no cross-package analysis.
package ruleslinter

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Violation is a single linter failure.
type Violation struct {
	RuleID   string // ID field from the api.Rule literal, if resolvable.
	Position token.Position
	Message  string
}

func (v Violation) String() string {
	label := v.RuleID
	if label == "" {
		label = "<unknown>"
	}
	return fmt.Sprintf("%s: rule %s: %s", v.Position, label, v.Message)
}

// AdHocCacheException identifies a sync.Map declaration that pre-dates
// the no-ad-hoc-cache gate and is allowed to exist temporarily. Each
// entry is keyed by file basename + var/field name. Removing an entry
// here is part of the migration of that cache to a shared facts layer
// (typically internal/filefacts/).
type AdHocCacheException struct {
	File string // basename within the rules dir, e.g. "complexity.go"
	Name string // identifier of the sync.Map var or field
}

// grandfatheredAdHocCaches lists package-level sync.Map declarations and
// rule-struct sync.Map fields that existed before the gate landed. They
// represent rule-local memoization that should migrate to a shared
// per-run facts layer. The list MUST shrink over time; new entries are
// not accepted.
var grandfatheredAdHocCaches = map[AdHocCacheException]bool{}

// adHocCacheInfraFiles holds files that contain sync.Map declarations
// which are NOT rule-local memoization but legitimate dispatcher
// infrastructure.
var adHocCacheInfraFiles = map[string]bool{
	"dispatcher.go": true,
}

// AnalyzeAdHocCaches scans every .go file in dir for sync.Map
// declarations (package-level vars or struct fields) and returns one
// violation per unknown declaration. Known cases are listed in
// grandfatheredAdHocCaches; legitimate infrastructure files are listed
// in adHocCacheInfraFiles. Test files are skipped.
func AnalyzeAdHocCaches(dir string) ([]Violation, error) {
	return walkRulesPackageDir(dir, func(name string) bool {
		return adHocCacheInfraFiles[name]
	}, scanAdHocCaches)
}

// walkRulesPackageDir parses every non-test .go file in dir (skipping
// directories, test files, and any filename for which extraSkip returns
// true) and concatenates the violations returned by scan. Results are
// sorted by (filename, offset). extraSkip may be nil.
func walkRulesPackageDir(
	dir string,
	extraSkip func(basename string) bool,
	scan func(fset *token.FileSet, file *ast.File, basename string) []Violation,
) ([]Violation, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ruleslinter: read dir: %w", err)
	}
	var violations []Violation
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		if extraSkip != nil && extraSkip(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("ruleslinter: parse %s: %w", path, err)
		}
		violations = append(violations, scan(fset, f, e.Name())...)
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Position.Filename != violations[j].Position.Filename {
			return violations[i].Position.Filename < violations[j].Position.Filename
		}
		return violations[i].Position.Offset < violations[j].Position.Offset
	})
	return violations, nil
}

func scanAdHocCaches(fset *token.FileSet, file *ast.File, basename string) []Violation {
	var out []Violation
	report := func(name string, pos token.Pos, message string) {
		if grandfatheredAdHocCaches[AdHocCacheException{File: basename, Name: name}] {
			return
		}
		out = append(out, Violation{
			RuleID:   name,
			Position: fset.Position(pos),
			Message:  message,
		})
	}

	// 1. sync.Map declarations — package-level vars and struct fields.
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || !isSyncMapType(vs.Type) {
				continue
			}
			for _, name := range vs.Names {
				report(name.Name, name.Pos(),
					fmt.Sprintf("ad-hoc package-level var %q is a sync.Map cache; route memoization through internal/filefacts/ instead", name.Name))
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		st, ok := n.(*ast.StructType)
		if !ok || st.Fields == nil {
			return true
		}
		for _, field := range st.Fields.List {
			if !isSyncMapType(field.Type) {
				continue
			}
			for _, name := range field.Names {
				report(name.Name, name.Pos(),
					fmt.Sprintf("ad-hoc struct field %q is a sync.Map cache; route memoization through internal/filefacts/ instead", name.Name))
			}
		}
		return true
	})

	// 2. Package-level sync.Mutex / sync.RWMutex declared alongside a map
	//    var — the pre-filefacts handwritten cache pattern.
	scanMutexMapPairs(file, report)
	return out
}

// scanMutexMapPairs flags package-level pairs of (sync.Mutex|sync.RWMutex)
// + map[...] vars whose names share a stem like "fooMu"/"foo" or
// "fooCacheMu"/"fooCache". These are the handwritten cache pattern that
// predates filefacts; new instances should use filefacts.StringFact /
// FileFact / NodeFact instead.
func scanMutexMapPairs(file *ast.File, report func(name string, pos token.Pos, message string)) {
	mutexes := map[string]token.Pos{}
	maps := map[string]token.Pos{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if vs.Type != nil {
				if isMutexType(vs.Type) {
					for _, name := range vs.Names {
						mutexes[name.Name] = name.Pos()
					}
				} else if _, ok := vs.Type.(*ast.MapType); ok {
					for _, name := range vs.Names {
						maps[name.Name] = name.Pos()
					}
				}
			} else {
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						break
					}
					if cl, ok := vs.Values[i].(*ast.CompositeLit); ok {
						if _, ok := cl.Type.(*ast.MapType); ok {
							maps[name.Name] = name.Pos()
						}
					}
				}
			}
		}
	}
	for muName, muPos := range mutexes {
		stem := strings.TrimSuffix(muName, "Mu")
		stem = strings.TrimSuffix(stem, "Mutex")
		stem = strings.TrimSuffix(stem, "Lock")
		if stem == "" || stem == muName {
			continue
		}
		if _, paired := maps[stem]; paired {
			report(stem, muPos,
				fmt.Sprintf("package-level mutex %q + map %q is a handwritten cache pattern; route memoization through internal/filefacts/ instead", muName, stem))
		}
	}
}

func isMutexType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "sync" {
		return false
	}
	return sel.Sel.Name == "Mutex" || sel.Sel.Name == "RWMutex"
}

func isSyncMapType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == "sync" && sel.Sel.Name == "Map"
}

// Analyze parses every .go file in dir as a single package and returns
// violations. The directory need not be a Go module; only syntactic
// parsing is performed.
func Analyze(dir string) ([]Violation, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ruleslinter: read dir: %w", err)
	}
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		// Skip test files: they may register synthetic rules to exercise
		// the linter itself or the dispatcher.
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("ruleslinter: parse %s: %w", path, err)
		}
		files = append(files, f)
	}
	return analyzeFiles(fset, files), nil
}

// funcKey identifies a function body in the package-level index.
// For plain functions the receiver is empty; for methods it is the
// receiver type name stripped of its leading '*'.
type funcKey struct {
	Receiver string
	Name     string
}

type funcInfo struct {
	Body     *ast.BlockStmt
	CtxParam string // name of the parameter typed *api.Context, if any
	RecvName string // name of the receiver parameter, if any (e.g. "r")
	RecvType string // type of the receiver, stripped of leading '*' (e.g. "FooRule")
}

func analyzeFiles(fset *token.FileSet, files []*ast.File) []Violation {
	funcs := make(map[funcKey]funcInfo)
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			recv := receiverTypeName(fn)
			funcs[funcKey{Receiver: recv, Name: fn.Name.Name}] = funcInfo{
				Body:     fn.Body,
				CtxParam: ctxParamName(fn.Type),
				RecvName: receiverParamName(fn),
				RecvType: recv,
			}
		}
	}

	var violations []Violation
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !isAPIRegisterCall(call) {
				return true
			}
			v := analyzeRegisterCall(fset, funcs, f, call)
			violations = append(violations, v...)
			return true
		})
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Position.Filename != violations[j].Position.Filename {
			return violations[i].Position.Filename < violations[j].Position.Filename
		}
		return violations[i].Position.Offset < violations[j].Position.Offset
	})
	return violations
}

// receiverParamName returns the name of the method receiver parameter,
// or "" if the function is not a method or the receiver is unnamed.
func receiverParamName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	names := fn.Recv.List[0].Names
	if len(names) == 0 {
		return ""
	}
	return names[0].Name
}

func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// ctxParamName returns the name of the parameter typed *api.Context (or
// plain *Context when inside package api). Empty string means the
// function does not take a rule context.
func ctxParamName(ft *ast.FuncType) string {
	if ft == nil || ft.Params == nil {
		return ""
	}
	for _, field := range ft.Params.List {
		if !isContextType(field.Type) {
			continue
		}
		if len(field.Names) == 0 {
			return ""
		}
		return field.Names[0].Name
	}
	return ""
}

func isContextType(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	switch x := star.X.(type) {
	case *ast.SelectorExpr:
		// api.Context
		if id, ok := x.X.(*ast.Ident); ok && id.Name == "api" && x.Sel.Name == "Context" {
			return true
		}
	case *ast.Ident:
		// same-package Context reference (unused today, kept for safety)
		if x.Name == "Context" {
			return true
		}
	}
	return false
}

func isAPIRegisterCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == "api" && sel.Sel.Name == "Register"
}

// registration holds the parts of a api.Register(&api.Rule{...}) call that
// the linter cares about.
type registration struct {
	Lit          *ast.CompositeLit
	ID           string
	NeedsNames   map[string]bool // set of capability constant names found in Needs
	CheckExpr    ast.Expr
	HasOracleCfg bool
	FixLevel     string // name of api.Fix* constant, e.g. "FixNone" / "FixSemantic"
}

func parseRegistration(lit *ast.CompositeLit) registration {
	reg := registration{Lit: lit, NeedsNames: map[string]bool{}}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "ID":
			reg.ID = identOrLiteralString(kv.Value)
		case "Needs":
			collectCapabilityNames(kv.Value, reg.NeedsNames)
		case "Check":
			reg.CheckExpr = kv.Value
		case "Oracle":
			reg.HasOracleCfg = true
		case "Fix":
			reg.FixLevel = fixLevelName(kv.Value)
		}
	}
	return reg
}

// fixLevelName returns the trailing identifier of an api.Fix* selector
// expression (e.g. "FixSemantic" for `api.FixSemantic`). Returns ""
// when the expression is not a recognisable Fix constant.
func fixLevelName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		return v.Sel.Name
	case *ast.Ident:
		return v.Name
	}
	return ""
}

func capabilityViolations(reg registration, usage bodyUsage, pos token.Position) []Violation {
	satisfiesResolver := reg.NeedsNames["NeedsResolver"] || reg.NeedsNames["NeedsTypeInfo"]
	satisfiesOracle := reg.NeedsNames["NeedsOracle"] || reg.NeedsNames["NeedsTypeInfo"]
	declaresConcurrent := reg.NeedsNames["NeedsConcurrent"]

	var out []Violation
	if usage.resolver && !satisfiesResolver {
		out = append(out, Violation{
			RuleID:   reg.ID,
			Position: pos,
			Message:  "calls ctx.Resolver but does not declare NeedsResolver or NeedsTypeInfo in Meta()",
		})
	}
	if usage.oracle && !satisfiesOracle {
		out = append(out, Violation{
			RuleID:   reg.ID,
			Position: pos,
			Message:  "calls (*oracle.CompositeResolver).Oracle() but does not declare NeedsOracle or NeedsTypeInfo in Meta()",
		})
	}
	if usage.concurrent && !declaresConcurrent {
		out = append(out, Violation{
			RuleID:   reg.ID,
			Position: pos,
			Message:  "uses concurrent finding collector but does not declare NeedsConcurrent in Meta()",
		})
	}
	if declaresConcurrent && !usage.concurrent {
		out = append(out, Violation{
			RuleID:   reg.ID,
			Position: pos,
			Message:  "declares NeedsConcurrent in Meta() but does not use concurrent state (goroutines, sync.WaitGroup, or scanner.MergeCollectors)",
		})
	}
	if reg.FixLevel != "" && reg.FixLevel != "FixNone" && !usage.fixAssigned {
		out = append(out, Violation{
			RuleID:   reg.ID,
			Position: pos,
			Message:  "declares Fix: api." + reg.FixLevel + " but Check body never assigns a Fix to the finding; set Fix to api.FixNone or implement the fix",
		})
	}
	return out
}

func analyzeRegisterCall(fset *token.FileSet, funcs map[funcKey]funcInfo, file *ast.File, call *ast.CallExpr) []Violation {
	if len(call.Args) != 1 {
		return nil
	}
	unary, ok := call.Args[0].(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return nil
	}
	lit, ok := unary.X.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	reg := parseRegistration(lit)
	if reg.CheckExpr == nil {
		return nil
	}
	info, ok := resolveCheckBody(funcs, file, call, reg.CheckExpr)
	if !ok {
		return nil
	}
	sc := &scanCtx{funcs: funcs, visited: map[funcKey]bool{}}
	usage := scanBodyUsage(sc, info)
	return capabilityViolations(reg, usage, fset.Position(call.Pos()))
}

func identOrLiteralString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s := v.Value
			if len(s) >= 2 {
				return s[1 : len(s)-1]
			}
		}
	case *ast.SelectorExpr:
		// Pattern: r.RuleName → name field on rule struct. Try to resolve
		// the companion `r := &FooRule{BaseRule: BaseRule{RuleName: "..."}}`.
		if id, ok := v.X.(*ast.Ident); ok && v.Sel.Name == "RuleName" {
			if id.Obj != nil {
				if as, ok := id.Obj.Decl.(*ast.AssignStmt); ok {
					if s := extractRuleName(as); s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

func extractRuleName(as *ast.AssignStmt) string {
	if len(as.Rhs) != 1 {
		return ""
	}
	unary, ok := as.Rhs[0].(*ast.UnaryExpr)
	if !ok {
		return ""
	}
	lit, ok := unary.X.(*ast.CompositeLit)
	if !ok {
		return ""
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		// BaseRule: BaseRule{RuleName: "X"}
		inner, ok := kv.Value.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, ie := range inner.Elts {
			ikv, ok := ie.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if id, ok := ikv.Key.(*ast.Ident); ok && id.Name == "RuleName" {
				if bl, ok := ikv.Value.(*ast.BasicLit); ok && bl.Kind == token.STRING && len(bl.Value) >= 2 {
					return bl.Value[1 : len(bl.Value)-1]
				}
			}
		}
	}
	return ""
}

// collectCapabilityNames walks a bit-or expression of capability
// constants and records the simple identifier names (e.g. NeedsResolver).
func collectCapabilityNames(expr ast.Expr, out map[string]bool) {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		if e.Op == token.OR {
			collectCapabilityNames(e.X, out)
			collectCapabilityNames(e.Y, out)
		}
	case *ast.ParenExpr:
		collectCapabilityNames(e.X, out)
	case *ast.SelectorExpr:
		out[e.Sel.Name] = true
	case *ast.Ident:
		out[e.Name] = true
	}
}

// resolveCheckBody resolves the Check expression to a function body and
// the metadata of its *api.Context parameter and method receiver. The
// second return reports whether resolution succeeded.
func resolveCheckBody(funcs map[funcKey]funcInfo, file *ast.File, call *ast.CallExpr, expr ast.Expr) (funcInfo, bool) {
	switch e := expr.(type) {
	case *ast.FuncLit:
		return funcInfo{Body: e.Body, CtxParam: ctxParamName(e.Type)}, true
	case *ast.Ident:
		if info, ok := funcs[funcKey{Name: e.Name}]; ok {
			return info, true
		}
	case *ast.SelectorExpr:
		recv, ok := receiverTypeForSelector(file, call, e)
		if !ok {
			return funcInfo{}, false
		}
		if info, ok := funcs[funcKey{Receiver: recv, Name: e.Sel.Name}]; ok {
			return info, true
		}
	}
	return funcInfo{}, false
}

// receiverTypeForSelector figures out the concrete receiver type for an
// expression like `r.checkNode` by walking up to the enclosing block and
// finding `r := &FooRule{...}` (or any composite literal of a named
// type).
func receiverTypeForSelector(file *ast.File, call *ast.CallExpr, sel *ast.SelectorExpr) (string, bool) {
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	// Prefer the ident's resolved declaration when present.
	if id.Obj != nil {
		if as, ok := id.Obj.Decl.(*ast.AssignStmt); ok {
			if t := typeNameFromAssign(as, id.Name); t != "" {
				return t, true
			}
		}
	}
	// Fallback: scan the enclosing block (same file) for the assignment.
	block := enclosingBlock(file, call.Pos())
	if block == nil {
		return "", false
	}
	for _, stmt := range block.List {
		as, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}
		if t := typeNameFromAssign(as, id.Name); t != "" {
			return t, true
		}
	}
	return "", false
}

func typeNameFromAssign(as *ast.AssignStmt, varName string) string {
	if len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return ""
	}
	lhs, ok := as.Lhs[0].(*ast.Ident)
	if !ok || lhs.Name != varName {
		return ""
	}
	var lit *ast.CompositeLit
	switch r := as.Rhs[0].(type) {
	case *ast.UnaryExpr:
		if r.Op != token.AND {
			return ""
		}
		if cl, ok := r.X.(*ast.CompositeLit); ok {
			lit = cl
		}
	case *ast.CompositeLit:
		lit = r
	}
	if lit == nil {
		return ""
	}
	switch t := lit.Type.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

func enclosingBlock(file *ast.File, pos token.Pos) *ast.BlockStmt {
	var result *ast.BlockStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if n.Pos() <= pos && pos < n.End() {
			if b, ok := n.(*ast.BlockStmt); ok {
				result = b
			}
			return true
		}
		return false
	})
	return result
}

// bodyUsage bundles the capability-usage signals scanBodyUsage detects
// in a Check body (and its transitively-called same-package helpers).
type bodyUsage struct {
	resolver    bool
	oracle      bool
	concurrent  bool
	fixAssigned bool // any `<x>.Fix = <expr>` assignment seen
}

func (u bodyUsage) merge(other bodyUsage) bodyUsage {
	return bodyUsage{
		resolver:    u.resolver || other.resolver,
		oracle:      u.oracle || other.oracle,
		concurrent:  u.concurrent || other.concurrent,
		fixAssigned: u.fixAssigned || other.fixAssigned,
	}
}

func usageFromSelector(e *ast.SelectorExpr, ctxName string) bodyUsage {
	var u bodyUsage
	if ctxName != "" && e.Sel.Name == "Resolver" {
		if id, ok := e.X.(*ast.Ident); ok && id.Name == ctxName {
			u.resolver = true
		}
	}
	if e.Sel.Name == "WaitGroup" {
		if id, ok := e.X.(*ast.Ident); ok && id.Name == "sync" {
			u.concurrent = true
		}
	}
	return u
}

// scanCtx bundles the per-scan state threaded through scanBodyUsage
// and usageFromCall to avoid parameter sprawl.
type scanCtx struct {
	funcs    map[funcKey]funcInfo
	ctxName  string // name of the *api.Context parameter
	selfName string // name of the method receiver (e.g. "r")
	selfType string // type of the method receiver (e.g. "FooRule")
	visited  map[funcKey]bool
}

func usageFromCall(sc *scanCtx, e *ast.CallExpr) bodyUsage {
	var u bodyUsage
	if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Oracle" && len(e.Args) == 0 {
		u.oracle = true
	}
	if isMergeCollectorsCall(e) {
		u.concurrent = true
	}
	switch fn := e.Fun.(type) {
	case *ast.Ident:
		key := funcKey{Name: fn.Name}
		if info, ok := sc.funcs[key]; ok && !sc.visited[key] {
			sc.visited[key] = true
			u = u.merge(scanBodyUsage(sc, info))
		}
	case *ast.SelectorExpr:
		recv := guessReceiverType(fn)
		if recv == "" && sc.selfName != "" && sc.selfType != "" {
			if id, ok := fn.X.(*ast.Ident); ok && id.Name == sc.selfName {
				recv = sc.selfType
			}
		}
		if recv != "" {
			key := funcKey{Receiver: recv, Name: fn.Sel.Name}
			if info, ok := sc.funcs[key]; ok && !sc.visited[key] {
				sc.visited[key] = true
				u = u.merge(scanBodyUsage(sc, info))
			}
		}
	}
	return u
}

// scanBodyUsage reports whether body (or any same-package helper it
// transitively calls) uses ctx.Resolver, calls <x>.Oracle() with no
// args, or uses concurrent-state primitives (go statement,
// sync.WaitGroup, or *.MergeCollectors). Visited tracks helpers already
// inspected to avoid infinite recursion.
//
// The concurrent-state signal set is intentionally narrow: it matches
// the exact primitives the cross-file concurrent dispatcher uses to manage
// per-worker collectors. False negatives are preferred over false positives.
func scanBodyUsage(sc *scanCtx, info funcInfo) bodyUsage {
	var usage bodyUsage
	if info.Body == nil {
		return usage
	}
	prevCtx, prevName, prevType := sc.ctxName, sc.selfName, sc.selfType
	sc.ctxName = info.CtxParam
	sc.selfName = info.RecvName
	sc.selfType = info.RecvType
	defer func() { sc.ctxName, sc.selfName, sc.selfType = prevCtx, prevName, prevType }()
	ast.Inspect(info.Body, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.GoStmt:
			usage.concurrent = true
		case *ast.SelectorExpr:
			usage = usage.merge(usageFromSelector(e, sc.ctxName))
		case *ast.CallExpr:
			usage = usage.merge(usageFromCall(sc, e))
		case *ast.AssignStmt:
			if assignmentTargetsFix(e) {
				usage.fixAssigned = true
			}
		}
		return true
	})
	return usage
}

// assignmentTargetsFix reports whether any LHS of `stmt` is a selector
// expression ending in `.Fix` (e.g. `f.Fix = ...`, `finding.Fix = ...`).
// Used to detect rules that actually populate a Fix on the finding.
func assignmentTargetsFix(stmt *ast.AssignStmt) bool {
	for _, lhs := range stmt.Lhs {
		sel, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if sel.Sel.Name == "Fix" {
			return true
		}
	}
	return false
}

// isMergeCollectorsCall matches both the qualified call
// scanner.MergeCollectors(...) and an unqualified MergeCollectors(...)
// identifier (e.g. if a future rule package dot-imports scanner or the
// helper is re-exported). Matching by selector name is deliberate: the
// linter has no type information, so we rely on the distinctive name.
func isMergeCollectorsCall(call *ast.CallExpr) bool {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name == "MergeCollectors"
	case *ast.SelectorExpr:
		return fn.Sel.Name == "MergeCollectors"
	}
	return false
}

// AnalyzeOptInReason scans every .go file in dir for api.Rule and
// api.RuleDescriptor composite literals and enforces the OptInReason
// invariant:
//
//   - DefaultActive: false  → OptInReason MUST be present and non-zero
//     (i.e. not api.OptInReasonUnspecified). Every off-by-default rule
//     must classify *why* it ships off so the registry can be audited.
//   - DefaultActive: true   → OptInReason MUST be absent or set to
//     api.OptInReasonUnspecified. The reason field only applies to
//     opt-in rules; an active rule carrying one is a documentation bug.
//   - DefaultActive omitted → treated as false (Go zero value).
//
// Test files are skipped.
func AnalyzeOptInReason(dir string) ([]Violation, error) {
	return walkRulesPackageDir(dir, nil, func(fset *token.FileSet, file *ast.File, _ string) []Violation {
		return scanOptInReason(fset, file)
	})
}

func scanOptInReason(fset *token.FileSet, file *ast.File) []Violation {
	var out []Violation
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		typeName := apiTypeName(lit.Type)
		if typeName != "Rule" && typeName != "RuleDescriptor" {
			return true
		}
		var (
			id              string
			idIsLiteral     bool
			hasDefaultKey   bool
			defaultActive   bool
			hasReasonKey    bool
			reasonName      string // e.g. "OptInReasonOpinionated"
			reasonValuePos  token.Pos
			defaultValuePos token.Pos
		)
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch key.Name {
			case "ID":
				id = identOrLiteralString(kv.Value)
				if _, ok := kv.Value.(*ast.BasicLit); ok {
					idIsLiteral = true
				}
			case "DefaultActive":
				hasDefaultKey = true
				defaultValuePos = kv.Value.Pos()
				if b, ok := kv.Value.(*ast.Ident); ok {
					defaultActive = b.Name == "true"
				}
			case "OptInReason":
				hasReasonKey = true
				reasonValuePos = kv.Value.Pos()
				reasonName = optInReasonName(kv.Value)
			}
		}
		// Skip literals that are not concrete rule registrations:
		//   - factory templates (ID is a variable / parameter, not a literal string)
		//   - zero-value returns like `return api.RuleDescriptor{}` inside helpers
		// In both cases the OptInReason classification cannot be made statically;
		// the factory's caller is responsible for supplying it.
		if !idIsLiteral || id == "" {
			return true
		}
		// Treat omitted DefaultActive as the zero value (false).
		_ = hasDefaultKey
		if defaultActive {
			if hasReasonKey && reasonName != "OptInReasonUnspecified" {
				out = append(out, Violation{
					RuleID:   id,
					Position: fset.Position(reasonValuePos),
					Message:  "DefaultActive: true must not carry an OptInReason (got api." + reasonName + "); remove the OptInReason field or set DefaultActive: false",
				})
			}
			return true
		}
		// DefaultActive is false (explicit or omitted).
		if !hasReasonKey || reasonName == "OptInReasonUnspecified" {
			pos := defaultValuePos
			if pos == token.NoPos {
				pos = lit.Pos()
			}
			out = append(out, Violation{
				RuleID:   id,
				Position: fset.Position(pos),
				Message:  "DefaultActive: false (or omitted) must declare an OptInReason in the same literal; pick one of api.OptInReason{Opinionated,ProjectPolicy,ThresholdTuning,DomainSpecific,AndroidOnly,RequiresUserConfig,DuplicatesCompiler,Expensive}",
			})
		}
		return true
	})
	return out
}

// apiTypeName returns the trailing identifier of an api.<Name> selector
// (e.g. "Rule" for api.Rule, "RuleDescriptor" for api.RuleDescriptor).
// Returns "" when expr is not an api-qualified type reference.
func apiTypeName(expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "api" {
		return ""
	}
	return sel.Sel.Name
}

// optInReasonName returns the trailing identifier of an api.OptInReason*
// selector (e.g. "OptInReasonOpinionated"). Returns the empty string
// when expr is not a recognisable OptInReason constant.
func optInReasonName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		return v.Sel.Name
	case *ast.Ident:
		return v.Name
	}
	return ""
}

// AnalyzeDefensiveContextGuards scans every .go file in dir for the
// defensive `if ctx.File == nil { return }` / `if ctx.Idx == 0 { return }`
// boilerplate that historically appeared at the top of rule callbacks.
//
// The dispatcher in internal/rules/dispatcher.go guarantees that ctx.File
// is non-nil for every rule callback invocation, and that ctx.Idx points
// at the matched flat-tree index (never 0 for ScopePerFileNode rules whose
// NodeTypes filter out the source_file root). The guards are therefore
// theater that costs review attention and obscures real preconditions.
//
// A finding here means: a function with a *api.Context parameter contains
// an `if` whose condition mentions ctx.File == nil or ctx.Idx == 0 and
// whose body is a bare `return` (no value). Helper functions that return
// a value (false / 0 / "") are intentionally not flagged because they
// may be invoked from non-dispatcher paths (tests, other helpers) where
// the dispatcher contract does not apply.
//
// Test files are skipped so this file's tests, which construct example
// rule source containing the pattern as a string fixture, do not trip
// the gate.
func AnalyzeDefensiveContextGuards(dir string) ([]Violation, error) {
	return walkRulesPackageDir(dir, func(name string) bool {
		// dispatcher.go legitimately reads ctx.File / ctx.Idx to wire the
		// context up; skip it (and any sibling dispatcher_*.go) so the
		// gate only inspects rule callbacks, not the dispatcher itself.
		return name == "dispatcher.go" || strings.HasPrefix(name, "dispatcher_")
	}, func(fset *token.FileSet, file *ast.File, _ string) []Violation {
		return scanDefensiveContextGuards(fset, file)
	})
}

// scanDefensiveContextGuards inspects every function in file whose
// signature carries a *api.Context parameter and flags `if` statements
// of the form `if ctx.File == nil { return }` (and variants involving
// ctx.Idx == 0). Functions that return values are skipped — they are
// helpers, not rule callbacks, and may be invoked from non-dispatcher
// paths.
//
// Limitation: init-form guards like `if x := ctx.File; x == nil { return }`
// are not detected (the analysis would have to track the assigned name
// through the body). New code should not introduce that shape; the bare
// `if ctx.File == nil { return }` form is what previously accumulated.
func scanDefensiveContextGuards(fset *token.FileSet, file *ast.File) []Violation {
	var out []Violation
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ctxName := ctxParamName(fn.Type)
		if ctxName == "" {
			continue
		}
		// Only functions that have no return values: rule callbacks return
		// nothing. Helpers that return false/0/"" remain in charge of their
		// own nil-safety.
		if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
			continue
		}
		for _, stmt := range fn.Body.List {
			ifStmt, ok := stmt.(*ast.IfStmt)
			if !ok || ifStmt.Init != nil {
				continue
			}
			if !ifBodyIsBareReturn(ifStmt.Body) {
				continue
			}
			if !condReferencesContextGuard(ifStmt.Cond, ctxName) {
				continue
			}
			advice := "Delete the guard"
			if _, compound := ifStmt.Cond.(*ast.BinaryExpr); compound && condIsLogicalOr(ifStmt.Cond) {
				advice = "Drop the dispatcher-guaranteed half of the `||`, or split the remaining check into a separate `if`"
			}
			out = append(out, Violation{
				RuleID:   fn.Name.Name,
				Position: fset.Position(ifStmt.Pos()),
				Message:  "defensive `if " + ctxName + ".File == nil` / `" + ctxName + ".Idx == 0` guard in rule callback; the dispatcher guarantees both. " + advice + " — see internal/rules/dispatcher.go.",
			})
		}
	}
	return out
}

// condIsLogicalOr reports whether the top-level operator of cond is `||`.
// Used to tailor the violation message when the guard is compounded with
// a real precondition.
func condIsLogicalOr(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	return ok && bin.Op == token.LOR
}

// ifBodyIsBareReturn reports whether body is `{ return }` — a single bare
// `return` statement with no value. Guards with `return false` belong to
// helper functions and stay.
func ifBodyIsBareReturn(body *ast.BlockStmt) bool {
	if body == nil || len(body.List) != 1 {
		return false
	}
	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok {
		return false
	}
	return len(ret.Results) == 0
}

// condReferencesContextGuard reports whether the boolean expression cond
// mentions `<ctx>.File == nil` or `<ctx>.Idx == 0` anywhere in its tree
// (including the LHS of `||` chains).
func condReferencesContextGuard(cond ast.Expr, ctxName string) bool {
	found := false
	ast.Inspect(cond, func(n ast.Node) bool {
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || bin.Op != token.EQL {
			return true
		}
		if isContextFieldNilCompare(bin.X, bin.Y, ctxName, "File") ||
			isContextFieldNilCompare(bin.Y, bin.X, ctxName, "File") {
			found = true
			return false
		}
		if isContextFieldZeroCompare(bin.X, bin.Y, ctxName, "Idx") ||
			isContextFieldZeroCompare(bin.Y, bin.X, ctxName, "Idx") {
			found = true
			return false
		}
		return true
	})
	return found
}

// isContextFieldNilCompare reports whether lhs is `<ctxName>.<field>` and
// rhs is the identifier `nil`.
func isContextFieldNilCompare(lhs, rhs ast.Expr, ctxName, field string) bool {
	sel, ok := lhs.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != field {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != ctxName {
		return false
	}
	rid, ok := rhs.(*ast.Ident)
	if !ok {
		return false
	}
	return rid.Name == "nil"
}

// isContextFieldZeroCompare reports whether lhs is `<ctxName>.<field>` and
// rhs is the literal `0`.
func isContextFieldZeroCompare(lhs, rhs ast.Expr, ctxName, field string) bool {
	sel, ok := lhs.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != field {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != ctxName {
		return false
	}
	lit, ok := rhs.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	return lit.Value == "0"
}

// guessReceiverType looks at sel.X and returns the type name if it
// resolves through AssignStmt object metadata. Best-effort: returns ""
// when the type cannot be determined without full type checking.
func guessReceiverType(sel *ast.SelectorExpr) string {
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Obj == nil {
		return ""
	}
	if as, ok := id.Obj.Decl.(*ast.AssignStmt); ok {
		return typeNameFromAssign(as, id.Name)
	}
	return ""
}
