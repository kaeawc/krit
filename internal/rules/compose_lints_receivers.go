package rules

import (
	"bytes"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// composeUnstableReceiverMessage is compose-lint-checks 1.6.1's ComposeUnstableReceiver message.
const composeUnstableReceiverMessage = "Instance composable functions on non-stable classes will always be recomposed. " +
	"If possible, make the receiver type stable or refactor this function if that isn't possible. See " +
	"https://slackhq.github.io/compose-lints/rules/#unstable-receivers for more information."

// ComposeUnstableReceiverRule ports compose-lints' ComposeUnstableReceiver: a Unit composable whose extension
// receiver, or whose containing class, is not stable. It runs over the cross-file index to classify project
// classes; library types are classified from compose-lints' known stable constructs and the Compose scope tables.
type ComposeUnstableReceiverRule struct {
	BaseRule
}

// composeLintsStablePrimitives are the Kotlin types that map to JVM primitives (stable unless nullable).
var composeLintsStablePrimitives = map[string]bool{"Int": true, "Long": true, "Short": true, "Byte": true,
	"Float": true, "Double": true, "Boolean": true, "Char": true}

// composeLintsKnownStableTypes are compose-lints' KnownStableConstructs, by FQN.
var composeLintsKnownStableTypes = map[string]bool{
	"kotlin.Pair": true, "kotlin.Triple": true, "kotlin.Comparator": true, "java.util.Comparator": true,
	"kotlin.Result": true, "kotlin.ranges.ClosedRange": true, "kotlin.ranges.ClosedFloatingPointRange": true,
	"kotlin.String": true, "java.lang.String": true,
	"com.google.common.collect.ImmutableList": true, "com.google.common.collect.ImmutableEnumMap": true,
	"com.google.common.collect.ImmutableMap": true, "com.google.common.collect.ImmutableEnumSet": true,
	"com.google.common.collect.ImmutableSet":            true,
	"kotlinx.collections.immutable.ImmutableCollection": true, "kotlinx.collections.immutable.ImmutableList": true,
	"kotlinx.collections.immutable.ImmutableSet": true, "kotlinx.collections.immutable.ImmutableMap": true,
	"kotlinx.collections.immutable.PersistentCollection": true, "kotlinx.collections.immutable.PersistentList": true,
	"kotlinx.collections.immutable.PersistentSet": true, "kotlinx.collections.immutable.PersistentMap": true,
	"dagger.Lazy": true, "kotlin.coroutines.EmptyCoroutineContext": true,
}

// composeLintsKnownUnstableTypes are default-imported Kotlin types whose JVM classes have no stable supertype.
var composeLintsKnownUnstableTypes = map[string]bool{
	"kotlin.Any": true, "kotlin.collections.List": true, "kotlin.collections.MutableList": true,
	"kotlin.collections.Set": true, "kotlin.collections.MutableSet": true, "kotlin.collections.Map": true,
	"kotlin.collections.MutableMap": true, "kotlin.collections.Collection": true,
	"kotlin.collections.MutableCollection": true, "kotlin.collections.Iterable": true,
	"kotlin.collections.ArrayList": true, "kotlin.collections.HashMap": true, "kotlin.collections.HashSet": true,
}

// composeLintsStability is the verdict of compose-lints' isStable: stable, unstable, or not provable from source.
type composeLintsStability int8

const (
	composeLintsStabilityUnknown composeLintsStability = iota
	composeLintsStable
	composeLintsUnstable
)

// composeLintsClasses indexes the project's class, interface, enum and object declarations by FQN.
type composeLintsClasses struct {
	decls     map[string]composeLintsAnnotationClass
	aliases   composeLintsTypeAliases
	memo      map[string]composeLintsStability
	wildcards map[*scanner.File][]string
	pkgs      map[*scanner.File]string
}

func newComposeLintsClasses(files []*scanner.File) *composeLintsClasses {
	c := &composeLintsClasses{decls: map[string]composeLintsAnnotationClass{}, aliases: newComposeLintsTypeAliases(files),
		memo: map[string]composeLintsStability{}, wildcards: map[*scanner.File][]string{}, pkgs: map[*scanner.File]string{}}
	for _, file := range files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin {
			continue
		}
		pkg := filePackageName(file)
		c.pkgs[file] = pkg
		var walk func(parent uint32, prefix string)
		walk = func(parent uint32, prefix string) {
			for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
				switch file.FlatType(child) {
				case "class_declaration", "object_declaration":
					fqn := composeLintsJoin(prefix, acrDeclarationName(file, child))
					c.decls[fqn] = composeLintsAnnotationClass{file: file, decl: child}
					if body, ok := file.FlatFindChild(child, "class_body"); ok {
						walk(body, fqn)
					} else if body, ok := file.FlatFindChild(child, "enum_class_body"); ok {
						walk(body, fqn)
					}
				}
			}
		}
		walk(0, pkg)
	}
	return c
}

