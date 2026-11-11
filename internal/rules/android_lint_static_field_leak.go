package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const staticFieldLeakDirectMessage = "Do not place Android context classes in static fields; this is a memory leak"

// staticFieldLeakRoots are the types Lint treats as holding a Context, so a static field of them leaks it.
var staticFieldLeakRoots = []string{
	"android.content.Context", "android.view.View", "android.app.Fragment",
	"androidx.lifecycle.Lifecycle", "android.arch.lifecycle.Lifecycle",
}

// staticFieldLeakClassBases are the base classes whose non-static inner and anonymous subclasses leak the outer instance.
var staticFieldLeakClassBases = map[string]string{
	"android.os.AsyncTask":              "AsyncTask",
	"android.content.Loader":            "Loader",
	"android.support.v4.content.Loader": "Loader",
	"androidx.loader.content.Loader":    "Loader",
	"android.arch.lifecycle.ViewModel":  "ViewModel",
	"androidx.lifecycle.ViewModel":      "ViewModel",
}

// StaticFieldLeakRule mirrors Lint's StaticFieldLeak for Kotlin: Context-holding types in static fields, and inner or
// anonymous AsyncTask, Loader and ViewModel subclasses.
type StaticFieldLeakRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: field types come from imports and the class hierarchy.
func (r *StaticFieldLeakRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StaticFieldLeakRule) check(ctx *api.Context) {
	file := ctx.File
	if file.Language != scanner.LangKotlin {
		return
	}
	switch file.FlatType(ctx.Idx) {
	case "property_declaration":
		r.checkStaticProperty(ctx)
	case "object_declaration":
		r.checkObjectInstance(ctx)
	case "class_declaration":
		if file.FlatHasModifier(ctx.Idx, "inner") {
			r.checkLeakyClassBase(ctx, ctx.Idx, true)
		}
	case "object_literal":
		if !staticFieldLeakInStaticFunction(file, ctx.Idx) {
			r.checkLeakyClassBase(ctx, ctx.Idx, false)
		}
	}
}

func (r *StaticFieldLeakRule) emit(ctx *api.Context, node uint32, msg string) {
	ctx.EmitAt(ctx.File.FlatRow(node)+1, ctx.File.FlatCol(node)+1, msg)
}

func (r *StaticFieldLeakRule) checkStaticProperty(ctx *api.Context) {
	file := ctx.File
	if !staticFieldLeakIsStaticProperty(file, ctx.Idx) || !staticFieldLeakHasBackingField(file, ctx.Idx) ||
		staticFieldLeakIsAppContext(file, ctx.Idx) {
		return
	}
	typeName := staticFieldLeakPropertyType(ctx, ctx.Idx)
	if typeName == "" {
		return
	}
	if staticFieldLeakIsLeakType(ctx, typeName) {
		r.emit(ctx, ctx.Idx, staticFieldLeakDirectMessage)
		return
	}
	if field, pointsTo := staticFieldLeakSameFileClassField(ctx, typeName, 0); field != "" {
		r.emit(ctx, ctx.Idx, staticFieldLeakIndirectMessage(androidSimpleName(typeName), field, pointsTo))
	}
}

// checkObjectInstance reports the static INSTANCE field of an object that holds a Context-typed property.
func (r *StaticFieldLeakRule) checkObjectInstance(ctx *api.Context) {
	file := ctx.File
	name := extractIdentifierFlat(file, ctx.Idx)
	if name == "" {
		return
	}
	if field, pointsTo := staticFieldLeakClassField(ctx, ctx.Idx, 0); field != "" {
		r.emit(ctx, ctx.Idx, staticFieldLeakIndirectMessage(name, field, pointsTo))
	}
}

