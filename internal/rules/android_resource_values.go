package rules

// Android Resource XML rules: Value/string rules.

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
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
func (r *WebViewInScrollViewResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WebViewInScrollViewResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		findWebViewInScroll(layout.RootView, false, layout.FilePath, r.BaseRule, ctx)
	}
}

func findWebViewInScroll(v *android.View, insideScroll bool, path string, rule BaseRule, ctx *api.Context) {
	if v == nil || resourceToolsIgnoreMatches(v, "WebViewLayout") {
		return
	}
	isScroll := android.IsScrollableView(v.Type)
	if insideScroll && v.Type == "WebView" {
		ctx.Emit(baseFindingAt(path, v.Line, v.Col, rule,
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
func (r *OnClickResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *OnClickResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"OnClick"}, func(v *android.View) {
			// A data-binding expression (@{...}) is not the reflective XML handler.
			if handler := v.Attributes["android:onClick"]; handler != "" && !strings.HasPrefix(handler, "@{") && !strings.HasPrefix(handler, "@=") {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:onClick", r.BaseRule,
					fmt.Sprintf("`android:onClick=\"%s\"` in `%s`. Use `View.setOnClickListener` in code instead of XML onClick handlers.",
						handler, v.Type)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WebViewLayoutResource
// ---------------------------------------------------------------------------

// WebViewLayoutResourceRule mirrors Lint's WebViewLayout check: a WebView whose parent uses
// wrap_content for its width or height.
type WebViewLayoutResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *WebViewLayoutResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WebViewLayoutResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"WebViewLayout"}, func(v, parent *android.View) {
			if v.Type != "WebView" || parent == nil {
				return
			}
			// The height wins when both dimensions wrap, as in Lint.
			bad := ""
			for _, attr := range []string{"layout_width", "layout_height"} {
				if parent.Attributes["android:"+attr] == "wrap_content" {
					bad = attr
				}
			}
			if bad == "" {
				return
			}
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
				"Placing a `<WebView>` in a parent element that uses a `wrap_content "+bad+"` can lead to subtle bugs; use `match_parent` instead"))
		})
	}
}

// ---------------------------------------------------------------------------
// TextViewEditsResource
// ---------------------------------------------------------------------------

// TextViewEditsResourceRule mirrors Lint's TextViewDetector: editing attributes such as inputType on a
// TextView or other non-EditText text widget.
type TextViewEditsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *TextViewEditsResourceRule) Confidence() float64 { return api.ConfidenceMedium }

var textViewEditsWidgets = map[string]bool{
	"TextView": true, "Button": true, "ToggleButton": true, "CheckBox": true,
	"RadioButton": true, "CheckedTextView": true, "Switch": true,
}

// textViewEditsAttr reports whether the attribute is one of the editing attributes TextViewDetector flags.
func textViewEditsAttr(name, value string) bool {
	switch name {
	case "autoText", "password", "phoneNumber", "privateImeOptions", "capitalize", "cursorVisible",
		"digits", "editorExtras", "numeric", "imeOptions", "imeActionLabel", "imeActionId", "inputMethod":
		return true
	case "bufferType":
		return value == "editable"
	case "editable":
		return value == "true"
	case "inputType":
		return value != "" && value != "none"
	}
	return false
}

func (r *TextViewEditsResourceRule) check(ctx *api.Context) {
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"TextViewEdits"}, func(v *android.View) {
			if !textViewEditsWidgets[v.Type] {
				return
			}
			for _, full := range rtlViewAttrsInOrder(v) {
				name := strings.TrimPrefix(full, "android:")
				if !textViewEditsAttr(name, v.Attributes[full]) {
					continue
				}
				msg := "Attribute `" + name + "` should not be used with `<" + v.Type + ">`: intended for editable text widgets"
				if v.Type == "TextView" {
					msg = "Attribute `" + name + "` should not be used with `<TextView>`: Change element type to `<EditText>` ?"
				}
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, full, r.BaseRule, msg))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TextFieldsResource
// ---------------------------------------------------------------------------

// TextFieldsResourceRule mirrors Lint's TextFieldDetector: an EditText without an inputType,
// and an inputType that does not fit the word the view id suggests (phone, number, password,
// email, pin, uri, date).
type TextFieldsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *TextFieldsResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *TextFieldsResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"TextFields"}, func(v *android.View) {
			if v.Type != "EditText" {
				return
			}
			r.checkEditText(ctx, idx, layout.FilePath, v)
		})
	}
}

