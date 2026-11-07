package rules

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

// VisibleForTestingCallerInNonTestRule flags calls to @VisibleForTesting
// functions from non-test files.
type VisibleForTestingCallerInNonTestRule struct {
	BaseRule
}

func (r *VisibleForTestingCallerInNonTestRule) Confidence() float64 { return api.ConfidenceMediumHigh }
func (r *VisibleForTestingCallerInNonTestRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}

	targetsByName := make(map[string][]visibleForTestingTarget)
	plainOverloads := make(map[string][]visibleForTestingSignature)
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		// Byte-level prefilter: skip files that don't contain the
		// string "VisibleForTesting" anywhere. The annotation name
		// is mandatory on every target site (Kotlin/Java don't
		// permit wildcard-imported annotations to be elided at the
		// declaration), so the absence of those bytes guarantees no
		// declaration walk can match. Same shape as the R.-byte
		// prefilter in resource-source rules. On the Signal-Android
		// corpus this skips ~98 % of the 4 600 files without
		// touching the parsed tree.
		if !visibleForTestingAnnotationMayMatch(file) {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "function_declaration" {
				return
			}
			if !flatHasAnnotationNamed(file, idx, "VisibleForTesting") {
				if plainName := file.FlatChildTextOrEmpty(idx, "simple_identifier"); plainName != "" {
					key := flatEnclosingOwnerName(file, idx) + "." + plainName
					plainOverloads[key] = append(plainOverloads[key], visibleForTestingSignature{
						minArgs: flatFunctionMinArgumentCount(file, idx),
						maxArgs: flatFunctionMaxArgumentCount(file, idx),
						params:  flatFunctionParameterTypes(file, idx),
					})
				}
				return
			}
			name := file.FlatChildTextOrEmpty(idx, "simple_identifier")
			if name == "" {
				return
			}
			targetsByName[name] = append(targetsByName[name], visibleForTestingTarget{
				name:  name,
				owner: flatEnclosingOwnerName(file, idx),
				file:  file.Path,
				node:  idx,
				visibleForTestingSignature: visibleForTestingSignature{
					minArgs: flatFunctionMinArgumentCount(file, idx),
					maxArgs: flatFunctionMaxArgumentCount(file, idx),
					params:  flatFunctionParameterTypes(file, idx),
				},
			})
		})
	}

	if len(targetsByName) == 0 {
		return
	}
	// Narrow the scan to files that reference at least one target
	// name. The CodeIndex's per-name ReferenceFiles map gives us the
	// exact set, so a corpus with thousands of files but only a few
	// hundred @VisibleForTesting declarations only walks the files
	// that mention those names instead of every non-test file. On
	// Signal-Android this collapses ~470 ms of full-tree walks into
	// the few ms of a reference-set union.
	candidateFiles := make(map[string]bool)
	for name := range targetsByName {
		for file := range index.ReferenceFiles(name) {
			candidateFiles[file] = true
		}
	}
	if len(candidateFiles) == 0 {
		return
	}

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !candidateFiles[file.Path] {
			continue
		}
		if scanner.IsTestFile(file.Path) {
			continue
		}
		file.FlatWalkAllNodes(0, func(idx uint32) {
			if file.FlatType(idx) != "call_expression" {
				return
			}
			if visibleForTestingCallerIsExempt(file, idx) {
				return
			}
			target, confidence, ok := resolveVisibleForTestingCallTarget(ctx, file, idx, targetsByName, plainOverloads)
			if !ok {
				return
			}
			ctx.Emit(scanner.Finding{
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				Col:        file.FlatCol(idx) + 1,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("@VisibleForTesting function %q called from non-test code.", target.name),
				Confidence: confidence,
			})
		})
	}
}

// visibleForTestingAnnotationMayMatch reports whether file's source
// bytes contain the literal "VisibleForTesting" anywhere. Kotlin and
// Java require the annotation name to be syntactically present at the
// declaration (no wildcard alias mechanism elides it), so a file
// without those bytes cannot contain a target. The check skips the
// flat-tree walk for every non-matching file.
func visibleForTestingAnnotationMayMatch(file *scanner.File) bool {
	if file == nil || len(file.Content) == 0 {
		return false
	}
	return bytes.Contains(file.Content, visibleForTestingAnnotationBytes)
}

