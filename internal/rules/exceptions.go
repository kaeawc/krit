package rules

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ExceptionRaisedInUnexpectedLocationRule detects throw inside equals/hashCode/toString/finalize.
type ExceptionRaisedInUnexpectedLocationRule struct {
	FlatDispatchBase
	BaseRule
	MethodNames []string
}

var unexpectedThrowFunctions = map[string]bool{
	"equals":   true,
	"hashCode": true,
	"toString": true,
	"finalize": true,
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ExceptionRaisedInUnexpectedLocationRule) Confidence() float64 { return 0.75 }

func (r *ExceptionRaisedInUnexpectedLocationRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *ExceptionRaisedInUnexpectedLocationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := extractIdentifierFlat(file, idx)
	if !unexpectedThrowFunctions[name] {
		return nil
	}
	var findings []scanner.Finding
	walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
		findings = append(findings, r.Finding(file, file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
			fmt.Sprintf("Exception thrown inside '%s()'. This method should not throw exceptions.", name)))
	})
	return findings
}

func (r *ExceptionRaisedInUnexpectedLocationRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

// InstanceOfCheckForExceptionRule detects `is SomeException` inside catch blocks.
type InstanceOfCheckForExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

var isExceptionRe = regexp.MustCompile(`\bis\s+\w*Exception\w*`)

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *InstanceOfCheckForExceptionRule) Confidence() float64 { return 0.75 }

func (r *InstanceOfCheckForExceptionRule) NodeTypes() []string { return []string{"catch_block"} }

func (r *InstanceOfCheckForExceptionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	catchVarName := extractCaughtVarNameFlat(file, idx)
	if catchVarName == "" {
		return nil
	}
	skipWhenDispatch := experiment.Enabled("instance-of-check-skip-when-dispatch")

	var findings []scanner.Finding
	// tree-sitter Kotlin uses "check_expression" for `is` checks (e.g., `e is IOException`)
	for _, nodeType := range []string{"is_expression", "type_check", "check_expression"} {
		file.FlatWalkNodes(idx, nodeType, func(isNode uint32) {
			text := file.FlatNodeText(isNode)
			if !isExceptionRe.MatchString(text) {
				return
			}
			// Only flag if the LHS of the is-check is the caught variable directly
			// (not a property like e.cause). The LHS is the first child.
			if file.FlatChildCount(isNode) < 1 {
				return
			}
			lhs := file.FlatNodeText(file.FlatChild(isNode, 0))
			if strings.TrimSpace(lhs) != catchVarName {
				return
			}
			// Skip when the `is` check is inside `when (<catchVar>) { is X -> }`
			// — Kotlin's when-is dispatch on the caught variable is the
			// idiomatic way to handle related exception types with shared
			// handlers.
			if skipWhenDispatch && isInsideWhenDispatchOnCatchVarFlat(file, isNode, catchVarName) {
				return
			}
			findings = append(findings, r.Finding(file, file.FlatRow(isNode)+1, file.FlatCol(isNode)+1,
				"Instance-of check for exception type inside catch block. Use specific catch clauses instead."))
		})
	}
	return findings
}

func isInsideWhenDispatchOnCatchVarFlat(file *scanner.File, isNode uint32, caughtVar string) bool {
	for p, ok := file.FlatParent(isNode); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "when_expression" {
			for i := 0; i < file.FlatChildCount(p); i++ {
				c := file.FlatChild(p, i)
				ctype := file.FlatType(c)
				if ctype == "when_subject" || ctype == "parenthesized_expression" ||
					ctype == "value_arguments" {
					text := strings.TrimSpace(file.FlatNodeText(c))
					text = strings.TrimPrefix(text, "(")
					text = strings.TrimSuffix(text, ")")
					text = strings.TrimSpace(text)
					if text == caughtVar {
						return true
					}
					return false
				}
			}
			return false
		}
		if t == "function_declaration" || t == "lambda_literal" || t == "source_file" {
			return false
		}
	}
	return false
}

