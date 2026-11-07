package rules

// Android Resource XML rules: ID/reference/namespace rules.

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// DuplicateIdsResource
// ---------------------------------------------------------------------------

// DuplicateIDsResourceRule detects the same android:id used more than once
// in a single layout file.
type DuplicateIDsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *DuplicateIDsResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DuplicateIDsResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		seen := make(map[string]int) // id -> first line
		walkViewsUnlessIgnored(layout.RootView, []string{"DuplicateIds"}, func(v *android.View) {
			// Lint only compares @+id/ definitions; @id/ and @android:id/ are references.
			if !strings.HasPrefix(v.ID, "@+id/") {
				return
			}
			if _, ok := seen[v.ID]; ok {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:id", r.BaseRule,
					fmt.Sprintf("Duplicate id `%s`, already defined earlier in this layout", v.ID)))
			} else {
				seen[v.ID] = v.Line
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InvalidIdResource
// ---------------------------------------------------------------------------

// InvalidIDResourceRule detects malformed android:id values. Valid IDs match
// @+id/name or @id/name where name contains only [a-zA-Z0-9_.].
type InvalidIDResourceRule struct {
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
		rest = strings.TrimPrefix(rest, "@+id/")
	} else if strings.HasPrefix(rest, "@id/") {
		rest = strings.TrimPrefix(rest, "@id/")
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
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' && c != '.' {
			return false
		}
	}
	return true
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *InvalidIDResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InvalidIDResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"InvalidId"}, func(v *android.View) {
			id := v.Attributes["android:id"]
			if id == "" {
				return
			}
			if !isValidAndroidID(id) {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:id", r.BaseRule,
					fmt.Sprintf("Invalid `android:id` value `%s`. IDs must be `@+id/name` or `@id/name` with alphanumeric/underscore names.",
						id)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MissingIdResource
// ---------------------------------------------------------------------------

// MissingIDResourceRule detects <fragment> and FragmentContainerView tags that
// lack an android:id or android:tag. The fragment manager needs one to restore
// the fragment after a configuration change. An <include> needs no id.
type MissingIDResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *MissingIDResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MissingIDResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"MissingId"}, func(v *android.View) {
			switch {
			case v.Type == "fragment":
			case frameworkViewName(v.Type) == "FragmentContainerView" && v.Attributes["android:name"] != "":
			default:
				return
			}
			if v.ID != "" {
				return
			}
			// Also accept android:tag as alternative
			if v.Attributes["android:tag"] != "" {
				return
			}
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
				fmt.Sprintf("<%s> should specify an `android:id` or `android:tag` so the fragment can be restored.",
					v.Type)))
		})
	}
}

// ---------------------------------------------------------------------------
// IdTypePrefixMismatchResource
// ---------------------------------------------------------------------------

// IDTypePrefixMismatchResourceRule detects likely copy-paste ID mistakes where a view's
// android:id name contains a prefix/hint that suggests a different view type.
// For example, `@+id/textview_name` on a Button suggests copy-paste from a
// TextView.
//
// This is a krit-only layout check. It was named CutPasteIdResource, which is
// the layout-side name of Android Lint's CutPasteId; that check is about
// repeated findViewById lookups in source (see CutPasteIDRule). The old id stays
// a suppression and config alias.
type IDTypePrefixMismatchResourceRule struct {
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
	// Label-style suffixes describe a companion, not the widget type.
	"_label", "_description", "_desc", "_hint", "_title",
}

