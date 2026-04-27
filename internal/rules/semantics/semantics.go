package semantics

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

type flatOracleLookup interface {
	LookupCallTargetFlat(file *scanner.File, idx uint32) string
	LookupCallTargetSuspendFlat(file *scanner.File, idx uint32) (bool, bool)
	LookupCallTargetAnnotationsFlat(file *scanner.File, idx uint32) []string
	LookupDiagnosticsForFlatRange(file *scanner.File, idx uint32) []oracle.OracleDiagnostic
}

// NodeRef identifies a flat syntax node in a source file.
type NodeRef struct {
	File *scanner.File
	Node uint32
}

// Valid reports whether the reference points at an existing flat node.
func (r NodeRef) Valid() bool {
	return r.File != nil && r.File.FlatTree != nil && int(r.Node) < len(r.File.FlatTree.Nodes)
}

// CallTarget is the rule-facing view of a Kotlin call expression.
type CallTarget struct {
	CalleeName    string
	QualifiedName string
	Receiver      NodeRef
	Arguments     []NodeRef
	Resolved      bool
}

// SymbolRef is the rule-facing view of a reference-like node.
type SymbolRef struct {
	Name          string
	Node          uint32
	Owner         uint32
	QualifiedName string
	Resolved      bool
}

// TypeInfo wraps resolver output in a stable shape for rule helpers.
type TypeInfo struct {
	Name     string
	FQN      string
	Kind     typeinfer.TypeKind
	Nullable bool
	Type     *typeinfer.ResolvedType
}

// ConstValue is a conservative constant value extracted from literal nodes or
// same-file constant declarations.
type ConstValue struct {
	Kind     string
	Int64    int64
	Float64  float64
	String   string
	Bool     bool
	Resolved bool
}

// TypeFact records a local, structurally provable type relationship.
type TypeFact struct {
	Expr     uint32
	TypeName string
	Positive bool
}

// PermissionGuard records an enclosing permission check.
type PermissionGuard struct {
	Node       uint32
	Permission ConstValue
	Positive   bool
}

// ExceptionHandler records an enclosing catch handler.
type ExceptionHandler struct {
	Node      uint32
	ParamName string
	TypeName  string
}

// SemanticEvidence describes the strength of a rule's semantic proof.
type SemanticEvidence int

const (
	EvidenceResolved SemanticEvidence = iota
	EvidenceQualifiedReceiver
	EvidenceSameOwner
	EvidenceSameFileDeclaration
	EvidenceUnresolved
)

// ConfidenceForEvidence applies the shared policy for unresolved semantic
// data. Unresolved evidence is not actionable by default.
func ConfidenceForEvidence(base float64, evidence SemanticEvidence) (float64, bool) {
	if base <= 0 {
		base = 0.75
	}
	switch evidence {
	case EvidenceResolved:
		return clampConfidence(base), true
	case EvidenceQualifiedReceiver:
		return clampConfidence(minFloat(base, 0.90)), true
	case EvidenceSameOwner:
		return clampConfidence(minFloat(base, 0.90)), true
	case EvidenceSameFileDeclaration:
		return clampConfidence(minFloat(base, 0.80)), true
	default:
		return 0, false
	}
}

// ReferenceName returns the terminal identifier for a reference-like node.
func ReferenceName(file *scanner.File, idx uint32) string {
	return referenceName(file, idx)
}

// DeclarationName returns the declared identifier for common declaration nodes.
func DeclarationName(file *scanner.File, idx uint32) string {
	return declarationName(file, idx)
}