// NotImplementedDeclarationRule detects TODO() calls.
type NotImplementedDeclarationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *NotImplementedDeclarationRule) Confidence() float64 { return 0.75 }

func (r *NotImplementedDeclarationRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *NotImplementedDeclarationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "TODO" {
		return nil
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"TODO() call found. Replace with an actual implementation.")}
}

// RethrowCaughtExceptionRule detects catch { throw e } where e is the caught variable.
type RethrowCaughtExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *RethrowCaughtExceptionRule) Confidence() float64 { return 0.75 }

func (r *RethrowCaughtExceptionRule) NodeTypes() []string { return []string{"catch_block"} }

func (r *RethrowCaughtExceptionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	caughtVar := extractCaughtVarNameFlat(file, idx)
	if caughtVar == "" {
		return nil
	}
	// Find the catch body (statements_block)
	body := file.FlatFindChild(idx, "statements")
	if body == 0 {
		return nil
	}
	// Check if the only statement is throw <caughtVar>
	stmtCount := 0
	var onlyThrow uint32
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		if file.FlatType(child) == "jump_expression" && strings.HasPrefix(file.FlatNodeText(child), "throw") {
			onlyThrow = child
			stmtCount++
		} else if t := file.FlatType(child); t != "line_comment" && t != "multiline_comment" && t != "{" && t != "}" {
			stmtCount++
		}
	}
	if stmtCount == 1 && onlyThrow != 0 {
		throwText := strings.TrimSpace(file.FlatNodeText(onlyThrow))
		if throwText == "throw "+caughtVar || throwText == "throw "+caughtVar+";" {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				fmt.Sprintf("Caught exception '%s' is immediately rethrown. Remove the catch block or add handling logic.", caughtVar))}
		}
	}
	return nil
}

// ReturnFromFinallyRule detects return statements inside finally blocks.
type ReturnFromFinallyRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreLabeled bool
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ReturnFromFinallyRule) Confidence() float64 { return 0.75 }

func (r *ReturnFromFinallyRule) NodeTypes() []string { return []string{"finally_block"} }

func (r *ReturnFromFinallyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	file.FlatWalkNodes(idx, "jump_expression", func(jumpNode uint32) {
		text := file.FlatNodeText(jumpNode)
		if strings.HasPrefix(text, "return") {
			f := r.Finding(file, file.FlatRow(jumpNode)+1, file.FlatCol(jumpNode)+1,
				"Return from finally block. This can swallow exceptions from try/catch.")
			// Fix: remove the return statement line (byte-mode for precision)
			lineIdx := file.FlatRow(jumpNode)
			lineStart := file.LineOffset(lineIdx)
			lineEnd := lineStart + len(file.Lines[lineIdx]) + 1
			if lineEnd > len(file.Content) {
				lineEnd = len(file.Content)
			}
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   lineStart,
				EndByte:     lineEnd,
				Replacement: "",
			}
			findings = append(findings, f)
		}
	})
	return findings
}

// SwallowedExceptionRule detects catch blocks that either never use the exception
// variable or that throw a new exception without passing the original as the cause.
// Matches detekt's SwallowedException semantics: referencing only e.message or
// e.toString() (directly or via a variable) in a throw counts as swallowed.
// swallowedExceptionLogCallRe matches any `Log.verb(...)` call — used to
// detect catch blocks that handle the exception via logging even without
// passing the caught variable.
var swallowedExceptionLogCallRe = regexp.MustCompile(`\bLog\.[vdiwef]\s*\(`)

// swallowedExceptionBroadLogCallRe expands the logging detection to
// common patterns beyond AOSP `Log.*`:
//   - top-level logging functions like `warn(...)`, `error(...)`, `info(...)`,
//     `debug(...)`, `trace(...)`, `log(...)`, `logError(...)`, `logWarning(...)`
//   - `Timber.verb(...)`, `Slf4j.verb(...)`
//   - instance-style: `logger.warn(...)`, `log.error(...)`, `this.logger.info(...)`
var swallowedExceptionBroadLogCallRe = regexp.MustCompile(`(?:^|[^a-zA-Z0-9_.])(?:warn|error|info|debug|trace|log|logError|logWarning|logWarn|logInfo|logDebug|logTrace|logger\.(?:warn|error|info|debug|trace)|log\.(?:warn|error|info|debug|trace)|Timber\.[vdiwef])\s*\(`)

