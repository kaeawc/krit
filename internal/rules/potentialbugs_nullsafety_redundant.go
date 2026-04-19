package rules

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// ---------------------------------------------------------------------------
// UnnecessaryNotNullCheckRule detects != null on non-nullable.
// ---------------------------------------------------------------------------
type UnnecessaryNotNullCheckRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs null safety rule. Detection leans on structural patterns
// around nullable expressions and has a heuristic fallback when the
// resolver is absent. Classified per roadmap/17.
func (r *UnnecessaryNotNullCheckRule) Confidence() float64 { return 0.75 }

var unnecessaryNullCheckRe = regexp.MustCompile(`\bval\s+(\w+)\s*:\s*([A-Z]\w+)\s*=`)

// mutableVarPropertyRe matches `[modifiers] var name` declarations (not val).
// Used to detect properties where Kotlin smart-cast is disabled.
var mutableVarPropertyRe = regexp.MustCompile(`\bvar\s+(\w+)`)

// isMutableVarProperty returns true if the given name is declared as `var`
// anywhere in the file. Kotlin cannot smart-cast mutable properties because
// their value could change between the null check and access.
func isMutableVarProperty(file *scanner.File, name string) bool {
	for _, line := range file.Lines {
		matches := mutableVarPropertyRe.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if m[1] == name {
				return true
			}
		}
	}
	return false
}

// explicitNullableDeclRe matches `[val|var] name : SomeType?` patterns.
var explicitNullableDeclRe = regexp.MustCompile(`\b(?:val|var)\s+(\w+)\s*:\s*[\w<>.]+\?`)

// isFrameworkNullableProperty returns true if `name` is a bare identifier that
// in typical Android framework usage resolves to an inherited property whose
// getter is `@Nullable`. A conservative name-based resolver cannot see the
// Java `@Nullable` annotation and may widen these to non-null, producing
// false positives for UnnecessarySafeCall / UnnecessaryNotNullOperator.
//
// Only listed here are names whose framework property is nullable AND which
// are rarely the subject of a legitimately-unnecessary !! / ?. in Android
// code. The FP risk from these framework properties outweighs any missed
// findings on identically-named local vals.
func isFrameworkNullableProperty(name string) bool {
	switch name {
	// RecyclerView.adapter, RecyclerView.layoutManager, RecyclerView.itemAnimator
	case "adapter", "layoutManager", "itemAnimator":
		return true
	// DialogFragment.dialog, Fragment.view, Fragment.parentFragment,
	// Fragment.targetFragment, Fragment.host
	case "dialog", "parentFragment", "targetFragment", "host":
		return true
	// View.parent, View.tag, View.rootView, View.background, View.contentDescription
	case "parent", "tag", "background", "contentDescription":
		return true
	}
	return false
}

// hasMemberAccessInitializer returns true if `name` is declared as a local
// val whose initializer is a member access expression like `something.field`
// (without explicit non-null assertion) that a bare-name resolver cannot
// prove non-null. Used to suppress false positives where the resolver
// incorrectly widens a nullable member access to non-null.
var valMemberInitRe = regexp.MustCompile(`\bval\s+(\w+)\s*=\s*([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)+)\s*(?:$|//|\n)`)

func hasMemberAccessInitializer(file *scanner.File, name string) bool {
	for _, line := range file.Lines {
		m := valMemberInitRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[1] != name {
			continue
		}
		init := m[2]
		// If the initializer ends with `!!`, it's explicitly non-null; allow flagging.
		if strings.HasSuffix(init, "!!") {
			return false
		}
		return true
	}
	return false
}

// isExplicitNullableDeclaration returns true if the given name is declared
// with an explicit nullable type annotation (`: T?`) anywhere in the file.
func isExplicitNullableDeclaration(file *scanner.File, name string) bool {
	for _, line := range file.Lines {
		matches := explicitNullableDeclRe.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if m[1] == name {
				return true
			}
		}
	}
	return false
}

func (r *UnnecessaryNotNullCheckRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	// Check for "name != null" or "name == null" patterns
	isNotNull := strings.Contains(text, "!= null")
	isEqNull := strings.Contains(text, "== null")
	if !isNotNull && !isEqNull {
		return
	}
	// Extract the variable name from the comparison
	var varName string
	if isNotNull {
		parts := strings.SplitN(text, " != null", 2)
		varName = strings.TrimSpace(parts[0])
	} else {
		parts := strings.SplitN(text, " == null", 2)
		varName = strings.TrimSpace(parts[0])
	}
	if varName == "" {
		return
	}
	// Check if varName is declared as a non-nullable val in this file
	nonNullable := false
	for _, line := range file.Lines {
		if m := unnecessaryNullCheckRe.FindStringSubmatch(line); m != nil {
			if m[1] == varName && !strings.HasSuffix(m[2], "?") {
				nonNullable = true
				break
			}
		}
	}
	if !nonNullable {
		return
	}
	replacement := "true"
	if isEqNull {
		replacement = "false"
	}
	f := r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Unnecessary null check on non-nullable val '%s'.", varName))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: replacement,
	}
	ctx.Emit(f)
}

// ---------------------------------------------------------------------------
// UnnecessaryNotNullOperatorRule detects !! on non-null.
// ---------------------------------------------------------------------------
type UnnecessaryNotNullOperatorRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *UnnecessaryNotNullOperatorRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — flags !! on a
// non-null type; relies on resolver for nullability. Heuristic fallback is
// conservative but noisy. Classified per roadmap/17.
func (r *UnnecessaryNotNullOperatorRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryNotNullOperatorRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	if !strings.HasSuffix(text, "!!") {
		return
	}

	// Extract the receiver name (everything before !!)
	receiver := strings.TrimSuffix(text, "!!")
	receiver = strings.TrimSpace(receiver)

	// Skip idiomatic Android/Java platform API patterns where !! is required.
	if isIdiomaticNullAssertionReceiver(receiver, file) {
		return
	}

	// Only resolve simple identifiers — dotted member accesses cannot be
	// reliably resolved by bare name lookup (risk of name collision).
	if !strings.Contains(receiver, ".") {
		name := strings.TrimSpace(receiver)
		if name == "this" && nullableThisFromLambdaReceiverCallFlat(file, idx, r.resolver) {
			return
		}

		// Skip if the name refers to a mutable `var` property/field.
		// Kotlin does NOT smart-cast mutable properties (the value could
		// change between the null check and the access), so !! is required
		// even after a null check.
		if isMutableVarProperty(file, name) {
			return
		}
		// Skip if the declaration's initializer contains an `else null`
		// tail or a `?.let` chain — a conservative resolver may widen
		// these to non-null incorrectly.
		if hasBranchNullInitializer(file, name) {
			return
		}
		// Skip framework-inherited nullable properties (e.g. RecyclerView.adapter,
		// DialogFragment.dialog, View.parent) when not shadowed by a local decl.
		if isFrameworkNullableProperty(name) {
			return
		}
		// Skip local vals initialized from a member access the resolver can't
		// prove non-null (e.g. `val attachment = mediaItem.attachment`).
		if hasMemberAccessInitializer(file, name) {
			return
		}
		if hasNullableGenericParamBoundFlat(file, idx, name) {
			return
		}

		// Use type resolver with position-aware smart cast lookup
		if r.resolver != nil {
			resolved := r.resolver.ResolveByNameFlat(name, idx, file)
			if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
				if resolved.Kind == typeinfer.TypeGeneric {
					return
				}
				if resolved.IsNullable() {
					return // Actually nullable — !! is needed
				}
				// Known non-null — flag it
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Unnecessary not-null assertion (!!) on non-nullable '%s'.", name))
				bangStart := int(file.FlatEndByte(idx)) - 2
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: bangStart, EndByte: int(file.FlatEndByte(idx)), Replacement: ""}
				ctx.Emit(f)
				return
			}
		}

		// Fallback: heuristic — check if receiver is declared as non-nullable val in the file
		for _, fline := range file.Lines {
			if m := unnecessaryNullCheckRe.FindStringSubmatch(fline); m != nil {
				if m[1] == name && !strings.HasSuffix(m[2], "?") {
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("Unnecessary not-null assertion (!!) on non-nullable val '%s'.", name))
					bangStart := int(file.FlatEndByte(idx)) - 2
					f.Fix = &scanner.Fix{ByteMode: true, StartByte: bangStart, EndByte: int(file.FlatEndByte(idx)), Replacement: ""}
					ctx.Emit(f)
					return
				}
			}
		}
	}
}

