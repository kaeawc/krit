package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func registerStyleClassesRules() {
	registerStyleAbstractClassCanBeConcreteClass()
	registerStyleAbstractClassCanBeInterface()
	registerStyleDataClassShouldBeImmutable()
	registerStyleDataClassContainsFunctions()
	registerStyleProtectedMemberInFinalClass()
	registerStyleNestedClassesVisibility()
	registerStyleUtilityClassWithPublicConstructor()
	registerStyleOptionalAbstractKeyword()
	registerStyleClassOrdering()
	registerStyleObjectLiteralToLambda()
	registerStyleSerialVersionUIDInSerializableClass()
}

func registerStyleAbstractClassCanBeConcreteClass() {
	r := &AbstractClassCanBeConcreteClassRule{BaseRule: BaseRule{RuleName: "AbstractClassCanBeConcreteClass", RuleSetName: "style", Sev: "warning", Desc: "Detects abstract classes that have no abstract members and could be made concrete."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
		Needs: api.NeedsResolver,
		// Source-level class facts only (#956): the oracle's class facts carry
		// members only when some active rule asked krit-fir for them, so a
		// library supertype could look like it has nothing abstract, and the
		// answer would depend on which other rules ran (and on the daemon).
		// Required: with no resolver at all it still runs (it needs one only for supertypes).
		TypeInfo: api.TypeInfoHint{PreferBackend: api.PreferResolver, Required: true},
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasModifier(idx, "abstract") {
				return
			}
			if abstractClassIsInjectionOrTestFixture(file, idx) || abstractClassHasSubtypes(ctx.Resolver, file, idx) {
				return
			}
			for i := 0; i < file.FlatChildCount(idx); i++ {
				if file.FlatType(file.FlatChild(idx, i)) == "type_parameters" {
					return
				}
			}
			mods, _ := file.FlatFindChild(idx, "modifiers")
			body, _ := file.FlatFindChild(idx, "class_body")
			if mods == 0 || body == 0 {
				return
			}
			hasAbstractMember := false
			hasOpenMember := false
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
			if hasOpenMember || hasProtectedMember {
				return
			}
			if !hasAbstractMember {
				hasSupertype := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					if file.FlatType(file.FlatChild(idx, i)) == "delegation_specifier" {
						hasSupertype = true
						break
					}
				}
				if hasSupertype {
					if ctx.Resolver == nil {
						return
					}
					name := extractIdentifierFlat(file, idx)
					info := ctx.Resolver.ClassHierarchy(name)
					if info == nil || len(info.Supertypes) == 0 {
						return
					}
					implemented := make(map[string]bool)
					file.FlatWalkAllNodes(body, func(child uint32) {
						if t := file.FlatType(child); t == "function_declaration" || t == "property_declaration" {
							memberName := extractIdentifierFlat(file, child)
							if memberName != "" {
								implemented[memberName] = true
							}
						}
					})
					// `override val x` constructor parameters implement abstract properties.
					file.FlatWalkNodes(idx, "class_parameter", func(param uint32) {
						if memberName := extractIdentifierFlat(file, param); memberName != "" {
							implemented[memberName] = true
						}
					})
					found, allResolved := abstractClassInheritsAbstractMember(ctx.Resolver, info.Supertypes, implemented, map[string]bool{})
					if !allResolved {
						return
					}
					hasAbstractMember = found
				}
			}
			if !hasAbstractMember {
				name := extractIdentifierFlat(file, idx)
				f := r.Finding(file, declLine(file, idx), declCol(file, idx),
					fmt.Sprintf("Abstract class '%s' has no abstract members. Make it concrete.", name))
				ctx.Emit(f)
			}
		},
	})
}

