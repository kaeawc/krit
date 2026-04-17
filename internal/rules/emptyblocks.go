package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
	"regexp"
	"strings"
)

// nodeLineRange returns the start and end byte offsets that cover the full line(s)
// of a node, including leading whitespace and the trailing newline. Useful for
// byte-mode deletion that should remove whole lines.
func nodeLineRange(content []byte, startByte, endByte int) (int, int) {
	s := startByte
	for s > 0 && content[s-1] != '\n' {
		s--
	}
	e := endByte
	if e < len(content) && content[e] == '\n' {
		e++
	}
	return s, e
}

// detectIndent returns the whitespace indentation at the line containing the given byte offset.
func detectIndent(content []byte, byteOffset int) string {
	// Walk backwards to find the start of the line
	lineStart := byteOffset
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	// Collect leading whitespace
	var indent []byte
	for i := lineStart; i < len(content) && (content[i] == ' ' || content[i] == '\t'); i++ {
		indent = append(indent, content[i])
	}
	return string(indent)
}

// stripComments removes line comments and block comments from a string.
func stripComments(s string) string {
	// Remove block comments
	blockRe := regexp.MustCompile(`(?s)/\*.*?\*/`)
	s = blockRe.ReplaceAllString(s, "")
	// Remove line comments
	lineRe := regexp.MustCompile(`//[^\n]*`)
	s = lineRe.ReplaceAllString(s, "")
	return s
}

func isBlockEmptyFlat(file *scanner.File, idx uint32) bool {
	text := file.FlatNodeText(idx)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return true
	}
	body := strings.TrimSpace(text[start+1 : end])
	cleaned := stripComments(body)
	return strings.TrimSpace(cleaned) == ""
}

// EmptyCatchBlockRule detects catch blocks with empty body.
type EmptyCatchBlockRule struct {
	FlatDispatchBase
	BaseRule
	AllowedExceptionNameRegex *regexp.Regexp // exception names matching this are allowed to be empty
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyCatchBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyCatchBlockRule) NodeTypes() []string { return []string{"catch_block"} }

func (r *EmptyCatchBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isBlockEmptyFlat(file, idx) {
		return nil
	}
	// Skip if the caught exception name matches the allowed regex
	if r.AllowedExceptionNameRegex != nil {
		caughtVar := extractCaughtVarNameFlat(file, idx)
		if caughtVar != "" && r.AllowedExceptionNameRegex.MatchString(caughtVar) {
			return nil
		}
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Empty catch block detected. Empty catch blocks should be avoided.")
	// Find the { } of the catch block body and insert a TODO comment
	nodeText := file.FlatNodeText(idx)
	braceStart := strings.Index(nodeText, "{")
	braceEnd := strings.LastIndex(nodeText, "}")
	if braceStart >= 0 && braceEnd > braceStart {
		indent := detectIndent(file.Content, int(file.FlatStartByte(idx)))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(idx)) + braceStart,
			EndByte:     int(file.FlatStartByte(idx)) + braceEnd + 1,
			Replacement: "{\n" + indent + "    // TODO: handle exception\n" + indent + "}",
		}
	}
	return []scanner.Finding{f}
}

// EmptyClassBlockRule detects classes with empty body.
type EmptyClassBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyClassBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyClassBlockRule) NodeTypes() []string { return []string{"class_body"} }

func (r *EmptyClassBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip test files and fixture directories — `class Foo {}` marker
	// classes are a common testData convention and aren't real code.
	if isTestFile(file.Path) {
		return nil
	}
	// Skip anonymous object expressions — `object : Foo {}` and
	// `object : Bar() {}` require the braces to exist as a valid expression,
	// even if the body is empty. Removing them is a compile error.
	if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "object_literal" {
		return nil
	}
	text := file.FlatNodeText(idx)
	// Strip { and } and check contents
	inner := strings.TrimSpace(text)
	if len(inner) >= 2 && inner[0] == '{' && inner[len(inner)-1] == '}' {
		body := strings.TrimSpace(inner[1 : len(inner)-1])
		cleaned := stripComments(body)
		if strings.TrimSpace(cleaned) == "" {
			f := r.Finding(file, file.FlatRow(idx)+1, 1,
				"Empty class body detected. Consider removing the empty braces.")
			// Remove the empty class_body node and any preceding whitespace
			startByte := int(file.FlatStartByte(idx))
			for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '\t') {
				startByte--
			}
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   startByte,
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: "",
			}
			return []scanner.Finding{f}
		}
	}
	return nil
}

