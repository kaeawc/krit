package rules

// Pure helpers extracted from observability.go.
//
// Functions in this file have no dependency on tree-sitter ASTs
// (*scanner.File) or any other Krit infrastructure — they take and return
// strings/maps/slices. That makes them straightforward to unit-test in
// isolation; see observability_helpers_test.go.

import "strings"

// isLikelyLogReceiver reports whether receiver names a logger import we have
// classified for this file (alias-aware).
func isLikelyLogReceiver(receiver string, aliases map[string]string) bool {
	if receiver == "" {
		return false
	}
	_, ok := aliases[receiver]
	return ok
}

// isKnownLoggerTypeText reports whether text is the (possibly nullable) name of
// a known SLF4J/Logback/log4j/kotlin-logging logger type.
func isKnownLoggerTypeText(text string) bool {
	text = compactConditionText(strings.TrimSuffix(text, "?"))
	switch text {
	case "org.slf4j.Logger",
		"ch.qos.logback.classic.Logger",
		"org.apache.logging.log4j.Logger",
		"mu.KLogger",
		"io.github.oshai.kotlinlogging.KLogger":
		return true
	default:
		return false
	}
}

// conditionTextRequiresGuard reports whether the condition text guarantees
// that the matching log level is enabled when its branch is taken.
func conditionTextRequiresGuard(text, receiver, guardProperty string) bool {
	text = normalizeConditionText(text)
	if text == "" {
		return false
	}
	if conditionTextMatchesGuard(text, receiver, guardProperty) {
		return true
	}

	disjunctions := splitTopLevelLogicalOr(text)
	if len(disjunctions) > 1 {
		for _, clause := range disjunctions {
			if !conditionTextRequiresGuard(clause, receiver, guardProperty) {
				return false
			}
		}
		return true
	}

	clauses := splitTopLevelLogicalAnd(text)
	if len(clauses) > 1 {
		for _, clause := range clauses {
			if conditionTextRequiresGuard(clause, receiver, guardProperty) {
				return true
			}
		}
	}

	return false
}

// conditionTextPreventsGuard reports whether the condition text guarantees
// that the matching log level is disabled when its branch is taken (so a
// subsequent log call after an early-exit is safe).
func conditionTextPreventsGuard(text, receiver, guardProperty string) bool {
	text = normalizeConditionText(text)
	if text == "" {
		return false
	}
	if conditionTextMatchesNegatedGuard(text, receiver, guardProperty) {
		return true
	}

	conjunctions := splitTopLevelLogicalAnd(text)
	if len(conjunctions) > 1 {
		for _, clause := range conjunctions {
			if !conditionTextPreventsGuard(clause, receiver, guardProperty) {
				return false
			}
		}
		return true
	}

	clauses := splitTopLevelLogicalOr(text)
	if len(clauses) > 1 {
		for _, clause := range clauses {
			if conditionTextPreventsGuard(clause, receiver, guardProperty) {
				return true
			}
		}
	}

	return false
}

func conditionTextMatchesGuard(text, receiver, guardProperty string) bool {
	text = normalizeConditionText(text)
	for _, candidate := range logLevelGuardCandidates(receiver, guardProperty) {
		if conditionTextMatchesBooleanGuard(text, candidate) {
			return true
		}
	}
	return false
}

func conditionTextMatchesNegatedGuard(text, receiver, guardProperty string) bool {
	text = normalizeConditionText(text)
	if !strings.HasPrefix(text, "!") {
		for _, candidate := range logLevelGuardCandidates(receiver, guardProperty) {
			if conditionTextMatchesBooleanNegatedGuard(text, candidate) {
				return true
			}
		}
		return false
	}

	inner := normalizeConditionText(strings.TrimSpace(strings.TrimPrefix(text, "!")))
	return conditionTextMatchesGuard(inner, receiver, guardProperty)
}

// logLevelGuardCandidates returns the set of textual call/property forms that
// represent a "level enabled" check for the given receiver.
func logLevelGuardCandidates(receiver, guardProperty string) []string {
	if receiver == "" || guardProperty == "" {
		return nil
	}
	return []string{
		receiver + "." + guardProperty,
		receiver + "." + guardProperty + "()",
		receiver + "?." + guardProperty,
		receiver + "?." + guardProperty + "()",
	}
}

func conditionTextMatchesBooleanGuard(text, candidate string) bool {
	text = compactConditionText(text)
	for _, form := range []string{
		candidate,
		candidate + "==true",
		candidate + "!=false",
		"true==" + candidate,
		"false!=" + candidate,
	} {
		if text == form || strings.HasSuffix(text, "."+form) {
			return true
		}
	}
	return false
}

func conditionTextMatchesBooleanNegatedGuard(text, candidate string) bool {
	text = compactConditionText(text)
	for _, form := range []string{
		candidate + "==false",
		candidate + "!=true",
		"false==" + candidate,
		"true!=" + candidate,
	} {
		if text == form || strings.HasSuffix(text, "."+form) {
			return true
		}
	}
	return false
}

