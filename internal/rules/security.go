package rules

import (
	"regexp"
	"strconv"
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

// LogPiiRule detects logger calls that interpolate or concatenate sensitive
// variable names into log messages.
type LogPiiRule struct {
	FlatDispatchBase
	BaseRule
	PiiNamePattern *regexp.Regexp
}

// JdbcStatementExecuteRule detects java.sql.Statement execution calls whose
// SQL argument is computed from non-static data.
type JdbcStatementExecuteRule struct {
	FlatDispatchBase
	BaseRule
}

// XMLExternalEntityRule detects XML parser factories created without obvious
// XXE-disabling hardening in the same callable scope.
type XMLExternalEntityRule struct {
	FlatDispatchBase
	BaseRule
}

// JavaObjectInputStreamRule detects direct Java serialization input streams in
// production source.
type JavaObjectInputStreamRule struct {
	FlatDispatchBase
	BaseRule
}

// JacksonDefaultTypingRule detects Jackson default typing APIs that can enable
// polymorphic deserialization gadget attacks.
type JacksonDefaultTypingRule struct {
	FlatDispatchBase
	BaseRule
}

// GsonPolymorphicFromJSONRule detects Gson deserialization into Object/Any.
type GsonPolymorphicFromJSONRule struct {
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

func (r *LogPiiRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *JdbcStatementExecuteRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *XMLExternalEntityRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *JavaObjectInputStreamRule) Confidence() float64 { return api.ConfidenceMediumHigh }

func (r *JacksonDefaultTypingRule) Confidence() float64 { return api.ConfidenceMediumHigh }

func (r *GsonPolymorphicFromJSONRule) Confidence() float64 { return api.ConfidenceMediumHigh }

var defaultLogPiiNamePattern = regexp.MustCompile(`(?i)(password|passwd|token|secret|apiKey|api_key|authHeader|authorization|ssn|pan|cvv|jwt|sessionId|cookie)`)

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

func logPiiEffectivePattern(r *LogPiiRule) *regexp.Regexp {
	if r != nil && r.PiiNamePattern != nil {
		return r.PiiNamePattern
	}
	return defaultLogPiiNamePattern
}

var androidLogMethods = map[string]bool{"v": true, "d": true, "i": true, "w": true, "e": true, "wtf": true}

func logPiiIsLoggerCall(file *scanner.File, call uint32) bool {
	name := javaAwareCallName(file, call)
	if name == "println" && logPiiIsStdoutPrintln(file, call) {
		return true
	}
	text := file.FlatNodeText(call)
	receiver := databaseCallReceiverName(file, call)
	if androidLogMethods[name] {
		if receiver == "Log" || strings.HasSuffix(receiver, ".Log") {
			return !logPiiHasLocalType(file, "Log")
		}
		if receiver == "Timber" || strings.Contains(text, "Timber.") {
			return !logPiiHasLocalType(file, "Timber")
		}
	}
	if loggerLevelMethods[name] {
		if file.FlatType(call) == "call_expression" {
			return receiverIsKnownLoggerFlat(file, call, receiver)
		}
		return receiver == "logger" || receiver == "log" || receiver == "LOG" || receiver == "LOGGER" ||
			strings.Contains(strings.ToLower(receiver), "logger")
	}
	return false
}

// logPiiIsStdoutPrintln returns true only when the call resolves to a
// stdout/stderr-style println: a bare `println(...)` that is not shadowed
// by a same-file declaration or non-kotlin.io import, or an explicit
// `System.out.println` / `System.err.println` invocation (Kotlin or Java).
// Returning true for any method named `println` would false-positive on
// arbitrary `obj.println(...)` calls and on files that shadow the kotlin.io
// built-in with a local function.
func logPiiIsStdoutPrintln(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 {
		return false
	}
	switch file.FlatType(call) {
	case "call_expression":
		callee, _ := flatCallExpressionParts(file, call)
		if callee != 0 && file.FlatType(callee) == "navigation_expression" {
			segs := flatNavigationChainIdentifiers(file, callee)
			return len(segs) == 3 && segs[0] == "System" &&
				(segs[1] == "out" || segs[1] == "err") &&
				segs[2] == "println"
		}
		// Bare `println(...)` resolves to kotlin.io.println only when nothing
		// in the file shadows the built-in name.
		return !kotlinFileShadowsBuiltinPrint(file, "println")
	case "method_invocation":
		// Java has no bare top-level println; require System.out / System.err
		// as the receiver. The Java parser represents `System.out` as a
		// single field_access child, so wrongViewCastCallReceiverName (which
		// joins bare identifier siblings) does not capture it. Trim trailing
		// whitespace from the receiver text and compare directly.
		receiverText := logPiiMethodInvocationReceiverText(file, call)
		return receiverText == "System.out" || receiverText == "System.err"
	}
	return false
}

// logPiiMethodInvocationReceiverText returns the textual form of a Java
// method_invocation's receiver — the named child that appears before the
// `.identifier(args)` suffix — or "" when the call has no qualifier.
// Tracks "saw a `.` separator" so a bare `foo(args)` returns "" instead of
// "foo". Used to distinguish `System.out.println(...)` from `obj.println(...)`.
func logPiiMethodInvocationReceiverText(file *scanner.File, call uint32) string {
	if file == nil || call == 0 || file.FlatType(call) != "method_invocation" {
		return ""
	}
	var receiver uint32
	sawDot := false
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "argument_list" {
			break
		}
		if file.FlatType(child) == "." {
			sawDot = true
			continue
		}
		if !file.FlatIsNamed(child) {
			continue
		}
		if sawDot {
			// We've already captured the receiver in `receiver`; remaining
			// named children are the method name (or type arguments).
			break
		}
		receiver = child
	}
	if !sawDot || receiver == 0 {
		return ""
	}
	return strings.TrimSpace(file.FlatNodeText(receiver))
}

func logPiiHasLocalType(file *scanner.File, name string) bool {
	found := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "interface_declaration"} {
		file.FlatWalkNodes(0, nodeType, func(idx uint32) {
			if found {
				return
			}
			if extractIdentifierFlat(file, idx) == name {
				found = true
			}
		})
	}
	return found
}