// visibleForTestingCallerIsExempt reports calls made from another @VisibleForTesting declaration or a Fake* class.
func visibleForTestingCallerIsExempt(file *scanner.File, call uint32) bool {
	for cur, ok := file.FlatParent(call); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "property_declaration":
			if flatHasAnnotationNamed(file, cur, "VisibleForTesting") {
				return true
			}
		case "class_declaration", "object_declaration":
			if flatHasAnnotationNamed(file, cur, "VisibleForTesting") || strings.HasPrefix(extractIdentifierFlat(file, cur), "Fake") {
				return true
			}
		}
	}
	return false
}

var visibleForTestingAnnotationBytes = []byte("VisibleForTesting")

// visibleForTestingSignature is the arity range and normalized parameter types of one overload.
type visibleForTestingSignature struct {
	minArgs int
	maxArgs int
	params  []string
}

type visibleForTestingTarget struct {
	visibleForTestingSignature
	name  string
	owner string
	file  string
	node  uint32
}

func resolveVisibleForTestingCallTarget(ctx *api.Context, file *scanner.File, call uint32, targetsByName map[string][]visibleForTestingTarget, plainOverloads map[string][]visibleForTestingSignature) (visibleForTestingTarget, float64, bool) {
	// Cheap AST-only callee-name peek BEFORE the expensive
	// semantic ResolveCallTarget. The flatCallExpressionName lookup
	// is a constant-time child read; ResolveCallTarget can walk
	// resolver / type-inference state per call. Most call sites in
	// the candidate file set have a callee outside targetsByName
	// (the prefilter narrows by reference NAME, not declaration
	// site, so plenty of false-positive call sites still slip
	// through). Cheaply rejecting them buys back ~30 % of the
	// rule's wall time on the Signal-Android corpus.
	if name := flatCallExpressionName(file, call); name == "" || len(targetsByName[name]) == 0 {
		return visibleForTestingTarget{}, 0, false
	}
	callCtx := &api.Context{File: file, Resolver: ctx.Resolver}
	callTarget, ok := semantics.ResolveCallTarget(callCtx, call)
	if !ok || callTarget.CalleeName == "" {
		return visibleForTestingTarget{}, 0, false
	}
	candidates := targetsByName[callTarget.CalleeName]
	if len(candidates) == 0 {
		return visibleForTestingTarget{}, 0, false
	}
	argCount := len(callTarget.Arguments)

	receiver := semantics.ReferenceName(file, callTarget.Receiver.Node)
	if receiver != "" {
		if target, ok := uniqueVisibleForTestingTarget(candidates, func(t visibleForTestingTarget) bool {
			return visibleForTestingArityMatches(t, argCount) &&
				(t.owner == receiver || strings.HasSuffix(t.owner, "."+receiver)) &&
				visibleForTestingResolvesToTarget(callCtx, call, callTarget, t, candidates, plainOverloads[t.owner+"."+t.name])
		}); ok {
			confidence, ok := semantics.ConfidenceForEvidence(0.85, semantics.EvidenceQualifiedReceiver)
			return target, confidence, ok
		}
	}

	return visibleForTestingTarget{}, 0, false
}

func visibleForTestingArityMatches(target visibleForTestingTarget, argCount int) bool {
	return argCount >= target.minArgs && argCount <= target.maxArgs
}

func uniqueVisibleForTestingTarget(candidates []visibleForTestingTarget, match func(visibleForTestingTarget) bool) (visibleForTestingTarget, bool) {
	var found visibleForTestingTarget
	count := 0
	for _, candidate := range candidates {
		if !match(candidate) {
			continue
		}
		found = candidate
		count++
		if count > 1 {
			return visibleForTestingTarget{}, false
		}
	}
	return found, count == 1
}

func flatFunctionMinArgumentCount(file *scanner.File, funcDecl uint32) int {
	count := 0
	params, ok := file.FlatFindChild(funcDecl, "function_value_parameters")
	if !ok {
		return 0
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "parameter" && !flatParameterHasDefaultValue(file, child) {
			count++
		}
	}
	return count
}