// swallowedExceptionUICallRe matches UI notification APIs that communicate
// errors to the user. Catch blocks using these are handling the exception.
var swallowedExceptionUICallRe = regexp.MustCompile(`\b(Toast\.makeText|Snackbar\.make|AlertDialog|MaterialAlertDialog|showError|showDialog)\b`)

// swallowedExceptionAssignmentRe matches assignment statements in the catch
// body — assigning to state is handling. Includes compound assignments
// (+=, -=, *=, /=, %=) which mutate collections/accumulators as a fallback.
var swallowedExceptionAssignmentRe = regexp.MustCompile(`\w+\s*(=|\+=|-=|\*=|/=|%=)\s*\S`)

// swallowedExceptionHandlerCallRe matches invocation of a handler-style
// function whose name clearly indicates error recovery — the exception
// variable may not be referenced but the handler routine is handling.
var swallowedExceptionHandlerCallRe = regexp.MustCompile(`\b(?:toastOn|showError|showErrorDialog|handleError|reportError|recoverFrom|onError|fallback|notifyError)\w*\s*\(`)

// stripCommentsFromBody removes // line comments and /* */ block comments
// from a code snippet. Used to check if a catch body is comment-only.
func stripCommentsFromBody(s string) string {
	// Remove /* ... */ block comments
	for {
		start := strings.Index(s, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "*/")
		if end < 0 {
			break
		}
		s = s[:start] + s[start+end+2:]
	}
	// Remove // line comments
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

type SwallowedExceptionRule struct {
	FlatDispatchBase
	BaseRule
	AllowedExceptionNameRegex *regexp.Regexp // exception names matching this are allowed to be swallowed
	IgnoredExceptionTypes     []string       // exception types that are allowed to be swallowed
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *SwallowedExceptionRule) Confidence() float64 { return 0.75 }

func (r *SwallowedExceptionRule) NodeTypes() []string { return []string{"catch_block"} }

func (r *SwallowedExceptionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if isTestFile(file.Path) {
		return nil
	}
	caughtVar := extractCaughtVarNameFlat(file, idx)
	if caughtVar == "" || caughtVar == "_" {
		return nil
	}
	// Skip if the exception variable name matches the allowed regex
	if r.AllowedExceptionNameRegex != nil && r.AllowedExceptionNameRegex.MatchString(caughtVar) {
		return nil
	}
	// Skip if the caught exception type is in the ignored list (case-insensitive substring match like detekt)
	if len(r.IgnoredExceptionTypes) > 0 {
		caughtType := extractCaughtTypeNameFlat(file, idx)
		if caughtType != "" {
			lowerType := strings.ToLower(caughtType)
			for _, ignored := range r.IgnoredExceptionTypes {
				if strings.Contains(lowerType, strings.ToLower(ignored)) {
					return nil
				}
			}
		}
	}
	catchText := file.FlatNodeText(idx)
	// Remove the catch(...) header to get only the body
	bodyStart := strings.Index(catchText, "{")
	if bodyStart < 0 {
		return nil
	}
	body := catchText[bodyStart+1:]
	// Dedup with EmptyCatchBlock: if the body is literally empty, the
	// other rule already reports this and will offer a fix. Don't
	// double-report the same issue.
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), "}"))
	if inner == "" {
		return nil
	}

	// If the catch is part of a try expression (value is used), the specific
	// exception type itself IS the handling — the catch branch returns a
	// semantic fallback value based on which exception was thrown.
	if isCatchPartOfTryExpressionFlat(file, idx) {
		return nil
	}
	// EOFException is the standard end-of-stream sentinel — catching it to
	// return null/break/continue is idiomatic.
	caughtType := extractCaughtTypeNameFlat(file, idx)
	if caughtType == "EOFException" {
		return nil
	}
	// Check if the caught variable is referenced in the body at all
	varRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(caughtVar) + `\b`)
	if !varRe.MatchString(body) {
		// If the catch body contains a comment (single-line `//` or block `/* */`)
		// explaining why the exception is swallowed, treat as intentional.
		bodyStripped := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(body, "{"), "}"))
		if strings.HasPrefix(bodyStripped, "//") || strings.HasPrefix(bodyStripped, "/*") {
			// Body starts with a comment — likely intentional swallow.
			// Also check if the ONLY content after stripping comments is empty
			// (i.e., comment-only body).
			noComments := stripCommentsFromBody(bodyStripped)
			if strings.TrimSpace(noComments) == "" {
				return nil
			}
		}
		// If the catch body contains a Log.* call, treat the logging as
		// handling even if `e` isn't passed — many projects intentionally
		// log a static message without the stack trace.
		if swallowedExceptionLogCallRe.MatchString(body) {
			return nil
		}
		// Opt-in broader logging patterns: top-level warn()/error() helpers,
		// Timber, slf4j loggers, etc. Many projects use a static logger
		// object imported at the top of the file.
		if experiment.Enabled("swallowed-exception-broader-logging") &&
			swallowedExceptionBroadLogCallRe.MatchString(body) {
			return nil
		}
		// UI notification APIs (Toast, Snackbar, AlertDialog) count as
		// handling — the exception is communicated to the user.
		if swallowedExceptionUICallRe.MatchString(body) {
			return nil
		}
		// Catch body that is a `return` with a value (including `return null`)
		// is handling — the exception is being translated to a domain value.
		trimmedBody := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(body, ""), "}"))
		if strings.HasPrefix(trimmedBody, "return ") ||
			strings.HasPrefix(trimmedBody, "return\n") ||
			trimmedBody == "return" {
			return nil
		}
		// Catch body that assigns to a variable counts as state handling.
		if swallowedExceptionAssignmentRe.MatchString(body) {
			return nil
		}
		// Catch body that calls a handler-like function with names such as
		// `toastOn*`, `showError`, `handle*`, `report*`, `recoverFrom*` —
		// the exception is handled via a named recovery routine even if
		// `e` isn't passed explicitly.
		if swallowedExceptionHandlerCallRe.MatchString(body) {
			return nil
		}
		// Exception is completely unused
		return r.makeUnusedFindingFlat(idx, file, caughtVar)
	}

	// Exception is referenced — check if it's swallowed in throw expressions.
	// "Swallowed" means every throw that references the variable only uses
	// e.message, e.toString(), e.localizedMessage (or variable aliases of those)
	// without passing the exception itself as a constructor argument.
	if isExceptionSwallowedInThrows(body, caughtVar) {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Exception '%s' is caught but not passed as a cause. Pass it directly to preserve the stack trace.", caughtVar))
		return []scanner.Finding{f}
	}

	return nil
}

