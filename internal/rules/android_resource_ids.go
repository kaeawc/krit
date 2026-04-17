package rules

// Android Resource XML rules: ID/reference/namespace rules.

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// DuplicateIdsResource
// ---------------------------------------------------------------------------

// DuplicateIdsResourceRule detects the same android:id used more than once
// in a single layout file.
type DuplicateIdsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *DuplicateIdsResourceRule) Confidence() float64 { return 0.75 }

func (r *DuplicateIdsResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		seen := make(map[string]int) // id -> first line
		walkViews(layout.RootView, func(v *android.View) {
			if v.ID == "" {
				return
			}
			id := v.ID
			id = strings.TrimPrefix(id, "@+id/")
			id = strings.TrimPrefix(id, "@id/")
			if firstLine, ok := seen[id]; ok {
				findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Duplicate id `@+id/%s` in layout `%s` (first used at line %d).",
						id, layout.Name, firstLine)))
			} else {
				seen[id] = v.Line
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// InvalidIdResource
// ---------------------------------------------------------------------------

// InvalidIdResourceRule detects malformed android:id values. Valid IDs match
// @+id/name or @id/name where name contains only [a-zA-Z0-9_].
type InvalidIdResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

func isValidAndroidID(id string) bool {
	if id == "" {
		return true
	}
	// Strip @+id/ or @id/ prefix
	rest := id
	if strings.HasPrefix(rest, "@+id/") {
		rest = rest[5:]
	} else if strings.HasPrefix(rest, "@id/") {
		rest = rest[4:]
	} else if strings.HasPrefix(rest, "@android:id/") {
		// Framework IDs are valid
		return true
	} else {
		// Must start with @+id/ or @id/
		return false
	}
	if rest == "" {
		return false
	}
	for _, c := range rest {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *InvalidIdResourceRule) Confidence() float64 { return 0.75 }

func (r *InvalidIdResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			id := v.Attributes["android:id"]
			if id == "" {
				return
			}
			if !isValidAndroidID(id) {
				findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Invalid `android:id` value `%s`. IDs must be `@+id/name` or `@id/name` with alphanumeric/underscore names.",
						id)))
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// MissingIdResource
// ---------------------------------------------------------------------------

// MissingIdResourceRule detects <fragment> and <include> tags that lack an
// android:id attribute. Fragment and include views should have IDs so they can
// be properly restored during configuration changes.
type MissingIdResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *MissingIdResourceRule) Confidence() float64 { return 0.75 }

func (r *MissingIdResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "fragment" && v.Type != "include" {
				return
			}
			if v.ID != "" {
				return
			}
			// Also accept android:tag as alternative
			if v.Attributes["android:tag"] != "" {
				return
			}
			findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("<%s> should specify an `android:id` or `android:tag` to allow proper state saving.",
					v.Type)))
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// CutPasteIdResource
// ---------------------------------------------------------------------------

// CutPasteIdResourceRule detects likely copy-paste ID mistakes where a view's
// android:id name contains a prefix/hint that suggests a different view type.
// For example, `@+id/textview_name` on a Button suggests copy-paste from a
// TextView.
type CutPasteIdResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// semanticSuffixWords are words that commonly follow a feature prefix in
// compound IDs (e.g., `button_strip_container`). When the rest of an ID
// after the type-prefix contains any of these, the prefix is semantic
// rather than a type claim.
var semanticSuffixWords = []string{
	"_container", "_frame", "_barrier", "_toggle", "_strip", "_target",
	"_start", "_end", "_widget", "_progress", "_holder", "_group",
	"_wrapper", "_background", "_foreground", "_layout", "_parent",
	"_inner", "_outer", "_top", "_bottom", "_left", "_right",
}

// containsSemanticSuffix returns true if the ID suffix contains a word that
// indicates a non-type-claim compound name.
func containsSemanticSuffix(suffix string) bool {
	// Add a leading underscore so we catch `container` at the start of the suffix.
	prefixed := "_" + suffix
	for _, word := range semanticSuffixWords {
		if strings.Contains(prefixed, word) {
			return true
		}
	}
	return false
}

