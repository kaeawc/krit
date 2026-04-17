package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// AbstractClassCanBeConcreteClassRule detects abstract classes with no abstract members.
// With type inference: also checks inherited abstract members via ClassHierarchy
// to avoid false positives when abstract members come from a supertype.
type AbstractClassCanBeConcreteClassRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *AbstractClassCanBeConcreteClassRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — classifying
// abstractness requires knowing all concrete method bodies;
// resolver-assisted but falls back to structural heuristic. Classified per
// roadmap/17.
func (r *AbstractClassCanBeConcreteClassRule) Confidence() float64 { return 0.75 }

func (r *AbstractClassCanBeConcreteClassRule) NodeTypes() []string {
	return []string{"class_declaration"}
}

func (r *AbstractClassCanBeConcreteClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding

	// Check modifiers for abstract using AST walking
	if !file.FlatHasModifier(idx, "abstract") {
		return nil
	}
	// Skip classes with type parameters — generic abstract base classes
	// almost always exist specifically for subclassing.
	for i := 0; i < file.FlatChildCount(idx); i++ {
		if file.FlatType(file.FlatChild(idx, i)) == "type_parameters" {
			return nil
		}
	}
	mods := file.FlatFindChild(idx, "modifiers")
	body := file.FlatFindChild(idx, "class_body")
	if mods == 0 || body == 0 {
		// No body means no abstract members either, but empty abstract class is ok
		return nil
	}
	// Check if any member is abstract
	hasAbstractMember := false
	// Also check for `open` members — an abstract class with `open` members
	// is a legitimate base class that exists specifically to be subclassed
	// and have those members overridden (with default implementations).
	hasOpenMember := false
	// And `protected` members — meaningful only when subclasses exist,
	// so the class is clearly designed for extension.
	hasProtectedMember := false
	file.FlatWalkAllNodes(body, func(child uint32) {
		if file.FlatType(child) == "modifiers" && child != mods {
			if parent, ok := file.FlatParent(child); ok {
				if file.FlatHasModifier(parent, "abstract") {
					hasAbstractMember = true
				}
				if file.FlatHasModifier(parent, "open") {
					hasOpenMember = true
				}
				if file.FlatHasModifier(parent, "protected") {
					hasProtectedMember = true
				}
			}
		}
	})
	// Skip classes that have `open` or `protected` members — they're
	// designed for subclassing.
	if hasOpenMember || hasProtectedMember {
		return nil
	}

	// If the class has any supertype, skip unless the resolver can confirm
	// all inherited abstract members are implemented. Without full resolution,
	// we can't know if an interface method is left abstract for subclasses.
	if !hasAbstractMember {
		hasSupertype := false
		for i := 0; i < file.FlatChildCount(idx); i++ {
			if file.FlatType(file.FlatChild(idx, i)) == "delegation_specifier" {
				hasSupertype = true
				break
			}
		}
		if hasSupertype {
			if r.resolver == nil {
				return nil // Can't verify — skip to avoid false positives
			}
			name := extractIdentifierFlat(file, idx)
			info := r.resolver.ClassHierarchy(name)
			if info == nil || len(info.Supertypes) == 0 {
				// Resolver doesn't have supertype info — can't verify inherited
				// abstract members, so skip conservatively.
				return nil
			}
			// Collect implemented member names from this class
			implemented := make(map[string]bool)
			file.FlatWalkAllNodes(body, func(child uint32) {
				if t := file.FlatType(child); t == "function_declaration" || t == "property_declaration" {
					memberName := extractIdentifierFlat(file, child)
					if memberName != "" {
						implemented[memberName] = true
					}
				}
			})
			// Check supertypes for unimplemented abstract members
			allResolved := true
			for _, st := range info.Supertypes {
				parts := strings.Split(st, ".")
				stName := parts[len(parts)-1]
				stInfo := r.resolver.ClassHierarchy(stName)
				if stInfo == nil {
					stInfo = r.resolver.ClassHierarchy(st)
				}
				if stInfo == nil {
					// Unknown supertype — can't verify, skip conservatively
					allResolved = false
					break
				}
				for _, m := range stInfo.Members {
					if m.IsAbstract && !implemented[m.Name] {
						hasAbstractMember = true
						break
					}
				}
				if hasAbstractMember {
					break
				}
			}
			if !allResolved {
				return nil
			}
		}
	}

	if !hasAbstractMember {
		name := extractIdentifierFlat(file, idx)
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Abstract class '%s' has no abstract members. Make it concrete.", name))
		// Remove "abstract " from the modifiers
		modsText2 := file.FlatNodeText(mods)
		newMods := strings.Replace(modsText2, "abstract ", "", 1)
		if newMods == modsText2 {
			// Try without trailing space (e.g. "abstract\n")
			newMods = strings.Replace(modsText2, "abstract", "", 1)
		}
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(mods)),
			EndByte:     int(file.FlatEndByte(mods)),
			Replacement: newMods,
		}
		findings = append(findings, f)
	}

	return findings
}