func (r *SwallowedExceptionRule) makeUnusedFindingFlat(idx uint32, file *scanner.File, caughtVar string) []scanner.Finding {
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Exception '%s' is caught but never used. Either log/handle it or rethrow.", caughtVar))
	endByte := int(file.FlatEndByte(idx))
	if endByte > 0 && file.Content[endByte-1] == '}' {
		catchLine := file.Lines[file.FlatRow(idx)]
		indent := ""
		for _, ch := range catchLine {
			if ch == ' ' || ch == '\t' {
				indent += string(ch)
			} else {
				break
			}
		}
		insertion := indent + "    throw " + caughtVar + "\n" + indent
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   endByte - 1,
			EndByte:     endByte,
			Replacement: insertion + "}",
		}
	}
	return []scanner.Finding{f}
}

// isExceptionSwallowedInThrows checks whether there is at least one throw
// expression that references the caught variable but only via property/method
// access (e.g. e.message, e.toString()) without passing e directly as a cause.
// A throw that does NOT reference the variable at all is ignored — the
// exception may be used elsewhere (e.g. logging).  This matches detekt's
// isExceptionSwallowed semantics.
func isExceptionSwallowedInThrows(body, caughtVar string) bool {
	quotedVar := regexp.QuoteMeta(caughtVar)
	throwRe := regexp.MustCompile(`throw\s+\w+\s*\(`)
	dotAccessRe := regexp.MustCompile(`\b` + quotedVar + `\.(message|toString\(\)|localizedMessage|stackTraceToString\(\)|cause)`)
	directRefRe := regexp.MustCompile(`\b` + quotedVar + `\b`)
	directNoDotRe := regexp.MustCompile(`\b` + quotedVar + `(?:\s*[,)\s])`)

	throwLocs := throwRe.FindAllStringIndex(body, -1)
	if len(throwLocs) == 0 {
		return false
	}

	// Build alias maps: val x = e.message  ->  string alias (swallowed)
	//                    val x = e          ->  direct alias (not swallowed)
	aliasRe := regexp.MustCompile(`val\s+(\w+)\s*=\s*` + quotedVar + `\.(message|toString\(\)|localizedMessage|stackTraceToString\(\))`)
	stringAliases := make(map[string]bool)
	for _, m := range aliasRe.FindAllStringSubmatch(body, -1) {
		stringAliases[m[1]] = true
	}
	directAliasRe := regexp.MustCompile(`val\s+(\w+)\s*=\s*` + quotedVar + `\s*[\n;})]`)
	directAliases := make(map[string]bool)
	for _, m := range directAliasRe.FindAllStringSubmatch(body, -1) {
		directAliases[m[1]] = true
	}

	for _, loc := range throwLocs {
		throwStart := loc[0]
		throwExpr := extractBalancedParens(body[loc[1]-1:])
		if throwExpr == "" {
			continue
		}
		fullThrow := body[throwStart : loc[1]-1+len(throwExpr)]

		// Only inspect throws that actually reference the caught variable
		// (directly or via an alias).
		if !directRefRe.MatchString(fullThrow) {
			// Check aliases
			hasStringAlias := false
			for alias := range stringAliases {
				if regexp.MustCompile(`\b` + regexp.QuoteMeta(alias) + `\b`).MatchString(fullThrow) {
					hasStringAlias = true
					break
				}
			}
			for alias := range directAliases {
				if regexp.MustCompile(`\b` + regexp.QuoteMeta(alias) + `\b`).MatchString(fullThrow) {
					// Direct alias used — exception forwarded properly
					return false
				}
			}
			if !hasStringAlias {
				// Throw doesn't reference the exception at all — skip it
				continue
			}
			// Only string aliases used in this throw — swallowed
			return true
		}

		// Variable is referenced directly in the throw.
		// Strip dot-access usages and check if a bare reference remains.
		stripped := dotAccessRe.ReplaceAllString(fullThrow, "")
		if directNoDotRe.MatchString(stripped) {
			// Exception is passed directly as a constructor argument — not swallowed
			return false
		}
		// Only dot-access references (e.message, etc.) — swallowed
		return true
	}

	return false
}