// ResolveCallTarget inspects a call expression structurally and, when the
// resolver wraps the Kotlin oracle, attaches the resolved callable target.
func ResolveCallTarget(ctx *v2.Context, call uint32) (CallTarget, bool) {
	if ctx == nil || ctx.File == nil || ctx.File.FlatType(call) != "call_expression" {
		return CallTarget{}, false
	}
	file := ctx.File
	callee := callExpressionName(file, call)
	if callee == "" {
		return CallTarget{}, false
	}

	target := CallTarget{
		CalleeName: callee,
		Receiver:   NodeRef{File: file, Node: callReceiver(file, call)},
		Arguments:  callArguments(file, call),
	}
	if qn := oracleCallTarget(ctx, call); qn != "" {
		if looksResolvedName(qn) {
			target.QualifiedName = qn
			target.CalleeName = simpleCallableName(qn)
			target.Resolved = true
		} else if target.CalleeName == "" {
			target.CalleeName = qn
		}
	}
	return target, true
}

// MatchQualifiedReceiver reports whether the call receiver resolves to or
// structurally names one of allowed.
func MatchQualifiedReceiver(ctx *v2.Context, call uint32, allowed ...string) bool {
	target, ok := ResolveCallTarget(ctx, call)
	if !ok || !target.Receiver.Valid() {
		return false
	}
	file := target.Receiver.File
	receiver := target.Receiver.Node
	if ctx != nil && ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(receiver, file); typeMatchesAny(typ, allowed...) {
			return true
		}
		if name := terminalName(file, receiver); name != "" {
			if fqn := ctx.Resolver.ResolveImport(name, file); qualifiedNameMatchesAny(fqn, allowed...) {
				return true
			}
		}
	}
	return qualifiedNameMatchesAny(qualifiedPath(file, receiver), allowed...)
}

// IsResolvedCall reports whether a call has a resolved callable target that
// matches one of the supplied fully-qualified names.
func IsResolvedCall(ctx *v2.Context, call uint32, fqNames ...string) bool {
	target, ok := ResolveCallTarget(ctx, call)
	if !ok || !target.Resolved {
		return false
	}
	return qualifiedNameMatchesAny(target.QualifiedName, fqNames...)
}

// ResolveReference returns the terminal identifier and the nearest enclosing
// owner for a reference-like node. It intentionally does not treat substrings
// in comments or string literals as references.
func ResolveReference(ctx *v2.Context, ref uint32) (SymbolRef, bool) {
	if ctx == nil || ctx.File == nil {
		return SymbolRef{}, false
	}
	file := ctx.File
	name := referenceName(file, ref)
	if name == "" {
		return SymbolRef{}, false
	}
	out := SymbolRef{Name: name, Node: ref, Owner: enclosingOwner(file, ref)}
	if ctx.Resolver != nil {
		if fqn := ctx.Resolver.ResolveImport(name, file); fqn != "" {
			out.QualifiedName = fqn
			out.Resolved = true
			return out, true
		}
	}
	if ctx.CodeIndex != nil {
		var matched *scanner.Symbol
		for i := range ctx.CodeIndex.Symbols {
			sym := &ctx.CodeIndex.Symbols[i]
			if sym.Name != name || sym.File != file.Path {
				continue
			}
			node, ok := nodeForSymbol(file, *sym)
			if !ok || !SameEnclosingOwner(file, node, ref) {
				continue
			}
			if matched != nil {
				return out, true
			}
			matched = sym
			out.Node = node
			out.Owner = enclosingOwner(file, node)
			out.Resolved = true
		}
	}
	return out, true
}

// SameDeclaration reports whether ref structurally points at decl. Resolved
// data is preferred when available, with same-file declaration matching as the
// conservative fallback.
func SameDeclaration(ctx *v2.Context, decl uint32, ref uint32) bool {
	if ctx == nil || ctx.File == nil || decl == 0 || ref == 0 {
		return false
	}
	if sameNodeRange(ctx.File, decl, ref) {
		return true
	}
	if sym, ok := ResolveReference(ctx, ref); ok && sym.Resolved && sym.Node != ref {
		return sameNodeRange(ctx.File, decl, sym.Node)
	}
	return SameFileDeclarationMatch(ctx, decl, ref)
}

