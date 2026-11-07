package rules

import (
	"bytes"
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// DeadBindingsRule detects @Provides/@Binds functions whose return type is not
// requested by any @Inject site or component exposure anywhere in the project.
type DeadBindingsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (lower) base confidence. Reachability is approximated
// from source-visible @Inject sites and component method exposures; project-specific
// DI machinery (assisted factories, multibindings, generated code) is not modeled.
func (r *DeadBindingsRule) Confidence() float64 { return api.ConfidenceLow }

var deadBindingComponentAnnotations = []string{
	"Component", "Subcomponent", "MergeComponent", "MergeSubcomponent",
	"DefineComponent", "EntryPoint", "EarlyEntryPoint", "GraphExtension",
	"DependencyGraph", "ContributesSubcomponent", "ComponentExtension",
}

func (r *DeadBindingsRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}
	files := index.Files
	if len(files) == 0 {
		return
	}

	type bindingCandidate struct {
		file    *scanner.File
		idx     uint32
		name    string
		retType string
	}
	var bindings []bindingCandidate
	demand := make(map[string]struct{})

	addDemandTypes := func(types []string) {
		for _, t := range types {
			if t == "" {
				continue
			}
			demand[t] = struct{}{}
		}
	}
	// Lazy<Foo>, Provider<Foo> and Set<Foo> request Foo as well as the wrapper.
	addDemandTypeTexts := func(texts []string) {
		for _, text := range texts {
			addDemandTypes(typeIdentifiersIn(text))
		}
	}

	for _, file := range files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			switch file.FlatType(idx) {
			case "function_declaration":
				isProvides := hasAnnotationFlat(file, idx, "Provides")
				isBinds := hasAnnotationFlat(file, idx, "Binds")
				inComponent := isInsideComponentInterfaceFlat(file, idx)
				if isProvides || isBinds {
					ret := lastTypeSegment(extractFunctionReturnTypeNameFlat(file, idx))
					// Multibinding contributions are consumed as part of the Set or Map they join.
					if ret != "" && !diIsMultibindingContribution(file, idx) {
						bindings = append(bindings, bindingCandidate{
							file:    file,
							idx:     idx,
							name:    extractIdentifierFlat(file, idx),
							retType: ret,
						})
					}
					addDemandTypeTexts(functionValueParameterTypeTexts(file, idx))
				}
				if inComponent {
					if ret := lastTypeSegment(extractFunctionReturnTypeNameFlat(file, idx)); ret != "" {
						demand[ret] = struct{}{}
					}
					addDemandTypeTexts(functionValueParameterTypeTexts(file, idx))
				}
			case "primary_constructor":
				if hasAnnotationFlat(file, idx, "Inject") {
					addDemandTypeTexts(classParameterTypeTexts(file, idx))
				}
			case "secondary_constructor":
				if hasAnnotationFlat(file, idx, "Inject") {
					addDemandTypeTexts(functionValueParameterTypeTexts(file, idx))
				}
			case "property_declaration":
				if hasAnnotationFlat(file, idx, "Inject") {
					addDemandTypes(typeIdentifiersIn(extractPropertyTypeNameFlat(file, idx)))
				}
			}
		})
	}

	for _, b := range bindings {
		if _, ok := demand[b.retType]; ok {
			continue
		}
		name := b.name
		if name == "" {
			name = "binding"
		}
		ctx.Emit(r.Finding(
			b.file,
			declLine(b.file, b.idx),
			declCol(b.file, b.idx),
			fmt.Sprintf("@Provides/@Binds function '%s' returning '%s' is not requested by any @Inject site or component exposure in the project.", name, b.retType),
		))
	}
}

func extractFunctionReturnTypeNameFlat(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	seenColon := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		if t == ":" {
			seenColon = true
			continue
		}
		if !seenColon {
			continue
		}
		switch t {
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			return strings.TrimSpace(file.FlatNodeText(child))
		case "function_body", "=":
			return ""
		}
	}
	return ""
}