// AbstractClassCanBeInterfaceRule detects abstract classes with no state.
type AbstractClassCanBeInterfaceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *AbstractClassCanBeInterfaceRule) Confidence() float64 { return 0.75 }

func (r *AbstractClassCanBeInterfaceRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *AbstractClassCanBeInterfaceRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	var findings []scanner.Finding

	if !file.FlatHasModifier(idx, "abstract") {
		return nil
	}
	// Dagger/Hilt @Module containers are conventionally abstract classes
	// (the `@Binds` method syntax historically only worked on abstract
	// classes; Hilt's own guides still recommend abstract class for
	// modules). Skip when the class carries a @Module annotation.
	if hasAnnotationFlat(file, idx, "Module") {
		return nil
	}
	// Skip if any supertype uses constructor invocation syntax (`: Foo()` or
	// `: Foo(arg)`) — that indicates the supertype is a class, not an
	// interface, and Kotlin interfaces cannot extend classes.
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		// A delegation_specifier with a constructor_invocation child means
		// the supertype is a class being constructed.
		if file.FlatFindChild(child, "constructor_invocation") != 0 {
			return nil
		}
	}
	if ctor := file.FlatFindChild(idx, "primary_constructor"); ctor != 0 {
		paramsText := file.FlatNodeText(ctor)
		if strings.Contains(paramsText, "val ") || strings.Contains(paramsText, "var ") {
			return nil // Has state via constructor
		}
	}
	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}
	hasState := false
	file.FlatWalkNodes(body, "property_declaration", func(propNode uint32) {
		propText := file.FlatNodeText(propNode)
		if strings.Contains(propText, "=") {
			hasState = true
		}
	})
	if hasState {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Abstract class '%s' has no state and could be an interface.", name))
	// Auto-fix: use AST nodes to find byte ranges for "abstract" and "class" keywords,
	// then replace them independently to avoid matching inside string literals or comments.
	// Build a list of byte-range replacements sorted descending by offset.
	type replEntry struct {
		start, end int
		repl       string
	}
	var repls []replEntry

	// 1. Remove the "abstract" modifier on the class declaration itself
	abstractNode := file.FlatFindModifierNode(idx, "abstract")
	if abstractNode != 0 {
		endByte := int(file.FlatEndByte(abstractNode))
		// Consume trailing whitespace (space/newline) after "abstract"
		for endByte < int(file.FlatEndByte(idx)) && (file.Content[endByte] == ' ' || file.Content[endByte] == '\t') {
			endByte++
		}
		repls = append(repls, replEntry{int(file.FlatStartByte(abstractNode)), endByte, ""})
	}

	// 2. Replace "class" keyword child with "interface"
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatNodeTextEquals(child, "class") {
			repls = append(repls, replEntry{int(file.FlatStartByte(child)), int(file.FlatEndByte(child)), "interface"})
			break
		}
	}

	// 3. Remove "abstract" modifiers from member declarations in the body
	if body != 0 {
		file.FlatWalkAllNodes(body, func(member uint32) {
			if t := file.FlatType(member); t == "function_declaration" || t == "property_declaration" {
				absNode := file.FlatFindModifierNode(member, "abstract")
				if absNode != 0 {
					endByte := int(file.FlatEndByte(absNode))
					for endByte < int(file.FlatEndByte(member)) && (file.Content[endByte] == ' ' || file.Content[endByte] == '\t') {
						endByte++
					}
					repls = append(repls, replEntry{int(file.FlatStartByte(absNode)), endByte, ""})
				}
			}
		})
	}

	// Apply replacements in reverse byte order to keep offsets valid
	if len(repls) > 0 {
		// Sort descending by start byte
		for i := 0; i < len(repls); i++ {
			for j := i + 1; j < len(repls); j++ {
				if repls[j].start > repls[i].start {
					repls[i], repls[j] = repls[j], repls[i]
				}
			}
		}
		nodeText := file.FlatNodeText(idx)
		base := int(file.FlatStartByte(idx))
		for _, r := range repls {
			relStart := r.start - base
			relEnd := r.end - base
			if relStart >= 0 && relEnd <= len(nodeText) {
				nodeText = nodeText[:relStart] + r.repl + nodeText[relEnd:]
			}
		}
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(idx)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: nodeText,
		}
	}
	findings = append(findings, f)

	return findings
}