func flatFunctionMaxArgumentCount(file *scanner.File, funcDecl uint32) int {
	count := 0
	params, ok := file.FlatFindChild(funcDecl, "function_value_parameters")
	if !ok {
		return 0
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "parameter" {
			count++
		}
	}
	return count
}

func flatParameterHasDefaultValue(file *scanner.File, param uint32) bool {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			return true
		}
	}
	return false
}

// OpenForTestingCallerInNonTestRule flags subclasses of @OpenForTesting
// types outside test source sets.
type OpenForTestingCallerInNonTestRule struct {
	BaseRule
}

func (r *OpenForTestingCallerInNonTestRule) Confidence() float64 { return api.ConfidenceMedium }
func (r *OpenForTestingCallerInNonTestRule) check(ctx *api.Context) {
	index := ctx.CodeIndex
	if index == nil {
		return
	}

	scopes := make(map[string]openForTestingFileScope, len(index.Files))
	targets := openForTestingTargetSet{
		byFQN:    make(map[string]openForTestingTarget),
		bySimple: make(map[string][]openForTestingTarget),
	}
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		scope := openForTestingScope(file)
		scopes[file.Path] = scope
		file.FlatWalkAllNodes(0, func(idx uint32) {
			nodeType := file.FlatType(idx)
			if nodeType != "class_declaration" && nodeType != "object_declaration" {
				return
			}
			if !openForTestingDeclarationHasAnnotation(file, idx, scope.imports) {
				return
			}
			target := openForTestingTargetForDeclaration(file, idx, scope.pkg)
			if target.simple == "" || target.fqn == "" {
				return
			}
			targets.byFQN[target.fqn] = target
			targets.bySimple[target.simple] = append(targets.bySimple[target.simple], target)
		})
	}

	if len(targets.byFQN) == 0 {
		return
	}

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if scanner.IsTestFile(file.Path) {
			continue
		}
		scope := scopes[file.Path]
		file.FlatWalkNodes(0, "class_declaration", func(classDecl uint32) {
			for child := file.FlatFirstChild(classDecl); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) != "delegation_specifier" {
					continue
				}
				ref := openForTestingSupertypeRef(file, child)
				if ref.simple == "" {
					continue
				}
				target, confidence, ok := resolveOpenForTestingSupertype(file, ref, scope, targets)
				if !ok {
					continue
				}
				ctx.Emit(scanner.Finding{
					File:       file.Path,
					Line:       file.FlatRow(ref.node) + 1,
					Col:        file.FlatCol(ref.node) + 1,
					RuleSet:    r.RuleSetName,
					Rule:       r.RuleName,
					Severity:   r.Sev,
					Message:    fmt.Sprintf("@OpenForTesting type %q subclassed outside test code.", target.simple),
					Confidence: confidence,
				})
			}
		})
	}
}

type openForTestingFileScope struct {
	pkg     string
	imports map[string]string
}

type openForTestingTarget struct {
	simple string
	fqn    string
	file   string
	node   uint32
}

type openForTestingTargetSet struct {
	byFQN    map[string]openForTestingTarget
	bySimple map[string][]openForTestingTarget
}

type openForTestingTypeRef struct {
	simple string
	path   []string
	node   uint32
}

func openForTestingScope(file *scanner.File) openForTestingFileScope {
	return openForTestingFileScope{
		pkg:     openForTestingPackageName(file),
		imports: openForTestingImports(file),
	}
}

func openForTestingTargetForDeclaration(file *scanner.File, decl uint32, pkg string) openForTestingTarget {
	name := openForTestingDeclarationName(file, decl)
	if name == "" {
		return openForTestingTarget{}
	}
	parts := make([]string, 0, 4)
	if pkg != "" {
		parts = append(parts, pkg)
	}
	if owner := flatEnclosingOwnerName(file, decl); owner != "" {
		parts = append(parts, strings.Split(owner, ".")...)
	}
	parts = append(parts, name)
	return openForTestingTarget{
		simple: name,
		fqn:    strings.Join(parts, "."),
		file:   file.Path,
		node:   decl,
	}
}