func registerStyleAbstractClassCanBeInterface() {
	r := &AbstractClassCanBeInterfaceRule{BaseRule: BaseRule{RuleName: "AbstractClassCanBeInterface", RuleSetName: "style", Sev: "warning", Desc: "Detects abstract classes with no state that could be converted to interfaces."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
		Needs: api.NeedsResolver,
		// Required: still runs without a resolver, it only needs one to find subclasses.
		TypeInfo: api.TypeInfoHint{PreferBackend: api.PreferResolver, Required: true},
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasModifier(idx, "abstract") {
				return
			}
			if abstractClassIsInjectionOrTestFixture(file, idx) || abstractClassHasClassOnlyConstruct(file, idx) || abstractClassHasSubtypes(ctx.Resolver, file, idx) {
				return
			}
			if abstractClassCanBeInterfaceHasDaggerBindMethods(file, idx) {
				return
			}
			if abstractClassCanBeInterfaceSupertypeCarriesState(file, idx) {
				return
			}
			if abstractClassCanBeInterfaceHasConcreteMember(file, idx) {
				return
			}
			if ctor, ok := file.FlatFindChild(idx, "primary_constructor"); ok {
				paramsText := file.FlatNodeText(ctor)
				if strings.Contains(paramsText, "val ") || strings.Contains(paramsText, "var ") {
					return
				}
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			hasState := false
			file.FlatWalkNodes(body, "property_declaration", func(propNode uint32) {
				if abstractClassCanBeInterfacePropertyCarriesState(file, idx, propNode) {
					hasState = true
				}
			})
			if hasState {
				return
			}
			name := extractIdentifierFlat(file, idx)
			f := r.Finding(file, declLine(file, idx), declCol(file, idx),
				fmt.Sprintf("Abstract class '%s' has no state and could be an interface.", name))
			ctx.Emit(f)
		},
	})
}

// dataClassMutableMessage is detekt's message for a mutable property of a data class.
func dataClassMutableMessage(file *scanner.File, class, prop uint32) string {
	name := ""
	if file.FlatType(prop) == "class_parameter" {
		name = extractIdentifierFlat(file, prop)
	} else {
		name = propertyDeclarationNameFlat(file, prop)
	}
	return "The data class " + extractIdentifierFlat(file, class) + " contains a mutable property. The offending property is called " + name
}

func registerStyleDataClassShouldBeImmutable() {
	r := &DataClassShouldBeImmutableRule{BaseRule: BaseRule{RuleName: "DataClassShouldBeImmutable", RuleSetName: "style", Sev: "warning", Desc: "Detects data class properties declared as var instead of val."}, ReportMutableCollections: true}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Needs: api.NeedsResolver,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasModifier(idx, "data") {
				return
			}
			ctor, _ := file.FlatFindChild(idx, "primary_constructor")
			if ctor == 0 {
				return
			}
			file.FlatWalkNodes(ctor, "class_parameter", func(child uint32) {
				if varKeyword := dataClassBindingKeyword(file, child, "var"); varKeyword != 0 {
					f := r.Finding(file, file.FlatRow(child)+1, file.FlatCol(child)+1,
						dataClassMutableMessage(file, idx, child))
					// An override of a mutable property cannot become a val.
					if !file.FlatHasModifier(child, "override") {
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(varKeyword)),
							EndByte:     int(file.FlatEndByte(varKeyword)),
							Replacement: "val",
						}
					}
					ctx.Emit(f)
				}
				if r.ReportMutableCollections && ctx.Resolver != nil && dataClassBindingKeyword(file, child, "val") != 0 {
					for i := 0; i < file.FlatChildCount(child); i++ {
						typeChild := file.FlatChild(child, i)
						if t := file.FlatType(typeChild); t == "user_type" || t == "nullable_type" {
							resolved := ctx.Resolver.ResolveFlatNode(typeChild, file)
							if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && mutableCollectionTypes[resolved.Name] {
								ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1,
									fmt.Sprintf("Data class property uses mutable type '%s'. Use an immutable collection type for true immutability.", resolved.Name))
							}
							break
						}
					}
				}
			})
			if body, _ := file.FlatFindChild(idx, "class_body"); body != 0 {
				file.FlatWalkNodes(body, "property_declaration", func(prop uint32) {
					if !dataClassPropertyBelongsToClass(file, prop, idx) {
						return
					}
					varKeyword := dataClassBindingKeyword(file, prop, "var")
					if varKeyword == 0 {
						return
					}
					f := r.Finding(file, file.FlatRow(prop)+1, file.FlatCol(prop)+1,
						dataClassMutableMessage(file, idx, prop))
					// A val cannot have a setter, and an override of a mutable property cannot become a val.
					if !dataClassPropertyHasSetter(file, prop) && !file.FlatHasModifier(prop, "override") {
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(varKeyword)),
							EndByte:     int(file.FlatEndByte(varKeyword)),
							Replacement: "val",
						}
					}
					ctx.Emit(f)
				})
			}
		},
	})
}