// DataClassShouldBeImmutableRule detects var properties in data classes.
// With type inference: also flags val properties whose types are mutable collections
// (MutableList, MutableMap, MutableSet) since they undermine data class immutability.
type DataClassShouldBeImmutableRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *DataClassShouldBeImmutableRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — detecting
// mutable properties in data classes needs type-aware var detection;
// fallback uses keyword matching. Classified per roadmap/17.
func (r *DataClassShouldBeImmutableRule) Confidence() float64 { return 0.75 }

// mutableCollectionTypes lists types that are mutable collections.
var mutableCollectionTypes = map[string]bool{
	"MutableList": true, "MutableSet": true, "MutableMap": true,
	"MutableCollection": true, "MutableIterable": true,
}

func (r *DataClassShouldBeImmutableRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *DataClassShouldBeImmutableRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !file.FlatHasModifier(idx, "data") {
		return nil
	}
	ctor := file.FlatFindChild(idx, "primary_constructor")
	if ctor == 0 {
		return nil
	}
	var findings []scanner.Finding
	file.FlatWalkNodes(ctor, "class_parameter", func(child uint32) {
		text := file.FlatNodeText(child)
		if strings.HasPrefix(strings.TrimSpace(text), "var ") {
			f := r.Finding(file, file.FlatRow(child)+1, 1,
				"Data class property should be immutable. Use 'val' instead of 'var'.")
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(child)),
				EndByte:     int(file.FlatStartByte(child)) + 3,
				Replacement: "val",
			}
			findings = append(findings, f)
		}
		// With resolver: check if val property has a mutable collection type
		if r.resolver != nil && strings.HasPrefix(strings.TrimSpace(text), "val ") {
			// Find the type annotation node inside the class_parameter
			for i := 0; i < file.FlatChildCount(child); i++ {
				typeChild := file.FlatChild(child, i)
				if t := file.FlatType(typeChild); t == "user_type" || t == "nullable_type" {
					resolved := r.resolver.ResolveFlatNode(typeChild, file)
					if resolved.Kind != typeinfer.TypeUnknown && mutableCollectionTypes[resolved.Name] {
						findings = append(findings, r.Finding(file, file.FlatRow(child)+1, 1,
							fmt.Sprintf("Data class property uses mutable type '%s'. Use an immutable collection type for true immutability.", resolved.Name)))
					}
					break
				}
			}
		}
	})
	return findings
}

// DataClassContainsFunctionsRule detects data classes with function members.
type DataClassContainsFunctionsRule struct {
	FlatDispatchBase
	BaseRule
	ConversionFunctionPrefix []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *DataClassContainsFunctionsRule) Confidence() float64 { return 0.75 }

func (r *DataClassContainsFunctionsRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *DataClassContainsFunctionsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !file.FlatHasModifier(idx, "data") {
		return nil
	}
	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}
	if file.FlatCountNodes(body, "function_declaration") > 0 {
		name := extractIdentifierFlat(file, idx)
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			fmt.Sprintf("Data class '%s' contains functions. Consider using a regular class.", name))}
	}
	return nil
}

// ProtectedMemberInFinalClassRule detects protected members in final classes.
// With type inference: verifies via ClassHierarchy that no subclass exists,
// confirming the class is truly final even if it appears in a different module.
type ProtectedMemberInFinalClassRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *ProtectedMemberInFinalClassRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — flags protected
// members on non-open classes; class-openness detection depends on declared
// modifiers plus resolver for inherited finality. Classified per
// roadmap/17.
func (r *ProtectedMemberInFinalClassRule) Confidence() float64 { return 0.75 }

func (r *ProtectedMemberInFinalClassRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *ProtectedMemberInFinalClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check if class is final (no open/abstract/sealed modifier)
	if file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "sealed") {
		return nil
	}

	// With type inference: double-check via class hierarchy that the class
	// truly has no subclasses (it might be extended in another file despite
	// lacking the `open` keyword in Kotlin, which would be a compile error,
	// but we verify anyway for completeness).
	if r.resolver != nil {
		name := extractIdentifierFlat(file, idx)
		if name != "" {
			info := r.resolver.ClassHierarchy(name)
			if info != nil && info.IsOpen {
				// Class hierarchy says it's open — skip
				return nil
			}
		}
	}

	var findings []scanner.Finding
	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}
	forEachDirectClassMemberFlat(file, body, func(member uint32) {
		if member == 0 || !file.FlatHasModifier(member, "protected") {
			return
		}
		f := r.Finding(file, file.FlatRow(member)+1, 1,
			"Protected member in final class should be private.")
		// Find the "protected" modifier node in the AST and replace its byte range
		protectedNode := file.FlatFindModifierNode(member, "protected")
		if protectedNode != 0 {
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(protectedNode)),
				EndByte:     int(file.FlatEndByte(protectedNode)),
				Replacement: "private",
			}
		}
		findings = append(findings, f)
	})
	return findings
}

func forEachDirectClassMemberFlat(file *scanner.File, body uint32, fn func(uint32)) {
	if file == nil || body == 0 {
		return
	}
	for i := 0; i < file.FlatNamedChildCount(body); i++ {
		child := file.FlatNamedChild(body, i)
		if child == 0 {
			continue
		}
		if file.FlatType(child) == "class_member_declarations" {
			for j := 0; j < file.FlatNamedChildCount(child); j++ {
				member := file.FlatNamedChild(child, j)
				if member != 0 {
					fn(member)
				}
			}
			continue
		}
		fn(child)
	}
}

// NestedClassesVisibilityRule reports nested classes/objects/interfaces that use an explicit
// public modifier inside an internal parent class. The public keyword is misleading because
// the nested class still has internal visibility. Matches detekt's NestedClassesVisibility.
type NestedClassesVisibilityRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *NestedClassesVisibilityRule) Confidence() float64 { return 0.75 }

func (r *NestedClassesVisibilityRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *NestedClassesVisibilityRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Only check top-level classes (not deeply nested ones)
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "source_file" {
		return nil
	}
	// Skip interfaces — their nested members have different visibility semantics.
	if file.FlatFindChild(idx, "interface") != 0 {
		return nil
	}
	// Only applies to internal classes
	if !file.FlatHasModifier(idx, "internal") {
		return nil
	}
	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}
	var findings []scanner.Finding
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		childType := file.FlatType(child)
		// Check nested class_declaration and object_declaration (but not companion_object)
		if childType != "class_declaration" && childType != "object_declaration" {
			continue
		}
		// Skip companion objects (tree-sitter node type is "companion_object")
		if childType == "companion_object" {
			continue
		}
		// Check child nodes for enum/companion keywords
		isEnum := false
		for j := 0; j < file.FlatChildCount(child); j++ {
			ct := file.FlatType(file.FlatChild(child, j))
			if ct == "enum" {
				isEnum = true
				break
			}
		}
		if isEnum {
			continue
		}
		// Only flag if the nested class has an explicit "public" modifier
		if !file.FlatHasModifier(child, "public") {
			continue
		}
		name := extractIdentifierFlat(file, child)
		findings = append(findings, r.Finding(file, file.FlatRow(child)+1, 1,
			fmt.Sprintf("The nested class '%s' has an explicit public modifier. Within an internal class this is misleading, as the nested class is still internal.", name)))
	}
	return findings
}

// UtilityClassWithPublicConstructorRule detects utility classes with public constructors.
type UtilityClassWithPublicConstructorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *UtilityClassWithPublicConstructorRule) Confidence() float64 { return 0.75 }

func (r *UtilityClassWithPublicConstructorRule) NodeTypes() []string {
	return []string{"class_declaration"}
}

func (r *UtilityClassWithPublicConstructorRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip interfaces, sealed classes, data classes, enum classes — not utility classes
	nodeText := file.FlatNodeText(idx)
	prefix := strings.TrimSpace(nodeText)
	if len(prefix) > 200 {
		prefix = prefix[:200]
	}
	if strings.Contains(prefix, "interface ") ||
		strings.Contains(prefix, "sealed ") ||
		strings.Contains(prefix, "data ") ||
		strings.Contains(prefix, "enum ") {
		return nil
	}
	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}
	// All members should be static-like (companion object or top-level)
	hasFunctions := false
	hasNonStaticMember := false
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		switch file.FlatType(child) {
		case "companion_object":
			hasFunctions = true
		case "function_declaration", "property_declaration":
			hasNonStaticMember = true
		}
	}
	if !hasFunctions || hasNonStaticMember {
		return nil
	}
	// Check if constructor is private — use FindChild since it's a simple O(1) lookup
	ctor := file.FlatFindChild(idx, "primary_constructor")
	if ctor != 0 {
		if file.FlatHasModifier(ctor, "private") {
			return nil
		}
		// If the primary constructor has val/var parameters, these are instance
		// properties — the class is not a utility class.
		ctorText := file.FlatNodeText(ctor)
		if strings.Contains(ctorText, "val ") || strings.Contains(ctorText, "var ") {
			return nil
		}
	}
	// Also skip classes with supertypes (they extend something, not pure utility)
	for i := 0; i < file.FlatChildCount(idx); i++ {
		if file.FlatType(file.FlatChild(idx, i)) == "delegation_specifier" {
			return nil
		}
	}
	name := extractIdentifierFlat(file, idx)
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Utility class '%s' should have a private constructor.", name))

	// Autofix. Two shapes:
	//   A. Explicit visibility modifier on an existing primary_constructor
	//      (`class Foo public constructor(...) { ... }`) — swap the
	//      modifier for `private`.
	//   B. No primary_constructor at all
	//      (`class Foo { companion object { ... } }`) — insert
	//      ` private constructor()` just before the class body.
	// The implicit case (`class Foo()`) is intentionally not fixed: the
	// minimum valid transform there is inserting both `private` and the
	// `constructor` keyword around a paren that the AST doesn't expose,
	// and the result needs to interact with whitespace on either side
	// in a ktfmt-compatible way. Leave that to hand-written edits.
	if ctor != 0 {
		for _, vis := range []string{"public", "protected", "internal"} {
			if modNode := file.FlatFindModifierNode(ctor, vis); modNode != 0 {
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(modNode)),
					EndByte:     int(file.FlatEndByte(modNode)),
					Replacement: "private",
				}
				break
			}
		}
	} else {
		body := file.FlatFindChild(idx, "class_body")
		if body != 0 {
			insertAt := int(file.FlatStartByte(body))
			// Walk back over whitespace so the insertion lands right after
			// the identifier (or type parameters) and leaves the existing
			// ` {` spacing intact.
			for insertAt > 0 && (file.Content[insertAt-1] == ' ' || file.Content[insertAt-1] == '\t') {
				insertAt--
			}
			if insertAt > 0 && file.Content[insertAt-1] != '\n' && file.Content[insertAt-1] != '\r' {
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   insertAt,
					EndByte:     insertAt,
					Replacement: " private constructor()",
				}
			}
		}
	}
	return []scanner.Finding{f}
}

func (r *UtilityClassWithPublicConstructorRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

// OptionalAbstractKeywordRule detects abstract keyword on interface members.
type OptionalAbstractKeywordRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *OptionalAbstractKeywordRule) Confidence() float64 { return 0.75 }

func (r *OptionalAbstractKeywordRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *OptionalAbstractKeywordRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Only apply to interfaces
	if file.FlatFindChild(idx, "interface") == 0 {
		return nil
	}
	var findings []scanner.Finding
	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}
	baseColumn := -1
	for i := 0; i < file.FlatNamedChildCount(body); i++ {
		member := file.FlatNamedChild(body, i)
		if member == 0 {
			continue
		}
		if col := file.FlatCol(member); baseColumn == -1 || col < baseColumn {
			baseColumn = col
		}
	}
	for i := 0; i < file.FlatNamedChildCount(body); i++ {
		member := file.FlatNamedChild(body, i)
		if member == 0 {
			continue
		}
		switch file.FlatType(member) {
		case "function_declaration", "property_declaration":
		default:
			continue
		}
		// Tree-sitter recovery can flatten nested abstract classes declared
		// inside interfaces into sibling function/property nodes. Keep the
		// rule on the interface's direct members only.
		if baseColumn >= 0 && file.FlatCol(member) > baseColumn {
			continue
		}
		memberText := strings.TrimSpace(file.FlatNodeText(member))
		if strings.HasPrefix(memberText, "abstract class ") ||
			strings.HasPrefix(memberText, "class ") ||
			strings.HasPrefix(memberText, "abstract interface ") ||
			strings.HasPrefix(memberText, "interface ") {
			continue
		}
		mods := file.FlatFindChild(member, "modifiers")
		if mods == 0 || !file.FlatHasModifier(member, "abstract") {
			continue
		}
		modsText := file.FlatNodeText(mods)
		f := r.Finding(file, file.FlatRow(mods)+1, 1,
			"'abstract' modifier is redundant on interface members.")
		newMods := strings.Replace(modsText, "abstract ", "", 1)
		if newMods == modsText {
			newMods = strings.Replace(modsText, "abstract", "", 1)
		}
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(mods)),
			EndByte:     int(file.FlatEndByte(mods)),
			Replacement: newMods,
		}
		findings = append(findings, f)
	}
	return findings
}

