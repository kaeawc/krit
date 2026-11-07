package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// DoubleMutabilityForCollectionRule detects var with mutable collection type.
// ---------------------------------------------------------------------------
type DoubleMutabilityForCollectionRule struct {
	FlatDispatchBase
	BaseRule
	MutableTypes []string
}

// Confidence reports a tier-2 (medium) base confidence. The rule uses a fixed
// allow-list of mutable-collection type names and factory function names; it
// correctly flags the common case (`var xs = mutableListOf<Foo>()`) with local
// AST/import evidence, but won't detect the pattern when the collection is
// returned from a wrapper function. Medium confidence matches the known
// scope-analysis gap.
func (r *DoubleMutabilityForCollectionRule) Confidence() float64 { return api.ConfidenceMedium }

var defaultDoubleMutableTypes = []string{
	"MutableList", "MutableSet", "MutableMap", "MutableCollection",
	"ArrayList", "HashMap", "HashSet", "LinkedHashMap", "LinkedHashSet",
}

var mutableCollectionFactories = map[string]bool{
	"mutableListOf": true,
	"mutableSetOf":  true,
	"mutableMapOf":  true,
	"arrayListOf":   true,
	"hashMapOf":     true,
	"hashSetOf":     true,
	"linkedMapOf":   true,
	"linkedSetOf":   true,
	"ArrayList":     true,
	"HashMap":       true,
	"HashSet":       true,
	"LinkedHashMap": true,
	"LinkedHashSet": true,
}

func (r *DoubleMutabilityForCollectionRule) configuredMutableTypes() map[string]bool {
	types := r.MutableTypes
	if len(types) == 0 {
		types = defaultDoubleMutableTypes
	}
	out := make(map[string]bool, len(types))
	for _, t := range types {
		if t == "" {
			continue
		}
		out[simpleTypeName(t)] = true
	}
	return out
}

func doubleMutabilityHasExplicitMutableType(file *scanner.File, varDecl uint32, mutableTypes map[string]bool) bool {
	if file == nil || varDecl == 0 {
		return false
	}
	for child := file.FlatFirstChild(varDecl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type":
			if doubleMutabilityTypeNodeHasMutableEvidence(file, child, mutableTypes) {
				return true
			}
		}
	}
	return false
}

func doubleMutabilityTypeNodeHasMutableEvidence(file *scanner.File, typeNode uint32, mutableTypes map[string]bool) bool {
	typeText := file.FlatNodeText(typeNode)
	return doubleMutabilityTypeTextHasMutableEvidence(file, typeText, mutableTypes)
}

func doubleMutabilityTypeTextHasMutableEvidence(file *scanner.File, typeText string, mutableTypes map[string]bool) bool {
	typeName := strings.TrimSpace(typeText)
	typeName = strings.TrimSuffix(typeName, "?")
	if idx := strings.Index(typeName, "<"); idx >= 0 {
		typeName = strings.TrimSpace(typeName[:idx])
	}
	simple := simpleTypeName(typeName)
	if simple == "" || !mutableTypes[simple] {
		return false
	}
	if strings.Contains(typeName, ".") {
		return knownMutableCollectionFQN(typeName) || !knownDefaultMutableCollectionName(simple)
	}
	if !knownDefaultMutableCollectionName(simple) {
		return true
	}
	return !doubleMutabilitySimpleTypeShadowed(file, simple)
}

func knownDefaultMutableCollectionName(name string) bool {
	for _, typ := range defaultDoubleMutableTypes {
		if typ == name {
			return true
		}
	}
	return false
}

func knownMutableCollectionFQN(fqn string) bool {
	switch fqn {
	case "kotlin.collections.MutableList",
		"kotlin.collections.MutableSet",
		"kotlin.collections.MutableMap",
		"kotlin.collections.MutableCollection",
		"kotlin.collections.ArrayList",
		"kotlin.collections.HashMap",
		"kotlin.collections.HashSet",
		"kotlin.collections.LinkedHashMap",
		"kotlin.collections.LinkedHashSet",
		"java.util.ArrayList",
		"java.util.HashMap",
		"java.util.HashSet",
		"java.util.LinkedHashMap",
		"java.util.LinkedHashSet":
		return true
	default:
		return false
	}
}

// doubleMutabilityShadowSet holds the per-file set of simple type names
// that are shadowed by a local declaration or non-collection import,
// plus a sentinel indicating an unrelated wildcard import (which makes
// every name potentially shadowed).
type doubleMutabilityShadowSet struct {
	declared       map[string]struct{}
	importedShadow map[string]struct{}
	wildcardShadow bool
}