func registerStyleDataClassContainsFunctions() {
	r := &DataClassContainsFunctionsRule{BaseRule: BaseRule{RuleName: "DataClassContainsFunctions", RuleSetName: "style", Sev: "warning", Desc: "Detects data classes that contain function members."}, ConversionFunctionPrefix: []string{"to"}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasModifier(idx, "data") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			name := extractIdentifierFlat(file, idx)
			for i, fn := range dataClassOffendingFunctionsFlat(file, body, r.ConversionFunctionPrefix, r.AllowOperators) {
				f := r.Finding(file, declLine(file, fn), declCol(file, fn),
					fmt.Sprintf("The data class %s contains functions which are not registered conversion functions. The offending method is called %s", name, extractIdentifierFlat(file, fn)))
				// One fix per class: every finding in a class drops the same modifier.
				if modNode := file.FlatFindModifierNode(idx, "data"); i == 0 && modNode != 0 {
					startByte := int(file.FlatStartByte(modNode))
					endByte := int(file.FlatEndByte(modNode))
					for endByte < len(file.Content) && (file.Content[endByte] == ' ' || file.Content[endByte] == '\t') {
						endByte++
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   startByte,
						EndByte:     endByte,
						Replacement: "",
					}
				}
				ctx.Emit(f)
			}
		},
	})
}

func registerStyleProtectedMemberInFinalClass() {
	r := &ProtectedMemberInFinalClassRule{BaseRule: BaseRule{RuleName: "ProtectedMemberInFinalClass", RuleSetName: "style", Sev: "warning", Desc: "Detects protected members in final classes where they should be private."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Needs: api.NeedsResolver,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "sealed") {
				return
			}
			// Shadow classes (Robolectric @Implements) expose protected members by contract.
			if flatHasAnnotationNamed(file, idx, "Implements") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			hasProtectedMember := false
			forEachDirectClassMemberFlat(file, body, func(member uint32) {
				if member != 0 && file.FlatHasModifier(member, "protected") && !protectedMemberHasFixedVisibility(file, member) {
					hasProtectedMember = true
				}
			})
			if !hasProtectedMember {
				return
			}
			if ctx.Resolver != nil {
				name := extractIdentifierFlat(file, idx)
				if name != "" {
					info := ctx.Resolver.ClassHierarchy(name)
					if info != nil && info.IsOpen {
						return
					}
				}
			}
			forEachDirectClassMemberFlat(file, body, func(member uint32) {
				if member == 0 || !file.FlatHasModifier(member, "protected") || protectedMemberHasFixedVisibility(file, member) {
					return
				}
				f := r.Finding(file, file.FlatRow(member)+1, file.FlatCol(member)+1,
					"Protected member in final class should be private.")
				protectedNode := file.FlatFindModifierNode(member, "protected")
				if protectedNode != 0 {
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(protectedNode)),
						EndByte:     int(file.FlatEndByte(protectedNode)),
						Replacement: "private",
					}
				}
				ctx.Emit(f)
			})
		},
	})
}