func openForTestingDeclarationName(file *scanner.File, decl uint32) string {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "simple_identifier":
			return file.FlatNodeString(child, nil)
		}
	}
	return ""
}

func openForTestingDeclarationHasAnnotation(file *scanner.File, decl uint32, imports map[string]string) bool {
	if mods, ok := file.FlatFindChild(decl, "modifiers"); ok {
		for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "annotation" && openForTestingAnnotationMatches(file, child, imports) {
				return true
			}
		}
	}
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "annotation" && openForTestingAnnotationMatches(file, child, imports) {
			return true
		}
	}
	return false
}

func openForTestingAnnotationMatches(file *scanner.File, annotation uint32, imports map[string]string) bool {
	ref := openForTestingTypeRefFromNode(file, annotation)
	if ref.simple == "" {
		return false
	}
	if ref.simple == "OpenForTesting" {
		return true
	}
	fqn := imports[ref.simple]
	return fqn == "OpenForTesting" || strings.HasSuffix(fqn, ".OpenForTesting")
}

func openForTestingSupertypeRef(file *scanner.File, spec uint32) openForTestingTypeRef {
	return openForTestingTypeRefFromNode(file, spec)
}

func openForTestingTypeRefFromNode(file *scanner.File, root uint32) openForTestingTypeRef {
	userType := openForTestingFirstUserType(file, root)
	if userType == 0 {
		return openForTestingTypeRef{}
	}
	path, nameNode := openForTestingUserTypePath(file, userType)
	if len(path) == 0 || nameNode == 0 {
		return openForTestingTypeRef{}
	}
	return openForTestingTypeRef{
		simple: path[len(path)-1],
		path:   path,
		node:   nameNode,
	}
}

func openForTestingFirstUserType(file *scanner.File, root uint32) uint32 {
	if file == nil || file.FlatTree == nil || int(root) >= file.FlatTree.Len() {
		return 0
	}
	if file.FlatType(root) == "user_type" {
		return root
	}
	for child := file.FlatFirstChild(root); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "type_arguments" {
			continue
		}
		if found := openForTestingFirstUserType(file, child); found != 0 {
			return found
		}
	}
	return 0
}

func openForTestingUserTypePath(file *scanner.File, userType uint32) ([]string, uint32) {
	var path []string
	var nameNode uint32
	var walk func(uint32)
	walk = func(idx uint32) {
		if file.FlatType(idx) == "type_arguments" {
			return
		}
		switch file.FlatType(idx) {
		case "type_identifier", "simple_identifier":
			path = append(path, file.FlatNodeString(idx, nil))
			nameNode = idx
			return
		}
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			walk(child)
		}
	}
	walk(userType)
	return path, nameNode
}

func resolveOpenForTestingSupertype(file *scanner.File, ref openForTestingTypeRef, scope openForTestingFileScope, targets openForTestingTargetSet) (openForTestingTarget, float64, bool) {
	if len(ref.path) > 1 {
		qualified := strings.Join(ref.path, ".")
		if target, ok := targets.byFQN[qualified]; ok {
			return target, 0.90, true
		}
		if scope.pkg != "" {
			if target, ok := targets.byFQN[scope.pkg+"."+qualified]; ok {
				return target, 0.85, true
			}
		}
	}

	if fqn := scope.imports[ref.simple]; fqn != "" {
		if target, ok := targets.byFQN[fqn]; ok {
			return target, 0.90, true
		}
	}
	if scope.pkg != "" {
		if target, ok := targets.byFQN[scope.pkg+"."+ref.simple]; ok {
			return target, 0.85, true
		}
	}

	candidates := targets.bySimple[ref.simple]
	if len(candidates) == 0 {
		return openForTestingTarget{}, 0, false
	}
	var found openForTestingTarget
	count := 0
	refOwner := openForTestingInheritanceOwner(file, ref.node)
	for _, candidate := range candidates {
		if candidate.file != file.Path || flatEnclosingOwnerName(file, candidate.node) != refOwner {
			continue
		}
		found = candidate
		count++
		if count > 1 {
			return openForTestingTarget{}, 0, false
		}
	}
	if count == 1 {
		return found, 0.80, true
	}
	return openForTestingTarget{}, 0, false
}