func logPiiSensitiveArgument(file *scanner.File, call uint32, pattern *regexp.Regexp) uint32 {
	for _, arg := range logPiiArgumentExpressions(file, call) {
		if logPiiExpressionMentionsSensitiveIdentifier(file, arg, pattern) {
			return arg
		}
	}
	return 0
}

func logPiiArgumentExpressions(file *scanner.File, call uint32) []uint32 {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return nil
		}
		var out []uint32
		for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatType(arg) != "value_argument" {
				continue
			}
			if expr := flatValueArgumentExpression(file, arg); expr != 0 {
				out = append(out, expr)
			}
		}
		return out
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
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

var logPiiInterpolationNamePattern = regexp.MustCompile(`\$\{?\s*([A-Za-z_][A-Za-z0-9_]*)`)

func logPiiExpressionMentionsSensitiveIdentifier(file *scanner.File, expr uint32, pattern *regexp.Regexp) bool {
	if file == nil || expr == 0 || pattern == nil {
		return false
	}
	text := file.FlatNodeText(expr)
	if flatContainsStringInterpolation(file, expr) {
		for _, match := range logPiiInterpolationNamePattern.FindAllStringSubmatch(text, -1) {
			if len(match) > 1 && pattern.MatchString(match[1]) {
				return true
			}
		}
	}
	if len(splitSQLConcatOperands(text)) > 1 {
		return logPiiConcatMentionsSensitiveIdentifier(file, expr, pattern)
	}
	return false
}