func (r *StaticFieldLeakRule) checkLeakyClassBase(ctx *api.Context, node uint32, named bool) {
	file := ctx.File
	for _, super := range androidDirectSupertypesFlat(file, node) {
		display := staticFieldLeakClassBase(ctx, super.name)
		if display == "" {
			continue
		}
		if !named {
			r.emit(ctx, node, "This `"+display+"` class should be static or leaks might occur (anonymous "+staticFieldLeakBaseFQN(ctx, super.name)+")")
			return
		}
		at := handlerClassNameNode(file, node)
		qualified := qualifySourceName(sourcePackageName(file), handlerClassQualifiedName(file, node))
		r.emit(ctx, at, "This `"+display+"` class should be static or leaks might occur ("+qualified+")")
		return
	}
}

func staticFieldLeakIndirectMessage(class, field, pointsTo string) string {
	return "Do not place Android context classes in static fields (static reference to `" + class + "` which has field `" +
		field + "` pointing to `" + pointsTo + "`); this is a memory leak"
}

// staticFieldLeakIsStaticProperty reports whether prop compiles to a static field: top-level, or in an object or companion.
func staticFieldLeakIsStaticProperty(file *scanner.File, prop uint32) bool {
	parent, ok := file.FlatParent(prop)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "source_file":
		return true
	case "class_body":
		owner, ok := file.FlatParent(parent)
		return ok && (file.FlatType(owner) == "object_declaration" || file.FlatType(owner) == "companion_object")
	}
	return false
}

// staticFieldLeakHasBackingField is false for delegated properties and getter-only computed properties.
func staticFieldLeakHasBackingField(file *scanner.File, prop uint32) bool {
	if _, delegated := file.FlatFindChild(prop, "property_delegate"); delegated {
		return false
	}
	if propertyInitializerExpression(file, prop) != 0 || file.FlatHasModifier(prop, "lateinit") {
		return true
	}
	_, getter := file.FlatFindChild(prop, "getter")
	_, setter := file.FlatFindChild(prop, "setter")
	return !getter && !setter
}

// staticFieldLeakIsAppContext mirrors Lint's name and annotation heuristics for application contexts.
func staticFieldLeakIsAppContext(file *scanner.File, decl uint32) bool {
	name := strings.ToLower(propertyDeclarationName(file, decl))
	if name == "" {
		name = strings.ToLower(extractIdentifierFlat(file, decl))
	}
	if strings.Contains(name, "appcontext") || strings.Contains(name, "application") {
		return true
	}
	if mods, ok := file.FlatFindChild(decl, "modifiers"); ok {
		return strings.Contains(file.FlatNodeText(mods), "ApplicationContext")
	}
	return false
}

// staticFieldLeakDeclaredTypeName returns the written type name of a variable or class parameter, without nullability.
func staticFieldLeakDeclaredTypeName(file *scanner.File, decl uint32) string {
	root := decl
	if varDecl, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
		root = varDecl
	}
	for child := file.FlatFirstChild(root); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type":
			return androidUserTypeNameFlat(file, child)
		case "nullable_type":
			if ut, ok := file.FlatFindChild(child, "user_type"); ok {
				return androidUserTypeNameFlat(file, ut)
			}
		}
	}
	return ""
}

// staticFieldLeakMemberType returns the written type of a constructor property or the written or inferred type of a body property.
func staticFieldLeakMemberType(ctx *api.Context, member uint32) string {
	if ctx.File.FlatType(member) == "property_declaration" {
		return staticFieldLeakPropertyType(ctx, member)
	}
	return staticFieldLeakDeclaredTypeName(ctx.File, member)
}

// staticFieldLeakPropertyType returns the written or inferred type name of a property.
func staticFieldLeakPropertyType(ctx *api.Context, prop uint32) string {
	if name := staticFieldLeakDeclaredTypeName(ctx.File, prop); name != "" {
		return name
	}
	init := propertyInitializerExpression(ctx.File, prop)
	if init == 0 {
		return ""
	}
	typ, ok := semantics.ExpressionType(ctx, init)
	if !ok || typ.Type == nil {
		return ""
	}
	return typ.Type.FQN
}

