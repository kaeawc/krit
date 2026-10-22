package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// StringNotLocalizableResource
// ---------------------------------------------------------------------------

// StringNotLocalizableResourceRule detects string resources containing values
// that should not be localized, such as URLs, email addresses, or technical
// identifiers (all-uppercase strings).
type StringNotLocalizableResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringNotLocalizableResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringNotLocalizableResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	skipNonDefault := experiment.Enabled("string-not-localizable-default-values-only")
	for name, val := range idx.Strings {
		if val == "" {
			continue
		}
		// Skip strings already marked translatable="false" — the developer
		// has already applied the suggested fix.
		if idx.StringsNonTranslate[name] {
			continue
		}
		// Look up actual file path and line; fall back to directory if missing.
		loc := idx.StringsLocation[name]
		filePath := loc.FilePath
		if filePath == "" {
			filePath = "res/values/strings.xml"
		}
		// Skip translation / qualified value sets — strings there are
		// overrides of the default. Flagging them produces duplicate
		// findings for every locale and conflates "localizable resource"
		// concerns with "translation override" concerns.
		if skipNonDefault && isNonDefaultValuesPath(filePath) {
			continue
		}
		line := loc.Line
		if isURL(val) {
			ctx.Emit(baseFinding(filePath, line, r.BaseRule,
				fmt.Sprintf("String `%s` contains a URL (`%s`). "+
					"URLs should not be in localizable string resources; use a non-translatable resource.",
					name, truncate(val, 60))))
			continue
		}
		if isEmail(val) {
			ctx.Emit(baseFinding(filePath, line, r.BaseRule,
				fmt.Sprintf("String `%s` contains an email address (`%s`). "+
					"Email addresses should not be in localizable string resources.",
					name, truncate(val, 60))))
			continue
		}
		if isTechnicalIdentifier(val) {
			ctx.Emit(baseFinding(filePath, line, r.BaseRule,
				fmt.Sprintf("String `%s` appears to be a technical identifier (`%s`). "+
					"Consider marking as `translatable=\"false\"`.",
					name, truncate(val, 60))))
		}
	}
}

// isNonDefaultValuesPath reports whether the path references a qualified
// values directory (e.g., values-zh-rTW/, values-night/, values-sw600dp/).
// Default strings live only under `res/values/` with no qualifier.
func isNonDefaultValuesPath(path string) bool {
	p := strings.ReplaceAll(path, "\\", "/")
	// Look for `/res/values-` (any qualifier). `/res/values/` (no qualifier)
	// is the default and is NOT considered non-default.
	idx := strings.Index(p, "/res/values-")
	return idx >= 0
}

// isURL returns true if the value starts with http:// or https://.
func isURL(val string) bool {
	return strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://")
}

// isEmail returns true if the value looks like an email address.
func isEmail(val string) bool {
	atIdx := strings.Index(val, "@")
	if atIdx <= 0 || atIdx >= len(val)-1 {
		return false
	}
	after := val[atIdx+1:]
	return strings.Contains(after, ".") && !strings.Contains(val, " ")
}

// ---------------------------------------------------------------------------
// GoogleApiKeyInResources
// ---------------------------------------------------------------------------

// GoogleAPIKeyInResourcesRule detects string resources whose names suggest
// they embed a Google API key directly in XML instead of referencing a
// build-time injected string resource.
type GoogleAPIKeyInResourcesRule struct {
	ValuesStringsResourceBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *GoogleAPIKeyInResourcesRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GoogleAPIKeyInResourcesRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, val := range idx.Strings {
		if !isGoogleAPIKeyResourceName(name) {
			continue
		}
		loc, ok := idx.StringsLocation[name]
		if !ok || !isValuesStringsXMLPath(loc.FilePath) {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(val), "@string/") {
			continue
		}
		ctx.Emit(baseFinding(loc.FilePath, loc.Line, r.BaseRule,
			fmt.Sprintf("String resource `%s` appears to embed a Google API key directly. Reference a build-time injected `@string/...` value instead.", name)))
	}
}

func isGoogleAPIKeyResourceName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "api_key") || strings.Contains(lower, "api.key")
}

func isValuesStringsXMLPath(path string) bool {
	p := strings.ReplaceAll(path, "\\", "/")
	idx := strings.Index(p, "res/values")
	if idx < 0 {
		return false
	}
	suffix := p[idx+len("res/values"):]
	if suffix == "/strings.xml" {
		return true
	}
	return strings.HasPrefix(suffix, "-") && strings.HasSuffix(suffix, "/strings.xml")
}

// isTechnicalIdentifier returns true if the value is all uppercase letters,
// digits, and underscores (at least 2 chars, at least one letter).
func isTechnicalIdentifier(val string) bool {
	if len(val) < 2 {
		return false
	}
	hasLetter := false
	for _, c := range val {
		if c >= 'A' && c <= 'Z' {
			hasLetter = true
		} else if c < '0' || (c > '9' && c != '_') {
			return false
		}
	}
	return hasLetter
}