// SameEnclosingOwner reports whether both nodes belong to the same enclosing
// class/object/interface owner. Top-level nodes share owner 0.
func SameEnclosingOwner(file *scanner.File, a uint32, b uint32) bool {
	if file == nil || a == 0 || b == 0 {
		return false
	}
	return enclosingOwner(file, a) == enclosingOwner(file, b)
}

// SameFileDeclarationMatch performs the explicit local fallback used when
// external resolution is unavailable.
func SameFileDeclarationMatch(ctx *v2.Context, decl uint32, ref uint32) bool {
	if ctx == nil || ctx.File == nil || decl == 0 || ref == 0 {
		return false
	}
	file := ctx.File
	declName := declarationName(file, decl)
	refName := referenceName(file, ref)
	if declName == "" || refName == "" || declName != refName {
		return false
	}
	return SameEnclosingOwner(file, decl, ref)
}

// ExpressionType resolves the type of an expression through ctx.Resolver.
func ExpressionType(ctx *v2.Context, expr uint32) (TypeInfo, bool) {
	if ctx == nil || ctx.File == nil || ctx.Resolver == nil || expr == 0 {
		return TypeInfo{}, false
	}
	typ := ctx.Resolver.ResolveFlatNode(expr, ctx.File)
	if typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return TypeInfo{}, false
	}
	return TypeInfo{Name: typ.Name, FQN: typ.FQN, Kind: typ.Kind, Nullable: typ.IsNullable(), Type: typ}, true
}

// IsNullableExpression reports nullability when the resolver can prove it.
func IsNullableExpression(ctx *v2.Context, expr uint32) (bool, bool) {
	if ctx == nil || ctx.File == nil || ctx.Resolver == nil || expr == 0 {
		return false, false
	}
	if nullable := ctx.Resolver.IsNullableFlat(expr, ctx.File); nullable != nil {
		return *nullable, true
	}
	if typ, ok := ExpressionType(ctx, expr); ok {
		return typ.Nullable, true
	}
	return false, false
}

// IsSupportedIsNullOrEmptyReceiver reports whether expr is a string,
// collection, array, or map receiver supported by isNullOrEmpty.
func IsSupportedIsNullOrEmptyReceiver(ctx *v2.Context, expr uint32) bool {
	typ, ok := ExpressionType(ctx, expr)
	if !ok {
		return false
	}
	return typeNameIn(typ.Type,
		"kotlin.String", "String",
		"kotlin.collections.Collection", "Collection",
		"kotlin.collections.List", "List",
		"kotlin.collections.Set", "Set",
		"kotlin.collections.Map", "Map",
		"kotlin.collections.MutableCollection", "MutableCollection",
		"kotlin.collections.MutableList", "MutableList",
		"kotlin.collections.MutableSet", "MutableSet",
		"kotlin.collections.MutableMap", "MutableMap",
		"kotlin.Array", "Array",
	)
}

// IsMapLikeReceiver reports whether expr resolves to a Kotlin or Java map.
func IsMapLikeReceiver(ctx *v2.Context, expr uint32) bool {
	typ, ok := ExpressionType(ctx, expr)
	if !ok {
		return false
	}
	return typeNameIn(typ.Type,
		"kotlin.collections.Map", "Map",
		"kotlin.collections.MutableMap", "MutableMap",
		"java.util.Map", "java.util.MutableMap",
	)
}

