package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/experiment"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func isRequireFunctionBangBodyFlat(file *scanner.File, idx uint32) bool {
	var fn uint32
	hops := 0
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		hops++
		if hops > 6 {
			return false
		}
		t := file.FlatType(p)
		if t == "function_declaration" {
			fn = p
			break
		}
		switch t {
		case "statements", "lambda_literal", "if_expression", "when_expression", "try_expression", "control_structure_body":
			return false
		}
	}
	if fn == 0 {
		return false
	}
	name := extractIdentifierFlat(file, fn)
	if !strings.HasPrefix(name, "require") {
		return false
	}
	if len(name) > len("require") {
		c := name[len("require")]
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	fnText := file.FlatNodeText(fn)
	if !strings.Contains(fnText, "=") {
		return false
	}
	afterEq := strings.SplitN(fnText, "=", 2)
	if len(afterEq) != 2 {
		return false
	}
	body := strings.TrimSpace(afterEq[1])
	if strings.HasPrefix(body, "{") {
		return false
	}
	return true
}

func isGuardedNonNullFlat(file *scanner.File, idx uint32, receiver uint32) bool {
	if receiver == 0 {
		return false
	}
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		t := file.FlatType(current)
		if t == "function_declaration" || t == "lambda_literal" {
			return false
		}
		if t != "control_structure_body" {
			continue
		}
		parent, ok := file.FlatParent(current)
		if !ok || file.FlatType(parent) != "if_expression" {
			continue
		}
		var cond uint32
		var thenBody uint32
		var elseBody uint32
		foundElse := false
		for i := 0; i < file.FlatChildCount(parent); i++ {
			c := file.FlatChild(parent, i)
			if c == 0 {
				continue
			}
			switch file.FlatType(c) {
			case "parenthesized_expression", "check_expression", "conjunction_expression",
				"disjunction_expression", "equality_expression", "comparison_expression",
				"prefix_expression", "call_expression", "navigation_expression":
				if cond == 0 {
					cond = c
				}
			case "control_structure_body":
				if !foundElse && thenBody == 0 {
					thenBody = c
				} else if foundElse && elseBody == 0 {
					elseBody = c
				}
			case "else":
				foundElse = true
			}
		}
		if cond == 0 {
			continue
		}
		if thenBody == current && conditionTrueProvesNonNullFlat(file, cond, receiver, idx) {
			return true
		}
		if elseBody == current && conditionFalseProvesNonNullFlat(file, cond, receiver, idx) {
			return true
		}
	}
	return false
}

func isEarlyReturnGuardedFlat(file *scanner.File, idx uint32, receiver uint32) bool {
	if receiver == 0 {
		return false
	}
	var anchor uint32
	var statements uint32
	child := idx
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "function_declaration" || t == "lambda_literal" {
			break
		}
		if t == "statements" {
			statements = p
			anchor = child
			break
		}
		child = p
	}
	if statements == 0 || anchor == 0 {
		return false
	}
	for i := 0; i < file.FlatNamedChildCount(statements); i++ {
		stmt := file.FlatNamedChild(statements, i)
		if stmt == 0 {
			continue
		}
		if stmt == anchor || file.FlatStartByte(stmt) >= file.FlatStartByte(anchor) {
			break
		}
		if file.FlatType(stmt) != "if_expression" {
			continue
		}
		hasElse := false
		var cond uint32
		var thenBody uint32
		for j := 0; j < file.FlatChildCount(stmt); j++ {
			c := file.FlatChild(stmt, j)
			if c == 0 {
				continue
			}
			switch file.FlatType(c) {
			case "else":
				hasElse = true
			case "parenthesized_expression", "check_expression", "conjunction_expression",
				"disjunction_expression", "equality_expression", "comparison_expression",
				"prefix_expression", "call_expression", "navigation_expression":
				if cond == 0 {
					cond = c
				}
			case "control_structure_body":
				if thenBody == 0 {
					thenBody = c
				}
			}
		}
		if hasElse || cond == 0 || thenBody == 0 {
			continue
		}
		if !bodyAlwaysExitsFlat(file, thenBody) {
			continue
		}
		if conditionFalseProvesNonNullFlat(file, cond, receiver, idx) {
			return true
		}
	}
	return false
}

func isPostFilterSmartCastFlat(file *scanner.File, idx uint32, receiverText string) bool {
	base := strings.TrimSuffix(receiverText, ".")
	if !strings.HasPrefix(base, "it.") && base != "it" {
		return false
	}
	field := strings.TrimPrefix(base, "it.")
	var lambda uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			return false
		case "lambda_literal":
			lambda = p
			break
		}
		if lambda != 0 {
			break
		}
	}
	if lambda == 0 {
		return false
	}
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "call_expression" {
			if file.FlatType(p) == "function_declaration" {
				return false
			}
			continue
		}
		chain := p
		for {
			parent, ok := file.FlatParent(chain)
			if !ok {
				break
			}
			pt := file.FlatType(parent)
			if pt != "navigation_expression" && pt != "call_expression" {
				break
			}
			chain = parent
		}
		navExpr, _ := file.FlatFindChild(chain, "navigation_expression")
		if navExpr == 0 {
			continue
		}
		callee := flatNavigationExpressionLastIdentifier(file, navExpr)
		switch callee {
		case "map", "mapNotNull", "mapIndexed", "flatMap", "forEach",
			"forEachIndexed", "associate", "associateBy", "associateWith",
			"sortedBy", "sortedByDescending", "groupBy", "onEach", "let":
		default:
			continue
		}
		filterNeedles := []string{
			".filter { it." + field + " != null }",
			".filter { it." + field + " != null}",
			".filter { it." + field + "!= null }",
		}
		cur := navExpr
		for i := 0; i < 8; i++ {
			if cur == 0 || file.FlatNamedChildCount(cur) == 0 {
				return false
			}
			recv := file.FlatNamedChild(cur, 0)
			if recv == 0 {
				return false
			}
			if file.FlatType(recv) == "call_expression" {
				recvCallee, _ := file.FlatFindChild(recv, "navigation_expression")
				if recvCallee != 0 {
					last := flatNavigationExpressionLastIdentifier(file, recvCallee)
					if last == "filter" || last == "filterKeys" || last == "filterValues" {
						recvText := file.FlatNodeText(recv)
						for _, needle := range filterNeedles {
							if strings.Contains(recvText, needle) {
								return true
							}
						}
						if strings.Contains(recvText, ".filter {") &&
							strings.Contains(recvText, "it."+field+" != null") {
							return true
						}
					}
					cur = recvCallee
					continue
				}
			}
			if file.FlatType(recv) == "navigation_expression" {
				cur = recv
				continue
			}
			return false
		}
	}
	return false
}

