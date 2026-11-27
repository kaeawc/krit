package rules

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/lazyre"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's StringEscapeDetector (StringEscaping), StartDestinationDetector (InvalidNavigation),
// RestrictionsDetector (ValidRestrictions) and RelativeClassAttributeDetector (RelativeClassResolution).

const (
	resAndroidURI = "http://schemas.android.com/apk/res/android"
	resAutoURI    = "http://schemas.android.com/apk/res-auto"
)

// StringEscapingRule mirrors Lint's StringEscaping: an unescaped apostrophe or a bad \u escape in a string resource,
// as aapt's string parser sees it.
type StringEscapingRule struct {
	ValuesResourceBase
	AndroidRule
}

// InvalidNavigationRule mirrors Lint's InvalidNavigation: a <navigation> without a valid start destination.
type InvalidNavigationRule struct {
	ValuesResourceBase
	AndroidRule
}

// ValidRestrictionsRule mirrors Lint's ValidRestrictions: a malformed application restrictions descriptor.
type ValidRestrictionsRule struct {
	ValuesResourceBase
	AndroidRule
}

// RelativeClassResolutionRule mirrors Lint's RelativeClassResolution: a relative class name in layout_behavior or
// layoutManager, which resolves against the applicationId at runtime.
type RelativeClassResolutionRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read the resource XML as Lint does.
func (r *StringEscapingRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the resource XML as Lint does.
func (r *InvalidNavigationRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the resource XML as Lint does.
func (r *ValidRestrictionsRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the value and the module's build script are read directly.
func (r *RelativeClassResolutionRule) Confidence() float64 { return api.ConfidenceHigh }

// resXMLDoc is one parsed resource XML file.
type resXMLDoc struct {
	path string
	data []byte
	root *android.XMLNode
	// lineStarts holds the byte offset of each line start, for byte offset to line/column conversion.
	lineStarts []int
}

// resFolderFiles lists the XML files of the resource folders of the given type (`values`, `xml`, `navigation`, ...)
// in every res directory, qualifiers included, in path order. When needle is not empty, files without it are
// skipped before parsing; mayMatch, when set, is a finer byte prefilter.
func resFolderFiles(idx *android.ResourceIndex, folderType string, needle []byte, mayMatch ...func([]byte) bool) []*resXMLDoc {
	if idx == nil {
		return nil
	}
	var out []*resXMLDoc
	for _, resDir := range idx.ResDirs {
		folders, err := os.ReadDir(resDir)
		if err != nil {
			continue
		}
		for _, folder := range folders {
			name := folder.Name()
			if !folder.IsDir() || (name != folderType && !strings.HasPrefix(name, folderType+"-")) {
				continue
			}
			files, err := os.ReadDir(filepath.Join(resDir, name))
			if err != nil {
				continue
			}
			for _, f := range files {
				if f.IsDir() || !strings.HasSuffix(f.Name(), ".xml") {
					continue
				}
				path := filepath.Join(resDir, name, f.Name())
				data, err := os.ReadFile(path)
				if err != nil || (len(needle) > 0 && !bytes.Contains(data, needle)) || (len(mayMatch) > 0 && !mayMatch[0](data)) {
					continue
				}
				root, err := android.ParseXMLAST(context.Background(), data)
				if err != nil || root == nil {
					continue
				}
				out = append(out, &resXMLDoc{path: path, data: data, root: root, lineStarts: resLineStarts(data)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func resLineStarts(data []byte) []int {
	starts := []int{0}
	for i, b := range data {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// pos converts a byte offset into a 1-based line and a 1-based column counted in characters, as Lint counts them.
func (d *resXMLDoc) pos(offset int) (int, int) {
	line := sort.Search(len(d.lineStarts), func(i int) bool { return d.lineStarts[i] > offset }) - 1
	if line < 0 {
		line = 0
	}
	return line + 1, utf8.RuneCount(d.data[d.lineStarts[line]:offset]) + 1
}

func (d *resXMLDoc) finding(offset int, msg string) scanner.Finding {
	line, col := d.pos(offset)
	return scanner.Finding{File: d.path, Line: line, Col: col, Message: msg}
}

// resPrefixes maps each namespace URI declared on the element or its ancestors to its prefix.
type resPrefixes map[string]string

func (p resPrefixes) with(n *android.XMLNode) resPrefixes {
	var out resPrefixes
	for _, a := range n.Attrs {
		if !strings.HasPrefix(a.Name, "xmlns:") {
			continue
		}
		if out == nil {
			out = resPrefixes{}
			for k, v := range p {
				out[k] = v
			}
		}
		out[a.Value] = strings.TrimPrefix(a.Name, "xmlns:")
	}
	if out == nil {
		return p
	}
	return out
}

// attrNS returns the attribute in the namespace uri with the local name, as getAttributeNodeNS does.
func (p resPrefixes) attrNS(n *android.XMLNode, uri, local string) (android.XMLAttribute, bool) {
	prefix, ok := p[uri]
	if !ok {
		return android.XMLAttribute{}, false
	}
	for _, a := range n.Attrs {
		if a.Name == prefix+":"+local {
			return a, true
		}
	}
	return android.XMLAttribute{}, false
}

// valueOffset returns the byte offset of an attribute's value text.
func (d *resXMLDoc) valueOffset(a android.XMLAttribute) int {
	raw := d.data[a.StartByte:a.EndByte]
	if q := bytes.IndexAny(raw, `"'`); q >= 0 {
		return int(a.StartByte) + q + 1
	}
	return int(a.StartByte)
}

// elementOffset returns where Lint's getElementLocation points: the tag name of an element with child elements,
// the `<` of one without.
func (d *resXMLDoc) elementOffset(n *android.XMLNode) int {
	if len(n.Children) > 0 {
		return int(n.StartByte) + 1
	}
	return int(n.StartByte)
}

// nameOffset returns the offset of an element's tag name, one past its `<`, where Lint's getNameLocation points.
func (d *resXMLDoc) nameOffset(n *android.XMLNode) int { return int(n.StartByte) + 1 }

// ---------------------------------------------------------------------------
// StringEscaping
// ---------------------------------------------------------------------------

// stringEscapingMayMatch reports whether a values file can hold a finding: a <string>, <string-array> or <plurals>
// with an apostrophe (literal or as an entity) or a \u escape.
func stringEscapingMayMatch(data []byte) bool {
	if !bytes.Contains(data, []byte("<string")) && !bytes.Contains(data, []byte("<plurals")) {
		return false
	}
	return bytes.IndexByte(data, '\'') >= 0 || bytes.Contains(data, []byte("&apos;")) || bytes.Contains(data, []byte("&#")) ||
		bytes.Contains(data, []byte(`\u`))
}

func (r *StringEscapingRule) check(ctx *api.Context) {
	for _, doc := range resFolderFiles(ctx.ResourceIndex, "values", nil, stringEscapingMayMatch) {
		if doc.root.Tag != "resources" {
			continue
		}
		for _, el := range doc.root.Children {
			switch el.Tag {
			case "string":
				r.checkElement(ctx, doc, el)
			case "string-array", "plurals":
				for _, item := range el.Children {
					r.checkElement(ctx, doc, item)
				}
			}
		}
	}
}

// checkElement checks the first text or CDATA child of the element, as findTextNode does.
func (r *StringEscapingRule) checkElement(ctx *api.Context, doc *resXMLDoc, el *android.XMLNode) {
	text, offsets, ok := resFirstTextNode(doc.data, el)
	if !ok {
		return
	}
	if f, ok := stringEscapingCheck(doc, text, offsets); ok {
		ctx.Emit(f)
	}
}

// resFirstTextNode returns the decoded value of the element's first text or CDATA child (comments and child
// elements are skipped over; a whitespace-only run counts) with the source byte offset of each decoded character.
func resFirstTextNode(data []byte, el *android.XMLNode) ([]rune, []int, bool) {
	// The start tag ends at the first `>` after its last attribute; an attribute value may itself hold `>`.
	from := el.StartByte
	for _, a := range el.Attrs {
		if a.EndByte > from && a.EndByte <= el.EndByte {
			from = a.EndByte
		}
	}
	start := bytes.IndexByte(data[from:el.EndByte], '>')
	if start < 0 || data[int(from)+start-1] == '/' {
		return nil, nil, false
	}
	i := int(from) + start + 1
	end := int(el.EndByte)
	if closeTag := bytes.LastIndex(data[i:end], []byte("</")); closeTag >= 0 {
		end = i + closeTag
	}
	for i < end {
		switch {
		case bytes.HasPrefix(data[i:], []byte("<!--")):
			c := bytes.Index(data[i:end], []byte("-->"))
			if c < 0 {
				return nil, nil, false
			}
			i += c + 3
		case bytes.HasPrefix(data[i:], []byte("<![CDATA[")):
			body := i + len("<![CDATA[")
			c := bytes.Index(data[body:end], []byte("]]>"))
			if c < 0 {
				return nil, nil, false
			}
			var text []rune
			var offsets []int
			for j := body; j < body+c; {
				r, size := utf8.DecodeRune(data[j:])
				text, offsets = append(text, r), append(offsets, j)
				j += size
			}
			return text, offsets, true
		case data[i] == '<':
			// A child element: skip to the end of its subtree.
			child := resChildAt(el, i)
			if child == nil {
				return nil, nil, false
			}
			i = int(child.EndByte)
		default:
			var text []rune
			var offsets []int
			for i < end && data[i] != '<' {
				r, size, at := resDecodeChar(data, i, end)
				text, offsets = append(text, r), append(offsets, at)
				i += size
			}
			return text, offsets, true
		}
	}
	return nil, nil, false
}

func resChildAt(el *android.XMLNode, offset int) *android.XMLNode {
	for _, c := range el.Children {
		if int(c.StartByte) == offset {
			return c
		}
	}
	return nil
}

// resDecodeChar decodes one character of XML character data at i, expanding the predefined and numeric entities.
func resDecodeChar(data []byte, i, end int) (rune, int, int) {
	if data[i] == '&' {
		if semi := bytes.IndexByte(data[i:end], ';'); semi > 1 {
			name := string(data[i+1 : i+semi])
			switch name {
			case "lt":
				return '<', semi + 1, i
			case "gt":
				return '>', semi + 1, i
			case "amp":
				return '&', semi + 1, i
			case "apos":
				return '\'', semi + 1, i
			case "quot":
				return '"', semi + 1, i
			}
			if strings.HasPrefix(name, "#") {
				num, base := name[1:], 10
				if strings.HasPrefix(num, "x") || strings.HasPrefix(num, "X") {
					num, base = num[1:], 16
				}
				if v, err := strconv.ParseInt(num, base, 32); err == nil {
					return rune(v), semi + 1, i
				}
			}
		}
	}
	r, size := utf8.DecodeRune(data[i:end])
	return r, size, i
}

// stringEscapingCheck ports checkXmlEscapes (aapt's ResourceTypes.cpp stringToValue / collectString). The location is
// the text node's start plus the character index, as Lint computes it.
func stringEscapingCheck(doc *resXMLDoc, str []rune, offsets []int) (scanner.Finding, bool) {
	at := func(p int) int {
		if len(offsets) == 0 {
			return 0
		}
		// Lint adds the index to the node's start offset, without accounting for expanded entities.
		return offsets[0] + resByteDelta(doc.data, offsets[0], p)
	}
	s, l := escTrim(str)
	var quoted rune
	p := s
	for p < s+l {
		var apostrophe bool
		if p, apostrophe = escScanPlain(str, p, s+l, quoted); apostrophe {
			return doc.finding(at(p), `Apostrophe not preceded by \\`), true
		}
		if p >= s+l {
			continue
		}
		var bad int
		if p, quoted, bad = escConsume(str, p, s+l, l, quoted); bad >= 0 {
			return doc.finding(at(bad), `Bad character in \\u unicode escape sequence`), true
		}
		l -= p - s
		s = p
	}
	return scanner.Finding{}, false
}

// escTrim strips leading and trailing whitespace, keeping a space escaped by a trailing backslash, and returns the
// start and length of what is left.
func escTrim(str []rune) (int, int) {
	s, l := 0, len(str)
	for l > 0 && unicode.IsSpace(str[s]) {
		s++
		l--
	}
	for l > 0 && unicode.IsSpace(str[s+l-1]) {
		l--
	}
	if l > 0 && str[s+l-1] == '\\' && s+l < len(str) {
		l++
	}
	return s, l
}

// escScanPlain advances over plain characters up to the next backslash, quote or collapsible whitespace. It reports
// true, with its index, when it meets an apostrophe that is not inside double quotes.
func escScanPlain(str []rune, p, end int, quoted rune) (int, bool) {
	n := len(str)
	for p < end {
		c := str[p]
		switch {
		case c == '\\':
			return p, false
		case quoted == 0 && unicode.IsSpace(c) && (c != ' ' || p == n-1 || unicode.IsSpace(str[p+1])):
			return p, false
		case c == '"' && (quoted == 0 || quoted == '"'):
			return p, false
		case c == '\'' && (quoted == 0 || quoted == '\''):
			return p, true
		}
		p++
	}
	return p, false
}

// escConsume consumes the quote, whitespace run or escape at p and returns the new index and quote state, plus the
// index of a bad \u digit or -1. Like Lint, the \u scan is bounded by the remaining length l, not the end index.
func escConsume(str []rune, p, end, l int, quoted rune) (int, rune, int) {
	switch cp := str[p]; {
	case cp == '"' || cp == '\'':
		if quoted == 0 {
			quoted = cp
		} else {
			quoted = 0
		}
		p++
	case unicode.IsSpace(cp):
		p++
		for p < end && unicode.IsSpace(str[p]) {
			p++
		}
	case cp == '\\':
		p++
		if p < end {
			if str[p] == 'u' {
				for i := 0; i < 4 && p+1 < l; i++ {
					p++
					if !isHexDigit(str[p]) {
						return p, quoted, p
					}
				}
			}
			p++
		}
	}
	return p, quoted, -1
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// resByteDelta returns the byte length of the first count characters of the source starting at offset.
func resByteDelta(data []byte, offset, count int) int {
	i := offset
	for c := 0; c < count && i < len(data); c++ {
		_, size := utf8.DecodeRune(data[i:])
		i += size
	}
	return i - offset
}

// ---------------------------------------------------------------------------
// InvalidNavigation
// ---------------------------------------------------------------------------

var navigationNeedle = []byte("<navigation")

// resourceURL is the part of a parsed resource reference Lint's ResourceUrl gives these checks.
type resourceURL struct {
	typ, name string
}

var resRefURLRe = lazyre.New(`^[@?]\+?(?:[A-Za-z0-9_.]+:)?([A-Za-z]+)/([^/\s]+)$`)

// parseResourceURL parses `@type/name`, `@+id/name`, `@pkg:type/name` and `?attr/name`.
func parseResourceURL(v string) (resourceURL, bool) {
	m := resRefURLRe.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return resourceURL{}, false
	}
	return resourceURL{typ: m[1], name: m[2]}, true
}

func (r *InvalidNavigationRule) check(ctx *api.Context) {
	docs := resFolderFiles(ctx.ResourceIndex, "navigation", navigationNeedle)
	roots := navigationRootIndex{idx: ctx.ResourceIndex}
	for _, doc := range docs {
		var visit func(n *android.XMLNode, ns resPrefixes)
		visit = func(n *android.XMLNode, ns resPrefixes) {
			ns = ns.with(n)
			if n.Tag == "navigation" {
				r.checkNavigation(ctx, doc, n, ns, &roots)
			}
			for _, c := range n.Children {
				visit(c, ns)
			}
		}
		visit(doc.root, nil)
	}
}

func (r *InvalidNavigationRule) checkNavigation(ctx *api.Context, doc *resXMLDoc, n *android.XMLNode, ns resPrefixes,
	index *navigationRootIndex) {
	if len(n.Children) == 0 {
		return
	}
	attr, ok := ns.attrNS(n, resAutoURI, "startDestination")
	if !ok || strings.TrimSpace(attr.Value) == "" {
		ctx.Emit(doc.finding(doc.nameOffset(n), "No start destination specified"))
		return
	}
	url, ok := parseResourceURL(attr.Value)
	if !ok || url.typ != "id" {
		ctx.Emit(doc.finding(doc.nameOffset(n), "`startDestination` must be an id"))
		return
	}
	for _, child := range n.Children {
		cns := ns.with(child)
		if child.Tag == "include" {
			graph, _ := cns.attrNS(child, resAutoURI, "graph")
			included, ok := parseResourceURL(graph.Value)
			if !ok {
				continue
			}
			roots := index.roots(included.typ, included.name)
			if len(roots) == 0 {
				// The included graph lives in another module; Lint cannot check it.
				return
			}
			for _, root := range roots {
				if id, ok := resPrefixes(nil).with(root).attrNS(root, resAndroidURI, "id"); ok && resStripIDPrefix(id.Value) == url.name {
					return
				}
			}
			continue
		}
		id, ok := cns.attrNS(child, resAndroidURI, "id")
		if !ok {
			continue
		}
		if childURL, ok := parseResourceURL(id.Value); ok && childURL.name == url.name {
			return
		}
	}
	ctx.Emit(doc.finding(doc.valueOffset(attr), "Invalid start destination "+attr.Value))
}

// navigationRootIndex reads each resource folder type once per check and finds root elements by file name, so a
// graph with many <include>s doesn't re-read the folders for each.
type navigationRootIndex struct {
	idx    *android.ResourceIndex
	byType map[string]map[string][]*android.XMLNode
}

func (x *navigationRootIndex) roots(typ, name string) []*android.XMLNode {
	if x.byType == nil {
		x.byType = map[string]map[string][]*android.XMLNode{}
	}
	byName, ok := x.byType[typ]
	if !ok {
		byName = map[string][]*android.XMLNode{}
		for _, doc := range resFolderFiles(x.idx, typ, nil) {
			key := strings.TrimSuffix(filepath.Base(doc.path), ".xml")
			byName[key] = append(byName[key], doc.root)
		}
		x.byType[typ] = byName
	}
	return byName[name]
}

func resStripIDPrefix(v string) string {
	for _, p := range []string{"@+id/", "@id/"} {
		if strings.HasPrefix(v, p) {
			return v[len(p):]
		}
	}
	return v
}

// ---------------------------------------------------------------------------
// ValidRestrictions
// ---------------------------------------------------------------------------

var restrictionsNeedle = []byte("<restrictions")

const (
	restrictionsMaxDepth  = 20
	restrictionsMaxNested = 1000
)

func (r *ValidRestrictionsRule) check(ctx *api.Context) {
	for _, doc := range resFolderFiles(ctx.ResourceIndex, "xml", restrictionsNeedle) {
		if doc.root.Tag != "restrictions" {
			continue
		}
		v := &restrictionsValidator{ctx: ctx, doc: doc, keys: map[string]bool{}}
		v.nested(doc.root, resPrefixes(nil).with(doc.root), "", false, 0)
	}
}

type restrictionsValidator struct {
	ctx  *api.Context
	doc  *resXMLDoc
	keys map[string]bool
}

func (v *restrictionsValidator) report(offset int, msg string) {
	v.ctx.Emit(v.doc.finding(offset, msg))
}

// nested validates the children of an element, as validateNestedRestrictions does. typ is the element's
// restrictionType; hasType is false for the root, whose type prints as `null`.
func (v *restrictionsValidator) nested(el *android.XMLNode, ns resPrefixes, typ string, hasType bool, depth int) {
	children := el.Children
	if depth == 0 || typ == "bundle" || typ == "bundle_array" {
		if def, ok := ns.attrNS(el, resAndroidURI, "defaultValue"); ok {
			shown := typ
			if !hasType {
				shown = "null"
			}
			v.report(int(def.StartByte), fmt.Sprintf("Restriction type `%s` should not have a default value", shown))
		}
		for _, child := range children {
			if child.Tag != "restriction" {
				v.report(v.doc.nameOffset(child), fmt.Sprintf("Unexpected tag `<%s>`, expected `<restriction>`", child.Tag))
				continue
			}
			v.restriction(child, ns.with(child), depth+1)
		}
		switch {
		case depth == 0:
		case typ == "bundle_array":
			if len(children) != 1 {
				v.report(v.doc.elementOffset(el), "Expected exactly one child for restriction of type `bundle_array`")
			}
		case len(children) == 0:
			v.report(v.doc.elementOffset(el), "Restriction type `bundle` should have at least one nested restriction")
		}
		if len(children) > restrictionsMaxNested {
			v.report(v.doc.elementOffset(el), fmt.Sprintf("Invalid nested restriction: too many nested restrictions (was %d, max %d)",
				len(children), restrictionsMaxNested))
		} else if depth > restrictionsMaxDepth {
			v.report(v.doc.elementOffset(el), fmt.Sprintf("Invalid nested restriction: nesting depth %d too large (max %d",
				depth, restrictionsMaxDepth))
		}
	} else if len(children) > 0 {
		v.report(v.doc.nameOffset(el), "Only restrictions of type `bundle` and `bundle_array` can have one or multiple nested restriction elements")
	}
}

func (v *restrictionsValidator) required(el *android.XMLNode, ns resPrefixes, name string) (string, bool) {
	if a, ok := ns.attrNS(el, resAndroidURI, name); ok {
		return a.Value, true
	}
	full := name
	if prefix, ok := ns[resAndroidURI]; ok {
		full = prefix + ":" + name
	}
	v.report(v.doc.elementOffset(el), fmt.Sprintf("Missing required attribute `%s`", full))
	return "", false
}

func (v *restrictionsValidator) restriction(el *android.XMLNode, ns resPrefixes, depth int) {
	typ, okType := v.required(el, ns, "restrictionType")
	key, okKey := v.required(el, ns, "key")
	_, okTitle := v.required(el, ns, "title")
	if !okType || !okKey || !okTitle {
		return
	}
	keyAttr, _ := ns.attrNS(el, resAndroidURI, "key")
	switch {
	case strings.HasPrefix(key, "@string/"):
		v.report(v.doc.valueOffset(keyAttr), "Keys cannot be localized, they should be specified with a string literal")
	case v.keys[key]:
		v.report(v.doc.valueOffset(keyAttr), fmt.Sprintf("Duplicate key `%s`", key))
	default:
		v.keys[key] = true
	}
	switch typ {
	case "choice", "multi-select":
		if _, ok := v.required(el, ns, "entries"); ok {
			break
		}
		v.required(el, ns, "entryValues")
	case "hidden":
		v.required(el, ns, "defaultValue")
	case "integer":
		if def, ok := ns.attrNS(el, resAndroidURI, "defaultValue"); ok && !strings.HasPrefix(def.Value, "@") {
			if _, ok := javaIntegerDecode(def.Value); !ok {
				v.report(v.doc.valueOffset(def), "Invalid number")
			}
		}
	}
	v.nested(el, ns, typ, true, depth)
}

// javaIntegerDecode mirrors java.lang.Integer.decode: an optional sign, then decimal, 0x/0X/# hexadecimal, or
// 0-prefixed octal digits.
func javaIntegerDecode(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}
	base := 10
	switch {
	case strings.HasPrefix(s, "0x"), strings.HasPrefix(s, "0X"):
		base, s = 16, s[2:]
	case strings.HasPrefix(s, "#"):
		base, s = 16, s[1:]
	case strings.HasPrefix(s, "0") && len(s) > 1:
		base, s = 8, s[1:]
	}
	if s == "" || s[0] == '-' || s[0] == '+' || strings.Contains(s, "_") {
		return 0, false
	}
	v, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		v = -v
	}
	if v < -1<<31 || v > 1<<31-1 {
		return 0, false
	}
	return v, true
}

// ---------------------------------------------------------------------------
// RelativeClassResolution
// ---------------------------------------------------------------------------

// moduleIdentity is what Lint's Project gives RelativeClassAttributeDetector: the namespace (package), whether the
// module is a library, and the applicationId when the build declares one.
type moduleIdentity struct {
	namespace, applicationID string
	library                  bool
}

var (
	gradleNamespaceRe = lazyre.New(`(?m)^\s*namespace\s*=?\s*["']([^"']+)["']`)
	gradleAppIDRe     = lazyre.New(`(?m)^\s*applicationId\s*=?\s*["']([^"']+)["']`)
	manifestPackageRe = lazyre.New(`<manifest\b[^>]*\bpackage\s*=\s*["']([^"']+)["']`)
)

// moduleIdentityForRes reads the identity of the module that owns a res directory (`<module>/src/<set>/res` or
// `<module>/res`). An application module without an applicationId gets its namespace, as AGP does.
func moduleIdentityForRes(resDir string) moduleIdentity {
	return filefacts.RequestFact(fileFactsCache(), resDir, slotModuleIdentity, func() moduleIdentity {
		return readModuleIdentity(resDir)
	})
}

// slotModuleIdentity memoizes moduleIdentityForRes per res directory for one request.
const slotModuleIdentity = "lintModuleIdentity"

func readModuleIdentity(resDir string) moduleIdentity {
	module := resModuleDir(resDir)
	var id moduleIdentity
	for _, name := range manifestBuildScriptNames {
		data, err := os.ReadFile(filepath.Join(module, name))
		if err != nil {
			continue
		}
		content := gradleWithoutComments(string(data))
		if m := gradleNamespaceRe.FindStringSubmatch(content); m != nil {
			id.namespace = m[1]
		}
		if m := gradleAppIDRe.FindStringSubmatch(content); m != nil {
			id.applicationID = m[1]
		}
		info := readManifestGradleInfo(module)
		// Decide from the plugins alone: info.isApplication also holds for any `applicationId` text, such as a
		// library's testApplicationId.
		id.library = info.isLibrary && !moduleAppliesApplicationPlugin(module, content)
		if !id.library && id.applicationID == "" && info.isApplication {
			id.applicationID = id.namespace
		}
		break
	}
	for _, manifest := range moduleIdentityManifests(resDir) {
		if id.namespace != "" {
			break
		}
		if data, err := os.ReadFile(manifest); err == nil {
			if m := manifestPackageRe.FindSubmatch(data); m != nil {
				id.namespace = string(m[1])
			}
		}
	}
	return id
}

// gradleWithoutComments blanks the comments of a build script, so a commented-out namespace or applicationId is not
// read.
func gradleWithoutComments(content string) string {
	lines := strings.Split(content, "\n")
	inBlock := false
	for i, line := range lines {
		lines[i], inBlock = blankGradleComments(line, inBlock)
	}
	return strings.Join(lines, "\n")
}

// moduleAppliesApplicationPlugin reports a build script that applies the Android application plugin, by id or
// through a version-catalog alias.
func moduleAppliesApplicationPlugin(module, content string) bool {
	return appliesAndroidPlugin(content, "application", catalogPluginIDs(module, content))
}

// appliesAndroidPlugin reports whether a build script applies com.android.<kind>: by id, by a convention plugin whose
// id ends in <kind>, or through a catalog alias that ids (catalogPluginIDs) resolves to it.
func appliesAndroidPlugin(content, kind string, ids []string) bool {
	id := "com.android." + kind
	if strings.Contains(content, id) || containsPluginSuffix(content, kind) {
		return true
	}
	for _, resolved := range ids {
		if resolved == id || strings.HasPrefix(resolved, id+".") {
			return true
		}
	}
	return false
}

func (r *RelativeClassResolutionRule) check(ctx *api.Context) {
	if ctx.ResourceIndex == nil {
		return
	}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		resDir := filepath.Dir(filepath.Dir(layout.FilePath))
		walkViews(layout.RootView, func(v *android.View) {
			for name, value := range v.Attributes {
				local := name[strings.LastIndexByte(name, ':')+1:]
				if (local != "layout_behavior" && local != "layoutManager") || !strings.HasPrefix(value, ".") {
					continue
				}
				id := moduleIdentityForRes(resDir)
				if id.namespace == "" {
					continue
				}
				full := id.namespace + value
				line, col := v.PosOf(name)
				f := scanner.Finding{File: layout.FilePath, Line: line, Col: col}
				if id.library || (id.applicationID != "" && id.applicationID != id.namespace) {
					f.Message = fmt.Sprintf("Relative class name (`%s`) will resolve using applicationId at runtime and crash. "+
						"Use the fully qualified name instead: `%s`", value, full)
				} else {
					f.Severity = "warning"
					f.Message = fmt.Sprintf("Relative class name (`%s`) is fragile. If the `applicationId` changes via build "+
						"flavors or refactoring, this will crash at runtime. Use the fully qualified name instead: `%s`", value, full)
				}
				ctx.Emit(f)
			}
		})
	}
}
