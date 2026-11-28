package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Messages of compose-lint-checks 1.6.1's RedundantComposableDetector, as Lint reports them.
const (
	composeRedundantComposableMessage = "This declaration is annotated with @Composable but doesn't call any other " +
		"@Composable functions or read any @Composable properties (like a CompositionLocal's current), so it doesn't " +
		"use the composition and the @Composable annotation can be removed. See " +
		"https://slackhq.github.io/compose-lints/rules/#remove-unnecessary-composable-annotations for more information."
	composeReadOnlyComposableMessage = "All composable functions and properties used by this declaration are marked " +
		"@ReadOnlyComposable, so this declaration can be marked @ReadOnlyComposable too. See " +
		"https://slackhq.github.io/compose-lints/rules/#remove-unnecessary-composable-annotations for more information."
)

// ComposeRedundantComposableRule ports compose-lints' ComposeRedundantComposable: a @Composable function, getter or
// annotated lambda that uses nothing from the composition. krit reports only what it can prove: every call and
// property read must resolve to a non-composable declaration in the project or a scanned library.
type ComposeRedundantComposableRule struct {
	BaseRule
	// IgnoreAnnotated lists annotation FQNs whose declarations are not reported (compose-lints `ignore-annotated`).
	IgnoreAnnotated []string
}

// ComposeReadOnlyComposableRule ports compose-lints' ComposeReadOnlyComposable: a @Composable declaration whose
// composable uses are all @ReadOnlyComposable (CompositionLocal reads, MaterialTheme.colorScheme, stringResource,
// ...).
type ComposeReadOnlyComposableRule struct {
	BaseRule
}

// composeLintsUsage orders compose-lints' CompositionUsage, plus unknown for what krit cannot classify.
type composeLintsUsage int8

const (
	composeLintsUsageNone composeLintsUsage = iota
	composeLintsUsageReadOnly
	composeLintsUsageOther
	composeLintsUsageUnknown
)

// composeLintsMergeUsage combines two usages: OTHER dominates, then unknown, then READ_ONLY.
func composeLintsMergeUsage(a, b composeLintsUsage) composeLintsUsage {
	switch {
	case a == composeLintsUsageOther || b == composeLintsUsageOther:
		return composeLintsUsageOther
	case a == composeLintsUsageUnknown || b == composeLintsUsageUnknown:
		return composeLintsUsageUnknown
	case a == composeLintsUsageReadOnly || b == composeLintsUsageReadOnly:
		return composeLintsUsageReadOnly
	}
	return composeLintsUsageNone
}

func composeLintsUsageOfKind(kind int8) composeLintsUsage {
	switch kind {
	case 1:
		return composeLintsUsageOther
	case 2:
		return composeLintsUsageReadOnly
	}
	return composeLintsUsageNone
}

// composeLintsStateTypes are Compose State types: reading `.value` uses the composition.
var composeLintsStateTypes = map[string]bool{"State": true, "MutableState": true, "IntState": true,
	"MutableIntState": true, "LongState": true, "MutableLongState": true, "FloatState": true,
	"MutableFloatState": true, "DoubleState": true, "MutableDoubleState": true}

// composeLintsStateFactories create a State without remember.
var composeLintsStateFactories = map[string]bool{"mutableStateOf": true, "derivedStateOf": true,
	"mutableIntStateOf": true, "mutableLongStateOf": true, "mutableFloatStateOf": true,
	"mutableDoubleStateOf": true}

// composeLintsUsageClassifier classifies calls and reads against the cross-file index and the library tables.
type composeLintsUsageClassifier struct {
	index     *scanner.CodeIndex
	files     map[string]*scanner.File
	packages  map[string]bool // packages the project declares, built on first use
	symbols   map[composeLintsSymbolKey]composeLintsUsage
	ctorProps map[string][]scanner.Symbol // constructor val/var parameters by name, built on first use
	receivers map[composeLintsNodeKey]composeLintsReceivers
	wildcards map[*scanner.File][]string
	pkgs      map[*scanner.File]string
	// aliasNames and aliasIndex hold the project's top-level type aliases, by simple name and by FQN, built on
	// first use.
	aliasNames map[string]bool
	aliasIndex composeLintsTypeAliases
}

// composeLintsNodeKey identifies a node of a file.
type composeLintsNodeKey struct {
	file *scanner.File
	node uint32
}

// composeLintsSymbolKey identifies a project declaration, read or invoked.
type composeLintsSymbolKey struct {
	file    string
	start   int
	invoked bool
}

func newComposeLintsUsageClassifier(index *scanner.CodeIndex) *composeLintsUsageClassifier {
	files := make(map[string]*scanner.File, len(index.Files))
	for _, f := range index.Files {
		if f != nil {
			files[f.Path] = f
		}
	}
	return &composeLintsUsageClassifier{index: index, files: files, pkgs: map[*scanner.File]string{},
		symbols:   map[composeLintsSymbolKey]composeLintsUsage{},
		receivers: map[composeLintsNodeKey]composeLintsReceivers{}, wildcards: map[*scanner.File][]string{}}
}

// isProjectPackage reports whether a project file declares pkg.
func (c *composeLintsUsageClassifier) isProjectPackage(pkg string) bool {
	if c.packages == nil {
		c.packages = map[string]bool{}
		for _, f := range c.files {
			c.packages[c.packageOf(f)] = true
		}
	}
	return c.packages[pkg]
}

// packageOf returns a file's package from its package_header, memoized.
func (c *composeLintsUsageClassifier) packageOf(file *scanner.File) string {
	pkg, ok := c.pkgs[file]
	if !ok {
		if header, found := file.FlatFindChild(0, "package_header"); found {
			if id, found := file.FlatFindChild(header, "identifier"); found {
				pkg = strings.Join(strings.Fields(file.FlatNodeText(id)), "")
			}
		} else if file.Language != scanner.LangKotlin {
			pkg = filePackageName(file)
		}
		c.pkgs[file] = pkg
	}
	return pkg
}

// wildcardPackages memoizes composeLintsWildcardPackages per file.
func (c *composeLintsUsageClassifier) wildcardPackages(file *scanner.File) []string {
	w, ok := c.wildcards[file]
	if !ok {
		w = composeLintsWildcardPackages(file)
		c.wildcards[file] = w
	}
	return w
}

// projectUsage classifies the project's functions and properties named name that keep accepts, as a read or, when
// invoked, as the callee of a call; ok is false when there are none. Overloads that differ make it unknown.
func (c *composeLintsUsageClassifier) projectUsage(name string, invoked bool, keep func(scanner.Symbol) bool) (composeLintsUsage, bool) {
	found, mixed := false, false
	result := composeLintsUsageNone
	for _, sym := range c.symbolsNamed(name) {
		if keep != nil && !keep(sym) {
			continue
		}
		u, ok := c.symbolUsage(sym, invoked)
		if !ok {
			continue
		}
		if found && u != result {
			mixed = true
		}
		found, result = true, u
	}
	if mixed {
		result = composeLintsUsageUnknown
	}
	return result, found
}