// hasBranchNullInitializer returns true if the given name is declared in
// the file with an initializer that has an `else null` / `-> null` /
// `?: null` tail, producing a nullable inferred type that a conservative
// resolver may widen incorrectly. Used to suppress false positives on
// UnnecessaryNotNullOperator / UnnecessarySafeCall.
func hasBranchNullInitializer(file *scanner.File, name string) bool {
	decl := regexp.MustCompile(`\b(?:val|var)\s+` + regexp.QuoteMeta(name) + `\b[^\n=]*=`)
	for i, line := range file.Lines {
		if !decl.MatchString(line) {
			continue
		}
		// Track brace/paren balance to find the end of the initializer.
		depth := 0
		joined := line
		depth += strings.Count(line, "(") + strings.Count(line, "{")
		depth -= strings.Count(line, ")") + strings.Count(line, "}")
		for j := i + 1; j < len(file.Lines) && j < i+60; j++ {
			cur := file.Lines[j]
			joined += " " + cur
			depth += strings.Count(cur, "(") + strings.Count(cur, "{")
			depth -= strings.Count(cur, ")") + strings.Count(cur, "}")
			if depth <= 0 {
				break
			}
		}
		if strings.Contains(joined, "else null") ||
			strings.Contains(joined, "-> null") ||
			strings.Contains(joined, "?: null") ||
			strings.Contains(joined, "?.let") {
			return true
		}
	}
	return false
}

func hasNullableGenericParamBoundFlat(file *scanner.File, idx uint32, name string) bool {
	var fn uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" {
			fn = p
			break
		}
	}
	if fn == 0 {
		return false
	}
	fnText := file.FlatNodeText(fn)
	funStart := strings.Index(fnText, "fun ")
	if funStart < 0 {
		return false
	}
	sigText := fnText[funStart:]
	typeParamsStart := strings.Index(sigText, "<")
	paramsStart := strings.Index(sigText, "(")
	paramsEnd := strings.Index(sigText, ")")
	if typeParamsStart < 0 || paramsStart < 0 || paramsEnd < 0 || typeParamsStart > paramsStart {
		return false
	}
	typeParamsEnd := -1
	depth := 0
	for i := typeParamsStart; i < len(sigText); i++ {
		switch sigText[i] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				typeParamsEnd = i
				break
			}
		}
	}
	if typeParamsEnd < 0 {
		return false
	}
	typeParams := sigText[typeParamsStart+1 : typeParamsEnd]
	params := sigText[paramsStart+1 : paramsEnd]
	nullableTypeParams := make(map[string]bool)
	for _, entry := range splitTopLevelCommaParts(typeParams) {
		entry = strings.TrimSpace(entry)
		colon := strings.Index(entry, ":")
		if colon < 0 {
			continue
		}
		typeParam := strings.TrimSpace(entry[:colon])
		if typeParam == "" {
			continue
		}
		if strings.Contains(entry[colon+1:], "?") {
			nullableTypeParams[typeParam] = true
		}
	}
	if len(nullableTypeParams) == 0 {
		return false
	}
	paramPattern := regexp.MustCompile(`(^|[,(])\s*` + regexp.QuoteMeta(name) + `\s*:\s*([A-Z][A-Za-z0-9_]*)\b`)
	if match := paramPattern.FindStringSubmatch(params); match != nil && nullableTypeParams[match[2]] {
		return true
	}
	var localMatch bool
	file.FlatWalkNodes(fn, "property_declaration", func(prop uint32) {
		if localMatch || extractIdentifierFlat(file, prop) != name {
			return
		}
		varDecl := file.FlatFindChild(prop, "variable_declaration")
		if varDecl == 0 {
			return
		}
		for i := 0; i < file.FlatNamedChildCount(varDecl); i++ {
			child := file.FlatNamedChild(varDecl, i)
			switch file.FlatType(child) {
			case "user_type", "nullable_type":
				localMatch = nullableTypeParams[strings.TrimSpace(file.FlatNodeText(child))]
				return
			}
		}
	})
	if localMatch {
		return true
	}
	return false
}