// EmptyDefaultConstructorRule detects explicit empty default constructors.
type EmptyDefaultConstructorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyDefaultConstructorRule) Confidence() float64 { return 0.95 }

func (r *EmptyDefaultConstructorRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *EmptyDefaultConstructorRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip `annotation class Foo()` — annotation classes idiomatically
	// include `()` to signal "no parameters" and Kotlin requires the
	// parentheses in some tooling contexts.
	if file.FlatHasModifier(idx, "annotation") {
		return nil
	}
	// Look for primary_constructor child
	ctor := file.FlatFindChild(idx, "primary_constructor")
	if ctor == 0 {
		return nil
	}
	ctorText := file.FlatNodeText(ctor)
	// Check for empty parameter list: constructor() or just ()
	emptyCtorRe := regexp.MustCompile(`constructor\s*\(\s*\)`)
	emptyParenRe := regexp.MustCompile(`^\s*\(\s*\)\s*$`)
	if !emptyCtorRe.MatchString(ctorText) && !emptyParenRe.MatchString(ctorText) {
		return nil
	}
	// If the constructor has visibility modifiers or annotations, the empty ()
	// is load-bearing — removing it would also remove the modifier/annotation.
	// Common pattern: `private constructor()` to enforce factory-only construction.
	if file.FlatHasModifier(ctor, "private") ||
		file.FlatHasModifier(ctor, "internal") ||
		file.FlatHasModifier(ctor, "protected") ||
		file.FlatHasModifier(ctor, "public") {
		return nil
	}
	// Skip if the constructor has any annotations attached.
	if mods := file.FlatFindChild(ctor, "modifiers"); mods != 0 && file.FlatFindChild(mods, "annotation") != 0 {
		return nil
	}
	f := r.Finding(file, file.FlatRow(ctor)+1, 1,
		"Empty default constructor detected. It can be removed.")
	// Remove the constructor text (and any preceding whitespace on the same line)
	startByte := int(file.FlatStartByte(ctor))
	// Walk back to remove preceding whitespace (space before "constructor")
	for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '\t') {
		startByte--
	}
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   startByte,
		EndByte:     int(file.FlatEndByte(ctor)),
		Replacement: "",
	}
	return []scanner.Finding{f}
}

// EmptyDoWhileBlockRule detects do-while loops with empty body.
type EmptyDoWhileBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyDoWhileBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyDoWhileBlockRule) NodeTypes() []string { return []string{"do_while_statement"} }

func (r *EmptyDoWhileBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isBlockEmptyFlat(file, idx) {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Empty do-while block detected.")
	doS, doE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   doS,
		EndByte:     doE,
		Replacement: "",
	}
	return []scanner.Finding{f}
}

// EmptyElseBlockRule detects else blocks with empty body.
type EmptyElseBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyElseBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyElseBlockRule) NodeTypes() []string { return []string{"if_expression"} }

func (r *EmptyElseBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	// Find the else keyword
	elseIdx := strings.LastIndex(text, "else")
	if elseIdx < 0 {
		return nil
	}
	afterElse := text[elseIdx+4:]
	braceStart := strings.Index(afterElse, "{")
	if braceStart < 0 {
		return nil
	}
	braceEnd := strings.Index(afterElse[braceStart:], "}")
	if braceEnd < 0 {
		return nil
	}
	body := afterElse[braceStart+1 : braceStart+braceEnd]
	cleaned := stripComments(body)
	if strings.TrimSpace(cleaned) == "" {
		// Calculate the line of the else keyword
		beforeElse := text[:elseIdx]
		elseLine := file.FlatRow(idx) + strings.Count(beforeElse, "\n") + 1
		f := r.Finding(file, elseLine, 1,
			"Empty else block detected.")
		// Remove from "else" keyword through closing "}" of else block
		elseByteStart := int(file.FlatStartByte(idx)) + elseIdx
		// Walk back to remove preceding whitespace/newline before "else"
		for elseByteStart > 0 && (file.Content[elseByteStart-1] == ' ' || file.Content[elseByteStart-1] == '\t' || file.Content[elseByteStart-1] == '\n' || file.Content[elseByteStart-1] == '\r') {
			elseByteStart--
		}
		elseByteEnd := int(file.FlatStartByte(idx)) + elseIdx + 4 + braceStart + braceEnd + 1
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   elseByteStart,
			EndByte:     elseByteEnd,
			Replacement: "",
		}
		return []scanner.Finding{f}
	}
	return nil
}

