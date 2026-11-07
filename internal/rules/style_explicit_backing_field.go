package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ExplicitBackingFieldCandidateRule flags the classic backing-property pair:
//
//	private val _state = MutableStateFlow(0)
//	val state: StateFlow<Int> get() = _state      // or = _state.asStateFlow()
//
// Kotlin 2.4 stable explicit backing fields express the same thing with one
// declaration, `val state: StateFlow<Int> field = MutableStateFlow(0)`, and
// inside the declaring class `state` is smart cast to the field type. The Go
// rule is syntactic: it needs a known mutable family (flows, LiveData, Compose
// state, standard mutable collections, channels) on the private property and a
// declared read-only supertype (or an `asStateFlow()`/`asSharedFlow()` wrapper)
// on the public one. The finding is on the private property.
type ExplicitBackingFieldCandidateRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Types come from
// declared types and well-known factory names, not from resolution.
func (r *ExplicitBackingFieldCandidateRule) Confidence() float64 { return api.ConfidenceMedium }

// ebfFamily is a group of mutable types with the read-only supertypes that a
// public property may declare for the same instance.
type ebfFamily struct {
	supers       map[string]bool
	importPrefix string
	wrappers     map[string]bool
}

func newEbfFamily(importPrefix string, supers []string, wrappers ...string) *ebfFamily {
	f := &ebfFamily{supers: map[string]bool{}, importPrefix: importPrefix, wrappers: map[string]bool{}}
	for _, s := range supers {
		f.supers[s] = true
	}
	for _, w := range wrappers {
		f.wrappers[w] = true
	}
	return f
}

// ebfFamilies maps a type or factory name written on the private property to
// its family.
var ebfFamilies = func() map[string]*ebfFamily {
	stateFlow := newEbfFamily("kotlinx.coroutines.flow.", []string{"StateFlow", "SharedFlow", "Flow"}, "asStateFlow", "asSharedFlow")
	sharedFlow := newEbfFamily("kotlinx.coroutines.flow.", []string{"SharedFlow", "Flow"}, "asSharedFlow")
	liveData := newEbfFamily("androidx.lifecycle.", []string{"LiveData"})
	state := newEbfFamily("androidx.compose.runtime.", []string{"State"})
	list := newEbfFamily("", []string{"List", "Collection", "Iterable"})
	set := newEbfFamily("", []string{"Set", "Collection", "Iterable"})
	mapFamily := newEbfFamily("", []string{"Map"})
	channel := newEbfFamily("kotlinx.coroutines.channels.", []string{"ReceiveChannel"})
	out := map[string]*ebfFamily{}
	add := func(f *ebfFamily, names ...string) {
		for _, n := range names {
			out[n] = f
		}
	}
	add(stateFlow, "MutableStateFlow")
	add(sharedFlow, "MutableSharedFlow")
	add(liveData, "MutableLiveData", "MediatorLiveData")
	add(state, "MutableState", "mutableStateOf")
	add(list, "MutableList", "mutableListOf", "ArrayList", "arrayListOf", "mutableStateListOf")
	add(set, "MutableSet", "mutableSetOf", "HashSet", "hashSetOf", "LinkedHashSet", "linkedSetOf")
	add(mapFamily, "MutableMap", "mutableMapOf", "HashMap", "hashMapOf", "LinkedHashMap", "linkedMapOf", "mutableStateMapOf")
	add(channel, "Channel")
	return out
}()

// ebfProp is the parsed shape of a property_declaration.
type ebfProp struct {
	idx           uint32
	name          string
	nameNode      uint32
	declType      string // written type after the colon, "" when inferred
	declEnd       uint32 // end byte of the name-and-type part
	init          uint32 // initializer expression, 0 when absent
	getter        uint32 // getter node (child or following sibling), 0 when absent
	siblingGetter bool   // the getter is the next sibling, not a child
	regionEnd     uint32 // end byte including a sibling getter
	mods          uint32
	isVal         bool
}