// ClassOrderingRule checks that class members are in the conventional order.
type ClassOrderingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/classes rule. Detection relies on modifier and declaration
// structure plus (optional) resolver-backed inheritance checks; the
// fallback path is heuristic. Classified per roadmap/17.
func (r *ClassOrderingRule) Confidence() float64 { return 0.75 }

func (r *ClassOrderingRule) NodeTypes() []string { return []string{"class_body"} }

func (r *ClassOrderingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Conventional order: properties, init blocks, secondary constructors, functions, companion object
	const (
		orderProperty    = 1
		orderInit        = 2
		orderConstructor = 3
		orderFunction    = 4
		orderCompanion   = 5
	)
	lastOrder := 0
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		var currentOrder int
		switch file.FlatType(child) {
		case "property_declaration":
			currentOrder = orderProperty
		case "anonymous_initializer":
			currentOrder = orderInit
		case "secondary_constructor":
			currentOrder = orderConstructor
		case "function_declaration":
			currentOrder = orderFunction
		case "companion_object":
			currentOrder = orderCompanion
		default:
			continue
		}
		if currentOrder < lastOrder {
			return []scanner.Finding{r.Finding(file, file.FlatRow(child)+1, 1,
				"Class members should be ordered: properties, init blocks, constructors, functions, companion object.")}
		}
		lastOrder = currentOrder
	}
	return nil
}

// ObjectLiteralToLambdaRule detects object literals that could be lambdas.
// With type inference: uses ClassHierarchy to detect SAM interfaces from
// dependencies and other project files beyond the hardcoded allowlist.
type ObjectLiteralToLambdaRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *ObjectLiteralToLambdaRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence. SAM
// conversion eligibility depends on whether the supertype is a
// single-abstract-method interface, which needs either a class
// hierarchy from the resolver or a hardcoded allow-list — both
// paths miss project-defined SAM interfaces. The rule also can't
// easily detect when the object literal captures state or has
// side-effect init blocks that prevent conversion.
func (r *ObjectLiteralToLambdaRule) Confidence() float64 { return 0.75 }

func (r *ObjectLiteralToLambdaRule) NodeTypes() []string { return []string{"object_literal"} }

func (r *ObjectLiteralToLambdaRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	delegations := flatDirectChildrenOfType(file, idx, "delegation_specifier")
	if len(delegations) != 1 {
		// No supertypes or multiple supertypes — can't SAM convert
		return nil
	}
	// SAM conversion requires exactly one supertype (no multiple inheritance).
	// Also reject supertypes with constructor args (abstract classes, not interfaces).
	specText := file.FlatNodeText(delegations[0])
	if strings.Contains(specText, "(") {
		// Has constructor args — abstract class, not a SAM interface
		return nil
	}
	// Extract the base type name (strip generics)
	supertypeName := extractSupertypeNameFlat(file, delegations[0])

	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}

	funCount := 0
	propCount := 0
	hasInit := false
	var singleFun uint32
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		switch file.FlatType(child) {
		case "function_declaration":
			funCount++
			singleFun = child
		case "property_declaration":
			propCount++
		case "anonymous_initializer":
			hasInit = true
		}
	}

	// Must be exactly one function, no properties, no init blocks
	if funCount != 1 || propCount != 0 || hasInit {
		return nil
	}

	// The single method must be an override (implementing the interface method)
	if !file.FlatHasModifier(singleFun, "override") {
		return nil
	}

	// Check for bare 'this' in the method body — if the object references itself,
	// it can't be converted to a lambda (lambdas don't have their own 'this').
	// Labeled 'this@Label' refers to an outer scope, so that's fine.
	funBody := file.FlatFindChild(singleFun, "function_body")
	if funBody != 0 && objectBodyContainsBareThisFlat(file, funBody) {
		return nil
	}

	// SAM conversion only works for Java SAM interfaces and Kotlin "fun interface".
	// Check: (1) known Java SAM interfaces, (2) "fun interface" in the same file,
	// (3) oracle ClassHierarchy for project/dependency interfaces with exactly 1 abstract method.
	if supertypeName != "" && !isSAMConvertible(supertypeName, file, r.resolver) {
		return nil
	}

	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		"Object literal with single method can be converted to a lambda.")}
}

