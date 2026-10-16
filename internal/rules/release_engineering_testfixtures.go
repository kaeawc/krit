package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/sourceheader"
)

// TestFixtureAccessedFromProductionRule flags usage of types declared under
// src/testFixtures/ from non-test files.
type TestFixtureAccessedFromProductionRule struct {
	BaseRule
}

func (r *TestFixtureAccessedFromProductionRule) Confidence() float64 { return api.ConfidenceMediumHigh }

type testFixtureDeclaration struct {
	qualifiedName string
}

func (r *TestFixtureAccessedFromProductionRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}

	parsedFiles := ctx.ParsedFiles
	if len(parsedFiles) == 0 {
		parsedFiles = index.Files
	}
	if len(parsedFiles) == 0 {
		return
	}

	filesByPath := make(map[string]*scanner.File, len(parsedFiles))
	packageByPath := make(map[string]string, len(parsedFiles))
	for _, file := range parsedFiles {
		if file == nil {
			continue
		}
		filesByPath[file.Path] = file
		packageByPath[file.Path] = sourcePackageName(file)
	}

	fixturesByQualified := make(map[string]testFixtureDeclaration)
	productionDeclarations := make(map[string]bool)
	for _, sym := range index.Symbols {
		if !isClassLikeFixtureSymbol(sym) {
			continue
		}
		file := filesByPath[sym.File]
		if file == nil {
			continue
		}
		packageName := packageByPath[sym.File]
		qualifiedName := qualifySourceName(packageName, sym.Name)
		if isTestFixturePath(sym.File) {
			if isNestedClassLikeSymbol(file, sym) {
				continue
			}
			fixturesByQualified[qualifiedName] = testFixtureDeclaration{
				qualifiedName: qualifiedName,
			}
			continue
		}
		productionDeclarations[qualifiedName] = true
	}
	for _, file := range parsedFiles {
		if file == nil || isTestFixturePath(file.Path) || scanner.IsTestFile(filepath.ToSlash(file.Path)) {
			continue
		}
		for _, name := range topLevelClassLikeNames(file) {
			productionDeclarations[qualifySourceName(packageByPath[file.Path], name)] = true
		}
	}

	if len(fixturesByQualified) == 0 {
		return
	}

	emitted := make(map[string]bool)
	for _, file := range parsedFiles {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if scanner.IsTestFile(filepath.ToSlash(file.Path)) || isTestFixturePath(file.Path) || isGeneratedSourcePath(file.Path) {
			continue
		}

		imports, importedNames := fixtureImportBindings(file, fixturesByQualified)
		for _, binding := range imports {
			if productionDeclarations[binding.decl.qualifiedName] {
				continue
			}
			r.emitTestFixtureAccess(ctx, file, binding.node, binding.decl, 0.95, emitted)
		}

		packageName := packageByPath[file.Path]
		fileCtx := &api.Context{File: file, Resolver: ctx.Resolver, CodeIndex: index}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			decl, confidence, ok := resolveFixtureAccessNode(fileCtx, idx, packageName, fixturesByQualified, imports, importedNames, productionDeclarations)
			if !ok {
				return
			}
			r.emitTestFixtureAccess(ctx, file, idx, decl, confidence, emitted)
		})
	}
}

func resolveFixtureAccessNode(ctx *api.Context, idx uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if ctx.File == nil {
		return testFixtureDeclaration{}, 0, false
	}
	file := ctx.File
	switch file.FlatType(idx) {
	case "call_expression":
		return resolveFixtureCall(ctx, idx, packageName, fixtures, imports, importedNames, productionDeclarations)
	case "navigation_expression":
		if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" {
			return testFixtureDeclaration{}, 0, false
		}
		return resolveFixtureNavigation(ctx, idx, packageName, fixtures, imports, importedNames, productionDeclarations)
	case "user_type", "object_creation_expression":
		return resolveFixtureTypeLikeNode(ctx, idx, packageName, fixtures, imports, importedNames, productionDeclarations)
	case "type_identifier":
		if file.Language != scanner.LangJava || isDeclarationIdentifier(file, idx) || hasFlatAncestorTypeName(file, idx, "import_declaration", "package_declaration") {
			return testFixtureDeclaration{}, 0, false
		}
		return resolveFixtureTypeLikeNode(ctx, idx, packageName, fixtures, imports, importedNames, productionDeclarations)
	}
	return testFixtureDeclaration{}, 0, false
}

func resolveFixtureCall(ctx *api.Context, call uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if decl, ok := fixtureByQualifiedReferenceText(ctx.File, call, fixtures); ok {
		if productionDeclarations[decl.qualifiedName] {
			return testFixtureDeclaration{}, 0, false
		}
		return decl, 0.95, true
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return testFixtureDeclaration{}, 0, false
	}
	if target.Resolved {
		if decl, ok := fixtures[target.QualifiedName]; ok {
			if productionDeclarations[decl.qualifiedName] {
				return testFixtureDeclaration{}, 0, false
			}
			return decl, 0.95, true
		}
	}
	if target.Receiver.Valid() {
		if decl, confidence, ok := resolveFixtureName(ctx, semantics.ReferenceName(ctx.File, target.Receiver.Node), target.Receiver.Node, packageName, fixtures, imports, importedNames, productionDeclarations); ok {
			return decl, confidence, true
		}
	}
	return resolveFixtureName(ctx, target.CalleeName, call, packageName, fixtures, imports, importedNames, productionDeclarations)
}