// EmptyFinallyBlockRule detects finally blocks with empty body.
type EmptyFinallyBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyFinallyBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyFinallyBlockRule) NodeTypes() []string { return []string{"finally_block"} }

func (r *EmptyFinallyBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isBlockEmptyFlat(file, idx) {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Empty finally block detected.")
	// Remove the entire finally block
	startByte := int(file.FlatStartByte(idx))
	for startByte > 0 && (file.Content[startByte-1] == ' ' || file.Content[startByte-1] == '\t' || file.Content[startByte-1] == '\n' || file.Content[startByte-1] == '\r') {
		startByte--
	}
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   startByte,
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: "",
	}
	return []scanner.Finding{f}
}

// EmptyForBlockRule detects for loops with empty body.
type EmptyForBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyForBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyForBlockRule) NodeTypes() []string { return []string{"for_statement"} }

func (r *EmptyForBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Find the control_structure_body (the loop's body node). If the body
	// is a single statement without braces, it's NOT empty — it just has
	// no braces. Only flag when there's an actual empty block { }.
	body := file.FlatFindChild(idx, "control_structure_body")
	if body == 0 {
		return nil
	}
	bodyText := file.FlatNodeText(body)
	if !strings.Contains(bodyText, "{") {
		return nil // single-statement body, not a block
	}
	if !isBlockEmptyFlat(file, body) {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Empty for block detected.")
	forS, forE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   forS,
		EndByte:     forE,
		Replacement: "",
	}
	return []scanner.Finding{f}
}

// EmptyFunctionBlockRule detects functions with empty body.
type EmptyFunctionBlockRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreOverridden bool // if true, skip override functions with empty body
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyFunctionBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyFunctionBlockRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *EmptyFunctionBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip override/open functions — empty body is an intentional no-op
	// implementation or a subclass extension hook.
	if file.FlatHasModifier(idx, "override") || file.FlatHasModifier(idx, "open") {
		return nil
	}
	// Skip JSR-330/Dagger method-injection points: `@Inject fun foo(dep: T) {}`.
	// The empty body is the point — Dagger calls the method to trigger injection.
	// Also skip DI provider annotations where an empty body would be unusual but
	// still part of the DI graph contract.
	if HasIgnoredAnnotation(file.FlatNodeText(idx),
		[]string{"Inject", "Provides", "Binds", "BindsInstance",
			"BindsOptionalOf", "IntoSet", "IntoMap", "ElementsIntoSet",
			"Multibinds", "ContributesBinding", "ContributesMultibinding",
			"ContributesTo", "ContributesSubcomponent"}) {
		return nil
	}
	// Skip functions inside an interface — empty bodies are valid
	// default implementations allowing selective overrides.
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "class_declaration" {
			break
		}
		if file.FlatType(p) == "interface" {
			return nil
		}
	}
	// Also skip when the enclosing class_declaration is declared as an
	// interface (tree-sitter represents interfaces as class_declaration
	// with an `interface` keyword child).
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "class_declaration" {
			continue
		}
		for i := 0; i < file.FlatChildCount(p); i++ {
			if file.FlatType(file.FlatChild(p, i)) == "interface" {
				return nil
			}
		}
		break
	}
	// Find function_body child
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}
	// Expression-body functions (fun foo() = expr) have a function_body
	// that contains "=" but no braces — skip those.
	bodyText := file.FlatNodeText(body)
	if !strings.Contains(bodyText, "{") {
		return nil
	}
	if !isBlockEmptyFlat(file, body) {
		return nil
	}
	// Skip bodies whose only content is a comment — author intent is
	// clearly "intentionally empty, see comment".
	inner := bodyText
	if i := strings.Index(inner, "{"); i >= 0 {
		inner = inner[i+1:]
	}
	if j := strings.LastIndex(inner, "}"); j >= 0 {
		inner = inner[:j]
	}
	trimmedInner := strings.TrimSpace(inner)
	if strings.HasPrefix(trimmedInner, "//") || strings.HasPrefix(trimmedInner, "/*") ||
		strings.Contains(trimmedInner, "TODO") {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Empty function body detected.")
	braceStart := strings.Index(bodyText, "{")
	braceEnd := strings.LastIndex(bodyText, "}")
	if braceStart >= 0 && braceEnd > braceStart {
		indent := detectIndent(file.Content, int(file.FlatStartByte(body)))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(body)) + braceStart,
			EndByte:     int(file.FlatStartByte(body)) + braceEnd + 1,
			Replacement: "{\n" + indent + "    TODO(\"Not yet implemented\")\n" + indent + "}",
		}
	}
	return []scanner.Finding{f}
}

