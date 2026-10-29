package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// UnnecessaryNotNullOperatorRule detects !! on non-null.
// ---------------------------------------------------------------------------
type UnnecessaryNotNullOperatorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — flags !! on a
// non-null type; relies on resolver for nullability. Heuristic fallback is
// conservative but noisy. Classified per roadmap/17.
func (r *UnnecessaryNotNullOperatorRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnnecessaryNotNullOperatorRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	if !flatPostfixHasNotNullAssertion(file, idx) {
		return
	}

	// Trust the compiler's UNNECESSARY_NOT_NULL_ASSERTION verdict when the
	// oracle supplies it: it proves the `!!` is redundant without the
	// heuristic guards below, and covers dotted / complex receivers the
	// name-based fallback deliberately skips. Falls through to the heuristic
	// when no oracle diagnostic is present.
	if projectDiagnostic(ctx, DiagnosticProjection{
		RuleID:       "UnnecessaryNotNullOperator",
		FactoryNames: []string{"UNNECESSARY_NOT_NULL_ASSERTION"},
		Message: func(ctx *api.Context, _ oracle.Diagnostic) string {
			recv := strings.TrimSpace(strings.TrimSuffix(ctx.File.FlatNodeText(ctx.Idx), "!!"))
			return fmt.Sprintf("Unnecessary not-null assertion (!!) on non-nullable '%s'.", recv)
		},
		Fix: func(ctx *api.Context, _ oracle.Diagnostic) *scanner.Fix {
			end := int(ctx.File.FlatEndByte(ctx.Idx))
			return &scanner.Fix{ByteMode: true, StartByte: end - 2, EndByte: end, Replacement: ""}
		},
		Confidence: api.ConfidenceVeryHigh,
	}) {
		return
	}

	// An indexed operand `m[k]!!` is nullable for a Map receiver; the name-based
	// resolution path below cannot disambiguate it from a non-null List/Array
	// get, so skip it structurally to avoid a false positive.
	if flatExpressionIsIndexing(file, flatFirstNamedChild(file, idx)) {
		return
	}

	// Extract the receiver name (everything before !!)
	receiver := strings.TrimSuffix(text, "!!")
	receiver = strings.TrimSpace(receiver)

	// Skip idiomatic Android/Java platform API patterns where !! is required.
	if isIdiomaticNullAssertionReceiver(receiver, file, flatFirstNamedChild(file, idx)) {
		return
	}

	// Only resolve simple identifiers — dotted member accesses cannot be
	// reliably resolved by bare name lookup (risk of name collision).
	if !strings.Contains(receiver, ".") {
		name := strings.TrimSpace(receiver)
		if name == "this" && nullableThisFromLambdaReceiverCallFlat(file, idx, ctx.Resolver) {
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
		if ctx.Resolver != nil {
			resolved := ctx.Resolver.ResolveByNameFlat(name, idx, file)
			if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
				if !resolved.Resolved {
					return
				}
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
		if sameFileExplicitNonNullValueDeclaration(file, idx, name) {
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				fmt.Sprintf("Unnecessary not-null assertion (!!) on non-nullable val '%s'.", name))
			bangStart := int(file.FlatEndByte(idx)) - 2
			f.Fix = &scanner.Fix{ByteMode: true, StartByte: bangStart, EndByte: int(file.FlatEndByte(idx)), Replacement: ""}
			ctx.Emit(f)
			return
		}

	}
}

func flatPostfixHasNotNullAssertion(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "postfix_expression" {
		return false
	}
	found := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "!!" || file.FlatNodeTextEquals(child, "!!") {
			found = true
			break
		}
	}
	return found
}