func registerStyleNestedClassesVisibility() {
	r := &NestedClassesVisibilityRule{BaseRule: BaseRule{RuleName: "NestedClassesVisibility", RuleSetName: "style", Sev: "warning", Desc: "Detects nested classes with explicit public modifier inside internal parent classes."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			parent, ok := file.FlatParent(idx)
			if !ok || file.FlatType(parent) != "source_file" {
				return
			}
			if file.FlatHasChildOfType(idx, "interface") {
				return
			}
			if !file.FlatHasModifier(idx, "internal") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			for i := 0; i < file.FlatChildCount(body); i++ {
				child := file.FlatChild(body, i)
				childType := file.FlatType(child)
				if childType != "class_declaration" && childType != "object_declaration" {
					continue
				}
				if childType == "companion_object" {
					continue
				}
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
				if !file.FlatHasModifier(child, "public") {
					continue
				}
				name := extractIdentifierFlat(file, child)
				f := r.Finding(file, file.FlatRow(child)+1, file.FlatCol(child)+1,
					fmt.Sprintf("The nested class '%s' has an explicit public modifier. Within an internal class this is misleading, as the nested class is still internal.", name))
				if modNode := file.FlatFindModifierNode(child, "public"); modNode != 0 {
					startByte := int(file.FlatStartByte(modNode))
					endByte := int(file.FlatEndByte(modNode))
					for endByte < len(file.Content) && (file.Content[endByte] == ' ' || file.Content[endByte] == '\t') {
						endByte++
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   startByte,
						EndByte:     endByte,
						Replacement: "",
					}
				}
				ctx.Emit(f)
			}
		},
	})
}

func registerStyleUtilityClassWithPublicConstructor() {
	r := &UtilityClassWithPublicConstructorRule{BaseRule: BaseRule{RuleName: "UtilityClassWithPublicConstructor", RuleSetName: "style", Sev: "warning", Desc: "Detects utility classes that have a public constructor instead of a private one."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatHasChildOfType(idx, "interface") {
				return
			}
			for _, mod := range []string{"sealed", "data", "enum", "open", "annotation", "inner", "value", "inline"} {
				if file.FlatHasModifier(idx, mod) {
					return
				}
			}
			if file.FlatHasModifier(idx, "abstract") || utilityClassHasFrameworkAnnotation(file, idx) {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			hasFunctions := false
			hasNonStaticMember := false
			hasNamespaceMember := false
			for i := 0; i < file.FlatChildCount(body); i++ {
				child := file.FlatChild(body, i)
				switch file.FlatType(child) {
				case "companion_object":
					// An empty companion adds no static member.
					if companion, ok := file.FlatFindChild(child, "class_body"); ok &&
						(file.FlatHasChildOfType(companion, "function_declaration") || file.FlatHasChildOfType(companion, "property_declaration")) {
						hasFunctions = true
					}
				case "function_declaration", "property_declaration", "anonymous_initializer", "secondary_constructor":
					hasNonStaticMember = true
				case "class_declaration", "object_declaration":
					// A class that nests types is a namespace holder, not a utility class.
					hasNamespaceMember = true
				}
			}
			if !hasFunctions || hasNonStaticMember || hasNamespaceMember {
				return
			}
			ctor, _ := file.FlatFindChild(idx, "primary_constructor")
			if ctor != 0 {
				if file.FlatHasModifier(ctor, "private") || file.FlatHasModifier(ctor, "internal") || file.FlatHasModifier(ctor, "protected") ||
					utilityClassHasFrameworkAnnotation(file, ctor) {
					return
				}
				ctorText := file.FlatNodeText(ctor)
				if strings.Contains(ctorText, "val ") || strings.Contains(ctorText, "var ") {
					return
				}
			}
			for i := 0; i < file.FlatChildCount(idx); i++ {
				if file.FlatType(file.FlatChild(idx, i)) == "delegation_specifier" {
					return
				}
			}
			name := extractIdentifierFlat(file, idx)
			f := r.Finding(file, declLine(file, idx), declCol(file, idx),
				fmt.Sprintf("Utility class '%s' should have a private constructor.", name))
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
				body2, _ := file.FlatFindChild(idx, "class_body")
				if body2 != 0 {
					insertAt := int(file.FlatStartByte(body2))
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
			ctx.Emit(f)
		},
	})
}

func registerStyleOptionalAbstractKeyword() {
	r := &OptionalAbstractKeywordRule{BaseRule: BaseRule{RuleName: "OptionalAbstractKeyword", RuleSetName: "style", Sev: "warning", Desc: "Detects redundant abstract modifier on interface members where it is implied."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		// The grammar parses `fun interface X { ... }` as an ERROR header followed by a lambda_literal body.
		NodeTypes: []string{"class_declaration", "lambda_literal", "class_body"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			switch file.FlatType(idx) {
			case "lambda_literal":
				if members, ok := funInterfaceMisparsedMembers(file, idx); ok {
					r.checkInterfaceMembers(ctx, members, false)
				}
				return
			case "class_body":
				if members := funInterfaceNestedMembers(file, idx); len(members) > 0 {
					r.checkInterfaceMembers(ctx, members, false)
				}
				return
			}
			if !file.FlatHasChildOfType(idx, "interface") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			// An ERROR before the body means the header or braces misparsed, and this body may belong to a
			// nested class (for example one with an annotated supertype).
			for child := file.FlatFirstChild(idx); child != 0 && child != body; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "ERROR" {
					return
				}
			}
			var members []uint32
			for i := 0; i < file.FlatNamedChildCount(body); i++ {
				member := file.FlatNamedChild(body, i)
				if member == 0 {
					continue
				}
				members = append(members, member)
			}
			r.checkInterfaceMembers(ctx, members, true)
		},
	})
}