// ebfParseProp returns false for delegated, extension, generic, setter-carrying
// or destructuring properties, which never take part in the pattern.
func ebfParseProp(file *scanner.File, idx uint32) (ebfProp, bool) {
	p := ebfProp{idx: idx, regionEnd: file.FlatEndByte(idx)}
	sawEq := false
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "modifiers":
			p.mods = c
		case "binding_pattern_kind":
			p.isVal = file.FlatNodeText(c) == "val"
		case "variable_declaration":
			if p.nameNode != 0 {
				return p, false
			}
			p.declEnd = file.FlatEndByte(c)
			for n := file.FlatFirstChild(c); n != 0; n = file.FlatNextSib(n) {
				if file.FlatType(n) == "simple_identifier" && p.nameNode == 0 {
					p.nameNode = n
					p.name = file.FlatNodeText(n)
				}
			}
			if t := extractPropertyTypeFlat(file, idx); t != "" {
				p.declType = strings.TrimSpace(t)
			}
		case "=":
			sawEq = true
		case "getter":
			p.getter = c
		case "property_delegate", "setter", "multi_variable_declaration", "type_parameters", "type_constraints":
			return p, false
		default:
			if sawEq && p.init == 0 {
				p.init = c
			}
		}
	}
	if p.nameNode == 0 || !p.isVal {
		return p, false
	}
	// Anything between the binding keyword and the name is a receiver or type parameter list.
	if kind, ok := file.FlatFindChild(idx, "binding_pattern_kind"); ok {
		between := strings.TrimSpace(string(file.Content[file.FlatEndByte(kind):file.FlatStartByte(p.nameNode)]))
		if between != "" {
			return p, false
		}
	}
	if next, ok := file.FlatNextSibling(idx); ok {
		switch file.FlatType(next) {
		case "getter":
			if p.getter == 0 && p.init == 0 {
				p.getter = next
				p.siblingGetter = true
				p.regionEnd = file.FlatEndByte(next)
			}
		case "setter":
			return p, false
		}
	}
	return p, true
}

// ebfTypeName is the simple name of a written type or callee, without
// qualifiers or type arguments.
func ebfTypeName(text string) string {
	text = strings.TrimSpace(text)
	if i := strings.IndexAny(text, "<("); i >= 0 {
		text = text[:i]
	}
	if i := strings.LastIndex(text, "."); i >= 0 {
		text = text[i+1:]
	}
	return strings.TrimSpace(text)
}

// ebfBackingFamily returns the family of a private property from its declared
// type, else from the factory call that initializes it.
func ebfBackingFamily(file *scanner.File, p ebfProp) *ebfFamily {
	if p.declType != "" {
		if strings.HasSuffix(p.declType, "?") {
			return nil
		}
		return ebfFamilies[ebfTypeName(p.declType)]
	}
	if p.init == 0 || file.FlatType(p.init) != "call_expression" {
		return nil
	}
	callee := file.FlatFirstChild(p.init)
	if callee == 0 {
		return nil
	}
	switch file.FlatType(callee) {
	case "simple_identifier", "navigation_expression":
		return ebfFamilies[ebfTypeName(file.FlatNodeText(callee))]
	}
	return nil
}

// ebfIsBackingRef reports `name` or `this.name`.
func ebfIsBackingRef(file *scanner.File, n uint32, name string) bool {
	switch file.FlatType(n) {
	case "simple_identifier":
		return file.FlatNodeText(n) == name
	case "navigation_expression":
		return file.FlatNodeText(n) == "this."+name
	}
	return false
}

// ebfExposedWrapper reports whether expr exposes the backing property
// unchanged (`_x`) or through a read-only wrapper (`_x.asStateFlow()`).
func ebfExposedWrapper(file *scanner.File, expr uint32, backing string) (wrapper string, ok bool) {
	if expr == 0 {
		return "", false
	}
	if ebfIsBackingRef(file, expr, backing) {
		return "", true
	}
	if file.FlatType(expr) != "call_expression" {
		return "", false
	}
	nav := file.FlatFirstChild(expr)
	if nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return "", false
	}
	recv := file.FlatFirstChild(nav)
	suffix := file.FlatNextSib(recv)
	if recv == 0 || suffix == 0 || file.FlatType(suffix) != "navigation_suffix" || !ebfIsBackingRef(file, recv, backing) {
		return "", false
	}
	name := strings.TrimPrefix(file.FlatNodeText(suffix), ".")
	if name != "asStateFlow" && name != "asSharedFlow" {
		return "", false
	}
	if call, ok := file.FlatFindChild(expr, "call_suffix"); !ok || strings.TrimSpace(file.FlatNodeText(call)) != "()" {
		return "", false
	}
	return name, true
}

