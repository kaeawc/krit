package rules

import (
	"bytes"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// DeprecationRule detects usage of deprecated elements.
// It checks:
//  1. Oracle annotations (kotlin.Deprecated / java.lang.Deprecated) on call targets
//  2. Source-level @Deprecated annotations on function/class declarations
//
// Limitations vs detekt (which uses KaFirDiagnostic.Deprecation):
//   - Cannot detect @Deprecated on transitive library APIs not in the oracle's scope
//   - Cannot detect deprecation level (WARNING/ERROR/HIDDEN) from library metadata
//
// ---------------------------------------------------------------------------
// deprecationInfo stores extracted @Deprecated annotation details.
type deprecationInfo struct {
	message     string // @Deprecated("message")
	replaceWith string // @Deprecated(replaceWith = ReplaceWith("expr"))
	level       string // WARNING, ERROR, or HIDDEN
}

type DeprecationRule struct {
	FlatDispatchBase
	BaseRule
	ExcludeImportStatements bool
	// per-file cache of deprecated declaration info. cacheMu guards
	// concurrent access from parallel file-scan goroutines which all
	// share the same DeprecationRule instance.
	cacheMu               sync.Mutex
	cachedFile            string
	deprecatedInfos       map[string]*deprecationInfo
	deprecatedInfosByFile map[string]map[string]*deprecationInfo
}

// Confidence reports a tier-2 (medium) base confidence — matches on
// deprecation markers and annotations via pattern, with resolver-backed
// type checks used only when available. Classified per roadmap/17.
func (r *DeprecationRule) Confidence() float64 { return 0.75 }

// ensureDeprecatedIndex lazily builds a set of deprecated declaration info
// for the current file. The index is rebuilt when the file path changes.
func (r *DeprecationRule) ensureDeprecatedIndex(file *scanner.File) {
	if file == nil {
		return
	}
	if r.deprecatedInfosByFile == nil {
		r.deprecatedInfosByFile = make(map[string]map[string]*deprecationInfo)
	}
	if infos, ok := r.deprecatedInfosByFile[file.Path]; ok {
		r.cachedFile = file.Path
		r.deprecatedInfos = infos
		return
	}

	r.cachedFile = file.Path
	r.deprecatedInfos = make(map[string]*deprecationInfo)
	r.deprecatedInfosByFile[file.Path] = r.deprecatedInfos
	if file.FlatTree == nil || !bytes.Contains(file.Content, []byte("Deprecated")) {
		return
	}
	collectDeprecatedDeclsFlat(file, r.deprecatedInfos)
}

func collectDeprecatedDeclsFlat(file *scanner.File, out map[string]*deprecationInfo) {
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		if nodeType != "function_declaration" && nodeType != "class_declaration" && nodeType != "property_declaration" {
			return
		}
		if info := extractDeprecatedInfoFlat(file, idx); info != nil {
			name := extractIdentifierFlat(file, idx)
			if name != "" {
				out[name] = info
			}
		}
	})
}

func extractDeprecatedInfoFlat(file *scanner.File, idx uint32) *deprecationInfo {
	if file == nil || idx == 0 {
		return nil
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return nil
	}
	text := file.FlatNodeText(mods)
	if !strings.Contains(text, "Deprecated") {
		return nil
	}
	info := &deprecationInfo{}
	info.message = extractAnnotationArg(text, "message")
	if info.message == "" {
		info.message = extractFirstPositionalArg(text)
	}
	info.level = extractDeprecationLevel(text)
	info.replaceWith = extractReplaceWith(text)
	return info
}

func flatDeprecationRefName(file *scanner.File, idx uint32) string {
	if file == nil {
		return ""
	}
	switch file.FlatType(idx) {
	case "call_expression":
		return flatCallExpressionName(file, idx)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, idx)
	case "user_type":
		for i := 0; i < file.FlatNamedChildCount(idx); i++ {
			child := file.FlatNamedChild(idx, i)
			if file.FlatType(child) == "type_identifier" {
				return file.FlatNodeText(child)
			}
		}
		text := strings.TrimSpace(file.FlatNodeText(idx))
		if idx := strings.LastIndex(text, "."); idx >= 0 {
			return text[idx+1:]
		}
		return text
	default:
		return ""
	}
}

