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

// defaultLogPiiNamePattern matches whole snake_case words. Identifiers are
// split into words before matching (see logPiiNameMatches) so that `pan` does
// not match `expandedState`, `company`, or `span`.
var defaultLogPiiNamePattern = regexp.MustCompile(`(?i)(^|_)(passwords?|passwd|tokens?|secrets?|api_?keys?|auth_?headers?|authorization|ssn|pan|cvv|jwt|session_?ids?|cookies?)(_|$)`)

var logPiiCamelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])|([A-Z]+)([A-Z][a-z])`)

// logPiiNameMatches reports whether name matches pattern either as written or
// after splitting camelCase into snake_case words.
func logPiiNameMatches(pattern *regexp.Regexp, name string) bool {
	if pattern.MatchString(name) {
		return true
	}
	snake := logPiiCamelBoundary.ReplaceAllString(name, "${1}${3}_${2}${4}")
	return snake != name && pattern.MatchString(snake)
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
	args := logPiiArgumentExpressions(file, call)
	for i, arg := range args {
		if logPiiExpressionMentionsSensitiveIdentifier(file, arg, pattern) {
			return arg
		}
		// The first argument of a multi-argument call is usually the log tag.
		if i > 0 || len(args) == 1 {
			if logPiiIsBareSensitiveReference(file, arg, pattern) {
				return arg
			}
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

var (
	logPiiSimpleInterpolation = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	logPiiBraceInterpolation  = regexp.MustCompile(`\$\{([^}]*)\}`)
	logPiiIdentifier          = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	logPiiPropertyChain       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\??\.[A-Za-z_][A-Za-z0-9_]*)+$`)
)

// logPiiInterpolatedNames returns every identifier read by a `$name` or
// `${expr}` template entry, including selectors such as `user.password`.
func logPiiInterpolatedNames(text string) []string {
	var names []string
	for _, m := range logPiiSimpleInterpolation.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	for _, m := range logPiiBraceInterpolation.FindAllStringSubmatch(text, -1) {
		names = append(names, logPiiIdentifier.FindAllString(m[1], -1)...)
	}
	return names
}

// logPiiLoggedNames is logPiiInterpolatedNames, except that a plain property chain
// such as `${tokenResult.errorMessage}` contributes only its last name, the value logged.
func logPiiLoggedNames(text string) []string {
	var names []string
	for _, m := range logPiiSimpleInterpolation.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	for _, m := range logPiiBraceInterpolation.FindAllStringSubmatch(text, -1) {
		ids := logPiiIdentifier.FindAllString(m[1], -1)
		if logPiiPropertyChain.MatchString(strings.TrimSpace(m[1])) && len(ids) > 1 {
			ids = ids[len(ids)-1:]
		}
		names = append(names, ids...)
	}
	return names
}

func logPiiExpressionMentionsSensitiveIdentifier(file *scanner.File, expr uint32, pattern *regexp.Regexp) bool {
	if file == nil || expr == 0 || pattern == nil {
		return false
	}
	text := file.FlatNodeText(expr)
	if flatContainsStringInterpolation(file, expr) {
		for _, name := range logPiiLoggedNames(text) {
			if logPiiNameMatches(pattern, name) {
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
			if logPiiNameMatches(pattern, name) {
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
	if strings.HasPrefix(trimmed, "-----BEGIN PRIVATE KEY-----") {
		return gcpPrivateKeyHasMaterial(strings.TrimPrefix(trimmed, "-----BEGIN PRIVATE KEY-----"))
	}
	// Only a JSON object counts; prose that quotes the type does not.
	return strings.HasPrefix(trimmed, "{") && strings.Contains(body, `"type": "service_account"`)
}

// gcpPrivateKeyHasMaterial reports a PEM body with key bytes after the header,
// so a bare `-----BEGIN PRIVATE KEY-----` delimiter constant is not a leaked key.
func gcpPrivateKeyHasMaterial(rest string) bool {
	rest = strings.ReplaceAll(rest, `\n`, "")
	if end := strings.Index(rest, "-----END"); end >= 0 {
		rest = rest[:end]
	}
	count := 0
	for _, r := range rest {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '/' || r == '=' {
			count++
		}
	}
	return count >= 40
}

// logPiiIsBareSensitiveReference reports a message argument that is only a
// variable or property reference such as `password` or `user.authToken`.
func logPiiIsBareSensitiveReference(file *scanner.File, expr uint32, pattern *regexp.Regexp) bool {
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		return logPiiNameMatches(pattern, file.FlatNodeText(expr))
	case "navigation_expression":
		segs := flatNavigationChainIdentifiers(file, expr)
		return len(segs) > 0 && logPiiNameMatches(pattern, segs[len(segs)-1])
	}
	return false
}
