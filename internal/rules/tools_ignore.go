package rules

// tools:ignore support for every XML resource kind: layouts, values, xml/ and
// menu/ resources, and the manifest. One token model and one element-span
// index serve all of them.
//
// Semantics, checked against the Android Lint 32 CLI:
//   - the attribute lives on any element and covers that element, its
//     attributes and everything under it, so an ancestor (the <resources>
//     root of a values file, a parent layout, <application>, <manifest>)
//     silences its descendants;
//   - "all" silences every rule;
//   - a token names a rule by its krit ID, its Android Lint issue ID, or an
//     alias of either, ignoring case, and may carry Lint's "AndroidLint" prefix;
//   - tokens are separated by commas. Krit also accepts whitespace as a
//     separator, a superset of Lint (see android.ToolsIgnoreTokens);
//   - a finding on the XML prolog or a comment before the root element belongs
//     to the root element.
// Lint also accepts issue category names as tokens; krit does not.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/scanner"
)

// toolsIgnoreSet is a set of lower-cased tools:ignore tokens in effect.
type toolsIgnoreSet map[string]bool

// toolsIgnoreSetOf builds a set from already-split tokens.
func toolsIgnoreSetOf(tokens []string) toolsIgnoreSet {
	set := make(toolsIgnoreSet, len(tokens))
	for _, token := range tokens {
		set.add(token)
	}
	return set
}

func (s toolsIgnoreSet) add(token string) {
	lower := strings.ToLower(strings.TrimSpace(token))
	if lower == "" {
		return
	}
	s[lower] = true
	// IDE inspections prefix issue IDs with "AndroidLint"; Lint accepts the prefixed form.
	if rest := strings.TrimPrefix(lower, "androidlint"); rest != lower && rest != "" {
		s[rest] = true
	}
}

// extend returns the set in effect for an element that carries raw as its
// tools:ignore value under a parent whose set is s.
func (s toolsIgnoreSet) extend(raw string) toolsIgnoreSet {
	tokens := android.ToolsIgnoreTokens(raw)
	if len(tokens) == 0 {
		return s
	}
	out := make(toolsIgnoreSet, len(s)+len(tokens))
	for token := range s {
		out[token] = true
	}
	for _, token := range tokens {
		out.add(token)
	}
	return out
}

// silences reports whether the set names the rule: by "all", its ID, or an Android Lint alias.
func (s toolsIgnoreSet) silences(rule string) bool {
	if len(s) == 0 {
		return false
	}
	if s["all"] || s[strings.ToLower(rule)] {
		return true
	}
	for _, alias := range lintAliasesForRuleID(rule) {
		if s[strings.ToLower(alias)] {
			return true
		}
	}
	return false
}

// resourceToolsIgnoreMatches reports whether the view itself lists any of the
// issue IDs (or "all") in tools:ignore. Ancestors are covered by
// walkViewsUnlessIgnored, which stops at an ignoring view.
func resourceToolsIgnoreMatches(v *android.View, issueIDs ...string) bool {
	if v == nil {
		return false
	}
	raw := v.Attributes["tools:ignore"]
	if raw == "" {
		return false
	}
	set := toolsIgnoreSetOf(android.ToolsIgnoreTokens(raw))
	for _, id := range issueIDs {
		if set.silences(id) {
			return true
		}
	}
	return false
}

// walkXMLToolsIgnore visits every element with the tokens in effect from it and its ancestors.
func walkXMLToolsIgnore(root *android.XMLNode, visit func(n *android.XMLNode, tokens toolsIgnoreSet)) {
	var walk func(n *android.XMLNode, inherited toolsIgnoreSet)
	walk = func(n *android.XMLNode, inherited toolsIgnoreSet) {
		tokens := inherited.extend(n.Attr("tools:ignore"))
		visit(n, tokens)
		for _, child := range n.Children {
			walk(child, tokens)
		}
	}
	walk(root, nil)
}

// toolsIgnoreSpans maps a source line to the tokens of the innermost element
// that contains it, so a finding anywhere inside an element, including a later
// line of a multi-line start tag, a child, or the closing tag, is covered.
type toolsIgnoreSpans struct {
	root  *android.XMLNode
	spans []toolsIgnoreSpan
}