// EvalConst evaluates simple literal constants and same-file constant refs.
func EvalConst(ctx *v2.Context, expr uint32) (ConstValue, bool) {
	if ctx == nil || ctx.File == nil || expr == 0 {
		return ConstValue{}, false
	}
	file := ctx.File
	expr = unwrapExpression(file, expr)
	switch file.FlatType(expr) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		if containsStringInterpolation(file, expr) {
			return ConstValue{}, false
		}
		if s, ok := unquoteKotlinString(file.FlatNodeText(expr)); ok {
			return ConstValue{Kind: "string", String: s, Resolved: true}, true
		}
	case "integer_literal":
		if v, ok := parseIntLiteral(file.FlatNodeText(expr)); ok {
			return ConstValue{Kind: "int", Int64: v, Resolved: true}, true
		}
	case "long_literal":
		if v, ok := parseIntLiteral(file.FlatNodeText(expr)); ok {
			return ConstValue{Kind: "int", Int64: v, Resolved: true}, true
		}
	case "real_literal":
		if v, ok := parseFloatLiteral(file.FlatNodeText(expr)); ok {
			return ConstValue{Kind: "float", Float64: v, Resolved: true}, true
		}
	case "boolean_literal":
		switch file.FlatNodeText(expr) {
		case "true":
			return ConstValue{Kind: "bool", Bool: true, Resolved: true}, true
		case "false":
			return ConstValue{Kind: "bool", Bool: false, Resolved: true}, true
		}
	case "prefix_expression":
		return evalPrefixConst(ctx, expr)
	case "simple_identifier", "navigation_expression":
		return EvalSameFileConst(ctx, expr)
	case "value_argument":
		if argExpr := valueArgumentExpression(file, expr); argExpr != 0 {
			return EvalConst(ctx, argExpr)
		}
	}
	return ConstValue{}, false
}

// EvalSameFileConst resolves a same-file val/const reference to a literal
// initializer when the declaration belongs to the same owner.
func EvalSameFileConst(ctx *v2.Context, ref uint32) (ConstValue, bool) {
	if ctx == nil || ctx.File == nil || ref == 0 {
		return ConstValue{}, false
	}
	file := ctx.File
	name := referenceName(file, ref)
	if name == "" {
		return ConstValue{}, false
	}
	var out ConstValue
	var ok bool
	file.FlatWalkNodes(0, "property_declaration", func(decl uint32) {
		if ok || declarationName(file, decl) != name || !SameFileDeclarationMatch(ctx, decl, ref) {
			return
		}
		init := propertyInitializer(file, decl)
		if init == 0 {
			return
		}
		out, ok = EvalConst(ctx, init)
	})
	return out, ok
}

// DominatingTypeFacts returns simple type facts from enclosing if bodies.
func DominatingTypeFacts(ctx *v2.Context, node uint32) []TypeFact {
	if ctx == nil || ctx.File == nil || node == 0 {
		return nil
	}
	file := ctx.File
	var facts []TypeFact
	for body, ok := file.FlatParent(node); ok; body, ok = file.FlatParent(body) {
		if file.FlatType(body) != "control_structure_body" {
			continue
		}
		parent, ok := file.FlatParent(body)
		if !ok || file.FlatType(parent) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := ifParts(file, parent)
		if cond == 0 {
			continue
		}
		positive := body == thenBody
		if body != thenBody && body != elseBody {
			continue
		}
		for _, fact := range conditionTypeFacts(file, cond) {
			if !positive {
				fact.Positive = !fact.Positive
			}
			facts = append(facts, fact)
		}
	}
	return facts
}

// EnclosingPermissionGuards returns obvious enclosing permission guards.
func EnclosingPermissionGuards(ctx *v2.Context, call uint32) []PermissionGuard {
	if ctx == nil || ctx.File == nil || call == 0 {
		return nil
	}
	file := ctx.File
	var guards []PermissionGuard
	for body, ok := file.FlatParent(call); ok; body, ok = file.FlatParent(body) {
		if file.FlatType(body) != "control_structure_body" {
			continue
		}
		parent, ok := file.FlatParent(body)
		if !ok || file.FlatType(parent) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := ifParts(file, parent)
		if cond == 0 || (body != thenBody && body != elseBody) {
			continue
		}
		positiveBody := body == thenBody
		for _, guard := range permissionGuardsInCondition(ctx, cond) {
			guard.Positive = guard.Positive == positiveBody
			guards = append(guards, guard)
		}
	}
	return guards
}

