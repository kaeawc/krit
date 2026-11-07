package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

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
func (r *IllegalResourceRefResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *IllegalResourceRefResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViews(layout.RootView, func(v *android.View) {
			for attr, val := range v.Attributes {
				if !strings.HasPrefix(val, "@") {
					continue
				}
				// Skip tool and style references
				if strings.HasPrefix(attr, "tools:") {
					continue
				}
				// Skip @null and data binding expressions (`@{...}`, `@={...}`).
				if val == "@null" || strings.HasPrefix(val, "@{") || strings.HasPrefix(val, "@={") {
					continue
				}
				// Strip leading @[+]
				ref := val[1:]
				ref = strings.TrimPrefix(ref, "+")
				// Must contain a / separating type from name
				if !strings.Contains(ref, "/") {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
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
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
						fmt.Sprintf("Malformed resource reference `%s` in attribute `%s`. "+
							"Missing resource type before `/`.",
							val, attr)))
					continue
				}
				if resName == "" {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
						fmt.Sprintf("Malformed resource reference `%s` in attribute `%s`. "+
							"Missing resource name after `/`.",
							val, attr)))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WrongCaseResource
// ---------------------------------------------------------------------------

// WrongCaseResourceRule mirrors Lint's WrongCaseDetector: the layout tags that must start with a
// lowercase letter (fragment, requestFocus, include, merge) written with a capital.
type WrongCaseResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// wrongCaseTags maps the capitalized spelling Lint looks for to the correct tag.
var wrongCaseTags = map[string]string{
	"Fragment": "fragment", "RequestFocus": "requestFocus", "Include": "include", "Merge": "merge",
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *WrongCaseResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WrongCaseResourceRule) check(ctx *api.Context) {
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"WrongCase"}, func(v *android.View) {
			if correct, ok := wrongCaseTags[v.Type]; ok {
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
					fmt.Sprintf("Invalid tag `<%s>`; should be `<%s>`", v.Type, correct)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WrongFolderResource
// ---------------------------------------------------------------------------

// WrongFolderResourceRule mirrors Lint's WrongLocationDetector: a layout folder file whose root element
// is <resources> belongs in a values folder.
type WrongFolderResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *WrongFolderResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WrongFolderResourceRule) check(ctx *api.Context) {
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		root := layout.RootView
		if root == nil || root.Type != "resources" || !strings.HasPrefix(filepath.Base(filepath.Dir(layout.FilePath)), "layout") {
			continue
		}
		if resourceToolsIgnoreMatches(root, "WrongFolder") {
			continue
		}
		ctx.Emit(baseFindingAt(layout.FilePath, root.Line, root.Col, r.BaseRule,
			"This file should be placed in a `values`/ folder, not a `layout`/ folder"))
	}
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
	"anim", "animator", "color", "menu", "font", "navigation", "transition", "interpolator",
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
func (r *InvalidResourceFolderResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InvalidResourceFolderResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	reported := make(map[string]bool)
	report := func(path, folder string) {
		if reported[path] || isValidResFolderName(folder) {
			return
		}
		reported[path] = true
		ctx.Emit(baseFinding(path, 1, r.BaseRule,
			fmt.Sprintf("Invalid resource folder name `%s`. "+
				"Expected one of: layout, drawable, mipmap, values, raw, xml, anim, animator, color, menu, font, navigation, transition, interpolator.",
				folder)))
	}
	for _, layout := range idx.AllLayouts() {
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
		report(layout.FilePath, folder)
	}
	// Folders with an unknown type are never scanned for layouts, so check the directory names too.
	for _, resRoot := range resourceRootsFromIndex(idx) {
		entries, err := os.ReadDir(resRoot)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				report(filepath.Join(resRoot, e.Name()), e.Name())
			}
		}
	}
}
