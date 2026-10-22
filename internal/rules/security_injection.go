package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/api/evidence"
	"github.com/kaeawc/krit/internal/scanner"
)

// ContentProviderQueryWithSelectionInterpolationRule detects interpolated
// selection strings passed to ContentResolver.query(...).
type ContentProviderQueryWithSelectionInterpolationRule struct {
	FlatDispatchBase
	BaseRule
}

// SQLInjectionRawQueryRule detects SQLiteDatabase SQL arguments built from
// interpolation or non-static concatenation.
type SQLInjectionRawQueryRule struct {
	FlatDispatchBase
	BaseRule
}

// RuntimeExecUnsafeShapeRule detects Runtime.getRuntime().exec(String) calls
// whose single command string is computed from non-static data.
type RuntimeExecUnsafeShapeRule struct {
	FlatDispatchBase
	BaseRule
}

// RoomRawQueryStringConcatRule detects Room SimpleSQLiteQuery SQL strings
// built from interpolation or non-static concatenation without bind args.
type RoomRawQueryStringConcatRule struct {
	FlatDispatchBase
	BaseRule
}

// ProcessBuilderShellArgRule detects shell ProcessBuilder invocations whose
// script argument is computed from non-static data.
type ProcessBuilderShellArgRule struct {
	FlatDispatchBase
	BaseRule
}

// JdbcStatementExecuteRule detects java.sql.Statement execution calls whose
// SQL argument is computed from non-static data.
type JdbcStatementExecuteRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *ContentProviderQueryWithSelectionInterpolationRule) Confidence() float64 {
	return api.ConfidenceMedium
}

func (r *SQLInjectionRawQueryRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RuntimeExecUnsafeShapeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RoomRawQueryStringConcatRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ProcessBuilderShellArgRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *JdbcStatementExecuteRule) Confidence() float64 { return api.ConfidenceMedium }

type sqlArgumentShape int

const (
	sqlArgumentStatic sqlArgumentShape = iota
	sqlArgumentInterpolated
	sqlArgumentComputed
)

func argumentIsUntrustedShape(file *scanner.File, expr uint32) sqlArgumentShape {
	if file == nil || expr == 0 {
		return sqlArgumentStatic
	}
	expr = flatUnwrapParenExpr(file, expr)
	text := strings.TrimSpace(file.FlatNodeText(expr))
	if text == "" || text == "null" {
		return sqlArgumentStatic
	}
	if flatContainsStringInterpolation(file, expr) {
		if sqlInterpolationUsesOnlyStaticSchemaConstants(text) {
			return sqlArgumentStatic
		}
		return sqlArgumentInterpolated
	}
	if operands := splitSQLConcatOperands(text); len(operands) > 1 {
		for _, operand := range operands {
			if !sqlStaticOperand(operand) {
				return sqlArgumentComputed
			}
		}
		return sqlArgumentStatic
	}
	if sqlStaticOperand(text) {
		return sqlArgumentStatic
	}
	return sqlArgumentComputed
}

func sqlStaticOperand(text string) bool {
	text = strings.TrimSpace(text)
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		inner := strings.TrimSpace(text[1 : len(text)-1])
		if inner == "" {
			break
		}
		text = inner
	}
	if text == "null" {
		return true
	}
	if isStringLiteralExpr(text) {
		return !strings.Contains(text, "$")
	}
	return sqlSchemaConstantName(sqlLastIdentifierSegment(text))
}