// idPrefixToType maps common ID prefixes to the expected view types.
// Only short unambiguous prefixes (tv_, btn_, iv_, etc.) are included.
// Generic words like "text_" are excluded because they often appear as
// feature namespaces in compound IDs (e.g., text_story_post_background
// refers to a "text story" feature, not a TextView).
var idPrefixToType = map[string][]string{
	"tv_":          {"TextView", "AppCompatTextView", "MaterialTextView"},
	"textview_":    {"TextView", "AppCompatTextView", "MaterialTextView"},
	"btn_":         {"Button", "AppCompatButton", "MaterialButton"},
	"button_":      {"Button", "AppCompatButton", "MaterialButton"},
	"iv_":          {"ImageView", "AppCompatImageView", "ShapeableImageView"},
	"img_":         {"ImageView", "AppCompatImageView", "ShapeableImageView"},
	"imageview_":   {"ImageView", "AppCompatImageView", "ShapeableImageView"},
	"rv_":          {"RecyclerView"},
	"et_":          {"EditText", "AppCompatEditText", "TextInputEditText"},
	"edittext_":    {"EditText", "AppCompatEditText", "TextInputEditText"},
	"cb_":          {"CheckBox", "AppCompatCheckBox", "MaterialCheckBox"},
	"checkbox_":    {"CheckBox", "AppCompatCheckBox", "MaterialCheckBox"},
	"rb_":          {"RadioButton", "AppCompatRadioButton"},
	"radiobutton_": {"RadioButton", "AppCompatRadioButton"},
	"sw_":          {"Switch", "SwitchCompat", "SwitchMaterial", "MaterialSwitch"},
	"switch_":      {"Switch", "SwitchCompat", "SwitchMaterial", "MaterialSwitch"},
	"pb_":          {"ProgressBar"},
	"progressbar_": {"ProgressBar"},
	"fab_":         {"FloatingActionButton"},
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *CutPasteIdResourceRule) Confidence() float64 { return 0.75 }

func (r *CutPasteIdResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.ID == "" {
				return
			}
			id := v.ID
			id = strings.TrimPrefix(id, "@+id/")
			id = strings.TrimPrefix(id, "@id/")
			idLower := strings.ToLower(id)

			for prefix, expectedTypes := range idPrefixToType {
				if !strings.HasPrefix(idLower, prefix) {
					continue
				}
				// Skip compound IDs where the prefix is used semantically
				// rather than as a type claim. e.g., `button_strip_container`
				// uses `button_` as a feature prefix, not claiming Button.
				// Rule of thumb: require the ID after the prefix to be either
				// empty or a single word. Multi-word IDs with suffixes like
				// _container, _frame, _barrier, _toggle, _strip, _target,
				// _start, _end, _widget, _progress, _holder indicate a
				// semantic prefix.
				suffix := idLower[len(prefix):]
				if containsSemanticSuffix(suffix) {
					break
				}
				// Check if the actual type matches any expected type.
				actualSimple := v.Type
				if dotIdx := strings.LastIndex(actualSimple, "."); dotIdx >= 0 {
					actualSimple = actualSimple[dotIdx+1:]
				}
				matched := false
				for _, expected := range expectedTypes {
					if actualSimple == expected || v.Type == expected {
						matched = true
						break
					}
				}
				if !matched {
					findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("ID `%s` suggests a `%s` but the view is `%s`. Possible copy-paste mistake.",
							v.ID, expectedTypes[0], v.Type)))
				}
				break // Only check the first matching prefix
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// DuplicateIncludedIdsResource
// ---------------------------------------------------------------------------

// DuplicateIncludedIdsResourceRule detects IDs that appear in multiple layout
// files. Since we don't resolve <include> tags, we flag IDs appearing in 3+
// layouts as likely copy-paste duplicates that may cause runtime ID conflicts
// when layouts include each other.
type DuplicateIncludedIdsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *DuplicateIncludedIdsResourceRule) Confidence() float64 { return 0.75 }

func (r *DuplicateIncludedIdsResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	// Build include graph: layout name -> set of included layout names
	includes := make(map[string]map[string]bool)
	for _, layout := range idx.Layouts {
		layoutIncludes := make(map[string]bool)
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "include" {
				return
			}
			// <include layout="@layout/foo"/>
			if layoutAttr, ok := v.Attributes["layout"]; ok {
				name := strings.TrimPrefix(layoutAttr, "@layout/")
				if name != "" {
					layoutIncludes[name] = true
				}
			}
		})
		if len(layoutIncludes) > 0 {
			includes[layout.Name] = layoutIncludes
		}
	}

	// Build map of ID -> list of layouts containing it (with their file paths)
	type layoutRef struct {
		name string
		path string
		line int
	}
	idToLayouts := make(map[string][]layoutRef)
	for _, layout := range idx.Layouts {
		seen := make(map[string]bool)
		walkViews(layout.RootView, func(v *android.View) {
			if v.ID == "" || v.Type == "include" {
				return
			}
			id := v.ID
			id = strings.TrimPrefix(id, "@+id/")
			id = strings.TrimPrefix(id, "@id/")
			if id != "" && !seen[id] {
				seen[id] = true
				idToLayouts[id] = append(idToLayouts[id], layoutRef{
					name: layout.Name,
					path: layout.FilePath,
					line: v.Line,
				})
			}
		})
	}

	// Only flag IDs where at least two of the layouts have an actual include
	// relationship (direct or transitive) between them.
	var findings []scanner.Finding
	for id, refs := range idToLayouts {
		if len(refs) < 2 {
			continue
		}
		// Find a pair of layouts where one includes the other (transitively)
		var conflictPair [2]layoutRef
		foundConflict := false
		for i := 0; i < len(refs) && !foundConflict; i++ {
			for j := 0; j < len(refs); j++ {
				if i == j {
					continue
				}
				if reachableInclude(includes, refs[i].name, refs[j].name, make(map[string]bool)) {
					conflictPair = [2]layoutRef{refs[i], refs[j]}
					foundConflict = true
					break
				}
			}
		}
		if !foundConflict {
			continue
		}
		findings = append(findings, resourceFinding(conflictPair[0].path, conflictPair[0].line, r.BaseRule,
			fmt.Sprintf("ID `%s` collides: layout `%s` includes `%s` which also defines `%s`. Runtime findViewById will return the wrong view.",
				id, conflictPair[0].name, conflictPair[1].name, id)))
	}
	return findings
}

// reachableInclude returns true if `from` transitively includes `to` via the
// include graph.
func reachableInclude(graph map[string]map[string]bool, from, to string, visited map[string]bool) bool {
	if visited[from] {
		return false
	}
	visited[from] = true
	targets, ok := graph[from]
	if !ok {
		return false
	}
	if targets[to] {
		return true
	}
	for next := range targets {
		if reachableInclude(graph, next, to, visited) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// MissingPrefixResource
// ---------------------------------------------------------------------------

// MissingPrefixResourceRule detects layout attributes that are missing the
// android: namespace prefix. Attributes like "text" or "id" without "android:"
// are silently ignored at runtime.
type MissingPrefixResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// knownAndroidAttrs maps attribute names that require the android: prefix.
var knownAndroidAttrs = map[string]bool{
	"id": true, "text": true, "hint": true,
	"layout_width": true, "layout_height": true,
	"layout_weight": true, "layout_gravity": true, "layout_margin": true,
	"layout_marginLeft": true, "layout_marginRight": true,
	"layout_marginTop": true, "layout_marginBottom": true,
	"layout_marginStart": true, "layout_marginEnd": true,
	"padding": true, "paddingLeft": true, "paddingRight": true,
	"paddingTop": true, "paddingBottom": true,
	"paddingStart": true, "paddingEnd": true,
	"orientation": true, "gravity": true, "visibility": true,
	"background": true, "textColor": true, "textSize": true,
	"src": true, "contentDescription": true, "inputType": true,
	"clickable": true, "focusable": true, "enabled": true,
	"minWidth": true, "minHeight": true, "maxWidth": true, "maxHeight": true,
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *MissingPrefixResourceRule) Confidence() float64 { return 0.75 }

func (r *MissingPrefixResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attrName := range v.Attributes {
				// Skip attributes that are legitimately unprefixed
				if attrName == "style" || strings.HasPrefix(attrName, "xmlns") ||
					strings.HasPrefix(attrName, "class") {
					continue
				}
				// Skip any attribute that already has a prefix (contains :)
				if strings.Contains(attrName, ":") {
					continue
				}
				// Check if this is a known android attribute missing its prefix
				if knownAndroidAttrs[attrName] {
					findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Attribute `%s` missing `android:` prefix. Use `android:%s` instead.",
							attrName, attrName)))
				}
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// NamespaceTypoResource
// ---------------------------------------------------------------------------

// NamespaceTypoResourceRule detects misspelled Android namespace URIs.
// The correct namespace is "http://schemas.android.com/apk/res/android".
type NamespaceTypoResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

const correctAndroidNS = "http://schemas.android.com/apk/res/android"

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *NamespaceTypoResourceRule) Confidence() float64 { return 0.75 }