// funInterfaceMisparsedMembers returns the members of a `fun interface` body that the grammar
// parsed as a lambda_literal directly after an ERROR node holding the `fun interface Name` header.
func funInterfaceMisparsedMembers(file *scanner.File, lambda uint32) ([]uint32, bool) {
	prev, ok := file.FlatPrevSibling(lambda)
	if !ok || file.FlatType(prev) != "ERROR" || !funInterfaceHeaderRe.MatchString(file.FlatNodeText(prev)) {
		return nil, false
	}
	statements, ok := file.FlatFindChild(lambda, "statements")
	if !ok {
		return nil, false
	}
	var members []uint32
	for member := file.FlatFirstChild(statements); member != 0; member = file.FlatNextSib(member) {
		if file.FlatIsNamed(member) {
			members = append(members, member)
		}
	}
	return members, true
}

// funInterfaceNestedMembers returns the members that follow a misparsed nested `fun interface X {`
// header in a class body. The closing brace ends the class body early, so the rest belong to the interface.
func funInterfaceNestedMembers(file *scanner.File, body uint32) []uint32 {
	var members []uint32
	inInterface := false
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "ERROR" {
			inInterface = funInterfaceHeaderRe.MatchString(file.FlatNodeText(child))
			continue
		}
		if inInterface {
			members = append(members, child)
		}
	}
	return members
}

var funInterfaceHeaderRe = regexp.MustCompile(`^\s*(?:[\w@()."]+\s+)*fun\s+interface\s+\w+`)