// resolve returns the FQN a type name refers to in file, among project classes and imports; for a name the
// project does not declare it returns the import target, or kotlin.<name> / kotlin.collections.<name> for
// default-imported names.
func (c *composeLintsClasses) resolve(file *scanner.File, name string) string {
	head, rest, qualified := strings.Cut(name, ".")
	facts := fileFactsCache().Imports(file)
	if facts != nil {
		if fqn, ok := facts.Aliases[head]; ok {
			if qualified {
				return fqn + "." + rest
			}
			return fqn
		}
	}
	pkg, ok := c.pkgs[file]
	if !ok {
		pkg = filePackageName(file)
		c.pkgs[file] = pkg
	}
	if candidate := composeLintsJoin(pkg, name); c.decls[candidate].file != nil || c.aliases[candidate].file != nil {
		return candidate
	}
	if c.decls[name].file != nil {
		return name
	}
	if !qualified {
		wildcards, ok := c.wildcards[file]
		if !ok {
			wildcards = composeLintsWildcardPackages(file)
			c.wildcards[file] = wildcards
		}
		for _, pkg := range wildcards {
			candidate := pkg + "." + name
			if c.decls[candidate].file != nil || c.aliases[candidate].file != nil {
				return candidate
			}
		}
		// Library types krit knows by FQN: the Compose stability annotations and the stability tables' types.
		for _, pkg := range wildcards {
			candidate := pkg + "." + name
			if composeLintsRuntimeStabilityAnnotations[candidate] || composeLintsStableLibraryTypes[candidate] ||
				composeLintsUnstableLibraryTypes[candidate] {
				return candidate
			}
		}
		if composeLintsKnownStableTypes["kotlin."+name] || composeLintsKnownUnstableTypes["kotlin."+name] {
			return "kotlin." + name
		}
		if composeLintsKnownUnstableTypes["kotlin.collections."+name] {
			return "kotlin.collections." + name
		}
	}
	return name
}

// typeStability classifies a receiver type node.
func (c *composeLintsClasses) typeStability(file *scanner.File, t uint32, typeParams map[string]bool) composeLintsStability {
	nullable := false
	for t != 0 {
		switch file.FlatType(t) {
		case "function_type":
			return composeLintsStable
		case "nullable_type":
			nullable = true
			t = file.FlatNamedChild(t, 0)
		case "parenthesized_type":
			t = file.FlatNamedChild(t, 0)
		case "user_type":
			name := composeLintsTypeText(file, t)
			if typeParams[name] {
				return composeLintsUnstable // a type parameter resolves to no class
			}
			if composeLintsStablePrimitives[name] {
				if nullable {
					return composeLintsUnstable // boxed: java.lang.Integer and friends have no stable supertype
				}
				return composeLintsStable
			}
			fqn := c.resolve(file, name)
			if alias, ok := c.aliases[fqn]; ok {
				file, t = alias.file, composeLintsTypeAliasTarget(alias.file, alias.decl)
				continue
			}
			return c.classStability(fqn, 0)
		default:
			return composeLintsStabilityUnknown
		}
	}
	return composeLintsStabilityUnknown
}