func (r *NamespaceTypoResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		if layout.RootView == nil {
			continue
		}
		for attrName, attrVal := range layout.RootView.Attributes {
			if !strings.HasPrefix(attrName, "xmlns:") {
				continue
			}
			if attrVal == correctAndroidNS {
				continue
			}
			// Check if the namespace looks like a typo of the Android NS
			if strings.Contains(attrVal, "schemas.android.com") ||
				strings.Contains(attrVal, "schema.android.com") ||
				strings.Contains(attrVal, "shemas.android.com") {
				dist := levenshtein(attrVal, correctAndroidNS)
				if dist > 0 && dist <= 5 {
					findings = append(findings, resourceFinding(layout.FilePath, layout.RootView.Line, r.BaseRule,
						fmt.Sprintf("Possible typo in namespace URI `%s`. Did you mean `%s`?",
							attrVal, correctAndroidNS)))
				}
			}
		}
	}
	return findings
}

// ---------------------------------------------------------------------------
// ResAutoResource
// ---------------------------------------------------------------------------

// ResAutoResourceRule detects hardcoded package namespaces in resource XML.
// Attributes using http://schemas.android.com/apk/res/<package> should use
// http://schemas.android.com/apk/res-auto instead.
type ResAutoResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

const resPackagePrefix = "http://schemas.android.com/apk/res/"

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *ResAutoResourceRule) Confidence() float64 { return 0.75 }

func (r *ResAutoResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attr, val := range v.Attributes {
				// Check xmlns declarations that use a hardcoded package
				if strings.HasPrefix(attr, "xmlns:") && strings.HasPrefix(val, resPackagePrefix) {
					suffix := val[len(resPackagePrefix):]
					if suffix != "" && suffix != "android" {
						findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
							fmt.Sprintf("Namespace declaration `%s=\"%s\"` uses a hardcoded package name. "+
								"Use `http://schemas.android.com/apk/res-auto` instead.",
								attr, val)))
					}
				}
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// UnusedNamespaceResource
// ---------------------------------------------------------------------------

// UnusedNamespaceResourceRule detects xmlns namespace declarations on the root
// view that are never used by any attribute in the layout tree.
type UnusedNamespaceResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *UnusedNamespaceResourceRule) Confidence() float64 { return 0.75 }