func isMapContainsKeyGuardedFlat(file *scanner.File, idx uint32, receiver, key uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "lambda_literal" {
			break
		}
		if file.FlatType(p) != "control_structure_body" {
			continue
		}
		parent, ok := file.FlatParent(p)
		if !ok || file.FlatType(parent) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := flatIfConditionBodies(file, parent)
		if cond == 0 {
			continue
		}
		if thenBody == p && mapContainsKeyConditionProves(file, cond, receiver, key, true) {
			return true
		}
		if elseBody == p && mapContainsKeyConditionProves(file, cond, receiver, key, false) {
			return true
		}
	}
	return false
}

func isEarlyReturnMapContainsKeyGuardedFlat(file *scanner.File, idx uint32, receiver, key uint32) bool {
	var anchor uint32
	var statements uint32
	child := idx
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "function_declaration" || t == "lambda_literal" {
			break
		}
		if t == "statements" {
			statements = p
			anchor = child
			break
		}
		child = p
	}
	if statements == 0 || anchor == 0 {
		return false
	}
	for stmt := file.FlatFirstChild(statements); stmt != 0; stmt = file.FlatNextSib(stmt) {
		if !file.FlatIsNamed(stmt) {
			continue
		}
		if stmt == anchor || file.FlatStartByte(stmt) >= file.FlatStartByte(anchor) {
			break
		}
		if file.FlatType(stmt) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := flatIfConditionBodies(file, stmt)
		if cond == 0 || thenBody == 0 || elseBody != 0 {
			continue
		}
		if !bodyAlwaysExitsFlat(file, thenBody) {
			continue
		}
		if mapContainsKeyConditionProves(file, cond, receiver, key, false) {
			return true
		}
	}
	return false
}

func isInsideContainsKeyFilterChainFlat(file *scanner.File, idx uint32, receiver uint32) bool {
	var lambda uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "lambda_literal" {
			lambda = p
			break
		}
		if t == "function_declaration" || t == "source_file" {
			return false
		}
	}
	if lambda == 0 {
		return false
	}
	var transformCall uint32
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "call_expression" {
			transformCall = p
			break
		}
		if t == "function_declaration" || t == "source_file" {
			return false
		}
	}
	if transformCall == 0 {
		return false
	}
	navExpr, _ := file.FlatFindChild(transformCall, "navigation_expression")
	if navExpr == 0 {
		return false
	}
	callee := flatNavigationExpressionLastIdentifier(file, navExpr)
	switch callee {
	case "map", "mapNotNull", "mapIndexed", "flatMap", "forEach",
		"forEachIndexed", "associate", "associateBy", "associateWith",
		"sortedBy", "sortedByDescending", "groupBy", "onEach", "let":
	default:
		return false
	}
	cur := navExpr
	for i := 0; i < 8; i++ {
		if cur == 0 || file.FlatNamedChildCount(cur) == 0 {
			return false
		}
		recv := file.FlatNamedChild(cur, 0)
		if recv == 0 {
			return false
		}
		if file.FlatType(recv) == "call_expression" {
			recvCallee, _ := file.FlatFindChild(recv, "navigation_expression")
			if recvCallee != 0 {
				last := flatNavigationExpressionLastIdentifier(file, recvCallee)
				if last == "filter" || last == "filterKeys" || last == "filterValues" {
					if flatSubtreeHasContainsKeyForReceiver(file, recv, receiver) {
						return true
					}
				}
				cur = recvCallee
				continue
			}
		}
		if file.FlatType(recv) == "navigation_expression" {
			cur = recv
			continue
		}
		return false
	}
	return false
}

func flatIfConditionBodies(file *scanner.File, ifExpr uint32) (cond, thenBody, elseBody uint32) {
	foundElse := false
	for child := file.FlatFirstChild(ifExpr); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "else":
			foundElse = true
		case "control_structure_body":
			if !foundElse && thenBody == 0 {
				thenBody = child
			} else if foundElse && elseBody == 0 {
				elseBody = child
			}
		default:
			if cond == 0 && file.FlatIsNamed(child) && file.FlatType(child) != "control_structure_body" {
				cond = child
			}
		}
	}
	return cond, thenBody, elseBody
}

func mapContainsKeyConditionProves(file *scanner.File, cond, receiver, key uint32, truth bool) bool {
	proves := false
	file.FlatWalkAllNodes(cond, func(candidate uint32) {
		if proves || file.FlatType(candidate) != "call_expression" {
			return
		}
		if !mapContainsKeyCallMatches(file, candidate, receiver, key) {
			return
		}
		negated := flatCallNegatedWithin(file, candidate, cond)
		if truth {
			if !negated && !flatHasAncestorBetween(file, candidate, cond, "disjunction_expression") {
				proves = true
			}
			return
		}
		if negated && !flatHasAncestorBetween(file, candidate, cond, "conjunction_expression") {
			proves = true
		}
	})
	return proves
}

func mapContainsKeyCallMatches(file *scanner.File, call, receiver, key uint32) bool {
	nav, args := flatCallExpressionParts(file, call)
	if nav == 0 || args == 0 || flatNavigationExpressionLastIdentifier(file, nav) != "containsKey" {
		return false
	}
	if flatNavigationLastSuffixHasSafeAccess(file, nav) {
		return false
	}
	callReceiver := flatNavigationExpressionReceiver(file, nav)
	if callReceiver == 0 || !flatExpressionsEquivalent(file, callReceiver, receiver) {
		return false
	}
	arg, ok := flatSingleValueArgumentExpression(file, args)
	return ok && flatExpressionsEquivalent(file, arg, key)
}

func flatSubtreeHasContainsKeyForReceiver(file *scanner.File, root, receiver uint32) bool {
	found := false
	file.FlatWalkAllNodes(root, func(candidate uint32) {
		if found || file.FlatType(candidate) != "call_expression" {
			return
		}
		nav, _ := flatCallExpressionParts(file, candidate)
		if nav == 0 || flatNavigationExpressionLastIdentifier(file, nav) != "containsKey" {
			return
		}
		callReceiver := flatNavigationExpressionReceiver(file, nav)
		if callReceiver != 0 && flatExpressionsEquivalent(file, callReceiver, receiver) {
			found = true
		}
	})
	return found
}

func flatCallNegatedWithin(file *scanner.File, call, root uint32) bool {
	negated := false
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "prefix_expression" && flatPrefixExpressionIsBang(file, p) {
			negated = !negated
		}
		if p == root {
			break
		}
	}
	return negated
}

func flatPrefixExpressionIsBang(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatType(child) == "!" {
			return true
		}
	}
	return false
}

func flatHasAncestorBetween(file *scanner.File, idx, root uint32, nodeType string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if p == root {
			return false
		}
		if file.FlatType(p) == nodeType {
			return true
		}
	}
	return false
}

func flatExpressionsEquivalent(file *scanner.File, a, b uint32) bool {
	a = flatUnwrapParenExpr(file, a)
	b = flatUnwrapParenExpr(file, b)
	if a == 0 || b == 0 {
		return false
	}
	if a == b {
		return true
	}
	if file.FlatType(a) != file.FlatType(b) {
		return false
	}
	return strings.TrimSpace(file.FlatNodeText(a)) == strings.TrimSpace(file.FlatNodeText(b))
}