// EmptyIfBlockRule detects if blocks with empty body.
type EmptyIfBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyIfBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyIfBlockRule) NodeTypes() []string { return []string{"if_expression"} }

func (r *EmptyIfBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	// Find the if condition close paren, then the block
	// We need the first { ... } after the condition, before any else
	condEnd := strings.Index(text, ")")
	if condEnd < 0 {
		return nil
	}
	rest := text[condEnd+1:]
	// Strip text after else keyword for the if-body check
	elseIdx := strings.Index(rest, "else")
	ifPart := rest
	if elseIdx >= 0 {
		ifPart = rest[:elseIdx]
	}
	braceStart := strings.Index(ifPart, "{")
	if braceStart < 0 {
		return nil
	}
	braceEnd := strings.LastIndex(ifPart, "}")
	if braceEnd <= braceStart {
		return nil
	}
	body := ifPart[braceStart+1 : braceEnd]
	cleaned := stripComments(body)
	if strings.TrimSpace(cleaned) == "" {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Empty if block detected.")
		ifS, ifE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   ifS,
			EndByte:     ifE,
			Replacement: "",
		}
		return []scanner.Finding{f}
	}
	return nil
}

// EmptyInitBlockRule detects init blocks with empty body.
type EmptyInitBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyInitBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyInitBlockRule) NodeTypes() []string { return []string{"anonymous_initializer"} }

func (r *EmptyInitBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isBlockEmptyFlat(file, idx) {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Empty init block detected.")
	initS, initE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   initS,
		EndByte:     initE,
		Replacement: "",
	}
	return []scanner.Finding{f}
}

// EmptyKotlinFileRule detects files with no meaningful code.
type EmptyKotlinFileRule struct {
	LineBase
	BaseRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — the check walks the AST root for any non-package/import/
// comment child, which is a precise structural determination.
func (r *EmptyKotlinFileRule) Confidence() float64 { return 0.95 }

func (r *EmptyKotlinFileRule) CheckLines(file *scanner.File) []scanner.Finding {
	// Skip Spotless / format-tool template files (e.g. spotless/copyright.kt
	// spotless/copyright.kts). These are copyright-header templates with a
	// .kt/.kts extension for syntax highlighting, not real Kotlin source.
	if isSpotlessTemplateFile(file.Path) {
		return nil
	}
	if file == nil || file.FlatTree == nil {
		return nil
	}
	for i := 0; i < file.FlatChildCount(0); i++ {
		child := file.FlatChild(0, i)
		t := file.FlatType(child)
		// Skip package, imports, and comments
		if t == "package_header" || t == "import_header" || t == "import_list" ||
			t == "line_comment" || t == "multiline_comment" {
			continue
		}
		// Any other node means the file has content
		return nil
	}
	return []scanner.Finding{r.Finding(file, 1, 1, "Empty Kotlin file detected.")}
}

// isSpotlessTemplateFile reports whether the path looks like a Spotless
// copyright/license template — these files have a .kt or .kts extension
// but are template inputs for the Spotless formatter plugin, not source.
func isSpotlessTemplateFile(path string) bool {
	p := strings.ReplaceAll(path, "\\", "/")
	// Directory markers.
	if strings.Contains(p, "/spotless/") {
		base := strings.ToLower(filepathBase(p))
		if strings.HasPrefix(base, "copyright.") ||
			strings.HasPrefix(base, "license.") ||
			strings.HasPrefix(base, "header.") {
			return true
		}
	}
	return false
}

// filepathBase is a stdlib-free path basename helper to keep this file's
// imports unchanged.
func filepathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// EmptySecondaryConstructorRule detects secondary constructors with empty body.
type EmptySecondaryConstructorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptySecondaryConstructorRule) Confidence() float64 { return 0.95 }

