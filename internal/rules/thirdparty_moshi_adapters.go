package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

const (
	fqcnMoshiJSONAdapter        = "com.squareup.moshi.JsonAdapter"
	fqcnMoshiJSONAdapterFactory = "com.squareup.moshi.JsonAdapter.Factory"
	fqcnMoshiJSONQualifier      = "com.squareup.moshi.JsonQualifier"
	fqcnAndroidxKeep            = "androidx.annotation.Keep"
)

// moshiDeclaredClass is a Kotlin or Java class declared in the project.
type moshiDeclaredClass struct {
	file       *scanner.File
	node       uint32
	pkg        string
	anns       []moshiAnnotation
	supertypes []string // as written, without type arguments
}

// moshiClassIndex maps the FQN of every class declared in the parsed files
// to its declaration. It is built on first use, only when an @AdaptedBy
// target has to be resolved.
type moshiClassIndex struct {
	byFQN map[string]*moshiDeclaredClass
}

func (a *moshiAnalyzer) projectClasses() *moshiClassIndex {
	if a.classes != nil {
		return a.classes
	}
	idx := &moshiClassIndex{byFQN: map[string]*moshiDeclaredClass{}}
	for _, file := range a.result.files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		pkg := moshiFilePackage(file)
		file.FlatWalkAllNodes(0, func(node uint32) {
			if !moshiIsTypeDeclaration(file, node) {
				return
			}
			fqn := moshiClassFQN(file, pkg, node)
			if _, dup := idx.byFQN[fqn]; dup {
				return
			}
			idx.byFQN[fqn] = &moshiDeclaredClass{
				file:       file,
				node:       node,
				pkg:        pkg,
				anns:       moshiAnnotationsOf(file, node),
				supertypes: moshiDeclaredSupertypes(file, node),
			}
		})
	}
	a.classes = idx
	return idx
}

// moshiIsTypeDeclaration reports whether node declares a named Kotlin or Java type.
func moshiIsTypeDeclaration(file *scanner.File, node uint32) bool {
	switch file.FlatType(node) {
	case "class_declaration", "object_declaration", "companion_object":
		return true
	case "interface_declaration", "enum_declaration", "annotation_type_declaration", "record_declaration":
		return file.Language == scanner.LangJava
	}
	return false
}

// moshiDeclaredSupertypes returns the supertypes of a Kotlin or Java type.
func moshiDeclaredSupertypes(file *scanner.File, node uint32) []string {
	if file.Language != scanner.LangJava {
		return moshiSupertypeNames(file, node)
	}
	var out []string
	file.FlatForEachChild(node, func(child uint32) {
		switch file.FlatType(child) {
		case "superclass", "super_interfaces", "extends_interfaces":
		default:
			return
		}
		var collect func(n uint32)
		collect = func(n uint32) {
			switch file.FlatType(n) {
			case "type_identifier", "scoped_type_identifier":
				out = append(out, strings.Join(strings.Fields(file.FlatNodeText(n)), ""))
			case "generic_type":
				if first := file.FlatNamedChild(n, 0); first != 0 {
					collect(first)
				}
			default:
				file.FlatForEachNamedChild(n, collect)
			}
		}
		file.FlatForEachNamedChild(child, collect)
	})
	return out
}