// ---------------------------------------------------------------------------
// UnsafeCallOnNullableTypeRule detects !! operator usage.
// ---------------------------------------------------------------------------
type UnsafeCallOnNullableTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs null safety rule. Detection leans on structural patterns
// around nullable expressions and has a heuristic fallback when the
// resolver is absent. Classified per roadmap/17.
func (r *UnsafeCallOnNullableTypeRule) Confidence() float64 { return 0.75 }

func (r *UnsafeCallOnNullableTypeRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	if !strings.HasSuffix(text, "!!") {
		return
	}

	// Skip test sources — tests use `!!` freely on setup fixtures;
	// a NullPointerException there is just a failed test, not a runtime
	// bug affecting production.
	if isTestFile(file.Path) {
		return
	}
	// Skip Gradle / Kotlin script files — script blocks commonly use
	// `listFiles()!!`, `project.findProperty(...)!!`, and similar
	// patterns where the alternative is more verbose boilerplate.
	if strings.HasSuffix(file.Path, ".gradle.kts") ||
		strings.HasSuffix(file.Path, ".main.kts") ||
		strings.HasSuffix(file.Path, ".kts") {
		return
	}
	// Skip @Preview / sample / fixture functions — these are UI tooling
	// scaffolding with hand-crafted test data, and `!!` is used liberally
	// to build fixtures without null-handling noise.
	if isInsidePreviewOrSampleFunctionFlat(file, idx) {
		return
	}
	// Skip proto-processor files: any Kotlin file importing Wire /
	// com.google.protobuf / Signal's generated proto packages is treated
	// as a "proto processor". Wire-generated fields are all nullable by
	// type but required at runtime, and `!!` is the idiomatic unwrap.
	// Skip only pure dotted field-chain receivers (2+ segments, no
	// parentheses), preserving checks on single-identifier locals and
	// method-call chains.
	if fileImportsProto(file) && isDottedFieldChain(strings.TrimSuffix(text, "!!")) {
		return
	}
	// Skip idiomatic Android patterns where !! is the canonical way to
	// consume platform-typed APIs:
	//   - Bundle.getX(...)!!, requireArguments().getX()!!
	//   - Parcel.readX()!! in Parcelable constructors
	//   - Intent.getX(...)!! / Intent.extras!!
	receiverText := strings.TrimSuffix(text, "!!")
	// De-dup with MapGetWithNotNullAssertionOperator: map[key]!! / foo.get(k)!!
	// is the sibling rule's concern.
	if strings.HasSuffix(receiverText, "]") {
		return
	}
	if isIdiomaticNullAssertionReceiver(receiverText, file) {
		return
	}
	// Normalize the receiver: strip inner `!!` and `this.` so that
	// `dialog!!.window` and `this.window` match the plain `window` in
	// the allowlist.
	normalized := strings.ReplaceAll(receiverText, "!!", "")
	normalized = strings.TrimPrefix(normalized, "this.")
	if normalized != receiverText && isIdiomaticNullAssertionReceiver(normalized, file) {
		return
	}

	// Flow-sensitive guard: if the receiver expression (or its leading
	// safe-call chain) is proven non-null by an enclosing `if (x != null)`
	// or `if (x?.y != null)` branch, the `!!` is a smart-cast workaround
	// rather than an unsafe assertion.
	receiverIdx := uint32(0)
	if file.FlatChildCount(idx) >= 1 {
		receiverIdx = file.FlatChild(idx, 0)
	}
	if isGuardedNonNullFlat(file, idx, receiverIdx) {
		return
	}
	// Early-return guard: `if (x == null) return` earlier in the same block
	// proves non-null for any subsequent `x!!` in the same statements scope.
	if isEarlyReturnGuardedFlat(file, idx, receiverIdx) {
		return
	}
	// Post-filter smart cast: `.filter { it.x != null }.map { it.x!! }` —
	// if an enclosing lambda is inside a `.map` / `.forEach` / `.let` call
	// whose chain has a preceding `.filter { it.<field> != null }`, the
	// subsequent `!!` on that field is safe.
	if isPostFilterSmartCastFlat(file, idx, receiverText) {
		return
	}
	// `fun requireXxx(): T = field!!` — the function name explicitly
	// documents the precondition ("the caller must have verified this").
	// The `!!` is the idiomatic implementation. Detekt skips these too.
	if experiment.Enabled("unsafe-call-skip-require-function-body") &&
		isRequireFunctionBangBodyFlat(file, idx) {
		return
	}

	// If resolver is available, check if the receiver is known non-null.
	// If so, suppress the finding — it's not actually unsafe.
	if ctx.Resolver != nil && file.FlatChildCount(idx) >= 1 {
		isNull := ctx.Resolver.IsNullableFlat(file.FlatChild(idx, 0), file)
		if isNull != nil && !*isNull {
			return // receiver is known non-null, !! is safe
		}
	}

	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Not-null assertion operator (!!) used. Consider using safe calls (?.) instead."))
}

// fileImportsProto returns true if the Kotlin file imports any Wire or
// Signal-generated protobuf package. Proto fields are structurally
// nullable but conventionally required; `!!` is idiomatic.
func fileImportsProto(file *scanner.File) bool {
	// Simple scan over the file's content for import lines mentioning
	// proto-related packages. Limited to the top 100 lines to bound cost.
	content := string(file.Content)
	upper := len(content)
	if upper > 8000 {
		upper = 8000
	}
	header := content[:upper]
	return strings.Contains(header, "import com.squareup.wire") ||
		strings.Contains(header, "import com.google.protobuf") ||
		strings.Contains(header, ".databaseprotos.") ||
		strings.Contains(header, ".storageservice.protos.") ||
		strings.Contains(header, ".signalservice.protos.") ||
		strings.Contains(header, "signalservice.internal.push")
}

// fileImportsKsp reports whether the file imports KSP symbol-processing APIs.
func fileImportsKsp(file *scanner.File) bool {
	content := string(file.Content)
	upper := len(content)
	if upper > 8000 {
		upper = 8000
	}
	header := content[:upper]
	return strings.Contains(header, "import com.google.devtools.ksp")
}

// fileImportsCompilerApis reports whether the file imports Kotlin compiler
// IR / backend / FIR / analysis APIs.
func fileImportsCompilerApis(file *scanner.File) bool {
	content := string(file.Content)
	upper := len(content)
	if upper > 8000 {
		upper = 8000
	}
	header := content[:upper]
	return strings.Contains(header, "import org.jetbrains.kotlin.ir") ||
		strings.Contains(header, "import org.jetbrains.kotlin.backend") ||
		strings.Contains(header, "import org.jetbrains.kotlin.fir") ||
		strings.Contains(header, "import org.jetbrains.kotlin.analysis")
}