// EnclosingCaughtExceptionHandlers returns enclosing catch handlers.
func EnclosingCaughtExceptionHandlers(ctx *v2.Context, call uint32) []ExceptionHandler {
	if ctx == nil || ctx.File == nil || call == 0 {
		return nil
	}
	file := ctx.File
	var out []ExceptionHandler
	for cur, ok := file.FlatParent(call); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "catch_block", "catch_clause":
			name, typ := catchParameter(file, cur)
			out = append(out, ExceptionHandler{Node: cur, ParamName: name, TypeName: typ})
		case "function_declaration", "class_declaration", "object_declaration":
			return out
		}
	}
	return out
}

func oracleCallTarget(ctx *v2.Context, idx uint32) string {
	return OracleCallTarget(ctx, idx)
}

// OracleCallTarget resolves the compiler-backed call target for a FlatNode.
func OracleCallTarget(ctx *v2.Context, idx uint32) string {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return ""
	}
	var lookup oracle.Lookup
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		lookup = cr.Oracle()
	}
	if lookup == nil {
		return ""
	}
	if flat, ok := lookup.(flatOracleLookup); ok {
		return flat.LookupCallTargetFlat(ctx.File, idx)
	}
	return lookup.LookupCallTarget(ctx.File.Path, ctx.File.FlatRow(idx)+1, ctx.File.FlatCol(idx)+1)
}

// OracleCallTargetSuspend resolves suspend-call evidence for a FlatNode.
func OracleCallTargetSuspend(ctx *v2.Context, idx uint32) (bool, bool) {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return false, false
	}
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		lookup := cr.Oracle()
		if flat, ok := lookup.(flatOracleLookup); ok {
			return flat.LookupCallTargetSuspendFlat(ctx.File, idx)
		}
		return lookup.LookupCallTargetSuspend(ctx.File.Path, ctx.File.FlatRow(idx)+1, ctx.File.FlatCol(idx)+1)
	}
	return false, false
}

// OracleCallTargetAnnotations returns annotations on the resolved call target.
func OracleCallTargetAnnotations(ctx *v2.Context, idx uint32) []string {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return nil
	}
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		lookup := cr.Oracle()
		if flat, ok := lookup.(flatOracleLookup); ok {
			return flat.LookupCallTargetAnnotationsFlat(ctx.File, idx)
		}
		return lookup.LookupCallTargetAnnotations(ctx.File.Path, ctx.File.FlatRow(idx)+1, ctx.File.FlatCol(idx)+1)
	}
	return nil
}

// OracleDiagnosticsForFlatRange returns compiler diagnostics inside a FlatNode.
func OracleDiagnosticsForFlatRange(ctx *v2.Context, idx uint32) []oracle.OracleDiagnostic {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return nil
	}
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		lookup := cr.Oracle()
		if flat, ok := lookup.(flatOracleLookup); ok {
			return flat.LookupDiagnosticsForFlatRange(ctx.File, idx)
		}
		return lookup.LookupDiagnostics(ctx.File.Path)
	}
	return nil
}

func callExpressionParts(file *scanner.File, idx uint32) (uint32, uint32) {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return 0, 0
	}
	var nav, args uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "navigation_expression":
			nav = child
		case "value_arguments":
			args = child
		case "call_suffix":
			if args == 0 {
				args, _ = file.FlatFindChild(child, "value_arguments")
			}
		}
	}
	return nav, args
}

func callExpressionName(file *scanner.File, idx uint32) string {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return ""
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			return file.FlatNodeString(child, nil)
		case "navigation_expression":
			if name := navigationLastIdentifier(file, child); name != "" {
				return name
			}
		case "call_expression":
			if name := callExpressionName(file, child); name != "" {
				return name
			}
		}
	}
	return ""
}

func navigationLastIdentifier(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	last := ""
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "navigation_suffix":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatIsNamed(gc) && file.FlatType(gc) == "simple_identifier" {
					last = file.FlatNodeString(gc, nil)
				}
			}
		case "simple_identifier", "type_identifier":
			last = file.FlatNodeString(child, nil)
		case "navigation_expression":
			if name := navigationLastIdentifier(file, child); name != "" {
				last = name
			}
		}
	}
	return last
}

