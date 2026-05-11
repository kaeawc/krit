package rules

import (
	"fmt"
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
		NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
		Needs: api.NeedsResolver,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasModifier(idx, "abstract") {
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
					allResolved := true
					for _, st := range info.Supertypes {
						parts := strings.Split(st, ".")
						stName := parts[len(parts)-1]
						stInfo := ctx.Resolver.ClassHierarchy(stName)
						if stInfo == nil {
							stInfo = ctx.Resolver.ClassHierarchy(st)
						}
						if stInfo == nil {
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
						return
					}
				}
			}
			if !hasAbstractMember {
				name := extractIdentifierFlat(file, idx)
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Abstract class '%s' has no abstract members. Make it concrete.", name))
				modsText2 := file.FlatNodeText(mods)
				newMods := strings.Replace(modsText2, "abstract ", "", 1)
				if newMods == modsText2 {
					newMods = strings.Replace(modsText2, "abstract", "", 1)
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(mods)),
					EndByte:     int(file.FlatEndByte(mods)),
					Replacement: newMods,
				}
				ctx.Emit(f)
			}
		},
	})
}

func registerStyleAbstractClassCanBeInterface() {
	r := &AbstractClassCanBeInterfaceRule{BaseRule: BaseRule{RuleName: "AbstractClassCanBeInterface", RuleSetName: "style", Sev: "warning", Desc: "Detects abstract classes with no state that could be converted to interfaces."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasModifier(idx, "abstract") {
				return
			}
			if abstractClassCanBeInterfaceHasFrameworkClassAnnotation(file, idx) {
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
			f := r.Finding(file, file.FlatRow(idx)+1, 1,
				fmt.Sprintf("Abstract class '%s' has no state and could be an interface.", name))
			type replEntry struct {
				start, end int
				repl       string
			}
			var repls []replEntry
			abstractNode := file.FlatFindModifierNode(idx, "abstract")
			if abstractNode != 0 {
				endByte := int(file.FlatEndByte(abstractNode))
				for endByte < int(file.FlatEndByte(idx)) && (file.Content[endByte] == ' ' || file.Content[endByte] == '\t') {
					endByte++
				}
				repls = append(repls, replEntry{int(file.FlatStartByte(abstractNode)), endByte, ""})
			}
			for i := 0; i < file.FlatChildCount(idx); i++ {
				child := file.FlatChild(idx, i)
				if file.FlatNodeTextEquals(child, "class") {
					repls = append(repls, replEntry{int(file.FlatStartByte(child)), int(file.FlatEndByte(child)), "interface"})
					break
				}
			}
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
			if len(repls) > 0 {
				for i := 0; i < len(repls); i++ {
					for j := i + 1; j < len(repls); j++ {
						if repls[j].start > repls[i].start {
							repls[i], repls[j] = repls[j], repls[i]
						}
					}
				}
				nodeText := file.FlatNodeText(idx)
				base := int(file.FlatStartByte(idx))
				for _, rr := range repls {
					relStart := rr.start - base
					relEnd := rr.end - base
					if relStart >= 0 && relEnd <= len(nodeText) {
						nodeText = nodeText[:relStart] + rr.repl + nodeText[relEnd:]
					}
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: nodeText,
				}
			}
			ctx.Emit(f)
		},
	})
}

func registerStyleDataClassShouldBeImmutable() {
	r := &DataClassShouldBeImmutableRule{BaseRule: BaseRule{RuleName: "DataClassShouldBeImmutable", RuleSetName: "style", Sev: "warning", Desc: "Detects data class properties declared as var instead of val."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
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
					f := r.Finding(file, file.FlatRow(child)+1, 1,
						"Data class property should be immutable. Use 'val' instead of 'var'.")
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(varKeyword)),
						EndByte:     int(file.FlatEndByte(varKeyword)),
						Replacement: "val",
					}
					ctx.Emit(f)
				}
				if ctx.Resolver != nil && dataClassBindingKeyword(file, child, "val") != 0 {
					for i := 0; i < file.FlatChildCount(child); i++ {
						typeChild := file.FlatChild(child, i)
						if t := file.FlatType(typeChild); t == "user_type" || t == "nullable_type" {
							resolved := ctx.Resolver.ResolveFlatNode(typeChild, file)
							if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && mutableCollectionTypes[resolved.Name] {
								ctx.EmitAt(file.FlatRow(child)+1, 1,
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
					f := r.Finding(file, file.FlatRow(prop)+1, 1,
						"Data class property should be immutable. Use 'val' instead of 'var'.")
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(varKeyword)),
						EndByte:     int(file.FlatEndByte(varKeyword)),
						Replacement: "val",
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
		NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasModifier(idx, "data") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			if dataClassHasNonConversionFunctionFlat(file, body, r.ConversionFunctionPrefix) {
				name := extractIdentifierFlat(file, idx)
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Data class '%s' contains functions. Consider using a regular class.", name))
			}
		},
	})
}

