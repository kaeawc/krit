package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// WrongRegionResource
// ---------------------------------------------------------------------------

// WrongRegionResourceRule mirrors Lint's LocaleFolderDetector WrongRegion check: a two-letter language
// paired with a valid region that LocaleManager does not list for it, such as values-de-rUS.
type WrongRegionResourceRule struct {
	ResourceBase
	AndroidRule
}

// AndroidDependencies asks for layouts and string values because both live in locale folders.
func (WrongRegionResourceRule) AndroidDependencies() AndroidDataDependency {
	return AndroidDepLayout | AndroidDepValuesStrings
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *WrongRegionResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WrongRegionResourceRule) check(ctx *api.Context) {
	for _, dir := range resourceFolderDirs(ctx) {
		loc, ok := parseFolderLocale(filepath.Base(dir))
		if !ok || loc.region == "" || len(loc.language) != 2 {
			continue
		}
		relevant := lintRelevantRegions[loc.language]
		if len(relevant) == 0 || slices.Contains(relevant, loc.region) || !lintValidRegions[loc.region] {
			continue
		}
		sorted := append([]string(nil), relevant...)
		defaultRegion := lintDefaultRegion[loc.language]
		sort.SliceStable(sorted, func(i, j int) bool {
			ri, rj := 2, 2
			if sorted[i] == defaultRegion {
				ri = 1
			}
			if sorted[j] == defaultRegion {
				rj = 1
			}
			if ri != rj {
				return ri < rj
			}
			return sorted[i] < sorted[j]
		})
		parts := make([]string, 0, len(sorted))
		for _, region := range sorted {
			parts = append(parts, region+" ("+lintRegionNames[region]+")")
		}
		ctx.Emit(baseFinding(dir, 1, r.BaseRule,
			fmt.Sprintf("Suspicious language and region combination %s (%s) with %s (%s): language %s is usually paired with: %s",
				loc.language, lintLanguageNames[loc.language], loc.region, lintRegionNames[loc.region], loc.language, strings.Join(parts, ", "))))
	}
}

// folderLocale is the locale qualifier of a resource folder name.
type folderLocale struct{ language, region string }

var (
	folderMccRe    = regexp.MustCompile(`^mcc\d{3}$`)
	folderMncRe    = regexp.MustCompile(`^mnc\d{1,3}$`)
	folderRegionRe = regexp.MustCompile(`^r(?:[A-Z]{2}|\d{3})$`)
	folderLangRe   = regexp.MustCompile(`^[a-z]{2,3}$`)
)

// folderOtherQualifierRe matches the non-locale qualifiers FolderConfiguration accepts.
var folderOtherQualifierRe = regexp.MustCompile(`^(?:ldrtl|ldltr|sw\d+dp|w\d+dp|h\d+dp|small|normal|large|xlarge|long|notlong|round|notround|port|land|` +
	`car|desk|appliance|television|tv|watch|vrheadset|night|notnight|ldpi|mdpi|hdpi|xhdpi|xxhdpi|xxxhdpi|tvdpi|anydpi|nodpi|\d+dpi|` +
	`notouch|finger|stylus|keysexposed|keyshidden|keyssoft|nokeys|qwerty|12key|navexposed|navhidden|nonav|dpad|trackball|wheel|` +
	`v\d+|widecg|nowidecg|lowdr|highdr|square)$`)

// parseFolderLocale returns the locale qualifier of a resource folder name such as values-en-rUS or
// values-b+en+US. It reports false when the folder has no locale or is not a valid configuration.
func parseFolderLocale(folder string) (folderLocale, bool) {
	dash := strings.Index(folder, "-")
	if dash < 0 {
		return folderLocale{}, false
	}
	tokens := strings.Split(folder[dash+1:], "-")
	var loc folderLocale
	found := false
	i := 0
	for i < len(tokens) && (folderMccRe.MatchString(tokens[i]) || folderMncRe.MatchString(tokens[i])) {
		i++
	}
	if i < len(tokens) {
		switch tok := tokens[i]; {
		case strings.HasPrefix(tok, "b+"):
			parts := strings.Split(tok[2:], "+")
			if len(parts) == 0 || !folderLangRe.MatchString(parts[0]) {
				return folderLocale{}, false
			}
			loc.language = parts[0]
			for _, part := range parts[1:] {
				if len(part) == 2 && part == strings.ToUpper(part) || len(part) == 3 && part[0] >= '0' && part[0] <= '9' {
					loc.region = part
				}
			}
			found = true
			i++
		case folderLangRe.MatchString(tok):
			loc.language = tok
			found = true
			i++
			if i < len(tokens) && folderRegionRe.MatchString(tokens[i]) {
				loc.region = tokens[i][1:]
				i++
			}
		}
	}
	if !found {
		return folderLocale{}, false
	}
	for ; i < len(tokens); i++ {
		if !folderOtherQualifierRe.MatchString(tokens[i]) {
			return folderLocale{}, false
		}
	}
	return loc, true
}