// symbolUsage classifies one project declaration, read or invoked (a property called as a function invokes its
// value); ok is false for kinds that are not called or read.
func (c *composeLintsUsageClassifier) symbolUsage(sym scanner.Symbol, invoked bool) (composeLintsUsage, bool) {
	file := c.files[sym.File]
	if file == nil {
		return composeLintsUsageNone, false
	}
	key := composeLintsSymbolKey{file: sym.File, start: sym.StartByte, invoked: invoked}
	if u, ok := c.symbols[key]; ok {
		return u, true
	}
	var u composeLintsUsage
	switch sym.Kind {
	case "function":
		if !invoked {
			return composeLintsUsageNone, false // a read (`x.name`) never names a function
		}
		if fn := c.node(file, sym.StartByte, "function_declaration"); fn != 0 {
			u = composeLintsDeclUsage(file, fn)
		} else {
			u = composeLintsUsageUnknown
		}
	case "property":
		prop := c.node(file, sym.StartByte, "property_declaration")
		u = composeLintsPropertyUsage(file, prop)
		if invoked && prop != 0 {
			u = composeLintsMergeUsage(u, c.invokedValueUsage(file, prop))
		}
	case composeLintsConstructorPropertyKind:
		u = composeLintsUsageNone // reading a constructor property
		if invoked {
			u = composeLintsUsageUnknown
			if param := c.node(file, sym.StartByte, "class_parameter"); param != 0 {
				u = c.invokedValueUsage(file, param)
			}
		}
	case "class", "object", "interface":
		u = composeLintsUsageNone // a constructor or a type reference
	default:
		return composeLintsUsageNone, false
	}
	c.symbols[key] = u
	return u, true
}

// composeLintsConstructorPropertyKind marks the constructor val/var parameters krit adds to the index's symbols.
const composeLintsConstructorPropertyKind = "constructor-property"

// symbolsNamed returns the index's declarations named name plus the constructor properties of that name, which the
// index does not record.
func (c *composeLintsUsageClassifier) symbolsNamed(name string) []scanner.Symbol {
	syms := c.index.SymbolsNamed(name)
	if c.ctorProps == nil {
		c.ctorProps = map[string][]scanner.Symbol{}
		for _, cls := range c.index.Symbols {
			if cls.Kind != "class" || cls.Language != scanner.LangKotlin {
				continue
			}
			f := c.files[cls.File]
			if f == nil {
				continue
			}
			decl := c.node(f, cls.StartByte, "class_declaration")
			ctor, ok := f.FlatFindChild(decl, "primary_constructor")
			if decl == 0 || !ok {
				continue
			}
			for p := f.FlatFirstChild(ctor); p != 0; p = f.FlatNextSib(p) {
				if f.FlatType(p) != "class_parameter" || !f.FlatHasChildOfType(p, "binding_pattern_kind") {
					continue
				}
				prop := strings.Trim(f.FlatChildTextOrEmpty(p, "simple_identifier"), "`")
				c.ctorProps[prop] = append(c.ctorProps[prop], scanner.Symbol{Name: prop,
					Kind: composeLintsConstructorPropertyKind, File: cls.File, StartByte: int(f.FlatStartByte(p)),
					Language: cls.Language, Package: cls.Package, Owner: cls.FQN, FQN: cls.FQN + "." + prop})
			}
		}
	}
	if props := c.ctorProps[name]; len(props) > 0 {
		return append(append(make([]scanner.Symbol, 0, len(syms)+len(props)), syms...), props...)
	}
	return syms
}

// composeLintsTopLevelIn keeps the top-level declarations of pkg.
func composeLintsTopLevelIn(pkg string) func(scanner.Symbol) bool {
	return func(s scanner.Symbol) bool { return s.Owner == "" && s.Package == pkg }
}

// composeLintsMember keeps member declarations.
func composeLintsMember(s scanner.Symbol) bool { return s.Owner != "" }

// composeLintsAgree combines the usages of candidates the call may resolve to: unknown unless they agree.
func composeLintsAgree(us []composeLintsUsage) composeLintsUsage {
	for _, u := range us[1:] {
		if u != us[0] {
			return composeLintsUsageUnknown
		}
	}
	return us[0]
}

// node returns the declaration of the given kind that starts at byte start, descending from the root through the
// nodes that contain it, or 0.
func (c *composeLintsUsageClassifier) node(file *scanner.File, start int, kind string) uint32 {
	for n := uint32(0); ; {
		next := uint32(0)
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			if int(file.FlatStartByte(child)) <= start && start < int(file.FlatEndByte(child)) {
				next = child
				break
			}
		}
		if next == 0 {
			return 0
		}
		if int(file.FlatStartByte(next)) == start && file.FlatType(next) == kind {
			return next
		}
		n = next
	}
}

// composeLintsDeclUsage is how calling a function uses the composition: OTHER for @Composable, READ_ONLY for
// @ReadOnlyComposable, NONE otherwise.
func composeLintsDeclUsage(file *scanner.File, fn uint32) composeLintsUsage {
	if !acrFunctionIsComposable(file, fn) {
		return composeLintsUsageNone
	}
	if composeLintsHasRuntimeAnnotation(file, fn, "ReadOnlyComposable") {
		return composeLintsUsageReadOnly
	}
	return composeLintsUsageOther
}

// composeLintsHasRuntimeAnnotation reports whether decl carries an annotation that resolves to
// androidx.compose.runtime.<name>, not a same-named lookalike.
func composeLintsHasRuntimeAnnotation(file *scanner.File, decl uint32, name string) bool {
	found := false
	composeLintsEachAnnotation(file, decl, func(a uint32) {
		if !found && acrAnnotationIsRuntime(file, a, name) {
			found = true
		}
	})
	return found
}

// composeLintsPropertyUsage is how reading a property uses the composition, from its getter's annotations.
func composeLintsPropertyUsage(file *scanner.File, prop uint32) composeLintsUsage {
	if prop == 0 {
		return composeLintsUsageUnknown
	}
	getter := composeLintsPropertyGetter(file, prop)
	if getter == 0 || !acrHasComposableAnnotation(file, getter) {
		return composeLintsUsageNone
	}
	if composeLintsHasRuntimeAnnotation(file, getter, "ReadOnlyComposable") {
		return composeLintsUsageReadOnly
	}
	return composeLintsUsageOther
}

// composeLintsPropertyGetter returns a property's getter, inside it or as the detached sibling tree-sitter
// produces for a getter on its own line.
func composeLintsPropertyGetter(file *scanner.File, prop uint32) uint32 {
	if g, ok := file.FlatFindChild(prop, "getter"); ok {
		return g
	}
	for next := file.FlatNextSib(prop); next != 0; next = file.FlatNextSib(next) {
		switch file.FlatType(next) {
		case "getter":
			return next
		case "setter", "line_comment", "multiline_comment":
			continue
		}
		return 0
	}
	return 0
}

// importedUsage classifies a name bound by an explicit import to fqn.
func (c *composeLintsUsageClassifier) importedUsage(fqn string, invoked bool) composeLintsUsage {
	dot := strings.LastIndex(fqn, ".")
	if dot < 0 {
		return composeLintsUsageUnknown
	}
	pkg, name := fqn[:dot], fqn[dot+1:]
	if composeLintsScannedPackages[pkg] {
		return composeLintsUsageOfKind(composeLintsLibraryComposableFunctions[fqn])
	}
	if u, ok := c.projectUsage(name, invoked, func(s scanner.Symbol) bool { return s.FQN == fqn }); ok {
		return u
	}
	return composeLintsUsageUnknown
}

// nameUsage classifies an unqualified function or property name used at node, in Kotlin's lookup order: an explicit
// import, the file's package, star imports, then default imports (kotlin stdlib). Members of implicit receivers
// (an extension receiver, an enclosing class, a lambda's receiver) take part too, and a receiver whose type krit
// cannot see makes a name it cannot place unknown.
func (c *composeLintsUsageClassifier) nameUsage(file *scanner.File, node uint32, name string, invoked bool) composeLintsUsage {
	if facts := fileFactsCache().Imports(file); facts != nil {
		if fqn, ok := facts.Aliases[name]; ok {
			return c.importedUsage(fqn, invoked)
		}
	}
	result, found := c.topLevelUsage(file, name, invoked)
	receivers := c.implicitReceivers(file, node)
	if receivers.any {
		var candidates []composeLintsUsage
		if u, ok := c.projectUsage(name, invoked, composeLintsMember); ok {
			candidates = append(candidates, u)
		}
		if kind := composeLintsLibraryComposableMembers[name]; kind != 0 {
			candidates = append(candidates, composeLintsUsageOfKind(kind))
		}
		if len(candidates) > 0 {
			if found {
				candidates = append(candidates, result)
			}
			result, found = composeLintsAgree(candidates), true
		}
		if receivers.unknown && result != composeLintsUsageOther {
			return composeLintsUsageUnknown // a member of a type krit cannot see may take the name
		}
	}
	if !found {
		return composeLintsUsageNone // a default import (kotlin stdlib)
	}
	return result
}

