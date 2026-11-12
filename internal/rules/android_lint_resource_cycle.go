package rules

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// This file ports Lint's ResourceCycleDetector (ResourceCycle): resources that refer to each other in a loop, such as
// a style whose parent chain returns to it, a string that points at itself, or a layout that includes itself.
// Every module is checked on its own, over the res directories of all its source sets. The values files are visited
// while the translation checks read them, so a scan parses each of them once.

const lintResourceCycle = "ResourceCycle"

// cycleLoc is where a resource reference sits in a file.
type cycleLoc struct {
	file      string
	line, col int
}

// cycleDisplayNames are ResourceType.getDisplayName for the types a cycle can involve.
var cycleDisplayNames = map[string]string{
	"anim": "Animation", "animator": "Animator", "array": "Array", "attr": "Attr", "bool": "Boolean", "color": "Color",
	"dimen": "Dimension", "drawable": "Drawable", "font": "Font", "fraction": "Fraction", "id": "ID", "integer": "Integer",
	"interpolator": "Interpolator", "layout": "Layout", "menu": "Menu", "mipmap": "Mip Map", "navigation": "Navigation",
	"plurals": "Plurals", "raw": "Raw", "string": "String", "style": "Style", "styleable": "Styleable",
	"transition": "Transition", "xml": "XML",
}

// cycleTypeOf is ResourceType.fromXmlValue for the type attribute of an <item>; it returns "" for unknown types.
func cycleTypeOf(value string) string {
	switch value {
	case "string-array", "integer-array":
		return "array"
	case "declare-styleable":
		return "styleable"
	}
	if _, ok := cycleDisplayNames[value]; ok {
		return value
	}
	return ""
}

// cycleFinding is one report of the check.
type cycleFinding struct {
	loc cycleLoc
	msg string
}

// newCycleModule returns an empty collector.
func newCycleModule() *cycleModule {
	return &cycleModule{refs: map[string]map[string][]string{}, locs: map[string]map[string][]cycleLoc{}, seen: map[cycleFinding]bool{}}
}

// cycleModule collects the references and locations of one module.
type cycleModule struct {
	refs     map[string]map[string][]string   // type -> from -> targets in file order
	locs     map[string]map[string][]cycleLoc // type -> from -> places the name is defined or referenced
	findings []cycleFinding
	seen     map[cycleFinding]bool
}

// cycleFile is the file being read.
type cycleFile struct {
	path      string
	folderTy  string
	baseName  string
	namespace map[string]string // xmlns prefix -> URI of the root element
}

// cycleFolderTypes are the resource folder types Lint's detector reads.
var cycleFolderTypes = map[string]bool{"values": true, "color": true, "drawable": true, "layout": true, "font": true}

// cycleMayReference is a cheap text test that skips files whose elements and attributes cannot refer to a resource
// of their own kind. A values file needs a style or an element whose text starts with a reference; the other folders
// need an attribute value that starts with a reference to their own type.
func cycleMayReference(data []byte, folderTy string) bool {
	if folderTy == "values" {
		return bytes.Contains(data, []byte("<style")) || bytes.Contains(data, []byte(">@"))
	}
	return bytes.Contains(data, []byte("@"+folderTy+"/"))
}

// readNonValuesFile reads one file of a color, drawable, layout or font folder when it can hold a reference.
func (m *cycleModule) readNonValuesFile(path, folderTy string) {
	data, err := os.ReadFile(path)
	if err != nil || !cycleMayReference(data, folderTy) {
		return
	}
	if root, err := android.ParseXMLAST(context.Background(), data); err == nil && root != nil {
		m.readFile(path, folderTy, root)
	}
}

// readFile visits one parsed resource file.
func (m *cycleModule) readFile(path, folderTy string, root *android.XMLNode) {
	file := &cycleFile{path: path, folderTy: folderTy, baseName: fileResourceName(filepath.Base(path)), namespace: map[string]string{}}
	for _, a := range root.Attrs {
		if prefix, ok := strings.CutPrefix(a.Name, "xmlns:"); ok {
			file.namespace[prefix] = a.Value
		}
	}
	m.visit(file, root, false)
}

