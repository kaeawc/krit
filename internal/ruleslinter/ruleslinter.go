// Package ruleslinter statically verifies that each v2 rule which calls
// ctx.Resolver or (*oracle.CompositeResolver).Oracle() declares a
// matching capability in its v2.Rule registration. The accepted
// declarations are NeedsResolver, NeedsOracle, or the unified
// NeedsTypeInfo (which subsumes both). The runtime dispatcher only
// wires the resolver / oracle when those bits are set, so a missing
// declaration silently drops findings. This gate catches the mistake
// at build time.
//
// The same gate also enforces the NeedsConcurrent capability shipped
// with PR #326: a rule body that manages its own worker-local
// finding-collector concurrency (calls scanner.MergeCollectors, spawns
// goroutines with `go`, or uses sync.WaitGroup) must declare
// NeedsConcurrent in Meta(); conversely, a rule declaring
// NeedsConcurrent whose body contains none of those signals is flagged
// as declared-but-unused. The signal set is intentionally narrow — see
// scanBodyUsage below — and errs toward false negatives over false
// positives, matching the direction given in the originating issue.
//
// Scope is intentionally conservative:
//   - Analyzes one Go package (typically internal/rules) at a time.
//   - Resolves the Check expression to a single function body: a FuncLit,
//     a package-level FuncDecl, or a method on the rule type bound in
//     the surrounding { r := &FooRule{...}; v2.Register(...) } block.
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
	RuleID   string // ID field from the v2.Rule literal, if resolvable.
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
	CtxParam string // name of the parameter typed *v2.Context, if any
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
			if !isV2RegisterCall(call) {
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

// ctxParamName returns the name of the parameter typed *v2.Context (or
// plain *Context when inside package v2). Empty string means the function
// does not take a v2 context.
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
		// v2.Context
		if id, ok := x.X.(*ast.Ident); ok && id.Name == "v2" && x.Sel.Name == "Context" {
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

func isV2RegisterCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == "v2" && sel.Sel.Name == "Register"
}

// registration holds the parts of a v2.Register(&v2.Rule{...}) call that
// the linter cares about.
type registration struct {
	Lit          *ast.CompositeLit
	ID           string
	NeedsNames   map[string]bool // set of capability constant names found in Needs
	CheckExpr    ast.Expr
	HasOracleCfg bool
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
		}
	}
	if reg.CheckExpr == nil {
		return nil
	}
	body, ctxName, ok := resolveCheckBody(funcs, file, call, reg.CheckExpr)
	if !ok {
		return nil
	}
	usage := scanBodyUsage(funcs, body, ctxName, map[funcKey]bool{})

	// NeedsTypeInfo is a composite of NeedsResolver|NeedsOracle and
	// satisfies either requirement on its own.
	satisfiesResolver := reg.NeedsNames["NeedsResolver"] || reg.NeedsNames["NeedsTypeInfo"]
	satisfiesOracle := reg.NeedsNames["NeedsOracle"] || reg.NeedsNames["NeedsTypeInfo"]
	declaresConcurrent := reg.NeedsNames["NeedsConcurrent"]

	var out []Violation
	pos := fset.Position(call.Pos())
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
	return out
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
// the name of its *v2.Context parameter. The third return reports
// whether resolution succeeded.
func resolveCheckBody(funcs map[funcKey]funcInfo, file *ast.File, call *ast.CallExpr, expr ast.Expr) (*ast.BlockStmt, string, bool) {
	switch e := expr.(type) {
	case *ast.FuncLit:
		return e.Body, ctxParamName(e.Type), true
	case *ast.Ident:
		if info, ok := funcs[funcKey{Name: e.Name}]; ok {
			return info.Body, info.CtxParam, true
		}
	case *ast.SelectorExpr:
		recv, ok := receiverTypeForSelector(file, call, e)
		if !ok {
			return nil, "", false
		}
		if info, ok := funcs[funcKey{Receiver: recv, Name: e.Sel.Name}]; ok {
			return info.Body, info.CtxParam, true
		}
	}
	return nil, "", false
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
	resolver   bool
	oracle     bool
	concurrent bool
}

func (u bodyUsage) merge(other bodyUsage) bodyUsage {
	return bodyUsage{
		resolver:   u.resolver || other.resolver,
		oracle:     u.oracle || other.oracle,
		concurrent: u.concurrent || other.concurrent,
	}
}

// scanBodyUsage reports whether body (or any same-package helper it
// transitively calls) uses ctx.Resolver, calls <x>.Oracle() with no
// args, or uses concurrent-state primitives (go statement,
// sync.WaitGroup, or *.MergeCollectors). Visited tracks helpers already
// inspected to avoid infinite recursion.
//
// The concurrent-state signal set is intentionally narrow: it matches
// the exact primitives the cross-file concurrent dispatcher shipped in
// PR #326 uses to manage per-worker collectors. False negatives are
// preferred over false positives, per the originating issue's
// guidance.
func scanBodyUsage(funcs map[funcKey]funcInfo, body *ast.BlockStmt, ctxName string, visited map[funcKey]bool) bodyUsage {
	var usage bodyUsage
	if body == nil {
		return usage
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.GoStmt:
			usage.concurrent = true
		case *ast.SelectorExpr:
			if ctxName != "" && e.Sel.Name == "Resolver" {
				if id, ok := e.X.(*ast.Ident); ok && id.Name == ctxName {
					usage.resolver = true
				}
			}
			// sync.WaitGroup type reference, e.g. `var wg sync.WaitGroup`
			// or `&sync.WaitGroup{}`.
			if e.Sel.Name == "WaitGroup" {
				if id, ok := e.X.(*ast.Ident); ok && id.Name == "sync" {
					usage.concurrent = true
				}
			}
		case *ast.CallExpr:
			if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Oracle" && len(e.Args) == 0 {
				usage.oracle = true
			}
			if isMergeCollectorsCall(e) {
				usage.concurrent = true
			}
			// Follow same-package helper calls.
			switch fn := e.Fun.(type) {
			case *ast.Ident:
				key := funcKey{Name: fn.Name}
				if info, ok := funcs[key]; ok && !visited[key] {
					visited[key] = true
					usage = usage.merge(scanBodyUsage(funcs, info.Body, info.CtxParam, visited))
				}
			case *ast.SelectorExpr:
				// Method call on a local variable or struct: try (recv, name).
				if recv := guessReceiverType(fn); recv != "" {
					key := funcKey{Receiver: recv, Name: fn.Sel.Name}
					if info, ok := funcs[key]; ok && !visited[key] {
						visited[key] = true
						usage = usage.merge(scanBodyUsage(funcs, info.Body, info.CtxParam, visited))
					}
				}
			}
		}
		return true
	})
	return usage
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