// topLevelUsage classifies name among the top-level declarations visible in file without an explicit import: the
// file's package, then the star-imported packages. found is false when neither declares it.
func (c *composeLintsUsageClassifier) topLevelUsage(file *scanner.File, name string, invoked bool) (composeLintsUsage, bool) {
	if u, ok := c.projectUsage(name, invoked, composeLintsTopLevelIn(c.packageOf(file))); ok {
		return u, true
	}
	var candidates []composeLintsUsage
	unscanned := false
	for _, pkg := range c.wildcardPackages(file) {
		if u, ok := c.projectUsage(name, invoked, composeLintsTopLevelIn(pkg)); ok {
			candidates = append(candidates, u)
			continue
		}
		switch {
		case composeLintsScannedPackages[pkg]:
			if kind := composeLintsLibraryComposableFunctions[pkg+"."+name]; kind != 0 {
				candidates = append(candidates, composeLintsUsageOfKind(kind))
			}
		case !c.isProjectPackage(pkg):
			unscanned = true
		}
	}
	switch {
	case unscanned:
		return composeLintsUsageUnknown, true // a package krit has not scanned may declare it
	case len(candidates) == 0:
		return composeLintsUsageNone, false
	}
	return composeLintsAgree(candidates), true
}

// composeLintsReceivers describes the implicit receivers in scope at a node.
type composeLintsReceivers struct {
	any     bool // an extension receiver, an enclosing class or object, or a lambda (which may have a receiver)
	unknown bool // one of them is, or extends, a type krit has not scanned
}

func (c *composeLintsUsageClassifier) implicitReceivers(file *scanner.File, node uint32) composeLintsReceivers {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal", "function_declaration", "getter", "setter", "class_declaration", "object_declaration",
			"object_literal":
			return c.receiversOf(file, p)
		}
	}
	return composeLintsReceivers{}
}

// receiversOf returns the implicit receivers inside scope: its own and those of the scopes around it, memoized.
func (c *composeLintsUsageClassifier) receiversOf(file *scanner.File, scope uint32) composeLintsReceivers {
	key := composeLintsNodeKey{file: file, node: scope}
	if r, ok := c.receivers[key]; ok {
		return r
	}
	r := c.implicitReceivers(file, scope)
	switch file.FlatType(scope) {
	case "lambda_literal":
		r.any = true
	case "function_declaration":
		if t := composeLintsReceiverType(file, scope); t != 0 {
			r.any = true
			if !c.typeKnown(file, t, composeLintsTypeParameters(file, scope), 0) {
				r.unknown = true
			}
		}
	case "getter", "setter":
		if prop := composeLintsAccessorProperty(file, scope); prop != 0 {
			if t := composeLintsReceiverType(file, prop); t != 0 {
				r.any = true
				if !c.typeKnown(file, t, nil, 0) {
					r.unknown = true
				}
			}
		}
	case "class_declaration", "object_declaration", "object_literal":
		r.any = true
		if !c.supertypesKnown(file, scope, 0) {
			r.unknown = true
		}
	}
	c.receivers[key] = r
	return r
}

// typeKnown reports whether every member a type node may contribute is visible to krit: a project class whose
// supertypes are known, a type from a scanned library, a function type, a type parameter or a default-imported
// kotlin type.
func (c *composeLintsUsageClassifier) typeKnown(file *scanner.File, t uint32, typeParams map[string]bool, depth int) bool {
	for t != 0 {
		switch file.FlatType(t) {
		case "function_type":
			return true
		case "nullable_type", "parenthesized_type":
			t = file.FlatNamedChild(t, 0)
		case "user_type":
			name := composeLintsTypeText(file, t)
			if typeParams[name] {
				return true
			}
			if c.isAliasName(name[strings.LastIndex(name, ".")+1:]) && depth < 8 {
				if alias, ok := c.aliasIndex[c.aliasIndex.resolve(file, name)]; ok {
					return c.typeKnown(alias.file, composeLintsTypeAliasTarget(alias.file, alias.decl), nil, depth+1)
				}
			}
			if fqn := c.typeFQN(file, name); fqn != "" {
				return c.classKnown(fqn, depth)
			}
			return c.starImportsKnown(file) // a default import or a type from a star-imported package
		default:
			return false
		}
	}
	return false
}

// classKnown reports whether a class FQN's members are visible to krit: a project class whose supertypes are
// known, or a class of a scanned library, the project's packages, the Android SDK or the JDK.
func (c *composeLintsUsageClassifier) classKnown(fqn string, depth int) bool {
	if sym, ok := c.index.SymbolByFQN(fqn); ok && composeLintsClassLike(sym.Kind) {
		f := c.files[sym.File]
		if f == nil || f.Language != scanner.LangKotlin || depth > 8 {
			return true
		}
		decl := c.node(f, sym.StartByte, "class_declaration")
		if decl == 0 {
			decl = c.node(f, sym.StartByte, "object_declaration")
		}
		return decl == 0 || c.supertypesKnown(f, decl, depth+1)
	}
	// Walk outward through class segments only (a.b.Outer.Inner), so a subpackage of a scanned package
	// (androidx.compose.material.icons under androidx.compose.material) is not mistaken for it.
	for pkg := fqn; strings.Contains(pkg, "."); {
		dot := strings.LastIndex(pkg, ".")
		if !acrFirstRuneIsUpper(pkg[dot+1:]) {
			return false
		}
		pkg = pkg[:dot]
		// Project packages are scanned (enum entries and nested declarations included); Android SDK and JDK
		// classes declare no composables.
		if composeLintsScannedPackages[pkg] || c.isProjectPackage(pkg) || composeLintsPlatformPackage(pkg+".") {
			return true
		}
	}
	return false
}

func composeLintsClassLike(kind string) bool {
	return kind == "class" || kind == "object" || kind == "interface"
}

// typeFQN resolves a type name to a project class or an imported FQN: an import, a class in the file's package,
// a qualified name, or a class of a star-imported project package. It returns "" for a default import or a type
// from a star-imported library.
func (c *composeLintsUsageClassifier) typeFQN(file *scanner.File, name string) string {
	head, rest, qualified := strings.Cut(name, ".")
	if facts := fileFactsCache().Imports(file); facts != nil && facts.Aliases[head] != "" {
		if qualified {
			return facts.Aliases[head] + "." + rest
		}
		return facts.Aliases[head]
	}
	if sym, ok := c.index.SymbolByFQN(composeLintsJoin(c.packageOf(file), name)); ok && composeLintsClassLike(sym.Kind) {
		return sym.FQN
	}
	if qualified {
		return name
	}
	for _, pkg := range c.wildcardPackages(file) {
		if sym, ok := c.index.SymbolByFQN(pkg + "." + name); ok && composeLintsClassLike(sym.Kind) {
			return sym.FQN
		}
	}
	return ""
}

// starImportsKnown reports whether every star-imported package of file is one krit can see into.
func (c *composeLintsUsageClassifier) starImportsKnown(file *scanner.File) bool {
	for _, pkg := range c.wildcardPackages(file) {
		if !composeLintsScannedPackages[pkg] && !c.isProjectPackage(pkg) && !composeLintsPlatformPackage(pkg+".") {
			return false
		}
	}
	return true
}