// checkInterfaceMembers reports redundant `abstract` on interface function and property members.
func (r *OptionalAbstractKeywordRule) checkInterfaceMembers(ctx *api.Context, members []uint32, alignedOnly bool) {
	file := ctx.File
	baseColumn := -1
	if alignedOnly {
		for _, member := range members {
			if col := file.FlatCol(member); baseColumn == -1 || col < baseColumn {
				baseColumn = col
			}
		}
	}
	for _, member := range members {
		switch file.FlatType(member) {
		case "function_declaration", "property_declaration":
		default:
			continue
		}
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
		mods, _ := file.FlatFindChild(member, "modifiers")
		if mods == 0 || !file.FlatHasModifier(member, "abstract") {
			continue
		}
		modsText := file.FlatNodeText(mods)
		f := r.Finding(file, file.FlatRow(mods)+1, file.FlatCol(mods)+1,
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
		ctx.Emit(f)
	}
}

// classOrderingSection returns the detekt section priority of a class member, or -1 when the member is not ordered.
func classOrderingSection(file *scanner.File, member uint32) int {
	switch file.FlatType(member) {
	case "property_declaration", "anonymous_initializer":
		return 0
	case "secondary_constructor":
		return 1
	case "function_declaration":
		return 2
	case "companion_object":
		return 3
	}
	return -1
}

var classOrderingSectionNames = [...]string{
	"property declarations and initializer blocks",
	"secondary constructors",
	"method declarations",
	"companion object",
}

func classOrderingDescription(file *scanner.File, member uint32) string {
	switch file.FlatType(member) {
	case "property_declaration":
		return "property `" + extractIdentifierFlat(file, member) + "`"
	case "anonymous_initializer":
		return "initializer blocks"
	case "secondary_constructor":
		return "secondary constructor"
	case "function_declaration":
		return "method `" + extractIdentifierFlat(file, member) + "()`"
	}
	return "companion object"
}

// classOrderingViolators mirrors detekt: keep a longest non-decreasing run of sections (the first one
// found) and report the members outside it.
func classOrderingViolators(sections []int) (violators []int, kept map[int]bool) {
	n := len(sections)
	if n == 0 {
		return nil, nil
	}
	lengths := make([]int, n)
	prev := make([]int, n)
	for i := range lengths {
		lengths[i], prev[i] = 1, i
	}
	for i := 0; i < n; i++ {
		for j := 0; j < i; j++ {
			if sections[i] >= sections[j] && lengths[i] < lengths[j]+1 {
				lengths[i] = lengths[j] + 1
				prev[i] = j
			}
		}
	}
	best := 0
	for i := 1; i < n; i++ {
		if lengths[best] < lengths[i] {
			best = i
		}
	}
	kept = map[int]bool{}
	for i := best; ; i = prev[i] {
		kept[i] = true
		if prev[i] == i {
			break
		}
	}
	for i := 0; i < n; i++ {
		if !kept[i] {
			violators = append(violators, i)
		}
	}
	return violators, kept
}

func registerStyleClassOrdering() {
	r := &ClassOrderingRule{BaseRule: BaseRule{RuleName: "ClassOrdering", RuleSetName: "style", Sev: "warning", Desc: "Detects class members that are not in the conventional ordering of properties, init blocks, constructors, functions, and companion objects."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_body", "enum_class_body"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			var members []uint32
			var sections []int
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				if sec := classOrderingSection(file, child); sec >= 0 {
					members = append(members, child)
					sections = append(sections, sec)
				}
			}
			violators, kept := classOrderingViolators(sections)
			for _, v := range violators {
				// A kept member before the violator with a later section wins; otherwise the last kept
				// member of an earlier section, wherever it sits.
				ref, direction := -1, ""
				for i := 0; i < v; i++ {
					if kept[i] && sections[i] > sections[v] {
						ref, direction = i, "before"
						break
					}
				}
				if ref < 0 {
					for i := len(sections) - 1; i >= 0; i-- {
						if kept[i] && sections[i] < sections[v] {
							ref, direction = i, "after"
							break
						}
					}
				}
				if ref < 0 {
					continue
				}
				ctx.EmitAt(declLine(file, members[v]), declCol(file, members[v]),
					classOrderingDescription(file, members[v])+" should be declared "+direction+" "+classOrderingSectionNames[sections[ref]]+".")
			}
		},
	})
}

func registerStyleObjectLiteralToLambda() {
	r := &ObjectLiteralToLambdaRule{BaseRule: BaseRule{RuleName: "ObjectLiteralToLambda", RuleSetName: "style", Sev: "warning", Desc: "Detects object literals implementing a single method that could be converted to a lambda."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"object_literal"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Needs: api.NeedsResolver,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			delegations := flatDirectChildrenOfType(file, idx, "delegation_specifier")
			if len(delegations) != 1 {
				return
			}
			specText := file.FlatNodeText(delegations[0])
			if strings.Contains(specText, "(") {
				return
			}
			supertypeName := extractSupertypeNameFlat(file, delegations[0])
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
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
			if funCount != 1 || propCount != 0 || hasInit {
				return
			}
			if !file.FlatHasModifier(singleFun, "override") {
				return
			}
			funBody, _ := file.FlatFindChild(singleFun, "function_body")
			if funBody != 0 && objectBodyContainsBareThisFlat(file, funBody) {
				return
			}
			// A generic method cannot be a lambda, and a call to the method's own name recurses into the object, not the outer scope.
			if file.FlatHasChildOfType(singleFun, "type_parameters") {
				return
			}
			if funBody != 0 && objectBodyCallsFunctionFlat(file, funBody, extractIdentifierFlat(file, singleFun)) {
				return
			}
			// Without a resolvable supertype name the interface may not be a SAM, so skip.
			if supertypeName == "" || !isSAMConvertible(supertypeName, file, ctx.Resolver) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				"Object literal with single method can be converted to a lambda.")
		},
	})
}