// ebfGetterExpr is the returned expression of `get() = x` or `get() { return x }`.
func ebfGetterExpr(file *scanner.File, getter uint32) uint32 {
	body, ok := file.FlatFindChild(getter, "function_body")
	if !ok {
		return 0
	}
	first := file.FlatFirstChild(body)
	if first == 0 {
		return 0
	}
	if file.FlatType(first) == "=" {
		return file.FlatNextSib(first)
	}
	stmts, ok := file.FlatFindChild(body, "statements")
	if !ok || file.FlatChildCount(stmts) != 1 {
		return 0
	}
	ret := file.FlatFirstChild(stmts)
	if file.FlatType(ret) != "jump_expression" {
		return 0
	}
	kw := file.FlatFirstChild(ret)
	if kw == 0 || file.FlatType(kw) != "return" {
		return 0
	}
	return file.FlatNextSib(kw)
}

// ebfPublicMods checks the modifiers of the exposing property: visibility other
// than private, optionally override, and no other modifier or JvmField.
func ebfPublicMods(file *scanner.File, p ebfProp) (override, ok bool) {
	if p.mods == 0 {
		return false, true
	}
	for m := file.FlatFirstChild(p.mods); m != 0; m = file.FlatNextSib(m) {
		switch file.FlatType(m) {
		case "visibility_modifier":
			if file.FlatNodeText(m) == "private" {
				return false, false
			}
		case "member_modifier":
			if file.FlatNodeText(m) != "override" {
				return false, false
			}
			override = true
		case "annotation":
			if strings.Contains(file.FlatNodeText(m), "JvmField") {
				return false, false
			}
		default:
			return false, false
		}
	}
	return override, true
}