func openForTestingInheritanceOwner(file *scanner.File, ref uint32) string {
	classDecl, ok := flatEnclosingAncestor(file, ref, "class_declaration")
	if !ok {
		return flatEnclosingOwnerName(file, ref)
	}
	return flatEnclosingOwnerName(file, classDecl)
}

func openForTestingPackageName(file *scanner.File) string {
	var parts []string
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "package_header" {
			continue
		}
		openForTestingCollectImportPath(file, child, false, &parts, nil)
		break
	}
	return strings.Join(parts, ".")
}

func openForTestingImports(file *scanner.File) map[string]string {
	imports := make(map[string]string)
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "import_header":
			openForTestingAddImport(file, child, imports)
		case "import_list":
			for imp := file.FlatFirstChild(child); imp != 0; imp = file.FlatNextSib(imp) {
				if file.FlatType(imp) == "import_header" {
					openForTestingAddImport(file, imp, imports)
				}
			}
		}
	}
	return imports
}

func openForTestingAddImport(file *scanner.File, importHeader uint32, imports map[string]string) {
	var path []string
	var alias string
	openForTestingCollectImportPath(file, importHeader, false, &path, &alias)
	if len(path) == 0 {
		return
	}
	fqn := strings.Join(path, ".")
	key := path[len(path)-1]
	if alias != "" {
		key = alias
	}
	imports[key] = fqn
}

func openForTestingCollectImportPath(file *scanner.File, idx uint32, afterAs bool, path *[]string, alias *string) {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		childType := file.FlatType(child)
		switch childType {
		case "as":
			afterAs = true
			continue
		case "simple_identifier", "type_identifier":
			name := file.FlatNodeString(child, nil)
			if afterAs && alias != nil {
				*alias = name
			} else {
				*path = append(*path, name)
			}
			continue
		}
		openForTestingCollectImportPath(file, child, afterAs, path, alias)
	}
}

// visibleForTestingResolvesToTarget reports whether the call binds to target rather than to a
// sibling overload. An oracle signature decides first; otherwise argument types are compared with
// each overload's parameter types, and any other overload that could also accept the call makes it ambiguous.
func visibleForTestingResolvesToTarget(ctx *api.Context, call uint32, callTarget semantics.CallTarget, target visibleForTestingTarget, annotated []visibleForTestingTarget, plain []visibleForTestingSignature) bool {
	if sig, ok := visibleForTestingOracleSignature(callTarget); ok {
		return slices.Equal(sig, target.params)
	}
	argTypes := visibleForTestingArgumentTypes(ctx, call)
	argCount := len(callTarget.Arguments)
	if !visibleForTestingAccepts(target.visibleForTestingSignature, argCount, argTypes) {
		return false
	}
	for _, other := range annotated {
		if other.node != target.node && other.owner == target.owner && visibleForTestingAccepts(other.visibleForTestingSignature, argCount, argTypes) {
			return false
		}
	}
	for _, other := range plain {
		if visibleForTestingAccepts(other, argCount, argTypes) {
			return false
		}
	}
	return true
}

// visibleForTestingOracleSignature parses `owner.name(kotlin.Boolean, kotlin.Int)` style targets.
func visibleForTestingOracleSignature(target semantics.CallTarget) ([]string, bool) {
	if !target.Resolved {
		return nil, false
	}
	open, closeIdx := strings.Index(target.QualifiedName, "("), strings.LastIndex(target.QualifiedName, ")")
	if open < 0 || closeIdx < open {
		return nil, false
	}
	var params []string
	for _, part := range splitTopLevelCommas(target.QualifiedName[open+1 : closeIdx]) {
		params = append(params, normalizeParameterTypeName(part))
	}
	return params, true
}