// attrNS returns the value of a namespaced attribute, matching on the namespace URI rather than the prefix.
func (f *cycleFile) attrNS(n *android.XMLNode, uri, local string) (android.XMLAttribute, bool) {
	for _, a := range n.Attrs {
		prefix, name, ok := strings.Cut(a.Name, ":")
		if ok && name == local && f.namespace[prefix] == uri {
			return a, true
		}
	}
	return android.XMLAttribute{}, false
}

func (m *cycleModule) visit(f *cycleFile, n *android.XMLNode, inheritedIgnore bool) {
	ignored := inheritedIgnore || xmlToolsIgnores(n, lintResourceCycle)
	m.visitElement(f, n, ignored)
	if f.folderTy != "values" {
		for _, a := range n.Attrs {
			m.visitAttribute(f, a, ignored)
		}
	}
	for _, c := range n.Children {
		m.visit(f, c, ignored)
	}
}

// xmlToolsIgnores reports whether the element's tools:ignore lists the issue.
func xmlToolsIgnores(n *android.XMLNode, issue string) bool {
	for _, a := range n.Attrs {
		if strings.HasSuffix(a.Name, ":ignore") {
			for _, tok := range android.ToolsIgnoreTokens(a.Value) {
				if tok == issue || tok == "all" {
					return true
				}
			}
		}
	}
	return false
}

const androidNSURI = "http://schemas.android.com/apk/res/android"

func (m *cycleModule) visitElement(f *cycleFile, n *android.XMLNode, ignored bool) {
	pos := func(a android.XMLAttribute) cycleLoc { return cycleLoc{f.path, a.Line, a.Col} }
	switch n.Tag {
	case "include":
		if a, ok := attrByName(n, "layout"); ok && strings.HasPrefix(a.Value, "@layout/") {
			m.handleReference(pos(a), "layout", f.baseName, a.Value[len("@layout/"):], ignored)
		}
	case "item":
		switch f.folderTy {
		case "values":
			m.visitValuesItem(f, n)
		case "color":
			if a, ok := f.attrNS(n, androidNSURI, "color"); ok && strings.HasPrefix(a.Value, "@color/") {
				m.handleReference(cycleLoc{f.path, n.Line, n.Col}, "color", f.baseName, a.Value[len("@color/"):], ignored)
			}
		case "drawable":
			if a, ok := f.attrNS(n, androidNSURI, "drawable"); ok && strings.HasPrefix(a.Value, "@drawable/") {
				m.handleReference(cycleLoc{f.path, n.Line, n.Col}, "drawable", f.baseName, a.Value[len("@drawable/"):], ignored)
			}
		}
	case "font":
		if a, ok := f.attrNS(n, androidNSURI, "font"); ok && strings.HasPrefix(a.Value, "@font/") {
			m.handleReference(pos(a), "font", f.baseName, a.Value[len("@font/"):], ignored)
		}
	case "string", "color", "dimen":
		m.visitValueText(f, n, n.Tag, ignored)
	case "style":
		m.visitStyle(f, n, ignored)
	}
}

func attrByName(n *android.XMLNode, name string) (android.XMLAttribute, bool) {
	for _, a := range n.Attrs {
		if a.Name == name {
			return a, true
		}
	}
	return android.XMLAttribute{}, false
}

// textPos is the position of the first character data of an element.
func textPos(n *android.XMLNode) cycleLoc {
	if len(n.TextNodes) > 0 {
		return cycleLoc{"", n.TextNodes[0].Line, n.TextNodes[0].Col}
	}
	return cycleLoc{"", n.Line, n.Col}
}