// extractAnnotationArg extracts a named argument from annotation text.
// e.g., extractAnnotationArg(`@Deprecated(message = "old")`, "message") → "old"
func extractAnnotationArg(annText, argName string) string {
	pattern := argName + "="
	// Also try with spaces: argName + " ="
	idx := strings.Index(annText, pattern)
	if idx < 0 {
		pattern = argName + " ="
		idx = strings.Index(annText, pattern)
	}
	if idx < 0 {
		return ""
	}
	rest := annText[idx+len(pattern):]
	rest = strings.TrimSpace(rest)
	if len(rest) == 0 {
		return ""
	}
	if rest[0] == '"' {
		// Find closing quote
		end := strings.Index(rest[1:], "\"")
		if end >= 0 {
			return rest[1 : end+1]
		}
	}
	return ""
}

// extractFirstPositionalArg extracts the first positional string arg.
// e.g., `@Deprecated("Use newMethod instead")` → "Use newMethod instead"
func extractFirstPositionalArg(annText string) string {
	parenIdx := strings.Index(annText, "(")
	if parenIdx < 0 {
		return ""
	}
	rest := strings.TrimSpace(annText[parenIdx+1:])
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	end := strings.Index(rest[1:], "\"")
	if end >= 0 {
		return rest[1 : end+1]
	}
	return ""
}

// extractDeprecationLevel extracts the level argument from @Deprecated.
// e.g., `@Deprecated(..., level = DeprecationLevel.ERROR)` → "ERROR"
func extractDeprecationLevel(annText string) string {
	idx := strings.Index(annText, "level")
	if idx < 0 {
		return ""
	}
	rest := annText[idx:]
	eqIdx := strings.Index(rest, "=")
	if eqIdx < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[eqIdx+1:])
	// Could be DeprecationLevel.WARNING or just WARNING
	for _, level := range []string{"WARNING", "ERROR", "HIDDEN"} {
		if strings.Contains(rest, level) {
			return level
		}
	}
	return ""
}

// extractReplaceWith extracts the replaceWith expression from @Deprecated.
// e.g., `@Deprecated(..., replaceWith = ReplaceWith("newMethod()"))` → "newMethod()"
func extractReplaceWith(annText string) string {
	idx := strings.Index(annText, "ReplaceWith")
	if idx < 0 {
		return ""
	}
	rest := annText[idx:]
	parenIdx := strings.Index(rest, "(")
	if parenIdx < 0 {
		return ""
	}
	rest = rest[parenIdx+1:]
	rest = strings.TrimSpace(rest)
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	end := strings.Index(rest[1:], "\"")
	if end >= 0 {
		return rest[1 : end+1]
	}
	return ""
}