// extractBalancedParens extracts text from the opening paren to its matching close.
func extractBalancedParens(s string) string {
	if len(s) == 0 || s[0] != '(' {
		return ""
	}
	depth := 0
	for i, ch := range s {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return ""
}

// ThrowingExceptionFromFinallyRule detects throw inside finally blocks.
type ThrowingExceptionFromFinallyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionFromFinallyRule) Confidence() float64 { return 0.75 }

func (r *ThrowingExceptionFromFinallyRule) NodeTypes() []string { return []string{"finally_block"} }

func (r *ThrowingExceptionFromFinallyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding
	walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
		f := r.Finding(file, file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
			"Exception thrown inside finally block. This can swallow exceptions from try/catch.")
		// Fix: delete the throw statement line (byte-mode for precision)
		lineIdx := file.FlatRow(throwNode)
		lineStart := file.LineOffset(lineIdx)
		lineEnd := lineStart + len(file.Lines[lineIdx]) + 1
		if lineEnd > len(file.Content) {
			lineEnd = len(file.Content)
		}
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   lineStart,
			EndByte:     lineEnd,
			Replacement: "",
		}
		findings = append(findings, f)
	})
	return findings
}

// ThrowingExceptionsWithoutMessageOrCauseRule detects throw Exception() with no args.
// Uses tree-sitter dispatch to find call_expression nodes, then checks if the parent
// is a throw (jump_expression) and the exception type is in the configured allowlist.
type ThrowingExceptionsWithoutMessageOrCauseRule struct {
	FlatDispatchBase
	BaseRule
	Exceptions    []string
	allowlistOnce sync.Once
	allowlist     map[string]bool
}

