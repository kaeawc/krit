package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

const (
	fqcnMoshixTypeLabel     = "dev.zacsweers.moshix.sealed.annotations.TypeLabel"
	fqcnMoshixDefaultObject = "dev.zacsweers.moshix.sealed.annotations.DefaultObject"
)

// moshiSealedIndex records the project's Kotlin classes that sealed
// subtypes may extend, by FQN.
type moshiSealedIndex struct {
	// parents are sealed types with @JsonClass(generator = "sealed:...").
	parents map[string]bool
	// unknown are sealed types whose @JsonClass generator is not a literal;
	// subtypes of these abstain.
	unknown map[string]bool
	// classes maps each Kotlin class declared in the indexed files to its package.
	classes map[string]string
	// packages holds the packages of parents and unknown, whose files the
	// pass must visit even when they do not mention Moshi.
	packages map[string]bool
}

// buildMoshiSealedIndex indexes the classes declared in files that mention
// Moshi, then the classes of every Kotlin file in a package that declares a
// moshi-sealed type, so subtype supertypes can be resolved.
func buildMoshiSealedIndex(files []*scanner.File) *moshiSealedIndex {
	idx := &moshiSealedIndex{parents: map[string]bool{}, unknown: map[string]bool{}, classes: map[string]string{}, packages: map[string]bool{}}
	for _, file := range files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin || !moshiFileMayMatch(file.Content) {
			continue
		}
		pkg := moshiFilePackage(file)
		file.FlatWalkAllNodes(0, func(node uint32) {
			c, ok := moshiClassDecl(file, node)
			if !ok {
				return
			}
			fqn := moshiClassFQN(file, pkg, node)
			idx.classes[fqn] = pkg
			if c.isEnum || !moshiHasModifier(c, "sealed") {
				return
			}
			jsonClass := moshiFindAnnotation(file, c.anns, fqcnMoshiJSONClass)
			if jsonClass == nil {
				return
			}
			generator, known := moshiAnnotationStringArg(file, *jsonClass, "generator", 1, "")
			switch {
			case !known:
				idx.unknown[fqn] = true
				idx.packages[pkg] = true
			case strings.HasPrefix(generator, "sealed:"):
				idx.parents[fqn] = true
				idx.packages[pkg] = true
			}
		})
	}
	for _, file := range files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin || moshiFileMayMatch(file.Content) {
			continue
		}
		pkg := moshiFilePackage(file)
		if !idx.packages[pkg] {
			continue
		}
		file.FlatWalkAllNodes(0, func(node uint32) {
			if _, ok := moshiClassDecl(file, node); ok {
				idx.classes[moshiClassFQN(file, pkg, node)] = pkg
			}
		})
	}
	return idx
}

// moshiClassFQN returns the FQN of the Kotlin class declared at node: its
// package and the names of its enclosing classes.
func moshiClassFQN(file *scanner.File, pkg string, node uint32) string {
	names := moshiEnclosingClassNames(file, node)
	if pkg != "" {
		names = append([]string{pkg}, names...)
	}
	return strings.Join(names, ".")
}

// moshiEnclosingClassNames returns the names of the classes enclosing node,
// outermost first, ending with node's own name when node is a class.
func moshiEnclosingClassNames(file *scanner.File, node uint32) []string {
	var names []string
	for cur := node; cur != 0; {
		if moshiIsTypeDeclaration(file, cur) {
			nameType := "type_identifier"
			if file.Language == scanner.LangJava {
				nameType = "identifier"
			}
			name, ok := file.FlatFindChild(cur, nameType)
			text := "Companion"
			if ok {
				text = file.FlatNodeText(name)
			}
			names = append([]string{text}, names...)
		}
		parent, ok := file.FlatParent(cur)
		if !ok {
			break
		}
		cur = parent
	}
	return names
}