// hasBranchNullInitializer returns true if the given name is declared in
// the file with an initializer that has an `else null` / `-> null` /
// `?: null` tail, producing a nullable inferred type that a conservative
// resolver may widen incorrectly. Used to suppress false positives on
// UnnecessaryNotNullOperator / UnnecessarySafeCall.
func hasBranchNullInitializer(file *scanner.File, name string) bool {
	return nullSafetySummaryFor(file).branchNullInitializer[name]
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
		varDecl, _ := file.FlatFindChild(prop, "variable_declaration")
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
	return localMatch
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

// nullSafetyFileSummary collects the per-file facts needed by the four
// name-keyed helper queries used by UnnecessarySafeCall and
// UnnecessaryNotNullOperator: mutable `var` names, explicitly-nullable
// declarations, declarations whose initializer has a branch-null tail,
// and declarations whose initializer is a member access.
//
// Built lazily per file on first use and cached in nullSafetyFileCache
// keyed by file.Path. The single-pass build replaces what used to be
// four independent O(lines) scans per `?.` / `!!` receiver name, which
// was the dominant UnnecessarySafeCall cost on large corpora.
//
// Semantics of every map are preserved from the original helpers. In
// particular `memberAccessInitializer` records only the first matching
// declaration line per name (and drops the entry when that first line's
// initializer ends with `!!`) to match the early-return behavior of
// hasMemberAccessInitializer.
type nullSafetyFileSummary struct {
	mutableVar               map[string]bool
	explicitNullable         map[string]bool
	branchNullInitializer    map[string]bool
	memberAccessInitializer  map[string]bool
	uncertainCallInitializer map[string]bool
}

// nullSafetySummaryFor returns a cached summary for the file, building
// it on first call. Backed by the shared filefacts cache.
func nullSafetySummaryFor(file *scanner.File) *nullSafetyFileSummary {
	if file == nil {
		return emptyNullSafetySummary
	}
	return filefacts.FileFact(fileFactsCache(), file, slotNullSafety, func() *nullSafetyFileSummary {
		return buildNullSafetySummary(file)
	})
}

var emptyNullSafetySummary = &nullSafetyFileSummary{
	mutableVar:               map[string]bool{},
	explicitNullable:         map[string]bool{},
	branchNullInitializer:    map[string]bool{},
	memberAccessInitializer:  map[string]bool{},
	uncertainCallInitializer: map[string]bool{},
}

func buildNullSafetySummary(file *scanner.File) *nullSafetyFileSummary {
	s := &nullSafetyFileSummary{
		mutableVar:               make(map[string]bool),
		explicitNullable:         make(map[string]bool),
		branchNullInitializer:    make(map[string]bool),
		memberAccessInitializer:  make(map[string]bool),
		uncertainCallInitializer: make(map[string]bool),
	}
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		name := propertyDeclarationName(file, prop)
		if name == "" {
			name = extractIdentifierFlat(file, prop)
		}
		if name == "" {
			return
		}
		if propertyDeclarationIsVar(file, prop) {
			s.mutableVar[name] = true
		}
		if declarationHasNullableTypeFlat(file, prop) {
			s.explicitNullable[name] = true
		}
		init := propertyInitializerExpression(file, prop)
		if init == 0 {
			return
		}
		if !propertyDeclarationIsVar(file, prop) && initializerIsUnresolvedMemberAccessFlat(file, init) {
			s.memberAccessInitializer[name] = true
		}
		if !propertyDeclarationIsVar(file, prop) &&
			!declarationHasExplicitTypeFlat(file, prop) &&
			initializerIsUncertainCallFlat(file, init) {
			s.uncertainCallInitializer[name] = true
		}
		if initializerCanProduceNullFlat(file, init) {
			s.branchNullInitializer[name] = true
		}
	})
	return s
}

func declarationHasNullableTypeFlat(file *scanner.File, decl uint32) bool {
	if file == nil || decl == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(decl, func(candidate uint32) {
		if !found && file.FlatType(candidate) == "nullable_type" {
			found = true
		}
	})
	return found
}

func initializerIsUnresolvedMemberAccessFlat(file *scanner.File, init uint32) bool {
	if file == nil || init == 0 {
		return false
	}
	init = flatUnwrapParenExpr(file, init)
	if file.FlatType(init) == "postfix_expression" && flatPostfixHasNotNullAssertion(file, init) {
		return false
	}
	return file.FlatType(init) == "navigation_expression" && !flatNavigationHasSafeCall(file, init)
}

func initializerIsUncertainCallFlat(file *scanner.File, init uint32) bool {
	if file == nil || init == 0 {
		return false
	}
	init = flatUnwrapParenExpr(file, init)
	if file.FlatType(init) == "postfix_expression" && flatPostfixHasNotNullAssertion(file, init) {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(init))
	if strings.Contains(text, ")(") {
		return true
	}
	if file.FlatType(init) == "call_expression" {
		name := flatCallExpressionName(file, init)
		return callNameCanReturnNullByConvention(name)
	}
	if file.FlatType(init) == "navigation_expression" {
		uncertain := false
		file.FlatWalkAllNodes(init, func(candidate uint32) {
			if uncertain || file.FlatType(candidate) != "call_expression" {
				return
			}
			name := flatCallExpressionName(file, candidate)
			if callNameCanReturnNullByConvention(name) {
				uncertain = true
			}
		})
		return uncertain
	}
	return false
}

func callNameCanReturnNullByConvention(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasSuffix(name, "OrNull") || strings.HasPrefix(name, "find")
}

func initializerCanProduceNullFlat(file *scanner.File, init uint32) bool {
	if file == nil || init == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(init, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "null":
			found = true
		case "navigation_expression":
			if flatNavigationHasSafeCall(file, candidate) {
				found = true
			}
		}
	})
	return found
}
