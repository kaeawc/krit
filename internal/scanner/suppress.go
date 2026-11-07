package scanner

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// SuppressionFilter is the single per-file query object that combines
// every suppression source Krit understands:
//   - @Suppress / @SuppressWarnings / @SuppressLint annotations (byte-range, via SuppressionIndex)
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
	// ruleAliases maps a canonical rule ID to its legacy/alternate IDs.
	// Populated lazily by callers via WithRuleAliases. When set, an
	// @Suppress / inline-ignore listing any alias silences findings
	// emitted under the canonical ID. nil disables alias matching.
	ruleAliases map[string][]string
}

// WithRuleAliases attaches an alias index to the filter and returns it
// for chaining. Map key is a canonical rule ID; value is the list of
// alternate IDs (legacy names, renames) that should also suppress the
// canonical rule when written in @Suppress / krit:ignore.
//
// Safe on a nil receiver (returns nil). Callers without alias data
// simply omit the call — alias matching is purely additive.
func (f *SuppressionFilter) WithRuleAliases(aliases map[string][]string) *SuppressionFilter {
	if f == nil {
		return nil
	}
	if len(aliases) == 0 {
		f.ruleAliases = nil
		return f
	}
	f.ruleAliases = aliases
	return f
}

// WithExcludedRules suppresses every finding of the listed rule IDs in this
// file, like a config `excludes` glob that matches the file. The set is shared,
// not copied, and must not be modified afterwards. Safe on a nil receiver.
func (f *SuppressionFilter) WithExcludedRules(ids map[string]bool) *SuppressionFilter {
	if f == nil || len(ids) == 0 {
		return f
	}
	if len(f.excludedRules) == 0 {
		f.excludedRules = ids
		return f
	}
	merged := make(map[string]bool, len(f.excludedRules)+len(ids))
	for id := range f.excludedRules {
		merged[id] = true
	}
	for id := range ids {
		merged[id] = true
	}
	f.excludedRules = merged
	return f
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
	return f.IsSuppressedAt(ruleID, ruleSet, line, -1)
}

// IsSuppressedAt is IsSuppressed for a finding whose start byte is known
// (byteOffset >= 0). An @Suppress / @SuppressWarnings annotation whose
// declaration covers either the finding's start byte or the start of its
// line suppresses it, so a finding on a declaration's first line is caught
// even when the declaration starts after the line's indentation. A negative
// byteOffset checks the line start only, like IsSuppressed.
func (f *SuppressionFilter) IsSuppressedAt(ruleID, ruleSet string, line, byteOffset int) bool {
	if f == nil {
		return false
	}
	if f.excludedRules[ruleID] {
		return true
	}
	aliases := f.ruleAliases[ruleID]
	if suppressed := f.inline[line]; suppressed != nil {
		if suppressed[""] || suppressed[ruleID] {
			return true
		}
		if ruleSet != "" && (suppressed[ruleSet+"."+ruleID] || suppressed[ruleSet+":"+ruleID]) {
			return true
		}
		for _, alias := range aliases {
			if suppressed[alias] {
				return true
			}
			if ruleSet != "" && (suppressed[ruleSet+"."+alias] || suppressed[ruleSet+":"+alias]) {
				return true
			}
		}
	}
	return f.annotationSuppresses(ruleID, ruleSet, line, byteOffset, aliases)
}

// IsSuppressedAtColumn is IsSuppressed for a finding with a 1-based column,
// so an annotation that starts mid-line (e.g. on a constructor parameter)
// covers the finding. A column below 2 checks the first non-blank byte of the
// line, since indented declarations start after their indent.
func (f *SuppressionFilter) IsSuppressedAtColumn(ruleID, ruleSet string, line, col int) bool {
	if f == nil {
		return false
	}
	byteOffset := -1
	if f.file != nil && line >= 1 {
		start := f.file.LineOffset(line - 1)
		off := start
		if col > 1 {
			off = start + col - 1
		} else {
			off = skipLineIndent(f.file.Content, start)
		}
		if off < len(f.file.Content) {
			byteOffset = off
		}
	}
	return f.IsSuppressedAt(ruleID, ruleSet, line, byteOffset)
}