// ---------------------------------------------------------------------------
// HasPlatformTypeRule detects public fun/property without explicit return type.
// With type inference: uses ResolveNode on expression-body return expressions
// to determine if the inferred type comes from Java interop (platform type).
// When the resolver can determine a concrete Kotlin type, the finding is suppressed.
// ---------------------------------------------------------------------------
type HasPlatformTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because the rule
// falls back to a string-prefix heuristic on "java./javax./android./Java/Javax"
// when the resolver cannot determine the return type. Findings from that
// fallback path carry known false-positive risk on identifiers that
// happen to start with those prefixes but are not Java interop.
func (r *HasPlatformTypeRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// IgnoredReturnValueRule detects call expressions whose non-Unit return value
// is discarded. Uses type inference to resolve return types and tree-sitter
// parent-node analysis to determine if the result is used (assigned, returned,
// passed as argument). Falls back to regex when resolver is unavailable.
// ---------------------------------------------------------------------------
type IgnoredReturnValueRule struct {
	FlatDispatchBase
	BaseRule
	RestrictToConfig             bool
	ReturnValueAnnotations       []string
	IgnoreReturnValueAnnotations []string
	ReturnValueTypes             []string
	IgnoreFunctionCall           []string
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule needs type inference to resolve the return type of the call
// target. When the resolver is unavailable it falls back to name-based
// heuristics against ReturnValueTypes/IgnoreFunctionCall lists, which
// can miss custom wrapper APIs or fire on look-alike names.
func (r *IgnoredReturnValueRule) Confidence() float64 { return 0.75 }

// returnValueTypes that should always be flagged when discarded (detekt defaults)
var returnValueFQNs = map[string]bool{
	"kotlin.sequences.Sequence":                 true,
	"kotlinx.coroutines.flow.Flow":              true,
	"kotlinx.coroutines.flow.StateFlow":         true,
	"kotlinx.coroutines.flow.SharedFlow":        true,
	"kotlinx.coroutines.flow.MutableStateFlow":  true,
	"kotlinx.coroutines.flow.MutableSharedFlow": true,
}

var defaultReturnValueTypePatterns = []string{
	"kotlin.Function*",
	"kotlin.sequences.Sequence",
	"kotlinx.coroutines.flow.*Flow",
	"java.util.stream.*Stream",
}

// checkReturnValueAnnotations are annotation FQNs (or simple names) that mark
// a function's return value as must-use.
var checkReturnValueAnnotations = map[string]bool{
	"CheckReturnValue": true,
	"CheckResult":      true,
	"com.google.errorprone.annotations.CheckReturnValue": true,
	"androidx.annotation.CheckResult":                    true,
	"javax.annotation.CheckReturnValue":                  true,
}

// canIgnoreReturnValueAnnotations are annotation FQNs that override @CheckReturnValue.
var canIgnoreReturnValueAnnotations = map[string]bool{
	"CanIgnoreReturnValue": true,
	"com.google.errorprone.annotations.CanIgnoreReturnValue": true,
}

// functionalOps are method names where discarding the result is almost certainly a bug
var functionalOps = map[string]bool{
	"map": true, "flatMap": true, "filter": true, "filterNot": true,
	"filterIsInstance": true, "sorted": true, "sortedBy": true, "sortedWith": true,
	"reversed": true, "zip": true, "take": true, "drop": true, "distinct": true,
	"groupBy": true, "associate": true, "partition": true, "fold": true, "reduce": true,
	"plus": true, "minus": true, "toList": true, "toSet": true, "toMap": true,
	"mapKeys": true, "mapValues": true, "flatten": true,
	"asSequence": true, "asFlow": true,
}

var ignoredReturnValueFunctionalOpNames = []string{
	"asFlow", "asSequence", "associate", "distinct", "drop", "filter", "filterIsInstance",
	"filterNot", "flatMap", "flatten", "fold", "groupBy", "map", "mapKeys", "mapValues",
	"minus", "partition", "plus", "reduce", "reversed", "sorted", "sortedBy", "sortedWith",
	"take", "toList", "toMap", "toSet", "zip",
}

var ignoredReturnValueOracleIdentifiersList = []string{
	"CheckReturnValue", "CheckResult",
	"asFlow", "asSequence", "associate", "distinct", "drop", "filter", "filterIsInstance",
	"filterNot", "flatMap", "flatten", "fold", "groupBy", "map", "mapKeys", "mapValues",
	"minus", "partition", "plus", "reduce", "reversed", "sorted", "sortedBy", "sortedWith",
	"take", "toList", "toMap", "toSet", "zip",
	"Function", "Sequence", "Flow", "Stream",
}

var ignoredReturnValueInPlaceMutationCalls = map[string]bool{
	"clear":   true,
	"fill":    true,
	"reverse": true,
	"shuffle": true,
	"sort":    true,
}

func ignoredReturnValueFunctionalCallees() []string {
	return append([]string(nil), ignoredReturnValueFunctionalOpNames...)
}

func ignoredReturnValueOracleIdentifiers() []string {
	return append([]string(nil), ignoredReturnValueOracleIdentifiersList...)
}

func flatIsUsedAsExpression(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "source_file":
		return false
	case "statements":
		// Walk backwards from idx looking for a preceding jump_expression
		// (return/throw/break/continue) that makes this position dead.
		// Linear in distance-to-nearest-jump rather than quadratic in
		// sibling index.
		for prev, ok := file.FlatPrevSibling(idx); ok; prev, ok = file.FlatPrevSibling(prev) {
			if !file.FlatTree.Nodes[prev].IsNamed() {
				continue
			}
			if file.FlatType(prev) == "jump_expression" {
				return true
			}
		}
		if gp, ok := file.FlatParent(parent); ok {
			gpType := file.FlatType(gp)
			if gpType == "control_structure_body" || gpType == "lambda_literal" {
				if flatLastNamedChild(file, parent) == idx {
					return flatIsUsedAsExpression(file, gp)
				}
			}
			if gpType == "catch_block" || gpType == "finally_block" {
				if flatLastNamedChild(file, parent) == idx {
					if ggp, ok := file.FlatParent(gp); ok && file.FlatType(ggp) == "try_expression" {
						return flatIsUsedAsExpression(file, ggp)
					}
				}
			}
			if gpType == "try_expression" && flatLastNamedChild(file, parent) == idx {
				return flatIsUsedAsExpression(file, gp)
			}
		}
		return false
	case "property_declaration", "variable_declaration", "value_argument", "value_arguments",
		"return_expression", "jump_expression", "assignment", "augmented_assignment",
		"binary_expression", "comparison_expression", "equality_expression",
		"additive_expression", "multiplicative_expression", "conjunction_expression",
		"disjunction_expression", "elvis_expression", "range_expression",
		"infix_expression", "check_expression", "as_expression", "if_expression",
		"when_expression", "call_expression":
		return true
	case "parenthesized_expression", "navigation_expression":
		return flatIsUsedAsExpression(file, parent)
	case "lambda_literal":
		for i := 0; i < file.FlatNamedChildCount(parent); i++ {
			child := file.FlatNamedChild(parent, i)
			if file.FlatType(child) == "statements" {
				return flatLastNamedChild(file, child) == idx
			}
		}
		return false
	case "control_structure_body":
		if gp, ok := file.FlatParent(parent); ok {
			gpType := file.FlatType(gp)
			if gpType == "if_expression" || gpType == "when_expression" {
				return flatIsUsedAsExpression(file, gp)
			}
			if gpType == "when_entry" {
				if ggp, ok := file.FlatParent(gp); ok && file.FlatType(ggp) == "when_expression" {
					return flatIsUsedAsExpression(file, ggp)
				}
			}
		}
		return false
	case "when_entry":
		if gp, ok := file.FlatParent(parent); ok && file.FlatType(gp) == "when_expression" {
			return flatIsUsedAsExpression(file, gp)
		}
		return false
	default:
		return true
	}
}

func flatValueArgumentStats(file *scanner.File, args uint32) (first uint32, count int) {
	if file == nil || args == 0 {
		return 0, 0
	}
	for i := 0; i < file.FlatNamedChildCount(args); i++ {
		child := file.FlatNamedChild(args, i)
		if file.FlatType(child) != "value_argument" {
			continue
		}
		if first == 0 {
			first = child
		}
		count++
	}
	return first, count
}

// isExplicitLocaleArg heuristically checks whether an argument is Locale.
// containsAsciiInvariantIdentifier returns true if text contains a common
// ASCII-invariant domain identifier name like `currencyCode`, `iban`,
// `mimeType`, etc. These values are always ASCII so locale cannot change
// their case conversion.
func containsAsciiInvariantIdentifier(text string) bool {
	identifiers := []string{
		"currencyCode", "currency", "isoCode", "countryCode", "languageCode",
		"iban", "IBAN",
		"mimeType", "contentType", "MIME",
		"protocol", "scheme", "host", "uri", "URI", "url", "URL",
		"uuid", "UUID", "guid", "GUID",
		"serviceId", "deviceId",
		"cipher", "algorithm", "digest",
		"columnName", "columnNames", "tableName", "indexName",
		// Hex / HTTP-method receivers — always ASCII-only by construction.
		"hex", "Hex", "toHexString", "toHex",
		"verb", "httpMethod", "method", "requestMethod",
	}
	for _, id := range identifiers {
		if strings.Contains(text, id) {
			return true
		}
	}
	return false
}

// isLocaleInsensitiveFormat reports whether a format string contains only
// locale-independent placeholders (%s, %S, %%, %n, %b, %c, %h, %x, %o). It
// returns false if any locale-sensitive placeholder (%d, %f, %e, %g, %t, %T,
// or any grouped numeric width specifier like %,d) is present. Krit follows
// detekt and Android lint here: even ungrouped %d depends on the default
// formatter locale because some locales emit localized digits.
func isLocaleInsensitiveFormat(formatStr string) bool {
	// Strip surrounding quotes
	s := formatStr
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	}
	// Strip triple-quoted strings
	s = strings.TrimPrefix(s, `""`)
	s = strings.TrimSuffix(s, `""`)

	hasAnyFormatSpec := false
	i := 0
	for i < len(s) {
		if s[i] != '%' {
			i++
			continue
		}
		if i+1 >= len(s) {
			return false // trailing % — malformed
		}
		next := s[i+1]
		if next == '%' || next == 'n' {
			i += 2
			continue
		}
		hasAnyFormatSpec = true
		// Skip flags, width, precision chars until the conversion character
		j := i + 1
		for j < len(s) && (s[j] == '-' || s[j] == '+' || s[j] == ' ' || s[j] == '#' || s[j] == '0' || s[j] == ',' || s[j] == '(' || (s[j] >= '0' && s[j] <= '9') || s[j] == '.') {
			if s[j] == ',' {
				return false // grouping separator is locale-sensitive
			}
			j++
		}
		if j >= len(s) {
			return false
		}
		conv := s[j]
		// Locale-independent conversions: s, S, b, B, c, C, h, H, x, X, o.
		// %d is intentionally locale-sensitive: Formatter can localize digits.
		switch conv {
		case 's', 'S', 'b', 'B', 'c', 'C', 'h', 'H', 'x', 'X', 'o':
			// OK
		case 'd', 'f', 'e', 'E', 'g', 'G', 'a', 'A':
			return false // numeric — locale-sensitive
		case 't', 'T':
			return false // date/time — locale-sensitive
		default:
			return false // unknown — be conservative
		}
		i = j + 1
	}
	// If no format specs at all, treat as locale-insensitive (nothing to format).
	_ = hasAnyFormatSpec
	return true
}

