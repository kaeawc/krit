package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

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
func (r *WrongRegionResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WrongRegionResourceRule) check(ctx *api.Context) {
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
			ctx.Emit(baseFinding(layout.FilePath, 1, r.BaseRule,
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
func (r *LocaleConfigStaleResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LocaleConfigStaleResourceRule) check(ctx *api.Context) {
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

		ctx.Emit(baseFinding(configPath, line, r.BaseRule,
			fmt.Sprintf("`locales_config.xml` does not match the explicit locale resource folders (%s).", strings.Join(parts, "; "))))
	}
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
	root, err := android.ParseXMLAST(context.Background(), data)
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
// LocaleFolder / UseAlpha2
// ---------------------------------------------------------------------------

// LocaleFolderRule flags `values-XX_YY` style directory names — the BCP-47
// underscore form Android does not accept. The correct form is
// `values-<lang>-r<REGION>`.
type LocaleFolderRule struct {
	ValuesResourceBase
	AndroidRule
}

var localeFolderBadRe = regexp.MustCompile(`^values-(?:[a-z0-9]+-)*([a-z]{2})_([A-Z]{2})(?:-|$)`)

// Confidence reports a tier-3 (high) base confidence: the rule matches against
// actual `res/values-*/` directory names from `os.ReadDir`, not source text,
// so the only error path is a filesystem read failure.
func (r *LocaleFolderRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *LocaleFolderRule) check(ctx *api.Context) {
	forEachValuesQualifierDir(ctx, func(resRoot, name string) {
		m := localeFolderBadRe.FindStringSubmatch(name)
		if m == nil {
			return
		}
		ctx.Emit(baseFinding(filepath.Join(resRoot, name), 1, r.BaseRule,
			"Wrong locale folder naming `"+name+"`. Use `values-<lang>-r<REGION>` format (e.g., `values-"+m[1]+"-r"+m[2]+"`)."))
	})
}

// UseAlpha2Rule flags 3-letter ISO 639-2 locale codes (e.g. `values-eng`)
// that have a 2-letter ISO 639-1 equivalent — Android resource resolution
// uses 639-1 codes, so `values-eng/` is silently ignored.
type UseAlpha2Rule struct {
	ValuesResourceBase
	AndroidRule
}

var alpha3to2 = map[string]string{"eng": "en", "fra": "fr", "deu": "de", "spa": "es", "ita": "it", "por": "pt", "rus": "ru", "jpn": "ja", "kor": "ko", "zho": "zh", "ara": "ar", "hin": "hi", "tur": "tr", "pol": "pl", "nld": "nl", "swe": "sv", "nor": "no", "dan": "da", "fin": "fi", "tha": "th"}
var alpha3FolderRe = regexp.MustCompile(`^values-([a-z]{3})(?:-|$)`)

// Confidence reports a tier-3 (high) base confidence: the 3→2 letter mapping
// is a closed allow-list, so an unknown 3-letter token (`mcc`, `xyz`, ...)
// cannot trigger a false positive.
func (r *UseAlpha2Rule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *UseAlpha2Rule) check(ctx *api.Context) {
	forEachValuesQualifierDir(ctx, func(resRoot, name string) {
		m := alpha3FolderRe.FindStringSubmatch(name)
		if m == nil {
			return
		}
		repl, ok := alpha3to2[m[1]]
		if !ok {
			return
		}
		ctx.Emit(baseFinding(filepath.Join(resRoot, name), 1, r.BaseRule,
			"Use 2-letter ISO 639-1 code `"+repl+"` instead of 3-letter code `"+m[1]+"` in locale folder."))
	})
}

// forEachValuesQualifierDir visits every `values-*` sub-directory under each
// resource root inferred from ctx.ResourceIndex. The walk is sorted and reads
// each resource root at most once per rule invocation.
//
// Caveat: the resource-findings cache is keyed by `resDirContentFingerprint`,
// which hashes file paths but not bare directory entries. An *empty*
// `values-XX_YY/` (committed without any XML file) therefore does not
// invalidate cached findings — in practice users always commit a strings.xml
// alongside the new qualifier, so the gap is narrow.
func forEachValuesQualifierDir(ctx *api.Context, visit func(resRoot, dirName string)) {
	if ctx == nil || ctx.ResourceIndex == nil {
		return
	}
	for _, resRoot := range resourceRootsFromIndex(ctx.ResourceIndex) {
		entries, err := os.ReadDir(resRoot)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, "values-") {
				continue
			}
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			visit(resRoot, name)
		}
	}
}