// annotationSuppresses checks the @Suppress / @SuppressWarnings index at the
// start of line and, when known, at the finding's own start byte.
func (f *SuppressionFilter) annotationSuppresses(ruleID, ruleSet string, line, byteOffset int, aliases []string) bool {
	if f.annotations == nil || f.file == nil {
		return false
	}
	lineOffset := suppressionByteOffsetForLine(f.file, line)
	if f.annotations.isSuppressedWithAliases(lineOffset, ruleID, ruleSet, aliases) {
		return true
	}
	return byteOffset >= 0 && byteOffset != lineOffset && byteOffset < len(f.file.Content) &&
		f.annotations.isSuppressedWithAliases(byteOffset, ruleID, ruleSet, aliases)
}

// skipLineIndent returns the offset of the first non-blank byte at or after
// offset on the same line, or offset itself when the line is blank.
func skipLineIndent(content []byte, offset int) int {
	i := offset
	for i < len(content) && (content[i] == ' ' || content[i] == '\t') {
		i++
	}
	if i >= len(content) || content[i] == '\n' || content[i] == '\r' {
		return offset
	}
	return i
}

// suppressionByteOffsetForLine maps a 1-based finding line to a byte
// offset inside `file` that suppression-range checks can use.
//
//   - `line >= 1` and within the file: returns the byte offset of the
//     start of that line.
//   - `line <= 0`: treats the finding as file-level (line is unknown
//     or the rule reports on the whole file) and returns 0. With
//     `EndByte` exclusive, a `@file:Suppress(...)` covering [0,
//     fileSize) still matches byte 0.
//   - `line` past the last line: clamps to the last byte in the file
//     so file-level suppressions still apply. `f.file.LineOffset`
//     returns `len(Content)` for out-of-range indices, which is past
//     the exclusive `EndByte` and would silently miss file-level
//     suppressions.
func suppressionByteOffsetForLine(file *File, line int) int {
	if line <= 0 {
		return 0
	}
	offset := file.LineOffset(line - 1)
	if n := len(file.Content); n > 0 && offset >= n {
		return n - 1
	}
	return offset
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

// IncludePatternPrefix marks an `includes` glob inside a rule's exclude list:
// the rule is skipped for a file unless some include glob matches it.
const IncludePatternPrefix = "!"

func matchAnyExcludePattern(filePath string, patterns []string) bool {
	filePath = filepath.ToSlash(filePath)
	hasInclude, included := false, false
	for _, p := range patterns {
		if strings.HasPrefix(p, IncludePatternPrefix) {
			hasInclude = true
			included = included || matchExcludePatternSlash(filePath, p[len(IncludePatternPrefix):])
			continue
		}
		if matchExcludePatternSlash(filePath, p) {
			return true
		}
	}
	return hasInclude && !included
}

// MatchExcludePattern reports whether filePath matches a rule exclude glob.
// Semantics:
//   - **/dir/**   matches any path containing a dir/ segment run, including a
//     relative path that starts with it; dir may hold further * or ** wildcards
//   - **/*suffix  matches any path ending with suffix
//   - **/name     matches any path ending with /name
//   - plain glob  matches the basename
func MatchExcludePattern(filePath, pattern string) bool {
	return matchExcludePatternSlash(filepath.ToSlash(filePath), pattern)
}

func matchExcludePatternSlash(filePath, pattern string) bool {
	pattern = filepath.ToSlash(pattern)
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		if strings.Contains(suffix, "/**") {
			inner := strings.TrimSuffix(suffix, "/**")
			rooted := "/" + strings.TrimPrefix(filePath, "/")
			if strings.ContainsRune(inner, '*') {
				return excludeInnerRegexp(inner).MatchString(rooted)
			}
			return strings.Contains(rooted, "/"+inner+"/")
		}
		if strings.HasPrefix(suffix, "*") {
			return strings.HasSuffix(filePath, suffix[1:])
		}
		return strings.HasSuffix(filePath, "/"+suffix) || filePath == suffix
	}
	matched, _ := filepath.Match(pattern, filepath.Base(filePath))
	return matched
}

var excludeInnerCache sync.Map // inner pattern -> *regexp.Regexp