func isExplicitLocaleArgFlat(file *scanner.File, arg uint32) bool {
	if file == nil || arg == 0 {
		return false
	}
	expr := flatValueArgumentExpression(file, arg)
	text := ""
	if expr != 0 {
		text = strings.TrimSpace(file.FlatNodeText(expr))
	}
	if text == "" {
		text = strings.TrimSpace(file.FlatNodeText(arg))
	}
	if strings.HasPrefix(text, "Locale.") || strings.HasPrefix(text, "Locale(") {
		return true
	}
	return false
}

// hasCheckReturnAnnotation checks if any annotation in the list matches the
// built-in @CheckReturnValue/@CheckResult patterns or the user-configured patterns.
func hasCheckReturnAnnotation(annotations []string, configPatterns []string) bool {
	for _, ann := range annotations {
		// Check against built-in annotation set
		if checkReturnValueAnnotations[ann] {
			return true
		}
		// Also check the simple name (last segment after '.')
		if idx := strings.LastIndex(ann, "."); idx >= 0 {
			if checkReturnValueAnnotations[ann[idx+1:]] {
				return true
			}
		}
		// Check user-configured patterns (supports wildcards like "*.CheckResult")
		for _, pat := range configPatterns {
			if matchAnnotationPattern(pat, ann) {
				return true
			}
		}
	}
	return false
}