func splitSQLConcatOperands(text string) []string {
	var out []string
	start := 0
	depth := 0
	inString := false
	rawQuoteCount := 0
	escaped := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if inString {
			if rawQuoteCount == 3 {
				if i+2 < len(text) && text[i:i+3] == `"""` {
					inString = false
					rawQuoteCount = 0
					i += 2
				}
				continue
			}
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			if i+2 < len(text) && text[i:i+3] == `"""` {
				inString = true
				rawQuoteCount = 3
				i += 2
			} else {
				inString = true
			}
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '+':
			if depth == 0 {
				out = append(out, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	out = append(out, strings.TrimSpace(text[start:]))
	return out
}

var sqlInterpolationIdentifierPattern = regexp.MustCompile(`\$\{?\s*([A-Za-z_][A-Za-z0-9_.]*)`)

func sqlInterpolationUsesOnlyStaticSchemaConstants(text string) bool {
	matches := sqlInterpolationIdentifierPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return false
	}
	for _, match := range matches {
		if len(match) < 2 || !sqlSchemaConstantName(sqlLastIdentifierSegment(match[1])) {
			return false
		}
	}
	return true
}

func sqlLastIdentifierSegment(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, ")")
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		text = text[dot+1:]
	}
	return strings.Trim(text, "` ")
}

func sqlSchemaConstantName(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "TABLE_") || strings.HasPrefix(name, "COLUMN_") {
		return true
	}
	if strings.HasSuffix(name, "_TABLE") || strings.HasSuffix(name, "_COLUMN") || strings.HasSuffix(name, "_KEY") {
		return true
	}
	return strings.ToUpper(name) == name && strings.ContainsAny(name, "_")
}

func sqlInjectionCallName(file *scanner.File, call uint32) string {
	switch file.FlatType(call) {
	case "call_expression":
		return flatCallExpressionName(file, call)
	case "method_invocation":
		return javaMethodInvocationName(file, call)
	default:
		return ""
	}
}

func sqlInjectionSQLArgument(file *scanner.File, call uint32, name string) uint32 {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return 0
		}
		if name == "query" {
			if arg := flatNamedValueArgument(file, args, "selection"); arg != 0 {
				return flatValueArgumentExpression(file, arg)
			}
			return flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 2))
		}
		if arg := flatNamedValueArgument(file, args, "sql"); arg != 0 {
			return flatValueArgumentExpression(file, arg)
		}
		return flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return 0
		}
		index := 0
		if name == "query" {
			index = 2
		}
		current := 0
		for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if current == index {
				return child
			}
			current++
		}
	}
	return 0
}

// isSQLiteDatabaseFQN reports whether fqn names the SQLite database type
// (or its androidx Support variant) by FQN equality. Receiver-typing in
// SqlInjectionRawQuery and friends goes through evidence.ResolveOwner →
// FQN compare instead of substring-matching the receiver text.
func isSQLiteDatabaseFQN(fqn string) bool {
	switch fqn {
	case "android.database.sqlite.SQLiteDatabase",
		"androidx.sqlite.db.SupportSQLiteDatabase":
		return true
	}
	return false
}

func runtimeExecSingleArgument(file *scanner.File, call uint32) uint32 {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return 0
		}
		arg := flatPositionalValueArgument(file, args, 0)
		if arg == 0 || flatPositionalValueArgument(file, args, 1) != 0 {
			return 0
		}
		expr := flatValueArgumentExpression(file, arg)
		if !runtimeExecArgumentLooksStringCommand(file, expr) {
			return 0
		}
		return expr
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) != 1 {
			return 0
		}
		expr := file.FlatNamedChild(args, 0)
		if !runtimeExecArgumentLooksStringCommand(file, expr) {
			return 0
		}
		return expr
	default:
		return 0
	}
}

func runtimeExecArgumentLooksStringCommand(file *scanner.File, expr uint32) bool {
	if file == nil || expr == 0 {
		return false
	}
	expr = flatUnwrapParenExpr(file, expr)
	text := strings.TrimSpace(file.FlatNodeText(expr))
	if text == "" {
		return false
	}
	if strings.HasPrefix(text, "arrayOf(") || strings.HasPrefix(text, "new String[]") || strings.HasPrefix(text, "String[]") {
		return false
	}
	return isStringLiteralExpr(text) || flatContainsStringInterpolation(file, expr) || len(splitSQLConcatOperands(text)) > 1
}