// Default exception types that should have a message (matches detekt defaults)
var defaultExceptionsRequiringMessage = map[string]bool{
	"ArrayIndexOutOfBoundsException": true,
	"Exception":                      true,
	"IllegalArgumentException":       true,
	"IllegalMonitorStateException":   true,
	"IllegalStateException":          true,
	"IndexOutOfBoundsException":      true,
	"NullPointerException":           true,
	"RuntimeException":               true,
	"Throwable":                      true,
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionsWithoutMessageOrCauseRule) Confidence() float64 { return 0.75 }

func (r *ThrowingExceptionsWithoutMessageOrCauseRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ThrowingExceptionsWithoutMessageOrCauseRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Must be inside a throw — check parent or previous sibling for jump_expression
	isThrow := false
	parent, ok := file.FlatParent(idx)
	if ok {
		if file.FlatType(parent) == "jump_expression" {
			text := file.FlatNodeText(parent)
			isThrow = strings.HasPrefix(strings.TrimSpace(text), "throw")
		}
		// Tree-sitter may also put throw as a sibling in statements.
		// Walk backwards from idx — linear in the nearest-preceding
		// distance rather than quadratic in sibling index.
		if file.FlatType(parent) == "statements" {
			for prev, ok := file.FlatPrevSibling(idx); ok; prev, ok = file.FlatPrevSibling(prev) {
				if file.FlatType(prev) != "jump_expression" {
					continue
				}
				text := file.FlatNodeText(prev)
				if strings.HasPrefix(strings.TrimSpace(text), "throw") {
					isThrow = true
					break
				}
			}
		}
	}
	if !isThrow {
		return nil
	}

	// Get the exception type name (first child = simple_identifier or navigation_expression)
	exName := ""
	if file.FlatChildCount(idx) > 0 {
		first := file.FlatChild(idx, 0)
		if file.FlatType(first) == "simple_identifier" {
			exName = file.FlatNodeText(first)
		}
	}
	if exName == "" {
		return nil
	}

	// Check against allowlist
	exceptionSet := r.exceptionAllowlist()
	if !experiment.Enabled("exceptions-allowlist-cache") && len(r.Exceptions) > 0 {
		exceptionSet = make(map[string]bool, len(r.Exceptions))
		for _, e := range r.Exceptions {
			exceptionSet[e] = true
		}
	}
	if !exceptionSet[exName] {
		return nil
	}

	// Check if the call has no arguments
	argCount := throwingExceptionArgCountFlat(file, idx)
	if argCount < 0 {
		return nil
	}
	if argCount > 0 {
		return nil // has arguments — ok
	}

	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		fmt.Sprintf("Exception '%s' thrown without a message or cause. Provide a descriptive message.", exName))}
}

