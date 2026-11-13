package rules

import (
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// RedundantNamespaceResourceRule mirrors Lint's RedundantNamespace: an xmlns declaration below the root element that
// repeats the root's declaration of the same prefix.
type RedundantNamespaceResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the check compares two declarations in one file.
func (r *RedundantNamespaceResourceRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *RedundantNamespaceResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	layouts := append(idx.AllLayouts(), idx.AllMenus()...)
	for _, layout := range layouts {
		root := layout.RootView
		if root == nil {
			continue
		}
		for _, child := range root.Children {
			r.checkChild(ctx, layout.FilePath, root, child, false)
		}
	}
}

func (r *RedundantNamespaceResourceRule) checkChild(ctx *api.Context, path string, root, v *android.View, ignored bool) {
	if v == nil {
		return
	}
	ignored = ignored || resourceToolsIgnoreMatches(v, "RedundantNamespace")
	if !ignored {
		names := make([]string, 0, len(v.Attributes))
		for attr := range v.Attributes {
			if strings.HasPrefix(attr, "xmlns:") {
				names = append(names, attr)
			}
		}
		sort.Strings(names)
		for _, attr := range names {
			if val, ok := root.Attributes[attr]; ok && val == v.Attributes[attr] {
				ctx.Emit(baseFindingAtAttr(path, v, attr, r.BaseRule, "This namespace declaration is redundant"))
			}
		}
	}
	for _, c := range v.Children {
		r.checkChild(ctx, path, root, c, ignored)
	}
}