func registerStyleSerialVersionUIDInSerializableClass() {
	r := &SerialVersionUIDInSerializableClassRule{BaseRule: BaseRule{RuleName: "SerialVersionUIDInSerializableClass", RuleSetName: "style", Sev: "warning", Desc: "Detects Serializable classes that are missing a serialVersionUID field."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration", "object_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			// An interface cannot hold the field, and an enum is serialized by name, so a serialVersionUID there is moot
			// (detekt reports an enum; krit does not).
			if file.FlatHasChildOfType(idx, "interface") || file.FlatHasChildOfType(idx, "enum") {
				return
			}
			name := extractIdentifierFlat(file, idx)
			// detekt compares each declared supertype entry's text to `Serializable`; it ignores `java.io.Serializable`,
			// `Externalizable`, and supertypes that are Serializable only through a parent class.
			implementsSerializable := false
			for i := 0; i < file.FlatChildCount(idx); i++ {
				child := file.FlatChild(idx, i)
				if file.FlatType(child) == "delegation_specifier" && strings.TrimSpace(file.FlatNodeText(child)) == "Serializable" {
					implementsSerializable = true
					break
				}
			}
			if !implementsSerializable {
				return
			}
			// Like detekt, only a property in the companion object (or the object itself) counts, and it must be
			// `private const val serialVersionUID: Long`.
			if prop := serialVersionUIDProperty(file, idx); prop != 0 {
				if !serialVersionUIDSignatureIsCorrect(file, prop) {
					ctx.EmitAt(declLine(file, prop), declCol(file, prop), serialVersionUIDSignatureMessage)
				}
				return
			}
			kind := "class"
			if file.FlatType(idx) == "object_declaration" {
				kind = "object"
			}
			f := r.Finding(file, declLine(file, idx), declCol(file, idx),
				fmt.Sprintf("The %s %s implements the `Serializable` interface and should thus define a `serialVersionUID`.", kind, name))
			f.Fix = buildSerialVersionUIDFix(file, idx)
			ctx.Emit(f)
		},
	})
}

// buildSerialVersionUIDFix injects a `private const val serialVersionUID: Long = 1L`
// property into the class. If the class already has a companion object,
// the property is added inside it; otherwise a new companion object is
// inserted at the end of the class body. Returns nil when the class has
// no body (declaration-only) — there's nowhere to inject.
func buildSerialVersionUIDFix(file *scanner.File, idx uint32) *scanner.Fix {
	body, _ := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}
	classCol := file.FlatCol(idx)
	indent := strings.Repeat(" ", classCol)
	memberIndent := indent + "    "
	innerIndent := memberIndent + "    "

	var companion uint32
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "companion_object" {
			companion = child
			break
		}
	}
	if companion != 0 {
		innerBody, _ := file.FlatFindChild(companion, "class_body")
		if innerBody == 0 {
			return nil
		}
		// Insert just after the opening `{` of the companion's body.
		insertAt := int(file.FlatStartByte(innerBody)) + 1
		// Use the companion's own column for indentation reference.
		companionCol := file.FlatCol(companion)
		propIndent := strings.Repeat(" ", companionCol) + "    "
		return &scanner.Fix{
			ByteMode:    true,
			StartByte:   insertAt,
			EndByte:     insertAt,
			Replacement: "\n" + propIndent + "private const val serialVersionUID: Long = 1L\n",
		}
	}

	// No companion object — insert a new one immediately before the
	// class body's closing `}`. FlatEndByte is exclusive, so `}` lives
	// at end-1.
	bodyEnd := int(file.FlatEndByte(body))
	insertAt := bodyEnd - 1
	if insertAt < 0 || insertAt >= len(file.Content) || file.Content[insertAt] != '}' {
		return nil
	}
	// Strip whitespace immediately before the closing `}` so the
	// inserted block sits on a fresh line at the right indent.
	stripStart := insertAt
	for stripStart > 0 {
		c := file.Content[stripStart-1]
		if c == ' ' || c == '\t' {
			stripStart--
			continue
		}
		break
	}
	replacement := "\n" +
		memberIndent + "companion object {\n" +
		innerIndent + "private const val serialVersionUID: Long = 1L\n" +
		memberIndent + "}\n" +
		indent
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   stripStart,
		EndByte:     insertAt,
		Replacement: replacement,
	}
}