// hasIgnoreReturnAnnotation checks if any annotation overrides @CheckReturnValue
// (e.g., @CanIgnoreReturnValue).
func hasIgnoreReturnAnnotation(annotations []string, configPatterns []string) bool {
	for _, ann := range annotations {
		if canIgnoreReturnValueAnnotations[ann] {
			return true
		}
		if idx := strings.LastIndex(ann, "."); idx >= 0 {
			if canIgnoreReturnValueAnnotations[ann[idx+1:]] {
				return true
			}
		}
		for _, pat := range configPatterns {
			if matchAnnotationPattern(pat, ann) {
				return true
			}
		}
	}
	return false
}

func ignoredReturnValueAnnotationEvidence(annotations []string, checkPatterns, ignorePatterns []string) (check bool, ignored bool) {
	return hasCheckReturnAnnotation(annotations, checkPatterns), hasIgnoreReturnAnnotation(annotations, ignorePatterns)
}

func ignoredReturnValueOracleAnnotations(lookup oracleAnnotationLookup, filePath string, line, col int) []string {
	if lookup == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(values []string) {
		for _, ann := range values {
			if ann == "" || seen[ann] {
				continue
			}
			seen[ann] = true
			out = append(out, ann)
		}
	}
	add(lookup.LookupCallTargetAnnotations(filePath, line, col))
	target := lookup.LookupCallTarget(filePath, line, col)
	if target == "" {
		return out
	}
	add(lookup.LookupAnnotations(target))
	for container := ignoredReturnValueContainerName(target); container != ""; container = ignoredReturnValueContainerName(container) {
		simple := ignoredReturnValueSimpleContainerName(container)
		if simple == "" || simple[0] < 'A' || simple[0] > 'Z' {
			break
		}
		add(lookup.LookupAnnotations(container))
		if simple != container {
			add(lookup.LookupAnnotations(simple))
		}
	}
	return out
}