func splitTopLevelCommaParts(s string) []string {
	var parts []string
	start := 0
	depthAngle := 0
	depthParen := 0
	depthBrace := 0
	depthBracket := 0
	for i, r := range s {
		switch r {
		case '<':
			depthAngle++
		case '>':
			if depthAngle > 0 {
				depthAngle--
			}
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '{':
			depthBrace++
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
		case '[':
			depthBracket++
		case ']':
			if depthBracket > 0 {
				depthBracket--
			}
		case ',':
			if depthAngle == 0 && depthParen == 0 && depthBrace == 0 && depthBracket == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// ---------------------------------------------------------------------------
// UnnecessarySafeCallRule detects ?. on non-null.
// ---------------------------------------------------------------------------
type UnnecessarySafeCallRule struct {
	FlatDispatchBase
	BaseRule
	resolver        typeinfer.TypeResolver
	nonNullableVals sync.Map
	localSummaries  sync.Map
}

func (r *UnnecessarySafeCallRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — flags ?. on a
// non-null receiver; needs resolver for nullability, falls back to
// name-based heuristic. Classified per roadmap/17.
func (r *UnnecessarySafeCallRule) Confidence() float64 { return 0.75 }

func (r *UnnecessarySafeCallRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "?.") {
		return
	}

	// The navigation_expression has children: receiver, navigation_suffix
	// The ?. operator is in the navigation_suffix
	if file.FlatChildCount(idx) < 2 {
		return
	}

	receiver := file.FlatChild(idx, 0)
	if receiver == 0 {
		return
	}

	// Extract receiver name
	receiverText := file.FlatNodeText(receiver)
	// If receiver itself uses safe calls, the expression is nullable —
	// the downstream ?. is justified.
	if strings.Contains(receiverText, "?.") {
		return
	}

	// Only resolve simple identifiers — dotted member accesses cannot be
	// reliably resolved by bare name lookup (risk of name collision).
	if strings.Contains(receiverText, ".") {
		return
	}

	name := strings.TrimSpace(receiverText)
	if name == "" {
		return
	}
	structural := experiment.Enabled("unnecessary-safe-call-structural")
	var localSummary *safeCallLocalSummary
	if experiment.Enabled("unnecessary-safe-call-local-nullability") {
		localSummary = r.localSummary(file)
	}

	// If the receiver is `this`, check if the enclosing function has a
	// nullable receiver type (extension function on nullable type). In that
	// case `this` is nullable and the safe call is justified.
	if name == "this" && unnecessarySafeCallNullableReceiverFlat(file, idx, structural) {
		return
	}

	// If the receiver is a simple identifier that matches a parameter of the
	// enclosing (override) function whose type is nullable, the safe call is
	// justified — framework methods often pass nullable parameters.
	if unnecessarySafeCallNullableFunctionParamFlat(file, idx, name) {
		return
	}

	// Skip if the name refers to a mutable var property — Kotlin does not
	// smart-cast mutable properties because the value can change between
	// the null check and the access.
	if localSummary != nil {
		if localSummary.mutableVar[name] || localSummary.explicitNullable[name] || localSummary.branchNullInitializer[name] || localSummary.memberAccessInitializer[name] {
			return
		}
	} else if isMutableVarProperty(file, name) {
		return
	}

	// Skip if the name is declared as an explicitly-nullable `val name: T?`.
	if localSummary == nil && isExplicitNullableDeclaration(file, name) {
		return
	}
	// Skip if the declaration has a branch-nullable initializer like
	// `if (...) X else null` or `?.let { ... }` — a conservative resolver
	// widens these incorrectly to non-null.
	if localSummary == nil && hasBranchNullInitializer(file, name) {
		return
	}
	// Skip framework-inherited nullable properties (RecyclerView.adapter,
	// DialogFragment.dialog, View.parent, etc.) when not shadowed by a
	// local declaration in this file.
	if isFrameworkNullableProperty(name) {
		return
	}
	// Skip local vals initialized from a member access the resolver can't
	// prove non-null (e.g. `val attachment = mediaItem.attachment`). A bare
	// name resolver widens the local val to the non-null path incorrectly.
	if localSummary == nil && hasMemberAccessInitializer(file, name) {
		return
	}

	// Use type resolver with position-aware smart cast lookup
	if r.resolver != nil {
		resolved := r.resolver.ResolveByNameFlat(name, idx, file)
		if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
			if resolved.IsNullable() {
				return // Actually nullable — safe call is needed
			}
			// Known non-null (possibly via smart cast) — flag
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("Unnecessary safe call (?.) on non-nullable '%s'.", name))
			// Find the ?. in the text and replace with .
			qIdx := strings.Index(text, "?.")
			if qIdx >= 0 {
				start := int(file.FlatStartByte(idx))
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: start + qIdx, EndByte: start + qIdx + 2, Replacement: "."}
			}
			ctx.Emit(f)
			return
		}
	}

	// Fallback: heuristic — check if receiver is declared as non-nullable val
	if r.nonNullableValNames(file)[name] {
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			fmt.Sprintf("Unnecessary safe call (?.) on non-nullable val '%s'.", name))
		qIdx := strings.Index(text, "?.")
		if qIdx >= 0 {
			start := int(file.FlatStartByte(idx))
			f.Fix = &scanner.Fix{ByteMode: true, StartByte: start + qIdx, EndByte: start + qIdx + 2, Replacement: "."}
		}
		ctx.Emit(f)
	}
}