func logPiiConcatMentionsSensitiveIdentifier(file *scanner.File, expr uint32, pattern *regexp.Regexp) bool {
	found := false
	file.FlatWalkAllNodes(expr, func(idx uint32) {
		if found {
			return
		}
		switch file.FlatType(idx) {
		case "simple_identifier", "identifier":
			name := file.FlatNodeText(idx)
			if pattern.MatchString(name) {
				found = true
			}
		}
	})
	return found
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

var xxeFactoryReceivers = map[string]bool{
	"DocumentBuilderFactory": true,
	"SAXParserFactory":       true,
	"XMLInputFactory":        true,
	"TransformerFactory":     true,
	"SchemaFactory":          true,
}

func xmlExternalEntityFactoryCall(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 || javaAwareCallName(file, call) != "newInstance" {
		return false
	}
	receiver := databaseCallReceiverName(file, call)
	if dot := strings.LastIndex(receiver, "."); dot >= 0 {
		receiver = receiver[dot+1:]
	}
	if !xxeFactoryReceivers[receiver] {
		return false
	}
	return sourceImportsOrMentions(file, receiver)
}

// xxeFeatureHardening maps each XXE-related feature URL to the boolean
// value that disables the unsafe behavior when passed to
// `factory.setFeature(URL, BOOL)`.
var xxeFeatureHardening = map[string]bool{
	"http://apache.org/xml/features/disallow-doctype-decl":    true,
	"http://xml.org/sax/features/external-general-entities":   false,
	"http://xml.org/sax/features/external-parameter-entities": false,
}

func xmlExternalEntityHasHardeningAfter(file *scanner.File, call uint32) bool {
	scope, ok := flatEnclosingCallable(file, call)
	if !ok {
		return false
	}
	callStart := file.FlatStartByte(call)
	found := false
	sawXInclude := false
	sawExpandEntities := false

	file.FlatWalkAllNodes(scope, func(n uint32) {
		if found || file.FlatStartByte(n) <= callStart {
			return
		}
		switch file.FlatType(n) {
		case "call_expression", "method_invocation":
			if xxeCallIsHardening(file, n) {
				found = true
			}
		case "assignment":
			switch xxePropertyAssignmentName(file, n) {
			case "isXIncludeAware":
				sawXInclude = true
			case "isExpandEntityReferences":
				sawExpandEntities = true
			}
			if sawXInclude && sawExpandEntities {
				found = true
			}
		}
	})
	return found
}

// xxeCallIsHardening reports whether the call_expression / method_invocation
// is one of the XXE-disabling setFeature / setProperty invocations with the
// expected literal arguments. Match is AST-based so formatting, whitespace,
// or interleaved comments do not break detection.
func xxeCallIsHardening(file *scanner.File, call uint32) bool {
	name := javaAwareCallName(file, call)
	switch name {
	case "setFeature":
		urlExpr, valExpr := xxeCallTwoArgExprs(file, call)
		if urlExpr == 0 || valExpr == 0 {
			return false
		}
		url, ok := xxeLiteralStringContent(file, urlExpr)
		if !ok {
			return false
		}
		expected, known := xxeFeatureHardening[url]
		if !known {
			return false
		}
		got, ok := xxeLiteralBool(file, valExpr)
		return ok && got == expected
	case "setProperty":
		urlExpr, valExpr := xxeCallTwoArgExprs(file, call)
		if urlExpr == 0 || valExpr == 0 {
			return false
		}
		if !xxeExprIsSupportDTD(file, urlExpr) {
			return false
		}
		got, ok := xxeLiteralBool(file, valExpr)
		return ok && !got
	}
	return false
}

// xxeCallTwoArgExprs returns the first two positional argument expressions
// of a two-arg call, regardless of whether it parsed as a Kotlin
// call_expression (value_arguments) or Java method_invocation
// (argument_list).
func xxeCallTwoArgExprs(file *scanner.File, call uint32) (uint32, uint32) {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return 0, 0
		}
		a0 := flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
		a1 := flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 1))
		return a0, a1
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) < 2 {
			return 0, 0
		}
		return file.FlatNamedChild(args, 0), file.FlatNamedChild(args, 1)
	}
	return 0, 0
}

// xxeLiteralStringContent returns the unquoted content of a string literal
// expression with no interpolation, or "", false.
func xxeLiteralStringContent(file *scanner.File, expr uint32) (string, bool) {
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		if flatContainsStringInterpolation(file, expr) {
			return "", false
		}
		if value := stringLiteralContent(file, expr); value != "" {
			return value, true
		}
		text := strings.TrimSpace(file.FlatNodeText(expr))
		if unq, err := strconv.Unquote(text); err == nil {
			return unq, true
		}
	}
	return "", false
}

