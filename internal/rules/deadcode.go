package rules

import (
	"fmt"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// DeadCodeRule detects public/internal symbols that are never referenced from any other file.
// This is a cross-file rule that requires the CodeIndex to be populated.
type DeadCodeRule struct {
	BaseRule
	// IgnoreCommentReferences: if true (default), a symbol referenced only in comments
	// is still considered dead code. If false, comment references count as usage.
	IgnoreCommentReferences bool
}

// Cross-file and parsed-files rules are identified structurally now —
// callers type-assert to anonymous interfaces describing just the
// CheckCrossFile / CheckParsedFiles method sets. See v2.Rule.Needs
// (NeedsCrossFile / NeedsParsedFiles) for the canonical form.

// DeadCode is advertised as not-fixable because the symbol-deletion
// span (line range, leading KDoc, surrounding blank lines) is
// non-trivial to compute from the cross-file index alone and the
// current Check() path never populates a Fix. Removing a dead symbol
// remains a manual operation. Left in fixes.go history for when the
// per-symbol deletion pipeline lands.
func (r *DeadCodeRule) IsFixable() bool { return false }

// Confidence reports a tier-2 (medium) base confidence. The rule
// relies on the cross-file code index to detect unreferenced symbols
// and then filters framework entry points, overrides, tests, lifecycle
// methods, and DI declarations that are consumed by generated code.
func (r *DeadCodeRule) Confidence() float64 { return 0.75 }

// check runs against the full code index.
func (r *DeadCodeRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	filesByPath := deadCodeFilesByPath(index.Files)

	unused := index.UnusedSymbols(r.IgnoreCommentReferences)
	for _, sym := range unused {
		// Skip common false positives
		if shouldSkipSymbolWithFile(sym, filesByPath[sym.File]) {
			continue
		}

		kindLabel := sym.Kind

		// Check if it's referenced in comments only
		hasCommentRef := false
		if r.IgnoreCommentReferences {
			hasCommentRef = index.IsReferencedOutsideFile(sym.Name, sym.File) &&
				!index.IsReferencedOutsideFileExcludingComments(sym.Name, sym.File)
		}

		var msg string
		if hasCommentRef {
			msg = fmt.Sprintf("%s %s '%s' appears to be unused. It is only referenced in comments, not in code.",
				strings.Title(sym.Visibility), kindLabel, sym.Name)
		} else {
			msg = fmt.Sprintf("%s %s '%s' appears to be unused. It is not referenced from any other file.",
				strings.Title(sym.Visibility), kindLabel, sym.Name)
		}

		f := scanner.Finding{
			File:     sym.File,
			Line:     sym.Line,
			Col:      1,
			RuleSet:  r.RuleSetName,
			Rule:     r.RuleName,
			Severity: r.Sev,
			Message:  msg,
		}

		// Fix: delete the declaration
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   sym.StartByte,
			EndByte:     sym.EndByte,
			Replacement: "",
		}

		ctx.Emit(f)
	}
}

func shouldSkipSymbol(sym scanner.Symbol) bool {
	// Skip overrides (called by framework/parent)
	if sym.IsOverride {
		return true
	}

	// Skip test classes and functions (invoked by test runner, not code)
	if sym.IsTest {
		return true
	}
	if strings.Contains(sym.File, "/test/") || strings.Contains(sym.File, "/androidTest/") ||
		strings.Contains(sym.File, "/benchmark/") || strings.Contains(sym.File, "/canary/") {
		return true
	}

	// Skip companion objects
	if sym.Kind == "object" && sym.Name == "Companion" {
		return true
	}

	// Skip Android lifecycle and framework entry points
	frameworkEntryPoints := map[string]bool{
		"main": true, "onCreate": true, "onDestroy": true, "onStart": true,
		"onStop": true, "onResume": true, "onPause": true, "onCreateView": true,
		"onViewCreated": true, "onBind": true, "onReceive": true, "invoke": true,
		"onAttach": true, "onDetach": true, "onDestroyView": true, "onActivityCreated": true,
		"onCreateOptionsMenu": true, "onOptionsItemSelected": true, "onSaveInstanceState": true,
		"onRestoreInstanceState": true, "onNewIntent": true, "onActivityResult": true,
		"onRequestPermissionsResult": true, "onConfigurationChanged": true,
		"onCreateDialog": true, "onDismiss": true, "onCancel": true,
	}
	if frameworkEntryPoints[sym.Name] {
		return true
	}

	// Skip serialization/reflection hooks
	if sym.Name == "serialVersionUID" || sym.Name == "CREATOR" {
		return true
	}

	// Skip @Preview/@Composable functions (used by Android Studio, not code)
	if strings.Contains(sym.Name, "Preview") {
		return true
	}

	// Skip classes that are likely referenced from XML or framework entry points.
	if isLikelyFrameworkEntryTypeName(sym.Name) {
		return true
	}

	return false
}

