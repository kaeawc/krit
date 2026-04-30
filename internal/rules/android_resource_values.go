package rules

// Android Resource XML rules: Value/string rules.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/experiment"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// ---------------------------------------------------------------------------
// WebViewInScrollViewResource
// ---------------------------------------------------------------------------

// WebViewInScrollViewResourceRule detects a WebView placed inside a
// ScrollView. WebView has its own scrolling and nesting it inside a
// ScrollView causes broken scroll behavior.
type WebViewInScrollViewResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *WebViewInScrollViewResourceRule) Confidence() float64 { return 0.75 }

func (r *WebViewInScrollViewResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		findWebViewInScroll(layout.RootView, false, layout.FilePath, r.BaseRule, ctx)
	}
}

func findWebViewInScroll(v *android.View, insideScroll bool, path string, rule BaseRule, ctx *v2.Context) {
	if v == nil {
		return
	}
	isScroll := android.IsScrollableView(v.Type)
	if insideScroll && v.Type == "WebView" {
		ctx.Emit(resourceFinding(path, v.Line, rule,
			"WebView inside a ScrollView causes broken scrolling. "+
				"Remove the ScrollView or use a different container."))
	}
	for _, child := range v.Children {
		findWebViewInScroll(child, insideScroll || isScroll, path, rule, ctx)
	}
}

// ---------------------------------------------------------------------------
// OnClickResource
// ---------------------------------------------------------------------------

// OnClickResourceRule detects android:onClick attributes in layout XML.
// Using onClick in XML is discouraged in favor of View.setOnClickListener in
// code, which is type-safe and does not rely on reflection.
type OnClickResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *OnClickResourceRule) Confidence() float64 { return 0.75 }

func (r *OnClickResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if handler := v.Attributes["android:onClick"]; handler != "" {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`android:onClick=\"%s\"` in `%s`. Use `View.setOnClickListener` in code instead of XML onClick handlers.",
						handler, v.Type)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TextFieldsResource
// ---------------------------------------------------------------------------

// TextFieldsResourceRule detects EditText views that have neither inputType
// nor hint specified. Without inputType the keyboard may be inappropriate.
type TextFieldsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *TextFieldsResourceRule) Confidence() float64 { return 0.75 }