// excludeInnerRegexp compiles the directory part of a **/inner/** pattern:
// ** spans any number of segments and * stays inside one segment.
func excludeInnerRegexp(inner string) *regexp.Regexp {
	if v, ok := excludeInnerCache.Load(inner); ok {
		return v.(*regexp.Regexp)
	}
	var b strings.Builder
	b.WriteString("/")
	for i := 0; i < len(inner); i++ {
		switch {
		case strings.HasPrefix(inner[i:], "**/"):
			b.WriteString("(?:[^/]+/)*")
			i += 2
		case strings.HasPrefix(inner[i:], "**"):
			b.WriteString(".*")
			i++
		case inner[i] == '*':
			b.WriteString("[^/]*")
		default:
			b.WriteString(regexp.QuoteMeta(string(inner[i])))
		}
	}
	b.WriteString("/")
	re := regexp.MustCompile(b.String())
	excludeInnerCache.Store(inner, re)
	return re
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
	lx := inlineLexer{content: content, line: 1, out: out}
	lx.scanCode(false)
	if len(out) == 0 {
		return nil
	}
	return out
}

// inlineLexer walks Kotlin source and reports only real `//` line comments, so
// a `// krit:ignore` inside a string literal or block comment does not suppress anything.
type inlineLexer struct {
	content []byte
	pos     int
	line    int
	out     map[int]map[string]bool
}

// scanCode scans code until the end of input or, when inTemplate is set, the `}` closing a string template.
func (lx *inlineLexer) scanCode(inTemplate bool) {
	depth := 0
	for lx.pos < len(lx.content) {
		c := lx.content[lx.pos]
		switch {
		case c == '\n':
			lx.line++
			lx.pos++
		case c == '/' && lx.peek(1) == '/':
			end := bytes.IndexByte(lx.content[lx.pos:], '\n')
			if end < 0 {
				end = len(lx.content) - lx.pos
			}
			addInlineIgnore(lx.out, lx.line, lx.content[lx.pos:lx.pos+end])
			lx.pos += end
		case c == '/' && lx.peek(1) == '*':
			lx.skipBlockComment()
		case c == '"':
			if lx.peek(1) == '"' && lx.peek(2) == '"' {
				lx.pos += 3
				lx.skipString(true)
			} else {
				lx.pos++
				lx.skipString(false)
			}
		case c == '\'':
			lx.skipChar()
		case inTemplate && c == '{':
			depth++
			lx.pos++
		case inTemplate && c == '}':
			if depth == 0 {
				lx.pos++
				return
			}
			depth--
			lx.pos++
		default:
			lx.pos++
		}
	}
}

func (lx *inlineLexer) peek(n int) byte {
	if lx.pos+n < len(lx.content) {
		return lx.content[lx.pos+n]
	}
	return 0
}

// skipBlockComment skips a possibly nested block comment starting at lx.pos.
func (lx *inlineLexer) skipBlockComment() {
	depth := 0
	for lx.pos < len(lx.content) {
		switch {
		case lx.content[lx.pos] == '/' && lx.peek(1) == '*':
			depth++
			lx.pos += 2
		case lx.content[lx.pos] == '*' && lx.peek(1) == '/':
			depth--
			lx.pos += 2
			if depth == 0 {
				return
			}
		default:
			if lx.content[lx.pos] == '\n' {
				lx.line++
			}
			lx.pos++
		}
	}
}

// skipString skips the rest of a string literal whose opening quote was consumed.
func (lx *inlineLexer) skipString(raw bool) {
	for lx.pos < len(lx.content) {
		c := lx.content[lx.pos]
		switch {
		case c == '\n':
			lx.line++
			lx.pos++
			if !raw {
				return // unterminated regular string; resume at the next line
			}
		case c == '\\' && !raw:
			lx.pos += 2
		case c == '$' && lx.peek(1) == '{':
			lx.pos += 2
			lx.scanCode(true)
		case c == '"' && !raw:
			lx.pos++
			return
		case c == '"' && lx.peek(1) == '"' && lx.peek(2) == '"':
			lx.pos += 3
			for lx.pos < len(lx.content) && lx.content[lx.pos] == '"' {
				lx.pos++
			}
			return
		default:
			lx.pos++
		}
	}
}