// xxeLiteralBool returns the boolean value of a `true` / `false` literal,
// or false, false for anything else.
func xxeLiteralBool(file *scanner.File, expr uint32) (bool, bool) {
	expr = flatUnwrapParenExpr(file, expr)
	switch strings.TrimSpace(file.FlatNodeText(expr)) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// xxeExprIsSupportDTD reports whether the expression resolves to a
// reference whose trailing identifier is `SUPPORT_DTD` (matching both
// `XMLInputFactory.SUPPORT_DTD` and the fully-qualified variant), or the
// bare `SUPPORT_DTD` constant.
func xxeExprIsSupportDTD(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		return file.FlatNodeTextEquals(expr, "SUPPORT_DTD")
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifierEquals(file, expr, "SUPPORT_DTD")
	case "field_access":
		// Java: XMLInputFactory.SUPPORT_DTD parses as field_access whose last
		// named child is the identifier.
		count := file.FlatNamedChildCount(expr)
		if count == 0 {
			return false
		}
		last := file.FlatNamedChild(expr, count-1)
		return file.FlatNodeTextEquals(last, "SUPPORT_DTD")
	}
	return false
}

// xxePropertyAssignmentName returns the trailing property name of a Kotlin
// property assignment whose right-hand side is the literal `false`, or "".
// Used to detect the `factory.isXIncludeAware = false` /
// `factory.isExpandEntityReferences = false` hardening pair.
func xxePropertyAssignmentName(file *scanner.File, assignment uint32) string {
	_, tail := chainSplitTrailing(kotlinAssignmentTargetChain(file, assignment))
	if tail == "" {
		return ""
	}
	value := findKotlinAssignmentValue(file, assignment)
	if value == 0 {
		return ""
	}
	if got, ok := xxeLiteralBool(file, value); !ok || got {
		return ""
	}
	return tail
}

func javaObjectInputStreamConstructor(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || scanner.IsTestFile(file.Path) {
		return false
	}
	if !sourceImportsOrMentions(file, "java.io.ObjectInputStream") {
		return false
	}
	if javaObjectInputStreamDuplicateNestedCall(file, idx) {
		return false
	}
	if javaObjectInputStreamSafeSubclassScope(file, idx) {
		return false
	}
	return javaObjectInputStreamCallShape(file, idx)
}

func javaObjectInputStreamCallShape(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "call_expression":
		// Kotlin: the callee is the first named child of the call_expression —
		// either a bare simple_identifier or a navigation_expression. Inspect
		// it directly so string-literal arguments that mention the class
		// cannot satisfy the shape check.
		navExpr, _ := flatCallExpressionParts(file, idx)
		if navExpr != 0 {
			chain := flatNavigationChainIdentifiers(file, navExpr)
			if len(chain) == 3 && chain[0] == "java" && chain[1] == "io" && chain[2] == "ObjectInputStream" {
				return true
			}
			return false
		}
		return flatCallExpressionName(file, idx) == "ObjectInputStream"
	case "object_creation_expression":
		// Java: `new X(args)` parses with the type as a named child —
		// type_identifier for `ObjectInputStream`, scoped_type_identifier
		// whose tail identifier is `ObjectInputStream` for the FQN form.
		for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
			if !file.FlatIsNamed(c) {
				continue
			}
			switch file.FlatType(c) {
			case "type_identifier":
				return file.FlatNodeTextEquals(c, "ObjectInputStream")
			case "scoped_type_identifier":
				return file.FlatNodeTextEquals(c, "java.io.ObjectInputStream")
			default:
				return false
			}
		}
		return false
	default:
		return false
	}
}

func javaObjectInputStreamDuplicateNestedCall(file *scanner.File, idx uint32) bool {
	if file.FlatType(idx) != "call_expression" {
		return false
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "call_expression":
			return file.FlatRow(parent) == file.FlatRow(idx) &&
				file.FlatCol(parent) == file.FlatCol(idx) &&
				javaObjectInputStreamCallShape(file, parent)
		case "function_declaration", "method_declaration", "class_declaration", "source_file":
			return false
		}
	}
	return false
}

func javaObjectInputStreamSafeSubclassScope(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration":
			text := file.FlatNodeText(cur)
			if strings.Contains(text, "ObjectInputStream") && strings.Contains(text, "resolveClass") {
				return true
			}
			return false
		case "source_file":
			return false
		}
	}
	return false
}