func extractPropertyTypeNameFlat(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "variable_declaration":
			if name := extractParameterTypeNameFlat(file, child); name != "" {
				return name
			}
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func extractParameterTypeNameFlat(file *scanner.File, param uint32) string {
	if file == nil || param == 0 {
		return ""
	}
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func extractFunctionParameterTypeNamesFlat(file *scanner.File, fn uint32) []string {
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return nil
	}
	var names []string
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		if name := lastTypeSegment(extractParameterTypeNameFlat(file, child)); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func extractClassParameterTypeNamesFlat(file *scanner.File, ctor uint32) []string {
	if file == nil || ctor == 0 {
		return nil
	}
	var names []string
	for child := file.FlatFirstChild(ctor); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "class_parameter" {
			continue
		}
		if name := lastTypeSegment(extractParameterTypeNameFlat(file, child)); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func lastTypeSegment(typeText string) string {
	s := strings.TrimSpace(typeText)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, "<"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSuffix(s, "?")
	s = strings.TrimSpace(s)
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		s = s[idx+1:]
	}
	return strings.TrimSpace(s)
}

func isInsideComponentInterfaceFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) != "class_declaration" {
			continue
		}
		if !file.FlatHasChildOfType(parent, "interface") {
			continue
		}
		for _, ann := range deadBindingComponentAnnotations {
			if hasAnnotationFlat(file, parent, ann) {
				return true
			}
		}
	}
	return false
}

// SubcomponentNotInstalledRule detects `@Subcomponent` declarations that are
// never returned from any parent `@Component` or parent `@Subcomponent`
// method, leaving the subcomponent orphaned.
type SubcomponentNotInstalledRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (lower) base confidence. Detection follows
// return-type text only; cross-module install graphs are not modeled.
func (r *SubcomponentNotInstalledRule) Confidence() float64 { return api.ConfidenceLow }

func (r *SubcomponentNotInstalledRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	type subDecl struct {
		file *scanner.File
		idx  uint32
		name string
	}
	var subcomponents []subDecl
	type ref struct {
		ownerName string
	}
	referencesByName := make(map[string][]ref)

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Component")) &&
			!bytes.Contains(file.Content, []byte("Subcomponent")) {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "class_declaration" {
				return
			}
			isSubcomponent := hasAnnotationFlat(file, idx, "Subcomponent") ||
				hasAnnotationFlat(file, idx, "MergeSubcomponent")
			isComponent := hasAnnotationFlat(file, idx, "Component") ||
				hasAnnotationFlat(file, idx, "MergeComponent")
			if isSubcomponent {
				name := extractIdentifierFlat(file, idx)
				if name != "" {
					subcomponents = append(subcomponents, subDecl{file: file, idx: idx, name: name})
				}
			}
			if isSubcomponent || isComponent {
				ownerName := extractIdentifierFlat(file, idx)
				returns := collectComponentReturnTypeNames(file, idx)
				for n := range returns {
					referencesByName[n] = append(referencesByName[n], ref{ownerName: ownerName})
				}
			}
		})
	}

	for _, sub := range subcomponents {
		installed := false
		for _, rf := range referencesByName[sub.name] {
			if rf.ownerName != sub.name {
				installed = true
				break
			}
		}
		if installed {
			continue
		}
		ctx.Emit(r.Finding(
			sub.file,
			declLine(sub.file, sub.idx),
			declCol(sub.file, sub.idx),
			fmt.Sprintf("@Subcomponent '%s' is never returned from any parent @Component or @Subcomponent method; the subcomponent is orphaned.", sub.name),
		))
	}
}

// collectComponentReturnTypeNames walks the body of a component class
// declaration and returns the set of last-segment names referenced by every
// method or property return type (including generic args and nested types).
func collectComponentReturnTypeNames(file *scanner.File, classIdx uint32) map[string]struct{} {
	out := make(map[string]struct{})
	body, ok := file.FlatFindChild(classIdx, "class_body")
	if !ok || body == 0 {
		return out
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_declaration":
			addTypeNameSegments(extractFunctionReturnTypeNameFlat(file, child), out)
		case "property_declaration":
			addTypeNameSegments(extractPropertyTypeNameFlat(file, child), out)
		}
	}
	return out
}

