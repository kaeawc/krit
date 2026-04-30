package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// StringResourcePlaceholderOrderRule flags translation variants that drop
// positional format syntax (`%1$s`, `%2$s`) when the default string uses it.
// Without positional indexes, a translator cannot reorder arguments to match
// target-language word order, and the runtime mapping is silently lost.
type StringResourcePlaceholderOrderRule struct {
	ValuesStringsResourceBase
	AndroidRule
}

func (r *StringResourcePlaceholderOrderRule) Confidence() float64 { return 0.9 }

func (r *StringResourcePlaceholderOrderRule) check(ctx *v2.Context) {
	if ctx.ResourceIndex == nil {
		return
	}
	for _, resRoot := range resourceRootsFromIndex(ctx.ResourceIndex) {
		defaults := make(map[string]formatPlaceholderInfo)
		forEachStringInValuesDir(filepath.Join(resRoot, "values"), func(_ string, s *android.XMLNode) {
			name := s.Attr("name")
			if name == "" || strings.EqualFold(s.Attr("formatted"), "false") {
				return
			}
			info := analyzeFormatPlaceholders(s.Text)
			if info.total == 0 {
				return
			}
			defaults[name] = info
		})
		if len(defaults) == 0 {
			continue
		}
		entries, err := os.ReadDir(resRoot)
		if err != nil {
			continue
		}
		var variantDirs []string
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if _, ok := localeTagFromValuesDir(entry.Name()); !ok {
				continue
			}
			variantDirs = append(variantDirs, entry.Name())
		}
		sort.Strings(variantDirs)
		for _, variant := range variantDirs {
			forEachStringInValuesDir(filepath.Join(resRoot, variant), func(path string, s *android.XMLNode) {
				name := s.Attr("name")
				if name == "" {
					return
				}
				def, ok := defaults[name]
				if !ok || !def.hasPositional {
					return
				}
				if strings.EqualFold(s.Attr("formatted"), "false") {
					return
				}
				info := analyzeFormatPlaceholders(s.Text)
				if info.total < 2 || info.hasPositional {
					return
				}
				ctx.Emit(resourceFinding(path, s.Line, r.BaseRule,
					fmt.Sprintf("String `%s` in `%s/` drops positional format syntax used by the default value. "+
						"Use `%%1$s`, `%%2$s`, ... so translators can reorder arguments.",
						name, variant)))
			})
		}
	}
}

// formatPlaceholderInfo summarizes the format specifiers of a single string.
type formatPlaceholderInfo struct {
	total         int
	hasPositional bool
}

// forEachStringInValuesDir walks every <string> element across the .xml files
// in a values directory in deterministic filename order.
func forEachStringInValuesDir(dir string, visit func(path string, s *android.XMLNode)) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".xml") {
			continue
		}
		names = append(names, f.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		root, err := android.ParseXMLAST(data)
		if err != nil || root == nil || root.Tag != "resources" {
			continue
		}
		for _, s := range root.ChildrenByTag("string") {
			visit(path, s)
		}
	}
}

// analyzeFormatPlaceholders scans a string for printf-style format specifiers
// and reports their count plus whether any use positional indexing (`%N$X`).
func analyzeFormatPlaceholders(s string) formatPlaceholderInfo {
	var info formatPlaceholderInfo
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
		positional := false
		k := j
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		if k > j && k < len(s) && s[k] == '$' {
			positional = true
			j = k + 1
		}
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
		if j >= len(s) || !validConversions[s[j]] {
			continue
		}
		info.total++
		if positional {
			info.hasPositional = true
		}
		i = j
	}
	return info
}