// resourceFolderDirs lists the res subfolders of the project: those that hold indexed files and, when the
// resource roots exist on disk, every directory under them. Each entry is the folder's directory path.
func resourceFolderDirs(ctx *api.Context) []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(dir string) {
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	if ctx == nil || ctx.ResourceIndex == nil {
		return nil
	}
	idx := ctx.ResourceIndex
	note := func(path string) {
		if path == "" {
			return
		}
		folder, ok := resourceFolderOfPath(path)
		if !ok {
			return
		}
		p := strings.ReplaceAll(path, "\\", "/")
		if i := strings.LastIndex(p, "res/"+folder+"/"); i >= 0 {
			add(filepath.FromSlash(p[:i+len("res/")] + folder))
		}
	}
	for _, layout := range idx.AllLayouts() {
		note(layout.FilePath)
	}
	for _, loc := range idx.StringsLocation {
		note(loc.FilePath)
	}
	for _, translations := range idx.StringTranslations {
		for _, tr := range translations {
			note(tr.FilePath)
		}
	}
	for _, loc := range idx.PluralsLocation {
		note(loc.FilePath)
	}
	for _, resRoot := range resourceRootsFromIndex(idx) {
		entries, err := os.ReadDir(resRoot)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				add(filepath.Join(resRoot, e.Name()))
			}
		}
	}
	sort.Strings(dirs)
	return dirs
}

// resourceFolderOfPath returns the res/ subfolder name holding path.
func resourceFolderOfPath(path string) (string, bool) {
	p := strings.ReplaceAll(path, "\\", "/")
	resIdx := strings.LastIndex(p, "res/")
	if resIdx < 0 {
		return "", false
	}
	rest := p[resIdx+len("res/"):]
	slash := strings.Index(rest, "/")
	if slash <= 0 {
		return "", false
	}
	return rest[:slash], true
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
	// "car" is the UI-mode qualifier, not a language code.
	if part == "car" {
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

// LocaleFolderRule mirrors Lint's LocaleFolder check: the folders for he, id and yi must use the legacy
// iw, in and ji codes that java.util.Locale uses.
type LocaleFolderRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-3 (high) base confidence.
func (r *LocaleFolderRule) Confidence() float64 { return api.ConfidenceVeryHigh }

var deprecatedLocaleCodes = map[string]string{"he": "iw", "id": "in", "yi": "ji"}

func (r *LocaleFolderRule) check(ctx *api.Context) {
	for _, dir := range resourceFolderDirs(ctx) {
		loc, ok := parseFolderLocale(filepath.Base(dir))
		if !ok {
			continue
		}
		if legacy, bad := deprecatedLocaleCodes[loc.language]; bad {
			ctx.Emit(baseFinding(dir, 1, r.BaseRule,
				fmt.Sprintf("The locale folder \"%s\" should be called \"%s\" instead; see the `java.util.Locale` documentation", loc.language, legacy)))
		}
	}
}

// UseAlpha2Rule mirrors Lint's UseAlpha2: a three-letter language or region code that has a
// two-letter equivalent.
type UseAlpha2Rule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-3 (high) base confidence: the code tables come from Lint's LocaleManager.
func (r *UseAlpha2Rule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *UseAlpha2Rule) check(ctx *api.Context) {
	for _, dir := range resourceFolderDirs(ctx) {
		loc, ok := parseFolderLocale(filepath.Base(dir))
		if !ok {
			continue
		}
		if len(loc.language) == 3 {
			if two, found := lintLanguageAlpha2[strings.ToLower(loc.language)]; found {
				ctx.Emit(baseFinding(dir, 1, r.BaseRule,
					fmt.Sprintf("For compatibility, should use 2-letter language codes when available; use `%s` instead of `%s`", two, loc.language)))
			}
		}
		if len(loc.region) == 3 && (loc.region[0] < '0' || loc.region[0] > '9') {
			if two, found := lintRegionAlpha2[strings.ToUpper(loc.region)]; found {
				ctx.Emit(baseFinding(dir, 1, r.BaseRule,
					fmt.Sprintf("For compatibility, should use 2-letter region codes when available; use `%s` instead of `%s`", two, loc.region)))
			}
		}
	}
}
