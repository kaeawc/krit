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

var mutableCollectionToMethodMap = map[string]string{
	"MutableList":         "toMutableList()",
	"MutableSet":          "toMutableSet()",
	"MutableMap":          "toMutableMap()",
	"MutableCollection":   "toMutableList()",
	"MutableIterator":     "toMutableList().iterator()",
	"MutableListIterator": "toMutableList().listIterator()",
	"MutableIterable":     "toMutableList()",
}

// immutableToMutableMap maps immutable collection types to their mutable counterparts.
var immutableToMutableMap = map[string]string{
	"List": "MutableList", "Set": "MutableSet", "Map": "MutableMap",
	"Collection": "MutableCollection", "Iterable": "MutableIterable",
	"Iterator": "MutableIterator", "ListIterator": "MutableListIterator",
}

func (r *DontDowncastCollectionTypesRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if file == nil || idx == 0 {
		return
	}

	// Walk as_expression children: find source expression (before 'as') and type node (after 'as').
	var sourceIdx, typeNode uint32
	seenAs := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		ct := file.FlatType(child)
		if ct == "as" || ct == "as?" {
			seenAs = true
			continue
		}
		if !file.FlatIsNamed(child) {
			continue
		}
		if !seenAs {
			sourceIdx = child
		} else if typeNode == 0 {
			typeNode = child
		}
	}
	if typeNode == 0 {
		return
	}

	// Resolve the base user_type (unwrap nullable_type if needed).
	baseType := typeNode
	if file.FlatType(typeNode) == "nullable_type" {
		inner, ok := file.FlatFindChild(typeNode, "user_type")
		if !ok {
			return
		}
		baseType = inner
	}
	if file.FlatType(baseType) != "user_type" {
		return
	}
	typeIdent, ok := file.FlatFindChild(baseType, "type_identifier")
	if !ok {
		return
	}
	targetType := file.FlatNodeText(typeIdent)

	if _, ok := mutableCollectionToMethodMap[targetType]; !ok {
		return
	}

	skip, resolverConfirmed := dontDowncastCheckResolver(ctx, file, sourceIdx, targetType)
	if skip {
		return
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Don't downcast collection type to '%s'. This can lead to unexpected mutations.", targetType))
	if resolverConfirmed {
		f.Confidence = 0.9
	} else {
		f.Confidence = 0.8
	}

	if sourceIdx != 0 {
		expr := file.FlatNodeText(sourceIdx)
		if method, ok := mutableCollectionToMethodMap[targetType]; ok {
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: expr + "." + method,
			}
		}
	}

	ctx.Emit(f)
}

func dontDowncastCheckResolver(ctx *api.Context, file *scanner.File, sourceIdx uint32, targetType string) (skip bool, confirmed bool) {
	if ctx.Resolver == nil || sourceIdx == 0 {
		return false, false
	}
	sourceType := ctx.Resolver.ResolveFlatNode(sourceIdx, file)
	if sourceType.Kind == typeinfer.TypeUnknown {
		return false, false
	}
	if _, alreadyMutable := mutableCollectionToMethodMap[sourceType.Name]; alreadyMutable {
		return true, false
	}
	expectedImmutable := ""
	for immutable, mutable := range immutableToMutableMap {
		if mutable == targetType {
			expectedImmutable = immutable
			break
		}
	}
	if expectedImmutable != "" && sourceType.Name != expectedImmutable {
		info := ctx.Resolver.ClassHierarchy(sourceType.Name)
		if info != nil {
			for _, st := range info.Supertypes {
				parts := strings.Split(st, ".")
				if parts[len(parts)-1] == expectedImmutable {
					return false, true
				}
			}
			return true, false
		}
	}
	return false, true
}