// visitValuesItem handles <item name= type=>@type/x</item>. Lint only records the edge; a self reference is not reported.
func (m *cycleModule) visitValuesItem(f *cycleFile, n *android.XMLNode) {
	typeAttr, hasType := attrByName(n, "type")
	nameAttr, hasName := attrByName(n, "name")
	if !hasType || !hasName {
		return
	}
	typ := cycleTypeOf(typeAttr.Value)
	if typ == "" || !strings.HasPrefix(n.Text, "@"+typ) {
		return
	}
	m.addRef(typ, nameAttr.Value, referenceTarget(strings.TrimSpace(n.Text)))
	loc := textPos(n)
	loc.file = f.path
	m.addLoc(typ, nameAttr.Value, loc)
}

// visitValueText handles <string>/<color>/<dimen> whose text is a reference to a resource of the same type.
func (m *cycleModule) visitValueText(f *cycleFile, n *android.XMLNode, tag string, ignored bool) {
	if !strings.HasPrefix(n.Text, "@"+tag) {
		return
	}
	text := strings.TrimSpace(n.Text)
	if len(text) < len(tag)+2 {
		return
	}
	loc := textPos(n)
	loc.file = f.path
	name, _ := attrByName(n, "name")
	m.handleReference(loc, tag, name.Value, text[len(tag)+2:], ignored)
}

// referenceTarget is the part of a reference after the type, as recordReference stores it.
func referenceTarget(value string) string {
	if value == "" || strings.HasPrefix(value, "@android:") {
		return ""
	}
	if value[0] == '@' {
		if slash := strings.IndexByte(value, '/'); slash >= 0 {
			return value[slash+1:]
		}
	}
	return value
}

func (m *cycleModule) visitStyle(f *cycleFile, n *android.XMLNode, ignored bool) {
	name, hasName := attrByName(n, "name")
	parent, hasParent := attrByName(n, "parent")
	loc := func(a android.XMLAttribute) cycleLoc { return cycleLoc{f.path, a.Line, a.Col} }
	switch {
	case hasName && hasParent:
		if rest, ok := strings.CutPrefix(parent.Value, "@style/"); ok && strings.HasPrefix(rest, name.Value) &&
			strings.HasPrefix(rest[len(name.Value):], ".") {
			m.report(ignored, loc(parent), fmt.Sprintf("Potential cycle: `%s` is the implied parent of `%s` and this defines the opposite", name.Value, rest))
			return
		}
		if parent.Value == "" || strings.HasPrefix(parent.Value, "@android:style/") {
			return
		}
		m.handleReference(loc(parent), "style", name.Value, parent.Value[strings.LastIndexByte(parent.Value, '/')+1:], ignored)
		if strings.HasPrefix(parent.Value, "@") && !strings.Contains(parent.Value, "style/") {
			m.report(ignored, loc(parent), "Invalid parent reference: expected a @style")
		}
	case hasName:
		// A dotted name without a parent attribute inherits from the name before its last dot.
		if dot := strings.LastIndexByte(name.Value, '.'); dot > 0 {
			m.addRef("style", name.Value, name.Value[:dot])
			m.addLoc("style", name.Value, loc(name))
		}
	}
}

// visitAttribute handles references to a resource of the folder's own type, such as android:drawable in a drawable.
func (m *cycleModule) visitAttribute(f *cycleFile, a android.XMLAttribute, ignored bool) {
	v := a.Value
	if v == "" || v[0] != '@' || strings.HasPrefix(v, "@+id/") || strings.HasPrefix(v, "@id/") {
		return
	}
	// The first resource type of the folder decides which references count.
	typ := f.folderTy
	if !strings.HasPrefix(strings.ToLower(v), "@"+typ) {
		return
	}
	rest, ok := strings.CutPrefix(v, "@"+typ+"/")
	if !ok || rest == "" || strings.Contains(rest, "/") {
		return
	}
	if prefix, _, ok := strings.Cut(a.Name, ":"); ok && f.namespace[prefix] == lintToolsURI {
		return
	}
	m.handleReference(cycleLoc{f.path, a.Line, a.Col}, typ, f.baseName, rest, ignored)
}