// ebfLanguageSupport reports whether explicit backing fields (stable in Kotlin
// 2.4) are available and whether that came from a known project version. An
// unknown version reports with low confidence.
func ebfLanguageSupport(facts *librarymodel.Facts) (supported, known bool) {
	if facts == nil {
		return true, false
	}
	k := facts.Profile.Kotlin
	version := ""
	if k.LanguageVersion.Presence == librarymodel.PresencePresent {
		version = k.LanguageVersion.Version
	}
	if version == "" && k.Compiler.Presence == librarymodel.PresencePresent {
		version = k.Compiler.Version
	}
	m := ebfVersionRe.FindStringSubmatch(version)
	if m == nil {
		return true, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major > 2 || (major == 2 && minor >= 4), true
}

var ebfVersionRe = regexp.MustCompile(`^(\d+)\.(\d+)`)

func checkExplicitBackingFieldCandidate(ctx *api.Context, r *ExplicitBackingFieldCandidateRule) {
	idx, file := ctx.Idx, ctx.File
	if !file.FlatHasModifier(idx, "private") {
		return
	}
	back, ok := ebfParseProp(file, idx)
	if !ok || back.init == 0 || back.getter != 0 || back.mods == 0 || strings.TrimSpace(file.FlatNodeText(back.mods)) != "private" {
		return
	}
	container, ok := file.FlatParent(idx)
	if !ok {
		return
	}
	var owner uint32
	switch file.FlatType(container) {
	case "source_file":
	case "class_body":
		owner, ok = file.FlatParent(container)
		if !ok {
			return
		}
		switch file.FlatType(owner) {
		case "class_declaration":
			if abstractRuleIsInterface(file, owner) {
				return
			}
		case "object_declaration", "companion_object":
		default:
			return
		}
	default:
		return
	}
	fam := ebfBackingFamily(file, back)
	if fam == nil || !ebfHasImportEvidence(file, fam) {
		return
	}
	var exposures []ebfProp
	for sib := file.FlatFirstChild(container); sib != 0; sib = file.FlatNextSib(sib) {
		if sib == idx || file.FlatType(sib) != "property_declaration" {
			continue
		}
		if exp, ok := ebfExposure(file, sib, back.name, fam, owner); ok {
			exposures = append(exposures, exp)
		}
	}
	if len(exposures) != 1 {
		return
	}
	exp := exposures[0]
	supported, known := ebfLanguageSupport(ctx.LibraryFacts)
	if !supported {
		return
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf(
		"Backing property '%s' only exposes '%s'; declare '%s' with an explicit backing field (`field = ...`) and remove '%s'.",
		back.name, exp.name, exp.name, back.name))
	if !known {
		f.Confidence = api.ConfidenceLow
	}
	f.Fix = ebfBuildFix(file, container, back, exp)
	ctx.Emit(f)
}

// ebfHasImportEvidence requires the family's library import (or a qualified
// use) so a project type with the same name is not matched.
func ebfHasImportEvidence(file *scanner.File, fam *ebfFamily) bool {
	if fam.importPrefix == "" {
		return true
	}
	return strings.Contains(string(file.Content), fam.importPrefix)
}

// ebfExposure returns the property when it only exposes backing as a read-only
// supertype of the backing family.
func ebfExposure(file *scanner.File, sib uint32, backing string, fam *ebfFamily, owner uint32) (ebfProp, bool) {
	exp, ok := ebfParseProp(file, sib)
	if !ok || exp.name == backing {
		return exp, false
	}
	override, ok := ebfPublicMods(file, exp)
	if !ok {
		return exp, false
	}
	// An override is only effectively final in a class that cannot be subclassed.
	if override {
		if owner == 0 || file.FlatType(owner) == "companion_object" {
			return exp, false
		}
		if file.FlatType(owner) == "class_declaration" &&
			(file.FlatHasModifier(owner, "open") || file.FlatHasModifier(owner, "abstract") || file.FlatHasModifier(owner, "sealed")) {
			return exp, false
		}
	}
	var expr uint32
	switch {
	case exp.init != 0 && exp.getter == 0:
		expr = exp.init
	case exp.getter != 0 && exp.init == 0:
		expr = ebfGetterExpr(file, exp.getter)
	}
	wrapper, ok := ebfExposedWrapper(file, expr, backing)
	if !ok {
		return exp, false
	}
	if wrapper != "" && !fam.wrappers[wrapper] {
		return exp, false
	}
	if exp.declType == "" {
		// Without a declared type only a wrapper pins a read-only type.
		return exp, wrapper != ""
	}
	if strings.HasSuffix(exp.declType, "?") || !fam.supers[ebfTypeName(exp.declType)] {
		return exp, false
	}
	return exp, true
}

// ebfEdit is one byte-range replacement.
type ebfEdit struct {
	start, end int
	text       string
}

// ebfBuildFix returns the rewrite that moves the initializer into an explicit
// backing field of the public property and renames the uses, or nil when any
// part of it is not provably safe.
func ebfBuildFix(file *scanner.File, container uint32, back, exp ebfProp) *scanner.Fix {
	// The pinned grammar reads a top-level `field = ...` as an assignment statement, so krit cannot re-parse the output.
	if exp.declType == "" || file.FlatType(container) != "class_body" {
		return nil
	}
	// Both declarations are adjacent (no other member or comment between them),
	// so moving the initializer does not change the initialization order.
	expRegionLast := exp.idx
	if exp.siblingGetter {
		expRegionLast = exp.getter
	}
	backFirst := file.FlatNextSib(back.idx) == exp.idx
	expFirst := file.FlatNextSib(expRegionLast) == back.idx
	if !backFirst && !expFirst {
		return nil
	}
	if backFirst {
		if prev, ok := file.FlatPrevSibling(back.idx); ok && ebfIsComment(file, prev) {
			return nil
		}
	}
	content := file.Content
	backStart, ok := ebfLineStart(content, int(file.FlatStartByte(back.idx)))
	if !ok {
		return nil
	}
	backEnd, ok := ebfLineEnd(content, int(file.FlatEndByte(back.idx)))
	if !ok {
		return nil
	}
	expIndentStart, ok := ebfLineStart(content, int(file.FlatStartByte(exp.idx)))
	if !ok {
		return nil
	}
	indent := string(content[expIndentStart:file.FlatStartByte(exp.idx)])
	step := ebfIndentStep(file, exp.idx, indent)

	initText := file.FlatNodeText(back.init)
	field := "field = " + initText
	if back.declType != "" {
		field = "field: " + back.declType + " = " + initText
	}
	edits := []ebfEdit{
		{backStart, backEnd, ""},
		{int(exp.declEnd), int(exp.regionEnd), "\n" + indent + step + field},
	}

	uses, ok := ebfRenameUses(file, back, exp)
	if !ok {
		return nil
	}
	for _, u := range uses {
		edits = append(edits, ebfEdit{int(file.FlatStartByte(u)), int(file.FlatEndByte(u)), exp.name})
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	start, end := edits[0].start, edits[0].end
	for i, e := range edits {
		if i > 0 && e.start < edits[i-1].end {
			return nil
		}
		end = max(end, e.end)
	}
	var b strings.Builder
	pos := start
	for _, e := range edits {
		b.Write(content[pos:e.start])
		b.WriteString(e.text)
		pos = e.end
	}
	return &scanner.Fix{ByteMode: true, StartByte: start, EndByte: end, Replacement: b.String()}
}

// ebfIndentStep is one indent level of the file, guessed from how deep the
// property sits and how far it is indented.
func ebfIndentStep(file *scanner.File, idx uint32, indent string) string {
	if strings.Contains(indent, "\t") {
		return "\t"
	}
	depth := 0
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) == "class_body" {
			depth++
		}
	}
	if depth > 0 && len(indent) > 0 && len(indent)%depth == 0 {
		return strings.Repeat(" ", len(indent)/depth)
	}
	return "    "
}