func jacksonDefaultTypingCall(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	name := javaAwareCallName(file, idx)
	if name != "enableDefaultTyping" && name != "activateDefaultTyping" {
		return false
	}
	if !jacksonFileImportsDatabind(file) {
		return false
	}
	return jacksonDefaultTypingReceiverLooksMapper(file, idx)
}

func jacksonFileImportsDatabind(file *scanner.File) bool {
	if file == nil {
		return false
	}
	text := string(file.Content)
	return strings.Contains(text, "com.fasterxml.jackson.databind") ||
		strings.Contains(text, "com.fasterxml.jackson.dataformat.xml.XmlMapper")
}

func jacksonDefaultTypingReceiverLooksMapper(file *scanner.File, idx uint32) bool {
	text := strings.Join(strings.Fields(file.FlatNodeText(idx)), "")
	if strings.Contains(text, "ObjectMapper().") ||
		strings.Contains(text, "XmlMapper().") ||
		strings.Contains(text, "JsonMapper.builder()") ||
		strings.Contains(text, "newObjectMapper().") ||
		strings.Contains(text, "newXmlMapper().") ||
		strings.Contains(text, "newJsonMapper") ||
		strings.Contains(text, "com.fasterxml.jackson.databind.ObjectMapper(") {
		return true
	}
	receiver := databaseCallReceiverName(file, idx)
	if receiver == "" {
		return false
	}
	if strings.Contains(receiver, "ObjectMapper") || strings.Contains(receiver, "XmlMapper") || strings.Contains(receiver, "JsonMapper") {
		return true
	}
	return receiverDeclaredFromCall(file, idx, "ObjectMapper") ||
		receiverDeclaredFromCall(file, idx, "XmlMapper") ||
		receiverDeclaredFromCall(file, idx, "JsonMapper") ||
		jacksonReceiverDeclaredAsMapper(file, idx, receiver)
}

func jacksonReceiverDeclaredAsMapper(file *scanner.File, call uint32, receiver string) bool {
	scope, ok := flatEnclosingCallable(file, call)
	if !ok {
		return false
	}
	found := false
	file.FlatWalkAllNodes(scope, func(n uint32) {
		if found || file.FlatStartByte(n) >= file.FlatStartByte(call) {
			return
		}
		typ := file.FlatType(n)
		if typ != "property_declaration" && typ != "variable_declaration" && typ != "local_variable_declaration" {
			return
		}
		text := file.FlatNodeText(n)
		if !strings.Contains(text, receiver) {
			return
		}
		compact := strings.Join(strings.Fields(text), "")
		if strings.Contains(compact, "ObjectMapper") || strings.Contains(compact, "XmlMapper") || strings.Contains(compact, "JsonMapper") {
			found = true
		}
	})
	return found
}

func gsonPolymorphicFromJSONCall(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || javaAwareCallName(file, idx) != "fromJson" {
		return false
	}
	if !gsonFileImportsGson(file) || !gsonFromJSONReceiverLooksGson(file, idx) {
		return false
	}
	arg, ok := gsonFromJSONTypeArg(file, idx)
	return ok && gsonFromJSONPolymorphicTypeArg(file.FlatNodeText(arg))
}

func gsonFileImportsGson(file *scanner.File) bool {
	if file == nil {
		return false
	}
	text := string(file.Content)
	return strings.Contains(text, "com.google.gson.Gson") ||
		strings.Contains(text, "com.google.gson.GsonBuilder") ||
		strings.Contains(text, "com.google.gson.*")
}

