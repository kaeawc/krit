package arch

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// AbiHashVersion is mixed into the hash input. Bump when the canonical
// signature encoding changes so consumers invalidate cached hashes on
// krit upgrade.
const AbiHashVersion uint32 = 1

// abiAnnotationAllowlist is the set of annotations that affect the binary
// surface and are therefore folded into the hash. Annotations outside this
// list (e.g. @VisibleForTesting) are ignored.
var abiAnnotationAllowlist = map[string]bool{
	"JvmStatic":     true,
	"JvmName":       true,
	"JvmField":      true,
	"JvmOverloads":  true,
	"JvmDefault":    true,
	"JvmSynthetic":  true,
	"Throws":        true,
	"Deprecated":    true,
	"Strictfp":      true,
	"Synchronized":  true,
	"Volatile":      true,
	"Transient":     true,
}

// abiModifierAllowlist is the set of declaration modifiers that are part
// of the ABI. Visibility (private/internal) is filtered earlier; protected
// is folded in here for the case where a class exposes both public and
// protected members.
var abiModifierAllowlist = map[string]bool{
	"open":        true,
	"final":       true,
	"abstract":    true,
	"sealed":      true,
	"inline":      true,
	"crossinline": true,
	"noinline":    true,
	"suspend":     true,
	"operator":    true,
	"infix":       true,
	"tailrec":     true,
	"external":    true,
	"vararg":      true,
	"data":        true,
	"value":       true,
	"enum":        true,
	"annotation":  true,
	"companion":   true,
	"const":       true,
	"protected":   true,
}

// AbiSignature is one declaration's normalized contribution to the ABI.
type AbiSignature struct {
	Kind        string   // class | interface | object | function | property
	FQN         string   // package + container chain + name
	Modifiers   []string // sorted, ABI-relevant modifiers only
	TypeParams  []string // raw text per type parameter, in declaration order
	Params      []AbiParam
	ReturnType  string
	Annotations []string // sorted, allowlisted annotation names only
}

// AbiParam captures parameter type and default-presence; names and default
// values are intentionally excluded.
type AbiParam struct {
	Type       string
	HasDefault bool
}

// ExtractAbiSignatures walks the public/protected declarations in files
// and returns deterministic, name-keyed signature records.
func ExtractAbiSignatures(files []*scanner.File) []AbiSignature {
	var sigs []AbiSignature
	for _, f := range files {
		if f == nil || f.FlatTree == nil {
			continue
		}
		sigs = append(sigs, extractFile(f)...)
	}
	sortSignatures(sigs)
	return sigs
}

// HashAbiSignatures returns the 16-character hex hash described in the
// abi-hash spec (first 8 bytes of SHA-256 over the canonical form,
// salted by AbiHashVersion).
func HashAbiSignatures(sigs []AbiSignature) string {
	h := sha256.New()
	var version [4]byte
	binary.BigEndian.PutUint32(version[:], AbiHashVersion)
	h.Write(version[:])
	for _, s := range sigs {
		writeCanonical(h, s)
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:8])
}

func sortSignatures(sigs []AbiSignature) {
	sort.Slice(sigs, func(i, j int) bool {
		if sigs[i].FQN != sigs[j].FQN {
			return sigs[i].FQN < sigs[j].FQN
		}
		if sigs[i].Kind != sigs[j].Kind {
			return sigs[i].Kind < sigs[j].Kind
		}
		return canonicalParams(sigs[i].Params) < canonicalParams(sigs[j].Params)
	})
}

func canonicalParams(params []AbiParam) string {
	var b strings.Builder
	for i, p := range params {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(p.Type)
		if p.HasDefault {
			b.WriteString("=?")
		}
	}
	return b.String()
}

func writeCanonical(w io.Writer, s AbiSignature) {
	parts := []string{
		s.Kind,
		s.FQN,
		strings.Join(s.Modifiers, ","),
		strings.Join(s.TypeParams, ";"),
		canonicalParams(s.Params),
		s.ReturnType,
		strings.Join(s.Annotations, ","),
	}
	w.Write([]byte(strings.Join(parts, "\x1f")))
	w.Write([]byte{'\n'})
}

// --- AST extraction --------------------------------------------------------

func extractFile(file *scanner.File) []AbiSignature {
	pkg := readPackage(file)
	var sigs []AbiSignature
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "class_declaration":
			if sig, ok := extractClass(file, idx, pkg); ok {
				sigs = append(sigs, sig)
			}
		case "object_declaration":
			if sig, ok := extractObject(file, idx, pkg); ok {
				sigs = append(sigs, sig)
			}
		case "function_declaration":
			if sig, ok := extractFunction(file, idx, pkg); ok {
				sigs = append(sigs, sig)
			}
		case "property_declaration":
			if sig, ok := extractProperty(file, idx, pkg); ok {
				sigs = append(sigs, sig)
			}
		}
	})
	return sigs
}