// staticFieldLeakFQN resolves a written type name to its fully qualified name, or "" when unknown.
func staticFieldLeakFQN(ctx *api.Context, typeName string) string {
	if strings.Contains(typeName, ".") && strings.ToLower(typeName[:1]) == typeName[:1] {
		return typeName
	}
	if ctx.Resolver != nil {
		if fqn := ctx.Resolver.ResolveImport(typeName, ctx.File); fqn != "" {
			return fqn
		}
	}
	return staticFieldLeakImportedFQN(ctx.File, typeName)
}

// staticFieldLeakImportedFQN resolves a simple name through the file's explicit and star imports of framework packages.
func staticFieldLeakImportedFQN(file *scanner.File, simple string) string {
	var found string
	file.FlatWalkNodes(0, "import_header", func(idx uint32) {
		path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(idx)), "import"))
		if as := strings.Index(path, " as "); as >= 0 {
			return
		}
		switch {
		case strings.HasSuffix(path, "."+simple):
			found = path
		case strings.HasSuffix(path, ".*") && found == "":
			candidate := strings.TrimSuffix(path, "*") + simple
			if _, known := typeinfer.KnownClassHierarchy[candidate]; known || staticFieldLeakKnownRoot(candidate) {
				found = candidate
			}
		}
	})
	return found
}

func staticFieldLeakKnownRoot(fqn string) bool {
	if _, ok := staticFieldLeakClassBases[fqn]; ok {
		return true
	}
	return viewTagFQNMatchesAny(fqn, staticFieldLeakRoots)
}

// staticFieldLeakIsLeakType reports whether the type holds a Context: a Context, View, framework Fragment or
// Lifecycle subtype that is not an Application.
func staticFieldLeakIsLeakType(ctx *api.Context, typeName string) bool {
	fqn := staticFieldLeakFQN(ctx, typeName)
	if fqn == "" || strings.HasPrefix(fqn, "java.") || strings.HasPrefix(fqn, "kotlin.") {
		return false
	}
	if staticFieldLeakInherits(ctx, fqn, []string{"android.app.Application"}) {
		return false
	}
	return staticFieldLeakInherits(ctx, fqn, staticFieldLeakRoots)
}

// staticFieldLeakInherits walks known framework hierarchies and the resolver's class hierarchy transitively.
func staticFieldLeakInherits(ctx *api.Context, fqn string, targets []string) bool {
	seen := map[string]bool{}
	queue := []string{fqn}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == "" || seen[cur] {
			continue
		}
		seen[cur] = true
		if viewTagFQNMatchesAny(cur, targets) {
			return true
		}
		queue = append(queue, typeinfer.KnownClassHierarchy[cur]...)
		queue = append(queue, staticFieldLeakFrameworkSupers[cur]...)
		if ctx.Resolver != nil {
			if info := ctx.Resolver.ClassHierarchy(cur); info != nil {
				queue = append(queue, info.Supertypes...)
			}
		}
	}
	return false
}

// staticFieldLeakFrameworkSupers fills framework supertypes missing from the shared hierarchy table.
var staticFieldLeakFrameworkSupers = map[string][]string{
	"android.content.ContextWrapper":                    {"android.content.Context"},
	"androidx.fragment.app.FragmentActivity":            {"android.app.Activity"},
	"androidx.appcompat.app.AppCompatActivity":          {"androidx.fragment.app.FragmentActivity"},
	"androidx.activity.ComponentActivity":               {"android.app.Activity"},
	"androidx.core.app.ComponentActivity":               {"android.app.Activity"},
	"androidx.lifecycle.LifecycleRegistry":              {"androidx.lifecycle.Lifecycle"},
	"android.view.ContextThemeWrapper":                  {"android.content.ContextWrapper"},
	"android.app.ListActivity":                          {"android.app.Activity"},
	"android.app.IntentService":                         {"android.app.Service"},
	"android.view.ViewGroup":                            {"android.view.View"},
	"androidx.constraintlayout.widget.ConstraintLayout": {"android.view.ViewGroup"},
}