func callReceiver(file *scanner.File, call uint32) uint32 {
	nav, _ := callExpressionParts(file, call)
	if nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return 0
	}
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) && file.FlatType(child) != "navigation_suffix" {
			return child
		}
	}
	return 0
}

func callArguments(file *scanner.File, call uint32) []NodeRef {
	args := callKeyArguments(file, call)
	var out []NodeRef
	if args != 0 {
		for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatType(arg) != "value_argument" {
				continue
			}
			if expr := valueArgumentExpression(file, arg); expr != 0 {
				out = append(out, NodeRef{File: file, Node: expr})
			}
		}
	}
	if lambda := callTrailingLambda(file, call); lambda != 0 {
		out = append(out, NodeRef{File: file, Node: lambda})
	}
	return out
}

func callKeyArguments(file *scanner.File, idx uint32) uint32 {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return 0
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "call_suffix":
			if va, ok := file.FlatFindChild(child, "value_arguments"); ok {
				return va
			}
		case "call_expression":
			if suffix, ok := file.FlatFindChild(child, "call_suffix"); ok {
				if va, ok := file.FlatFindChild(suffix, "value_arguments"); ok {
					return va
				}
			}
		}
	}
	return 0
}

func callTrailingLambda(file *scanner.File, idx uint32) uint32 {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return 0
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "call_suffix" {
			continue
		}
		if lambda, ok := file.FlatFindChild(child, "annotated_lambda"); ok {
			if lit, ok := file.FlatFindChild(lambda, "lambda_literal"); ok {
				return lit
			}
			return lambda
		}
		if lambda, ok := file.FlatFindChild(child, "lambda_literal"); ok {
			return lambda
		}
	}
	return 0
}

func valueArgumentExpression(file *scanner.File, arg uint32) uint32 {
	if file == nil || arg == 0 {
		return 0
	}
	skipNextEquals := false
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "value_argument_label":
			skipNextEquals = true
			continue
		case "simple_identifier":
			if next, ok := file.FlatNextSibling(child); ok && file.FlatType(next) == "=" {
				skipNextEquals = true
				continue
			}
		}
		if skipNextEquals && file.FlatType(child) == "=" {
			skipNextEquals = false
			continue
		}
		return child
	}
	return 0
}

func terminalName(file *scanner.File, idx uint32) string {
	idx = unwrapExpression(file, idx)
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		return file.FlatNodeString(idx, nil)
	case "user_type", "nullable_type":
		last := ""
		file.FlatWalkAllNodes(idx, func(node uint32) {
			switch file.FlatType(node) {
			case "simple_identifier", "type_identifier":
				last = file.FlatNodeString(node, nil)
			}
		})
		return last
	case "navigation_expression":
		return navigationLastIdentifier(file, idx)
	case "call_expression":
		return callExpressionName(file, idx)
	case "value_argument":
		return terminalName(file, valueArgumentExpression(file, idx))
	default:
		return ""
	}
}

func referenceName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier", "navigation_expression", "call_expression", "value_argument":
		return terminalName(file, idx)
	default:
		return ""
	}
}

func declarationName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	switch file.FlatType(idx) {
	case "function_declaration", "property_declaration", "variable_declaration", "parameter":
		if name := file.FlatChildTextOrEmpty(idx, "simple_identifier"); name != "" {
			return name
		}
		var found string
		file.FlatWalkNodes(idx, "simple_identifier", func(candidate uint32) {
			if found == "" {
				found = file.FlatNodeString(candidate, nil)
			}
		})
		return found
	case "class_declaration", "object_declaration", "interface_declaration":
		if name := file.FlatChildTextOrEmpty(idx, "type_identifier"); name != "" {
			return name
		}
		if name := file.FlatChildTextOrEmpty(idx, "simple_identifier"); name != "" {
			return name
		}
	case "simple_identifier", "type_identifier":
		return file.FlatNodeString(idx, nil)
	}
	return ""
}