func (r *TextFieldsResourceRule) checkEditText(ctx *api.Context, idx *android.ResourceIndex, path string, v *android.View) {
	inputType, hasAttr := v.Attributes["android:inputType"]
	if !hasAttr {
		if style := v.Attributes["style"]; style != "" {
			value, found, resolved := lintStyleAttribute(idx, style, "android:inputType")
			if !resolved {
				return
			}
			if found {
				inputType = value
			}
		}
	}
	if !hasAttr && inputType == "" || inputType == "TODO" {
		if _, ok := v.Attributes["android:inputMethod"]; ok {
			return
		}
		ctx.Emit(baseFindingAt(path, v.Line, v.Col, r.BaseRule, "This text field does not specify an `inputType`"))
		return
	}
	id, ok := v.Attributes["android:id"]
	if !ok || id == "" || id == "editText" {
		return
	}
	mismatch := func(msg string) {
		line, col := v.PosOf("android:id")
		if hasAttr {
			line, col = v.PosOf("android:inputType")
		}
		ctx.Emit(baseFindingAt(path, line, col, r.BaseRule, msg))
	}
	_, hasPhoneNumber := v.Attributes["android:phoneNumber"]
	_, hasPassword := v.Attributes["android:password"]
	switch {
	case lintContainsWord(id, "phone", true, true):
		if !strings.Contains(inputType, "phone") && !hasPhoneNumber {
			mismatch("The view name (`" + id + "`) suggests this is a phone number, but it does not include '`phone`' in the `inputType`")
		}
	case lintContainsWord(id, "width", false, true) || lintContainsWord(id, "height", false, true) ||
		lintContainsWord(id, "size", false, true) || lintContainsWord(id, "length", false, true) ||
		lintContainsWord(id, "weight", false, true) || lintContainsWord(id, "number", false, true):
		if !strings.Contains(inputType, "number") && !strings.Contains(inputType, "phone") {
			mismatch("The view name (`" + id + "`) suggests this is a number, but it does not include a numeric `inputType` (such as '`numberSigned`')")
		}
	case lintContainsWord(id, "password", true, true):
		if !strings.Contains(inputType, "Password") && !hasPassword {
			mismatch("The view name (`" + id + "`) suggests this is a password, but it does not include '`textPassword`' in the `inputType`")
		}
	case lintContainsWord(id, "email", true, true):
		if !strings.Contains(inputType, "Email") {
			mismatch("The view name (`" + id + "`) suggests this is an e-mail address, but it does not include '`textEmail`' in the `inputType`")
		}
	case lintEndsWithWord(id, "pin", false, true):
		if !strings.Contains(inputType, "numberPassword") && !hasPassword {
			mismatch("The view name (`" + id + "`) suggests this is a password, but it does not include '`numberPassword`' in the `inputType`")
		}
	default:
		if (lintContainsWord(id, "uri", false, false) || lintContainsWord(id, "url", false, false)) && !strings.Contains(inputType, "textUri") {
			mismatch("The view name (`" + id + "`) suggests this is a URI, but it does not include '`textUri`' in the `inputType`")
		}
		if lintContainsWord(id, "date", false, false) && !strings.Contains(inputType, "date") {
			mismatch("The view name (`" + id + "`) suggests this is a date, but it does not include '`date`' or '`datetime`' in the `inputType`")
		}
	}
}

// lintIndexOfWord mirrors TextFieldDetector.indexOfWord: it finds needle in a view id at a
// camelCase or underscore word boundary. The flags accept a match at the start or the end of
// the id without checking the boundary on the other side.
func lintIndexOfWord(haystack, needle string, allowPrefix, allowSuffix bool) int {
	if haystack == "" || len(needle) > len(haystack) {
		return -1
	}
	start := 0
	switch {
	case strings.HasPrefix(haystack, "@+id/"):
		start = len("@+id/")
	case strings.HasPrefix(haystack, "@id/"):
		start = len("@id/")
	}
	n := len(needle)
	last := len(haystack) - n
	for i := start; i <= last; i++ {
		if !strings.EqualFold(haystack[i:i+n], needle) {
			continue
		}
		if i <= start && allowPrefix {
			return i
		}
		if i == last && allowSuffix {
			return i
		}
		if i > start && haystack[i-1] != '_' && !isASCIIUpper(haystack[i]) {
			continue
		}
		if i == last {
			return i
		}
		if next := haystack[i+n]; next == '_' || isASCIIUpper(next) {
			return i
		}
	}
	return -1
}

func isASCIIUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

func lintContainsWord(haystack, needle string, allowPrefix, allowSuffix bool) bool {
	return lintIndexOfWord(haystack, needle, allowPrefix, allowSuffix) != -1
}

func lintEndsWithWord(haystack, needle string, allowPrefix, allowSuffix bool) bool {
	i := lintIndexOfWord(haystack, needle, allowPrefix, allowSuffix)
	return i != -1 && i == len(haystack)-len(needle)
}

// lintStyleAttribute looks up item in a project style the way Lint's getStyleAttributes does:
// it follows an explicit `@style/` parent, or the implicit parent named by the dots in the style
// name when there is no explicit parent. resolved is false when the style is not a project style.
func lintStyleAttribute(idx *android.ResourceIndex, styleRef, item string) (value string, found, resolved bool) {
	if idx == nil || !strings.HasPrefix(strings.TrimSpace(styleRef), "@style/") {
		return "", false, false
	}
	name := strings.TrimPrefix(strings.TrimSpace(styleRef), "@style/")
	for depth := 0; depth < 32; depth++ {
		style := idx.Styles[name]
		if style == nil {
			return "", false, depth > 0
		}
		if v, ok := style.Items[item]; ok {
			return strings.TrimSpace(v), true, true
		}
		parent := strings.TrimSpace(style.Parent)
		switch {
		case strings.HasPrefix(parent, "@style/"):
			name = strings.TrimPrefix(parent, "@style/")
		case parent == "" && strings.Contains(name, "."):
			name = name[:strings.LastIndex(name, ".")]
		default:
			return "", false, true
		}
	}
	return "", false, true
}

// ---------------------------------------------------------------------------
// UnusedAttributeResource
// ---------------------------------------------------------------------------

// UnusedAttributeResourceRule mirrors the layout half of Lint's ApiDetector UnusedAttribute check: an
// android: attribute that only exists from an API level above the module's minSdk, in a layout that
// neither sits in a -vNN folder of that level nor sits under a tools:targetApi of that level.
type UnusedAttributeResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UnusedAttributeResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// unusedAttributeBenign are the attributes ApiDetector.isBenignUnusedAttribute ignores, because older
// platforms degrade gracefully.
var unusedAttributeBenign = map[string]bool{
	"labelFor": true, "importantForAutofill": true, "layout_marginVertical": true, "fullBackupContent": true,
	"textIsSelectable": true, "roundIcon": true, "requestLegacyExternalStorage": true, "foregroundServiceType": true,
	"dataExtractionRules": true, "autofilledHighlight": true, "paddingVertical": true, "layout_marginHorizontal": true,
	"textAlignment": true, "autofillHints": true, "paddingHorizontal": true,
}

// unusedAttributeRtl are the start/end attributes RtlDetector.isRtlAttributeName names; RtlHardcoded covers them.
var unusedAttributeRtl = map[string]bool{
	"layout_alignParentStart": true, "layout_alignParentEnd": true, "layout_marginStart": true, "layout_marginEnd": true,
	"paddingStart": true, "paddingEnd": true, "drawableStart": true, "drawableEnd": true,
	"listPreferredItemPaddingStart": true, "listPreferredItemPaddingEnd": true, "layout_toStartOf": true,
	"layout_toEndOf": true, "layout_alignStart": true, "layout_alignEnd": true,
}

// targetAPILevel parses a tools:targetApi value, a number or a release name such as lollipop.
func targetAPILevel(value string) int {
	value = strings.TrimSpace(value)
	if n, err := strconv.Atoi(value); err == nil {
		return n
	}
	upper := strings.ToUpper(strings.ReplaceAll(value, "-", "_"))
	if level, ok := versionCodeLevels[upper]; ok {
		return level
	}
	for name, level := range map[string]int{"CUPCAKE": 3, "DONUT": 4, "ECLAIR": 5, "FROYO": 8, "GINGERBREAD": 9,
		"HONEYCOMB": 11, "ICE_CREAM_SANDWICH": 14, "JELLY_BEAN": 16, "KITKAT": 19, "LOLLIPOP": 21, "MARSHMALLOW": 23,
		"NOUGAT": 24, "OREO": 26, "PIE": 28} {
		if upper == name {
			return level
		}
	}
	return 0
}