func staticFieldLeakClassBase(ctx *api.Context, typeName string) string {
	fqn := staticFieldLeakFQN(ctx, typeName)
	if display, ok := staticFieldLeakClassBases[fqn]; ok {
		return display
	}
	bases := make([]string, 0, len(staticFieldLeakClassBases))
	for base := range staticFieldLeakClassBases {
		bases = append(bases, base)
	}
	if fqn == "" || !staticFieldLeakInherits(ctx, fqn, bases) {
		return ""
	}
	for _, base := range bases {
		if staticFieldLeakInherits(ctx, fqn, []string{base}) {
			return staticFieldLeakClassBases[base]
		}
	}
	return ""
}

func staticFieldLeakBaseFQN(ctx *api.Context, typeName string) string {
	if fqn := staticFieldLeakFQN(ctx, typeName); fqn != "" {
		return fqn
	}
	return typeName
}

// staticFieldLeakInStaticFunction reports whether node sits in a top-level or @JvmStatic function, which has no outer instance.
func staticFieldLeakInStaticFunction(file *scanner.File, node uint32) bool {
	for parent, ok := file.FlatParent(node); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration":
			if mods, ok := file.FlatFindChild(parent, "modifiers"); ok && strings.Contains(file.FlatNodeText(mods), "@JvmStatic") {
				return true
			}
			if owner, ok := file.FlatParent(parent); ok && file.FlatType(owner) == "source_file" {
				return true
			}
		case "class_declaration", "object_declaration", "companion_object", "object_literal":
			return false
		case "property_declaration":
			if owner, ok := file.FlatParent(parent); ok && file.FlatType(owner) == "source_file" {
				return true
			}
		}
	}
	return false
}

// staticFieldLeakSameFileClassField finds, in a class declared in the same file, the first field that holds a Context.
func staticFieldLeakSameFileClassField(ctx *api.Context, typeName string, depth int) (field, pointsTo string) {
	simple := androidSimpleName(typeName)
	var decl uint32
	ctx.File.FlatWalkAllNodes(0, func(idx uint32) {
		if decl == 0 && ctx.File.FlatType(idx) == "class_declaration" && extractIdentifierFlat(ctx.File, idx) == simple {
			decl = idx
		}
	})
	if decl == 0 {
		return "", ""
	}
	return staticFieldLeakClassField(ctx, decl, depth)
}

// staticFieldLeakClassField returns the first backing-field property of a class or object that holds a Context,
// looking through same-file superclasses.
func staticFieldLeakClassField(ctx *api.Context, decl uint32, depth int) (field, pointsTo string) {
	file := ctx.File
	if depth > 3 {
		return "", ""
	}
	check := func(member uint32) bool {
		if staticFieldLeakIsAppContext(file, member) {
			return false
		}
		typeName := staticFieldLeakMemberType(ctx, member)
		if typeName == "" || !staticFieldLeakIsLeakType(ctx, typeName) {
			return false
		}
		field, pointsTo = extractIdentifierFlat(file, member), androidSimpleName(typeName)
		if field == "" {
			field = propertyDeclarationName(file, member)
		}
		return true
	}
	if ctor, ok := file.FlatFindChild(decl, "primary_constructor"); ok {
		for param := file.FlatFirstChild(ctor); param != 0; param = file.FlatNextSib(param) {
			if file.FlatType(param) == "class_parameter" && staticFieldLeakIsPropertyParam(file, param) && check(param) {
				return field, pointsTo
			}
		}
	}
	if body, ok := file.FlatFindChild(decl, "class_body"); ok {
		for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
			if file.FlatType(member) == "property_declaration" && staticFieldLeakHasBackingField(file, member) && check(member) {
				return field, pointsTo
			}
		}
	}
	for _, super := range androidDirectSupertypesFlat(file, decl) {
		if f, p := staticFieldLeakSameFileClassField(ctx, super.name, depth+1); f != "" {
			return f, p
		}
	}
	return "", ""
}

func staticFieldLeakIsPropertyParam(file *scanner.File, param uint32) bool {
	_, isProperty := file.FlatFindChild(param, "binding_pattern_kind")
	return isProperty
}