// skipChar skips a char literal, or a lone quote that is not one.
func (lx *inlineLexer) skipChar() {
	rest := lx.content[lx.pos+1:]
	switch {
	case len(rest) >= 2 && rest[0] != '\\' && rest[0] != '\n' && rest[1] == '\'':
		lx.pos += 3
	case len(rest) >= 3 && rest[0] == '\\':
		end := bytes.IndexByte(rest[1:min(len(rest), 10)], '\'')
		if end < 0 {
			lx.pos++
			return
		}
		lx.pos += end + 3
	default:
		lx.pos++
	}
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
	if len(rest) > 0 && isIdentByte(rest[0]) {
		return // e.g. `krit:ignored` is not a directive
	}
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
	needleStarts []int         // sorted offsets of Suppress( / SuppressWarnings( in the source
}

// hasNeedleIn reports whether a suppress needle starts inside [start, end).
func (idx *SuppressionIndex) hasNeedleIn(start, end int) bool {
	i := sort.SearchInts(idx.needleStarts, start)
	return i < len(idx.needleStarts) && idx.needleStarts[i] < end
}

// findSuppressionNeedles returns the sorted start offsets of every suppress needle.
func findSuppressionNeedles(content []byte) []int {
	var out []int
	for _, needle := range suppressionNeedles {
		for from := 0; from < len(content); {
			i := bytes.Index(content[from:], needle)
			if i < 0 {
				break
			}
			out = append(out, from+i)
			from += i + 1
		}
	}
	sort.Ints(out)
	return out
}

var suppressionNeedles = [][]byte{
	[]byte("Suppress("),
	[]byte("SuppressWarnings("),
	[]byte("SuppressLint("), // Android Lint ids; same argument shape as @Suppress
}

// BuildSuppressionIndexFlat walks the flat tree once to find all
// @Suppress/@SuppressWarnings annotations and builds an index of suppressed
// byte ranges.
func BuildSuppressionIndexFlat(tree *FlatTree, content []byte) *SuppressionIndex {
	idx := &SuppressionIndex{
		suppressions: make([]Suppression, 0, 16),
	}
	if tree == nil || len(tree.Types) == 0 {
		return idx
	}
	idx.needleStarts = findSuppressionNeedles(content)
	flatWalkForSuppressions(tree, 0, content, idx)
	addIgnoreAnnotatedSuppressions(tree, content, idx)
	sort.Slice(idx.suppressions, func(i, j int) bool {
		return idx.suppressions[i].StartByte < idx.suppressions[j].StartByte
	})
	return idx
}

// IsSuppressed checks if a finding at the given byte offset is suppressed for the given rule.
func (idx *SuppressionIndex) IsSuppressed(byteOffset int, ruleName string, ruleSetName string) bool {
	return idx.isSuppressedWithAliases(byteOffset, ruleName, ruleSetName, nil)
}

func (idx *SuppressionIndex) isSuppressedWithAliases(byteOffset int, ruleName, ruleSetName string, aliases []string) bool {
	if len(idx.suppressions) == 0 {
		return false
	}

	// Binary search for the rightmost suppression starting at or before byteOffset.
	i := sort.Search(len(idx.suppressions), func(i int) bool {
		return idx.suppressions[i].StartByte > byteOffset
	})

	// Walk every candidate (StartByte <= byteOffset) and keep the ones
	// whose range still contains the offset. The suppression interval is
	// [StartByte, EndByte) — tree-sitter reports the byte one past the
	// last byte of the node — so a finding at byteOffset == EndByte is
	// past the annotation's scope and must not be suppressed.
	//
	// We cannot break early when one suppression's EndByte is at or
	// before byteOffset: the list is sorted by StartByte ascending, and
	// an earlier-listed (outer) suppression can have a larger EndByte
	// than this nested one. For example, a `@file:Suppress(...)`
	// covering the whole file appears before an inner `@Suppress(...)`
	// on a function; a finding past the function but still inside the
	// file must still see the file-level suppression.
	for j := i - 1; j >= 0; j-- {
		s := idx.suppressions[j]
		if byteOffset >= s.EndByte {
			continue
		}
		if suppressionMatches(s.Rules, ruleName, ruleSetName) {
			return true
		}
		for _, alias := range aliases {
			if suppressionMatches(s.Rules, alias, ruleSetName) {
				return true
			}
		}
	}
	return false
}

