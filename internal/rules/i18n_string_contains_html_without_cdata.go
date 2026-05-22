package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// StringContainsHTMLWithoutCDATARule flags <string> resources whose value
// contains literal `<` or `>` markup that is neither wrapped in a
// `<![CDATA[...]]>` section nor entity-escaped (`&lt;`, `&gt;`).
//
// Android i18n strings commonly contain non-HTML child elements that the
// platform parses as placeholders or annotations rather than markup, so the
// rule must distinguish those from real unescaped HTML:
//
//   - <xliff:g> wraps translator placeholders for format args and is the
//     canonical Android i18n placeholder shape.
//   - <annotation> spans are an Android Spanned primitive (parsed into
//     SpannedString.Annotation by Resources.getText).
//
// Only children whose tag is a known HTML formatting tag (a, b, i, u, em,
// strong, big, small, sub, sup, tt, font, br) count as evidence of literal
// HTML markup. Strings wrapped in a CDATA section have no element children
// and parse to empty Text once the parser strips the CDATA delimiters, so
// they are inherently clean here.
type StringContainsHTMLWithoutCDATARule struct {
	ValuesStringsResourceBase
	AndroidRule
}

func (r *StringContainsHTMLWithoutCDATARule) Confidence() float64 { return api.ConfidenceHigher }

// htmlMarkupChildTags is the set of child element tags that, when present
// inside a <string> resource without being wrapped in <![CDATA[...]]>,
// indicate literal HTML markup. It mirrors htmlInlineTags from i18n_markup.go
// and adds <a>, the canonical anchor for `Click <a href="...">here</a>`.
var htmlMarkupChildTags = map[string]bool{
	"a": true, "b": true, "i": true, "u": true,
	"em": true, "strong": true,
	"big": true, "small": true, "sub": true, "sup": true,
	"tt": true, "font": true, "br": true,
}

// hasUnescapedHTMLChild returns true when at least one direct child element
// of the <string> looks like an HTML formatting tag rather than a known-safe
// Android i18n primitive (xliff:g placeholder, annotation span, etc.).
func hasUnescapedHTMLChild(s *android.XMLNode) bool {
	for _, child := range s.Children {
		tag := strings.ToLower(child.Tag)
		if idx := strings.Index(tag, ":"); idx >= 0 {
			// Namespaced tags such as xliff:g are placeholders, not HTML.
			continue
		}
		if htmlMarkupChildTags[tag] {
			return true
		}
	}
	return false
}

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
			if !hasUnescapedHTMLChild(s) {
				return
			}
			ctx.Emit(baseFinding(path, s.Line, r.BaseRule,
				fmt.Sprintf("String `%s` contains literal HTML markup. Wrap the value in `<![CDATA[...]]>` or entity-escape `<` and `>` (`&lt;`, `&gt;`).",
					name)))
		})
	}
}