func (r *UnusedNamespaceResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		root := layout.RootView
		if root == nil {
			continue
		}
		// Collect xmlns declarations from root
		prefixes := make(map[string]bool)
		for attr := range root.Attributes {
			if strings.HasPrefix(attr, "xmlns:") {
				prefix := strings.TrimPrefix(attr, "xmlns:")
				prefixes[prefix] = true
			}
		}
		if len(prefixes) == 0 {
			continue
		}
		// Walk entire tree and mark prefixes as used
		used := make(map[string]bool)
		walkViews(root, func(v *android.View) {
			for attr := range v.Attributes {
				if strings.HasPrefix(attr, "xmlns:") {
					continue
				}
				if colonIdx := strings.Index(attr, ":"); colonIdx > 0 {
					prefix := attr[:colonIdx]
					if prefixes[prefix] {
						used[prefix] = true
					}
				}
			}
		})
		// Report unused
		for attr := range root.Attributes {
			if !strings.HasPrefix(attr, "xmlns:") {
				continue
			}
			prefix := strings.TrimPrefix(attr, "xmlns:")
			if !used[prefix] {
				findings = append(findings, resourceFinding(layout.FilePath, root.Line, r.BaseRule,
					fmt.Sprintf("Unused namespace declaration `xmlns:%s`. "+
						"No attributes in the layout use the `%s:` prefix.",
						prefix, prefix)))
			}
		}
	}
	return findings
}

// ---------------------------------------------------------------------------
// IllegalResourceRefResource
// ---------------------------------------------------------------------------

// IllegalResourceRefResourceRule detects malformed resource references in XML
// attributes. Valid references must match @[+][type/]name or @android:type/name.
type IllegalResourceRefResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *IllegalResourceRefResourceRule) Confidence() float64 { return 0.75 }

func (r *IllegalResourceRefResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attr, val := range v.Attributes {
				if !strings.HasPrefix(val, "@") {
					continue
				}
				// Skip tool and style references
				if strings.HasPrefix(attr, "tools:") {
					continue
				}
				// Skip @null
				if val == "@null" {
					continue
				}
				// Strip leading @[+]
				ref := val[1:]
				if strings.HasPrefix(ref, "+") {
					ref = ref[1:]
				}
				// Must contain a / separating type from name
				if !strings.Contains(ref, "/") {
					findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Malformed resource reference `%s` in attribute `%s`. "+
							"Expected format `@[type]/name`.",
							val, attr)))
					continue
				}
				parts := strings.SplitN(ref, "/", 2)
				resType := parts[0]
				resName := parts[1]
				// Type part (possibly prefixed with "android:")
				resType = strings.TrimPrefix(resType, "android:")
				if resType == "" {
					findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Malformed resource reference `%s` in attribute `%s`. "+
							"Missing resource type before `/`.",
							val, attr)))
					continue
				}
				if resName == "" {
					findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Malformed resource reference `%s` in attribute `%s`. "+
							"Missing resource name after `/`.",
							val, attr)))
				}
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// WrongCaseResource
// ---------------------------------------------------------------------------

// WrongCaseResourceRule detects view tags with incorrect capitalization.
// Common mistakes include "Textview" instead of "TextView", "linearlayout"
// instead of "LinearLayout", etc.
type WrongCaseResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// wrongCaseFixes maps lowercase view names to their correct casing.
var wrongCaseFixes = map[string]string{
	"textview":                  "TextView",
	"imageview":                 "ImageView",
	"linearlayout":              "LinearLayout",
	"relativelayout":            "RelativeLayout",
	"framelayout":               "FrameLayout",
	"scrollview":                "ScrollView",
	"listview":                  "ListView",
	"gridview":                  "GridView",
	"webview":                   "WebView",
	"edittext":                  "EditText",
	"imagebutton":               "ImageButton",
	"checkbox":                  "CheckBox",
	"radiobutton":               "RadioButton",
	"radiogroup":                "RadioGroup",
	"switch":                    "Switch",
	"togglebutton":              "ToggleButton",
	"progressbar":               "ProgressBar",
	"seekbar":                   "SeekBar",
	"ratingbar":                 "RatingBar",
	"videoview":                 "VideoView",
	"viewflipper":               "ViewFlipper",
	"viewswitcher":              "ViewSwitcher",
	"horizontalscrollview":      "HorizontalScrollView",
	"tablelayout":               "TableLayout",
	"tablerow":                  "TableRow",
	"viewstub":                  "ViewStub",
	"surfaceview":               "SurfaceView",
	"textureview":               "TextureView",
	"constraintlayout":          "ConstraintLayout",
	"coordinatorlayout":         "CoordinatorLayout",
	"recyclerview":              "RecyclerView",
	"cardview":                  "CardView",
	"nestedscrollview":          "NestedScrollView",
	"autocompletetextview":      "AutoCompleteTextView",
	"multiautocompletetextview": "MultiAutoCompleteTextView",
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *WrongCaseResourceRule) Confidence() float64 { return 0.75 }