// isDottedFieldChain returns true if s looks like `a.b`, `a.b.c`, etc. —
// a pure dotted identifier chain with at least one `.` and no method
// call parentheses or subscript brackets.
func isDottedFieldChain(s string) bool {
	if !strings.Contains(s, ".") {
		return false
	}
	if strings.ContainsAny(s, "()[]") {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' || c == '_' ||
			(c >= '0' && c <= '9') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') {
			continue
		}
		return false
	}
	return true
}

func conditionTrueProvesNonNullFlat(file *scanner.File, cond, receiver, useIdx uint32) bool {
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "equality_expression":
		nonNull, _, ok := equalityNullFactsFlat(file, cond, receiver, useIdx)
		return ok && nonNull
	case "conjunction_expression":
		return anyConditionOperandFlat(file, cond, func(child uint32) bool {
			return conditionTrueProvesNonNullFlat(file, child, receiver, useIdx)
		})
	case "disjunction_expression":
		return allConditionOperandsFlat(file, cond, func(child uint32) bool {
			return conditionTrueProvesNonNullFlat(file, child, receiver, useIdx)
		})
	case "prefix_expression":
		if prefixExpressionIsNotFlat(file, cond) {
			return conditionFalseProvesNonNullFlat(file, flatLastNamedChild(file, cond), receiver, useIdx)
		}
	}
	return false
}

func conditionTrueProvesNullFlat(file *scanner.File, cond, receiver, useIdx uint32) bool {
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "equality_expression":
		_, isNull, ok := equalityNullFactsFlat(file, cond, receiver, useIdx)
		return ok && isNull
	case "conjunction_expression":
		return anyConditionOperandFlat(file, cond, func(child uint32) bool {
			return conditionTrueProvesNullFlat(file, child, receiver, useIdx)
		})
	case "disjunction_expression":
		return allConditionOperandsFlat(file, cond, func(child uint32) bool {
			return conditionTrueProvesNullFlat(file, child, receiver, useIdx)
		})
	case "prefix_expression":
		if prefixExpressionIsNotFlat(file, cond) {
			return conditionFalseProvesNullFlat(file, flatLastNamedChild(file, cond), receiver, useIdx)
		}
	}
	return false
}

func conditionFalseProvesNonNullFlat(file *scanner.File, cond, receiver, useIdx uint32) bool {
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "equality_expression":
		_, isNull, ok := equalityNullFactsFlat(file, cond, receiver, useIdx)
		return ok && isNull
	case "call_expression":
		return nullPredicateCallFalseProvesNonNullFlat(file, cond, receiver, useIdx)
	case "disjunction_expression":
		return anyConditionOperandFlat(file, cond, func(child uint32) bool {
			return conditionFalseProvesNonNullFlat(file, child, receiver, useIdx)
		})
	case "conjunction_expression":
		return allConditionOperandsFlat(file, cond, func(child uint32) bool {
			return conditionFalseProvesNonNullFlat(file, child, receiver, useIdx)
		})
	case "prefix_expression":
		if prefixExpressionIsNotFlat(file, cond) {
			return conditionTrueProvesNonNullFlat(file, flatLastNamedChild(file, cond), receiver, useIdx)
		}
	}
	return false
}

func conditionFalseProvesNullFlat(file *scanner.File, cond, receiver, useIdx uint32) bool {
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "equality_expression":
		nonNull, _, ok := equalityNullFactsFlat(file, cond, receiver, useIdx)
		return ok && nonNull
	case "conjunction_expression":
		return allConditionOperandsFlat(file, cond, func(child uint32) bool {
			return conditionFalseProvesNullFlat(file, child, receiver, useIdx)
		})
	case "disjunction_expression":
		return anyConditionOperandFlat(file, cond, func(child uint32) bool {
			return conditionFalseProvesNullFlat(file, child, receiver, useIdx)
		})
	case "prefix_expression":
		if prefixExpressionIsNotFlat(file, cond) {
			return conditionTrueProvesNullFlat(file, flatLastNamedChild(file, cond), receiver, useIdx)
		}
	}
	return false
}

func equalityNullFactsFlat(file *scanner.File, expr, receiver, useIdx uint32) (nonNull bool, isNull bool, ok bool) {
	if file == nil || expr == 0 || file.FlatType(expr) != "equality_expression" || file.FlatChildCount(expr) < 3 {
		return false, false, false
	}
	left := flatUnwrapParenExpr(file, file.FlatChild(expr, 0))
	op := file.FlatChild(expr, 1)
	right := flatUnwrapParenExpr(file, file.FlatChild(expr, 2))
	if left == 0 || op == 0 || right == 0 {
		return false, false, false
	}

	var candidate uint32
	switch {
	case flatIsNullLiteral(file, right):
		candidate = left
	case flatIsNullLiteral(file, left):
		candidate = right
	default:
		return false, false, false
	}
	if !conditionReferenceMatchesReceiverFlat(file, candidate, receiver, useIdx) {
		return false, false, false
	}

	switch strings.TrimSpace(file.FlatNodeText(op)) {
	case "!=":
		return true, false, true
	case "==":
		return false, true, true
	default:
		return false, false, false
	}
}

func nullPredicateCallFalseProvesNonNullFlat(file *scanner.File, call, receiver, useIdx uint32) bool {
	navExpr, args := flatCallExpressionParts(file, call)
	if navExpr == 0 {
		return false
	}
	path, ok := flatReferencePathFromExpr(file, navExpr)
	if !ok || len(path.parts) == 0 {
		return false
	}
	callee := path.parts[len(path.parts)-1]
	switch callee {
	case "isNullOrEmpty", "isNullOrBlank":
		if len(path.parts) < 2 {
			return false
		}
		receiverExpr := file.FlatNamedChild(navExpr, 0)
		return conditionReferenceMatchesReceiverFlat(file, receiverExpr, receiver, useIdx)
	case "isEmpty":
		if len(path.parts) != 2 || path.parts[0] != "TextUtils" || args == 0 {
			return false
		}
		firstArg := flatPositionalValueArgument(file, args, 0)
		if firstArg == 0 {
			return false
		}
		return conditionReferenceMatchesReceiverFlat(file, flatValueArgumentExpression(file, firstArg), receiver, useIdx)
	default:
		return false
	}
}

func conditionReferenceMatchesReceiverFlat(file *scanner.File, candidate, receiver, useIdx uint32) bool {
	candidate = flatUnwrapParenExpr(file, candidate)
	receiver = flatUnwrapParenExpr(file, receiver)
	candPath, candOK := flatReferencePathFromExpr(file, candidate)
	recvPath, recvOK := flatReferencePathFromExpr(file, receiver)
	if !candOK || !recvOK {
		return false
	}
	if referencePathsMatchReceiverFlat(file, candPath, recvPath, useIdx) {
		return true
	}
	candTrimmed, candHadThis := flatTrimLeadingThisPath(candPath)
	recvTrimmed, recvHadThis := flatTrimLeadingThisPath(recvPath)
	if !candHadThis && !recvHadThis {
		return false
	}
	return referencePathsMatchReceiverFlat(file, candTrimmed, recvTrimmed, useIdx) &&
		sameExplicitThisReferenceTargetFlat(file, candPath, recvPath, useIdx)
}