// resolve returns the FQN of the project class a type reference names from
// within the class at node: an import, a class nested in an enclosing class,
// a class in the file's package, or a class reached by a star import.
func (idx *moshiClassIndex) resolve(file *scanner.File, pkg string, node uint32, name string) string {
	head, rest, _ := strings.Cut(name, ".")
	imports := lintImportsOf(file)
	if imported, ok := imports.bySimple[head]; ok {
		fqn := imported
		if rest != "" {
			fqn += "." + rest
		}
		if idx.byFQN[fqn] != nil {
			return fqn
		}
		return ""
	}
	enclosing := moshiEnclosingClassNames(file, node)
	if moshiIsTypeDeclaration(file, node) && len(enclosing) > 0 {
		enclosing = enclosing[:len(enclosing)-1]
	}
	for i := len(enclosing); i >= 0; i-- {
		parts := append([]string(nil), enclosing[:i]...)
		if pkg != "" {
			parts = append([]string{pkg}, parts...)
		}
		if fqn := strings.Join(append(parts, name), "."); idx.byFQN[fqn] != nil {
			return fqn
		}
	}
	if idx.byFQN[name] != nil {
		return name
	}
	for _, star := range imports.stars {
		if fqn := star + "." + name; idx.byFQN[fqn] != nil {
			return fqn
		}
	}
	return ""
}

// moshiTrivialSupertype reports whether a supertype reference is the root
// type, which is never an adapter.
func moshiTrivialSupertype(name string) bool {
	switch name {
	case "Any", "kotlin.Any", "Object", "java.lang.Object":
		return true
	}
	return false
}

// isAdapter reports whether the class extends JsonAdapter or implements
// JsonAdapter.Factory, directly or through project classes. known is false
// when a supertype outside the project leaves the answer open.
func (idx *moshiClassIndex) isAdapter(c *moshiDeclaredClass, seen map[*moshiDeclaredClass]bool) (adapter, known bool) {
	if seen[c] {
		return false, true
	}
	seen[c] = true
	known = true
	for _, super := range c.supertypes {
		if moshiRefIs(c.file, super, fqcnMoshiJSONAdapter) || moshiRefIs(c.file, super, fqcnMoshiJSONAdapterFactory) {
			return true, true
		}
		if moshiTrivialSupertype(super) {
			continue
		}
		parent := idx.byFQN[idx.resolve(c.file, c.pkg, c.node, super)]
		if parent == nil {
			known = false
			continue
		}
		isAdapter, parentKnown := idx.isAdapter(parent, seen)
		if isAdapter {
			return true, true
		}
		known = known && parentKnown
	}
	return false, known
}

// validateAdaptedBy ports MoshiUsageDetector.validateAdaptedByAnnotation:
// the @AdaptedBy adapter must be a JsonAdapter or JsonAdapter.Factory, and
// must be @Keep. Adapters outside the project are not checked.
func (a *moshiAnalyzer) validateAdaptedBy(file *scanner.File, scope uint32, ann moshiAnnotation) {
	expr := moshiAnnotationArg(file, ann, "adapter", 0)
	if expr == 0 {
		return
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
	name, ok := strings.CutSuffix(text, "::class")
	if !ok || name == "" || strings.ContainsAny(name, "<>?") {
		return
	}
	idx := a.projectClasses()
	target := idx.byFQN[idx.resolve(file, moshiFilePackage(file), scope, name)]
	if target == nil {
		return
	}
	adapter, known := idx.isAdapter(target, map[*moshiDeclaredClass]bool{})
	switch {
	case !known:
	case !adapter:
		a.reportIssue("MoshiUsageAdaptedByRequiresAdapter", file, expr)
	case moshiFindAnnotation(target.file, target.anns, fqcnAndroidxKeep) == nil:
		a.reportIssue("MoshiUsageAdaptedByRequiresKeep", file, expr)
	}
}

// moshiIsAnnotationType reports whether c declares an annotation class.
func moshiIsAnnotationType(c moshiClass) bool {
	if c.file.Language == scanner.LangJava {
		return c.file.FlatType(c.node) == "annotation_type_declaration"
	}
	return moshiHasModifier(c, "annotation")
}

// moshiRefIsKotlinDefault reports whether a Kotlin type reference names fqn,
// counting a bare simple name that Kotlin's default imports (fqn's package)
// provide when nothing in the file may shadow it.
func moshiRefIsKotlinDefault(file *scanner.File, text, fqn string) bool {
	if moshiRefIs(file, text, fqn) {
		return true
	}
	dot := strings.LastIndex(fqn, ".")
	if file.Language != scanner.LangKotlin || text != fqn[dot+1:] {
		return false
	}
	imports := lintImportsOf(file)
	if _, ok := imports.bySimple[text]; ok || lintSameFileClass(file, text) != 0 {
		return false
	}
	for _, star := range imports.stars {
		if star != "com.squareup.moshi" && !strings.HasPrefix(star, "dev.zacsweers.moshix") && star != fqn[:dot] &&
			!strings.HasPrefix(star, "java.lang.annotation") && !strings.HasPrefix(star, "kotlin.annotation") {
			return false
		}
	}
	return true
}

// moshiLastIdentifier returns the last simple name of a reference such as
// `AnnotationRetention.RUNTIME` or `RUNTIME`.
func moshiLastIdentifier(file *scanner.File, expr uint32) string {
	text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		text = text[dot+1:]
	}
	return text
}