// roomRawQuerySQLArgWithoutBindArgs returns the first positional SQL
// argument of a SimpleSQLiteQuery call ONLY when the call has no
// bind-args (no second positional argument). Receiver-typing has already
// been validated by the caller via evidence.ResolveCalleeFQN.
func roomRawQuerySQLArgWithoutBindArgs(file *scanner.File, call uint32) uint32 {
	if file == nil || call == 0 {
		return 0
	}
	_, args := flatCallExpressionParts(file, call)
	if args == 0 || flatPositionalValueArgument(file, args, 1) != 0 {
		return 0
	}
	first := flatPositionalValueArgument(file, args, 0)
	return flatValueArgumentExpression(file, first)
}

// processBuilderShellScriptArgument returns the third argument of a
// ProcessBuilder constructor call when the first two arguments form a
// shell + "-c" pair (e.g. ("sh", "-c", "<script>")). Receiver-typing
// is validated by the caller via evidence.ResolveCalleeFQN — this
// helper assumes the call has already been confirmed to construct
// java.lang.ProcessBuilder.
func processBuilderShellScriptArgument(file *scanner.File, idx uint32) uint32 {
	args := processBuilderArgumentExpressions(file, idx)
	if len(args) == 1 && file.FlatType(args[0]) == "call_expression" {
		name := flatCallExpressionName(file, args[0])
		if name == "listOf" || name == "arrayOf" {
			args = processBuilderArgumentExpressions(file, args[0])
		}
	}
	if len(args) < 3 {
		return 0
	}
	if !processBuilderLiteralIsShell(file, args[0]) || processBuilderLiteral(file, args[1]) != "-c" {
		return 0
	}
	return args[2]
}

func processBuilderArgumentExpressions(file *scanner.File, idx uint32) []uint32 {
	switch file.FlatType(idx) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, idx)
		if args == 0 {
			return nil
		}
		var out []uint32
		for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatType(arg) != "value_argument" || flatHasValueArgumentLabel(file, arg) {
				continue
			}
			if expr := flatValueArgumentExpression(file, arg); expr != 0 {
				out = append(out, expr)
			}
		}
		return out
	case "object_creation_expression":
		args, ok := file.FlatFindChild(idx, "argument_list")
		if !ok {
			return nil
		}
		var out []uint32
		for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatIsNamed(arg) {
				out = append(out, arg)
			}
		}
		return out
	default:
		return nil
	}
}

func processBuilderLiteralIsShell(file *scanner.File, idx uint32) bool {
	switch processBuilderLiteral(file, idx) {
	case "sh", "bash", "zsh", "/bin/sh", "/bin/bash", "/bin/zsh":
		return true
	default:
		return false
	}
}

func processBuilderLiteral(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	idx = flatUnwrapParenExpr(file, idx)
	if file.FlatType(idx) != "string_literal" || flatContainsStringInterpolation(file, idx) {
		return ""
	}
	return stringLiteralContent(file, idx)
}

var jdbcStatementExecuteMethods = map[string]bool{
	"execute":            true,
	"executeQuery":       true,
	"executeUpdate":      true,
	"executeLargeUpdate": true,
}

// jdbcStatementMethodName returns the method name of a call_expression /
// method_invocation, or "" if the node is neither.
func jdbcStatementMethodName(file *scanner.File, call uint32) string {
	switch file.FlatType(call) {
	case "call_expression":
		return flatCallExpressionName(file, call)
	case "method_invocation":
		return javaMethodInvocationName(file, call)
	}
	return ""
}

// jdbcStatementSQLArgumentExpr returns the first SQL argument expression
// of a JDBC execute*() call. Receiver-typing has already been validated
// by the caller — this is purely positional argument extraction.
func jdbcStatementSQLArgumentExpr(file *scanner.File, call uint32) uint32 {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		return flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return 0
		}
		return file.FlatNamedChild(args, 0)
	}
	return 0
}