func ignoredReturnValueMergedOracleAnnotations(lookup oracle.Lookup, file *scanner.File, idx uint32, line, col int) []string {
	if lookup == nil || file == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(values []string) {
		for _, ann := range values {
			if ann == "" || seen[ann] {
				continue
			}
			seen[ann] = true
			out = append(out, ann)
		}
	}
	add(oracleLookupCallTargetAnnotationsFlat(lookup, file, idx))
	add(ignoredReturnValueOracleAnnotations(lookup, file.Path, line, col))
	return out
}

type oracleAnnotationLookup interface {
	LookupAnnotations(key string) []string
	LookupCallTarget(filePath string, line, col int) string
	LookupCallTargetAnnotations(filePath string, line, col int) []string
}

func ignoredReturnValueContainerName(target string) string {
	if idx := strings.LastIndex(target, "."); idx > 0 {
		return target[:idx]
	}
	return ""
}

func ignoredReturnValueSimpleContainerName(target string) string {
	if idx := strings.LastIndex(target, "."); idx >= 0 && idx < len(target)-1 {
		return target[idx+1:]
	}
	return target
}

// matchAnnotationPattern matches an annotation FQN against a pattern.
// Patterns: exact match, "*.Name" matches any FQN ending with ".Name", "*Name"
// matches a suffix.
func matchAnnotationPattern(pattern, annotation string) bool {
	if pattern == annotation {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".Name"
		return strings.HasSuffix(annotation, suffix)
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(annotation, pattern[1:])
	}
	return false
}

// matchesReturnValueType checks if a resolved FQN matches the configured
// returnValueTypes patterns (e.g., "kotlinx.coroutines.flow.*Flow").
func matchesReturnValueType(fqn string, patterns []string) bool {
	fqn = strings.TrimSpace(fqn)
	if fqn == "" {
		return false
	}
	if idx := strings.IndexByte(fqn, '<'); idx >= 0 {
		fqn = fqn[:idx]
	}
	if returnValueFQNs[fqn] {
		return true
	}
	for _, pat := range patterns {
		pat = strings.TrimSpace(pat)
		if pat == fqn {
			return true
		}
		// "pkg.*Suffix" — prefix + suffix glob
		if idx := strings.Index(pat, "*"); idx >= 0 {
			prefix := pat[:idx]
			suffix := pat[idx+1:]
			if strings.HasPrefix(fqn, prefix) && strings.HasSuffix(fqn, suffix) {
				return true
			}
		}
	}
	return false
}

func ignoredReturnValueTypeMatches(rt *typeinfer.ResolvedType, configured []string, restrictToConfig bool) bool {
	if !ignoredReturnValueTypeKnown(rt) ||
		ignoredReturnValueTypeIsUnitOrNothing(rt) {
		return false
	}
	if rt.Kind == typeinfer.TypeFunction ||
		strings.HasPrefix(rt.FQN, "kotlin.Function") ||
		strings.HasPrefix(rt.Name, "Function") {
		return !restrictToConfig || matchesReturnValueType("kotlin.Function", configured)
	}
	patterns := configured
	if !restrictToConfig {
		patterns = append(append([]string{}, defaultReturnValueTypePatterns...), configured...)
	}
	if matchesReturnValueType(rt.FQN, patterns) {
		return true
	}
	if rt.FQN == "" && rt.Name != "" {
		return matchesReturnValueType(rt.Name, patterns)
	}
	return false
}

func ignoredReturnValueTypeKnown(rt *typeinfer.ResolvedType) bool {
	if rt == nil || rt.Kind == typeinfer.TypeUnknown {
		return false
	}
	return rt.FQN != "" || rt.Name != "" || rt.Kind == typeinfer.TypeFunction ||
		rt.Kind == typeinfer.TypeUnit || rt.Kind == typeinfer.TypeNothing
}

