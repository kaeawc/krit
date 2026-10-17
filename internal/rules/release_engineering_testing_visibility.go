package rules

import (
	"bytes"
	"fmt"
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
				return
			}
			name := file.FlatChildTextOrEmpty(idx, "simple_identifier")
			if name == "" {
				return
			}
			targetsByName[name] = append(targetsByName[name], visibleForTestingTarget{
				name:    name,
				owner:   flatEnclosingOwnerName(file, idx),
				file:    file.Path,
				node:    idx,
				minArgs: flatFunctionMinArgumentCount(file, idx),
				maxArgs: flatFunctionMaxArgumentCount(file, idx),
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
			target, confidence, ok := resolveVisibleForTestingCallTarget(file, idx, targetsByName)
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

var visibleForTestingAnnotationBytes = []byte("VisibleForTesting")

type visibleForTestingTarget struct {
	name    string
	owner   string
	file    string
	node    uint32
	minArgs int
	maxArgs int
}

func resolveVisibleForTestingCallTarget(file *scanner.File, call uint32, targetsByName map[string][]visibleForTestingTarget) (visibleForTestingTarget, float64, bool) {
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
	callTarget, ok := semantics.ResolveCallTarget(&api.Context{File: file}, call)
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
				(t.owner == receiver || strings.HasSuffix(t.owner, "."+receiver))
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
