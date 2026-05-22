package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerDiHygieneRules() {

	// --- from di_hygiene.go ---
	{
		r := &DiCycleDetectionRule{BaseRule: BaseRule{RuleName: "DiCycleDetection", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects cycles in the constructor-injected DI binding graph."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsParsedFiles, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &AnvilMergeComponentEmptyScopeRule{BaseRule: BaseRule{RuleName: "AnvilMergeComponentEmptyScope", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @MergeComponent scopes with no matching @ContributesTo or @ContributesBinding declarations."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &AnvilContributesBindingWithoutScopeRule{BaseRule: BaseRule{RuleName: "AnvilContributesBindingWithoutScope", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @ContributesBinding scope mismatches with the @ContributesTo scope on the bound interface."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				bindingScope := anvilAnnotationScopeFlat(file, idx, "ContributesBinding")
				if bindingScope == "" {
					return
				}
				interfaceScopes := anvilContributedInterfaceScopesFlat(file)
				if len(interfaceScopes) == 0 {
					return
				}
				for _, iface := range anvilImplementedTypesFlat(file, idx) {
					if iface == "" {
						continue
					}
					ifaceScope, ok := interfaceScopes[iface]
					if !ok || ifaceScope == "" || ifaceScope == bindingScope {
						continue
					}
					name := extractIdentifierFlat(file, idx)
					if name == "" {
						name = "binding"
					}
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@ContributesBinding(%s::class) on '%s' binds '%s', which is only contributed to %s::class.", bindingScope, name, iface, ifaceScope))
					return
				}
			},
		})
	}
	{
		r := &BindsMismatchedArityRule{BaseRule: BaseRule{RuleName: "BindsMismatchedArity", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @Binds functions that do not declare exactly one parameter as required by Dagger."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Binds") {
					return
				}
				paramCount := 0
				if params, ok := file.FlatFindChild(idx, "function_value_parameters"); ok {
					walkFunctionParametersFlat(file, params, func(_ uint32) {
						paramCount++
					})
				}
				if paramCount == 1 {
					return
				}
				name := extractIdentifierFlat(file, idx)
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@Binds function '%s' must declare exactly one parameter; found %d.", name, paramCount))
			},
		})
	}
	{
		r := &DeadBindingsRule{BaseRule: BaseRule{RuleName: "DeadBindings", RuleSetName: diHygieneRuleSet, Sev: "info", Desc: "Detects @Provides/@Binds functions whose return type is not requested by any @Inject site or component exposure in the project."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &HiltInstallInMismatchRule{BaseRule: BaseRule{RuleName: "HiltInstallInMismatch", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects Hilt @Module/@InstallIn classes whose @Provides scope does not match the installed component."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Module") {
					return
				}
				component := anvilAnnotationScopeFlat(file, idx, "InstallIn")
				if component == "" {
					return
				}
				allowed, known := hiltComponentScopes[component]
				if !known {
					return
				}
				body, ok := file.FlatFindChild(idx, "class_body")
				if !ok || body == 0 {
					return
				}
				for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) != "function_declaration" {
						continue
					}
					if !hasAnnotationFlat(file, child, "Provides") {
						continue
					}
					for _, scope := range hiltScopeAnnotations {
						if !hasAnnotationFlat(file, child, scope) {
							continue
						}
						if _, ok := allowed[scope]; ok {
							break
						}
						providerName := extractIdentifierFlat(file, child)
						if providerName == "" {
							providerName = "provider"
						}
						ctx.EmitAt(file.FlatRow(child)+1, 1, fmt.Sprintf("@Provides function '%s' is annotated @%s but the module is installed in %s; the scope does not match.", providerName, scope, component))
						break
					}
				}
			},
		})
	}
	{
		r := &InjectOnAbstractClassRule{BaseRule: BaseRule{RuleName: "InjectOnAbstractClass", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @Inject primary constructors on abstract classes; Dagger cannot instantiate an abstract class."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasModifier(idx, "abstract") {
					return
				}
				ctor, ok := file.FlatFindChild(idx, "primary_constructor")
				if !ok || ctor == 0 {
					return
				}
				if !hasAnnotationFlat(file, ctor, "Inject") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "class"
				}
				ctx.EmitAt(file.FlatRow(ctor)+1, 1, fmt.Sprintf("@Inject primary constructor on abstract class '%s'; Dagger cannot instantiate it, so the binding is unreachable.", name))
			},
		})
	}
	{
		r := &SingletonOnMutableClassRule{BaseRule: BaseRule{RuleName: "SingletonOnMutableClass", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @Singleton classes that hold unprotected mutable state (var properties or mutable collection val initializers)."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				scope := singletonScopeAnnotationFlat(file, idx)
				if scope == "" {
					return
				}
				body, ok := file.FlatFindChild(idx, "class_body")
				if !ok || body == 0 {
					return
				}
				for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) != "property_declaration" {
						continue
					}
					propName := extractIdentifierFlat(file, child)
					if propName == "" {
						continue
					}
					reason := ""
					if propertyDeclarationIsVar(file, child) {
						reason = "var property"
					} else if callee := singletonPropertyInitCallee(file, child); callee != "" {
						if _, mut := singletonMutableCollectionFactories[callee]; mut {
							reason = fmt.Sprintf("val initialised by %s()", callee)
						}
					}
					if reason == "" {
						continue
					}
					className := extractIdentifierFlat(file, idx)
					if className == "" {
						className = "class"
					}
					ctx.EmitAt(file.FlatRow(child)+1, 1, fmt.Sprintf("@%s class '%s' holds unprotected mutable state in '%s' (%s); singleton-scoped state should be thread-safe.", scope, className, propName, reason))
				}
			},
		})
	}
	{
		r := &MetroFactoryDeclarationShapeRule{BaseRule: BaseRule{RuleName: "MetroFactoryDeclarationShape", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects Metro factory annotations on concrete or sealed declarations; Metro factories must be interfaces or non-sealed abstract classes."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasMetroFactoryAnnotationFlat(file, idx) {
					return
				}
				if file.FlatHasChildOfType(idx, "interface") && !file.FlatHasModifier(idx, "sealed") {
					return
				}
				if file.FlatHasModifier(idx, "abstract") && !file.FlatHasModifier(idx, "sealed") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "graph factory"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Metro factory '%s' must be an interface or a non-sealed abstract class so Metro can generate its implementation.", name))
			},
		})
	}
	{
		r := &ScopeOnParameterizedClassRule{BaseRule: BaseRule{RuleName: "ScopeOnParameterizedClass", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects DI scope annotations on generic classes; the type parameter is erased so the scope holds a single instance."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if _, ok := file.FlatFindChild(idx, "type_parameters"); !ok {
					return
				}
				scope := firstScopeAnnotationFlat(file, idx)
				if scope == "" {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "class"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@%s on generic class '%s' shares one instance across all type arguments because the type parameter is erased at runtime.", scope, name))
			},
		})
	}
	{
		r := &MissingJvmSuppressWildcardsRule{BaseRule: BaseRule{RuleName: "MissingJvmSuppressWildcards", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @Provides/@Binds returning Set<T> or Map<K,V> without @JvmSuppressWildcards on the value type."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Provides") && !hasAnnotationFlat(file, idx, "Binds") {
					return
				}
				retText := extractFunctionReturnTypeNameFlat(file, idx)
				if retText == "" {
					return
				}
				wrapper, ok := multibindingReturnNeedsJvmSuppress(retText)
				if !ok {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "binding"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@Provides/@Binds function '%s' returning %s lacks @JvmSuppressWildcards on the value type; Kotlin emits ? extends T in JVM signatures, breaking Dagger multibinding resolution.", name, wrapper))
			},
		})
	}
	{
		r := &ModuleWithNonStaticProvidesRule{BaseRule: BaseRule{RuleName: "ModuleWithNonStaticProvides", RuleSetName: diHygieneRuleSet, Sev: "info", Desc: "Detects @Module abstract classes mixing @Binds with top-level @Provides; @Provides should live in a companion object."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Module") {
					return
				}
				if !file.FlatHasModifier(idx, "abstract") {
					return
				}
				body, ok := file.FlatFindChild(idx, "class_body")
				if !ok || body == 0 {
					return
				}
				hasBinds := false
				var providesIdx uint32
				var providesName string
				for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) != "function_declaration" {
						continue
					}
					if hasAnnotationFlat(file, child, "Binds") {
						hasBinds = true
					}
					if providesIdx == 0 && hasAnnotationFlat(file, child, "Provides") {
						providesIdx = child
						providesName = extractIdentifierFlat(file, child)
					}
				}
				if !hasBinds || providesIdx == 0 {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "module"
				}
				if providesName == "" {
					providesName = "provider"
				}
				ctx.EmitAt(file.FlatRow(providesIdx)+1, 1, fmt.Sprintf("@Module abstract class '%s' mixes @Binds with a top-level @Provides function '%s'; move '%s' into a companion object so Dagger can call it statically.", name, providesName, providesName))
			},
		})
	}
	{
		r := &IntoMapMissingKeyRule{BaseRule: BaseRule{RuleName: "IntoMapMissingKey", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @IntoMap @Provides/@Binds functions that lack a @*Key annotation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "IntoMap") {
					return
				}
				if !hasAnnotationFlat(file, idx, "Provides") && !hasAnnotationFlat(file, idx, "Binds") {
					return
				}
				if hasMapKeyAnnotationFlat(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "binding"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@IntoMap function '%s' is missing a @*Key annotation; Dagger requires a key annotation on every map contribution.", name))
			},
		})
	}
	{
		r := &IntoSetOnNonSetReturnRule{BaseRule: BaseRule{RuleName: "IntoSetOnNonSetReturn", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @IntoSet @Provides functions whose return type is a collection wrapper, which silently drops the intended contribution."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "IntoSet") {
					return
				}
				if !hasAnnotationFlat(file, idx, "Provides") && !hasAnnotationFlat(file, idx, "Binds") {
					return
				}
				retText := extractFunctionReturnTypeNameFlat(file, idx)
				if retText == "" {
					return
				}
				wrapper, ok := intoSetReturnIsCollectionWrapper(retText)
				if !ok {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "binding"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@IntoSet function '%s' returns '%s', a collection wrapper; Dagger collects by return type, so the contribution will be a Set<%s> entry rather than the intended elements.", name, retText, wrapper))
			},
		})
	}
	{
		r := &SubcomponentNotInstalledRule{BaseRule: BaseRule{RuleName: "SubcomponentNotInstalled", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @Subcomponent declarations not returned from any parent component method; the subcomponent is orphaned."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &BindsInsteadOfProvidesRule{BaseRule: BaseRule{RuleName: "BindsInsteadOfProvides", RuleSetName: diHygieneRuleSet, Sev: "info", Desc: "Detects @Provides functions that return their single parameter unchanged; @Binds is cheaper."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Provides") {
					return
				}
				paramCount := 0
				if params, ok := file.FlatFindChild(idx, "function_value_parameters"); ok {
					walkFunctionParametersFlat(file, params, func(_ uint32) {
						paramCount++
					})
				}
				if paramCount != 1 {
					return
				}
				paramName, _ := firstFunctionParameterNameAndType(file, idx)
				if paramName == "" {
					return
				}
				if expressionBodyReturnsIdentifier(file, idx) != paramName {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "function"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@Provides function '%s' returns its single parameter '%s' unchanged; replace with @Binds for a cheaper abstract binding.", name, paramName))
			},
		})
	}
	{
		r := &BindsReturnTypeMatchesParamRule{BaseRule: BaseRule{RuleName: "BindsReturnTypeMatchesParam", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @Binds functions whose parameter type equals the return type; a no-op binding."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Binds") {
					return
				}
				paramCount := 0
				if params, ok := file.FlatFindChild(idx, "function_value_parameters"); ok {
					walkFunctionParametersFlat(file, params, func(_ uint32) {
						paramCount++
					})
				}
				if paramCount != 1 {
					return
				}
				_, paramType := firstFunctionParameterNameAndType(file, idx)
				retType := extractFunctionReturnTypeNameFlat(file, idx)
				if paramType == "" || retType == "" {
					return
				}
				if strings.TrimSpace(paramType) != strings.TrimSpace(retType) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "function"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@Binds function '%s' has matching parameter and return type '%s'; the binding is a no-op.", name, paramType))
			},
		})
	}
	{
		r := &ComponentMissingModuleRule{BaseRule: BaseRule{RuleName: "ComponentMissingModule", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @Component(modules = [...]) declarations whose listed modules do not transitively cover every reachable binding."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &IntoMapDuplicateKeyRule{BaseRule: BaseRule{RuleName: "IntoMapDuplicateKey", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @IntoMap providers that share the same key in the same module/component; duplicate map keys create conflicting contributions."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &IntoSetDuplicateTypeRule{BaseRule: BaseRule{RuleName: "IntoSetDuplicateType", RuleSetName: diHygieneRuleSet, Sev: "info", Desc: "Detects @IntoSet providers that contribute the same concrete impl in the same module/component; the set dedupes, dropping contributions."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ProviderInsteadOfLazyRule{BaseRule: BaseRule{RuleName: "ProviderInsteadOfLazy", RuleSetName: diHygieneRuleSet, Sev: "info", Desc: "Detects Provider<T> constructor params whose .get() is called exactly once; Lazy<T> matches the intent and is cheaper."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				ctor, ok := file.FlatFindChild(idx, "primary_constructor")
				if !ok || ctor == 0 {
					return
				}
				for child := file.FlatFirstChild(ctor); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) != "class_parameter" {
						continue
					}
					name, typeName := classParameterNameAndType(file, child)
					if name == "" || typeName != "Provider" {
						continue
					}
					count, _ := countGetCallsInClassBody(file, idx, name)
					if count != 1 {
						continue
					}
					ctx.EmitAt(file.FlatRow(child)+1, 1, fmt.Sprintf("Provider<...> parameter '%s' is dereferenced with a single .get(); inject Lazy<...> instead — it matches the same intent at lower cost.", name))
				}
			},
		})
	}
	{
		r := &LazyInsteadOfDirectRule{BaseRule: BaseRule{RuleName: "LazyInsteadOfDirect", RuleSetName: diHygieneRuleSet, Sev: "info", Desc: "Detects Lazy<T> constructor params whose .get() is called eagerly at class init; direct injection is cheaper."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				ctor, ok := file.FlatFindChild(idx, "primary_constructor")
				if !ok || ctor == 0 {
					return
				}
				for child := file.FlatFirstChild(ctor); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) != "class_parameter" {
						continue
					}
					name, typeName := classParameterNameAndType(file, child)
					if name == "" || typeName != "Lazy" {
						continue
					}
					_, atInit := countGetCallsInClassBody(file, idx, name)
					if !atInit {
						continue
					}
					ctx.EmitAt(file.FlatRow(child)+1, 1, fmt.Sprintf("Lazy<...> parameter '%s' is dereferenced eagerly at class-init; inject the underlying type directly instead.", name))
				}
			},
		})
	}
	{
		r := &HiltSingletonWithActivityDepRule{BaseRule: BaseRule{RuleName: "HiltSingletonWithActivityDep", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @Singleton classes whose constructor takes Activity-, Fragment-, View-, or LifecycleOwner-scoped types."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Singleton") {
					return
				}
				ctor, ok := file.FlatFindChild(idx, "primary_constructor")
				if !ok || ctor == 0 {
					return
				}
				paramTypes := extractClassParameterTypeNamesFlat(file, ctor)
				for _, pt := range paramTypes {
					if _, bad := hiltSingletonActivityScopedTypes[pt]; !bad {
						continue
					}
					className := extractIdentifierFlat(file, idx)
					if className == "" {
						className = "class"
					}
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@Singleton class '%s' depends on '%s', which has a narrower (Activity/Fragment/View) scope; the singleton will outlive its dependency.", className, pt))
					return
				}
			},
		})
	}
	{
		r := &HiltEntryPointOnNonInterfaceRule{BaseRule: BaseRule{RuleName: "HiltEntryPointOnNonInterface", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects Hilt @EntryPoint annotations on classes or objects instead of interfaces."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration", "prefix_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				kind, name, line, ok := hiltEntryPointDeclarationFlat(file, idx)
				if !ok || kind == "interface" {
					return
				}
				if name == "" {
					name = "entry point"
				}
				ctx.EmitAt(line, 1, fmt.Sprintf("@EntryPoint '%s' must be declared as an interface, not a %s.", name, kind))
			},
		})
	}
}