func enclosingOwner(file *scanner.File, idx uint32) uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "interface_declaration":
			return cur
		}
	}
	return 0
}

func qualifiedPath(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	idx = unwrapExpression(file, idx)
	var parts []string
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		return file.FlatNodeString(idx, nil)
	case "navigation_expression":
		file.FlatWalkAllNodes(idx, func(node uint32) {
			switch file.FlatType(node) {
			case "simple_identifier", "type_identifier":
				parts = append(parts, file.FlatNodeString(node, nil))
			}
		})
		return strings.Join(parts, ".")
	case "call_expression":
		if receiver := callReceiver(file, idx); receiver != 0 {
			return qualifiedPath(file, receiver)
		}
		return callExpressionName(file, idx)
	default:
		return ""
	}
}

func nodeForSymbol(file *scanner.File, sym scanner.Symbol) (uint32, bool) {
	if file == nil || sym.StartByte < 0 || sym.EndByte <= sym.StartByte {
		return 0, false
	}
	return file.FlatNamedDescendantForByteRange(uint32(sym.StartByte), uint32(sym.EndByte))
}

func propertyInitializer(file *scanner.File, decl uint32) uint32 {
	seenEquals := false
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seenEquals = true
			continue
		}
		if seenEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func unwrapExpression(file *scanner.File, idx uint32) uint32 {
	for idx != 0 && file.FlatType(idx) == "parenthesized_expression" && file.FlatNamedChildCount(idx) > 0 {
		idx = file.FlatNamedChild(idx, 0)
	}
	return idx
}

func evalPrefixConst(ctx *v2.Context, idx uint32) (ConstValue, bool) {
	file := ctx.File
	sign := int64(1)
	var childExpr uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "-":
			sign = -1
		case "+":
			sign = 1
		default:
			if file.FlatIsNamed(child) {
				childExpr = child
			}
		}
	}
	val, ok := EvalConst(ctx, childExpr)
	if !ok {
		return ConstValue{}, false
	}
	switch val.Kind {
	case "int":
		val.Int64 *= sign
		return val, true
	case "float":
		val.Float64 *= float64(sign)
		return val, true
	default:
		return ConstValue{}, false
	}
}

func containsStringInterpolation(file *scanner.File, idx uint32) bool {
	found := false
	file.FlatWalkAllNodes(idx, func(node uint32) {
		if found {
			return
		}
		switch file.FlatType(node) {
		case "interpolated_identifier", "interpolated_expression",
			"line_string_expression", "multi_line_string_expression",
			"line_str_ref", "multi_line_str_ref":
			found = true
		}
	})
	return found
}

func unquoteKotlinString(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "\"\"\"") && strings.HasSuffix(text, "\"\"\"") && len(text) >= 6 {
		return text[3 : len(text)-3], true
	}
	s, err := strconv.Unquote(text)
	if err != nil {
		return "", false
	}
	return s, true
}

func parseIntLiteral(text string) (int64, bool) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "_", ""))
	text = strings.TrimSuffix(strings.TrimSuffix(text, "L"), "l")
	v, err := strconv.ParseInt(text, 0, 64)
	return v, err == nil
}

func parseFloatLiteral(text string) (float64, bool) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "_", ""))
	text = strings.TrimSuffix(strings.TrimSuffix(text, "F"), "f")
	v, err := strconv.ParseFloat(text, 64)
	return v, err == nil
}

func ifParts(file *scanner.File, idx uint32) (cond uint32, thenBody uint32, elseBody uint32) {
	foundElse := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "control_structure_body":
			if !foundElse && thenBody == 0 {
				thenBody = child
			} else if foundElse && elseBody == 0 {
				elseBody = child
			}
		case "else":
			foundElse = true
		default:
			if cond == 0 && file.FlatIsNamed(child) {
				cond = child
			}
		}
	}
	return cond, thenBody, elseBody
}