func shouldSkipSymbolWithFile(sym scanner.Symbol, file *scanner.File) bool {
	if shouldSkipSymbol(sym) {
		return true
	}
	return deadCodeSymbolHasGeneratedDIUse(sym, file)
}

func deadCodeFilesByPath(files []*scanner.File) map[string]*scanner.File {
	filesByPath := make(map[string]*scanner.File, len(files))
	for _, file := range files {
		if file == nil {
			continue
		}
		filesByPath[file.Path] = file
	}
	return filesByPath
}

func deadCodeSymbolHasGeneratedDIUse(sym scanner.Symbol, file *scanner.File) bool {
	node := deadCodeSymbolNode(sym, file)
	if node == 0 {
		return false
	}
	if deadCodeDeclarationHasDIAnnotation(file, node) {
		return true
	}
	for parent, ok := file.FlatParent(node); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "source_file" {
			break
		}
		if deadCodeDeclarationHasDIAnnotation(file, parent) {
			return true
		}
	}
	return deadCodeSymbolInsideDIAnnotatedContainer(sym, file)
}

func deadCodeSymbolNode(sym scanner.Symbol, file *scanner.File) uint32 {
	if file == nil || file.FlatTree == nil {
		return 0
	}
	wanted := map[string]bool{}
	switch sym.Kind {
	case "function":
		wanted["function_declaration"] = true
	case "class", "interface":
		wanted["class_declaration"] = true
	case "object":
		wanted["object_declaration"] = true
	case "property":
		wanted["property_declaration"] = true
	default:
		return 0
	}
	var found uint32
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found != 0 || !wanted[file.FlatType(idx)] {
			return
		}
		if int(file.FlatStartByte(idx)) == sym.StartByte && int(file.FlatEndByte(idx)) == sym.EndByte {
			found = idx
		}
	})
	return found
}

var deadCodeDIPackages = []string{
	"com.squareup.anvil.annotations",
	"dagger",
	"dagger.hilt",
	"dev.zacsweers.metro",
	"jakarta.inject",
	"javax.inject",
	"me.tatarka.inject.annotations",
}

var deadCodeDIAnnotationNames = []string{
	"AndroidEntryPoint",
	"AssistedFactory",
	"AssistedInject",
	"Binds",
	"BindsInstance",
	"BindsOptionalOf",
	"BindingContainer",
	"Component",
	"ContributesBinding",
	"ContributesMultibinding",
	"ContributesSubcomponent",
	"ContributesTo",
	"DependencyGraph",
	"ElementsIntoSet",
	"EntryPoint",
	"GraphExtension",
	"HiltAndroidApp",
	"Inject",
	"InstallIn",
	"IntoMap",
	"IntoSet",
	"MergeComponent",
	"MergeSubcomponent",
	"Module",
	"Multibinds",
	"Provides",
	"Qualifier",
	"Scope",
	"Subcomponent",
}

func deadCodeDeclarationHasDIAnnotation(file *scanner.File, node uint32) bool {
	if file == nil || node == 0 {
		return false
	}
	text := deadCodeDeclarationAnnotationText(file, node)
	if text == "" || !strings.Contains(text, "@") {
		return false
	}
	if deadCodeTextHasQualifiedDIAnnotation(text) {
		return true
	}
	if !deadCodeFileImportsDIPackage(file) {
		return false
	}
	for _, name := range deadCodeDIAnnotationNames {
		if deadCodeTextHasAnnotationName(text, name) {
			return true
		}
	}
	return false
}