func (m *cycleModule) addRef(typ, from, to string) {
	if to == "" {
		return
	}
	if m.refs[typ] == nil {
		m.refs[typ] = map[string][]string{}
	}
	m.refs[typ][from] = append(m.refs[typ][from], to)
}

func (m *cycleModule) addLoc(typ, from string, loc cycleLoc) {
	if m.locs[typ] == nil {
		m.locs[typ] = map[string][]cycleLoc{}
	}
	m.locs[typ][from] = append(m.locs[typ][from], loc)
}

func (m *cycleModule) report(ignored bool, loc cycleLoc, msg string) {
	if ignored {
		return
	}
	f := cycleFinding{loc: loc, msg: msg}
	if m.seen[f] {
		return
	}
	m.seen[f] = true
	m.findings = append(m.findings, f)
}

// handleReference reports a resource that refers to itself and records every other edge.
func (m *cycleModule) handleReference(loc cycleLoc, typ, from, to string, ignored bool) {
	if from == to {
		verb := "reference"
		switch typ {
		case "layout":
			verb = "include"
		case "style":
			verb = "extend"
		}
		m.report(ignored, loc, fmt.Sprintf("%s `%s` should not %s itself", cycleDisplayNames[typ], from, verb))
		return
	}
	m.addRef(typ, from, to)
	m.addLoc(typ, from, loc)
}

// findCycles looks for loops in the reference graph of each resource type and reports every loop once.
func (m *cycleModule) findCycles() {
	types := make([]string, 0, len(m.refs))
	for typ := range m.refs {
		types = append(types, typ)
	}
	sort.Strings(types)
	for _, typ := range types {
		refs := m.refs[typ]
		keys := make([]string, 0, len(refs))
		for k := range refs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		visiting := map[string]bool{}
		seen := map[string]bool{}
		for _, from := range keys {
			if seen[from] {
				continue
			}
			chain := cycleDFS(refs, from, visiting)
			// Self loops of two entries were already reported when the reference was read.
			if len(chain) <= 2 {
				continue
			}
			for _, name := range chain {
				seen[name] = true
			}
			slices.Reverse(chain)
			m.reportCycle(typ, chain)
		}
	}
}

// cycleDFS follows references from a name until one leads back into the path; it returns the path in reverse.
func cycleDFS(refs map[string][]string, from string, visiting map[string]bool) []string {
	visiting[from] = true
	for _, target := range refs[from] {
		if visiting[target] {
			return []string{target, from}
		}
		if chain := cycleDFS(refs, target, visiting); chain != nil {
			return append(chain, from)
		}
	}
	delete(visiting, from)
	return nil
}

// reportCycle reports a cycle at the first place the last name of the chain is defined.
func (m *cycleModule) reportCycle(typ string, chain []string) {
	var at *cycleLoc
	for _, name := range chain {
		if locs := m.locs[typ][name]; len(locs) > 0 {
			at = &locs[0]
		}
	}
	if at == nil {
		return
	}
	m.report(false, *at, fmt.Sprintf("%s Resource definition cycle: %s", cycleDisplayNames[typ], strings.Join(chain, " => ")))
}

// ResourceCycleRule mirrors Lint's ResourceCycle: a resource definition that refers back to itself.
type ResourceCycleRule struct {
	ResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *ResourceCycleRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ResourceCycleRule) check(ctx *api.Context) {
	emitTranslationFindings(ctx, r.BaseRule, lintResourceCycle)
}

func registerResourceCycle() {
	r := &ResourceCycleRule{AndroidRule: lintAndroidRule(lintResourceCycle, "Cycle in resource definitions", ALCCorrectness, ALSFatal, 8)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		Needs: api.NeedsResources, AndroidDeps: uint32(AndroidDepValues), Confidence: r.Confidence(), Implementation: r, Check: r.check,
	})
}
