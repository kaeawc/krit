package rules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
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
		// Judge only the default-locale definition; translation overrides
		// cannot be marked translatable="false" in a meaningful way.
		loc := idx.StringsLocation[name]
		if idx.StringSources != nil {
			src, ok := idx.StringSources[name]
			if !ok {
				continue
			}
			val = src.Value
			loc = android.StringLocation{FilePath: src.FilePath, Line: src.Line}
			if val == "" {
				continue
			}
		}
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
		// A value translated differently per locale is localizable on purpose.
		if translatedDifferently(idx, name, val) {
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

// translatedDifferently reports whether any locale overrides the string with another value.
func translatedDifferently(idx *android.ResourceIndex, name, defaultValue string) bool {
	for _, translated := range idx.TranslatedTokens[name] {
		if translated != defaultValue {
			return true
		}
	}
	return false
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
		if !looksLikeEmbeddedAPIKey(val) {
			continue
		}
		ctx.Emit(baseFinding(loc.FilePath, loc.Line, r.BaseRule,
			fmt.Sprintf("String resource `%s` appears to embed a Google API key directly. Reference a build-time injected `@string/...` value instead.", name)))
	}
}

// looksLikeEmbeddedAPIKey reports a single-token value that is not a resource
// reference or placeholder. UI labels such as "API key" are not keys.
func looksLikeEmbeddedAPIKey(val string) bool {
	val = strings.TrimSpace(val)
	if len(val) < 16 || strings.ContainsAny(val, " \t\r\n") {
		return false
	}
	if strings.HasPrefix(val, "@") || strings.HasPrefix(val, "?") || strings.HasPrefix(val, "${") {
		return false
	}
	if secretLooksLikePlaceholder(val) {
		return false
	}
	// Google API keys start with "AIza"; other values need key-like characters and a digit,
	// so a header or setting name such as "X-Custom-Api-Key" is not a key.
	if strings.HasPrefix(val, "AIza") {
		return true
	}
	hasDigit := false
	for _, r := range val {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '-', r == '_':
		default:
			return false
		}
	}
	return hasDigit
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
	hasLetter, hasSeparatorOrDigit := false, false
	for _, c := range val {
		switch {
		case c >= 'A' && c <= 'Z':
			hasLetter = true
		case c == '_' || (c >= '0' && c <= '9'):
			hasSeparatorOrDigit = true
		default:
			return false
		}
	}
	// A plain capitalized word such as OK or CANCEL is a button label that translators localize.
	return hasLetter && hasSeparatorOrDigit
}

// ---------------------------------------------------------------------------
// InconsistentArraysResource
// ---------------------------------------------------------------------------

// InconsistentArraysResourceRule detects a string-array that has a different
// number of items in different values directories, such as a translation that
// lacks entries the default array has.
type InconsistentArraysResourceRule struct {
	ValuesArraysResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *InconsistentArraysResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InconsistentArraysResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	names := make([]string, 0, len(idx.StringArrayVariants))
	for name := range idx.StringArrayVariants {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		variants := idx.StringArrayVariants[name]
		if len(variants) < 2 {
			continue
		}
		// Lint walks the files in path order: the first file is the reference and the first
		// file with another count is named in the message, once per array.
		byPath := append([]android.StringArrayVariant(nil), variants...)
		sort.SliceStable(byPath, func(i, j int) bool { return byPath[i].FilePath < byPath[j].FilePath })
		base := byPath[0]
		var other *android.StringArrayVariant
		for i := range byPath {
			if byPath[i].Count != base.Count {
				other = &byPath[i]
				break
			}
		}
		if other == nil {
			continue
		}
		// The finding sits on the first declaration in folder order, so `values` comes first.
		primary := variants[0]
		for _, v := range variants[1:] {
			if v.Dir < primary.Dir {
				primary = v
			}
		}
		ctx.Emit(baseFinding(primary.FilePath, primary.Line, r.BaseRule,
			fmt.Sprintf("Array `%s` has an inconsistent number of items (%d in `%s`, %d in `%s`)",
				name, other.Count, fileNameWithParent(other.FilePath), base.Count, fileNameWithParent(base.FilePath))))
	}
}

// fileNameWithParent returns "folder/file.xml" for a path, like Lint's getFileNameWithParent.
func fileNameWithParent(path string) string {
	return filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
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

// ExtraTextResourceRule mirrors Lint's ExtraTextDetector: stray text inside a
// non-values resource file such as a layout or menu. Lint reports the first
// text of each file as an error and skips values, xml and raw files.
type ExtraTextResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *ExtraTextResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ExtraTextResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	files := append(idx.AllLayouts(), idx.AllMenus()...)
	for _, file := range files {
		text, ok := firstStrayText(file.RootView)
		if !ok {
			continue
		}
		folder := filepath.Base(filepath.Dir(filepath.FromSlash(file.FilePath)))
		if i := strings.Index(folder, "-"); i >= 0 {
			folder = folder[:i]
		}
		if folder == "" || folder == "." {
			folder = "manifest"
		}
		collapsed := strings.Join(strings.Fields(strings.TrimSpace(text.Text)), " ")
		if len(collapsed) > 100 {
			collapsed = collapsed[:100] + "..."
		}
		ctx.Emit(baseFindingAt(file.FilePath, text.Line, text.Col, r.BaseRule,
			fmt.Sprintf("Unexpected text found in %s file: \"%s\"", folder, collapsed)))
	}
}

// firstStrayText returns the first non-blank text node in document order,
// skipping the contents of aapt:attr elements as Lint does.
func firstStrayText(v *android.View) (android.XMLTextNode, bool) {
	var best android.XMLTextNode
	found := false
	var walk func(v *android.View)
	walk = func(v *android.View) {
		if v == nil || v.Type == "aapt:attr" {
			return
		}
		for _, t := range v.TextNodes {
			if !found || t.Line < best.Line || (t.Line == best.Line && t.Col < best.Col) {
				best, found = t, true
			}
		}
		for _, c := range v.Children {
			walk(c)
		}
	}
	walk(v)
	return best, found
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
	emitTranslationFindings(ctx, r.BaseRule, lintReferenceType)
	for _, item := range idx.AliasItems {
		refType, ok := aliasReferenceType(item.Value)
		if !ok {
			// Plain text value (not an alias to another resource): nothing to check.
			continue
		}
		if refType == item.Type {
			continue
		}
		// A drawable resolves colors and mipmaps too.
		if item.Type == "drawable" && (refType == "color" || refType == "mipmap") {
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