// isAliasName reports whether a project file declares a top-level type alias with this simple name.
func (c *composeLintsUsageClassifier) isAliasName(name string) bool {
	if c.aliasNames == nil {
		c.aliasNames, c.aliasIndex = map[string]bool{}, composeLintsTypeAliases{}
		for _, f := range c.index.Files {
			if f == nil || f.FlatTree == nil || f.Language != scanner.LangKotlin {
				continue
			}
			for child := f.FlatFirstChild(0); child != 0; child = f.FlatNextSib(child) {
				if f.FlatType(child) == "type_alias" {
					alias := acrDeclarationName(f, child)
					c.aliasNames[alias] = true
					c.aliasIndex[composeLintsJoin(c.packageOf(f), alias)] = composeLintsAnnotationClass{file: f, decl: child}
				}
			}
		}
	}
	return c.aliasNames[name]
}

// supertypesKnown reports whether every supertype of a class, object or object literal is known.
func (c *composeLintsUsageClassifier) supertypesKnown(file *scanner.File, decl uint32, depth int) bool {
	known := true
	for spec := file.FlatFirstChild(decl); spec != 0 && known; spec = file.FlatNextSib(spec) {
		if file.FlatType(spec) != "delegation_specifier" {
			continue
		}
		ut := file.FlatFirstChild(spec)
		if t := file.FlatType(ut); t == "constructor_invocation" || t == "explicit_delegation" {
			ut = file.FlatFirstChild(ut)
		}
		known = ut != 0 && c.typeKnown(file, ut, nil, depth)
	}
	return known
}

// composeLintsAccessorProperty returns the property a getter or setter belongs to: its parent, or the property
// before it when tree-sitter detaches an accessor on its own line.
func composeLintsAccessorProperty(file *scanner.File, accessor uint32) uint32 {
	if parent, ok := file.FlatParent(accessor); ok && file.FlatType(parent) == "property_declaration" {
		return parent
	}
	for prev, ok := file.FlatPrevSibling(accessor); ok; prev, ok = file.FlatPrevSibling(prev) {
		switch file.FlatType(prev) {
		case "property_declaration":
			return prev
		case "getter", "setter", "line_comment", "multiline_comment":
			continue
		}
		return 0
	}
	return 0
}

// receiverKnown reports whether the receiver of a qualified call or read has a type krit can see. Receivers whose
// type krit does not infer (a call result, a chain) count as known.
func (c *composeLintsUsageClassifier) receiverKnown(file *scanner.File, scope composeLintsScope, receiver uint32) bool {
	return c.receiverKnownDepth(file, scope, receiver, 0)
}

func (c *composeLintsUsageClassifier) receiverKnownDepth(file *scanner.File, scope composeLintsScope, receiver uint32, depth int) bool {
	if receiver == 0 || file.FlatType(receiver) != "simple_identifier" {
		return true
	}
	name := file.FlatNodeText(receiver)
	if decl := composeLintsLocalDecl(file, scope, receiver, name); decl != 0 {
		var t uint32
		switch file.FlatType(decl) {
		case "parameter", "class_parameter":
			t = composeLintsDeclaredTypeAny(file, decl)
		case "property_declaration":
			if v, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
				t = composeLintsDeclaredTypeAny(file, v)
			}
			if init := composeLintsUnwrapParens(file, composeLintsPropertyInitializer(file, decl)); t == 0 && init != 0 &&
				file.FlatType(init) == "simple_identifier" && depth < 8 {
				// `val local = widget`: the alias has the aliased value's type.
				return c.receiverKnownDepth(file, scope, init, depth+1)
			}
		}
		return t == 0 || c.typeKnown(file, t, nil, 0)
	}
	if !acrFirstRuneIsUpper(name) {
		return true
	}
	// A type, object or enum entry reference (Widgets.render(), VendorBadge.render()): unknown only when it comes
	// from a library krit has not scanned.
	if facts := fileFactsCache().Imports(file); facts != nil && facts.Aliases[name] != "" {
		fqn := facts.Aliases[name]
		if _, ok := c.index.SymbolByFQN(fqn); ok {
			return true
		}
		return c.classKnown(fqn, 0)
	}
	if len(c.index.SymbolsNamed(name)) > 0 {
		return true
	}
	return c.starImportsKnown(file) // the file's package or a default import otherwise
}

// projectObjectMemberUsage classifies Receiver.name where Receiver is a project class or object visible in file (in
// its package or imported) that declares name itself; ok is false otherwise (an inherited member, an extension).
func (c *composeLintsUsageClassifier) projectObjectMemberUsage(file *scanner.File, scope composeLintsScope, receiver uint32, name string, invoked bool) (composeLintsUsage, bool) {
	if receiver == 0 || file.FlatType(receiver) != "simple_identifier" {
		return composeLintsUsageNone, false
	}
	owner := file.FlatNodeText(receiver)
	if !acrFirstRuneIsUpper(owner) || composeLintsLocalDecl(file, scope, receiver, owner) != 0 {
		return composeLintsUsageNone, false
	}
	fqn := composeLintsJoin(c.packageOf(file), owner)
	if facts := fileFactsCache().Imports(file); facts != nil && facts.Aliases[owner] != "" {
		fqn = facts.Aliases[owner]
	}
	sym, ok := c.index.SymbolByFQN(fqn)
	if !ok || (sym.Kind != "object" && sym.Kind != "class") {
		return composeLintsUsageNone, false
	}
	return c.projectUsage(name, invoked, func(s scanner.Symbol) bool { return s.Owner == sym.FQN })
}

// memberUsage classifies a selector name of a qualified call or read. A receiver that is an imported library class
// or object is looked up in that class's members; otherwise krit does not know the receiver type and classifies
// the name across the project and the libraries, abstaining when the receiver's type is one krit cannot see.
func (c *composeLintsUsageClassifier) memberUsage(file *scanner.File, scope composeLintsScope, receiver uint32, name string, invoked bool) composeLintsUsage {
	facts := fileFactsCache().Imports(file)
	if facts != nil {
		if fqn, ok := facts.Aliases[name]; ok {
			return c.importedUsage(fqn, invoked) // an imported extension
		}
		if receiver != 0 && file.FlatType(receiver) == "simple_identifier" {
			if cls, ok := facts.Aliases[file.FlatNodeText(receiver)]; ok && acrFirstRuneIsUpper(file.FlatNodeText(receiver)) {
				if dot := strings.LastIndex(cls, "."); dot > 0 && composeLintsScannedPackages[cls[:dot]] {
					return composeLintsUsageOfKind(composeLintsLibraryComposableMembersByClass[cls+"."+name])
				}
			}
		}
	}
	if u, ok := c.projectObjectMemberUsage(file, scope, receiver, name, invoked); ok {
		return u
	}
	result := composeLintsUsageNone
	if u, ok := c.projectUsage(name, invoked, nil); ok {
		result = u
	}
	if kind := composeLintsLibraryComposableMembers[name]; kind != 0 {
		result = composeLintsMergeUsage(result, composeLintsUsageOfKind(kind))
	}
	if result != composeLintsUsageNone {
		return result
	}
	for _, pkg := range c.wildcardPackages(file) {
		if !composeLintsScannedPackages[pkg] && !c.isProjectPackage(pkg) {
			return composeLintsUsageUnknown // an unscanned package may declare the extension
		}
	}
	if !c.receiverKnown(file, scope, receiver) || invoked && c.receiverHoldsComposableLambda(file, scope, receiver) {
		return composeLintsUsageUnknown
	}
	return result
}

// composeLintsScope is the function (or getter or lambda) being classified: its parameters and locals.
type composeLintsScope struct {
	params map[string]uint32
	root   uint32
}

