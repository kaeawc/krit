package scanner

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
)

// SuppressionFilter is the single per-file query object that combines
// every suppression source Krit understands:
//   - @Suppress / @SuppressWarnings annotations (byte-range, via SuppressionIndex)
//   - config-level per-rule `excludes` glob patterns
//   - baseline entries (project-level; pointer only, filtering is per-finding
//     and requires the full Finding struct, so callers apply it via
//     FilterByBaseline / FilterColumnsByBaseline)
//   - inline `// krit:ignore[RuleA,RuleB]` line comments (line-scoped)
//
// Built once per file in the Parse phase and cached on File.Suppression.
// The dispatcher, cross-file phase, and any other post-collect filter all
// ask the same filter, so adding a new suppression source is a single
// BuildSuppressionFilter edit instead of four disconnected code paths.
type SuppressionFilter struct {
	file          *File
	annotations   *SuppressionIndex
	excludedRules map[string]bool         // rule IDs whose glob matches this file
	inline        map[int]map[string]bool // line (1-based) → suppressed rule IDs; "" key means all
	baseline      *Baseline
	basePath      string
}

// BuildSuppressionFilter collects every per-file suppression source for
// a single parsed file. baseline and excludes are project-level inputs
// passed in by the caller (the pipeline Parse phase snapshots
// rules.GetAllRuleExcludes() and threads it through); the rest come
// directly from file contents.
//
// Safe to call with nil file; returns a non-nil filter that always
// reports IsSuppressed == false so dispatcher / cross-file call sites
// do not need nil checks.
func BuildSuppressionFilter(file *File, baseline *Baseline, excludes map[string][]string, basePath string) *SuppressionFilter {
	sf := &SuppressionFilter{
		file:     file,
		baseline: baseline,
		basePath: basePath,
	}
	if file == nil {
		return sf
	}
	if file.FlatTree != nil {
		sf.annotations = BuildSuppressionIndexFlat(file.FlatTree, file.Content)
	}
	if len(excludes) > 0 {
		excluded := make(map[string]bool)
		for ruleID, patterns := range excludes {
			if len(patterns) == 0 {
				continue
			}
			if matchAnyExcludePattern(file.Path, patterns) {
				excluded[ruleID] = true
			}
		}
		if len(excluded) > 0 {
			sf.excludedRules = excluded
		}
	}
	sf.inline = parseInlineIgnores(file.Content)
	return sf
}

// IsSuppressed reports whether a finding at (ruleID, ruleSet, line) is
// suppressed by any non-baseline source. Baseline filtering is applied
// separately via FilterByBaseline / FilterColumnsByBaseline because it
// requires the full Finding struct (message + signature).
//
// A nil filter reports false — matches the pre-filter "no suppression
// data available" behaviour.
func (f *SuppressionFilter) IsSuppressed(ruleID, ruleSet string, line int) bool {
	if f == nil {
		return false
	}
	if f.excludedRules[ruleID] {
		return true
	}
	if suppressed := f.inline[line]; suppressed != nil {
		if suppressed[""] || suppressed[ruleID] {
			return true
		}
		if ruleSet != "" && (suppressed[ruleSet+"."+ruleID] || suppressed[ruleSet+":"+ruleID]) {
			return true
		}
	}
	if f.annotations != nil && f.file != nil {
		byteOffset := 0
		if line > 0 {
			byteOffset = f.file.LineOffset(line - 1)
		}
		if f.annotations.IsSuppressed(byteOffset, ruleID, ruleSet) {
			return true
		}
	}
	return false
}

// IsFileExcluded reports whether the given rule is globally excluded
// for this file via config globs. Used by the dispatcher to skip rule
// execution entirely rather than filtering findings after the fact.
func (f *SuppressionFilter) IsFileExcluded(ruleID string) bool {
	if f == nil {
		return false
	}
	return f.excludedRules[ruleID]
}

// Annotations exposes the underlying @Suppress index so compat callers
// (legacy tests, the File.SuppressionIdx shim) can reuse the same data
// without rebuilding.
func (f *SuppressionFilter) Annotations() *SuppressionIndex {
	if f == nil {
		return nil
	}
	return f.annotations
}

// Baseline returns the project-level baseline the filter was built
// against, or nil if none was configured.
func (f *SuppressionFilter) Baseline() *Baseline {
	if f == nil {
		return nil
	}
	return f.baseline
}

func matchAnyExcludePattern(filePath string, patterns []string) bool {
	filePath = filepath.ToSlash(filePath)
	for _, p := range patterns {
		if matchExcludePatternSlash(filePath, p) {
			return true
		}
	}
	return false
}