// utilityClassHasFrameworkAnnotation reports annotations that need the class or its
// constructor to stay visible for DI or reflective construction.
func utilityClassHasFrameworkAnnotation(file *scanner.File, idx uint32) bool {
	for _, name := range []string{"Module", "Implements", "Component", "Inject", "AssistedInject"} {
		if flatHasAnnotationNamed(file, idx, name) {
			return true
		}
	}
	return false
}

// dataClassPropertyHasSetter reports a property followed by an explicit accessor block with a setter; the grammar
// places getters and setters as siblings after the declaration.
func dataClassPropertyHasSetter(file *scanner.File, prop uint32) bool {
	if _, ok := file.FlatFindChild(prop, "setter"); ok {
		return true
	}
	for sib, ok := file.FlatNextSibling(prop); ok; sib, ok = file.FlatNextSibling(sib) {
		switch file.FlatType(sib) {
		case "setter":
			return true
		case "getter":
			continue
		}
		return false
	}
	return false
}

// objectBodyCallsFunctionFlat reports whether body contains an unqualified call to name.
func objectBodyCallsFunctionFlat(file *scanner.File, body uint32, name string) bool {
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if callee := file.FlatFirstChild(call); callee != 0 && file.FlatType(callee) == "simple_identifier" && file.FlatNodeTextEquals(callee, name) {
			found = true
		}
	})
	return found
}

const serialVersionUIDSignatureMessage = "The property `serialVersionUID` signature is not correct. `serialVersionUID` should be `private` and `constant` and its type should be `Long`"

// serialVersionUIDProperty returns the serialVersionUID property declared in the companion object of a class, or in
// the body of an object, or 0.
func serialVersionUIDProperty(file *scanner.File, class uint32) uint32 {
	body, _ := file.FlatFindChild(class, "class_body")
	if body == 0 {
		return 0
	}
	find := func(scope uint32) uint32 {
		for child := file.FlatFirstChild(scope); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "property_declaration" && propertyDeclarationName(file, child) == "serialVersionUID" {
				return child
			}
		}
		return 0
	}
	if file.FlatType(class) == "object_declaration" {
		return find(body)
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "companion_object" {
			if inner, _ := file.FlatFindChild(child, "class_body"); inner != 0 {
				if prop := find(inner); prop != 0 {
					return prop
				}
			}
		}
	}
	return 0
}

// serialVersionUIDSignatureIsCorrect reports `private const val ...: Long`.
func serialVersionUIDSignatureIsCorrect(file *scanner.File, prop uint32) bool {
	if !file.FlatHasModifier(prop, "private") || !file.FlatHasModifier(prop, "const") {
		return false
	}
	if typeNode, ok := file.FlatFindChild(prop, "user_type"); ok && typeNode != 0 {
		return file.FlatNodeTextEquals(typeNode, "Long")
	}
	if v, ok := file.FlatFindChild(prop, "variable_declaration"); ok && v != 0 {
		if typeNode, ok := file.FlatFindChild(v, "user_type"); ok && typeNode != 0 {
			return file.FlatNodeTextEquals(typeNode, "Long")
		}
	}
	// Without a stated type the initializer decides: only a long literal is a Long.
	text := strings.TrimSpace(file.FlatNodeText(prop))
	if eq := strings.Index(text, "="); eq >= 0 {
		init := strings.TrimSpace(text[eq+1:])
		return strings.HasSuffix(init, "L") || strings.HasSuffix(init, "l")
	}
	return false
}