func referencePathsMatchReceiverFlat(file *scanner.File, candPath, recvPath flatReferencePath, useIdx uint32) bool {
	if len(candPath.parts) != len(recvPath.parts) || len(candPath.parts) == 0 {
		return false
	}
	for i := range candPath.parts {
		if candPath.parts[i] != recvPath.parts[i] {
			return false
		}
	}
	if len(candPath.parts) == 1 {
		return sameResolvableReferenceTargetFlat(file, candPath.nodes[0], recvPath.nodes[0])
	}
	return sameQualifiedReceiverTargetFlat(file, candPath.nodes[0], recvPath.nodes[0], useIdx)
}

type flatReferencePath struct {
	parts []string
	nodes []uint32
	root  uint32
}

func flatReferencePathFromExpr(file *scanner.File, idx uint32) (flatReferencePath, bool) {
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "simple_identifier", "this_expression":
		return flatReferencePath{parts: []string{file.FlatNodeText(idx)}, nodes: []uint32{idx}, root: idx}, true
	case "navigation_expression":
		var out flatReferencePath
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			switch file.FlatType(child) {
			case "simple_identifier", "this_expression", "navigation_expression":
				childPath, ok := flatReferencePathFromExpr(file, child)
				if !ok {
					return flatReferencePath{}, false
				}
				if out.root == 0 {
					out.root = childPath.root
				}
				out.parts = append(out.parts, childPath.parts...)
				out.nodes = append(out.nodes, childPath.nodes...)
			case "navigation_suffix":
				if flatCallSuffixValueArgs(file, child) != 0 {
					return flatReferencePath{}, false
				}
				ident, ok := file.FlatFindChild(child, "simple_identifier")
				if !ok || ident == 0 {
					return flatReferencePath{}, false
				}
				out.parts = append(out.parts, file.FlatNodeText(ident))
				out.nodes = append(out.nodes, ident)
			default:
				return flatReferencePath{}, false
			}
		}
		return out, out.root != 0 && len(out.parts) > 0
	default:
		return flatReferencePath{}, false
	}
}

func flatTrimLeadingThisPath(path flatReferencePath) (flatReferencePath, bool) {
	if len(path.parts) < 2 || path.parts[0] != "this" {
		return path, false
	}
	return flatReferencePath{
		parts: path.parts[1:],
		nodes: path.nodes[1:],
		root:  path.nodes[1],
	}, true
}

func sameExplicitThisReferenceTargetFlat(file *scanner.File, candPath, recvPath flatReferencePath, useIdx uint32) bool {
	candTrimmed, candHadThis := flatTrimLeadingThisPath(candPath)
	recvTrimmed, recvHadThis := flatTrimLeadingThisPath(recvPath)
	if !candHadThis && !recvHadThis {
		return false
	}
	if len(candTrimmed.parts) == 0 || len(recvTrimmed.parts) == 0 {
		return false
	}
	if candHadThis && recvHadThis {
		candClass, candOK := flatEnclosingAncestor(file, candPath.nodes[0], "class_declaration", "object_declaration")
		recvClass, recvOK := flatEnclosingAncestor(file, recvPath.nodes[0], "class_declaration", "object_declaration")
		return candOK && recvOK && candClass == recvClass
	}
	if candHadThis {
		return explicitThisMemberMatchesReferenceFlat(file, candPath.nodes[0], candTrimmed.nodes[0], recvTrimmed.nodes[0], useIdx)
	}
	return explicitThisMemberMatchesReferenceFlat(file, recvPath.nodes[0], recvTrimmed.nodes[0], candTrimmed.nodes[0], useIdx)
}

func explicitThisMemberMatchesReferenceFlat(file *scanner.File, thisNode, memberNameNode, otherRoot uint32, useIdx uint32) bool {
	classNode, ok := flatEnclosingAncestor(file, thisNode, "class_declaration", "object_declaration")
	if !ok {
		return false
	}
	useClass, ok := flatEnclosingAncestor(file, useIdx, "class_declaration", "object_declaration")
	if !ok || useClass != classNode {
		return false
	}
	memberDecl := classMemberDeclarationByNameFlat(file, classNode, file.FlatNodeText(memberNameNode))
	if memberDecl == 0 {
		return false
	}
	return resolveSimpleReferenceDeclarationFlat(file, otherRoot) == memberDecl
}

func classMemberDeclarationByNameFlat(file *scanner.File, classNode uint32, name string) uint32 {
	var found uint32
	file.FlatWalkAllNodes(classNode, func(candidate uint32) {
		if found != 0 || extractIdentifierFlat(file, candidate) != name {
			return
		}
		switch file.FlatType(candidate) {
		case "property_declaration":
			owner, ok := flatEnclosingAncestor(file, candidate, "class_declaration", "object_declaration")
			if ok && owner == classNode {
				found = candidate
			}
		case "class_parameter":
			if parameterDeclaresPropertyFlat(file, candidate) {
				found = candidate
			}
		}
	})
	return found
}

func parameterDeclaresPropertyFlat(file *scanner.File, param uint32) bool {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "val" || file.FlatType(child) == "var" ||
			file.FlatNodeTextEquals(child, "val") || file.FlatNodeTextEquals(child, "var") {
			return true
		}
	}
	return false
}

func sameResolvableReferenceTargetFlat(file *scanner.File, a, b uint32) bool {
	if a == 0 || b == 0 || !file.FlatNodeTextEquals(a, file.FlatNodeText(b)) {
		return false
	}
	declA := resolveSimpleReferenceDeclarationFlat(file, a)
	declB := resolveSimpleReferenceDeclarationFlat(file, b)
	if declA == 0 || declB == 0 {
		return false
	}
	return declA == declB
}

func sameQualifiedReceiverTargetFlat(file *scanner.File, a, b, useIdx uint32) bool {
	if a == 0 || b == 0 {
		return false
	}
	if file.FlatNodeTextEquals(a, "this") && file.FlatNodeTextEquals(b, "this") {
		classA, okA := flatEnclosingAncestor(file, a, "class_declaration", "object_declaration")
		classB, okB := flatEnclosingAncestor(file, b, "class_declaration", "object_declaration")
		return okA && okB && classA == classB
	}
	if sameResolvableReferenceTargetFlat(file, a, b) {
		return true
	}
	ownerA, okA := flatEnclosingAncestor(file, a, "function_declaration", "lambda_literal", "property_declaration")
	ownerB, okB := flatEnclosingAncestor(file, b, "function_declaration", "lambda_literal", "property_declaration")
	ownerUse, okUse := flatEnclosingAncestor(file, useIdx, "function_declaration", "lambda_literal", "property_declaration")
	return okA && okB && okUse && ownerA == ownerB && ownerA == ownerUse && file.FlatNodeTextEquals(a, file.FlatNodeText(b))
}