type toolsIgnoreSpan struct {
	from, to int
	tokens   toolsIgnoreSet
}

// newToolsIgnoreSpans indexes data. It returns nil when the document does not
// parse or no element lists tools:ignore.
func newToolsIgnoreSpans(data []byte) *toolsIgnoreSpans {
	if !bytes.Contains(data, []byte("ignore")) {
		return nil
	}
	root, err := android.ParseXMLAST(context.Background(), data)
	if err != nil || root == nil {
		return nil
	}
	lineStarts := []int{0}
	for i, b := range data {
		if b == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	lineOf := func(offset uint) int {
		return sort.Search(len(lineStarts), func(i int) bool { return lineStarts[i] > int(offset) })
	}
	index := &toolsIgnoreSpans{root: root}
	walkXMLToolsIgnore(root, func(n *android.XMLNode, tokens toolsIgnoreSet) {
		if len(tokens) == 0 {
			return
		}
		end := n.Line
		if n.EndByte > 0 {
			end = lineOf(n.EndByte - 1)
		}
		index.spans = append(index.spans, toolsIgnoreSpan{from: n.Line, to: end, tokens: tokens})
	})
	if len(index.spans) == 0 {
		return nil
	}
	return index
}

// at returns the tokens in effect at line, or nil.
func (x *toolsIgnoreSpans) at(line int) toolsIgnoreSet {
	// Spans are appended parent-first, so the last containing span is the innermost element.
	var tokens toolsIgnoreSet
	for _, s := range x.spans {
		if s.from <= line && line <= s.to {
			tokens = s.tokens
		}
	}
	// A whole-file finding anchored on the XML prolog belongs to the root element.
	if tokens == nil && line < x.root.Line && x.spans[0].from == x.root.Line {
		tokens = x.spans[0].tokens
	}
	return tokens
}

// filterXMLToolsIgnored drops findings whose element, or an ancestor, silences the rule. Only findings in
// files accepted by wanted are examined; data returns a file's source or nil.
func filterXMLToolsIgnored(cols scanner.FindingColumns, wanted func(path string) bool, data func(path string) []byte) scanner.FindingColumns {
	if cols.Len() == 0 {
		return cols
	}
	spansByFile := map[string]*toolsIgnoreSpans{}
	spansFor := func(path string) *toolsIgnoreSpans {
		if spans, ok := spansByFile[path]; ok {
			return spans
		}
		var spans *toolsIgnoreSpans
		if src := data(path); src != nil {
			spans = newToolsIgnoreSpans(src)
		}
		spansByFile[path] = spans
		return spans
	}
	return cols.FilterRows(func(row int) bool {
		path := cols.FileAt(row)
		if !wanted(path) {
			return true
		}
		spans := spansFor(path)
		if spans == nil {
			return true
		}
		return !spans.at(cols.LineAt(row)).silences(cols.RuleAt(row))
	})
}

// filterManifestToolsIgnoredInSource drops manifest findings that sit anywhere inside an element (including its
// multi-line attributes and children) that lists the rule's ID or Android lint ID in tools:ignore, on the
// element or any ancestor such as <application> or <manifest>.
func filterManifestToolsIgnoredInSource(cols scanner.FindingColumns, path string, data []byte) scanner.FindingColumns {
	return filterXMLToolsIgnored(cols,
		func(p string) bool { return p == path },
		func(string) []byte { return data })
}

// filterValuesToolsIgnored drops findings in XML resource files on disk (values, layouts, xml, menu and the
// like) whose element, or an ancestor such as the <resources> root, silences the rule.
func filterValuesToolsIgnored(cols scanner.FindingColumns) scanner.FindingColumns {
	return filterXMLToolsIgnored(cols, isResourceXMLPath, func(path string) []byte {
		if data, err := os.ReadFile(path); err == nil {
			return data
		}
		return nil
	})
}

// resourceXMLDirs are the res/ folder types whose XML files can carry tools:ignore.
var resourceXMLDirs = []string{
	"values", "xml", "menu", "layout", "drawable", "color", "anim", "animator",
	"interpolator", "navigation", "transition", "font", "mipmap",
}

// isResourceXMLPath reports whether path is an XML file directly inside a res/ folder such as values-night or layout-land.
func isResourceXMLPath(path string) bool {
	if !strings.HasSuffix(path, ".xml") {
		return false
	}
	dir := filepath.Base(filepath.Dir(path))
	for _, kind := range resourceXMLDirs {
		if dir == kind || strings.HasPrefix(dir, kind+"-") {
			return true
		}
	}
	return false
}

// filterLayoutToolsIgnored drops layout findings whose view, or an ancestor, lists the rule's ID or Android
// lint ID in tools:ignore, so rules need no per-rule handling of the attribute. It works from the parsed view
// model, so it also covers layouts that exist only in the index.
func filterLayoutToolsIgnored(cols scanner.FindingColumns, idx *android.ResourceIndex) scanner.FindingColumns {
	if cols.Len() == 0 || idx == nil {
		return cols
	}
	ignoresByFile := map[string]map[int]toolsIgnoreSet{}
	for _, configs := range idx.LayoutConfigs {
		for _, layout := range configs {
			if layout != nil && layout.FilePath != "" {
				collectLayoutIgnores(layout, ignoresByFile)
			}
		}
	}
	for _, layout := range idx.Layouts {
		if layout != nil && layout.FilePath != "" && ignoresByFile[layout.FilePath] == nil {
			collectLayoutIgnores(layout, ignoresByFile)
		}
	}
	if len(ignoresByFile) == 0 {
		return cols
	}
	return cols.FilterRows(func(row int) bool {
		return !ignoresByFile[cols.FileAt(row)][cols.LineAt(row)].silences(cols.RuleAt(row))
	})
}

// collectLayoutIgnores records, per view line, the tools:ignore tokens in effect from the view and its ancestors.
func collectLayoutIgnores(layout *android.Layout, out map[string]map[int]toolsIgnoreSet) {
	var walk func(v *android.View, inherited toolsIgnoreSet)
	walk = func(v *android.View, inherited toolsIgnoreSet) {
		if v == nil {
			return
		}
		tokens := inherited.extend(v.Attributes["tools:ignore"])
		if len(tokens) > 0 && v.Line > 0 {
			byLine := out[layout.FilePath]
			if byLine == nil {
				byLine = map[int]toolsIgnoreSet{}
				out[layout.FilePath] = byLine
			}
			merged := byLine[v.Line]
			if merged == nil {
				merged = toolsIgnoreSet{}
				byLine[v.Line] = merged
			}
			for t := range tokens {
				merged[t] = true
			}
		}
		for _, child := range v.Children {
			walk(child, tokens)
		}
	}
	walk(layout.RootView, nil)
}

// filterManifestToolsIgnoredByLine drops manifest findings anchored on an element whose tools:ignore, or an ancestor's,
// lists the rule's ID or Android lint ID. It works from the element-line index of the manifest model, which needs no
// source text.
func filterManifestToolsIgnoredByLine(cols scanner.FindingColumns, m *manifest.Manifest) scanner.FindingColumns {
	if cols.Len() == 0 || m == nil || len(m.ToolsIgnoreByLine) == 0 {
		return cols
	}
	setsByLine := map[int]toolsIgnoreSet{}
	for line, tokens := range m.ToolsIgnoreByLine {
		setsByLine[line] = toolsIgnoreSetOf(tokens)
	}
	return cols.FilterRows(func(row int) bool {
		line := cols.LineAt(row)
		// File-level findings anchor at line 1, which precedes the <manifest> element when an XML declaration comes first.
		if line == 1 && m.RootLine > 1 {
			line = m.RootLine
		}
		return !setsByLine[line].silences(cols.RuleAt(row))
	})
}

// filterManifestToolsIgnored applies both manifest filters: the element-line index of the manifest model, and the
// source scan that also covers later lines of multi-line elements. data is the manifest source, or nil to use the
// file at path.
func filterManifestToolsIgnored(cols scanner.FindingColumns, m *manifest.Manifest, path string, data []byte) scanner.FindingColumns {
	cols = filterManifestToolsIgnoredByLine(cols, m)
	if cols.Len() == 0 || path == "" && len(data) == 0 {
		return cols
	}
	if len(data) == 0 {
		data, _ = os.ReadFile(path)
	}
	return filterManifestToolsIgnoredInSource(cols, path, data)
}