func (r *UnusedAttributeResourceRule) check(ctx *api.Context) {
	projectMin := 0
	if facts := librarymodel.EnsureFacts(ctx.LibraryFacts); facts != nil {
		projectMin = facts.Profile.MinSdkVersion
	}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		// The module's own minSdk decides; a module that does not declare one cannot be judged.
		minSdk := resourceModuleMinSdk(layout.FilePath)
		if minSdk == 0 {
			minSdk = projectMin
		}
		if minSdk == 0 {
			continue
		}
		// A -vNN folder only loads on API NN and up, so the attribute is understood there.
		folderVersion := iconFolderVersion(layout.FilePath)
		var walk func(v *android.View, localMin int)
		walk = func(v *android.View, localMin int) {
			if v == nil || resourceToolsIgnoreMatches(v, "UnusedAttribute") {
				return
			}
			if level := targetAPILevel(v.Attributes["tools:targetApi"]); level > localMin {
				localMin = level
			}
			for _, full := range rtlViewAttrsInOrder(v) {
				name := strings.TrimPrefix(full, "android:")
				msg := unusedAttributeMessage(name, v, minSdk, folderVersion, localMin)
				if msg != "" {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, full, r.BaseRule, msg))
				}
			}
			for _, child := range v.Children {
				walk(child, localMin)
			}
		}
		walk(layout.RootView, 0)
	}
}

// unusedAttributeMessage returns Lint's UnusedAttribute message for one android: attribute, or "" when
// the attribute is understood at the module's minSdk, in this folder or under tools:targetApi.
func unusedAttributeMessage(name string, v *android.View, minSdk, folderVersion, localMin int) string {
	understood := func(level int) bool { return minSdk >= level || folderVersion >= level || localMin >= level }
	switch {
	case name == "layout_width" || name == "layout_height" || name == "id":
		return ""
	case name == "theme" && v.Type == "include":
		if understood(23) {
			return ""
		}
		return fmt.Sprintf("Attribute `android:theme` is only used by `<include>` tags in API level 23 and higher (current min is %d)", minSdk)
	case name == "foreground" && v.Type != "FrameLayout" && !strings.HasSuffix(v.Type, ".FrameLayout") && !strings.HasSuffix(v.Type, "FrameLayout"):
		if understood(23) {
			return ""
		}
		return fmt.Sprintf("Attribute `android:foreground` has no effect on API levels lower than 23 (current min is %d)", minSdk)
	case unusedAttributeBenign[name] || unusedAttributeRtl[name] || name == "supportsRtl":
		return ""
	}
	level, ok := lintAttrSince[name]
	if !ok || understood(level) {
		return ""
	}
	return fmt.Sprintf("Attribute `%s` is only used in API level %d and higher (current min is %d)", name, level, minSdk)
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
	for _, layout := range idx.AllLayouts() {
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
func (r *MissingQuantityResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MissingQuantityResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		if _, ok := quantities["other"]; !ok {
			pluralFile, pluralLine, located := pluralResourceLocation(idx, name)
			if !located {
				continue
			}
			ctx.Emit(baseFinding(pluralFile, pluralLine, r.BaseRule,
				fmt.Sprintf("Plural `%s` is missing the required `other` quantity.", name)))
		}
	}
}

// ---------------------------------------------------------------------------
// UnusedQuantityResource
// ---------------------------------------------------------------------------

// UnusedQuantityResourceRule detects plural quantities that the plural rules of
// the folder's language never select, such as "one" for Japanese. Like Android
// Lint it judges only folders with a language qualifier, reports one finding
// per `<plurals>` element, and ignores languages it has no plural data for.
type UnusedQuantityResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// pluralLanguageOfFolder returns the language of a values folder, or "" when the
// folder has no language qualifier (values, values-night, values-v21) or its
// qualifiers are not a valid configuration. Like Lint's FolderConfiguration it
// requires qualifiers in Android's canonical order and without duplicates, so
// `values-land-ja` and `values-ja-v21-night` have no locale.
func pluralLanguageOfFolder(folder string) string {
	parts := strings.Split(strings.ToLower(folder), "-")
	lang := ""
	next := 0 // index into resourceQualifierOrder of the first still-allowed group
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if strings.HasPrefix(part, "b+") {
			// BCP 47 locale; Lint accepts any tag whose first subtag is a language.
			tag := strings.SplitN(part[2:], "+", 2)[0]
			if next > qualifierLocale || !isLanguageSubtag(tag) {
				return ""
			}
			lang = tag
			next = qualifierLocale + 1
			continue
		}
		if next <= qualifierLocale && isLanguageSubtag(part) && !isOtherQualifierKeyword(part) {
			lang = part
			next = qualifierLocale + 1
			if i+1 < len(parts) && isRegionSubtag(parts[i+1]) {
				i++
			}
			continue
		}
		group := qualifierGroupOf(part)
		if group < next {
			return ""
		}
		next = group + 1
	}
	return lang
}