// extractSupertypeName gets the simple type name from a delegation_specifier node,
// handling dotted names (e.g., "Foo.Bar") and generics (e.g., "Callable<Int>").
// For dotted names like "MenuItem.OnActionExpandListener", returns the last segment.
func extractSupertypeNameFlat(file *scanner.File, specNode uint32) string {
	ut := file.FlatFindChild(specNode, "user_type")
	if ut == 0 {
		return ""
	}
	var lastIdent string
	for i := 0; i < file.FlatChildCount(ut); i++ {
		child := file.FlatChild(ut, i)
		if file.FlatType(child) == "type_identifier" {
			lastIdent = file.FlatNodeText(child)
		}
	}
	return lastIdent
}

// knownSAMInterfaces is a set of well-known Java SAM interfaces and Kotlin fun
// interfaces from common libraries that support SAM conversion.
var knownSAMInterfaces = map[string]bool{
	// Java standard library
	"Runnable":          true,
	"Callable":          true,
	"Comparator":        true,
	"Predicate":         true,
	"Function":          true,
	"Supplier":          true,
	"Consumer":          true,
	"BiFunction":        true,
	"BiConsumer":        true,
	"BiPredicate":       true,
	"UnaryOperator":     true,
	"BinaryOperator":    true,
	"FileFilter":        true,
	"FilenameFilter":    true,
	"InvocationHandler": true,
	"ThreadFactory":     true,
	"PrivilegedAction":  true,
	"Executor":          true,
	// Java AWT/Swing
	"ActionListener": true,
	// Kotlin coroutines
	"ChildHandle": true,
	// Compose runtime — DisposableEffectResult is a regular Kotlin interface, NOT fun interface
	// Android
	"OnClickListener":     true,
	"OnLongClickListener": true,
	"OnTouchListener":     true,
	"OnScrollListener":    true,
}

// isSAMConvertible checks whether the given supertype name is a SAM-convertible interface.
// It checks (1) known Java SAM interfaces, (2) "fun interface" in the same file,
// and (3) oracle ClassHierarchy for interfaces with exactly one abstract method.
func isSAMConvertible(name string, file *scanner.File, resolver typeinfer.TypeResolver) bool {
	// Check known SAM interfaces (Java SAM + common Kotlin fun interfaces)
	if knownSAMInterfaces[name] {
		return true
	}

	// Check for "fun interface <Name>" in the same file (Kotlin fun interfaces)
	fileText := string(file.Content)
	// Look for "fun interface Name" pattern — handles the common case
	target := "fun interface " + name
	if strings.Contains(fileText, target) {
		return true
	}

	// With type inference: check oracle for project-defined and dependency interfaces
	// that have exactly one abstract function member (SAM interfaces).
	// SAM conversion only works on Java interfaces or Kotlin "fun interface" —
	// regular Kotlin interfaces with a single abstract method are NOT SAM convertible.
	if resolver != nil {
		info := resolver.ClassHierarchy(name)
		if info != nil && info.Kind == "interface" {
			abstractFunCount := 0
			for _, m := range info.Members {
				if m.IsAbstract && m.Kind == "function" {
					abstractFunCount++
				}
			}
			if abstractFunCount == 1 {
				// Check if this is a Kotlin "fun interface" or a Java interface.
				// Java source files always support SAM. Kotlin interfaces only
				// support SAM if declared with "fun interface".
				if info.File != "" {
					if strings.HasSuffix(info.File, ".kt") || strings.HasSuffix(info.File, ".kts") {
						return false // Kotlin source without "fun" — not SAM convertible
					}
					return true // Java source — SAM convertible
				}
				// No file info — check if we're in a Kotlin project
				// by conservatively assuming non-SAM unless proven otherwise
				return false
			}
		}
	}

	return false
}