// usageOf classifies everything under root: calls, qualified reads and name reads.
func (c *composeLintsUsageClassifier) usageOf(file *scanner.File, scope composeLintsScope, roots ...uint32) composeLintsUsage {
	result := composeLintsUsageNone
	for _, root := range roots {
		if root == 0 {
			continue
		}
		file.FlatWalkAllNodes(root, func(n uint32) {
			if result == composeLintsUsageOther {
				return
			}
			switch file.FlatType(n) {
			case "call_expression":
				result = composeLintsMergeUsage(result, c.callUsage(file, scope, n))
			case "navigation_expression":
				if parent, ok := file.FlatParent(n); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == n {
					return // the selector is the callee; classified as the call
				}
				result = composeLintsMergeUsage(result, c.readUsage(file, scope, n))
			case "simple_identifier", "interpolated_identifier":
				result = composeLintsMergeUsage(result, c.nameReadUsage(file, scope, n))
			}
		})
	}
	return result
}

func (c *composeLintsUsageClassifier) callUsage(file *scanner.File, scope composeLintsScope, call uint32) composeLintsUsage {
	callee := file.FlatFirstChild(call)
	if file.FlatType(callee) == "call_expression" {
		if suffix, ok := file.FlatFindChild(call, "call_suffix"); ok && file.FlatHasChildOfType(suffix, "value_arguments") {
			return composeLintsUsageUnknown // f()(): invoking a call result krit cannot type
		}
		return composeLintsUsageNone // f(x) { }: tree-sitter nests the call; the inner call is classified on its own
	}
	switch file.FlatType(callee) {
	case "simple_identifier":
		name := file.FlatNodeText(callee)
		if decl := composeLintsLocalDecl(file, scope, callee, name); decl != 0 {
			return c.invokedValueUsage(file, decl)
		}
		return c.nameUsage(file, callee, name, true)
	case "navigation_expression":
		suffix, ok := file.FlatFindChild(callee, "navigation_suffix")
		if !ok {
			return composeLintsUsageUnknown
		}
		name := file.FlatChildTextOrEmpty(suffix, "simple_identifier")
		if name == "" {
			return composeLintsUsageUnknown
		}
		if root, selectors := composeLintsChain(file, callee); root != 0 && file.FlatType(root) == "simple_identifier" &&
			file.FlatNodeText(root) == "androidx" {
			return c.importedUsage("androidx."+strings.Join(selectors, "."), true)
		}
		// receiver.slot(x) invokes a function-typed parameter with a receiver.
		if decl := composeLintsLocalDecl(file, scope, callee, name); decl != 0 && file.FlatType(decl) == "parameter" {
			return c.invokedValueUsage(file, decl)
		}
		if name == "invoke" {
			if u, ok := c.invokedExpressionUsage(file, scope, file.FlatFirstChild(callee)); ok {
				return u
			}
		}
		return c.memberUsage(file, scope, file.FlatFirstChild(callee), name, true)
	case "parenthesized_expression", "lambda_literal", "this_expression":
		return composeLintsUsageUnknown
	}
	return composeLintsUsageUnknown
}

// readUsage classifies a qualified read `receiver.name` (not a call).
func (c *composeLintsUsageClassifier) readUsage(file *scanner.File, scope composeLintsScope, nav uint32) composeLintsUsage {
	suffix, ok := file.FlatFindChild(nav, "navigation_suffix")
	if !ok {
		return composeLintsUsageUnknown
	}
	name := file.FlatChildTextOrEmpty(suffix, "simple_identifier")
	receiver := file.FlatFirstChild(nav)
	switch name {
	case "":
		return composeLintsUsageNone // `::class` and the like
	case "current":
		// CompositionLocal.current is read-only; Locals are named Local* by convention (CompositionLocalNaming).
		if file.FlatType(receiver) == "simple_identifier" && strings.HasPrefix(file.FlatNodeText(receiver), "Local") &&
			composeLintsLocalDecl(file, scope, receiver, file.FlatNodeText(receiver)) == 0 {
			if u, ok := c.compositionLocalRead(file, file.FlatNodeText(receiver)); ok {
				return u
			}
		}
	case "value":
		return c.valueReadUsage(file, scope, receiver)
	}
	if root, selectors := composeLintsChain(file, nav); root != 0 && file.FlatType(root) == "simple_identifier" &&
		file.FlatNodeText(root) == "androidx" {
		return c.importedUsage("androidx."+strings.Join(selectors, "."), false)
	}
	return c.memberUsage(file, scope, receiver, name, false)
}

// invokedExpressionUsage classifies `expr.invoke()` as calling expr: a local, parameter, property or constructor
// property, by its declared type. ok is false for other receivers.
func (c *composeLintsUsageClassifier) invokedExpressionUsage(file *scanner.File, scope composeLintsScope, expr uint32) (composeLintsUsage, bool) {
	switch file.FlatType(expr) {
	case "simple_identifier":
		name := file.FlatNodeText(expr)
		if decl := composeLintsLocalDecl(file, scope, expr, name); decl != 0 {
			return c.invokedValueUsage(file, decl), true
		}
		return c.nameUsage(file, expr, name, true), true
	case "navigation_expression":
		suffix, ok := file.FlatFindChild(expr, "navigation_suffix")
		if !ok {
			return composeLintsUsageNone, false
		}
		if name := file.FlatChildTextOrEmpty(suffix, "simple_identifier"); name != "" {
			return c.memberUsage(file, scope, file.FlatFirstChild(expr), name, true), true
		}
		return composeLintsUsageNone, false
	}
	return composeLintsUsageUnknown, true // screens[route]?.invoke(), f().invoke(): a value krit cannot type
}

// receiverHoldsComposableLambda reports whether a receiver chain (`tabs[i]`, `pair`, `screens.getValue(r)`) is
// rooted in a local or parameter whose declared type mentions a @Composable function type, so a member called on it
// may return one (tabs[i].second(), pair.second()).
func (c *composeLintsUsageClassifier) receiverHoldsComposableLambda(file *scanner.File, scope composeLintsScope, receiver uint32) bool {
	root := receiver
	for root != 0 {
		switch file.FlatType(root) {
		case "navigation_expression", "indexing_expression", "call_expression", "postfix_expression":
			root = file.FlatFirstChild(root)
			continue
		case "parenthesized_expression":
			root = file.FlatNamedChild(root, 0)
			continue
		}
		break
	}
	if root == 0 || file.FlatType(root) != "simple_identifier" {
		return false
	}
	decl := composeLintsLocalDecl(file, scope, root, file.FlatNodeText(root))
	var t uint32
	switch file.FlatType(decl) {
	case "parameter", "class_parameter":
		t = composeLintsDeclaredTypeAny(file, decl)
	case "property_declaration":
		if v, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
			t = composeLintsDeclaredTypeAny(file, v)
		}
	}
	if t == 0 {
		return false
	}
	holds := false
	file.FlatWalkAllNodes(t, func(n uint32) {
		switch {
		case holds:
		case file.FlatType(n) == "annotation" && acrAnnotationIsComposable(file, n):
			holds = true
		case file.FlatType(n) == "user_type" && c.isComposableAlias(file, n, 0):
			holds = true
		}
	})
	return holds
}