func (r *WrongCaseResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			// Skip fully-qualified names (contain a dot)
			if strings.Contains(v.Type, ".") {
				return
			}
			lower := strings.ToLower(v.Type)
			correct, ok := wrongCaseFixes[lower]
			if !ok {
				return
			}
			if v.Type != correct {
				findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("View tag `%s` has wrong capitalization. Use `%s` instead.",
						v.Type, correct)))
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// WrongFolderResource
// ---------------------------------------------------------------------------

// WrongFolderResourceRule detects resources that reference drawables not found
// in any drawable or mipmap directory.
type WrongFolderResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *WrongFolderResourceRule) Confidence() float64 { return 0.75 }

func (r *WrongFolderResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	if len(idx.Drawables) == 0 {
		return nil
	}
	drawableSet := make(map[string]bool, len(idx.Drawables))
	for _, d := range idx.Drawables {
		drawableSet[d] = true
	}
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attr, val := range v.Attributes {
				if !strings.HasPrefix(val, "@drawable/") {
					continue
				}
				name := strings.TrimPrefix(val, "@drawable/")
				if name == "" {
					continue
				}
				if !drawableSet[name] {
					findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Resource reference `%s` in attribute `%s` not found in any drawable or mipmap directory.",
							val, attr)))
				}
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// InvalidResourceFolderResource
// ---------------------------------------------------------------------------

// InvalidResourceFolderResourceRule checks layout file paths for invalid
// resource folder prefixes. Valid prefixes are: layout, drawable, mipmap,
// values, raw, xml, anim, animator, color, menu, font, navigation, transition.
type InvalidResourceFolderResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// validResFolderPrefixes lists the valid Android resource folder name prefixes.
var validResFolderPrefixes = []string{
	"layout", "drawable", "mipmap", "values", "raw", "xml",
	"anim", "animator", "color", "menu", "font", "navigation", "transition",
}

func isValidResFolderName(folder string) bool {
	for _, prefix := range validResFolderPrefixes {
		if folder == prefix || strings.HasPrefix(folder, prefix+"-") {
			return true
		}
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *InvalidResourceFolderResourceRule) Confidence() float64 { return 0.75 }

func (r *InvalidResourceFolderResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		path := layout.FilePath
		// Extract the folder name from the path: look for res/<folder>/file.xml
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
		if !isValidResFolderName(folder) {
			findings = append(findings, resourceFinding(layout.FilePath, 1, r.BaseRule,
				fmt.Sprintf("Invalid resource folder name `%s`. "+
					"Expected one of: layout, drawable, mipmap, values, raw, xml, anim, animator, color, menu, font, navigation, transition.",
					folder)))
		}
	}
	return findings
}

// ---------------------------------------------------------------------------
// AppCompatResource
// ---------------------------------------------------------------------------

// AppCompatResourceRule detects menu items using android:showAsAction instead
// of app:showAsAction. When using AppCompat, the app: namespace is required.
type AppCompatResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *AppCompatResourceRule) Confidence() float64 { return 0.75 }

func (r *AppCompatResourceRule) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:showAsAction"] != "" {
				findings = append(findings, resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` uses `android:showAsAction` instead of `app:showAsAction`. Use `app:showAsAction` for AppCompat compatibility.", v.Type)))
			}
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