// objectBodyContainsBareThis walks the AST looking for unlabeled 'this' expressions.
// It does NOT descend into nested object_literal nodes (they have their own 'this').
func objectBodyContainsBareThisFlat(file *scanner.File, node uint32) bool {
	if file.FlatType(node) == "this_expression" {
		hasLabel := false
		for i := 0; i < file.FlatChildCount(node); i++ {
			if file.FlatType(file.FlatChild(node, i)) == "type_identifier" {
				hasLabel = true
				break
			}
		}
		return !hasLabel
	}
	if file.FlatType(node) == "object_literal" {
		return false
	}
	for i := 0; i < file.FlatChildCount(node); i++ {
		if objectBodyContainsBareThisFlat(file, file.FlatChild(node, i)) {
			return true
		}
	}
	return false
}

// SerialVersionUIDInSerializableClassRule detects missing serialVersionUID.
// With type inference: uses ClassHierarchy to verify the class actually implements
// java.io.Serializable (could be transitive through a supertype).
type SerialVersionUIDInSerializableClassRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *SerialVersionUIDInSerializableClassRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — Serializable
// detection uses supertype names; without resolver, falls back to matching
// `Serializable` in the delegation list. Classified per roadmap/17.
func (r *SerialVersionUIDInSerializableClassRule) Confidence() float64 { return 0.75 }

func (r *SerialVersionUIDInSerializableClassRule) NodeTypes() []string {
	return []string{"class_declaration"}
}

func (r *SerialVersionUIDInSerializableClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Enum classes use name-based serialization — serialVersionUID is irrelevant
	if file.FlatFindChild(idx, "enum") != 0 {
		return nil
	}

	text := file.FlatNodeText(idx)

	// Check for serialVersionUID early — if present, no issue regardless
	if strings.Contains(text, "serialVersionUID") {
		return nil
	}

	name := extractIdentifierFlat(file, idx)
	implementsSerializable := false

	// Walk declared supertypes from the class's delegation_specifiers and
	// check each against the resolver's class hierarchy. This avoids the
	// self-lookup name-collision bug where two classes sharing a simple
	// name (e.g. `RestoreState` data class + `RestoreState` enum) cause
	// the enum's `java.lang.Enum → Serializable` chain to taint the data
	// class.
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		supertypeName := viewConstructorSupertypeNameFlat(file, child)
		if supertypeName == "" {
			continue
		}
		// Direct match.
		if supertypeName == "Serializable" || supertypeName == "Externalizable" {
			implementsSerializable = true
			break
		}
		// Transitive check via resolver on the supertype, not on the
		// current class's own name.
		if r.resolver != nil {
			if info := r.resolver.ClassHierarchy(supertypeName); info != nil {
				if r.checksSerializable(info) {
					implementsSerializable = true
					break
				}
			}
		}
	}

	if !implementsSerializable {
		return nil
	}

	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Serializable class '%s' is missing serialVersionUID.", name))}
}

func flatDirectChildrenOfType(file *scanner.File, idx uint32, nodeType string) []uint32 {
	var out []uint32
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == nodeType {
			out = append(out, child)
		}
	}
	return out
}

// checksSerializable walks the class hierarchy to find java.io.Serializable.
func (r *SerialVersionUIDInSerializableClassRule) checksSerializable(info *typeinfer.ClassInfo) bool {
	visited := make(map[string]bool)
	return r.checksSerializableRec(info, visited)
}

func (r *SerialVersionUIDInSerializableClassRule) checksSerializableRec(info *typeinfer.ClassInfo, visited map[string]bool) bool {
	if visited[info.FQN] || visited[info.Name] {
		return false
	}
	visited[info.FQN] = true
	visited[info.Name] = true

	for _, st := range info.Supertypes {
		if st == "java.io.Serializable" || strings.HasSuffix(st, ".Serializable") || st == "Serializable" {
			return true
		}
		// Check transitively
		parts := strings.Split(st, ".")
		stName := parts[len(parts)-1]
		stInfo := r.resolver.ClassHierarchy(stName)
		if stInfo == nil {
			stInfo = r.resolver.ClassHierarchy(st)
		}
		if stInfo != nil && r.checksSerializableRec(stInfo, visited) {
			return true
		}
	}
	return false
}

func (r *SerialVersionUIDInSerializableClassRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}