func gsonFromJSONReceiverLooksGson(file *scanner.File, idx uint32) bool {
	text := strings.Join(strings.Fields(file.FlatNodeText(idx)), "")
	if strings.Contains(text, "com.google.gson.Gson(") ||
		strings.Contains(text, "Gson().fromJson(") ||
		strings.Contains(text, "newGson().fromJson(") ||
		strings.Contains(text, "com.google.gson.GsonBuilder(") ||
		strings.Contains(text, "GsonBuilder().create().fromJson(") ||
		strings.Contains(text, "newGsonBuilder().create().fromJson(") {
		return true
	}
	receiver := databaseCallReceiverName(file, idx)
	if receiver == "" {
		return false
	}
	if strings.Contains(receiver, "Gson") {
		return true
	}
	if staticIvFileDeclaresType(file, "Gson") || staticIvFileDeclaresType(file, "GsonBuilder") {
		return false
	}
	return receiverDeclaredFromCall(file, idx, "Gson") ||
		receiverDeclaredFromCall(file, idx, "GsonBuilder") ||
		gsonReceiverDeclaredAsGson(file, idx, receiver)
}

func gsonReceiverDeclaredAsGson(file *scanner.File, call uint32, receiver string) bool {
	scope, ok := flatEnclosingCallable(file, call)
	if !ok {
		return false
	}
	found := false
	file.FlatWalkAllNodes(scope, func(n uint32) {
		if found || file.FlatStartByte(n) >= file.FlatStartByte(call) {
			return
		}
		typ := file.FlatType(n)
		if typ != "property_declaration" && typ != "variable_declaration" && typ != "local_variable_declaration" {
			return
		}
		text := file.FlatNodeText(n)
		if !strings.Contains(text, receiver) {
			return
		}
		compact := strings.Join(strings.Fields(text), "")
		if strings.Contains(compact, "Gson()") || strings.Contains(compact, "newGson()") ||
			strings.Contains(compact, "GsonBuilder().create()") || strings.Contains(compact, "newGsonBuilder().create()") ||
			strings.Contains(compact, "Gson"+receiver+"=") || strings.Contains(compact, "Gson"+receiver+";") {
			found = true
		}
	})
	return found
}

func gsonFromJSONTypeArg(file *scanner.File, idx uint32) (uint32, bool) {
	switch file.FlatType(idx) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, idx)
		arg := flatPositionalValueArgument(file, args, 1)
		if arg == 0 {
			return 0, false
		}
		expr := flatValueArgumentExpression(file, arg)
		return expr, expr != 0
	case "method_invocation":
		args, ok := file.FlatFindChild(idx, "argument_list")
		if !ok || file.FlatNamedChildCount(args) < 2 {
			return 0, false
		}
		return file.FlatNamedChild(args, 1), true
	default:
		return 0, false
	}
}

func gsonFromJSONPolymorphicTypeArg(text string) bool {
	compact := strings.Join(strings.Fields(text), "")
	switch compact {
	case "Any::class.java", "kotlin.Any::class.java", "Object::class.java", "java.lang.Object::class.java",
		"Any::class", "kotlin.Any::class", "Object::class", "java.lang.Object::class",
		"Object.class", "java.lang.Object.class":
		return true
	default:
		return false
	}
}

// HardcodedBearerTokenRule detects bearer authorization strings that embed a
// long token literal directly in source.
type HardcodedBearerTokenRule struct {
	FlatDispatchBase
	BaseRule
}

// HardcodedGcpServiceAccountRule detects embedded GCP service-account JSON and
// private keys committed into source files.
type HardcodedGcpServiceAccountRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *HardcodedGcpServiceAccountRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *HardcodedBearerTokenRule) Confidence() float64 { return api.ConfidenceMedium }

func extractHardcodedBearerToken(text string) (string, bool) {
	body, ok := kotlinStringLiteralBody(text)
	if !ok || !strings.HasPrefix(body, "Bearer ") {
		return "", false
	}

	rest := strings.TrimSpace(strings.TrimPrefix(body, "Bearer "))
	if rest == "" {
		return "", false
	}

	var token string
	switch {
	case strings.HasPrefix(rest, "${") && strings.HasSuffix(rest, "}"):
		inner := strings.TrimSpace(rest[2 : len(rest)-1])
		var literal bool
		token, literal = kotlinStringLiteralBody(inner)
		if !literal {
			return "", false
		}
	case strings.Contains(rest, "${") || strings.Contains(rest, "$"):
		return "", false
	case strings.ContainsAny(rest, " \t\r\n"):
		return "", false
	default:
		token = rest
	}

	if !looksLikeHardcodedBearerToken(token) {
		return "", false
	}

	return token, true
}

