package rules

import (
	"regexp"
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// LogPiiRule detects logger calls that interpolate or concatenate sensitive
// variable names into log messages.
type LogPiiRule struct {
	FlatDispatchBase
	BaseRule
	PiiNamePattern *regexp.Regexp
}

func (r *LogPiiRule) Confidence() float64 { return api.ConfidenceMedium }

var defaultLogPiiNamePattern = regexp.MustCompile(`(?i)(password|passwd|token|secret|apiKey|api_key|authHeader|authorization|ssn|pan|cvv|jwt|sessionId|cookie)`)

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