// moshiSupertypeNames returns the supertype references of a Kotlin class, as
// written and without type arguments (`Outer.Base` for `Outer.Base<T>()`).
func moshiSupertypeNames(file *scanner.File, node uint32) []string {
	var out []string
	file.FlatForEachChild(node, func(spec uint32) {
		if file.FlatType(spec) != "delegation_specifier" {
			return
		}
		userType := moshiFirstDescendant(file, spec, "user_type")
		if userType == 0 {
			return
		}
		var parts []string
		file.FlatForEachChild(userType, func(part uint32) {
			if file.FlatType(part) == "type_identifier" {
				parts = append(parts, file.FlatNodeText(part))
			}
		})
		if len(parts) > 0 {
			out = append(out, strings.Join(parts, "."))
		}
	})
	return out
}

func moshiFirstDescendant(file *scanner.File, node uint32, nodeType string) uint32 {
	var found uint32
	file.FlatWalkNodes(node, nodeType, func(n uint32) {
		if found == 0 {
			found = n
		}
	})
	return found
}

// resolveSupertype returns the FQN a supertype reference names, scoped from
// the class at node: an import, a class nested in an enclosing class, or a
// class in the file's package. It returns "" when the name resolves to none
// of the indexed classes, which leaves it outside the package.
func (s *moshiSealedIndex) resolveSupertype(file *scanner.File, pkg string, node uint32, name string) string {
	head, rest, _ := strings.Cut(name, ".")
	if imported, ok := lintImportsOf(file).bySimple[head]; ok {
		if rest != "" {
			return imported + "." + rest
		}
		return imported
	}
	prefix := func(parts []string) string {
		all := parts
		if pkg != "" {
			all = append([]string{pkg}, parts...)
		}
		return strings.Join(append(all, name), ".")
	}
	enclosing := moshiEnclosingClassNames(file, node)
	// The class itself is the last entry; its own nested classes are not in scope for its supertypes.
	if len(enclosing) > 0 {
		enclosing = enclosing[:len(enclosing)-1]
	}
	for i := len(enclosing); i >= 0; i-- {
		if fqn := prefix(enclosing[:i]); s.hasClass(fqn) {
			return fqn
		}
	}
	return ""
}

func (s *moshiSealedIndex) hasClass(fqn string) bool {
	_, ok := s.classes[fqn]
	return ok
}

// checkSealedClass ports MoshiUsageDetector.checkSealedClass.
func (a *moshiAnalyzer) checkSealedClass(c moshiClass) {
	file := c.file
	typeLabel := moshiFindAnnotation(file, c.anns, fqcnMoshixTypeLabel)
	defaultObject := moshiFindAnnotation(file, c.anns, fqcnMoshixDefaultObject)
	if typeLabel != nil && defaultObject != nil {
		a.reportIssue("MoshiUsageDoubleTypeLabel", file, typeLabel.node)
		a.reportIssue("MoshiUsageDoubleTypeLabel", file, defaultObject.node)
		return
	}
	if typeLabel != nil && c.typeParams != 0 {
		a.reportIssue("MoshiUsageGenericSealedSubtype", file, c.typeParams)
	}
	annotated := typeLabel != nil || defaultObject != nil

	// Supertypes in other packages are left to the compiler; only a sealed
	// supertype in this package with a moshi-sealed generator counts.
	pkg := moshiFilePackage(file)
	unknownParent := false
	for _, name := range moshiSupertypeNames(file, c.node) {
		fqn := a.sealed.resolveSupertype(file, pkg, c.node, name)
		if parentPkg, ok := a.sealed.classes[fqn]; !ok || parentPkg != pkg {
			continue
		}
		switch {
		case a.sealed.parents[fqn]:
			if !annotated {
				a.reportIssue("MoshiUsageMissingTypeLabel", file, c.nameNode)
			}
			return
		case a.sealed.unknown[fqn]:
			unknownParent = true
		}
	}
	if unknownParent {
		return
	}
	if typeLabel != nil {
		a.reportIssue("MoshiUsageInappropriateTypeLabel", file, typeLabel.node)
	}
	if defaultObject != nil {
		a.reportIssue("MoshiUsageInappropriateTypeLabel", file, defaultObject.node)
	}
}