type safeCallLocalSummary struct {
	explicitNullable        map[string]bool
	branchNullInitializer   map[string]bool
	memberAccessInitializer map[string]bool
	mutableVar              map[string]bool
}

func (r *UnnecessarySafeCallRule) localSummary(file *scanner.File) *safeCallLocalSummary {
	if cached, ok := r.localSummaries.Load(file.Path); ok {
		return cached.(*safeCallLocalSummary)
	}
	summary := &safeCallLocalSummary{
		explicitNullable:        make(map[string]bool),
		branchNullInitializer:   make(map[string]bool),
		memberAccessInitializer: make(map[string]bool),
		mutableVar:              make(map[string]bool),
	}
	for i, line := range file.Lines {
		for _, m := range mutableVarPropertyRe.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				summary.mutableVar[m[1]] = true
			}
		}
		for _, m := range explicitNullableDeclRe.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				summary.explicitNullable[m[1]] = true
			}
		}
		if m := valMemberInitRe.FindStringSubmatch(line); m != nil && !strings.HasSuffix(m[2], "!!") {
			summary.memberAccessInitializer[m[1]] = true
		}
		if !strings.Contains(line, "=") {
			continue
		}
		header := strings.TrimSpace(line)
		if !strings.HasPrefix(header, "val ") && !strings.HasPrefix(header, "var ") {
			continue
		}
		name := safeCallDeclaredNameFromLine(line)
		if name == "" {
			continue
		}
		depth := strings.Count(line, "(") + strings.Count(line, "{") - strings.Count(line, ")") - strings.Count(line, "}")
		joined := line
		for j := i + 1; j < len(file.Lines) && j < i+60; j++ {
			cur := file.Lines[j]
			joined += " " + cur
			depth += strings.Count(cur, "(") + strings.Count(cur, "{") - strings.Count(cur, ")") - strings.Count(cur, "}")
			if depth <= 0 {
				break
			}
		}
		if strings.Contains(joined, "else null") ||
			strings.Contains(joined, "-> null") ||
			strings.Contains(joined, "?: null") ||
			strings.Contains(joined, "?.let") {
			summary.branchNullInitializer[name] = true
		}
	}
	if cached, loaded := r.localSummaries.LoadOrStore(file.Path, summary); loaded {
		return cached.(*safeCallLocalSummary)
	}
	return summary
}

func safeCallDeclaredNameFromLine(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "val ") {
		line = strings.TrimPrefix(line, "val ")
	} else if strings.HasPrefix(line, "var ") {
		line = strings.TrimPrefix(line, "var ")
	} else {
		return ""
	}
	end := len(line)
	for i, r := range line {
		if r == ':' || r == '=' || r == ' ' || r == '\t' {
			end = i
			break
		}
	}
	if end == 0 {
		return ""
	}
	return strings.TrimSpace(line[:end])
}