// containsSemanticSuffix returns true if the ID suffix contains a word that
// indicates a non-type-claim compound name.
func containsSemanticSuffix(suffix string) bool {
	// Add a leading underscore so we catch `container` at the start of the suffix.
	prefixed := "_" + suffix
	// An id ending in a label word names text about the widget, not the widget itself.
	for _, word := range []string{"_text", "_label", "_title", "_description", "_hint"} {
		// A bare `tv_text` still claims a type, so the label word needs a qualifier before it.
		if strings.HasSuffix(prefixed, word) && prefixed != word {
			return true
		}
	}
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
	"tv_":       {"TextView", "AppCompatTextView", "MaterialTextView"},
	"textview_": {"TextView", "AppCompatTextView", "MaterialTextView"},
	"btn_":      {"Button", "AppCompatButton", "MaterialButton"},
	// `button_` is a role prefix (it names what the view does), so it is not listed.
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

// cutPasteContradictions lists, per id prefix, the exact framework widget tags that clearly belong to a
// different family. Only these pairs are reported: a layout has no source access to see how the id is used,
// and a prefix alone is weak evidence. TextView and clickable views are never listed, since text acts as a
// button or label often.
var cutPasteContradictions = map[string][]string{
	"btn_": {"ImageView", "RecyclerView", "ProgressBar", "EditText"},
	"tv_":  {"ImageView", "RecyclerView", "ProgressBar"}, "textview_": {"ImageView", "RecyclerView", "ProgressBar"},
	"iv_":        {"Button", "EditText", "RecyclerView", "ProgressBar", "CheckBox", "RadioButton", "Switch"},
	"img_":       {"Button", "EditText", "RecyclerView", "ProgressBar", "CheckBox", "RadioButton", "Switch"},
	"imageview_": {"Button", "EditText", "RecyclerView", "ProgressBar", "CheckBox", "RadioButton", "Switch"},
	"rv_":        {"Button", "ImageView", "EditText", "ProgressBar", "CheckBox"},
	"et_":        {"Button", "ImageView", "RecyclerView", "ProgressBar", "CheckBox"}, "edittext_": {"Button", "ImageView", "RecyclerView", "ProgressBar", "CheckBox"},
	"cb_": {"ImageView", "RecyclerView", "EditText", "ProgressBar"}, "checkbox_": {"ImageView", "RecyclerView", "EditText", "ProgressBar"},
	"rb_": {"ImageView", "RecyclerView", "EditText", "ProgressBar"}, "radiobutton_": {"ImageView", "RecyclerView", "EditText", "ProgressBar"},
	"sw_": {"ImageView", "RecyclerView", "EditText", "ProgressBar"}, "switch_": {"ImageView", "RecyclerView", "EditText", "ProgressBar"},
	"pb_": {"Button", "ImageView", "EditText", "RecyclerView", "CheckBox"}, "progressbar_": {"Button", "ImageView", "EditText", "RecyclerView", "CheckBox"},
}

// cutPasteContradicts reports whether the view is an exact framework widget from a family the id prefix rules out.
func cutPasteContradicts(prefix string, v *android.View) bool {
	if v.Attributes["android:clickable"] == "true" || v.Attributes["android:onClick"] != "" {
		return false
	}
	tag := strings.TrimPrefix(v.Type, "android.widget.")
	for _, bad := range cutPasteContradictions[prefix] {
		if tag == bad {
			return true
		}
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *IDTypePrefixMismatchResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *IDTypePrefixMismatchResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
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
				if cutPasteContradicts(prefix, v) {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:id", r.BaseRule,
						fmt.Sprintf("ID `%s` suggests a `%s` but the view is `%s`. Possible copy-paste mistake.",
							v.ID, expectedTypes[0], v.Type)))
				}
				break // Only check the first matching prefix
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DuplicateIncludedIdsResource
// ---------------------------------------------------------------------------

// DuplicateIncludedIDsResourceRule mirrors Lint's DuplicateIdDetector include
// check: an id that a layout defines and also receives through one or more
// <include> chains. Each layout that defines the id is listed once, and the
// finding anchors on the last <include> of the layout that carries the duplicate.
type DuplicateIncludedIDsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *DuplicateIncludedIDsResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func layoutDisplayPath(l *android.Layout) string {
	return layoutResourceDir(l.FilePath) + "/" + filepath.Base(l.FilePath)
}

// definedNewIDs lists the distinct @+id/ definitions of a layout in document order.
func definedNewIDs(l *android.Layout) []string {
	var ids []string
	seen := map[string]bool{}
	walkViews(l.RootView, func(v *android.View) {
		if v.ID == "" || v.Type == "include" || !strings.HasPrefix(v.ID, "@+id/") || seen[v.ID] {
			return
		}
		seen[v.ID] = true
		ids = append(ids, v.ID)
	})
	return ids
}

func includeTarget(v *android.View) (string, bool) {
	if v.Type != "include" {
		return "", false
	}
	name := strings.TrimPrefix(v.Attributes["layout"], "@layout/")
	return name, name != "" && name != v.Attributes["layout"]
}

func (r *DuplicateIncludedIDsResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	// An include resolves inside the including layout's qualifier set (layout-land includes layout-land/x),
	// then falls back to the default layout, as the resource system does for the active configuration.
	variants := make(map[string]map[string]*android.Layout) // layout name -> resource dir -> layout
	for _, layout := range idx.AllLayouts() {
		dir := layoutResourceDir(layout.FilePath)
		if variants[layout.Name] == nil {
			variants[layout.Name] = make(map[string]*android.Layout)
		}
		variants[layout.Name][dir] = layout
	}
	resolve := func(dir, name string) *android.Layout {
		if l := variants[name][dir]; l != nil {
			return l
		}
		return variants[name]["layout"]
	}

	// Lint ignores an <include>'s own android:id here, so it never hides the included root's id.
	type includeEdge struct {
		view   *android.View
		target *android.Layout
	}

	for _, top := range idx.AllLayouts() {
		dir := layoutResourceDir(top.FilePath)
		includesOf := func(l *android.Layout) []includeEdge {
			var edges []includeEdge
			walkViews(l.RootView, func(v *android.View) {
				if name, ok := includeTarget(v); ok {
					if next := resolve(dir, name); next != nil {
						edges = append(edges, includeEdge{v, next})
					}
				}
			})
			return edges
		}

		// Like Lint, each layout is walked once per top layout, so a layout included several times counts once.
		visited := map[*android.Layout]bool{top: true}
		var subtreeIDs func(l *android.Layout) []string
		subtreeIDs = func(l *android.Layout) []string {
			visited[l] = true
			ids := definedNewIDs(l)
			seen := map[string]bool{}
			for _, id := range ids {
				seen[id] = true
			}
			for _, e := range includesOf(l) {
				if visited[e.target] {
					continue
				}
				for _, id := range subtreeIDs(e.target) {
					if !seen[id] {
						seen[id] = true
						ids = append(ids, id)
					}
				}
			}
			return ids
		}
		// definitions lists each layout reachable from the top that defines id, once, by its first include path.
		definitions := func(id string) []string {
			var found []string
			seen := map[*android.Layout]bool{}
			var walk func(l *android.Layout, stack []string)
			walk = func(l *android.Layout, stack []string) {
				seen[l] = true
				stack = append(append([]string(nil), stack...), layoutDisplayPath(l))
				for _, d := range definedNewIDs(l) {
					if d == id {
						found = append(found, strings.Join(stack, " => ")+" defines "+id)
						break
					}
				}
				for _, e := range includesOf(l) {
					if !seen[e.target] {
						walk(e.target, stack)
					}
				}
			}
			walk(top, nil)
			// Lint orders these by length, then alphabetically.
			sort.SliceStable(found, func(i, j int) bool {
				if len(found[i]) != len(found[j]) {
					return len(found[i]) < len(found[j])
				}
				return found[i] < found[j]
			})
			return found
		}
		// lastInclude is the include Lint anchors on: the last <include> in the top layout that names the target.
		lastInclude := func(name string) *android.View {
			var last *android.View
			walkViews(top.RootView, func(v *android.View) {
				if n, ok := includeTarget(v); ok && n == name {
					last = v
				}
			})
			return last
		}

		ids := map[string]bool{}
		for _, id := range definedNewIDs(top) {
			ids[id] = true
		}
		for _, e := range includesOf(top) {
			if visited[e.target] {
				continue
			}
			for _, id := range subtreeIDs(e.target) {
				if !ids[id] {
					ids[id] = true
					continue
				}
				anchor := lastInclude(e.target.Name)
				if anchor == nil || includeIgnored(top.RootView, anchor, "DuplicateIncludedIds") {
					continue
				}
				ctx.Emit(baseFindingAt(top.FilePath, anchor.Line, anchor.Col, r.BaseRule,
					fmt.Sprintf("Duplicate id %s, defined or included multiple times in %s: [%s]",
						id, layoutDisplayPath(top), strings.Join(definitions(id), ", "))))
			}
		}
	}
}

// includeIgnored reports whether target or one of its ancestors under root carries a tools:ignore for the issue.
func includeIgnored(root, target *android.View, issue string) bool {
	var path []*android.View
	var find func(v *android.View) bool
	find = func(v *android.View) bool {
		path = append(path, v)
		if v == target {
			return true
		}
		for _, c := range v.Children {
			if find(c) {
				return true
			}
		}
		path = path[:len(path)-1]
		return false
	}
	if !find(root) {
		return false
	}
	for _, v := range path {
		if resourceToolsIgnoreMatches(v, issue) {
			return true
		}
	}
	return false
}

// layoutResourceDir returns the resource directory name of a layout file
// ("layout", "layout-land", ...), which identifies its qualifier set.
func layoutResourceDir(path string) string {
	return filepath.Base(filepath.Dir(path))
}

// ---------------------------------------------------------------------------
// UnknownId
// ---------------------------------------------------------------------------

// UnknownIDResourceRule detects a layout constraint that references an id with
// the pure `@id/name` form when that id is not declared anywhere in the
// project. Such a reference is a dangling pointer — typically a typo or an id
// that was renamed or deleted without updating its references.
//
// To stay false-positive free this rule is deliberately conservative:
//   - It only flags the pure reference form `@id/name`. The `@+id/name` form
//     creates the id on demand, so it is never a dangling reference and is
//     left alone.
//   - Framework ids (`@android:id/...`) are always defined by the platform and
//     are skipped.
//   - The "declared" set is the union of every id declaration source: every
//     `android:id`, every `@+id/...` appearing in any attribute across all
//     layouts, and every `<item type="id" name="..."/>` in values resources.
type UnknownIDResourceRule struct {
	ResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: ids are resolved from
// the parsed, project-wide resource index rather than a text scan, but the
// declared-id set depends on complete resource coverage. Classified per
// roadmap/17.
func (r *UnknownIDResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// lintDefinedIDs gathers the ids Lint's WrongIdDetector treats as defined: android:id values in
// any layout and `<item type="id">` entries. An `@+id/` in a reference attribute does not define an id.
func lintDefinedIDs(idx *android.ResourceIndex) map[string]bool {
	defined := make(map[string]bool, len(idx.IDs))
	for id := range idx.IDs {
		defined[id] = true
	}
	for _, item := range idx.AliasItems {
		if item.Type == "id" && item.Name != "" {
			defined[item.Name] = true
		}
	}
	for _, layout := range idx.AllLayouts() {
		walkViews(layout.RootView, func(v *android.View) {
			id := strings.TrimSpace(v.Attributes["android:id"])
			id = strings.TrimPrefix(strings.TrimPrefix(id, "@+id/"), "@id/")
			if id != "" {
				defined[id] = true
			}
		})
	}
	return defined
}

// unknownIDRelativeParent reports whether tag is a layout whose children reference sibling ids.
func unknownIDRelativeParent(tag string) bool {
	return tag == "RelativeLayout" || tag == "android.support.percent.PercentRelativeLayout" ||
		strings.HasSuffix(tag, "ConstraintLayout")
}

// lintIDSuggestions mirrors WrongIdDetector.getSpellingSuggestions: the closest known ids within an
// edit distance of 1 (2 for names of four or more characters), nearest group first.
func lintIDSuggestions(name string, defined map[string]bool) []string {
	maxDistance := 1
	if len(name) >= 4 {
		maxDistance = 2
	}
	groups := make([][]string, maxDistance)
	candidates := make([]string, 0, len(defined))
	for id := range defined {
		candidates = append(candidates, id)
	}
	sort.Strings(candidates)
	for i, id := range candidates {
		if i > 100 {
			break
		}
		if d := levenshtein(name, id); d < maxDistance {
			groups[d] = append(groups[d], id)
		}
	}
	for _, g := range groups {
		if len(g) > 0 {
			return g
		}
	}
	return nil
}

func (r *UnknownIDResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	defined := lintDefinedIDs(idx)
	for _, layout := range idx.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"UnknownId"}, func(v, parent *android.View) {
			if parent == nil || !unknownIDRelativeParent(parent.Type) {
				return
			}
			attrs := make([]string, 0, len(v.Attributes))
			for attr := range v.Attributes {
				attrs = append(attrs, attr)
			}
			sort.Strings(attrs)
			for _, attr := range attrs {
				local := attr[strings.LastIndex(attr, ":")+1:]
				if !strings.HasPrefix(local, "layout_") {
					continue
				}
				ref := strings.TrimSpace(v.Attributes[attr])
				name := strings.TrimPrefix(strings.TrimPrefix(ref, "@+id/"), "@id/")
				if name == ref || name == "" || defined[name] {
					continue
				}
				msg := "The id \"`" + name + "`\" is not defined anywhere."
				switch sugg := lintIDSuggestions(name, defined); len(sugg) {
				case 0:
				case 1:
					msg += " Did you mean " + sugg[0] + " ?"
				default:
					msg += " Did you mean one of {" + strings.Join(sugg, ", ") + "} ?"
				}
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule, msg))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

// layoutIDs returns the android:id names defined by any element of the layout file.
func layoutIDs(root *android.View) map[string]bool {
	ids := map[string]bool{}
	walkViews(root, func(v *android.View) {
		id := strings.TrimSpace(v.Attributes["android:id"])
		id = strings.TrimPrefix(strings.TrimPrefix(id, "@+id/"), "@id/")
		if id != "" {
			ids[id] = true
		}
	})
	return ids
}

// hasUnresolvedInclude reports whether a relative layout holds an id-less <include> of a layout the
// project does not contain. Lint cannot tell which ids such an include adds and skips the file.
func hasUnresolvedInclude(idx *android.ResourceIndex, root *android.View) bool {
	unresolved := false
	walkViews(root, func(v *android.View) {
		if !unknownIDRelativeParent(v.Type) {
			return
		}
		for _, child := range v.Children {
			if child.Type != "include" || strings.TrimSpace(child.Attributes["android:id"]) != "" {
				continue
			}
			name := strings.TrimPrefix(strings.TrimSpace(child.Attributes["layout"]), "@layout/")
			if idx.Layouts[name] == nil {
				unresolved = true
			}
		}
	})
	return unresolved
}

// check mirrors WrongIdDetector's UnknownIdInLayout: a layout_ attribute of a child of a relative or
// constraint layout that uses `@+id/name` for an id some other layout defines but this file does not.
func (r *UnknownIDInLayoutRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	defined := lintDefinedIDs(idx)
	for _, layout := range idx.AllLayouts() {
		if hasUnresolvedInclude(idx, layout.RootView) {
			continue
		}
		inFile := layoutIDs(layout.RootView)
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"UnknownIdInLayout"}, func(v, parent *android.View) {
			if parent == nil || !unknownIDRelativeParent(parent.Type) {
				return
			}
			attrs := make([]string, 0, len(v.Attributes))
			for attr := range v.Attributes {
				attrs = append(attrs, attr)
			}
			sort.Strings(attrs)
			for _, attr := range attrs {
				if !strings.HasPrefix(attr[strings.LastIndex(attr, ":")+1:], "layout_") {
					continue
				}
				ref := strings.TrimSpace(v.Attributes[attr])
				if !strings.HasPrefix(ref, "@+id/") {
					continue
				}
				name := ref[len("@+id/"):]
				if name == "" || inFile[name] || !defined[name] {
					continue
				}
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
					"The id \"`"+name+"`\" is not referring to any views in this layout"))
			}
		})
	}
}