// kotlinCompilerWarningAliases maps Kotlin compiler warning identifiers
// (used in `@Suppress("FOO")`) to the krit rule names that detect the
// same issue.
var kotlinCompilerWarningAliases = map[string][]string{
	"UNUSED_PARAMETER":           {"UnusedParameter"},
	"UNUSED_VARIABLE":            {"UnusedVariable"},
	"UNUSED_EXPRESSION":          {"UnusedPrivateProperty"},
	"UNUSED_ANONYMOUS_PARAMETER": {"UnusedParameter"},
	"NAME_SHADOWING":             {"NoNameShadowing"},
	"UNCHECKED_CAST":             {"UnsafeCast", "DontDowncastCollectionTypes"},
	"UNUSED_VALUE":               {"UnusedPrivateProperty"},
	"UNUSED_IMPORT":              {"UnusedImport"},
	"UnusedPrivateMember": {
		"UnusedPrivateMember",
		"UnusedPrivateFunction",
		"UnusedPrivateProperty",
		"UnusedPrivateClass",
	},
	// IntelliJ/Kotlin inspection id (lowercase). Covers all unused-declaration
	// krit rules.
	"unused": {"UnusedParameter", "UnusedPrivateProperty", "UnusedPrivateMember", "UnusedPrivateFunction", "UnusedPrivateClass", "UnusedVariable", "UnusedImport"},

	// detekt rule names whose krit ids differ.
	"ComplexMethod":              {"CyclomaticComplexMethod"},
	"PackageName":                {"PackageNaming"},
	"VariableName":               {"VariableNaming"},
	"FunctionMaxLength":          {"FunctionNameMaxLength"},
	"FunctionMinLength":          {"FunctionNameMinLength"},
	"PackageDirectoryMismatch":   {"InvalidPackageDeclaration"},
	"EmptyKtFile":                {"EmptyKotlinFile"},
	"CommentOverPrivateFunction": {"DocumentationOverPrivateFunction"},
	"CommentOverPrivateProperty": {"DocumentationOverPrivateProperty"},
	"UnnecessaryAbstractClass":   {"AbstractClassCanBeConcreteClass", "AbstractClassCanBeInterface"},

	// IntelliJ Kotlin inspection IDs used in `@Suppress(...)` for naming
	// conventions. These pair up with krit's naming rules.
	"EnumEntryName":       {"EnumNaming"},
	"ClassName":           {"ClassNaming"},
	"FunctionName":        {"FunctionNaming"},
	"PropertyName":        {"TopLevelPropertyNaming", "ObjectPropertyNaming"},
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
	if tree == nil || int(idx) >= len(tree.Types) {
		return
	}
	nodeType := nodeTypeName(tree.Types[idx])

	switch nodeType {
	case "annotation":
		flatProcessSuppressionNode(tree, idx, content, out)
		return
	case "prefix_expression":
		flatProcessSuppressionNode(tree, idx, content, out)
		// The annotated operand can hold further annotated expressions.
		for child := tree.FirstChildren[idx]; child != 0; child = tree.NextSibs[child] {
			if nodeTypeName(tree.Types[child]) != "annotation" {
				flatWalkForSuppressions(tree, child, content, out)
			}
		}
		return
	case "file_annotation":
		flatProcessFileAnnotation(tree, idx, content, out)
		return
	}

	// Descend into every node that contains a suppress needle, so annotations on
	// expressions nested in arguments, loops, elvis operands and the like count.
	if !out.hasNeedleIn(int(tree.StartBytes[idx]), int(tree.EndBytes[idx])) {
		return
	}
	for child := tree.FirstChildren[idx]; child != 0; child = tree.NextSibs[child] {
		switch nodeTypeName(tree.Types[child]) {
		case "file_annotation":
			flatProcessFileAnnotation(tree, child, content, out)
		default:
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
	// file_annotation is a direct child of source_file (the root, always at
	// index 0 in the FlatTree).  We want to suppress the whole file, so we
	// use the root node's range directly instead of walking parents (the
	// root's Parent field is also 0, making walk-until-parent==0 ambiguous).
	var root uint32
	out.suppressions = append(out.suppressions, Suppression{
		StartByte: int(tree.StartBytes[root]),
		EndByte:   int(tree.EndBytes[root]),
		Rules:     rules,
	})
}

func flatProcessSuppressionNode(tree *FlatTree, idx uint32, content []byte, out *SuppressionIndex) bool {
	parseText := suppressionAnnotationText(tree, idx, content)
	if parseText == nil {
		return false
	}
	rules := extractSuppressedRules(string(parseText))
	var target uint32
	if isAnnotationOnly(tree, idx, content) {
		// The grammar split the annotation from its declaration; it applies to
		// the next statement.
		target = nextStatementSibling(tree, idx, content)
		if target == 0 {
			return true
		}
	} else if operand := splitReturnOperand(tree, idx, content); operand != 0 {
		target = operand
	} else {
		target = flatFindAnnotationTarget(tree, idx)
	}
	if target == 0 && idx != 0 {
		return true
	}
	if cs, ce, ok := constructorSuppressionSpan(tree, target, content); ok {
		out.suppressions = append(out.suppressions, Suppression{StartByte: cs, EndByte: ce, Rules: rules})
		return true
	}
	// Cover the annotation itself when it precedes its target, so findings
	// reported at the annotation line match.
	start := min(int(tree.StartBytes[target]), int(tree.StartBytes[idx]))
	out.suppressions = append(out.suppressions, Suppression{
		StartByte: start,
		EndByte:   suppressionTargetEnd(tree, target, content),
		Rules:     rules,
	})
	return true
}

// suppressionAnnotationText returns the text of the suppress annotation that
// drives the annotation or prefix_expression at idx, including an argument
// list the grammar parsed as a separate node. It returns nil when idx carries
// no suppress annotation.
func suppressionAnnotationText(tree *FlatTree, idx uint32, content []byte) []byte {
	ann := idx
	if nodeTypeName(tree.Types[idx]) == "prefix_expression" {
		// Only a suppress annotation may drive a prefix_expression; others merely wrap code that mentions one.
		ann = 0
		for child := tree.FirstChildren[idx]; child != 0; child = tree.NextSibs[child] {
			if nodeTypeName(tree.Types[child]) == "annotation" {
				ann = child
				break
			}
		}
		if ann == 0 {
			return nil
		}
	}
	var text []byte
	if flatAnnotationHasArgs(tree, ann) {
		text = FlatNodeBytes(tree, ann, content)
	} else if end := detachedArgsEnd(tree, ann, content); end > 0 {
		text = content[tree.StartBytes[ann]:end]
	} else if ann == idx {
		text = FlatNodeBytes(tree, ann, content)
	}
	if !hasSuppressionNeedle(text) {
		return nil
	}
	return text
}

func flatAnnotationHasArgs(tree *FlatTree, idx uint32) bool {
	for child := tree.FirstChildren[idx]; child != 0; child = tree.NextSibs[child] {
		if nodeTypeName(tree.Types[child]) == "constructor_invocation" {
			return true
		}
	}
	return false
}

func flatFindAnnotationTarget(tree *FlatTree, idx uint32) uint32 {
	if nodeTypeName(tree.Types[idx]) == "prefix_expression" {
		var annChild uint32
		hasOperand := false
		for child := tree.FirstChildren[idx]; child != 0; child = tree.NextSibs[child] {
			if nodeTypeName(tree.Types[child]) == "annotation" {
				annChild = child
			} else {
				hasOperand = true
			}
		}
		if annChild != 0 && hasOperand && flatAnnotationHasArgs(tree, annChild) {
			return idx
		}
	}

	parent := tree.Parents[idx]
	if idx == 0 && parent == 0 {
		return 0
	}
	if nodeTypeName(tree.Types[parent]) == "modifiers" {
		return tree.Parents[parent]
	}
	parentType := nodeTypeName(tree.Types[parent])
	if parentType == "parameter_modifiers" {
		// Function parameter modifiers are a sibling that precedes the parameter they annotate.
		for sib := tree.NextSibs[parent]; sib != 0; sib = tree.NextSibs[sib] {
			if nodeTypeName(tree.Types[sib]) == "parameter" {
				return sib
			}
		}
		return parent
	}
	if parentType == "source_file" || parentType == "statements" || parentType == "class_body" {
		foundSelf := false
		for child := tree.FirstChildren[parent]; child != 0; child = tree.NextSibs[child] {
			if child == idx {
				foundSelf = true
				continue
			}
			if !foundSelf {
				continue
			}
			childType := nodeTypeName(tree.Types[child])
			if childType == "prefix_expression" || childType == "annotation" || isCommentNode(tree, child) {
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

// extractSuppressedRules parses rule names from @Suppress("Rule1", "Rule2"), @SuppressWarnings("...") or @SuppressLint("...")
func extractSuppressedRules(text string) map[string]bool {
	// Find the arguments between ( and )
	start := strings.Index(text, "(")
	end := strings.LastIndex(text, ")")
	if start < 0 || end <= start {
		return nil
	}
	args := text[start+1 : end]

	// Take every string literal, so `names = ["A", "B"]`, `value = [...]` and `arrayOf(...)` work like varargs.
	rules := make(map[string]bool)
	for _, arg := range suppressionStringLiterals(args) {
		arg = strings.TrimSpace(arg)
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

// suppressionStringLiterals returns the contents of the double-quoted string literals in an annotation argument list.
func suppressionStringLiterals(args string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if args[i] != '"' {
			continue
		}
		var b strings.Builder
		j := i + 1
		for ; j < len(args) && args[j] != '"'; j++ {
			if args[j] == '\\' && j+1 < len(args) {
				j++
			}
			b.WriteByte(args[j])
		}
		out = append(out, b.String())
		i = j
	}
	return out
}

var (
	ignoreAnnotatedMu    sync.RWMutex
	ignoreAnnotatedRules map[string]map[string]bool // simple annotation name -> rule IDs it silences
)

// SetRuleIgnoreAnnotated installs the per-rule `ignoreAnnotated` config: a
// declaration carrying one of a rule's annotations is exempt from that rule.
// It replaces any earlier setting; a nil map clears it.
func SetRuleIgnoreAnnotated(byRule map[string][]string) {
	idx := map[string]map[string]bool{}
	for rule, anns := range byRule {
		for _, ann := range anns {
			name := simpleAnnotationName(ann)
			if name == "" {
				continue
			}
			if idx[name] == nil {
				idx[name] = map[string]bool{}
			}
			idx[name][rule] = true
		}
	}
	ignoreAnnotatedMu.Lock()
	ignoreAnnotatedRules = idx
	ignoreAnnotatedMu.Unlock()
}

// simpleAnnotationName reduces `@com.x.Name(args)` or `Name` to `Name`.
func simpleAnnotationName(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "@"))
	if i := strings.IndexAny(s, "( \t\r\n"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// addIgnoreAnnotatedSuppressions adds a suppression over each declaration
// annotated with an annotation configured via ignoreAnnotated.
func addIgnoreAnnotatedSuppressions(tree *FlatTree, content []byte, out *SuppressionIndex) {
	ignoreAnnotatedMu.RLock()
	byAnn := ignoreAnnotatedRules
	ignoreAnnotatedMu.RUnlock()
	if len(byAnn) == 0 {
		return
	}
	for i := range tree.Types {
		idx := uint32(i)
		if nodeTypeName(tree.Types[idx]) != "annotation" {
			continue
		}
		name := simpleAnnotationName(strings.TrimPrefix(string(FlatNodeBytes(tree, idx, content)), "@file:"))
		rules, ok := byAnn[name]
		if !ok {
			continue
		}
		target := flatFindAnnotationTarget(tree, idx)
		if target == 0 {
			continue
		}
		out.suppressions = append(out.suppressions, Suppression{
			StartByte: int(tree.StartBytes[target]),
			EndByte:   suppressionTargetEnd(tree, target, content),
			Rules:     rules,
		})
	}
}