func unnecessarySafeCallNullableReceiverFlat(file *scanner.File, idx uint32, structural bool) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		pt := file.FlatType(p)
		if pt != "function_declaration" && pt != "property_declaration" && pt != "getter" {
			continue
		}
		if pt == "getter" {
			if getterNullableReceiverFlat(file, p, structural) {
				return true
			}
			continue
		}
		if structural {
			for i := 0; i < file.FlatNamedChildCount(p); i++ {
				child := file.FlatNamedChild(p, i)
				if child == 0 {
					continue
				}
				if t := file.FlatType(child); t == "receiver_type" || t == "nullable_type" {
					text := strings.TrimSpace(file.FlatNodeText(child))
					return strings.HasSuffix(text, "?") || file.FlatFindChild(child, "nullable_type") != 0
				}
			}
			continue
		}
		for i := 0; i < file.FlatChildCount(p); i++ {
			child := file.FlatChild(p, i)
			switch file.FlatType(child) {
			case "receiver_type":
				recText := strings.TrimSpace(file.FlatNodeText(child))
				if strings.HasSuffix(recText, "?") {
					return true
				}
				if file.FlatFindChild(child, "nullable_type") != 0 {
					return true
				}
			case "nullable_type":
				return true
			}
		}
		fnText := file.FlatNodeText(p)
		sigEnd := len(fnText)
		if pt == "function_declaration" {
			if parenIdx := strings.Index(fnText, "("); parenIdx > 0 {
				sigEnd = parenIdx
			}
		} else if colonIdx := strings.Index(fnText, ":"); colonIdx > 0 {
			sigEnd = colonIdx
		}
		prefix := fnText[:sigEnd]
		if strings.Contains(prefix, "?.") {
			return true
		}
		continue
	}
	return false
}

func getterNullableReceiverFlat(file *scanner.File, getter uint32, structural bool) bool {
	if file == nil || getter == 0 {
		return false
	}
	getterStart := file.FlatStartByte(getter)
	for i := int(getter) - 1; i > 0; i-- {
		candidate := uint32(i)
		if file.FlatType(candidate) != "property_declaration" {
			continue
		}
		if file.FlatEndByte(candidate) > getterStart {
			continue
		}
		if structural {
			for j := 0; j < file.FlatNamedChildCount(candidate); j++ {
				child := file.FlatNamedChild(candidate, j)
				if t := file.FlatType(child); t == "receiver_type" || t == "nullable_type" {
					text := strings.TrimSpace(file.FlatNodeText(child))
					return strings.HasSuffix(text, "?") || file.FlatFindChild(child, "nullable_type") != 0
				}
			}
		} else {
			for j := 0; j < file.FlatChildCount(candidate); j++ {
				child := file.FlatChild(candidate, j)
				switch file.FlatType(child) {
				case "receiver_type":
					recText := strings.TrimSpace(file.FlatNodeText(child))
					if strings.HasSuffix(recText, "?") || file.FlatFindChild(child, "nullable_type") != 0 {
						return true
					}
				case "nullable_type":
					return true
				}
			}
		}
		text := file.FlatNodeText(candidate)
		if colonIdx := strings.Index(text, ":"); colonIdx > 0 && strings.Contains(text[:colonIdx], "?.") {
			return true
		}
		return false
	}
	return false
}

func nullableThisFromLambdaReceiverCallFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || idx == 0 || resolver == nil {
		return false
	}
	var lambda uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			lambda = p
			goto foundLambda
		case "function_declaration", "source_file":
			return false
		}
	}
foundLambda:
	if lambda == 0 {
		return false
	}
	var call uint32
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			call = p
			goto foundCall
		case "function_declaration", "source_file":
			return false
		}
	}
foundCall:
	if call == 0 {
		return false
	}
	receiverExpr, _ := flatCallExpressionParts(file, call)
	if receiverExpr == 0 {
		return false
	}
	if file.FlatType(receiverExpr) == "navigation_expression" && file.FlatNamedChildCount(receiverExpr) > 0 {
		receiverExpr = file.FlatNamedChild(receiverExpr, 0)
	}
	receiverName := strings.TrimSpace(file.FlatNodeText(receiverExpr))
	if receiverName == "" || strings.Contains(receiverName, ".") {
		return false
	}
	if receiverName == "this" {
		return unnecessarySafeCallNullableReceiverFlat(file, call, experiment.Enabled("unnecessary-safe-call-structural"))
	}
	if unnecessarySafeCallNullableFunctionParamFlat(file, call, receiverName) {
		return true
	}
	resolved := resolver.ResolveByNameFlat(receiverName, call, file)
	return resolved != nil && resolved.Kind != typeinfer.TypeUnknown && resolved.IsNullable()
}