// compositionLocalRead classifies LocalX.current by what LocalX resolves to: a CompositionLocal from a scanned
// library or the project is a read-only read; a project declaration that is not one is classified as a member read
// (ok false); a name from a package krit has not scanned is unknown.
func (c *composeLintsUsageClassifier) compositionLocalRead(file *scanner.File, name string) (composeLintsUsage, bool) {
	project := func(keep func(scanner.Symbol) bool) (composeLintsUsage, bool, bool) {
		for _, sym := range c.index.SymbolsNamed(name) {
			if !keep(sym) {
				continue
			}
			f := c.files[sym.File]
			if f == nil || sym.Kind != "property" {
				return composeLintsUsageNone, false, true // an object or class named Local*: classify `current`
			}
			if composeLintsIsCompositionLocalDecl(f, c.node(f, sym.StartByte, "property_declaration")) {
				return composeLintsUsageReadOnly, true, true
			}
			return composeLintsUsageNone, false, true
		}
		return composeLintsUsageNone, false, false
	}
	if facts := fileFactsCache().Imports(file); facts != nil {
		if fqn, ok := facts.Aliases[name]; ok {
			if u, ok, found := project(func(s scanner.Symbol) bool { return s.FQN == fqn }); found {
				return u, ok
			}
			if dot := strings.LastIndex(fqn, "."); dot > 0 && composeLintsScannedPackages[fqn[:dot]] {
				return composeLintsUsageReadOnly, true
			}
			return composeLintsUsageUnknown, true
		}
	}
	if u, ok, found := project(composeLintsTopLevelIn(c.packageOf(file))); found {
		return u, ok
	}
	scanned := false
	for _, pkg := range c.wildcardPackages(file) {
		if u, ok, found := project(composeLintsTopLevelIn(pkg)); found {
			return u, ok
		}
		if !composeLintsScannedPackages[pkg] {
			return composeLintsUsageUnknown, true
		}
		scanned = true
	}
	if scanned {
		return composeLintsUsageReadOnly, true
	}
	return composeLintsUsageUnknown, true
}

// composeLintsIsCompositionLocalDecl reports whether a property is a CompositionLocal: created by a
// *compositionLocalOf factory or typed as a CompositionLocal.
func composeLintsIsCompositionLocalDecl(file *scanner.File, prop uint32) bool {
	if prop == 0 {
		return false
	}
	if v, ok := file.FlatFindChild(prop, "variable_declaration"); ok {
		if t := composeLintsDeclaredTypeAny(file, v); t != 0 {
			if file.FlatType(t) == "nullable_type" {
				t = file.FlatNamedChild(t, 0)
			}
			return file.FlatType(t) == "user_type" && (acrTypeIsRuntime(file, t, "CompositionLocal") ||
				acrTypeIsRuntime(file, t, "ProvidableCompositionLocal"))
		}
	}
	init := composeLintsUnwrapParens(file, composeLintsPropertyInitializer(file, prop))
	if init == 0 || file.FlatType(init) != "call_expression" {
		return false
	}
	// The factory must be Compose's, not a project function of the same name.
	switch name := composeLintsCalleeName(file, init); name {
	case "compositionLocalOf", "staticCompositionLocalOf", "compositionLocalWithComputedDefaultOf":
		return acrCalleeResolvesTo(file, init, acrRuntimePackage, name)
	}
	return false
}

// nameReadUsage classifies a bare name read: a delegated local, or a top-level property with a composable getter.
func (c *composeLintsUsageClassifier) nameReadUsage(file *scanner.File, scope composeLintsScope, id uint32) composeLintsUsage {
	parent, ok := file.FlatParent(id)
	if !ok {
		return composeLintsUsageNone
	}
	switch file.FlatType(parent) {
	case "navigation_suffix", "variable_declaration", "parameter", "class_parameter", "function_declaration",
		"lambda_parameters", "import_header", "package_header", "identifier", "value_parameter", "type_alias",
		"class_declaration", "object_declaration", "label", "annotation", "user_type", "enum_entry",
		"directly_assignable_expression", "binding_pattern_kind", "multi_variable_declaration", "catch_block",
		"for_statement", "when_subject":
		return composeLintsUsageNone
	case "call_expression":
		if file.FlatFirstChild(parent) == id {
			return composeLintsUsageNone // a callee, classified as the call
		}
	case "navigation_expression":
		// A receiver root (MaterialTheme in MaterialTheme.colorScheme) reads an object or a value; fall through.
	case "value_argument":
		if next := file.FlatNextSib(id); next != 0 && file.FlatType(next) == "=" {
			return composeLintsUsageNone // a named-argument label
		}
	}
	name := strings.TrimPrefix(file.FlatNodeText(id), "$")
	if decl := composeLintsLocalDecl(file, scope, id, name); decl != 0 {
		return c.delegatedReadUsage(file, scope, decl)
	}
	if !acrFirstRuneIsUpper(name) && !strings.Contains(name, "_") {
		// A lowercase name that is not a local: a property (top-level or member of an implicit receiver).
		return c.nameUsage(file, id, name, false)
	}
	// A capitalized name read is an object, class or CompositionLocal reference (MaterialTheme in
	// MaterialTheme.colorScheme), not a call of the same-named composable function.
	return composeLintsUsageNone
}

// composeLintsLocalDecl returns the parameter, or the local property declared before node inside the scope, that
// name refers to, or 0.
func composeLintsLocalDecl(file *scanner.File, scope composeLintsScope, node uint32, name string) uint32 {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			// A lambda parameter (or the implicit `it`) is a local value; it does not read the composition.
			if params, ok := file.FlatFindChild(p, "lambda_parameters"); ok {
				bound := uint32(0)
				file.FlatWalkNodes(params, "simple_identifier", func(id uint32) {
					if bound == 0 && file.FlatNodeTextEquals(id, name) {
						bound = id
					}
				})
				if bound != 0 {
					return bound
				}
			} else if name == "it" {
				return p
			}
		case "for_statement", "catch_block":
			found := uint32(0)
			for child := file.FlatFirstChild(p); child != 0; child = file.FlatNextSib(child) {
				switch file.FlatType(child) {
				case "variable_declaration", "simple_identifier":
					if file.FlatNodeTextEquals(child, name) || file.FlatChildTextOrEmpty(child, "simple_identifier") == name {
						found = child
					}
				}
			}
			if found != 0 {
				return found
			}
		}
		if file.FlatType(p) == "statements" {
			for child := file.FlatFirstChild(p); child != 0 && file.FlatStartByte(child) < file.FlatStartByte(node); child = file.FlatNextSib(child) {
				if file.FlatType(child) != "property_declaration" {
					continue
				}
				if v, ok := file.FlatFindChild(child, "variable_declaration"); ok && file.FlatChildTextOrEmpty(v, "simple_identifier") == name {
					return child
				}
				if multi, ok := file.FlatFindChild(child, "multi_variable_declaration"); ok {
					// `val (_, content) = pair`: a destructured entry, whose type krit does not infer.
					for v := file.FlatFirstChild(multi); v != 0; v = file.FlatNextSib(v) {
						if file.FlatType(v) == "variable_declaration" && file.FlatChildTextOrEmpty(v, "simple_identifier") == name {
							return child
						}
					}
				}
			}
		}
		if p == scope.root && file.FlatType(p) == "function_declaration" {
			break
		}
		if file.FlatType(p) == "function_declaration" {
			// A lambda or getter inside a function sees that function's parameters (slots included).
			for _, param := range composeLintsParams(file, p) {
				if strings.Trim(file.FlatChildTextOrEmpty(param.node, "simple_identifier"), "`") == name {
					return param.node
				}
			}
		}
	}
	return scope.params[name]
}

// invokedValueUsage classifies invoking a parameter, constructor property, property or local as a function, by its
// declared type: a @Composable function type, directly or through a project type alias, invokes a composable
// lambda (compose-lints' invokesComposableLambda).
func (c *composeLintsUsageClassifier) invokedValueUsage(file *scanner.File, decl uint32) composeLintsUsage {
	switch file.FlatType(decl) {
	case "parameter", "class_parameter":
		return c.declaredTypeInvokeUsage(file, decl)
	case "property_declaration":
		if v, ok := file.FlatFindChild(decl, "variable_declaration"); ok && composeLintsDeclaredTypeAny(file, v) != 0 {
			// The declared type decides: `val draw: @Composable () -> Unit = {}` invokes a composable lambda.
			return c.declaredTypeInvokeUsage(file, v)
		}
		init := composeLintsUnwrapParens(file, composeLintsPropertyInitializer(file, decl))
		switch {
		case init != 0 && file.FlatType(init) == "lambda_literal":
			return composeLintsUsageNone
		case init != 0 && file.FlatType(init) == "prefix_expression" && composeLintsIsComposableLambda(file, init):
			return composeLintsUsageOther
		}
	}
	return composeLintsUsageUnknown
}