func ignoredReturnValueTypeIsUnitOrNothing(rt *typeinfer.ResolvedType) bool {
	if rt == nil {
		return false
	}
	return rt.Kind == typeinfer.TypeUnit || rt.Kind == typeinfer.TypeNothing ||
		rt.FQN == "kotlin.Unit" || rt.FQN == "kotlin.Nothing" ||
		rt.Name == "Unit" || rt.Name == "Nothing"
}

func ignoredReturnValueTypeName(rt *typeinfer.ResolvedType) string {
	if rt == nil {
		return "a must-use type"
	}
	if rt.FQN != "" {
		return rt.FQN
	}
	if rt.Name != "" {
		return rt.Name
	}
	return "a must-use type"
}

var ignoredReturnValueCollectionSourceCalls = map[string]bool{
	"arrayListOf":   true,
	"asFlow":        true,
	"asSequence":    true,
	"emptyList":     true,
	"emptyMap":      true,
	"emptySet":      true,
	"flow":          true,
	"flowOf":        true,
	"hashMapOf":     true,
	"hashSetOf":     true,
	"linkedMapOf":   true,
	"linkedSetOf":   true,
	"listOf":        true,
	"mapOf":         true,
	"mutableListOf": true,
	"mutableMapOf":  true,
	"mutableSetOf":  true,
	"sequence":      true,
	"sequenceOf":    true,
	"setOf":         true,
	"stream":        true,
	"streamOf":      true,
	"toList":        true,
	"toMap":         true,
	"toSet":         true,
}

var ignoredReturnValueReceiverTypeHints = []string{
	"Array",
	"Collection",
	"Flow",
	"Iterable",
	"List",
	"Map",
	"Sequence",
	"Set",
	"Stream",
}

func ignoredReturnValueFunctionalFallbackHasReceiverEvidence(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 || file.FlatType(call) != "call_expression" {
		return false
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatType(navExpr) != "navigation_expression" || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	return ignoredReturnValueReceiverLooksFunctionalSource(file, flatUnwrapParenExpr(file, file.FlatNamedChild(navExpr, 0)))
}

func ignoredReturnValueReceiverLooksFunctionalSource(file *scanner.File, receiver uint32) bool {
	if file == nil || receiver == 0 {
		return false
	}
	receiver = flatUnwrapParenExpr(file, receiver)
	switch file.FlatType(receiver) {
	case "simple_identifier":
		return ignoredReturnValueIdentifierHasFunctionalType(file, file.FlatNodeText(receiver))
	case "navigation_expression":
		return ignoredReturnValueNavigationLooksFunctionalSource(file, receiver)
	case "call_expression":
		return ignoredReturnValueCallLooksFunctionalSource(file, receiver)
	default:
		return false
	}
}

func ignoredReturnValueNavigationLooksFunctionalSource(file *scanner.File, nav uint32) bool {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return false
	}
	if ignoredReturnValueIdentifierHasFunctionalType(file, flatNavigationExpressionLastIdentifier(file, nav)) {
		return true
	}
	if file.FlatNamedChildCount(nav) == 0 {
		return false
	}
	return ignoredReturnValueReceiverLooksFunctionalSource(file, file.FlatNamedChild(nav, 0))
}

func ignoredReturnValueCallLooksFunctionalSource(file *scanner.File, call uint32) bool {
	name := flatCallExpressionName(file, call)
	if name == "" {
		return false
	}
	if ignoredReturnValueCollectionSourceCalls[name] ||
		ignoredReturnValueFunctionReturnsFunctionalType(file, name) {
		return true
	}
	if !functionalOps[name] {
		return false
	}
	return ignoredReturnValueFunctionalFallbackHasReceiverEvidence(file, call)
}

func ignoredReturnValueIdentifierHasFunctionalType(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found {
			return
		}
		switch file.FlatType(idx) {
		case "function_value_parameter", "parameter", "variable_declaration", "property_declaration":
			declName, typeText := ignoredReturnValueDeclarationNameAndType(file, idx)
			found = declName == name && (ignoredReturnValueTypeTextLooksFunctional(typeText) ||
				ignoredReturnValueDeclarationInitializerLooksFunctional(file, idx))
		}
	})
	return found
}