const (
	qualifierMCC = iota
	qualifierMNC
	qualifierLocale
	qualifierLayoutDir
	qualifierSmallestWidth
	qualifierWidth
	qualifierHeight
	qualifierSize
	qualifierRatio
	qualifierRound
	qualifierColorGamut
	qualifierHDR
	qualifierOrientation
	qualifierUIMode
	qualifierNight
	qualifierDensity
	qualifierTouch
	qualifierKeysExposed
	qualifierKeyboard
	qualifierNavExposed
	qualifierNav
	qualifierDimension
	qualifierVersion
)

// fixedQualifierGroups maps keyword qualifiers to their position in the canonical order.
var fixedQualifierGroups = map[string]int{
	"ldrtl": qualifierLayoutDir, "ldltr": qualifierLayoutDir,
	"small": qualifierSize, "normal": qualifierSize, "large": qualifierSize, "xlarge": qualifierSize,
	"long": qualifierRatio, "notlong": qualifierRatio,
	"round": qualifierRound, "notround": qualifierRound,
	"widecg": qualifierColorGamut, "nowidecg": qualifierColorGamut,
	"highdr": qualifierHDR, "lowdr": qualifierHDR,
	"port": qualifierOrientation, "land": qualifierOrientation, "square": qualifierOrientation,
	"car": qualifierUIMode, "desk": qualifierUIMode, "television": qualifierUIMode,
	"appliance": qualifierUIMode, "watch": qualifierUIMode, "vrheadset": qualifierUIMode,
	"night": qualifierNight, "notnight": qualifierNight,
	"ldpi": qualifierDensity, "mdpi": qualifierDensity, "hdpi": qualifierDensity, "xhdpi": qualifierDensity,
	"xxhdpi": qualifierDensity, "xxxhdpi": qualifierDensity, "nodpi": qualifierDensity,
	"anydpi": qualifierDensity, "tvdpi": qualifierDensity,
	"notouch": qualifierTouch, "finger": qualifierTouch, "stylus": qualifierTouch,
	"keysexposed": qualifierKeysExposed, "keyshidden": qualifierKeysExposed, "keyssoft": qualifierKeysExposed,
	"nokeys": qualifierKeyboard, "qwerty": qualifierKeyboard, "12key": qualifierKeyboard,
	"navexposed": qualifierNavExposed, "navhidden": qualifierNavExposed,
	"nonav": qualifierNav, "dpad": qualifierNav, "trackball": qualifierNav, "wheel": qualifierNav,
}

var (
	mccQualifier       = regexp.MustCompile(`^mcc\d{3}$`)
	mncQualifier       = regexp.MustCompile(`^mnc\d{1,3}$`)
	smallestWidthQual  = regexp.MustCompile(`^sw\d+dp$`)
	widthQualifier     = regexp.MustCompile(`^w\d+dp$`)
	heightQualifier    = regexp.MustCompile(`^h\d+dp$`)
	densityQualifier   = regexp.MustCompile(`^\d+dpi$`)
	dimensionQualifier = regexp.MustCompile(`^\d+x\d+$`)
	versionQualifier   = regexp.MustCompile(`^v\d+$`)
)