func unnecessarySafeCallNullableFunctionParamFlat(file *scanner.File, idx uint32, name string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "function_declaration" {
			continue
		}
		params := file.FlatFindChild(p, "function_value_parameters")
		if params == 0 {
			return false
		}
		for i := 0; i < file.FlatChildCount(params); i++ {
			param := file.FlatChild(params, i)
			if file.FlatType(param) != "parameter" {
				continue
			}
			paramName := extractIdentifierFlat(file, param)
			if paramName != name {
				continue
			}
			paramText := file.FlatNodeText(param)
			if colonIdx := strings.Index(paramText, ":"); colonIdx >= 0 {
				typeText := strings.TrimSpace(paramText[colonIdx+1:])
				if eqIdx := strings.Index(typeText, "="); eqIdx >= 0 {
					typeText = strings.TrimSpace(typeText[:eqIdx])
				}
				if strings.HasSuffix(typeText, "?") {
					return true
				}
			}
			return false
		}
		return false
	}
	return false
}

func (r *UnnecessarySafeCallRule) nonNullableValNames(file *scanner.File) map[string]bool {
	if cached, ok := r.nonNullableVals.Load(file.Path); ok {
		return cached.(map[string]bool)
	}

	names := make(map[string]bool)
	for _, line := range file.Lines {
		if m := unnecessaryNullCheckRe.FindStringSubmatch(line); m != nil && !strings.HasSuffix(m[2], "?") {
			names[m[1]] = true
		}
	}
	if cached, loaded := r.nonNullableVals.LoadOrStore(file.Path, names); loaded {
		return cached.(map[string]bool)
	}
	return names
}

// ---------------------------------------------------------------------------
// NullCheckOnMutablePropertyRule detects null check on var property.
// ---------------------------------------------------------------------------
type NullCheckOnMutablePropertyRule struct {
	LineBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *NullCheckOnMutablePropertyRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — distinguishing
// val vs var requires type resolution of the receiver. Heuristic fallback
// uses declaration patterns. Classified per roadmap/17.
func (r *NullCheckOnMutablePropertyRule) Confidence() float64 { return 0.75 }

func (r *NullCheckOnMutablePropertyRule) check(ctx *v2.Context) {
	file := ctx.File
	// Collect var property names
	varProps := make(map[string]bool)
	for _, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "var ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				name := strings.TrimRight(parts[1], ":?")
				varProps[name] = true
			}
		}
	}
	if len(varProps) == 0 {
		return
	}
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "!= null") || strings.Contains(trimmed, "== null") {
			for prop := range varProps {
				if strings.Contains(trimmed, prop+" != null") || strings.Contains(trimmed, prop+" == null") {
					// If resolver is available, verify property is actually nullable
					if r.resolver != nil {
						offset := file.LineOffset(i) + strings.Index(line, prop)
						var resolved *typeinfer.ResolvedType
						if offset >= 0 {
							if propIdx, ok := file.FlatNamedDescendantForByteRange(uint32(offset), uint32(offset+len(prop))); ok {
								resolved = r.resolver.ResolveByNameFlat(prop, propIdx, file)
							}
						}
						if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && !resolved.IsNullable() {
							continue // property is not nullable, skip
						}
					}
					ctx.Emit(r.Finding(file, i+1, 1,
						fmt.Sprintf("Null check on mutable property '%s'. The value may change between the check and the use.", prop)))
					break
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// NullableToStringCallRule detects .toString() on nullable.
// ---------------------------------------------------------------------------
type NullableToStringCallRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *NullableToStringCallRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — needs resolver
// to know whether the receiver is nullable; heuristic fallback matches
// common null-returning APIs. Classified per roadmap/17.
func (r *NullableToStringCallRule) Confidence() float64 { return 0.75 }

var nullableToStringRe = regexp.MustCompile(`\?\s*\.\s*toString\(\)`)
var nullableToStringReceiverRe = regexp.MustCompile(`(\w+)\?\s*\.\s*toString\(\)`)

func (r *NullableToStringCallRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	if !nullableToStringRe.MatchString(text) {
		return
	}
	// If resolver is available, check if receiver is actually nullable
	if r.resolver != nil {
		if m := nullableToStringReceiverRe.FindStringSubmatch(text); m != nil {
			resolved := r.resolver.ResolveByNameFlat(m[1], idx, file)
			if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && !resolved.IsNullable() {
				return // known non-null, skip
			}
		}
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Calling toString() on a nullable type. Use '\\.toString()' with safe call or string templates instead."))
}