func ignoredReturnValueFunctionReturnsFunctionalType(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found || file.FlatType(idx) != "function_declaration" || flatFunctionName(file, idx) != name {
			return
		}
		found = ignoredReturnValueTypeTextLooksFunctional(ignoredReturnValueFunctionReturnTypeText(file, idx))
	})
	return found
}

func ignoredReturnValueDeclarationNameAndType(file *scanner.File, idx uint32) (string, string) {
	if file == nil || idx == 0 {
		return "", ""
	}
	return extractIdentifierFlat(file, idx), explicitTypeTextFlat(file, idx)
}

func ignoredReturnValueFunctionReturnTypeText(file *scanner.File, fn uint32) string {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return ""
	}
	return explicitTypeTextFlat(file, fn)
}

func ignoredReturnValueDeclarationInitializerLooksFunctional(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found || candidate == idx || file.FlatType(candidate) != "call_expression" {
			return
		}
		found = ignoredReturnValueCallLooksFunctionalSource(file, candidate)
	})
	return found
}

func ignoredReturnValueTypeTextLooksFunctional(typeText string) bool {
	typeText = strings.TrimSpace(typeText)
	if typeText == "" {
		return false
	}
	for _, hint := range ignoredReturnValueReceiverTypeHints {
		if typeText == hint ||
			strings.HasPrefix(typeText, hint+"<") ||
			strings.HasPrefix(typeText, hint+"?") ||
			strings.HasSuffix(typeText, "."+hint) ||
			strings.Contains(typeText, "."+hint+"<") {
			return true
		}
	}
	return false
}

func stringListContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ImplicitDefaultLocaleRule detects locale-sensitive String methods called
// without an explicit Locale argument. Covers case-conversion methods
// (lowercase, uppercase, capitalize, decapitalize) and String.format / ".format".
// Uses tree-sitter dispatch on call_expression for structural accuracy.
// ---------------------------------------------------------------------------
type ImplicitDefaultLocaleRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because the rule
// matches on method names (toLowerCase/toUpperCase/capitalize/
// decapitalize, String.format) without type resolution. Any user-
// defined non-String method with one of those names will produce a
// false positive. Accuracy improves when a type resolver is wired in
// but the current implementation is structural-only.
func (r *ImplicitDefaultLocaleRule) Confidence() float64 { return 0.75 }

// implicitLocaleMethods are case-conversion methods that use the default
// locale when called without arguments and therefore warrant a warning.
//
// NOTE: `lowercase()` / `uppercase()` (Kotlin 1.5+) are *locale-invariant*
// by design (they delegate to `toLowerCase(Locale.ROOT)` / `toUpperCase
// (Locale.ROOT)` respectively) and are NOT listed here — flagging them is
// a false positive. Only the deprecated `toLowerCase()`/`toUpperCase()`
// forms and the `capitalize()`/`decapitalize()` helpers depend on the
// default locale.
var implicitLocaleMethods = map[string]bool{
	"toLowerCase":  true,
	"toUpperCase":  true,
	"capitalize":   true,
	"decapitalize": true,
}

func fileDeclaresStringFormatExtension(file *scanner.File) bool {
	if file == nil {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || flatFunctionName(file, fn) != "format" {
			return
		}
		compact := compactKotlinExpr(file.FlatNodeText(fn))
		if strings.Contains(compact, "funString.format(") ||
			strings.Contains(compact, "funkotlin.String.format(") {
			found = true
		}
	})
	return found
}

// LocaleDefaultForCurrencyRule detects Currency.getInstance(Locale.getDefault())
// inside money-related classes. Currency in these flows should come from the
// business data being formatted, not from the user's device locale.
type LocaleDefaultForCurrencyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs rule. Detection uses structural AST patterns and optional
// resolver-backed type checks; fallback path is heuristic. Classified per
// roadmap/17.
func (r *LocaleDefaultForCurrencyRule) Confidence() float64 { return 0.75 }

func enclosingClassNameFlat(file *scanner.File, idx uint32) string {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if file.FlatType(current) == "class_declaration" {
			return extractIdentifierFlat(file, current)
		}
	}
	return ""
}

func isCurrencyCarrierClassName(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	return strings.Contains(lower, "price") ||
		strings.Contains(lower, "money") ||
		strings.Contains(lower, "amount")
}

func compactKotlinExpr(text string) string {
	return strings.Join(strings.Fields(text), "")
}