// classStability mirrors compose-lints' isStable over a class and its supertypes.
func (c *composeLintsClasses) classStability(fqn string, depth int) composeLintsStability {
	switch {
	case fqn == "String" || composeLintsKnownStableTypes[fqn] || strings.HasPrefix(fqn, "kotlin.Function") ||
		strings.HasPrefix(fqn, "kotlin.jvm.functions."):
		return composeLintsStable
	case composeLintsKnownUnstableTypes[fqn]:
		return composeLintsUnstable
	case composeLintsStableLibraryTypes[fqn]:
		return composeLintsStable
	case composeLintsUnstableLibraryTypes[fqn]:
		return composeLintsUnstable
	}
	decl, ok := c.decls[fqn]
	if !ok || depth > 8 {
		return composeLintsStabilityUnknown
	}
	if v, ok := c.memo[fqn]; ok {
		return v
	}
	c.memo[fqn] = composeLintsUnstable // break cycles
	v := c.declStability(decl.file, decl.decl, depth)
	c.memo[fqn] = v
	return v
}

func (c *composeLintsClasses) declStability(file *scanner.File, decl uint32, depth int) composeLintsStability {
	if file.FlatHasChildOfType(decl, "enum") || composeLintsIsFunInterface(file, decl) {
		return composeLintsStable
	}
	stable, unresolved := false, false
	composeLintsEachAnnotation(file, decl, func(a uint32) {
		if stable {
			return
		}
		text := composeLintsAnnotationTypeText(file, a)
		fqn := c.resolve(file, text)
		switch fqn {
		case acrRuntimePackage + ".Stable", acrRuntimePackage + ".Immutable", "java.lang.FunctionalInterface",
			"FunctionalInterface": // java.lang is default-imported
			stable = true
		case "Stable", "Immutable":
			unresolved = true // from a star import krit cannot resolve
		}
		if marker, ok := c.decls[fqn]; ok && composeLintsHasRuntimeAnnotation(marker.file, marker.decl, "StableMarker") {
			stable = true
		}
	})
	if stable {
		return composeLintsStable
	}
	result := composeLintsUnstable
	if unresolved {
		result = composeLintsStabilityUnknown
	}
	for spec := file.FlatFirstChild(decl); spec != 0; spec = file.FlatNextSib(spec) {
		if file.FlatType(spec) != "delegation_specifier" {
			continue
		}
		// `Base()`, `Iface<T>` or `Iface by delegate`: the supertype is the first user_type.
		ut := file.FlatFirstChild(spec)
		if t := file.FlatType(ut); t == "constructor_invocation" || t == "explicit_delegation" {
			ut = file.FlatFirstChild(ut)
		}
		if ut == 0 || file.FlatType(ut) != "user_type" {
			result = composeLintsStabilityUnknown
			continue
		}
		switch c.classStability(c.resolve(file, composeLintsTypeText(file, ut)), depth+1) {
		case composeLintsStable:
			return composeLintsStable
		case composeLintsStabilityUnknown:
			result = composeLintsStabilityUnknown
		}
	}
	if acrFunctionHasModifier(file, decl, "value") || acrFunctionHasModifier(file, decl, "inline") {
		// As in compose-lints, a value class is as stable as its underlying type.
		switch c.valueClassUnderlyingStability(file, decl, depth) {
		case composeLintsStable:
			return composeLintsStable
		case composeLintsStabilityUnknown:
			result = composeLintsStabilityUnknown
		}
	}
	return result
}

// valueClassUnderlyingStability classifies a value class's single constructor property type; compose-lints assumes
// stable when there is none to read.
func (c *composeLintsClasses) valueClassUnderlyingStability(file *scanner.File, decl uint32, depth int) composeLintsStability {
	ctor, ok := file.FlatFindChild(decl, "primary_constructor")
	if !ok {
		return composeLintsStable
	}
	var param uint32
	count := 0
	file.FlatWalkNodes(ctor, "class_parameter", func(p uint32) {
		count++
		param = p
	})
	if count != 1 || depth > 8 {
		return composeLintsStable
	}
	t := composeLintsDeclaredTypeAny(file, param)
	if t == 0 {
		return composeLintsStable
	}
	return c.typeStability(file, t, nil)
}