func readPackage(file *scanner.File) string {
	pkgIdx, ok := file.FlatFindChild(0, "package_header")
	if !ok {
		return ""
	}
	if id, ok := file.FlatFindChild(pkgIdx, "identifier"); ok {
		return strings.TrimSpace(file.FlatNodeText(id))
	}
	return ""
}

func declVisibility(file *scanner.File, idx uint32) string {
	switch {
	case file.FlatHasModifier(idx, "private"):
		return "private"
	case file.FlatHasModifier(idx, "internal"):
		return "internal"
	case file.FlatHasModifier(idx, "protected"):
		return "protected"
	default:
		return "public"
	}
}

func isOnSurface(file *scanner.File, idx uint32) bool {
	v := declVisibility(file, idx)
	if v != "public" && v != "protected" {
		return false
	}
	// Skip declarations nested inside private/internal containers.
	for parent, ok := file.FlatParent(idx); ok && parent != 0; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "class_declaration", "object_declaration":
			pv := declVisibility(file, parent)
			if pv != "public" && pv != "protected" {
				return false
			}
		}
	}
	return true
}

func enclosingChain(file *scanner.File, idx uint32) string {
	var names []string
	for parent, ok := file.FlatParent(idx); ok && parent != 0; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "class_declaration", "object_declaration":
			name := file.FlatChildTextOrEmpty(parent, "type_identifier")
			if name == "" {
				name = file.FlatChildTextOrEmpty(parent, "simple_identifier")
			}
			if name != "" {
				names = append(names, name)
			}
		}
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return strings.Join(names, ".")
}

func qualify(pkg, chain, name string) string {
	parts := make([]string, 0, 3)
	if pkg != "" {
		parts = append(parts, pkg)
	}
	if chain != "" {
		parts = append(parts, chain)
	}
	parts = append(parts, name)
	return strings.Join(parts, ".")
}