func resolveSimpleReferenceDeclarationFlat(file *scanner.File, ref uint32) uint32 {
	if file == nil || ref == 0 {
		return 0
	}
	name := file.FlatNodeText(ref)
	if name == "" || name == "this" {
		return 0
	}
	var bestLocal uint32
	var bestMember uint32
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if candidate == 0 || candidate == ref {
			return
		}
		switch file.FlatType(candidate) {
		case "parameter", "class_parameter", "property_declaration":
			if extractIdentifierFlat(file, candidate) != name || !declarationVisibleFromReferenceFlat(file, candidate, ref) {
				return
			}
			if _, local := flatEnclosingAncestor(file, candidate, "function_declaration", "lambda_literal"); local {
				if bestLocal == 0 || file.FlatStartByte(candidate) >= file.FlatStartByte(bestLocal) {
					bestLocal = candidate
				}
				return
			}
			if bestMember == 0 || file.FlatStartByte(candidate) >= file.FlatStartByte(bestMember) {
				bestMember = candidate
			}
		}
	})
	if bestLocal != 0 {
		return bestLocal
	}
	return bestMember
}

func declarationVisibleFromReferenceFlat(file *scanner.File, decl, ref uint32) bool {
	declLocalOwner, declLocal := flatEnclosingAncestor(file, decl, "function_declaration", "lambda_literal")
	refLocalOwner, refLocal := flatEnclosingAncestor(file, ref, "function_declaration", "lambda_literal")
	if declLocal {
		return refLocal && declLocalOwner == refLocalOwner && file.FlatStartByte(decl) <= file.FlatStartByte(ref)
	}

	declClassOwner, declClass := flatEnclosingAncestor(file, decl, "class_declaration", "object_declaration")
	refClassOwner, refClass := flatEnclosingAncestor(file, ref, "class_declaration", "object_declaration")
	if declClass {
		return refClass && declClassOwner == refClassOwner
	}

	return true
}

func flatIsNullLiteral(file *scanner.File, idx uint32) bool {
	idx = flatUnwrapParenExpr(file, idx)
	return idx != 0 && (file.FlatType(idx) == "null" || file.FlatNodeTextEquals(idx, "null"))
}

func prefixExpressionIsNotFlat(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatNodeTextEquals(child, "!") {
			return true
		}
	}
	return false
}

func anyConditionOperandFlat(file *scanner.File, idx uint32, predicate func(uint32) bool) bool {
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child != 0 && predicate(child) {
			return true
		}
	}
	return false
}

func allConditionOperandsFlat(file *scanner.File, idx uint32, predicate func(uint32) bool) bool {
	seen := false
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child == 0 {
			continue
		}
		seen = true
		if !predicate(child) {
			return false
		}
	}
	return seen
}