// composeLintsIsFunInterface reports whether a class declaration is a Kotlin `fun interface`. tree-sitter keeps the
// `fun` keyword out of the tree, so it is read from the source between the modifiers and `interface`.
func composeLintsIsFunInterface(file *scanner.File, decl uint32) bool {
	iface, ok := file.FlatFindChild(decl, "interface")
	if !ok {
		return false
	}
	start := file.FlatStartByte(decl)
	if mods, ok := file.FlatFindChild(decl, "modifiers"); ok {
		start = file.FlatEndByte(mods)
	}
	end := file.FlatStartByte(iface)
	return start <= end && int(end) <= len(file.Content) && strings.TrimSpace(string(file.Content[start:end])) == "fun"
}

// composeLintsRuntimeStabilityAnnotations are the Compose stability annotations, by FQN.
var composeLintsRuntimeStabilityAnnotations = map[string]bool{acrRuntimePackage + ".Stable": true,
	acrRuntimePackage + ".Immutable": true, acrRuntimePackage + ".StableMarker": true}

// composeLintsWildcardPackages returns the packages a file star-imports, sorted, so lookups over them do not depend
// on map order.
func composeLintsWildcardPackages(file *scanner.File) []string {
	facts := fileFactsCache().Imports(file)
	if facts == nil || len(facts.Wildcards) == 0 {
		return nil
	}
	out := make([]string, 0, len(facts.Wildcards))
	for wildcard := range facts.Wildcards {
		out = append(out, strings.TrimSuffix(wildcard, ".*"))
	}
	sort.Strings(out)
	return out
}

func (r *ComposeUnstableReceiverRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	var classes *composeLintsClasses
	for _, file := range ctx.CodeIndex.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("Composable")) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) || acrFunctionReturnKind(file, fn) != acrReturnUnit {
				return
			}
			recv := composeLintsReceiverType(file, fn)
			owner := composeLintsContainingClass(file, fn)
			if recv == 0 && owner == 0 {
				return
			}
			if classes == nil {
				classes = newComposeLintsClasses(ctx.CodeIndex.Files)
			}
			if recv != 0 && classes.typeStability(file, recv, composeLintsTypeParameters(file, fn)) == composeLintsUnstable {
				ctx.Emit(r.Finding(file, file.FlatRow(recv)+1, file.FlatCol(recv)+1, composeUnstableReceiverMessage))
				return
			}
			if owner != 0 && classes.declStability(file, owner, 0) == composeLintsUnstable {
				if name := acrNameNode(file, fn); name != 0 {
					ctx.Emit(r.Finding(file, file.FlatRow(name)+1, file.FlatCol(name)+1, composeUnstableReceiverMessage))
				}
			}
		})
	}
}

// composeLintsContainingClass returns the class, interface or enum a member function belongs to (not an object or
// companion, whose members compose-lints treats as static-like), or 0.
func composeLintsContainingClass(file *scanner.File, fn uint32) uint32 {
	body, ok := file.FlatParent(fn)
	if !ok || (file.FlatType(body) != "class_body" && file.FlatType(body) != "enum_class_body") {
		return 0
	}
	decl, ok := file.FlatParent(body)
	if !ok || file.FlatType(decl) != "class_declaration" {
		return 0
	}
	return decl
}

// composeLintsTypeParameters returns the names of a function's type parameters.
func composeLintsTypeParameters(file *scanner.File, fn uint32) map[string]bool {
	out := map[string]bool{}
	if tps, ok := file.FlatFindChild(fn, "type_parameters"); ok {
		file.FlatWalkNodes(tps, "type_parameter", func(tp uint32) {
			if name := file.FlatChildTextOrEmpty(tp, "type_identifier"); name != "" {
				out[name] = true
			}
		})
	}
	return out
}