// declaredTypeInvokeUsage classifies invoking a value of decl's declared type.
func (c *composeLintsUsageClassifier) declaredTypeInvokeUsage(file *scanner.File, decl uint32) composeLintsUsage {
	t := composeLintsDeclaredTypeAny(file, decl)
	switch {
	case acrComposableFunctionType(file, decl) != 0 || c.isComposableAlias(file, t, 0):
		return composeLintsUsageOther
	case t != 0 && (file.FlatType(t) == "function_type" || file.FlatType(t) == "nullable_type"):
		return composeLintsUsageNone
	case c.isFunctionAlias(file, t, 0):
		return composeLintsUsageNone // typealias Action = () -> Unit
	}
	return composeLintsUsageUnknown
}

// isFunctionAlias reports whether a type names a project type alias of a function type without @Composable.
func (c *composeLintsUsageClassifier) isFunctionAlias(file *scanner.File, t uint32, depth int) bool {
	if t != 0 && file.FlatType(t) == "nullable_type" {
		t = file.FlatNamedChild(t, 0)
	}
	if t == 0 || file.FlatType(t) != "user_type" || depth > 8 || c.isComposableAlias(file, t, 0) {
		return false
	}
	name := composeLintsTypeText(file, t)
	if !c.isAliasName(name[strings.LastIndex(name, ".")+1:]) {
		return false
	}
	alias, ok := c.aliasIndex[c.aliasIndex.resolve(file, name)]
	if !ok {
		return false
	}
	target := composeLintsTypeAliasTarget(alias.file, alias.decl)
	for target != 0 && alias.file.FlatType(target) == "type_modifiers" {
		target = alias.file.FlatNextSib(target)
	}
	switch alias.file.FlatType(target) {
	case "function_type":
		return true
	case "nullable_type", "parenthesized_type":
		inner := alias.file.FlatNamedChild(target, 0)
		return inner != 0 && alias.file.FlatType(inner) == "function_type" ||
			c.isFunctionAlias(alias.file, target, depth+1)
	}
	return c.isFunctionAlias(alias.file, target, depth+1)
}

// delegatedReadUsage classifies reading a local or parameter by name: a `by` delegate over a State uses the
// composition (compose-lints' isDelegatedStateRead), `by lazy` does not.
func (c *composeLintsUsageClassifier) delegatedReadUsage(file *scanner.File, scope composeLintsScope, decl uint32) composeLintsUsage {
	if file.FlatType(decl) != "property_declaration" {
		return composeLintsUsageNone
	}
	delegate, ok := file.FlatFindChild(decl, "property_delegate")
	if !ok {
		return composeLintsUsageNone
	}
	expr := composeLintsUnwrapParens(file, flatLastNamedChild(file, delegate))
	switch file.FlatType(expr) {
	case "call_expression":
		if name := composeLintsCalleeName(file, expr); name == "lazy" {
			return composeLintsUsageNone
		} else if composeLintsStateFactories[name] {
			return composeLintsUsageOther
		}
		return composeLintsUsageUnknown // classified as a call too; a non-composable call may still return a State
	case "simple_identifier":
		return c.valueReadUsage(file, scope, expr)
	}
	return composeLintsUsageUnknown
}

// valueReadUsage classifies `receiver.value` or delegating to receiver: OTHER for a Compose State, NONE for a type
// that resolves to a known non-State library type, unknown otherwise.
func (c *composeLintsUsageClassifier) valueReadUsage(file *scanner.File, scope composeLintsScope, receiver uint32) composeLintsUsage {
	if file.FlatType(receiver) != "simple_identifier" {
		return composeLintsUsageUnknown
	}
	decl := composeLintsLocalDecl(file, scope, receiver, file.FlatNodeText(receiver))
	if decl == 0 {
		return composeLintsUsageUnknown
	}
	var t uint32
	if file.FlatType(decl) == "parameter" {
		t = composeLintsDeclaredType(file, decl)
	} else if v, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
		t = composeLintsDeclaredType(file, v)
		if t == 0 {
			init := composeLintsUnwrapParens(file, composeLintsPropertyInitializer(file, decl))
			if init != 0 && file.FlatType(init) == "call_expression" {
				name := composeLintsCalleeName(file, init)
				if composeLintsStateFactories[name] || strings.HasPrefix(name, "remember") || strings.HasPrefix(name, "collectAs") {
					return composeLintsUsageOther
				}
			}
		}
	}
	name := composeLintsTypeSimpleName(file, t)
	switch {
	case name == "":
		return composeLintsUsageUnknown
	case composeLintsStateTypes[name]:
		return composeLintsUsageOther
	case composeLintsNonStateValueTypes[c.valueTypeFQN(file, name)]:
		return composeLintsUsageNone
	}
	return composeLintsUsageUnknown
}

// composeLintsNonStateValueTypes have a `value` that does not read the composition.
var composeLintsNonStateValueTypes = map[string]bool{"kotlin.Result": true, "kotlin.Lazy": true,
	"kotlinx.coroutines.flow.StateFlow": true, "kotlinx.coroutines.flow.MutableStateFlow": true,
	"androidx.lifecycle.LiveData": true, "androidx.lifecycle.MutableLiveData": true}

// valueTypeFQN resolves a value's type name: an import, a project class, a star-imported scanned package that
// names one of composeLintsNonStateValueTypes, or a default-imported kotlin type.
func (c *composeLintsUsageClassifier) valueTypeFQN(file *scanner.File, name string) string {
	if facts := fileFactsCache().Imports(file); facts != nil && facts.Aliases[name] != "" {
		return facts.Aliases[name]
	}
	if fqn := c.typeFQN(file, name); fqn != "" {
		return fqn // a project class
	}
	unscanned := false
	for _, pkg := range c.wildcardPackages(file) {
		switch {
		case composeLintsNonStateValueTypes[pkg+"."+name]:
			return pkg + "." + name
		case !composeLintsScannedPackages[pkg] && !c.isProjectPackage(pkg):
			unscanned = true // star imports win over default imports
		}
	}
	if !unscanned && (name == "Result" || name == "Lazy") {
		return "kotlin." + name
	}
	return name
}

// composeLintsIsComposableLambda reports whether a prefix_expression is `@Composable { ... }`.
func composeLintsIsComposableLambda(file *scanner.File, prefix uint32) bool {
	annotated := false
	for child := file.FlatFirstChild(prefix); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "annotation":
			if acrAnnotationIsComposable(file, child) {
				annotated = true
			}
		case "lambda_literal":
			return annotated
		}
	}
	return false
}

// hasSlotParameter reports whether a function takes a @Composable function-typed parameter, directly or through a
// project type alias (`typealias Slot = @Composable () -> Unit`).
func (c *composeLintsUsageClassifier) hasSlotParameter(file *scanner.File, fn uint32) bool {
	for _, p := range composeLintsParams(file, fn) {
		if acrComposableFunctionType(file, p.node) != 0 || c.isComposableAlias(file, composeLintsDeclaredTypeAny(file, p.node), 0) {
			return true
		}
	}
	return false
}