// isIdiomaticNullAssertionReceiver returns true if the receiver text matches
// a known Android API where !! is the standard (and often only) consumption
// pattern.
func isIdiomaticNullAssertionReceiver(receiver string, file *scanner.File) bool {
	// `_binding!!` — the canonical Fragment ViewBinding idiom.
	// Google's recommended pattern:
	//   private var _binding: FooBinding? = null
	//   private val binding get() = _binding!!
	if strings.HasPrefix(receiver, "_") && !strings.ContainsAny(receiver, "().") {
		return true
	}
	// `binding!!` — accessing the ViewBinding delegate.
	if receiver == "binding" || receiver == "viewBinding" || receiver == "_binding" {
		return true
	}
	// `instance!!` — singleton DCL accessor inside companion object.
	if receiver == "instance" || receiver == "INSTANCE" {
		return true
	}
	// `context!!`, `activity!!`, `arguments!!`, `window!!` — Fragment
	// lifecycle properties that are non-null during the lifecycle window.
	if receiver == "context" || receiver == "activity" ||
		receiver == "arguments" || receiver == "window" ||
		receiver == "dialog" || receiver == "parentFragment" {
		return true
	}
	// `serializedData!!` — canonical Job.Factory.create idiom where the
	// framework always invokes with non-null serialized data despite the
	// nullable parameter type.
	if receiver == "serializedData" {
		return true
	}
	// `alertDialog.window!!` / `dialog!!.window!!` — Dialog.window is
	// nullable only before show(); after lifecycle attach callers
	// universally unwrap it. Match `.window` suffix where the receiver
	// chain contains a dialog-like prefix.
	if strings.HasSuffix(receiver, ".window") {
		low := strings.ToLower(receiver)
		if strings.Contains(low, "dialog") {
			return true
		}
	}
	// Android drawable/resource accessors that are non-null in practice.
	if strings.Contains(receiver, "getDrawable(") ||
		strings.Contains(receiver, "getColorStateList(") ||
		strings.Contains(receiver, "getParcelableExtra(") ||
		strings.Contains(receiver, "getStringExtra(") ||
		strings.Contains(receiver, "getIntExtra(") {
		return true
	}
	// `getSystemService()` / `getSystemService(...)` — Android always
	// returns non-null for a valid constant; reflection-generic variants
	// return T? but authors commonly assert. Already suppressed for
	// UnsafeCast; mirror it here for UnsafeCallOnNullableType.
	if strings.Contains(receiver, "getSystemService") {
		return true
	}
	// KSP / symbol-processing code commonly unwraps declaration qualified
	// names before calling asString(). Those names are nullable on local or
	// anonymous declarations, but processor code usually reaches them only for
	// named top-level symbols.
	if fileImportsKsp(file) && strings.HasSuffix(receiver, ".qualifiedName") {
		return true
	}
	// Circuit's assisted-factory KSP path selects creatorOrConstructor in the
	// surrounding branch before unwrapping its simple name. Gate this on KSP
	// imports so ordinary helper functions with the same variable name still
	// surface as unsafe.
	if fileImportsKsp(file) && receiver == "creatorOrConstructor" {
		return true
	}
	// Kotlin compiler / IR / FIR code commonly resolves symbol metadata via
	// lookup APIs that are guaranteed by the surrounding compiler phase.
	if fileImportsCompilerApis(file) && isCompilerLookupReceiver(receiver) {
		return true
	}
	// ViewModelProvider.Factory idiom — `modelClass.cast(X())!!` is the
	// canonical way to downcast to the requested ViewModel type.
	if strings.Contains(receiver, "modelClass.cast(") ||
		strings.Contains(receiver, ".cast(") {
		return true
	}
	// Wire proto decoding: `ADAPTER.decode(bytes)!!`, cursor blob readers,
	// and other helpers that return T? but are guaranteed non-null when
	// called with valid input.
	if strings.Contains(receiver, ".ADAPTER.decode(") ||
		strings.Contains(receiver, "cursor.requireBlob(") ||
		strings.Contains(receiver, "requireBlob(") ||
		strings.Contains(receiver, "requireNonNullBlob(") {
		return true
	}
	// Wire protobuf generated fields: `envelope.timestamp!!`, etc.
	// Proto3 fields are nullable in Wire but required in Signal's wire
	// protocol by invariant.
	wireProtoFields := []string{
		".timestamp", ".serverTimestamp", ".sourceDevice", ".sourceServiceId",
		".destination", ".destinationServiceId", ".groupId", ".masterKey",
		".content", ".dataMessage", ".syncMessage", ".sent", ".message",
		".type", ".serverGuid", ".ciphertextHash",
		// Signal proto message fields commonly accessed via !! in processors.
		".amount", ".badge", ".metadata", ".redemption", ".accessControl",
		".start", ".length", ".value", ".address", ".body", ".uri",
		".query", ".recipient", ".singleRecipient",
		".callMessage", ".offer", ".answer", ".hangup", ".busy", ".opaque",
		".fetchLatest", ".messageRequestResponse", ".blocked", ".verified",
		".configuration", ".keys", ".storageService", ".contacts",
		".callEvent", ".callLinkUpdate", ".callLogEvent", ".deleteForMe",
		".storyMessage", ".editMessage", ".giftBadge", ".paymentNotification",
		".inAppPayment", ".uploadSpec", ".backupData", ".credentials",
		".cdn", ".avatar", ".viewOnceOpen", ".outgoingPayment",
		".senderDevice", ".needsReceipt", ".serverReceivedTimestamp",
		".remoteDigest", ".aci", ".pni", ".style", ".receiptCredentialPresentation",
		".paymentMethod", ".failureReason", ".cancellationReason",
		// More Wire/Signal proto fields used in processors/exporters.
		".id", ".data_", ".targetSentTimestamp", ".latestRevisionId",
		".direction", ".conversationId", ".event", ".peekInfo",
		".ringUpdate", ".acknowledgedReceipt", ".observedReceipt",
		".flags", ".delete", ".edit", ".reaction", ".thread",
		".sticker", ".preview", ".attachments", ".quote",
	}
	for _, field := range wireProtoFields {
		if strings.HasSuffix(receiver, field) {
			return true
		}
	}
	// Bundle / requireArguments / arguments access
	bundleMethods := []string{
		".getString(", ".getStringArray(", ".getStringArrayList(",
		".getInt(", ".getIntArray(", ".getIntegerArrayList(",
		".getLong(", ".getLongArray(",
		".getFloat(", ".getFloatArray(",
		".getDouble(", ".getDoubleArray(",
		".getBoolean(", ".getBooleanArray(",
		".getByte(", ".getByteArray(",
		".getChar(", ".getCharArray(",
		".getShort(", ".getShortArray(",
		".getParcelable(", ".getParcelableArray(", ".getParcelableArrayList(",
		".getParcelableCompat(", ".getParcelableArrayCompat(",
		".getParcelableArrayListCompat(",
		".getSerializable(", ".getSerializableCompat(",
		".getBundle(", ".getCharSequence(", ".getCharSequenceArray(",
		".getCharSequenceArrayList(",
	}
	for _, m := range bundleMethods {
		if strings.Contains(receiver, m) {
			return true
		}
	}
	// Parcel.readX() patterns
	parcelMethods := []string{
		".readString(", ".readStringArray(", ".readStringList(",
		".readInt(", ".readLong(", ".readFloat(", ".readDouble(",
		".readByte(", ".readByteArray(", ".readBundle(",
		".readParcelable(", ".readParcelableArray(", ".readParcelableList(",
		".readSerializable(",
	}
	for _, m := range parcelMethods {
		if strings.Contains(receiver, m) {
			return true
		}
	}
	// Intent.extras and friends
	if strings.HasSuffix(receiver, ".extras") ||
		strings.HasSuffix(receiver, "intent.extras") {
		return true
	}
	return false
}

// isCompilerLookupReceiver reports compiler-plugin symbol lookups where `!!`
// is the conventional "this lookup must exist" assertion. This keeps the
// rule focused on application code while avoiding noisy compiler/IR codegen
// paths sampled in Metro, Anvil, and Circuit.
func isCompilerLookupReceiver(receiver string) bool {
	return strings.Contains(receiver, "referenceClass(") ||
		strings.Contains(receiver, "primaryConstructor") ||
		strings.Contains(receiver, "classFqName") ||
		strings.Contains(receiver, "getter") ||
		strings.Contains(receiver, "resolveKSClassDeclaration(") ||
		receiver == "classId" || strings.HasSuffix(receiver, ".classId") ||
		receiver == "creatorOrConstructor" || strings.HasSuffix(receiver, ".creatorOrConstructor") ||
		strings.Contains(receiver, "companionObject()")
}

// ---------------------------------------------------------------------------
// MapGetWithNotNullAssertionRule detects map[key]!!.
// ---------------------------------------------------------------------------
type MapGetWithNotNullAssertionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — tree-sitter
// structural check backed by resolver/source type confirmation that the
// receiver is Map-like. Classified per roadmap/17.
func (r *MapGetWithNotNullAssertionRule) Confidence() float64 { return 0.75 }

type mapGetBangAccess struct {
	access   uint32
	receiver uint32
	key      uint32
	safeCall bool
}

func (r *MapGetWithNotNullAssertionRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	// Skip test files — fail-fast `map[key]!!` is idiomatic in tests.
	if isTestFile(file.Path) {
		return
	}
	access, ok := flatMapGetBangAccess(file, idx)
	if !ok {
		return
	}
	if !mapGetReceiverIsMap(ctx, access.receiver) {
		return
	}
	// Skip when the access is guarded by `map.containsKey(key)` in an
	// enclosing if or earlier statement, or by a preceding filter.
	if isMapContainsKeyGuardedFlat(file, idx, access.receiver, access.key) ||
		isEarlyReturnMapContainsKeyGuardedFlat(file, idx, access.receiver, access.key) {
		return
	}
	if experiment.Enabled("map-get-bang-skip-contains-key-filter") &&
		isInsideContainsKeyFilterChainFlat(file, idx, access.receiver) {
		return
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Map access with not-null assertion operator (!!). Use getValue() or getOrDefault() instead.")
	if !access.safeCall {
		f.Fix = &scanner.Fix{
			ByteMode:  true,
			StartByte: int(file.FlatStartByte(idx)),
			EndByte:   int(file.FlatEndByte(idx)),
			Replacement: strings.TrimSpace(file.FlatNodeText(access.receiver)) +
				".getValue(" + strings.TrimSpace(file.FlatNodeText(access.key)) + ")",
		}
	}
	ctx.Emit(f)
}

func flatMapGetBangAccess(file *scanner.File, idx uint32) (mapGetBangAccess, bool) {
	if file == nil || file.FlatType(idx) != "postfix_expression" || !flatPostfixHasBangBang(file, idx) {
		return mapGetBangAccess{}, false
	}
	expr := flatFirstNamedChild(file, idx)
	if expr == 0 {
		return mapGetBangAccess{}, false
	}
	access := flatUnwrapParenExpr(file, expr)
	switch file.FlatType(access) {
	case "indexing_expression":
		receiver, key, ok := flatIndexingExpressionParts(file, access)
		if !ok {
			return mapGetBangAccess{}, false
		}
		return mapGetBangAccess{access: access, receiver: receiver, key: key}, true
	case "call_expression":
		receiver, key, safeCall, ok := flatGetCallExpressionParts(file, access)
		if !ok {
			return mapGetBangAccess{}, false
		}
		return mapGetBangAccess{access: access, receiver: receiver, key: key, safeCall: safeCall}, true
	default:
		return mapGetBangAccess{}, false
	}
}

func flatPostfixHasBangBang(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatType(child) == "!!" {
			return true
		}
	}
	return false
}