func kotlinStringLiteralBody(text string) (string, bool) {
	text = strings.TrimSpace(text)
	switch {
	case len(text) >= 6 && strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`):
		return text[3 : len(text)-3], true
	case len(text) >= 2 && strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`):
		unquoted, err := strconv.Unquote(text)
		if err == nil {
			return unquoted, true
		}
		return text[1 : len(text)-1], true
	default:
		return "", false
	}
}

func looksLikeHardcodedBearerToken(token string) bool {
	token = strings.TrimSpace(token)
	if len(token) < 16 {
		return false
	}
	return !secretLooksLikePlaceholder(token)
}

func looksLikeHardcodedGcpServiceAccount(body string) bool {
	trimmed := strings.TrimSpace(body)
	return strings.Contains(body, `"type": "service_account"`) ||
		strings.HasPrefix(trimmed, "-----BEGIN PRIVATE KEY-----")
}

// FileFromUntrustedPathRule detects File(parent, child) construction inside
// extract/upload/download-style functions where child is either a literal with
// parent traversal (`..`) or a non-literal path segment without an obvious
// canonical-path containment check.
type FileFromUntrustedPathRule struct {
	FlatDispatchBase
	BaseRule
}

// TempFileWorldReadableRule detects setReadable/setWritable/setExecutable(true,
// false) on a File that came from File.createTempFile or
// Files.createTempFile, which makes the temporary file world-accessible.
type TempFileWorldReadableRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TempFileWorldReadableRule) Confidence() float64 { return api.ConfidenceMedium }

var tempFilePermissionSetterNames = map[string]bool{
	"setReadable":   true,
	"setWritable":   true,
	"setExecutable": true,
}

func tempFileSetterAndReceiver(file *scanner.File, call uint32) (string, uint32, bool) {
	switch file.FlatType(call) {
	case "call_expression":
		if !tempFilePermissionSetterNames[flatCallExpressionName(file, call)] {
			return "", 0, false
		}
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return "", 0, false
		}
		receiver := flatReceiverNameFromCall(file, call)
		if receiver == "" {
			return "", 0, false
		}
		return receiver, args, true
	case "method_invocation":
		if !tempFilePermissionSetterNames[javaMethodInvocationName(file, call)] {
			return "", 0, false
		}
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return "", 0, false
		}
		receiver := javaMethodReceiverText(file, call)
		// Only accept a bare identifier receiver — chained receivers like
		// File.createTempFile(...).setReadable(...) wouldn't have a binding
		// to look up anyway, and we'd risk matching unrelated setReadable.
		if receiver == "" || strings.Contains(receiver, ".") {
			return "", 0, false
		}
		return receiver, args, true
	}
	return "", 0, false
}

func tempFileSetterArgIsLiteral(file *scanner.File, args uint32, index int, want string) bool {
	if args == 0 {
		return false
	}
	switch file.FlatType(args) {
	case "value_arguments":
		arg := flatPositionalValueArgument(file, args, index)
		if arg == 0 {
			return false
		}
		inner := arg
		if c := file.FlatNamedChild(arg, 0); c != 0 {
			inner = c
		}
		inner = flatUnwrapParenExpr(file, inner)
		return strings.TrimSpace(file.FlatNodeText(inner)) == want
	case "argument_list":
		var current int
		for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if current == index {
				return strings.TrimSpace(file.FlatNodeText(child)) == want
			}
			current++
		}
	}
	return false
}

func receiverBoundToCreateTempFile(file *scanner.File, call uint32, receiver string) bool {
	fn, ok := flatEnclosingCallable(file, call)
	if !ok {
		return false
	}
	targetRow := file.FlatRow(call)
	declType, identExtractor := "", (func(uint32) string)(nil)
	switch file.FlatType(fn) {
	case "function_declaration":
		declType = "property_declaration"
		identExtractor = func(decl uint32) string { return propertyDeclarationName(file, decl) }
	case "method_declaration":
		declType = "local_variable_declaration"
		identExtractor = func(decl uint32) string { return javaLocalDeclaratorName(file, decl) }
	default:
		return false
	}
	found := false
	file.FlatWalkNodes(fn, declType, func(decl uint32) {
		if found || file.FlatRow(decl) > targetRow {
			return
		}
		if identExtractor(decl) != receiver {
			return
		}
		if expressionContainsCreateTempFileCall(file, decl) {
			found = true
		}
	})
	return found
}