// ---------------------------------------------------------------------------
// InconsistentArraysResource
// ---------------------------------------------------------------------------

// InconsistentArraysResourceRule detects string-array resources with zero items.
// Arrays with zero items are likely incomplete or missing translations.
type InconsistentArraysResourceRule struct {
	ValuesArraysResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *InconsistentArraysResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InconsistentArraysResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, items := range idx.StringArrays {
		if len(items) == 0 {
			ctx.Emit(baseFinding("res/values/arrays.xml", 0, r.BaseRule,
				fmt.Sprintf("String-array `%s` has zero items. This may indicate an incomplete array definition.",
					name)))
		}
	}
}

// ---------------------------------------------------------------------------
// StringTrailingWhitespace
// ---------------------------------------------------------------------------

// StringTrailingWhitespaceResourceRule detects translatable <string> resource
// values whose raw text ends with whitespace. Trailing whitespace is
// significant in some locales and in concatenated strings, so it is almost
// always a mistake unless the resource is marked translatable="false".
type StringTrailingWhitespaceResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringTrailingWhitespaceResourceRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *StringTrailingWhitespaceResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name := range idx.StringsTrailingWS {
		if idx.StringsNonTranslate[name] {
			continue
		}
		filePath := "res/values/strings.xml"
		line := 0
		if loc, ok := idx.StringsLocation[name]; ok {
			if loc.FilePath != "" {
				filePath = loc.FilePath
			}
			if loc.Line > 0 {
				line = loc.Line
			}
		}
		ctx.Emit(baseFinding(filePath, line, r.BaseRule,
			fmt.Sprintf("String `%s` has trailing whitespace. Trailing whitespace is significant in concatenated strings and some locales; trim it or mark the resource `translatable=\"false\"`.", name)))
	}
}

// ---------------------------------------------------------------------------
// ExtraTextResource
// ---------------------------------------------------------------------------

// ExtraTextResourceRule detects extraneous text content between elements in
// values XML files. Stray text in <resources> is usually a copy-paste error.
type ExtraTextResourceRule struct {
	ValuesExtraTextResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *ExtraTextResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ExtraTextResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, et := range idx.ExtraTexts {
		ctx.Emit(baseFinding(et.FilePath, et.Line, r.BaseRule,
			fmt.Sprintf("Extraneous text `%s` found in resource file. "+
				"Text outside elements is usually a mistake.",
				truncate(et.Text, 40))))
	}
}

// ---------------------------------------------------------------------------
// ReferenceType
// ---------------------------------------------------------------------------

// ReferenceTypeRule detects a resource alias whose value references a resource
// of a different type than the alias declares. A `<item name="x" type="T">`
// whose value is `@R/name` must have R == T (ignoring an optional `android:`
// framework namespace); otherwise the alias is invalid.
type ReferenceTypeRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the alias type and
// reference are read from the parsed resource index, but values resources can
// carry project-specific shapes. Classified per roadmap/17.
func (r *ReferenceTypeRule) Confidence() float64 { return api.ConfidenceMedium }

// aliasReferenceType returns the resource type named by a reference value such
// as "@string/foo" or "@android:drawable/bar" and reports whether the value is
// a resource reference at all. The leading `@`, an optional `+` (id-creating
// form), and an optional `android:` (or other) namespace are all handled.
func aliasReferenceType(value string) (refType string, isReference bool) {
	v := strings.TrimSpace(value)
	if !strings.HasPrefix(v, "@") {
		return "", false
	}
	v = strings.TrimPrefix(v[1:], "+")
	slash := strings.IndexByte(v, '/')
	if slash <= 0 {
		return "", false
	}
	typePart := v[:slash]
	// Strip an optional namespace prefix, e.g. "android:drawable" -> "drawable".
	if colon := strings.IndexByte(typePart, ':'); colon >= 0 {
		typePart = typePart[colon+1:]
	}
	if typePart == "" {
		return "", false
	}
	return typePart, true
}

func (r *ReferenceTypeRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, item := range idx.AliasItems {
		refType, ok := aliasReferenceType(item.Value)
		if !ok {
			// Plain text value (not an alias to another resource): nothing to check.
			continue
		}
		if refType == item.Type {
			continue
		}
		ctx.Emit(baseFinding(item.FilePath, item.Line, r.BaseRule,
			fmt.Sprintf("Resource alias `%s` is declared as type `%s` but references `%s` of type `%s`. "+
				"An alias must reference a resource of the same type.",
				item.Name, item.Type, strings.TrimSpace(item.Value), refType)))
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