// qualifierGroupOf returns a qualifier's position in the canonical order, or -1
// (which never fits) when the text is not a known qualifier.
func qualifierGroupOf(part string) int {
	if group, ok := fixedQualifierGroups[part]; ok {
		return group
	}
	switch {
	case mccQualifier.MatchString(part):
		return qualifierMCC
	case mncQualifier.MatchString(part):
		return qualifierMNC
	case smallestWidthQual.MatchString(part):
		return qualifierSmallestWidth
	case widthQualifier.MatchString(part):
		return qualifierWidth
	case heightQualifier.MatchString(part):
		return qualifierHeight
	case densityQualifier.MatchString(part):
		return qualifierDensity
	case dimensionQualifier.MatchString(part):
		return qualifierDimension
	case versionQualifier.MatchString(part):
		return qualifierVersion
	}
	return -1
}

func isOtherQualifierKeyword(part string) bool {
	_, ok := fixedQualifierGroups[part]
	return ok
}

func isLanguageSubtag(s string) bool {
	if len(s) < 2 || len(s) > 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

// isRegionSubtag matches the `rUS` / `r001` region qualifier (input is lowercase).
func isRegionSubtag(s string) bool {
	if len(s) == 3 && s[0] == 'r' {
		return s[1] >= 'a' && s[1] <= 'z' && s[2] >= 'a' && s[2] <= 'z'
	}
	return len(s) == 4 && s[0] == 'r' && s[1] >= '0' && s[1] <= '9' && s[2] >= '0' && s[2] <= '9' && s[3] >= '0' && s[3] <= '9'
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UnusedQuantityResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnusedQuantityResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	defs := idx.PluralDefinitions
	if len(defs) == 0 {
		// Indexes built without per-locale definitions only know one entry per name.
		for name, quantities := range idx.Plurals {
			if file, line, ok := pluralResourceLocation(idx, name); ok {
				defs = append(defs, android.PluralDefinition{Name: name, FilePath: file, Line: line, Quantities: quantities})
			}
		}
	}
	sort.SliceStable(defs, func(i, j int) bool {
		if defs[i].FilePath != defs[j].FilePath {
			return defs[i].FilePath < defs[j].FilePath
		}
		return defs[i].Line < defs[j].Line
	})
	for _, def := range defs {
		// Lint ignores translatable="false" here, so it is not checked.
		folder, _ := resourceFolderOfPath(def.FilePath)
		lang := pluralLanguageOfFolder(folder)
		if lang == "" {
			continue
		}
		relevant, known := pluralRelevantQuantities[lang]
		if !known {
			continue
		}
		var unused []string
		for _, qty := range pluralQuantityOrder {
			if _, defined := def.Quantities[qty]; defined && !slices.Contains(relevant, qty) {
				unused = append(unused, "`"+qty+"`")
			}
		}
		if len(unused) == 0 {
			continue
		}
		ctx.Emit(baseFinding(def.FilePath, def.Line, r.BaseRule,
			fmt.Sprintf("For language %s the following quantities are not relevant: %s",
				pluralLanguageDescription(lang), strings.Join(unused, ", "))))
	}
}

// pluralQuantityOrder is the order Lint lists quantities in messages. `other` is
// always relevant, so it never appears.
var pluralQuantityOrder = []string{"few", "many", "one", "two", "zero"}

// pluralLanguageDescription formats a language like Lint: "ja" (Japanese).
func pluralLanguageDescription(lang string) string {
	if name, ok := pluralLanguageNames[lang]; ok {
		return fmt.Sprintf("%q (%s)", lang, name)
	}
	return fmt.Sprintf("%q", lang)
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
func (r *ImpliedQuantityResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ImpliedQuantityResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	type localizedPlural struct {
		def    android.PluralDefinition
		folder string
	}
	var plurals []localizedPlural
	for _, def := range idx.PluralDefinitions {
		folder, _ := resourceFolderOfPath(def.FilePath)
		plurals = append(plurals, localizedPlural{def, folder})
	}
	if len(plurals) == 0 {
		// Indexes built without per-locale definitions only know one entry per name.
		for name, quantities := range idx.Plurals {
			if file, line, ok := pluralResourceLocation(idx, name); ok {
				plurals = append(plurals, localizedPlural{android.PluralDefinition{Name: name, FilePath: file, Line: line, Quantities: quantities}, idx.PluralSources[name]})
			}
		}
	}
	sort.SliceStable(plurals, func(i, j int) bool {
		if plurals[i].def.FilePath != plurals[j].def.FilePath {
			return plurals[i].def.FilePath < plurals[j].def.FilePath
		}
		return plurals[i].def.Line < plurals[j].def.Line
	})
	for _, p := range plurals {
		def := p.def
		lang := pluralLanguageOfFolder(p.folder)
		multi := pluralMultiValueExamples[lang]
		if lang == "" || multi == nil {
			continue
		}
		for _, quantity := range []string{"zero", "one", "two"} {
			examples, hit := multi[quantity]
			value, defined := def.Quantities[quantity]
			if !hit || !defined || hasNumberPlaceholder(value) {
				continue
			}
			pos, ok := def.QuantityPos[quantity]
			if !ok {
				pos = android.XMLPos{Line: def.Line}
			}
			ctx.Emit(baseFindingAt(def.FilePath, pos.Line, pos.Col, r.BaseRule,
				fmt.Sprintf("The quantity `'%s'` matches more than one specific number in this locale (%s), but the message did not include a formatting argument (such as `%%d`). This is usually an internationalization error. See full issue explanation for more.",
					quantity, examples)))
		}
	}
}

// hasNumberPlaceholder mirrors Lint's haveFormattingParameter: any format argument counts, including
// positional `%1$s` and non-numeric conversions, as does a `^1` expand template.
func hasNumberPlaceholder(s string) bool {
	slots, _ := formatArgSlots(s)
	if len(slots) > 0 {
		return true
	}
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '^' && s[i+1] >= '0' && s[i+1] <= '9' {
			return true
		}
	}
	return false
}

// pluralMultiValueExamples lists, per language, the quantities (zero, one, two) that match more than one number
// and the integer examples Lint prints for them. Generated from Lint 32.4.1 (CLDR 42).
var pluralMultiValueExamples = map[string]map[string]string{
	"ak": {"one": "0, 1"},
	"am": {"one": "0, 1"},
	"as": {"one": "0, 1"},
	"be": {"one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"bn": {"one": "0, 1"},
	"br": {"one": "1, 21, 31, 41, 51, 61, 81, 101, 1001, …", "two": "2, 22, 32, 42, 52, 62, 82, 102, 1002, …"},
	"bs": {"one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"fa": {"one": "0, 1"},
	"ff": {"one": "0, 1"},
	"fr": {"one": "0, 1"},
	"gd": {"one": "1, 11", "two": "2, 12"},
	"gu": {"one": "0, 1"},
	"gv": {"one": "1, 11, 21, 31, 41, 51, 61, 71, 101, 1001, …", "two": "2, 12, 22, 32, 42, 52, 62, 72, 102, 1002, …"},
	"hi": {"one": "0, 1"},
	"hr": {"one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"hy": {"one": "0, 1"},
	"is": {"one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"kn": {"one": "0, 1"},
	"kw": {"two": "2, 22, 42, 62, 82, 102, 122, 142, 1000, 10000, 100000, …"},
	"ln": {"one": "0, 1"},
	"lt": {"one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"lv": {"zero": "0, 10~20, 30, 40, 50, 60, 100, 1000, 10000, 100000, 1000000, …", "one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"mg": {"one": "0, 1"},
	"mk": {"one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"pa": {"one": "0, 1"},
	"pt": {"one": "0, 1"},
	"ru": {"one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"si": {"one": "0, 1"},
	"sl": {"one": "1, 101, 201, 301, 401, 501, 601, 701, 1001, …", "two": "2, 102, 202, 302, 402, 502, 602, 702, 1002, …"},
	"sr": {"one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"ti": {"one": "0, 1"},
	"tl": {"one": "0~3, 5, 7, 8, 10~13, 15, 17, 18, 20, 21, 100, 1000, 10000, 100000, 1000000, …"},
	"uk": {"one": "1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"},
	"wa": {"one": "0, 1"},
	"zu": {"one": "0, 1"},
}

// validConversions is the set of valid format conversion characters.
var validConversions = map[byte]bool{
	's': true, 'S': true, 'd': true, 'D': true, 'f': true,
	'x': true, 'X': true, 'o': true, 'e': true, 'E': true,
	'g': true, 'G': true, 'b': true, 'B': true, 'h': true,
	'H': true, 'c': true, 'C': true, 'a': true, 'A': true,
	'n': true, 't': true, 'T': true,
}