func ebfIsComment(file *scanner.File, n uint32) bool {
	t := file.FlatType(n)
	return t == "line_comment" || t == "multiline_comment" || t == "comment"
}

// ebfLineStart returns the start of the line holding pos when only blanks
// precede pos on it.
func ebfLineStart(content []byte, pos int) (int, bool) {
	i := pos
	for i > 0 && (content[i-1] == ' ' || content[i-1] == '\t') {
		i--
	}
	if i > 0 && content[i-1] != '\n' {
		return 0, false
	}
	return i, true
}

// ebfLineEnd returns the offset after the line break following pos when only
// blanks sit between.
func ebfLineEnd(content []byte, pos int) (int, bool) {
	i := pos
	for i < len(content) && (content[i] == ' ' || content[i] == '\t' || content[i] == '\r') {
		i++
	}
	if i >= len(content) {
		return i, true
	}
	if content[i] != '\n' {
		return 0, false
	}
	return i + 1, true
}

func ebfIsIdentByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// ebfCountIdent counts whole-word occurrences of name in text.
func ebfCountIdent(text, name string) int {
	n := 0
	for i := 0; ; {
		j := strings.Index(text[i:], name)
		if j < 0 {
			return n
		}
		s, e := i+j, i+j+len(name)
		if (s == 0 || !ebfIsIdentByte(text[s-1])) && (e >= len(text) || !ebfIsIdentByte(text[e])) {
			n++
		}
		i = e
	}
}

// ebfRenameUses returns every use of the backing property that can be renamed
// to the public one, or false when any use or name clash makes that unsafe.
func ebfRenameUses(file *scanner.File, back, exp ebfProp) ([]uint32, bool) {
	expStart, expEnd := file.FlatStartByte(exp.idx), exp.regionEnd
	var uses []uint32
	nodeCount := 0
	clash := false
	file.FlatWalkNodes(0, "simple_identifier", func(n uint32) {
		text := file.FlatNodeText(n)
		if text == exp.name && n != exp.nameNode {
			if p, ok := file.FlatParent(n); ok {
				switch file.FlatType(p) {
				case "variable_declaration", "parameter", "class_parameter":
					clash = true
				}
			}
		}
		if text != back.name {
			return
		}
		nodeCount++
		if n == back.nameNode || (file.FlatStartByte(n) >= expStart && file.FlatEndByte(n) <= expEnd) {
			return
		}
		uses = append(uses, n)
	})
	if clash {
		return nil, false
	}
	// Every textual occurrence must be a node (or sit in a comment); a use inside a string template is not a node.
	text := string(file.Content)
	commentCount := 0
	for _, t := range []string{"line_comment", "multiline_comment"} {
		file.FlatWalkNodes(0, t, func(n uint32) { commentCount += ebfCountIdent(file.FlatNodeText(n), back.name) })
	}
	if ebfCountIdent(text, back.name) != nodeCount+commentCount {
		return nil, false
	}
	for _, u := range uses {
		parent, ok := file.FlatParent(u)
		if !ok || file.FlatFirstChild(parent) != u {
			return nil, false
		}
		switch file.FlatType(parent) {
		case "navigation_expression", "directly_assignable_expression":
		default:
			return nil, false
		}
		if next := file.FlatNextSib(u); next == 0 || file.FlatType(next) != "navigation_suffix" {
			return nil, false
		}
	}
	return uses, true
}