func (r *TextFieldsResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "EditText" && v.Type != "AppCompatEditText" {
				return
			}
			hasInputType := v.Attributes["android:inputType"] != ""
			hasHint := v.Attributes["android:hint"] != ""
			if !hasInputType && !hasHint {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` is missing both `android:inputType` and `android:hint`. "+
						"Specify `inputType` so the correct keyboard is shown.", v.Type)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UnusedAttributeResource
// ---------------------------------------------------------------------------

// UnusedAttributeResourceRule detects layout attributes that require a
// minimum API level. These attributes are silently ignored on older platforms.
type UnusedAttributeResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// apiLevelAttrs maps attribute names to the minimum API level that supports them.
var apiLevelAttrs = map[string]int{
	"android:elevation":         21,
	"android:translationZ":      21,
	"android:stateListAnimator": 21,
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UnusedAttributeResourceRule) Confidence() float64 { return 0.75 }

func (r *UnusedAttributeResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attr, minAPI := range apiLevelAttrs {
				if v.Attributes[attr] != "" {
					ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Attribute `%s` is only used in API level %d and higher "+
							"(current min is unspecified). It will be ignored on older platforms.",
							attr, minAPI)))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WrongRegionResource
// ---------------------------------------------------------------------------

// WrongRegionResourceRule detects suspicious language/region combinations in
// resource folder names, such as values-en-rBR (English + Brazil).
type WrongRegionResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// languageExpectedRegions maps language codes to their expected/natural region codes.
var languageExpectedRegions = map[string][]string{
	"en": {"US", "GB", "AU", "CA", "NZ", "IE", "ZA", "IN", "SG", "PH"},
	"es": {"ES", "MX", "AR", "CO", "CL", "PE", "VE", "EC", "GT", "CU", "BO", "DO", "HN", "PY", "SV", "NI", "CR", "PA", "UY"},
	"fr": {"FR", "CA", "BE", "CH", "LU", "MC", "SN", "CI", "ML", "BF", "NE", "TG", "BJ"},
	"de": {"DE", "AT", "CH", "LI", "LU", "BE"},
	"it": {"IT", "CH", "SM", "VA"},
	"pt": {"PT", "BR", "AO", "MZ"},
	"ru": {"RU", "BY", "KZ", "KG"},
	"zh": {"CN", "TW", "HK", "SG", "MO"},
	"ja": {"JP"},
	"ko": {"KR"},
	"ar": {"SA", "AE", "EG", "IQ", "MA", "DZ", "TN", "LY", "SD", "JO", "LB", "KW", "BH", "QA", "OM", "YE", "SY", "PS"},
	"nl": {"NL", "BE", "SR"},
	"pl": {"PL"},
	"tr": {"TR"},
	"vi": {"VN"},
	"th": {"TH"},
	"sv": {"SE", "FI"},
	"da": {"DK"},
	"fi": {"FI"},
	"nb": {"NO"},
	"uk": {"UA"},
	"el": {"GR", "CY"},
	"cs": {"CZ"},
	"ro": {"RO", "MD"},
	"hu": {"HU"},
	"hr": {"HR", "BA"},
	"sk": {"SK"},
	"bg": {"BG"},
	"hi": {"IN"},
	"he": {"IL"},
	"id": {"ID"},
	"ms": {"MY", "SG", "BN"},
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *WrongRegionResourceRule) Confidence() float64 { return 0.75 }

func (r *WrongRegionResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	seen := make(map[string]bool)
	for _, layout := range idx.Layouts {
		path := layout.FilePath
		resIdx := strings.Index(path, "res/")
		if resIdx < 0 {
			resIdx = strings.Index(path, "res\\")
		}
		if resIdx < 0 {
			continue
		}
		afterRes := path[resIdx+4:]
		slashIdx := strings.IndexAny(afterRes, "/\\")
		if slashIdx < 0 {
			continue
		}
		folder := afterRes[:slashIdx]
		if seen[folder] {
			continue
		}
		seen[folder] = true

		// Parse folder for language-rRegion pattern (e.g., values-en-rBR)
		parts := strings.Split(folder, "-")
		if len(parts) < 3 {
			continue
		}
		var lang, region string
		for i := 1; i < len(parts); i++ {
			if len(parts[i]) == 2 && parts[i] >= "aa" && parts[i] <= "zz" {
				lang = parts[i]
			} else if strings.HasPrefix(parts[i], "r") && len(parts[i]) == 3 {
				region = parts[i][1:] // strip 'r' prefix
			}
		}
		if lang == "" || region == "" {
			continue
		}
		expected, ok := languageExpectedRegions[lang]
		if !ok {
			continue
		}
		found := false
		for _, r := range expected {
			if r == region {
				found = true
				break
			}
		}
		if !found {
			ctx.Emit(resourceFinding(layout.FilePath, 1, r.BaseRule,
				fmt.Sprintf("Suspicious language/region combination: language `%s` with region `%s` "+
					"in folder `%s`. Expected regions for `%s`: %s.",
					lang, region, folder, lang, strings.Join(expected, ", "))))
		}
	}
}

// ---------------------------------------------------------------------------
// LocaleConfigStale
// ---------------------------------------------------------------------------

// LocaleConfigStaleResourceRule detects locale-config resources whose declared
// locales no longer match the explicit values-XX folders present under res/.
type LocaleConfigStaleResourceRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *LocaleConfigStaleResourceRule) Confidence() float64 { return 0.75 }

func (r *LocaleConfigStaleResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, resRoot := range resourceRootsFromIndex(idx) {
		configPath := filepath.Join(resRoot, "xml", "locales_config.xml")
		configLocales, line, err := readLocaleConfigLocales(configPath)
		if err != nil || len(configLocales) == 0 {
			continue
		}

		valuesLocales, err := readValuesLocales(resRoot)
		if err != nil {
			continue
		}

		missing, extra := diffLocaleSets(configLocales, valuesLocales)
		// One locale may legitimately live only in the default values/ folder.
		if len(extra) == 0 && len(missing) <= 1 {
			continue
		}

		var parts []string
		if len(extra) > 0 {
			parts = append(parts, fmt.Sprintf("extra values locales: %s", strings.Join(extra, ", ")))
		}
		if len(missing) > 1 {
			parts = append(parts, fmt.Sprintf("config locales without matching values folders: %s", strings.Join(missing, ", ")))
		}

		ctx.Emit(resourceFinding(configPath, line, r.BaseRule,
			fmt.Sprintf("`locales_config.xml` does not match the explicit locale resource folders (%s).", strings.Join(parts, "; "))))
	}
}

func resourceRootsFromIndex(idx *android.ResourceIndex) []string {
	seen := make(map[string]struct{})
	add := func(path string) {
		root, ok := resourceRootFromPath(path)
		if !ok {
			return
		}
		seen[root] = struct{}{}
	}

	for _, loc := range idx.StringsLocation {
		add(loc.FilePath)
	}
	for _, entry := range idx.ExtraTexts {
		add(entry.FilePath)
	}
	for _, layout := range idx.Layouts {
		if layout != nil {
			add(layout.FilePath)
		}
	}

	roots := make([]string, 0, len(seen))
	for root := range seen {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func resourceRootFromPath(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	parent := filepath.Dir(path)
	switch base := filepath.Base(parent); {
	case strings.HasPrefix(base, "values"),
		strings.HasPrefix(base, "layout"),
		strings.HasPrefix(base, "drawable"),
		strings.HasPrefix(base, "menu"),
		strings.HasPrefix(base, "xml"):
		return filepath.Dir(parent), true
	default:
		return "", false
	}
}

func readLocaleConfigLocales(path string) ([]string, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	root, err := android.ParseXMLAST(data)
	if err != nil {
		return nil, 0, err
	}
	if root.Tag != "locale-config" {
		return nil, root.Line, nil
	}

	seen := make(map[string]struct{})
	var locales []string
	for _, child := range root.ChildrenByTag("locale") {
		locale := normalizeLocaleTag(child.Attr("android:name"))
		if locale == "" {
			locale = normalizeLocaleTag(child.Attr("name"))
		}
		if locale == "" {
			continue
		}
		if _, ok := seen[locale]; ok {
			continue
		}
		seen[locale] = struct{}{}
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales, root.Line, nil
}

func readValuesLocales(resRoot string) ([]string, error) {
	entries, err := os.ReadDir(resRoot)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		locale, ok := localeTagFromValuesDir(entry.Name())
		if !ok {
			continue
		}
		seen[locale] = struct{}{}
	}

	locales := make([]string, 0, len(seen))
	for locale := range seen {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales, nil
}

func localeTagFromValuesDir(dir string) (string, bool) {
	if !strings.HasPrefix(dir, "values-") {
		return "", false
	}

	qualifier := strings.TrimPrefix(dir, "values-")
	if strings.HasPrefix(qualifier, "b+") {
		return normalizeLocaleTag(qualifier), true
	}

	parts := strings.Split(qualifier, "-")
	if len(parts) == 0 || !isLocaleLanguageQualifier(parts[0]) {
		return "", false
	}

	locale := parts[0]
	if len(parts) > 1 && strings.HasPrefix(parts[1], "r") && len(parts[1]) == 3 {
		locale += "-" + parts[1]
	}
	return normalizeLocaleTag(locale), true
}

func isLocaleLanguageQualifier(part string) bool {
	if len(part) < 2 || len(part) > 3 {
		return false
	}
	for _, r := range part {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func normalizeLocaleTag(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return ""
	}
	if strings.HasPrefix(locale, "b+") {
		return strings.Join(strings.Split(strings.TrimPrefix(locale, "b+"), "+"), "-")
	}

	locale = strings.ReplaceAll(locale, "_", "-")
	parts := strings.Split(locale, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			parts[i] = strings.ToLower(part)
			continue
		}
		if strings.HasPrefix(part, "r") && len(part) == 3 {
			part = part[1:]
		}
		switch {
		case len(part) == 2:
			parts[i] = strings.ToUpper(part)
		case len(part) == 4:
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		default:
			parts[i] = part
		}
	}
	return strings.Join(parts, "-")
}

func diffLocaleSets(configLocales, valuesLocales []string) (missing, extra []string) {
	configSet := make(map[string]struct{}, len(configLocales))
	for _, locale := range configLocales {
		configSet[locale] = struct{}{}
	}
	valuesSet := make(map[string]struct{}, len(valuesLocales))
	for _, locale := range valuesLocales {
		valuesSet[locale] = struct{}{}
	}

	for _, locale := range configLocales {
		if _, ok := valuesSet[locale]; !ok {
			missing = append(missing, locale)
		}
	}
	for _, locale := range valuesLocales {
		if _, ok := configSet[locale]; !ok {
			extra = append(extra, locale)
		}
	}
	return missing, extra
}

// ---------------------------------------------------------------------------
// MissingQuantityResource
// ---------------------------------------------------------------------------

// MissingQuantityResourceRule detects plural resources that are missing the
// required "other" quantity. In English (and all locales), "other" is always
// required.
type MissingQuantityResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *MissingQuantityResourceRule) Confidence() float64 { return 0.75 }

func (r *MissingQuantityResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		if _, ok := quantities["other"]; !ok {
			ctx.Emit(resourceFinding("res/values/plurals.xml", 0, r.BaseRule,
				fmt.Sprintf("Plural `%s` is missing the required `other` quantity.", name)))
		}
	}
}

// ---------------------------------------------------------------------------
// UnusedQuantityResource
// ---------------------------------------------------------------------------

// UnusedQuantityResourceRule detects plural quantities that are unnecessary
// for the default language (English). For English, "zero", "two", "few", and
// "many" are not used by the plural rules and will never be selected at runtime.
type UnusedQuantityResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// unusedEnglishQuantities are quantities that English plural rules never select.
var unusedEnglishQuantities = map[string]bool{
	"zero": true,
	"two":  true,
	"few":  true,
	"many": true,
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UnusedQuantityResourceRule) Confidence() float64 { return 0.75 }

func (r *UnusedQuantityResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		for qty := range quantities {
			if unusedEnglishQuantities[qty] {
				ctx.Emit(resourceFinding("res/values/plurals.xml", 0, r.BaseRule,
					fmt.Sprintf("Plural `%s` defines unused quantity `%s` for English. "+
						"Only `one` and `other` are used.", name, qty)))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ImpliedQuantityResource
// ---------------------------------------------------------------------------

// ImpliedQuantityResourceRule detects plural "one" quantities whose value does
// not contain a %d format specifier. When "one" is selected, the actual number
// may not be 1 (e.g., 21, 31 in some locales), so the value should include %d
// to display the correct number.
type ImpliedQuantityResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *ImpliedQuantityResourceRule) Confidence() float64 { return 0.75 }

func (r *ImpliedQuantityResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		if oneVal, ok := quantities["one"]; ok {
			if !strings.Contains(oneVal, "%d") {
				ctx.Emit(resourceFinding("res/values/plurals.xml", 0, r.BaseRule,
					fmt.Sprintf("Plural `%s` quantity `one` value `%s` does not contain `%%d`. "+
						"The actual number may not be 1; use `%%d` to display it correctly.",
						name, truncate(oneVal, 40))))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// StringFormatInvalidResource
// ---------------------------------------------------------------------------

// StringFormatInvalidResourceRule detects invalid format specifiers in string
// resources. Checks for bare `%` at end of string, `%` followed by invalid
// conversion characters, etc. Valid: %s, %d, %f, %1$s, %%, %n, %x, %o, %e, %g.
type StringFormatInvalidResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// validConversions is the set of valid format conversion characters.
var validConversions = map[byte]bool{
	's': true, 'S': true, 'd': true, 'D': true, 'f': true,
	'x': true, 'X': true, 'o': true, 'e': true, 'E': true,
	'g': true, 'G': true, 'b': true, 'B': true, 'h': true,
	'H': true, 'c': true, 'C': true, 'a': true, 'A': true,
	'n': true, 't': true, 'T': true,
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatInvalidResourceRule) Confidence() float64 { return 0.75 }

func (r *StringFormatInvalidResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, val := range idx.Strings {
		// Skip strings with formatted="false". These are NOT format
		// strings — the `%{var}` / `%word` sequences are literal text
		// (typically for gettext/phrase-style translation placeholders).
		if idx.StringsNonFormatted != nil && idx.StringsNonFormatted[name] {
			continue
		}
		msg := checkInvalidFormatString(val)
		if msg == "" {
			continue
		}
		// Use the real file path and line from the resource index. The
		// previous hardcoded `res/values/strings.xml` at line 0 both
		// mislocated findings (emitting a CWD-relative path that could
		// even reference other repos if krit was run from a parent dir)
		// and collapsed multiple findings into one key.
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
		ctx.Emit(resourceFinding(filePath, line, r.BaseRule,
			fmt.Sprintf("String `%s` has invalid format specifier: %s", name, msg)))
	}
}

// checkInvalidFormatString checks a string value for invalid % sequences.
// Returns an error description or "" if valid.
func checkInvalidFormatString(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 >= len(s) {
			return "bare `%` at end of string"
		}
		next := s[i+1]
		// %% is a literal percent
		if next == '%' {
			i++
			continue
		}
		// Skip optional argument index (digits followed by $)
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == '$' {
			j++
		}
		// Skip flags
		for j < len(s) && (s[j] == '-' || s[j] == '+' || s[j] == ' ' || s[j] == '0' || s[j] == '#') {
			j++
		}
		// Skip width
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		// Skip precision
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
		}
		if j >= len(s) {
			return "bare `%` at end of string (incomplete format)"
		}
		c := s[j]
		if !validConversions[c] {
			return fmt.Sprintf("invalid conversion character `%c` after `%%`", c)
		}
		i = j
	}
	return ""
}

// ---------------------------------------------------------------------------
// StringFormatCountResource
// ---------------------------------------------------------------------------

// StringFormatCountResourceRule detects format strings where positional argument
// indices are inconsistent. For example, `%1$s and %3$s` is suspicious because
// %2$ is missing.
type StringFormatCountResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatCountResourceRule) Confidence() float64 { return 0.75 }

func (r *StringFormatCountResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, val := range idx.Strings {
		if msg := checkFormatArgCount(val); msg != "" {
			ctx.Emit(resourceFinding("res/values/strings.xml", 0, r.BaseRule,
				fmt.Sprintf("String `%s`: %s", name, msg)))
		}
	}
}

// checkFormatArgCount checks for gaps in positional argument indices.
func checkFormatArgCount(s string) string {
	indices := make(map[int]bool)
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 >= len(s) {
			continue
		}
		if s[i+1] == '%' {
			i++
			continue
		}
		// Look for digits followed by $
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j > i+1 && j < len(s) && s[j] == '$' {
			idx, _ := strconv.Atoi(s[i+1 : j])
			if idx > 0 {
				indices[idx] = true
			}
			i = j
		}
	}
	if len(indices) < 2 {
		return ""
	}
	// Find max index and check for gaps
	maxIdx := 0
	for idx := range indices {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	var missing []int
	for i := 1; i <= maxIdx; i++ {
		if !indices[i] {
			missing = append(missing, i)
		}
	}
	if len(missing) > 0 {
		parts := make([]string, len(missing))
		for i, m := range missing {
			parts[i] = fmt.Sprintf("%%%d$", m)
		}
		return fmt.Sprintf("positional arguments have gap: missing %s (max is %%%d$)",
			strings.Join(parts, ", "), maxIdx)
	}
	return ""
}

// ---------------------------------------------------------------------------
// StringFormatMatchesResource
// ---------------------------------------------------------------------------

// StringFormatMatchesResourceRule detects format specifier type mismatches across
// plural quantities. If a plural has `%d` in "one" but `%s` in "other", the
// types are inconsistent and will likely cause a runtime crash.
type StringFormatMatchesResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatMatchesResourceRule) Confidence() float64 { return 0.75 }

func (r *StringFormatMatchesResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		if len(quantities) < 2 {
			continue
		}
		// Extract format specifier types for each quantity
		typesByQty := make(map[string][]byte)
		for qty, val := range quantities {
			types := extractFormatTypes(val)
			if len(types) > 0 {
				typesByQty[qty] = types
			}
		}
		if len(typesByQty) < 2 {
			continue
		}
		// Compare all quantities against the first one found
		var refQty string
		var refTypes []byte
		for qty, types := range typesByQty {
			refQty = qty
			refTypes = types
			break
		}
		for qty, types := range typesByQty {
			if qty == refQty {
				continue
			}
			if len(types) != len(refTypes) {
				ctx.Emit(resourceFinding("res/values/strings.xml", 0, r.BaseRule,
					fmt.Sprintf("Plural `%s`: quantity `%s` has %d format args but `%s` has %d",
						name, qty, len(types), refQty, len(refTypes))))
				continue
			}
			for i := range types {
				if types[i] != refTypes[i] {
					ctx.Emit(resourceFinding("res/values/strings.xml", 0, r.BaseRule,
						fmt.Sprintf("Plural `%s`: format type mismatch at arg %d — `%s` uses `%%%c` but `%s` uses `%%%c`",
							name, i+1, qty, types[i], refQty, refTypes[i])))
					break
				}
			}
		}
	}
}

// extractFormatTypes returns the conversion characters from format specifiers.
func extractFormatTypes(s string) []byte {
	var types []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 >= len(s) {
			continue
		}
		if s[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		// Skip argument index
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == '$' {
			j++
		}
		// Skip flags
		for j < len(s) && (s[j] == '-' || s[j] == '+' || s[j] == ' ' || s[j] == '0' || s[j] == '#') {
			j++
		}
		// Skip width
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		// Skip precision
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
		}
		if j < len(s) && validConversions[s[j]] {
			types = append(types, s[j])
			i = j
		}
	}
	return types
}

// ---------------------------------------------------------------------------
// StringFormatTrivialResource
// ---------------------------------------------------------------------------

// StringFormatTrivialResourceRule detects string resources that contain a
// single "%s" format specifier with no other specifiers. This is a trivial
// use of String.format that could be replaced with simple concatenation.
type StringFormatTrivialResourceRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *StringFormatTrivialResourceRule) Confidence() float64 { return 0.75 }

func (r *StringFormatTrivialResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, val := range idx.Strings {
		count := countFormatSpecifiers(val)
		if count == 1 && strings.Contains(val, "%s") {
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
			ctx.Emit(resourceFinding(filePath, line, r.BaseRule,
				fmt.Sprintf("String `%s` uses a single `%%s` format specifier. "+
					"Consider using string concatenation instead of `String.format`.", name)))
		}
	}
}

// countFormatSpecifiers counts the number of format specifiers in a string.
// It recognizes patterns like %s, %d, %f, %1$s, %2$d, etc.
func countFormatSpecifiers(s string) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 >= len(s) {
			continue
		}
		// Skip %%
		if s[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		// Skip argument index (e.g., "1$")
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == '$' {
			j++
		}
		// Skip flags, width, precision
		for j < len(s) && (s[j] == '-' || s[j] == '+' || s[j] == ' ' || s[j] == '0' || s[j] == '#') {
			j++
		}
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
		}
		if j < len(s) {
			c := s[j]
			if c == 's' || c == 'd' || c == 'f' || c == 'x' || c == 'o' || c == 'e' || c == 'g' || c == 'S' || c == 'D' {
				count++
				i = j
			}
		}
	}
	return count
}

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
func (r *StringNotLocalizableResourceRule) Confidence() float64 { return 0.75 }

func (r *StringNotLocalizableResourceRule) check(ctx *v2.Context) {
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
			ctx.Emit(resourceFinding(filePath, line, r.BaseRule,
				fmt.Sprintf("String `%s` contains a URL (`%s`). "+
					"URLs should not be in localizable string resources; use a non-translatable resource.",
					name, truncate(val, 60))))
			continue
		}
		if isEmail(val) {
			ctx.Emit(resourceFinding(filePath, line, r.BaseRule,
				fmt.Sprintf("String `%s` contains an email address (`%s`). "+
					"Email addresses should not be in localizable string resources.",
					name, truncate(val, 60))))
			continue
		}
		if isTechnicalIdentifier(val) {
			ctx.Emit(resourceFinding(filePath, line, r.BaseRule,
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

// GoogleApiKeyInResourcesRule detects string resources whose names suggest
// they embed a Google API key directly in XML instead of referencing a
// build-time injected string resource.
type GoogleApiKeyInResourcesRule struct {
	ValuesStringsResourceBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *GoogleApiKeyInResourcesRule) Confidence() float64 { return 0.75 }

func (r *GoogleApiKeyInResourcesRule) check(ctx *v2.Context) {
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
		ctx.Emit(resourceFinding(loc.FilePath, loc.Line, r.BaseRule,
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
		} else if c >= '0' && c <= '9' || c == '_' {
			// ok
		} else {
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
func (r *InconsistentArraysResourceRule) Confidence() float64 { return 0.75 }

func (r *InconsistentArraysResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, items := range idx.StringArrays {
		if len(items) == 0 {
			ctx.Emit(resourceFinding("res/values/arrays.xml", 0, r.BaseRule,
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
func (r *StringTrailingWhitespaceResourceRule) Confidence() float64 { return 0.85 }

func (r *StringTrailingWhitespaceResourceRule) check(ctx *v2.Context) {
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
		ctx.Emit(resourceFinding(filePath, line, r.BaseRule,
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
func (r *ExtraTextResourceRule) Confidence() float64 { return 0.75 }

func (r *ExtraTextResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, et := range idx.ExtraTexts {
		ctx.Emit(resourceFinding(et.FilePath, et.Line, r.BaseRule,
			fmt.Sprintf("Extraneous text `%s` found in resource file. "+
				"Text outside elements is usually a mistake.",
				truncate(et.Text, 40))))
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
