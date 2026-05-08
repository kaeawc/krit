package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// StringContainsHTMLWithoutCDATARule flags <string> resources whose value
// contains literal `<` or `>` markup that is neither wrapped in a
// `<![CDATA[...]]>` section nor entity-escaped (`&lt;`, `&gt;`).
type StringContainsHTMLWithoutCDATARule struct {
	ValuesStringsResourceBase
	AndroidRule
}

func (r *StringContainsHTMLWithoutCDATARule) Confidence() float64 { return 0.9 }

func (r *StringContainsHTMLWithoutCDATARule) check(ctx *api.Context) {
	if ctx.ResourceIndex == nil {
		return
	}
	for _, resRoot := range resourceRootsFromIndex(ctx.ResourceIndex) {
		r.checkResourceRoot(ctx, resRoot)
	}
}

func (r *StringContainsHTMLWithoutCDATARule) checkResourceRoot(ctx *api.Context, resRoot string) {
	entries, err := os.ReadDir(resRoot)
	if err != nil {
		return
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "values" {
			dirs = append(dirs, name)
			continue
		}
		if _, ok := localeTagFromValuesDir(name); ok {
			dirs = append(dirs, name)
		}
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		forEachStringInValuesDir(filepath.Join(resRoot, dir), func(path string, s *android.XMLNode) {
			name := s.Attr("name")
			if name == "" {
				return
			}
			if len(s.Children) == 0 {
				return
			}
			ctx.Emit(resourceFinding(path, s.Line, r.BaseRule,
				fmt.Sprintf("String `%s` contains literal HTML markup. Wrap the value in `<![CDATA[...]]>` or entity-escape `<` and `>` (`&lt;`, `&gt;`).",
					name)))
		})
	}
}