func (r *ThrowingExceptionsWithoutMessageOrCauseRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

func (r *ThrowingExceptionsWithoutMessageOrCauseRule) exceptionAllowlist() map[string]bool {
	if !experiment.Enabled("exceptions-allowlist-cache") || len(r.Exceptions) == 0 {
		if len(r.Exceptions) == 0 {
			return defaultExceptionsRequiringMessage
		}
		set := make(map[string]bool, len(r.Exceptions))
		for _, e := range r.Exceptions {
			set[e] = true
		}
		return set
	}
	r.allowlistOnce.Do(func() {
		r.allowlist = make(map[string]bool, len(r.Exceptions))
		for _, e := range r.Exceptions {
			r.allowlist[e] = true
		}
	})
	return r.allowlist
}

func throwingExceptionArgCountFlat(file *scanner.File, idx uint32) int {
	if !experiment.Enabled("exceptions-throw-fastpath") {
		callSuffix := file.FlatFindChild(idx, "call_suffix")
		if callSuffix == 0 {
			return -1
		}
		valueArgs := file.FlatFindChild(callSuffix, "value_arguments")
		if valueArgs == 0 {
			return -1
		}
		argCount := 0
		for i := 0; i < file.FlatChildCount(valueArgs); i++ {
			if file.FlatType(file.FlatChild(valueArgs, i)) == "value_argument" {
				argCount++
			}
		}
		return argCount
	}
	callSuffix := file.FlatFindChild(idx, "call_suffix")
	if callSuffix == 0 {
		return -1
	}
	argList := file.FlatFindChild(callSuffix, "value_arguments")
	if argList == 0 {
		return -1
	}
	return file.FlatNamedChildCount(argList)
}

// ThrowingNewInstanceOfSameExceptionRule detects catch(e: X) { throw X(e) }.
type ThrowingNewInstanceOfSameExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingNewInstanceOfSameExceptionRule) Confidence() float64 { return 0.75 }

func (r *ThrowingNewInstanceOfSameExceptionRule) NodeTypes() []string { return []string{"catch_block"} }

func (r *ThrowingNewInstanceOfSameExceptionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	caughtType := extractCaughtTypeNameFlat(file, idx)
	if caughtType == "" {
		return nil
	}
	// Extract the caught variable name so we can detect the enrichment pattern
	// `throw X(..., caughtVar, ...)` which is a legitimate rethrow that adds
	// context.
	caughtVar := ""
	for i := 0; i < file.FlatChildCount(idx); i++ {
		c := file.FlatChild(idx, i)
		if file.FlatType(c) == "simple_identifier" {
			caughtVar = file.FlatNodeText(c)
			break
		}
	}
	var findings []scanner.Finding
	walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
		throwText := file.FlatNodeText(throwNode)
		// Check for throw SameType(...)
		pattern := fmt.Sprintf(`throw\s+%s\s*\(`, regexp.QuoteMeta(caughtType))
		matched, err := regexp.MatchString(pattern, throwText)
		if err != nil {
			matched = strings.Contains(throwText, "throw "+caughtType+"(")
		}
		if !matched {
			return
		}
		// Skip when the new instance is created with the caught variable PLUS
		// additional arguments (enrichment pattern: wrap original as cause and
		// add a descriptive message). A bare `throw X(e)` with only the caught
		// var is still a pointless rethrow — it should just be `throw e`.
		if caughtVar != "" {
			parenIdx := strings.Index(throwText, "(")
			if parenIdx >= 0 {
				argsText := strings.TrimSpace(throwText[parenIdx+1:])
				// Find matching closing paren position
				closeIdx := strings.LastIndex(argsText, ")")
				if closeIdx >= 0 {
					argsText = strings.TrimSpace(argsText[:closeIdx])
					// Strip simple comma-separated args and check contents.
					// If the args contain both the caught var AND at least one
					// other non-trivial arg (comma-separated), skip.
					if strings.Contains(argsText, ",") {
						argPattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(caughtVar))
						if m, _ := regexp.MatchString(argPattern, argsText); m {
							return
						}
					}
				}
			}
		}
		findings = append(findings, r.Finding(file, file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
			fmt.Sprintf("New instance of '%s' thrown inside catch block that already catches it. Simply rethrow the original.", caughtType)))
	})
	return findings
}

func (r *ThrowingNewInstanceOfSameExceptionRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

// ThrowingExceptionInMainRule detects throw in main function.
type ThrowingExceptionInMainRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionInMainRule) Confidence() float64 { return 0.75 }

func (r *ThrowingExceptionInMainRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *ThrowingExceptionInMainRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := extractIdentifierFlat(file, idx)
	if name != "main" {
		return nil
	}
	var findings []scanner.Finding
	walkThrowExpressionsFlat(file, idx, func(throwNode uint32) {
		findings = append(findings, r.Finding(file, file.FlatRow(throwNode)+1, file.FlatCol(throwNode)+1,
			"Exception thrown in main(). Handle exceptions gracefully instead."))
	})
	return findings
}

// ErrorUsageWithThrowableRule detects error(throwable) calls.
type ErrorUsageWithThrowableRule struct {
	FlatDispatchBase
	BaseRule
}

var errorThrowableRe = regexp.MustCompile(`\berror\s*\(\s*\w+\s*\)`)

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ErrorUsageWithThrowableRule) Confidence() float64 { return 0.75 }

func (r *ErrorUsageWithThrowableRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ErrorUsageWithThrowableRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.HasPrefix(text, "error(") {
		return nil
	}
	// Check if the argument variable name suggests a throwable
	argText := text[6 : len(text)-1] // strip error( and )
	argText = strings.TrimSpace(argText)
	lower := strings.ToLower(argText)
	if strings.Contains(lower, "exception") || strings.Contains(lower, "throwable") ||
		strings.Contains(lower, "error") || lower == "e" || lower == "ex" || lower == "err" || lower == "t" {
		// Make sure the argument is not a string literal
		if !strings.HasPrefix(argText, "\"") {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("error(%s) passes a Throwable. Use throw instead, or pass the message string.", argText))}
		}
	}
	return nil
}

// ObjectExtendsThrowableRule detects object : Exception/Throwable/Error.
type ObjectExtendsThrowableRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *ObjectExtendsThrowableRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — relies on
// resolver to determine supertypes; falls back to name-based heuristics on
// the `Throwable` identifier. Classified per roadmap/17.
func (r *ObjectExtendsThrowableRule) Confidence() float64 { return 0.75 }

var throwableBaseTypes = []string{"Throwable", "Exception", "Error", "RuntimeException",
	"IllegalStateException", "IllegalArgumentException", "IOException",
	"UnsupportedOperationException"}

func (r *ObjectExtendsThrowableRule) NodeTypes() []string { return []string{"object_declaration"} }

func (r *ObjectExtendsThrowableRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := extractIdentifierFlat(file, idx)

	// With resolver: use ClassHierarchy to verify the object's supertype is actually Throwable/Exception
	if r.resolver != nil {
		info := r.resolver.ClassHierarchy(name)
		if info != nil {
			throwableSet := map[string]bool{
				"Throwable": true, "Exception": true, "Error": true, "RuntimeException": true,
				"kotlin.Throwable": true, "java.lang.Throwable": true,
				"java.lang.Exception": true, "java.lang.Error": true,
				"java.lang.RuntimeException": true,
			}
			for _, st := range info.Supertypes {
				simpleParts := strings.Split(st, ".")
				simpleName := simpleParts[len(simpleParts)-1]
				if throwableSet[st] || throwableSet[simpleName] {
					return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Object '%s' extends '%s'. Objects that extend Throwable are singletons and lose stack trace information.", name, simpleName))}
				}
			}
			// Hierarchy known but no Throwable supertype — not a match
			return nil
		}
		// Hierarchy not known — fall through to text heuristic
	}

	// Fallback: text-based heuristic
	text := file.FlatNodeText(idx)
	for _, t := range throwableBaseTypes {
		if strings.Contains(text, ": "+t) || strings.Contains(text, ":"+t+"(") || strings.Contains(text, ": "+t+"(") {
			return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
				fmt.Sprintf("Object '%s' extends '%s'. Objects that extend Throwable are singletons and lose stack trace information.", name, t))}
		}
	}
	return nil
}

func walkThrowExpressionsFlat(file *scanner.File, idx uint32, fn func(throwNode uint32)) {
	file.FlatWalkNodes(idx, "jump_expression", func(n uint32) {
		text := file.FlatNodeText(n)
		if strings.HasPrefix(text, "throw ") || text == "throw" {
			fn(n)
		}
	})
}