func (c *composeLintsUsageClassifier) isComposableAlias(file *scanner.File, t uint32, depth int) bool {
	if t != 0 && file.FlatType(t) == "nullable_type" {
		t = file.FlatNamedChild(t, 0)
	}
	if t == 0 || file.FlatType(t) != "user_type" || depth > 8 {
		return false
	}
	simple := ""
	for child := file.FlatFirstChild(t); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "type_identifier" {
			simple = file.FlatNodeText(child)
		}
	}
	if !c.isAliasName(simple) {
		return false
	}
	alias, ok := c.aliasIndex[c.aliasIndex.resolve(file, composeLintsTypeText(file, t))]
	if !ok {
		return false
	}
	return acrComposableFunctionType(alias.file, alias.decl) != 0 ||
		c.isComposableAlias(alias.file, composeLintsTypeAliasTarget(alias.file, alias.decl), depth+1)
}

// composeLintsIsContractDeclaration mirrors compose-lints' isContractDeclaration: a non-top-level member of an
// interface, or one marked override, open, abstract, expect, actual or external.
func composeLintsIsContractDeclaration(file *scanner.File, decl uint32) bool {
	if parent, ok := file.FlatParent(decl); ok && file.FlatType(parent) == "source_file" {
		return false
	}
	if composeLintsInInterface(file, decl) {
		return true
	}
	for _, m := range []string{"override", "open", "abstract", "expect", "actual", "external"} {
		if acrFunctionHasModifier(file, decl, m) {
			return true
		}
	}
	return false
}

// composeLintsComposableTarget is one declaration both rules judge.
type composeLintsComposableTarget struct {
	file       *scanner.File
	decl       uint32 // function_declaration, getter or prefix_expression (annotated lambda)
	annotation uint32
	usage      composeLintsUsage
	readOnly   bool // already @ReadOnlyComposable
	lambda     bool
}

// composeLintsComposableTargets classifies the project's composable functions, getters and annotated lambdas.
func composeLintsComposableTargets(index *scanner.CodeIndex, visit func(composeLintsComposableTarget)) {
	var c *composeLintsUsageClassifier
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("Composable")) {
			continue
		}
		classifier := func() *composeLintsUsageClassifier {
			if c == nil {
				c = newComposeLintsUsageClassifier(index)
			}
			return c
		}
		file.FlatWalkAllNodes(0, func(n uint32) {
			switch file.FlatType(n) {
			case "function_declaration":
				annotation := composeLintsComposableAnnotation(file, n)
				if annotation == 0 || acrIsLocalDeclaration(file, n) || composeLintsIsContractDeclaration(file, n) ||
					classifier().hasSlotParameter(file, n) {
					return
				}
				body, _ := file.FlatFindChild(n, "function_body")
				if body == 0 {
					return
				}
				scope := composeLintsScope{params: map[string]uint32{}, root: n}
				roots := []uint32{body}
				for _, p := range composeLintsParams(file, n) {
					scope.params[strings.Trim(file.FlatChildTextOrEmpty(p.node, "simple_identifier"), "`")] = p.node
					if p.def != 0 {
						roots = append(roots, p.def)
					}
				}
				visit(composeLintsComposableTarget{file: file, decl: n, annotation: annotation,
					usage: classifier().usageOf(file, scope, roots...), readOnly: composeLintsHasRuntimeAnnotation(file, n, "ReadOnlyComposable")})
			case "getter":
				var annotation uint32
				composeLintsEachAnnotation(file, n, func(a uint32) {
					if annotation == 0 && acrAnnotationIsComposable(file, a) {
						annotation = a
					}
				})
				body, _ := file.FlatFindChild(n, "function_body")
				if annotation == 0 || body == 0 {
					return
				}
				// Accessors take their modifiers (override, open) from the property.
				if prop := composeLintsAccessorProperty(file, n); prop != 0 && composeLintsIsContractDeclaration(file, prop) {
					return
				}
				visit(composeLintsComposableTarget{file: file, decl: n, annotation: annotation,
					usage:    classifier().usageOf(file, composeLintsScope{params: map[string]uint32{}, root: n}, body),
					readOnly: composeLintsHasRuntimeAnnotation(file, n, "ReadOnlyComposable")})
			case "prefix_expression":
				if !composeLintsIsComposableLambda(file, n) {
					return
				}
				lambda, _ := file.FlatFindChild(n, "lambda_literal")
				var annotation uint32
				for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "annotation" && acrAnnotationIsComposable(file, child) {
						annotation = child
					}
				}
				visit(composeLintsComposableTarget{file: file, decl: n, annotation: annotation, lambda: true,
					usage: classifier().usageOf(file, composeLintsScope{params: map[string]uint32{}, root: lambda}, lambda)})
			}
		})
	}
}

func (r *ComposeRedundantComposableRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	composeLintsComposableTargets(ctx.CodeIndex, func(t composeLintsComposableTarget) {
		if t.usage != composeLintsUsageNone || r.ignored(t) {
			return
		}
		ctx.Emit(r.Finding(t.file, t.file.FlatRow(t.annotation)+1, t.file.FlatCol(t.annotation)+1, composeRedundantComposableMessage))
	})
}

// ignored applies the ignore-annotated option: an ignored annotation on the declaration or lambda, or, as in
// compose-lints, on the object whose Unit `operator fun invoke` this is.
func (r *ComposeRedundantComposableRule) ignored(t composeLintsComposableTarget) bool {
	if len(r.IgnoreAnnotated) == 0 {
		return false
	}
	if t.lambda {
		for child := t.file.FlatFirstChild(t.decl); child != 0; child = t.file.FlatNextSib(child) {
			if t.file.FlatType(child) == "annotation" && r.ignoredAnnotation(t.file, child) {
				return true
			}
		}
		return false
	}
	if r.hasIgnoredAnnotation(t.file, t.decl) {
		return true
	}
	if t.file.FlatType(t.decl) != "function_declaration" || acrDeclarationName(t.file, t.decl) != "invoke" ||
		!acrFunctionHasModifier(t.file, t.decl, "operator") || acrFunctionReturnKind(t.file, t.decl) != acrReturnUnit {
		return false
	}
	body, ok := t.file.FlatParent(t.decl)
	if !ok || t.file.FlatType(body) != "class_body" {
		return false
	}
	owner, ok := t.file.FlatParent(body)
	return ok && t.file.FlatType(owner) == "object_declaration" && r.hasIgnoredAnnotation(t.file, owner)
}

func (r *ComposeRedundantComposableRule) hasIgnoredAnnotation(file *scanner.File, decl uint32) bool {
	found := false
	composeLintsEachAnnotation(file, decl, func(a uint32) {
		if !found && r.ignoredAnnotation(file, a) {
			found = true
		}
	})
	return found
}

// ignoredAnnotation reports whether an annotation resolves to an ignore-annotated FQN: written out, imported,
// declared in the file's package, or from a star-imported package.
func (r *ComposeRedundantComposableRule) ignoredAnnotation(file *scanner.File, a uint32) bool {
	text := composeLintsAnnotationTypeText(file, a)
	if text == "" {
		return false
	}
	head, rest, qualified := strings.Cut(text, ".")
	candidates := []string{text}
	if facts := fileFactsCache().Imports(file); facts != nil && facts.Aliases[head] != "" {
		candidates = []string{facts.Aliases[head]}
		if qualified {
			candidates[0] += "." + rest
		}
	} else {
		candidates = append(candidates, composeLintsJoin(filePackageName(file), text))
		for _, pkg := range composeLintsWildcardPackages(file) {
			candidates = append(candidates, pkg+"."+text)
		}
	}
	for _, candidate := range candidates {
		for _, ignored := range r.IgnoreAnnotated {
			if candidate == ignored {
				return true
			}
		}
	}
	return false
}

func (r *ComposeReadOnlyComposableRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	composeLintsComposableTargets(ctx.CodeIndex, func(t composeLintsComposableTarget) {
		if t.lambda || t.usage != composeLintsUsageReadOnly || t.readOnly {
			return
		}
		ctx.Emit(r.Finding(t.file, t.file.FlatRow(t.annotation)+1, t.file.FlatCol(t.annotation)+1, composeReadOnlyComposableMessage))
	})
}