func (r *EmptySecondaryConstructorRule) NodeTypes() []string {
	return []string{"secondary_constructor"}
}

func (r *EmptySecondaryConstructorRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	nodeText := file.FlatNodeText(idx)
	// A delegation constructor without a body (e.g. "constructor(x: Int) : this(x, 0)")
	// has no braces at all — it is NOT empty, it delegates. Skip it.
	if !strings.Contains(nodeText, "{") {
		return nil
	}
	if !isBlockEmptyFlat(file, idx) {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Empty secondary constructor detected.")
	// Remove the empty body { } from the constructor
	braceStart := strings.LastIndex(nodeText, "{")
	if braceStart >= 0 {
		// Remove from the opening brace (and preceding whitespace) to end
		removStart := int(file.FlatStartByte(idx)) + braceStart
		// Walk back to remove preceding whitespace
		for removStart > int(file.FlatStartByte(idx)) && (file.Content[removStart-1] == ' ' || file.Content[removStart-1] == '\t') {
			removStart--
		}
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   removStart,
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: "",
		}
	}
	return []scanner.Finding{f}
}

// EmptyTryBlockRule detects try blocks with empty body.
type EmptyTryBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyTryBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyTryBlockRule) NodeTypes() []string { return []string{"try_expression"} }

func (r *EmptyTryBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	// The try body is between 'try' and the first matching '}'
	tryIdx := strings.Index(text, "try")
	if tryIdx < 0 {
		return nil
	}
	afterTry := text[tryIdx+3:]
	braceStart := strings.Index(afterTry, "{")
	if braceStart < 0 {
		return nil
	}
	// Find matching closing brace
	depth := 0
	braceEnd := -1
	for i := braceStart; i < len(afterTry); i++ {
		if afterTry[i] == '{' {
			depth++
		} else if afterTry[i] == '}' {
			depth--
			if depth == 0 {
				braceEnd = i
				break
			}
		}
	}
	if braceEnd < 0 {
		return nil
	}
	body := afterTry[braceStart+1 : braceEnd]
	cleaned := stripComments(body)
	if strings.TrimSpace(cleaned) == "" {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Empty try block detected.")
		tryS, tryE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   tryS,
			EndByte:     tryE,
			Replacement: "",
		}
		return []scanner.Finding{f}
	}
	return nil
}

// EmptyWhenBlockRule detects when expressions with empty body.
type EmptyWhenBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyWhenBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyWhenBlockRule) NodeTypes() []string { return []string{"when_expression"} }

func (r *EmptyWhenBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check if when has any when_entry children
	hasEntries := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		if file.FlatType(file.FlatChild(idx, i)) == "when_entry" {
			hasEntries = true
			break
		}
	}
	if hasEntries {
		return nil
	}
	// No entries means empty when block
	if isBlockEmptyFlat(file, idx) {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Empty when block detected.")
		whenS, whenE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   whenS,
			EndByte:     whenE,
			Replacement: "",
		}
		return []scanner.Finding{f}
	}
	return nil
}

// EmptyWhileBlockRule detects while loops with empty body.
type EmptyWhileBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyWhileBlockRule) Confidence() float64 { return 0.95 }

func (r *EmptyWhileBlockRule) NodeTypes() []string { return []string{"while_statement"} }

func (r *EmptyWhileBlockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isBlockEmptyFlat(file, idx) {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		"Empty while block detected.")
	whileS, whileE := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   whileS,
		EndByte:     whileE,
		Replacement: "",
	}
	return []scanner.Finding{f}
}