func visibleForTestingAccepts(sig visibleForTestingSignature, argCount int, argTypes []visibleForTestingArgType) bool {
	if argCount < sig.minArgs || argCount > sig.maxArgs {
		return false
	}
	for i, arg := range argTypes {
		if i < len(sig.params) && !visibleForTestingTypeCompatible(arg, sig.params[i]) {
			return false
		}
	}
	return true
}

// visibleForTestingArgType is an argument type; name is empty when unknown.
type visibleForTestingArgType struct {
	name    string
	literal bool
}

// visibleForTestingArgumentTypes returns positional argument types, or nil when any argument is named.
func visibleForTestingArgumentTypes(ctx *api.Context, call uint32) []visibleForTestingArgType {
	file := ctx.File
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return nil
	}
	var types []visibleForTestingArgType
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if _, named := file.FlatFindChild(arg, "="); named {
			return nil
		}
		types = append(types, visibleForTestingExpressionType(ctx, flatValueArgumentExpression(file, arg)))
	}
	return types
}

func visibleForTestingExpressionType(ctx *api.Context, expr uint32) visibleForTestingArgType {
	file := ctx.File
	if expr == 0 {
		return visibleForTestingArgType{}
	}
	switch file.FlatType(expr) {
	case "boolean_literal":
		return visibleForTestingArgType{name: "Boolean", literal: true}
	case "string_literal":
		return visibleForTestingArgType{name: "String", literal: true}
	case "character_literal":
		return visibleForTestingArgType{name: "Char", literal: true}
	case "integer_literal":
		text := file.FlatNodeText(expr)
		if strings.HasSuffix(text, "L") {
			return visibleForTestingArgType{name: "Long", literal: true}
		}
		return visibleForTestingArgType{name: "Int", literal: true}
	case "real_literal":
		if text := file.FlatNodeText(expr); strings.HasSuffix(text, "f") || strings.HasSuffix(text, "F") {
			return visibleForTestingArgType{name: "Float", literal: true}
		}
		return visibleForTestingArgType{name: "Double", literal: true}
	}
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(expr, file); typ != nil && typ.Resolved {
			return visibleForTestingArgType{name: typ.Name}
		}
	}
	return visibleForTestingArgType{}
}

var visibleForTestingPrimitiveTypes = map[string]bool{
	"Boolean": true, "String": true, "Char": true, "Int": true, "Long": true,
	"Short": true, "Byte": true, "Float": true, "Double": true,
}

// visibleForTestingTypeCompatible is false only when both types are well-known value types that differ.
func visibleForTestingTypeCompatible(arg visibleForTestingArgType, param string) bool {
	if arg.name == "" || !visibleForTestingPrimitiveTypes[param] || !visibleForTestingPrimitiveTypes[arg.name] {
		return true
	}
	if arg.name == param {
		return true
	}
	// An integer literal also fits the narrower and wider integral types.
	return arg.literal && arg.name == "Int" && (param == "Long" || param == "Short" || param == "Byte")
}

func flatFunctionParameterTypes(file *scanner.File, funcDecl uint32) []string {
	params, ok := file.FlatFindChild(funcDecl, "function_value_parameters")
	if !ok {
		return nil
	}
	var types []string
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		text := file.FlatNodeText(child)
		if cut := strings.Index(text, "="); cut >= 0 {
			text = text[:cut]
		}
		if colon := strings.Index(text, ":"); colon >= 0 {
			types = append(types, normalizeParameterTypeName(text[colon+1:]))
		} else {
			types = append(types, "")
		}
	}
	return types
}

// normalizeParameterTypeName reduces a type to its simple name without nullability or type arguments.
func normalizeParameterTypeName(text string) string {
	text = strings.TrimSpace(text)
	if cut := strings.Index(text, "<"); cut >= 0 {
		text = text[:cut]
	}
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "?"))
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		text = text[dot+1:]
	}
	return text
}

// splitTopLevelCommas splits on commas outside angle brackets and parentheses.
func splitTopLevelCommas(text string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '<', '(':
			depth++
		case '>', ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, text[start:i])
				start = i + 1
			}
		}
	}
	if strings.TrimSpace(text[start:]) != "" {
		parts = append(parts, text[start:])
	}
	return parts
}