func addTypeNameSegments(typeText string, out map[string]struct{}) {
	if typeText == "" {
		return
	}
	if name := lastTypeSegment(typeText); name != "" {
		out[name] = struct{}{}
	}
	if open := strings.Index(typeText, "<"); open >= 0 && strings.HasSuffix(strings.TrimSpace(typeText), ">") {
		end := strings.LastIndex(typeText, ">")
		inner := typeText[open+1 : end]
		for _, part := range strings.Split(inner, ",") {
			addTypeNameSegments(strings.TrimSpace(part), out)
		}
	}
	// Nested type reference like `UserSubcomponent.Factory` — also add the
	// outer class so the parent-side reference satisfies the subcomponent
	// declaration.
	if dot := strings.Index(typeText, "."); dot >= 0 {
		head := strings.TrimSpace(typeText[:dot])
		if head != "" {
			out[head] = struct{}{}
		}
	}
}

// ComponentMissingModuleRule detects `@Component(modules = [...])`
// declarations whose listed modules do not transitively cover every binding
// reachable through the component's exposed methods.
type ComponentMissingModuleRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (lower) base confidence. Detection follows
// @Provides/@Binds return types and @Inject constructor parameters in source;
// generated code, multibindings, and assisted factories are not modeled.
func (r *ComponentMissingModuleRule) Confidence() float64 { return api.ConfidenceLow }

type componentMissingModuleProvider struct {
	moduleName string
	paramTypes []string
}

type componentMissingModuleComponent struct {
	file        *scanner.File
	idx         uint32
	name        string
	modules     map[string]struct{}
	returnTypes []string
}

func (r *ComponentMissingModuleRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}
	providersByType := make(map[string][]componentMissingModuleProvider)
	moduleIncludes := make(map[string]map[string]struct{})
	injectCtorParams := make(map[string][]string)
	var components []componentMissingModuleComponent

	for _, file := range index.Files {
		if !componentMissingModuleFileMayMatch(file) {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			t := file.FlatType(idx)
			if t != "class_declaration" && t != "object_declaration" {
				return
			}
			if hasAnnotationFlat(file, idx, "Module") {
				collectModuleProviders(file, idx, providersByType)
				if name := extractIdentifierFlat(file, idx); name != "" {
					if inc := annotationClassListAttr(findAnnotationTextFlat(file, idx, "Module"), "includes"); len(inc) > 0 {
						moduleIncludes[name] = inc
					}
				}
			}
			if hasAnnotationFlat(file, idx, "Component") || hasAnnotationFlat(file, idx, "MergeComponent") {
				components = append(components, collectComponentInfo(file, idx))
			}
			if name := extractIdentifierFlat(file, idx); name != "" && hasAnyInjectPrimaryConstructor(file, idx) {
				ctor, _ := file.FlatFindChild(idx, "primary_constructor")
				injectCtorParams[name] = extractClassParameterTypeNamesFlat(file, ctor)
			}
		})
	}

	for _, ci := range components {
		r.emitMissingModulesForComponent(ctx, ci, providersByType, moduleIncludes, injectCtorParams)
	}
}

func componentMissingModuleFileMayMatch(file *scanner.File) bool {
	if file == nil || file.FlatTree == nil {
		return false
	}
	return bytes.Contains(file.Content, []byte("Component")) ||
		bytes.Contains(file.Content, []byte("Module")) ||
		bytes.Contains(file.Content, []byte("Inject"))
}

func collectModuleProviders(file *scanner.File, classIdx uint32, providersByType map[string][]componentMissingModuleProvider) {
	moduleName := extractIdentifierFlat(file, classIdx)
	body, ok := file.FlatFindChild(classIdx, "class_body")
	if !ok || body == 0 {
		return
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_declaration":
			recordModuleProvider(file, child, moduleName, providersByType)
		case "companion_object", "object_declaration":
			recordCompanionProviders(file, child, moduleName, providersByType)
		}
	}
}