func resolveFixtureNavigation(ctx *api.Context, nav uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if decl, ok := fixtureByQualifiedReferenceText(ctx.File, nav, fixtures); ok {
		if productionDeclarations[decl.qualifiedName] {
			return testFixtureDeclaration{}, 0, false
		}
		return decl, 0.95, true
	}
	receiver := firstReferenceIdentifier(ctx.File, nav)
	return resolveFixtureName(ctx, receiver, nav, packageName, fixtures, imports, importedNames, productionDeclarations)
}

func resolveFixtureTypeLikeNode(ctx *api.Context, idx uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if hasFlatAncestorTypeName(ctx.File, idx, "line_comment", "multiline_comment", "string_literal", "raw_string_literal", "character_literal") {
		return testFixtureDeclaration{}, 0, false
	}
	if decl, ok := fixtureByQualifiedReferenceText(ctx.File, idx, fixtures); ok {
		if productionDeclarations[decl.qualifiedName] {
			return testFixtureDeclaration{}, 0, false
		}
		return decl, 0.95, true
	}
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(idx, ctx.File); typ != nil {
			if decl, ok := fixtures[typ.FQN]; ok {
				if productionDeclarations[decl.qualifiedName] {
					return testFixtureDeclaration{}, 0, false
				}
				return decl, 0.95, true
			}
		}
	}
	return resolveFixtureName(ctx, lastReferenceIdentifier(ctx.File, idx), idx, packageName, fixtures, imports, importedNames, productionDeclarations)
}

func resolveFixtureName(ctx *api.Context, name string, ref uint32, packageName string, fixtures map[string]testFixtureDeclaration, imports map[string]fixtureImportBinding, importedNames map[string]string, productionDeclarations map[string]bool) (testFixtureDeclaration, float64, bool) {
	if name == "" {
		return testFixtureDeclaration{}, 0, false
	}
	if binding, ok := imports[name]; ok {
		if productionDeclarations[binding.decl.qualifiedName] {
			return testFixtureDeclaration{}, 0, false
		}
		return binding.decl, 0.95, true
	}
	if ctx.Resolver != nil {
		if imported := ctx.Resolver.ResolveImport(name, ctx.File); imported != "" {
			if decl, ok := fixtures[imported]; ok {
				if productionDeclarations[decl.qualifiedName] {
					return testFixtureDeclaration{}, 0, false
				}
				return decl, 0.95, true
			}
			return testFixtureDeclaration{}, 0, false
		}
	}
	if imported, ok := importedNames[name]; ok {
		if _, isFixtureImport := fixtures[imported]; !isFixtureImport {
			return testFixtureDeclaration{}, 0, false
		}
	}

	qualifiedName := qualifySourceName(packageName, name)
	decl, ok := fixtures[qualifiedName]
	if !ok {
		return testFixtureDeclaration{}, 0, false
	}
	if productionDeclarations[qualifiedName] {
		return testFixtureDeclaration{}, 0, false
	}
	if sameFileClassLikeDeclarationShadows(ctx, name, ref) {
		return testFixtureDeclaration{}, 0, false
	}
	if !samePackageReferenceContext(ctx.File, ref) {
		return testFixtureDeclaration{}, 0, false
	}
	return decl, 0.80, true
}

func sameFileClassLikeDeclarationShadows(ctx *api.Context, name string, ref uint32) bool {
	if ctx.File == nil || name == "" {
		return false
	}
	file := ctx.File
	var shadowed bool
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if shadowed {
			return
		}
		switch file.FlatType(idx) {
		case "class_declaration", "object_declaration", "interface_declaration":
			if isNestedClassLikeNode(file, idx) && !semantics.SameEnclosingOwner(file, idx, ref) {
				return
			}
			if firstChildText(file, idx, "type_identifier", "simple_identifier", "identifier") == name &&
				semantics.SameFileDeclarationMatch(ctx, idx, ref) {
				shadowed = true
			}
		}
	})
	return shadowed
}

func samePackageReferenceContext(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "call_expression", "user_type", "type_identifier", "object_creation_expression":
		return true
	case "navigation_expression":
		return firstReferenceIdentifier(file, idx) != ""
	default:
		return false
	}
}

func firstReferenceIdentifier(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "simple_identifier", "type_identifier", "identifier":
			return file.FlatNodeText(child)
		case "scoped_identifier", "scoped_type_identifier", "navigation_expression":
			if name := firstReferenceIdentifier(file, child); name != "" {
				return name
			}
		}
	}
	return ""
}

func lastReferenceIdentifier(file *scanner.File, idx uint32) string {
	last := ""
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier", "identifier":
			if !isDeclarationIdentifier(file, candidate) {
				last = file.FlatNodeText(candidate)
			}
		}
	})
	if last != "" {
		return last
	}
	if file.FlatType(idx) == "simple_identifier" || file.FlatType(idx) == "type_identifier" || file.FlatType(idx) == "identifier" {
		return file.FlatNodeText(idx)
	}
	return ""
}