func flatFirstNamedChild(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func flatIndexingExpressionParts(file *scanner.File, idx uint32) (receiver, key uint32, ok bool) {
	if file.FlatType(idx) != "indexing_expression" {
		return 0, 0, false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "indexing_suffix" {
			if key != 0 || file.FlatNamedChildCount(child) != 1 {
				return 0, 0, false
			}
			key = file.FlatNamedChild(child, 0)
			continue
		}
		if receiver == 0 {
			receiver = child
		}
	}
	return receiver, key, receiver != 0 && key != 0
}

func flatGetCallExpressionParts(file *scanner.File, idx uint32) (receiver, key uint32, safeCall bool, ok bool) {
	nav, args := flatCallExpressionParts(file, idx)
	if nav == 0 || args == 0 || flatNavigationExpressionLastIdentifier(file, nav) != "get" {
		return 0, 0, false, false
	}
	receiver = flatNavigationExpressionReceiver(file, nav)
	if receiver == 0 {
		return 0, 0, false, false
	}
	key, ok = flatSingleValueArgumentExpression(file, args)
	if !ok {
		return 0, 0, false, false
	}
	return receiver, key, flatNavigationLastSuffixHasSafeAccess(file, nav), true
}

func flatNavigationExpressionReceiver(file *scanner.File, nav uint32) uint32 {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" || file.FlatNamedChildCount(nav) < 2 {
		return 0
	}
	return file.FlatNamedChild(nav, 0)
}

func flatNavigationLastSuffixHasSafeAccess(file *scanner.File, nav uint32) bool {
	var suffix uint32
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "navigation_suffix" {
			suffix = child
		}
	}
	if suffix == 0 {
		return false
	}
	for child := file.FlatFirstChild(suffix); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatType(child) == "?." {
			return true
		}
	}
	return false
}

func flatSingleValueArgumentExpression(file *scanner.File, args uint32) (uint32, bool) {
	var arg uint32
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "value_argument" {
			continue
		}
		if arg != 0 {
			return 0, false
		}
		arg = child
	}
	if arg == 0 {
		return 0, false
	}
	if flatHasValueArgumentLabel(file, arg) {
		expr := flatLastNamedChild(file, arg)
		return expr, expr != 0
	}
	expr := flatValueArgumentExpression(file, arg)
	return expr, expr != 0
}

func mapGetReceiverIsMap(ctx *v2.Context, receiver uint32) bool {
	if ctx == nil || ctx.File == nil || ctx.Resolver == nil || receiver == 0 {
		return false
	}
	receiver = flatUnwrapParenExpr(ctx.File, receiver)
	resolved := ctx.Resolver.ResolveFlatNode(receiver, ctx.File)
	if mapResolvedTypeIsMap(ctx.Resolver, resolved, nil) {
		return true
	}
	if ctx.File.FlatType(receiver) == "simple_identifier" {
		resolved = ctx.Resolver.ResolveByNameFlat(ctx.File.FlatNodeString(receiver, nil), receiver, ctx.File)
		return mapResolvedTypeIsMap(ctx.Resolver, resolved, nil)
	}
	if ctx.File.FlatType(receiver) == "navigation_expression" {
		name := flatNavigationExpressionLastIdentifier(ctx.File, receiver)
		if name != "" {
			resolved = ctx.Resolver.ResolveByNameFlat(name, receiver, ctx.File)
			return mapResolvedTypeIsMap(ctx.Resolver, resolved, nil) ||
				mapNamedDeclarationTypeIsMap(ctx.File, name)
		}
	}
	return false
}

func mapResolvedTypeIsMap(resolver typeinfer.TypeResolver, resolved *typeinfer.ResolvedType, seen map[string]bool) bool {
	if resolved == nil || resolved.Kind == typeinfer.TypeUnknown {
		return false
	}
	if mapTypeNameIsKnown(resolved.Name) || mapTypeNameIsKnown(resolved.FQN) {
		return true
	}
	for _, super := range resolved.Supertypes {
		if mapTypeNameIsKnown(super) {
			return true
		}
	}
	if resolver == nil {
		return false
	}
	if seen == nil {
		seen = make(map[string]bool)
	}
	for _, name := range []string{resolved.FQN, resolved.Name} {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if info := resolver.ClassHierarchy(name); info != nil {
			for _, super := range info.Supertypes {
				if mapTypeNameIsKnown(super) {
					return true
				}
				if mapResolvedTypeIsMap(resolver, &typeinfer.ResolvedType{Name: simpleTypeName(super), FQN: super, Kind: typeinfer.TypeClass}, seen) {
					return true
				}
			}
		}
	}
	return false
}

func mapTypeNameIsKnown(name string) bool {
	switch name {
	case "Map", "MutableMap", "HashMap", "LinkedHashMap", "TreeMap",
		"kotlin.collections.Map", "kotlin.collections.MutableMap",
		"kotlin.collections.HashMap", "kotlin.collections.LinkedHashMap", "kotlin.collections.TreeMap",
		"java.util.Map", "java.util.HashMap", "java.util.LinkedHashMap", "java.util.TreeMap":
		return true
	default:
		return false
	}
}

func mapNamedDeclarationTypeIsMap(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "class_parameter", "parameter", "property_declaration", "variable_declaration":
		default:
			return
		}
		if extractIdentifierFlat(file, candidate) != name {
			return
		}
		typeNode := mapExplicitTypeNode(file, candidate)
		if typeNode == 0 {
			return
		}
		typeName := simpleTypeName(file.FlatNodeText(typeNode))
		found = mapTypeNameIsKnown(typeName)
	})
	return found
}

func mapExplicitTypeNode(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "type_identifier":
			return child
		case "variable_declaration":
			if inner := mapExplicitTypeNode(file, child); inner != 0 {
				return inner
			}
		}
	}
	return 0
}