func deadCodeDeclarationAnnotationText(file *scanner.File, node uint32) string {
	var b strings.Builder
	if prev, ok := file.FlatPrevSibling(node); ok {
		switch file.FlatType(prev) {
		case "modifiers", "annotation":
			b.WriteString(file.FlatNodeText(prev))
			b.WriteByte('\n')
		}
	}
	if mods, ok := file.FlatFindChild(node, "modifiers"); ok {
		b.WriteString(file.FlatNodeText(mods))
		b.WriteByte('\n')
	}
	b.WriteString(file.FlatNodeText(node))
	return b.String()
}

func deadCodeTextHasQualifiedDIAnnotation(text string) bool {
	for _, pkg := range deadCodeDIPackages {
		if strings.Contains(text, "@"+pkg+".") {
			return true
		}
	}
	return false
}

func deadCodeFileImportsDIPackage(file *scanner.File) bool {
	if file == nil {
		return false
	}
	content := string(file.Content)
	for _, pkg := range deadCodeDIPackages {
		if fileImportsFQN(file, pkg+".Inject") || fileImportsFQN(file, pkg+".Provides") || fileImportsFQN(file, pkg+".Module") {
			return true
		}
		if strings.Contains(content, "import "+pkg+".") || strings.Contains(content, "import "+pkg+".*") {
			return true
		}
	}
	return false
}

func deadCodeTextHasAnnotationName(text, name string) bool {
	needle := "@" + name
	for searchStart := 0; searchStart < len(text); {
		idx := strings.Index(text[searchStart:], needle)
		if idx < 0 {
			return false
		}
		pos := searchStart + idx
		end := pos + len(needle)
		if end >= len(text) {
			return true
		}
		next := text[end]
		if next == '(' || next == ' ' || next == '\t' || next == '\n' || next == '\r' || next == '.' {
			return true
		}
		searchStart = end
	}
	return false
}

func deadCodeSymbolInsideDIAnnotatedContainer(sym scanner.Symbol, file *scanner.File) bool {
	return deadCodeByteInsideDIAnnotatedContainer(sym.StartByte, file)
}

func deadCodeByteInsideDIAnnotatedContainer(startByte int, file *scanner.File) bool {
	if file == nil || startByte <= 0 || startByte > len(file.Content) {
		return false
	}
	content := string(file.Content)
	searchEnd := startByte
	for pos := strings.Index(content[:searchEnd], "@"); pos >= 0 && pos < searchEnd; {
		annotationStart := pos
		brace := strings.IndexByte(content[annotationStart:], '{')
		if brace < 0 {
			return false
		}
		brace += annotationStart
		if brace >= startByte {
			break
		}
		header := content[annotationStart:brace]
		if deadCodeHeaderLooksLikeDIContainer(file, header) {
			if closeBrace := deadCodeMatchingBrace(content, brace); closeBrace > startByte {
				return true
			}
		}
		nextRel := strings.Index(content[annotationStart+1:searchEnd], "@")
		if nextRel < 0 {
			break
		}
		pos = annotationStart + 1 + nextRel
	}
	return false
}

func deadCodeHeaderLooksLikeDIContainer(file *scanner.File, header string) bool {
	if !strings.Contains(header, "class ") &&
		!strings.Contains(header, "interface ") &&
		!strings.Contains(header, "object ") {
		return false
	}
	if deadCodeTextHasQualifiedDIAnnotation(header) {
		return true
	}
	if !deadCodeFileImportsDIPackage(file) {
		return false
	}
	for _, name := range deadCodeDIAnnotationNames {
		if deadCodeTextHasAnnotationName(header, name) {
			return true
		}
	}
	return false
}

func deadCodeMatchingBrace(content string, open int) int {
	depth := 0
	for i := open; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