// moshiAnnotationValues returns every value expression of an annotation's
// arguments, flattening Kotlin varargs and array literals and Java arrays.
func moshiAnnotationValues(file *scanner.File, ann moshiAnnotation) []uint32 {
	if ann.args == 0 {
		return nil
	}
	var out []uint32
	var collect func(n uint32)
	collect = func(n uint32) {
		switch file.FlatType(n) {
		case "value_argument":
			if e := flatValueArgumentExpression(file, n); e != 0 {
				collect(e)
			}
		case "element_value_pair":
			if v := file.FlatNamedChild(n, file.FlatNamedChildCount(n)-1); v != 0 {
				collect(v)
			}
		case "collection_literal", "element_value_array_initializer":
			file.FlatForEachNamedChild(n, collect)
		default:
			out = append(out, n)
		}
	}
	file.FlatForEachNamedChild(ann.args, collect)
	return out
}

// checkJSONQualifier ports MoshiUsageDetector.checkJsonQualifierAnnotation for
// Kotlin and Java annotation classes annotated with @JsonQualifier.
func (a *moshiAnalyzer) checkJSONQualifier(c moshiClass) {
	file := c.file
	if moshiFindAnnotation(file, c.anns, fqcnMoshiJSONQualifier) == nil {
		return
	}
	isKotlin := file.Language == scanner.LangKotlin
	retentionFQN, targetFQN := "java.lang.annotation.Retention", "java.lang.annotation.Target"
	if isKotlin {
		retentionFQN, targetFQN = "kotlin.annotation.Retention", "kotlin.annotation.Target"
	}
	find := func(fqn string) *moshiAnnotation {
		for i := range c.anns {
			if (isKotlin && moshiRefIsKotlinDefault(file, c.anns[i].name, fqn)) || (!isKotlin && moshiRefIs(file, c.anns[i].name, fqn)) {
				return &c.anns[i]
			}
		}
		return nil
	}

	if retention := find(retentionFQN); retention != nil {
		// An empty Kotlin @Retention keeps the RUNTIME default.
		if values := moshiAnnotationValues(file, *retention); len(values) > 0 && moshiLastIdentifier(file, values[0]) != "RUNTIME" {
			a.reportIssue("MoshiUsageQualifierRetention", file, retention.node)
		}
	} else if !isKotlin {
		// Java's default retention is CLASS; Kotlin's is RUNTIME.
		a.reportIssue("MoshiUsageQualifierRetention", file, c.nameNode)
	}

	// A missing @Target is fine: the default includes FIELD in both languages.
	if target := find(targetFQN); target != nil {
		values := moshiAnnotationValues(file, *target)
		hasField := false
		for _, v := range values {
			if moshiLastIdentifier(file, v) == "FIELD" {
				hasField = true
				break
			}
		}
		if len(values) > 0 && !hasField {
			a.reportIssue("MoshiUsageQualifierTarget", file, target.node)
		}
	}
}