func javaLocalDeclaratorName(file *scanner.File, decl uint32) string {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "variable_declarator" {
			continue
		}
		if ident, ok := file.FlatFindChild(child, "identifier"); ok {
			return file.FlatNodeText(ident)
		}
	}
	return ""
}

func expressionContainsCreateTempFileCall(file *scanner.File, root uint32) bool {
	if root == 0 {
		return false
	}
	for _, nodeType := range [2]string{"call_expression", "method_invocation"} {
		found := false
		file.FlatWalkNodes(root, nodeType, func(call uint32) {
			if found {
				return
			}
			if isCreateTempFileCall(file, call) {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

func isCreateTempFileCall(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		if flatCallExpressionName(file, call) != "createTempFile" {
			return false
		}
		receiver := flatReceiverNameFromCall(file, call)
		return receiver == "File" || receiver == "Files"
	case "method_invocation":
		if javaMethodInvocationName(file, call) != "createTempFile" {
			return false
		}
		receiver := javaMethodReceiverText(file, call)
		return receiver == "File" || receiver == "Files"
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *FileFromUntrustedPathRule) Confidence() float64 { return api.ConfidenceMedium }

// ZipSlipUncheckedRule detects classic Zip-slip vulnerabilities where a zip
// extraction loop builds a destination File from a zip entry name without a
// subsequent canonical-path containment check.
type ZipSlipUncheckedRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *ZipSlipUncheckedRule) Confidence() float64 { return api.ConfidenceMedium }

var zipSlipAPIMarkers = []string{
	"ZipInputStream",
	"ZipFile",
	"JarFile",
	"JarInputStream",
	"ZipEntry",
}

func zipSlipChildArgIsEntryName(text string) bool {
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, "="); idx >= 0 {
		text = strings.TrimSpace(text[idx+1:])
	}
	return strings.HasSuffix(text, ".name")
}

func zipSlipFunctionMentionsZipAPI(file *scanner.File, fn uint32) bool {
	text := file.FlatNodeText(fn)
	for _, marker := range zipSlipAPIMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func zipSlipFunctionHasGuard(file *scanner.File, fn uint32) bool {
	text := file.FlatNodeText(fn)
	if strings.Contains(text, "canonicalPath.startsWith(") {
		return true
	}
	if strings.Contains(text, ".normalize()") && strings.Contains(text, ".startsWith(") {
		return true
	}
	if strings.Contains(text, ".toRealPath(") && strings.Contains(text, ".startsWith(") {
		return true
	}
	return false
}

func zipSlipInsideExtractionLoop(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "while_statement", "for_statement", "do_while_statement":
			return true
		case "lambda_literal":
			if call, found := flatEnclosingAncestor(file, cur, "call_expression"); found {
				switch flatCallExpressionName(file, call) {
				case "use", "forEach", "forEachIndexed", "onEach":
					return true
				}
			}
		}
	}
	return false
}

func isRiskyFileFromPathFunction(name string) bool {
	for _, fragment := range []string{"upload", "extract", "unzip", "download"} {
		if strings.Contains(name, fragment) {
			return true
		}
	}
	return false
}

func valueArgumentExpressionTextFlat(file *scanner.File, arg uint32) string {
	text := strings.TrimSpace(file.FlatNodeText(arg))
	if idx := strings.Index(text, "="); idx >= 0 {
		return strings.TrimSpace(text[idx+1:])
	}
	return text
}

func isStringLiteralExpr(text string) bool {
	return strings.HasPrefix(text, "\"") || strings.HasPrefix(text, "\"\"\"")
}

func hasCanonicalPathContainmentGuardFlat(file *scanner.File, fn uint32, parentExpr string) bool {
	if file == nil || parentExpr == "" {
		return false
	}
	fnText := file.FlatNodeText(fn)
	return strings.Contains(fnText, ".canonicalPath.startsWith(") &&
		strings.Contains(fnText, parentExpr+".canonicalPath") &&
		strings.Contains(fnText, "File.separator")
}
