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


// RethrowCaughtExceptionRule detects catch { throw e } where e is the caught variable.
type RethrowCaughtExceptionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *RethrowCaughtExceptionRule) Confidence() float64 { return 0.75 }


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


// ThrowingExceptionInMainRule detects throw in main function.
type ThrowingExceptionInMainRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ThrowingExceptionInMainRule) Confidence() float64 { return 0.75 }


// ErrorUsageWithThrowableRule detects error(throwable) calls.
type ErrorUsageWithThrowableRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *ErrorUsageWithThrowableRule) Confidence() float64 { return 0.75 }


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


func walkThrowExpressionsFlat(file *scanner.File, idx uint32, fn func(throwNode uint32)) {
	file.FlatWalkNodes(idx, "jump_expression", func(n uint32) {
		text := file.FlatNodeText(n)
		if strings.HasPrefix(text, "throw ") || text == "throw" {
			fn(n)
		}
	})
}