func recordModuleProvider(file *scanner.File, fn uint32, moduleName string, providersByType map[string][]componentMissingModuleProvider) {
	if !hasAnnotationFlat(file, fn, "Provides") && !hasAnnotationFlat(file, fn, "Binds") {
		return
	}
	ret := lastTypeSegment(extractFunctionReturnTypeNameFlat(file, fn))
	if ret == "" {
		return
	}
	providersByType[ret] = append(providersByType[ret], componentMissingModuleProvider{
		moduleName: moduleName,
		paramTypes: extractFunctionParameterTypeNamesFlat(file, fn),
	})
}

func recordCompanionProviders(file *scanner.File, comp uint32, moduleName string, providersByType map[string][]componentMissingModuleProvider) {
	for inner := file.FlatFirstChild(comp); inner != 0; inner = file.FlatNextSib(inner) {
		if file.FlatType(inner) != "class_body" {
			continue
		}
		for fn := file.FlatFirstChild(inner); fn != 0; fn = file.FlatNextSib(fn) {
			if file.FlatType(fn) != "function_declaration" {
				continue
			}
			recordModuleProvider(file, fn, moduleName, providersByType)
		}
	}
}

func collectComponentInfo(file *scanner.File, classIdx uint32) componentMissingModuleComponent {
	ci := componentMissingModuleComponent{
		file:    file,
		idx:     classIdx,
		name:    extractIdentifierFlat(file, classIdx),
		modules: parseComponentModulesAttr(file, classIdx),
	}
	body, ok := file.FlatFindChild(classIdx, "class_body")
	if !ok || body == 0 {
		return ci
	}
	for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "function_declaration":
			if ret := lastTypeSegment(extractFunctionReturnTypeNameFlat(file, c)); ret != "" {
				ci.returnTypes = append(ci.returnTypes, ret)
			}
		case "property_declaration":
			if ret := lastTypeSegment(extractPropertyTypeNameFlat(file, c)); ret != "" {
				ci.returnTypes = append(ci.returnTypes, ret)
			}
		}
	}
	return ci
}

func (r *ComponentMissingModuleRule) emitMissingModulesForComponent(
	ctx *api.Context,
	ci componentMissingModuleComponent,
	providersByType map[string][]componentMissingModuleProvider,
	moduleIncludes map[string]map[string]struct{},
	injectCtorParams map[string][]string,
) {
	if len(ci.modules) == 0 {
		return
	}
	// `@Module(includes = [...])` brings in the included modules' bindings.
	covered := make(map[string]struct{})
	var addModule func(name string)
	addModule = func(name string) {
		if _, ok := covered[name]; ok {
			return
		}
		covered[name] = struct{}{}
		for inc := range moduleIncludes[name] {
			addModule(inc)
		}
	}
	for name := range ci.modules {
		addModule(name)
	}
	ci.modules = covered
	seen := make(map[string]struct{})
	queue := append([]string{}, ci.returnTypes...)
	emitted := make(map[string]struct{})
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		providers := providersByType[t]
		// A listed module that provides the type covers it, whatever other modules also provide it.
		coveredByListed := false
		for _, p := range providers {
			if _, listed := ci.modules[p.moduleName]; listed {
				coveredByListed = true
				break
			}
		}
		for _, p := range providers {
			if !coveredByListed {
				r.maybeEmitMissingModule(ctx, ci, p, emitted)
			}
			queue = append(queue, p.paramTypes...)
		}
		queue = append(queue, injectCtorParams[t]...)
	}
}