// splitTopLevelLogicalAnd splits text on top-level "&&" tokens (depth 0
// w.r.t. parentheses). It always returns at least one clause.
func splitTopLevelLogicalAnd(text string) []string {
	var clauses []string
	depth := 0
	start := 0

	for i := 0; i < len(text)-1; i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '&':
			if depth == 0 && text[i+1] == '&' {
				clauses = append(clauses, strings.TrimSpace(text[start:i]))
				start = i + 2
				i++
			}
		}
	}

	if len(clauses) == 0 {
		return []string{text}
	}

	clauses = append(clauses, strings.TrimSpace(text[start:]))
	return clauses
}

// splitTopLevelLogicalOr splits text on top-level "||" tokens.
func splitTopLevelLogicalOr(text string) []string {
	var clauses []string
	depth := 0
	start := 0

	for i := 0; i < len(text)-1; i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '|':
			if depth == 0 && text[i+1] == '|' {
				clauses = append(clauses, strings.TrimSpace(text[start:i]))
				start = i + 2
				i++
			}
		}
	}

	if len(clauses) == 0 {
		return []string{text}
	}

	clauses = append(clauses, strings.TrimSpace(text[start:]))
	return clauses
}

// logLevelGuardProperty returns the SLF4J-style "isXEnabled" property name for
// a log level. Returns "isDebugEnabled" for any level other than "trace".
func logLevelGuardProperty(level string) string {
	if level == "trace" {
		return "isTraceEnabled"
	}
	return "isDebugEnabled"
}

// normalizeConditionText trims whitespace and strips paired surrounding
// parentheses from a Kotlin condition's source text.
func normalizeConditionText(text string) string {
	text = strings.TrimSpace(text)
	for len(text) >= 2 && text[0] == '(' && text[len(text)-1] == ')' {
		text = strings.TrimSpace(text[1 : len(text)-1])
	}
	return text
}

// compactConditionText collapses whitespace inside a normalised condition.
func compactConditionText(text string) string {
	return strings.Join(strings.Fields(normalizeConditionText(text)), "")
}

// tracingSpanStartText reports whether a snippet of source text contains a
// `spanBuilder(...).startSpan(...)` chain (whitespace-insensitive).
func tracingSpanStartText(text string) bool {
	compact := strings.Join(strings.Fields(text), "")
	return strings.Contains(compact, "spanBuilder(") && strings.Contains(compact, ".startSpan(")
}

// highCardinalityKeySet builds a lookup set from a list of configured key
// strings, falling back to a default list when configured is empty. Empty
// strings are skipped.
func highCardinalityKeySet(configured []string) map[string]bool {
	if len(configured) == 0 {
		configured = []string{"user_id", "session_id", "trace_id"}
	}
	keys := make(map[string]bool, len(configured))
	for _, key := range configured {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			keys[trimmed] = true
		}
	}
	return keys
}

// metricNameHasUnitSuffix reports whether name ends with one of the configured
// unit suffixes; falls back to the canonical Prometheus suffixes when no
// suffixes are configured.
func metricNameHasUnitSuffix(name string, suffixes []string) bool {
	if len(suffixes) == 0 {
		suffixes = []string{"_total", "_seconds", "_bytes", "_count"}
	}
	for _, suffix := range suffixes {
		if suffix != "" && strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// throwableLikeInterpolation reports whether text contains a Kotlin string
// interpolation (e.g. `$e` or `${error}`) of a throwable-looking identifier.
func throwableLikeInterpolation(text string) bool {
	for _, match := range throwableLikeIdentifierRe.FindAllString(text, -1) {
		if strings.Contains(text, "$"+match) || strings.Contains(text, "${"+match) {
			return true
		}
	}
	return false
}

// interpolationHasObservedIdentifier reports whether the Kotlin string-template
// text contains an interpolation of any identifier in identifiers.
func interpolationHasObservedIdentifier(text string, identifiers map[string]bool) bool {
	for _, match := range interpolationIdentifierRe.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 && identifiers[normalizeObservabilityIdentifier(match[1])] {
			return true
		}
	}
	return false
}

// normalizeObservabilityIdentifier lower-cases an identifier and strips '_' and
// '-' separators so trace_id, traceId, and trace-id compare equal.
func normalizeObservabilityIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

// structuredLogKeyConvention returns "snake_case" or "camelCase" for a
// structured-log key, or "" when the key is mixed/invalid (uppercase + '_',
// non-letter start, contains punctuation other than '_').
func structuredLogKeyConvention(key string) string {
	if key == "" {
		return ""
	}
	hasUpper := false
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return ""
			}
		case r == '_':
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		default:
			return ""
		}
	}
	first := rune(key[0])
	if first < 'a' || first > 'z' {
		return ""
	}
	if strings.Contains(key, "_") && !hasUpper {
		return "snake_case"
	}
	if hasUpper && !strings.Contains(key, "_") {
		return "camelCase"
	}
	if !hasUpper && !strings.Contains(key, "_") {
		return "snake_case"
	}
	return ""
}