func doubleMutabilityShadows(file *scanner.File) *doubleMutabilityShadowSet {
	return filefacts.FileFact(fileFactsCache(), file, slotDoubleMutabilityShadows, func() *doubleMutabilityShadowSet {
		set := &doubleMutabilityShadowSet{
			declared:       map[string]struct{}{},
			importedShadow: map[string]struct{}{},
		}
		for _, nodeType := range []string{"class_declaration", "object_declaration", "interface_declaration", "type_alias"} {
			file.FlatWalkNodes(0, nodeType, func(node uint32) {
				if name := extractIdentifierFlat(file, node); name != "" {
					set.declared[name] = struct{}{}
				}
			})
		}
		file.FlatWalkNodes(0, "import_header", func(node uint32) {
			text := cleanImportHeaderTextWithAlias(file.FlatNodeText(node))
			if alias := strings.Index(text, " as "); alias >= 0 {
				fqn := strings.TrimSpace(text[:alias])
				asName := strings.TrimSpace(text[alias+4:])
				if asName != "" && !knownMutableCollectionFQN(fqn) {
					set.importedShadow[asName] = struct{}{}
				}
				return
			}
			if strings.HasSuffix(text, ".*") {
				pkg := strings.TrimSuffix(text, ".*")
				if pkg != "kotlin.collections" && pkg != "java.util" {
					set.wildcardShadow = true
				}
				return
			}
			if dot := strings.LastIndex(text, "."); dot >= 0 && !knownMutableCollectionFQN(text) {
				set.importedShadow[text[dot+1:]] = struct{}{}
			}
		})
		return set
	})
}

func doubleMutabilitySimpleTypeShadowed(file *scanner.File, simple string) bool {
	if file == nil || simple == "" {
		return false
	}
	set := doubleMutabilityShadows(file)
	if _, ok := set.declared[simple]; ok {
		return true
	}
	if _, ok := set.importedShadow[simple]; ok {
		return true
	}
	return set.wildcardShadow
}

func doubleMutabilityInitializerLooksLikeMutableFactory(file *scanner.File, prop uint32) bool {
	init := propertyInitializerExpression(file, prop)
	if init == 0 || file.FlatType(init) != "call_expression" {
		return false
	}
	return mutableCollectionFactories[flatCallExpressionName(file, init)]
}

// ---------------------------------------------------------------------------
// DontDowncastCollectionTypesRule detects `as MutableList`, `as MutableMap`, etc.
// With type inference: uses ClassHierarchy on source and target of a cast to verify
// the source is a supertype of the target in the collection hierarchy.
// ---------------------------------------------------------------------------
type DontDowncastCollectionTypesRule struct {
	FlatDispatchBase
	BaseRule
}

// dontDowncastFixMethods maps the mutable targets that have a copying conversion to that conversion.
var dontDowncastFixMethods = map[string]string{
	"MutableList": "toMutableList()",
	"MutableSet":  "toMutableSet()",
	"MutableMap":  "toMutableMap()",
}

// dontDowncastImmutable and dontDowncastMutable are detekt 1.23.8's type lists.
var dontDowncastImmutable = map[string]bool{"List": true, "Map": true, "Set": true}

var dontDowncastMutable = map[string]bool{
	"MutableList": true, "MutableMap": true, "MutableSet": true, "ArrayList": true,
	"LinkedHashSet": true, "HashSet": true, "LinkedHashMap": true, "HashMap": true,
}

func (r *DontDowncastCollectionTypesRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if file == nil || idx == 0 || ctx.Resolver == nil {
		return
	}
	// The source is the first named child, the target type the last (`x as T`, `x is T`, `x !is T`).
	sourceIdx := uint32(0)
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			sourceIdx = child
			break
		}
	}
	typeNode := lastNamedChild(file, idx)
	// Only a plain user type counts: `as MutableList<T>?` is a nullable type and never reports.
	if sourceIdx == 0 || typeNode == sourceIdx || file.FlatType(typeNode) != "user_type" {
		return
	}
	typeIdent := uint32(0)
	for c := file.FlatFirstChild(typeNode); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "type_identifier" {
			typeIdent = c
		}
	}
	if typeIdent == 0 {
		return
	}
	targetType := file.FlatNodeText(typeIdent)
	if !dontDowncastMutable[targetType] {
		return
	}
	sourceType := ctx.Resolver.ResolveFlatNode(sourceIdx, file)
	if sourceType == nil || sourceType.Kind == typeinfer.TypeUnknown || !dontDowncastImmutable[resolvedTypeShortName(sourceType)] {
		return
	}

	msg := fmt.Sprintf("Down-casting from type %s to %s is risky.", resolvedTypeShortName(sourceType), targetType)
	if strings.HasPrefix(targetType, "Mutable") {
		msg += fmt.Sprintf(" Use `to%s()` instead.", targetType)
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
	f.Confidence = 0.9
	if method, ok := dontDowncastFixMethods[targetType]; ok && file.FlatType(idx) == "as_expression" {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(idx)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: file.FlatNodeText(sourceIdx) + "." + method,
		}
	}
	ctx.Emit(f)
}

// resolvedTypeShortName is the simple class name of t without type arguments or nullability.
func resolvedTypeShortName(t *typeinfer.ResolvedType) string {
	name := t.FQN
	if name == "" {
		name = t.Name
	}
	if i := strings.IndexByte(name, '<'); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimSuffix(name, "?")
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	return name
}