// matchExcludePatternSlash mirrors rules.matchExcludePattern but is kept
// in-package so scanner avoids an import cycle. Semantics:
//   - **/dir/**   matches any path containing /dir/
//   - **/*suffix  matches any path ending with suffix
//   - **/name     matches any path ending with /name
//   - plain glob  matches the basename
func matchExcludePatternSlash(filePath, pattern string) bool {
	pattern = filepath.ToSlash(pattern)
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		if strings.Contains(suffix, "/**") {
			inner := strings.TrimSuffix(suffix, "/**")
			return strings.Contains(filePath, "/"+inner+"/")
		}
		if strings.HasPrefix(suffix, "*") {
			return strings.HasSuffix(filePath, suffix[1:])
		}
		return strings.HasSuffix(filePath, "/"+suffix) || filePath == suffix
	}
	matched, _ := filepath.Match(pattern, filepath.Base(filePath))
	return matched
}

// parseInlineIgnores scans for `// krit:ignore[...]` and
// `// krit:ignore-all` comments. Returns a line → {ruleID: true} map,
// with the empty-string key signalling "suppress all rules on this line".
// Returns nil when no inline suppressions are present so the common case
// allocates nothing beyond a single byte scan.
func parseInlineIgnores(content []byte) map[int]map[string]bool {
	if !bytes.Contains(content, []byte("krit:ignore")) {
		return nil
	}
	out := make(map[int]map[string]bool)
	line := 1
	start := 0
	for i := 0; i <= len(content); i++ {
		if i < len(content) && content[i] != '\n' {
			continue
		}
		lineBytes := content[start:i]
		addInlineIgnore(out, line, lineBytes)
		line++
		start = i + 1
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func addInlineIgnore(out map[int]map[string]bool, line int, lineBytes []byte) {
	idx := bytes.Index(lineBytes, []byte("krit:ignore"))
	if idx < 0 {
		return
	}
	// Require a preceding `//` on the same line (ignore in strings/etc.)
	if !bytes.Contains(lineBytes[:idx], []byte("//")) {
		return
	}
	rest := lineBytes[idx+len("krit:ignore"):]
	rest = bytes.TrimLeft(rest, "-")
	if bytes.HasPrefix(rest, []byte("all")) {
		set := ensureInline(out, line)
		set[""] = true
		return
	}
	if !bytes.HasPrefix(rest, []byte("[")) {
		// Bare `// krit:ignore` → suppress all on this line.
		set := ensureInline(out, line)
		set[""] = true
		return
	}
	end := bytes.IndexByte(rest, ']')
	if end < 0 {
		return
	}
	body := rest[1:end]
	set := ensureInline(out, line)
	for _, tok := range bytes.Split(body, []byte(",")) {
		name := strings.TrimSpace(string(tok))
		if name == "" {
			continue
		}
		set[name] = true
	}
}

func ensureInline(out map[int]map[string]bool, line int) map[string]bool {
	if s := out[line]; s != nil {
		return s
	}
	s := make(map[string]bool, 2)
	out[line] = s
	return s
}

// Suppression represents a range of bytes where specific rules are suppressed.
type Suppression struct {
	StartByte int
	EndByte   int
	Rules     map[string]bool // rule names that are suppressed; nil = suppress all
}

// SuppressionIndex provides O(log n) lookup for whether a finding is suppressed.
type SuppressionIndex struct {
	suppressions []Suppression // sorted by StartByte
}

var suppressionNeedles = [][]byte{
	[]byte("Suppress("),
	[]byte("SuppressWarnings("),
}

// BuildSuppressionIndexFlat walks the flat tree once to find all
// @Suppress/@SuppressWarnings annotations and builds an index of suppressed
// byte ranges.
func BuildSuppressionIndexFlat(tree *FlatTree, content []byte) *SuppressionIndex {
	idx := &SuppressionIndex{
		suppressions: make([]Suppression, 0, 16),
	}
	if tree == nil || len(tree.Nodes) == 0 {
		return idx
	}
	flatWalkForSuppressions(tree, 0, content, idx)
	sort.Slice(idx.suppressions, func(i, j int) bool {
		return idx.suppressions[i].StartByte < idx.suppressions[j].StartByte
	})
	return idx
}

// IsSuppressed checks if a finding at the given byte offset is suppressed for the given rule.
func (idx *SuppressionIndex) IsSuppressed(byteOffset int, ruleName string, ruleSetName string) bool {
	if len(idx.suppressions) == 0 {
		return false
	}

	// Binary search for the rightmost suppression starting at or before byteOffset
	i := sort.Search(len(idx.suppressions), func(i int) bool {
		return idx.suppressions[i].StartByte > byteOffset
	})

	// Check all suppressions that could contain this offset (walk backwards)
	for j := i - 1; j >= 0; j-- {
		s := idx.suppressions[j]
		if s.EndByte < byteOffset {
			break // past this suppression's range
		}
		if byteOffset >= s.StartByte && byteOffset <= s.EndByte {
			if suppressionMatches(s.Rules, ruleName, ruleSetName) {
				return true
			}
		}
	}
	return false
}

// kotlinCompilerWarningAliases maps Kotlin compiler warning identifiers
// (used in `@Suppress("FOO")`) to the krit rule names that detect the
// same issue. Without this, Signal-style `@Suppress("UNUSED_PARAMETER")`
// suppressions would be ignored by krit.
var kotlinCompilerWarningAliases = map[string][]string{
	"UNUSED_PARAMETER":           {"UnusedParameter"},
	"UNUSED_VARIABLE":            {"UnusedVariable"},
	"UNUSED_EXPRESSION":          {"UnusedPrivateProperty"},
	"UNUSED_ANONYMOUS_PARAMETER": {"UnusedParameter"},
	"NAME_SHADOWING":             {"NoNameShadowing"},
	"UNCHECKED_CAST":             {"UnsafeCast"},
	"UNUSED_VALUE":               {"UnusedPrivateProperty"},
	"UNUSED_IMPORT":              {"UnusedImport"},
	// IntelliJ/Kotlin inspection id (lowercase) used by Signal via
	// `@Suppress("unused")`. Covers all unused-declaration krit rules.
	"unused": {"UnusedParameter", "UnusedPrivateProperty", "UnusedPrivateMember", "UnusedPrivateFunction", "UnusedPrivateClass", "UnusedVariable", "UnusedImport"},

	// IntelliJ Kotlin inspection IDs used in `@Suppress(...)` for naming
	// conventions. These pair up with krit's naming rules.
	"EnumEntryName":     {"EnumNaming"},
	"ClassName":         {"ClassNaming"},
	"FunctionName":      {"FunctionNaming"},
	"PropertyName":      {"TopLevelPropertyNaming", "ObjectPropertyNaming"},
	"PrivatePropertyName": {"TopLevelPropertyNaming", "ObjectPropertyNaming"},
	"ObjectPropertyName":  {"ObjectPropertyNaming"},
	"LocalVariableName":   {"VariableNaming"},
	"ConstPropertyName":   {"TopLevelPropertyNaming", "ObjectPropertyNaming"},
}

func suppressionMatches(rules map[string]bool, ruleName string, ruleSetName string) bool {
	if rules == nil {
		return true // suppress all
	}
	if rules[ruleName] || rules["all"] || rules["ALL"] || rules["All"] {
		return true
	}
	if ruleSetName != "" {
		if rules[ruleSetName+"."+ruleName] || rules[ruleSetName+":"+ruleName] {
			return true
		}
	}
	if rules["detekt."+ruleName] || rules["detekt:"+ruleName] {
		return true
	}
	// Check Kotlin compiler warning aliases.
	for alias, mapped := range kotlinCompilerWarningAliases {
		if !rules[alias] {
			continue
		}
		for _, m := range mapped {
			if m == ruleName {
				return true
			}
		}
	}
	return false
}

func flatWalkForSuppressions(tree *FlatTree, idx uint32, content []byte, out *SuppressionIndex) {
	if tree == nil || int(idx) >= len(tree.Nodes) {
		return
	}
	nodeType := tree.Nodes[idx].TypeName()

	switch nodeType {
	case "prefix_expression", "annotation":
		if flatProcessSuppressionNode(tree, idx, content, out) {
			return
		}
		return
	case "file_annotation":
		flatProcessFileAnnotation(tree, idx, content, out)
		return
	case "source_file", "class_body", "statements", "function_body", "modifiers",
		"class_declaration", "function_declaration", "property_declaration",
		"object_declaration", "companion_object", "secondary_constructor",
		"primary_constructor", "anonymous_initializer", "enum_entry", "lambda_literal",
		"class_member_declarations", "block",
		"control_structure_body", "catch_block", "finally_block",
		"call_expression", "annotated_lambda", "call_suffix",
		"if_expression", "when_expression", "when_entry", "try_expression":
	default:
		return
	}

	for child := tree.Nodes[idx].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
		switch tree.Nodes[child].TypeName() {
		case "prefix_expression", "annotation":
			flatProcessSuppressionNode(tree, child, content, out)
		case "file_annotation":
			flatProcessFileAnnotation(tree, child, content, out)
		case "modifiers", "source_file", "class_body", "statements", "function_body",
			"class_declaration", "function_declaration", "property_declaration",
			"object_declaration", "companion_object", "secondary_constructor",
			"primary_constructor", "anonymous_initializer", "enum_entry",
			"class_member_declarations", "block",
			"lambda_literal", "control_structure_body", "catch_block", "finally_block",
			"call_expression", "annotated_lambda", "call_suffix",
			"if_expression", "when_expression", "when_entry", "try_expression":
			flatWalkForSuppressions(tree, child, content, out)
		}
	}
}

func flatProcessFileAnnotation(tree *FlatTree, idx uint32, content []byte, out *SuppressionIndex) {
	textBytes := FlatNodeBytes(tree, idx, content)
	if !hasSuppressionNeedle(textBytes) {
		return
	}
	rules := extractSuppressedRules(string(textBytes))
	target := idx
	for parent := tree.Nodes[target].Parent; parent != 0; parent = tree.Nodes[target].Parent {
		target = parent
	}
	out.suppressions = append(out.suppressions, Suppression{
		StartByte: int(tree.Nodes[target].StartByte),
		EndByte:   int(tree.Nodes[target].EndByte),
		Rules:     rules,
	})
}

func flatProcessSuppressionNode(tree *FlatTree, idx uint32, content []byte, out *SuppressionIndex) bool {
	textBytes := FlatNodeBytes(tree, idx, content)
	if !hasSuppressionNeedle(textBytes) {
		return false
	}
	parseText := textBytes
	if tree.Nodes[idx].TypeName() == "prefix_expression" {
		for child := tree.Nodes[idx].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
			if tree.Nodes[child].TypeName() == "annotation" && flatAnnotationHasArgs(tree, child) {
				parseText = FlatNodeBytes(tree, child, content)
				break
			}
		}
	}
	rules := extractSuppressedRules(string(parseText))
	target := flatFindAnnotationTarget(tree, idx)
	if target == 0 && idx != 0 {
		return true
	}
	out.suppressions = append(out.suppressions, Suppression{
		StartByte: int(tree.Nodes[target].StartByte),
		EndByte:   int(tree.Nodes[target].EndByte),
		Rules:     rules,
	})
	return true
}