func registerStyleProtectedMemberInFinalClass() {
	r := &ProtectedMemberInFinalClassRule{BaseRule: BaseRule{RuleName: "ProtectedMemberInFinalClass", RuleSetName: "style", Sev: "warning", Desc: "Detects protected members in final classes where they should be private."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
		Needs: api.NeedsResolver,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "sealed") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
			hasProtectedMember := false
			forEachDirectClassMemberFlat(file, body, func(member uint32) {
				if member != 0 && file.FlatHasModifier(member, "protected") {
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
				if member == 0 || !file.FlatHasModifier(member, "protected") {
					return
				}
				f := r.Finding(file, file.FlatRow(member)+1, 1,
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
		NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Implementation: r,
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
				ctx.EmitAt(file.FlatRow(child)+1, 1,
					fmt.Sprintf("The nested class '%s' has an explicit public modifier. Within an internal class this is misleading, as the nested class is still internal.", name))
			}
		},
	})
}

func registerStyleUtilityClassWithPublicConstructor() {
	r := &UtilityClassWithPublicConstructorRule{BaseRule: BaseRule{RuleName: "UtilityClassWithPublicConstructor", RuleSetName: "style", Sev: "warning", Desc: "Detects utility classes that have a public constructor instead of a private one."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			nodeText := file.FlatNodeText(idx)
			prefix := strings.TrimSpace(nodeText)
			if len(prefix) > 200 {
				prefix = prefix[:200]
			}
			if strings.Contains(prefix, "interface ") ||
				strings.Contains(prefix, "sealed ") ||
				strings.Contains(prefix, "data ") ||
				strings.Contains(prefix, "enum ") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
			}
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
				return
			}
			ctor, _ := file.FlatFindChild(idx, "primary_constructor")
			if ctor != 0 {
				if file.FlatHasModifier(ctor, "private") {
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
			f := r.Finding(file, file.FlatRow(idx)+1, 1,
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
		NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: api.FixCosmetic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasChildOfType(idx, "interface") {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				return
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
				ctx.Emit(f)
			}
		},
	})
}

func registerStyleClassOrdering() {
	r := &ClassOrderingRule{BaseRule: BaseRule{RuleName: "ClassOrdering", RuleSetName: "style", Sev: "warning", Desc: "Detects class members that are not in the conventional ordering of properties, init blocks, constructors, functions, and companion objects."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_body"}, Confidence: 0.75, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
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
					ctx.EmitAt(file.FlatRow(child)+1, 1,
						"Class members should be ordered: properties, init blocks, constructors, functions, companion object.")
					return
				}
				lastOrder = currentOrder
			}
		},
	})
}

func registerStyleObjectLiteralToLambda() {
	r := &ObjectLiteralToLambdaRule{BaseRule: BaseRule{RuleName: "ObjectLiteralToLambda", RuleSetName: "style", Sev: "warning", Desc: "Detects object literals implementing a single method that could be converted to a lambda."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"object_literal"}, Confidence: 0.75, Implementation: r,
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
			if supertypeName != "" && !isSAMConvertible(supertypeName, file, ctx.Resolver) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, 1,
				"Object literal with single method can be converted to a lambda.")
		},
	})
}

func registerStyleSerialVersionUIDInSerializableClass() {
	r := &SerialVersionUIDInSerializableClassRule{BaseRule: BaseRule{RuleName: "SerialVersionUIDInSerializableClass", RuleSetName: "style", Sev: "warning", Desc: "Detects Serializable classes that are missing a serialVersionUID field."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
		Needs: api.NeedsResolver,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if file.FlatHasChildOfType(idx, "enum") {
				return
			}
			// Exact AST lookup: does the class (or its companion object)
			// declare a property named serialVersionUID? No text
			// scanning — a string literal or comment mentioning the
			// name no longer suppresses the warning by accident.
			if classDeclaresStaticProperty(file, idx, "serialVersionUID") {
				return
			}
			name := extractIdentifierFlat(file, idx)
			implementsSerializable := false
			for i := 0; i < file.FlatChildCount(idx); i++ {
				child := file.FlatChild(idx, i)
				if file.FlatType(child) != "delegation_specifier" {
					continue
				}
				supertypeName := viewConstructorSupertypeNameFlat(file, child)
				if supertypeName == "" {
					continue
				}
				if supertypeName == "Serializable" || supertypeName == "Externalizable" {
					implementsSerializable = true
					break
				}
				if ctx.Resolver != nil {
					if info := ctx.Resolver.ClassHierarchy(supertypeName); info != nil {
						if checksSerializable(ctx.Resolver, info) {
							implementsSerializable = true
							break
						}
					}
				}
			}
			if !implementsSerializable {
				return
			}
			f := r.Finding(file, file.FlatRow(idx)+1, 1,
				fmt.Sprintf("Serializable class '%s' is missing serialVersionUID.", name))
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