func collectModifiers(file *scanner.File, idx uint32) []string {
	mods, ok := file.FlatFindChild(idx, "modifiers")
	if !ok {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	// The modifiers node holds keywords either directly or wrapped in
	// classifier nodes (visibility_modifier, function_modifier, etc.), so
	// we look one level deep too.
	file.FlatForEachChild(mods, func(child uint32) {
		text := strings.TrimSpace(file.FlatNodeText(child))
		if abiModifierAllowlist[text] && !seen[text] {
			seen[text] = true
			out = append(out, text)
			return
		}
		file.FlatForEachChild(child, func(gc uint32) {
			t := strings.TrimSpace(file.FlatNodeText(gc))
			if abiModifierAllowlist[t] && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		})
	})
	sort.Strings(out)
	return out
}

func collectAnnotations(file *scanner.File, idx uint32) []string {
	mods, ok := file.FlatFindChild(idx, "modifiers")
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	file.FlatForEachChild(mods, func(child uint32) {
		t := file.FlatType(child)
		if t != "annotation" {
			return
		}
		// annotation -> user_type / constructor_invocation -> type_identifier
		name := annotationName(file, child)
		if name == "" || !abiAnnotationAllowlist[name] {
			return
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	})
	sort.Strings(out)
	return out
}

func annotationName(file *scanner.File, idx uint32) string {
	var found string
	var walk func(uint32)
	walk = func(n uint32) {
		if found != "" {
			return
		}
		if file.FlatType(n) == "type_identifier" {
			found = strings.TrimSpace(file.FlatNodeText(n))
			return
		}
		file.FlatForEachChild(n, walk)
	}
	walk(idx)
	return found
}

func collectTypeParams(file *scanner.File, idx uint32) []string {
	tps, ok := file.FlatFindChild(idx, "type_parameters")
	if !ok {
		return nil
	}
	var out []string
	file.FlatForEachChild(tps, func(child uint32) {
		if file.FlatType(child) != "type_parameter" {
			return
		}
		out = append(out, normalizeWhitespace(file.FlatNodeText(child)))
	})
	return out
}

func collectParams(file *scanner.File, idx uint32) []AbiParam {
	params, ok := file.FlatFindChild(idx, "function_value_parameters")
	if !ok {
		return nil
	}
	// A parameter has a default value when its next named sibling is not
	// itself a parameter — the grammar emits the default expression as a
	// sibling of the parameter node within function_value_parameters.
	var out []AbiParam
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "parameter" {
			out = append(out, AbiParam{Type: firstTypeChild(file, child, "")})
			continue
		}
		if n := len(out); n > 0 {
			out[n-1].HasDefault = true
		}
	}
	return out
}

func isTypeNode(t string) bool {
	switch t {
	case "user_type", "nullable_type", "function_type", "type_reference", "parenthesized_type":
		return true
	}
	return false
}

// firstTypeChild returns the text of the first child type node, optionally
// after a sentinel child type (e.g. "function_value_parameters" for return
// types). Stops at body/equals tokens.
func firstTypeChild(file *scanner.File, idx uint32, after string) string {
	skipping := after != ""
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		if skipping {
			if t == after {
				skipping = false
			}
			continue
		}
		if isTypeNode(t) {
			return normalizeWhitespace(file.FlatNodeText(child))
		}
		if t == "function_body" || t == "block" || t == "=" {
			return ""
		}
	}
	return ""
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func extractClass(file *scanner.File, idx uint32, pkg string) (AbiSignature, bool) {
	if !isOnSurface(file, idx) {
		return AbiSignature{}, false
	}
	name := file.FlatChildTextOrEmpty(idx, "type_identifier")
	if name == "" {
		name = file.FlatChildTextOrEmpty(idx, "simple_identifier")
	}
	if name == "" {
		return AbiSignature{}, false
	}
	kind := "class"
	if file.FlatHasChildOfType(idx, "interface") {
		kind = "interface"
	}
	chain := enclosingChain(file, idx)
	primary := primaryConstructor(file, idx)
	sig := AbiSignature{
		Kind:        kind,
		FQN:         qualify(pkg, chain, name),
		Modifiers:   collectModifiers(file, idx),
		TypeParams:  collectTypeParams(file, idx),
		Params:      primary,
		Annotations: collectAnnotations(file, idx),
	}
	return sig, true
}

func primaryConstructor(file *scanner.File, idx uint32) []AbiParam {
	pc, ok := file.FlatFindChild(idx, "primary_constructor")
	if !ok {
		return nil
	}
	cps, ok := file.FlatFindChild(pc, "class_parameters")
	if !ok {
		return nil
	}
	var out []AbiParam
	file.FlatForEachChild(cps, func(child uint32) {
		if file.FlatType(child) != "class_parameter" {
			return
		}
		out = append(out, AbiParam{Type: firstTypeChild(file, child, "")})
	})
	return out
}

func extractObject(file *scanner.File, idx uint32, pkg string) (AbiSignature, bool) {
	if !isOnSurface(file, idx) {
		return AbiSignature{}, false
	}
	name := file.FlatChildTextOrEmpty(idx, "type_identifier")
	if name == "" {
		name = file.FlatChildTextOrEmpty(idx, "simple_identifier")
	}
	if name == "" {
		return AbiSignature{}, false
	}
	chain := enclosingChain(file, idx)
	return AbiSignature{
		Kind:        "object",
		FQN:         qualify(pkg, chain, name),
		Modifiers:   collectModifiers(file, idx),
		Annotations: collectAnnotations(file, idx),
	}, true
}

func extractFunction(file *scanner.File, idx uint32, pkg string) (AbiSignature, bool) {
	if !isOnSurface(file, idx) {
		return AbiSignature{}, false
	}
	if file.FlatHasModifier(idx, "override") {
		return AbiSignature{}, false
	}
	name := file.FlatChildTextOrEmpty(idx, "simple_identifier")
	if name == "" {
		return AbiSignature{}, false
	}
	chain := enclosingChain(file, idx)
	return AbiSignature{
		Kind:        "function",
		FQN:         qualify(pkg, chain, name),
		Modifiers:   collectModifiers(file, idx),
		TypeParams:  collectTypeParams(file, idx),
		Params:      collectParams(file, idx),
		ReturnType:  firstTypeChild(file, idx, "function_value_parameters"),
		Annotations: collectAnnotations(file, idx),
	}, true
}

func extractProperty(file *scanner.File, idx uint32, pkg string) (AbiSignature, bool) {
	if !isOnSurface(file, idx) {
		return AbiSignature{}, false
	}
	if file.FlatHasModifier(idx, "override") {
		return AbiSignature{}, false
	}
	// Only top-level or class-body properties contribute to the ABI.
	parent, ok := file.FlatParent(idx)
	if !ok {
		return AbiSignature{}, false
	}
	parentType := file.FlatType(parent)
	if parentType != "source_file" && parentType != "class_body" {
		return AbiSignature{}, false
	}
	name := file.FlatChildTextOrEmpty(idx, "simple_identifier")
	if name == "" {
		if varDecl, ok := file.FlatFindChild(idx, "variable_declaration"); ok {
			name = file.FlatChildTextOrEmpty(varDecl, "simple_identifier")
		}
	}
	if name == "" || name == "_" {
		return AbiSignature{}, false
	}
	chain := enclosingChain(file, idx)
	return AbiSignature{
		Kind:        "property",
		FQN:         qualify(pkg, chain, name),
		Modifiers:   collectModifiers(file, idx),
		ReturnType:  firstTypeChild(file, idx, ""),
		Annotations: collectAnnotations(file, idx),
	}, true
}