func conditionTypeFacts(file *scanner.File, cond uint32) []TypeFact {
	var facts []TypeFact
	file.FlatWalkAllNodes(cond, func(node uint32) {
		expr, typ, positive, ok := isCheckParts(file, node)
		if !ok {
			return
		}
		facts = append(facts, TypeFact{Expr: expr, TypeName: terminalName(file, typ), Positive: positive})
	})
	return facts
}

func isCheckParts(file *scanner.File, node uint32) (expr uint32, typ uint32, positive bool, ok bool) {
	positive = true
	seenOperator := false
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "!is":
			positive = false
			seenOperator = true
			continue
		case "is":
			positive = true
			seenOperator = true
			continue
		}
		if !file.FlatIsNamed(child) {
			continue
		}
		if !seenOperator {
			if expr == 0 {
				expr = child
			}
			continue
		}
		if typ == 0 {
			typ = child
		}
	}
	return expr, typ, positive, seenOperator && expr != 0 && typ != 0
}

func permissionGuardsInCondition(ctx *v2.Context, cond uint32) []PermissionGuard {
	file := ctx.File
	var guards []PermissionGuard
	file.FlatWalkNodes(cond, "call_expression", func(call uint32) {
		localCtx := &v2.Context{File: file, Resolver: ctx.Resolver, CodeIndex: ctx.CodeIndex}
		target, ok := ResolveCallTarget(localCtx, call)
		if !ok || target.CalleeName != "checkSelfPermission" {
			return
		}
		for _, arg := range target.Arguments {
			if val, ok := EvalConst(localCtx, arg.Node); ok && val.Kind == "string" {
				guards = append(guards, PermissionGuard{Node: call, Permission: val, Positive: true})
			}
		}
	})
	return guards
}

func catchParameter(file *scanner.File, catchNode uint32) (string, string) {
	var name, typ string
	file.FlatWalkAllNodes(catchNode, func(node uint32) {
		if name != "" && typ != "" {
			return
		}
		switch file.FlatType(node) {
		case "simple_identifier":
			if name == "" {
				name = file.FlatNodeString(node, nil)
			}
		case "user_type", "nullable_type", "type_identifier":
			if typ == "" {
				typ = terminalName(file, node)
			}
		}
	})
	return name, typ
}

func sameNodeRange(file *scanner.File, a uint32, b uint32) bool {
	return file != nil && a != 0 && b != 0 &&
		file.FlatStartByte(a) == file.FlatStartByte(b) &&
		file.FlatEndByte(a) == file.FlatEndByte(b)
}

func simpleCallableName(qn string) string {
	qn = strings.TrimSpace(qn)
	if idx := strings.LastIndexAny(qn, ".#"); idx >= 0 && idx+1 < len(qn) {
		return qn[idx+1:]
	}
	return qn
}

func looksResolvedName(name string) bool {
	return strings.Contains(name, ".") || strings.Contains(name, "#")
}

func qualifiedNameMatchesAny(got string, allowed ...string) bool {
	if got == "" {
		return false
	}
	for _, want := range allowed {
		if want == "" {
			continue
		}
		if got == want || strings.TrimSuffix(got, "."+simpleCallableName(got)) == want {
			return true
		}
		if strings.HasSuffix(got, "."+want) || strings.HasSuffix(got, "#"+want) {
			return true
		}
	}
	return false
}

func typeMatchesAny(typ *typeinfer.ResolvedType, allowed ...string) bool {
	if typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	return typeNameIn(typ, allowed...)
}

func typeNameIn(typ *typeinfer.ResolvedType, names ...string) bool {
	if typ == nil {
		return false
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if typ.Name == name || typ.FQN == name || typ.IsSubtypeOf(name) {
			return true
		}
	}
	return false
}

func clampConfidence(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