func flatAnnotationHasArgs(tree *FlatTree, idx uint32) bool {
	for child := tree.Nodes[idx].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
		if tree.Nodes[child].TypeName() == "constructor_invocation" {
			return true
		}
	}
	return false
}

func flatFindAnnotationTarget(tree *FlatTree, idx uint32) uint32 {
	if tree.Nodes[idx].TypeName() == "prefix_expression" {
		var annChild uint32
		hasOperand := false
		for child := tree.Nodes[idx].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
			if tree.Nodes[child].TypeName() == "annotation" {
				annChild = child
			} else {
				hasOperand = true
			}
		}
		if annChild != 0 && hasOperand && flatAnnotationHasArgs(tree, annChild) {
			return idx
		}
	}

	parent := tree.Nodes[idx].Parent
	if idx == 0 && parent == 0 {
		return 0
	}
	if tree.Nodes[parent].TypeName() == "modifiers" {
		return tree.Nodes[parent].Parent
	}
	parentType := tree.Nodes[parent].TypeName()
	if parentType == "source_file" || parentType == "statements" || parentType == "class_body" {
		foundSelf := false
		for child := tree.Nodes[parent].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
			if child == idx {
				foundSelf = true
				continue
			}
			if !foundSelf {
				continue
			}
			childType := tree.Nodes[child].TypeName()
			if childType == "prefix_expression" || childType == "annotation" {
				continue
			}
			return child
		}
		return parent
	}
	return parent
}

func hasSuppressionNeedle(text []byte) bool {
	for _, needle := range suppressionNeedles {
		if bytes.Contains(text, needle) {
			return true
		}
	}
	return false
}

// extractSuppressedRules parses rule names from @Suppress("Rule1", "Rule2") or @SuppressWarnings("...")
func extractSuppressedRules(text string) map[string]bool {
	// Find the arguments between ( and )
	start := strings.Index(text, "(")
	end := strings.LastIndex(text, ")")
	if start < 0 || end <= start {
		return nil
	}
	args := text[start+1 : end]

	rules := make(map[string]bool)
	for _, arg := range strings.Split(args, ",") {
		arg = strings.TrimSpace(arg)
		// Remove quotes
		arg = strings.Trim(arg, "\"'")
		// Remove detekt prefix
		arg = strings.TrimPrefix(arg, "detekt.")
		arg = strings.TrimPrefix(arg, "detekt:")
		if arg != "" {
			rules[arg] = true
		}
	}

	// Special case: "all" suppresses everything
	if rules["all"] || rules["ALL"] || rules["All"] {
		return nil // nil means suppress all
	}

	return rules
}
