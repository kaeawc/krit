package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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
// containsASCIIInvariantIdentifier returns true if text contains a common
// ASCII-invariant domain identifier name like `currencyCode`, `iban`,
// `mimeType`, etc. These values are always ASCII so locale cannot change
// their case conversion.
func containsASCIIInvariantIdentifier(text string) bool {
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
// or any grouped numeric width specifier like %,d) is present. Even ungrouped
// %d depends on the default formatter locale because some locales emit
// localized digits.
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

func implicitDefaultLocaleConstFormatString(file *scanner.File, arg uint32) (string, bool) {
	if file == nil || arg == 0 {
		return "", false
	}
	expr := flatUnwrapParenExpr(file, flatValueArgumentExpression(file, arg))
	name := flatReferenceSimpleName(file, expr)
	if name == "" {
		return "", false
	}
	return findConstStringPropertyValue(file, name)
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
func (r *ImplicitDefaultLocaleRule) Confidence() float64 { return api.ConfidenceMedium }

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

// implicitLocaleAcceptsLocaleArg lists the methods in implicitLocaleMethods
// that have a Locale-taking overload on String. `capitalize` / `decapitalize`
// do not, so they cannot be autofixed by appending `Locale.ROOT`.
var implicitLocaleAcceptsLocaleArg = map[string]bool{
	"toLowerCase": true,
	"toUpperCase": true,
}

// implicitLocaleFileHasLocaleImport returns true when the file imports
// java.util.Locale (or a wildcard that covers it). Used to gate the
// `Locale.ROOT` autofix so we never reference an unimported symbol.
func implicitLocaleFileHasLocaleImport(file *scanner.File) bool {
	if file == nil {
		return false
	}
	return sourceImportsOrMentions(file, "java.util.Locale") ||
		sourceImportsOrMentions(file, "java.util.*")
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
func (r *LocaleDefaultForCurrencyRule) Confidence() float64 { return api.ConfidenceMedium }

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

// HardcodedDateFormatRule detects DateTimeFormatter.ofPattern(pattern) called
// with a single string argument. Without an explicit Locale the formatter
// silently falls back to the device default, which produces locale-dependent
// output for what is usually meant to be a stable machine-readable timestamp.
// SimpleDateFormat is intentionally not handled here; SimpleDateFormatRule in
// the android-correctness rule set covers that constructor.
type HardcodedDateFormatRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HardcodedDateFormatRule) Confidence() float64 { return api.ConfidenceHigh }

// HardcodedNumberFormatRule detects DecimalFormat(pattern) constructors and
// NumberFormat.getInstance() calls without an explicit Locale. Without one,
// the formatter falls back to the device default locale, producing output
// (grouping separators, decimal symbols) that varies by device.
type HardcodedNumberFormatRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *HardcodedNumberFormatRule) Confidence() float64 { return api.ConfidenceHigh }