func (r *ComponentMissingModuleRule) maybeEmitMissingModule(
	ctx *api.Context,
	ci componentMissingModuleComponent,
	p componentMissingModuleProvider,
	emitted map[string]struct{},
) {
	if p.moduleName == "" {
		return
	}
	if _, listed := ci.modules[p.moduleName]; listed {
		return
	}
	if _, already := emitted[p.moduleName]; already {
		return
	}
	emitted[p.moduleName] = struct{}{}
	compName := ci.name
	if compName == "" {
		compName = "component"
	}
	ctx.Emit(r.Finding(
		ci.file,
		declLine(ci.file, ci.idx),
		declCol(ci.file, ci.idx),
		fmt.Sprintf("@Component '%s' transitively requires bindings from '%s', which is not listed in `modules = [...]`.", compName, p.moduleName),
	))
}

// parseComponentModulesAttr returns the set of simple module names listed in
// `@Component(modules = [A::class, B::class])`. Empty if no modules attr.
func parseComponentModulesAttr(file *scanner.File, idx uint32) map[string]struct{} {
	text := findAnnotationTextFlat(file, idx, "Component")
	if text == "" {
		text = findAnnotationTextFlat(file, idx, "MergeComponent")
	}
	return annotationClassListAttr(text, "modules")
}

// annotationClassListAttr returns the simple class names in the `attr = [A::class, B::class]` argument of an
// annotation's source text, or `attr = A::class` for a single class.
func annotationClassListAttr(text, attr string) map[string]struct{} {
	out := make(map[string]struct{})
	at := -1
	for from := 0; from < len(text); {
		i := strings.Index(text[from:], attr)
		if i < 0 {
			break
		}
		i += from
		before := i == 0 || !isIdentByte(text[i-1])
		rest := strings.TrimSpace(text[i+len(attr):])
		if before && strings.HasPrefix(rest, "=") {
			at = len(text) - len(rest) + 1
			break
		}
		from = i + len(attr)
	}
	if at < 0 {
		return out
	}
	value := strings.TrimSpace(text[at:])
	if strings.HasPrefix(value, "[") {
		end := strings.Index(value, "]")
		if end < 0 {
			return out
		}
		value = value[1:end]
	} else if comma := strings.IndexAny(value, ",)"); comma >= 0 {
		value = value[:comma]
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if i := strings.Index(part, "::"); i >= 0 {
			part = strings.TrimSpace(part[:i])
		}
		if dot := strings.LastIndex(part, "."); dot >= 0 {
			part = part[dot+1:]
		}
		if part != "" {
			out[part] = struct{}{}
		}
	}
	return out
}

func hasAnyInjectPrimaryConstructor(file *scanner.File, idx uint32) bool {
	ctor, ok := file.FlatFindChild(idx, "primary_constructor")
	if !ok || ctor == 0 {
		return false
	}
	return hasAnnotationFlat(file, ctor, "Inject")
}

// typeIdentifiersIn returns every simple type name mentioned in type text such as `Lazy<Set<pkg.Foo>>`.
func typeIdentifiersIn(typeText string) []string {
	var names []string
	i := 0
	for i < len(typeText) {
		if !isIdentStartByte(typeText[i]) {
			i++
			continue
		}
		j := i
		for j < len(typeText) && isIdentByte(typeText[j]) {
			j++
		}
		// A package prefix such as `pkg.` is followed by another name; keep only the last segment of a dotted path.
		if j >= len(typeText) || typeText[j] != '.' {
			names = append(names, typeText[i:j])
		}
		i = j
	}
	return names
}

// classParameterTypeTexts returns the declared type text of each parameter of a primary constructor.
func classParameterTypeTexts(file *scanner.File, ctor uint32) []string {
	var texts []string
	for child := file.FlatFirstChild(ctor); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "class_parameter" {
			texts = append(texts, extractParameterTypeNameFlat(file, child))
		}
	}
	return texts
}

// diIsMultibindingContribution reports a binding that joins a Set or Map multibinding.
func diIsMultibindingContribution(file *scanner.File, fn uint32) bool {
	for _, ann := range []string{"IntoSet", "IntoMap", "ElementsIntoSet", "Multibinds", "BindsOptionalOf"} {
		if hasAnnotationFlat(file, fn, ann) {
			return true
		}
	}
	return false
}