// jdbcReceiverIsStatement reports whether the call's receiver is provably
// a java.sql.Statement. Two cases:
//
//  1. Chained: connection.createStatement().execute(...) — call.ReceiverIdx
//     points to the inner call_expression / method_invocation. We confirm
//     the inner callee is `createStatement` and that its receiver resolves
//     to java.sql.Connection.
//  2. Named receiver: stmt.execute(...) — try ResolveOwner first (covers
//     Java explicit `Statement stmt = ...` and any future case where the
//     resolver knows the local's type). Fall back to an AST trace of the
//     property/local initializer for Kotlin's `val stmt = conn.createStatement()`,
//     which the in-process resolver cannot infer because Connection's
//     return types are not stdlib facts.
func jdbcReceiverIsStatement(file *scanner.File, ev *evidence.Evidence, call *evidence.Call) bool {
	if call == nil {
		return false
	}
	if call.ReceiverIdx != 0 {
		switch file.FlatType(call.ReceiverIdx) {
		case "call_expression", "method_invocation":
			return jdbcCallProducesStatement(ev, call.ReceiverIdx)
		}
	}
	if call.Receiver == "" || strings.Contains(call.Receiver, ".") {
		return false
	}
	if fqn, src := ev.ResolveOwner(call); src != evidence.OwnerUnknown && fqn == "java.sql.Statement" {
		return true
	}
	return jdbcReceiverBoundToCreateStatement(file, ev, call.Idx, call.Receiver)
}

// jdbcCallProducesStatement reports whether callIdx is a call to
// `createStatement()` on a java.sql.Connection — the only Connection
// method whose return type is exactly Statement (NOT PreparedStatement /
// CallableStatement, which the rule deliberately ignores).
func jdbcCallProducesStatement(ev *evidence.Evidence, callIdx uint32) bool {
	inner := ev.Call(callIdx)
	if inner == nil || inner.Callee != "createStatement" {
		return false
	}
	fqn, src := ev.ResolveOwner(inner)
	return src != evidence.OwnerUnknown && fqn == "java.sql.Connection"
}

// jdbcReceiverBoundToCreateStatement walks the enclosing callable for a
// property/local declaration of `receiver` whose initializer is a
// connection.createStatement() call. Used for the Kotlin val-stmt case
// where the source resolver cannot infer the local's type from a non-
// stdlib return.
func jdbcReceiverBoundToCreateStatement(file *scanner.File, ev *evidence.Evidence, callIdx uint32, receiver string) bool {
	fn, ok := flatEnclosingCallable(file, callIdx)
	if !ok {
		return false
	}
	targetRow := file.FlatRow(callIdx)
	found := false
	for _, nodeType := range []string{"property_declaration", "local_variable_declaration"} {
		file.FlatWalkNodes(fn, nodeType, func(decl uint32) {
			if found || file.FlatRow(decl) > targetRow {
				return
			}
			if !jdbcDeclarationDeclaresName(file, decl, receiver) {
				return
			}
			initCall := jdbcDeclarationInitializerCall(file, decl)
			if initCall != 0 && jdbcCallProducesStatement(ev, initCall) {
				found = true
			}
		})
	}
	return found
}

// jdbcDeclarationDeclaresName checks whether a property_declaration
// (Kotlin) or local_variable_declaration (Java) declares `name`.
func jdbcDeclarationDeclaresName(file *scanner.File, decl uint32, name string) bool {
	if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
		for c := file.FlatFirstChild(vd); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "simple_identifier" && file.FlatNodeText(c) == name {
				return true
			}
		}
	}
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "variable_declarator" {
			for vc := file.FlatFirstChild(c); vc != 0; vc = file.FlatNextSib(vc) {
				if file.FlatType(vc) == "identifier" && file.FlatNodeText(vc) == name {
					return true
				}
			}
		}
	}
	return false
}

// jdbcDeclarationInitializerCall returns the flat node of the first
// call_expression / method_invocation initializer of decl, or 0.
func jdbcDeclarationInitializerCall(file *scanner.File, decl uint32) uint32 {
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "call_expression", "method_invocation":
			return c
		case "variable_declarator":
			for vc := file.FlatFirstChild(c); vc != 0; vc = file.FlatNextSib(vc) {
				switch file.FlatType(vc) {
				case "call_expression", "method_invocation":
					return vc
				}
			}
		}
	}
	return 0
}