type fixtureImportBinding struct {
	node uint32
	decl testFixtureDeclaration
}

func (r *TestFixtureAccessedFromProductionRule) emitTestFixtureAccess(ctx *api.Context, file *scanner.File, idx uint32, decl testFixtureDeclaration, confidence float64, emitted map[string]bool) {
	line := file.FlatRow(idx) + 1
	col := file.FlatCol(idx) + 1
	key := fmt.Sprintf("%s:%d:%d:%s", file.Path, line, col, decl.qualifiedName)
	if emitted[key] {
		return
	}
	emitted[key] = true
	ctx.Emit(scanner.Finding{
		File:       file.Path,
		Line:       line,
		Col:        col,
		RuleSet:    r.RuleSetName,
		Rule:       r.RuleName,
		Severity:   r.Sev,
		Message:    fmt.Sprintf("Test fixture type %q from testFixtures/ used in production code.", decl.qualifiedName),
		Confidence: confidence,
	})
}

func fixtureImportBindings(file *scanner.File, fixtures map[string]testFixtureDeclaration) (map[string]fixtureImportBinding, map[string]string) {
	bindings := make(map[string]fixtureImportBinding)
	importedNames := make(map[string]string)
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		if nodeType != "import_header" && nodeType != "import_declaration" {
			return
		}
		qualifiedName, localName, wildcard := parseSourceImport(file.FlatNodeText(idx))
		if qualifiedName == "" || wildcard {
			return
		}
		importedNames[localName] = qualifiedName
		if decl, ok := fixtures[qualifiedName]; ok {
			bindings[localName] = fixtureImportBinding{
				node: idx,
				decl: decl,
			}
		}
	})
	return bindings, importedNames
}

func fixtureByQualifiedReferenceText(file *scanner.File, idx uint32, fixtures map[string]testFixtureDeclaration) (testFixtureDeclaration, bool) {
	for current, ok := idx, true; ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "user_type", "navigation_expression", "call_expression", "object_creation_expression", "scoped_identifier", "scoped_type_identifier", "field_access", "method_invocation":
			text := compactSourceReference(file.FlatNodeText(current))
			for qualifiedName, decl := range fixtures {
				if text == qualifiedName ||
					strings.HasPrefix(text, qualifiedName+"<") ||
					strings.HasPrefix(text, qualifiedName+"(") ||
					strings.HasPrefix(text, qualifiedName+".") {
					return decl, true
				}
			}
		}
	}
	return testFixtureDeclaration{}, false
}

func isDeclarationIdentifier(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "class_declaration", "object_declaration", "function_declaration", "variable_declaration", "function_value_parameter",
		"interface_declaration", "method_declaration", "constructor_declaration", "variable_declarator", "formal_parameter":
		return true
	}
	return false
}

func topLevelClassLikeNames(file *scanner.File) []string {
	var names []string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "class_declaration", "object_declaration", "interface_declaration":
			if isNestedClassLikeNode(file, idx) {
				return
			}
			if name := firstChildText(file, idx, "type_identifier", "simple_identifier", "identifier"); name != "" {
				names = append(names, name)
			}
		}
	})
	return names
}

func firstChildText(file *scanner.File, idx uint32, nodeTypes ...string) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		childType := file.FlatType(child)
		for _, nodeType := range nodeTypes {
			if childType == nodeType {
				return file.FlatNodeText(child)
			}
		}
	}
	return ""
}

func sourcePackageName(file *scanner.File) string {
	if file == nil || file.FlatTree == nil {
		return ""
	}
	var packageName string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if packageName != "" {
			return
		}
		nodeType := file.FlatType(idx)
		if nodeType != "package_header" && nodeType != "package_declaration" {
			return
		}
		packageName = sourceheader.FirstHeaderLine(file.FlatNodeText(idx), "package")
	})
	return packageName
}

func isClassLikeFixtureSymbol(sym scanner.Symbol) bool {
	return sym.Name != "" && (sym.Kind == "class" || sym.Kind == "interface" || sym.Kind == "object")
}

func isNestedClassLikeSymbol(file *scanner.File, sym scanner.Symbol) bool {
	if file == nil || sym.StartByte < 0 || sym.EndByte <= sym.StartByte {
		return false
	}
	idx, ok := file.FlatNamedDescendantForByteRange(uint32(sym.StartByte), uint32(sym.EndByte))
	if !ok {
		return false
	}
	return isNestedClassLikeNode(file, idx)
}

func isNestedClassLikeNode(file *scanner.File, idx uint32) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "class_declaration", "object_declaration", "interface_declaration":
			return true
		}
	}
	return false
}

func hasFlatAncestorTypeName(file *scanner.File, idx uint32, nodeTypes ...string) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		currentType := file.FlatType(current)
		for _, nodeType := range nodeTypes {
			if currentType == nodeType {
				return true
			}
		}
	}
	return false
}
