package rules

import (
	"regexp"
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

func isGuardedNonNullFlat(file *scanner.File, idx uint32, receiverText string) bool {
	base := strings.TrimSuffix(receiverText, ".")
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
		condText := file.FlatNodeText(cond)
		if thenBody == current && (guardMatches(condText, base) || guardMatches(condText, receiverText)) {
			return true
		}
		if elseBody == current && (nonNullElseGuardMatches(condText, base) || nonNullElseGuardMatches(condText, receiverText)) {
			return true
		}
	}
	return false
}

func isEarlyReturnGuardedFlat(file *scanner.File, idx uint32, receiverText string) bool {
	base := strings.TrimSuffix(receiverText, ".")
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
		condText := file.FlatNodeText(cond)
		if earlyReturnGuardMatches(condText, base) || earlyReturnGuardMatches(condText, receiverText) {
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
		navExpr := file.FlatFindChild(chain, "navigation_expression")
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
				recvCallee := file.FlatFindChild(recv, "navigation_expression")
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

func isMapContainsKeyGuardedFlat(file *scanner.File, idx uint32, receiver, key string) bool {
	key = strings.TrimSpace(key)
	receiver = strings.TrimSpace(receiver)
	containsCall := receiver + ".containsKey(" + key + ")"
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
		if strings.Contains(file.FlatNodeText(parent), containsCall) {
			return true
		}
	}
	return false
}

func isInsideContainsKeyFilterChainFlat(file *scanner.File, idx uint32, receiver string) bool {
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return false
	}
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
	navExpr := file.FlatFindChild(transformCall, "navigation_expression")
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
	needle := receiver + ".containsKey("
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
			recvCallee := file.FlatFindChild(recv, "navigation_expression")
			if recvCallee != 0 {
				last := flatNavigationExpressionLastIdentifier(file, recvCallee)
				if last == "filter" || last == "filterKeys" || last == "filterValues" {
					if strings.Contains(file.FlatNodeText(recv), needle) {
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
	if isGuardedNonNullFlat(file, idx, receiverText) {
		return
	}
	// Early-return guard: `if (x == null) return` earlier in the same block
	// proves non-null for any subsequent `x!!` in the same statements scope.
	if isEarlyReturnGuardedFlat(file, idx, receiverText) {
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

// earlyReturnGuardMatches checks for `chain == null` in cond text.
func earlyReturnGuardMatches(condText, base string) bool {
	if base == "" {
		return false
	}
	norm := strings.Join(strings.Fields(condText), " ")
	candidates := []string{base}
	trimmed := base
	for {
		idx := strings.LastIndex(trimmed, ".")
		if idx < 0 {
			break
		}
		trimmed = trimmed[:idx]
		if trimmed == "" {
			break
		}
		candidates = append(candidates, trimmed)
	}
	for _, cand := range candidates {
		if strings.Contains(norm, cand+" == null") {
			return true
		}
		needle := cand + "?."
		if i := strings.Index(norm, needle); i >= 0 {
			rest := norm[i:]
			if strings.Contains(rest, "== null") {
				return true
			}
		}
	}
	return false
}

// guardMatches returns true if condText contains a `chain != null` test
// where chain is either the base or a prefix of it. Handles `x != null`,
// `x?.y != null`, and conjunctions like `a != null && b != null`.
func guardMatches(condText, base string) bool {
	if base == "" {
		return false
	}
	// Normalize whitespace
	norm := strings.Join(strings.Fields(condText), " ")
	// Accept exact match and any prefix up to a `.`
	candidates := []string{base}
	// Trim trailing ".field" components to widen the match.
	trimmed := base
	for {
		idx := strings.LastIndex(trimmed, ".")
		if idx < 0 {
			break
		}
		trimmed = trimmed[:idx]
		if trimmed == "" {
			break
		}
		candidates = append(candidates, trimmed)
	}
	for _, cand := range candidates {
		// `cand != null`
		if strings.Contains(norm, cand+" != null") {
			return true
		}
		// `cand?.x != null` — any suffix
		needle := cand + "?."
		if i := strings.Index(norm, needle); i >= 0 {
			// Ensure `!= null` appears somewhere after the needle.
			rest := norm[i:]
			if strings.Contains(rest, "!= null") {
				return true
			}
		}
	}
	return false
}

func nonNullElseGuardMatches(condText, base string) bool {
	if base == "" {
		return false
	}
	norm := strings.Join(strings.Fields(condText), " ")
	candidates := []string{base}
	trimmed := base
	for {
		idx := strings.LastIndex(trimmed, ".")
		if idx < 0 {
			break
		}
		trimmed = trimmed[:idx]
		if trimmed == "" {
			break
		}
		candidates = append(candidates, trimmed)
	}
	for _, cand := range candidates {
		if strings.Contains(norm, "TextUtils.isEmpty("+cand+")") {
			return true
		}
		if strings.Contains(norm, cand+".isNullOrEmpty()") ||
			strings.Contains(norm, cand+"?.isEmpty() == true") {
			return true
		}
		if strings.Contains(norm, cand+" == null || "+cand+".isEmpty()") ||
			strings.Contains(norm, cand+"== null || "+cand+".isEmpty()") ||
			strings.Contains(norm, cand+" == null|| "+cand+".isEmpty()") {
			return true
		}
	}
	return false
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
// structural check; resolver confirms the receiver is Map but falls back to
// name-based assumption. Classified per roadmap/17.
func (r *MapGetWithNotNullAssertionRule) Confidence() float64 { return 0.75 }

var mapBangRe = regexp.MustCompile(`\[[^\]]+\]\s*!!|\.get\([^)]+\)\s*!!`)

var mapBracketBangRe = regexp.MustCompile(`(\w+(?:\.\w+)*)\[([^\]]+)\]\s*!!`)
var mapGetBangRe = regexp.MustCompile(`(\w+(?:\.\w+)*)\.get\(([^)]+)\)\s*!!`)

func (r *MapGetWithNotNullAssertionRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	// Skip test files — fail-fast `map[key]!!` is idiomatic in tests.
	if isTestFile(file.Path) {
		return
	}
	text := file.FlatNodeText(idx)
	if !mapBangRe.MatchString(text) {
		return
	}
	// Skip when the access is guarded by `map.containsKey(key)` in an
	// enclosing if or earlier statement, or by a preceding filter.
	if m := mapBracketBangRe.FindStringSubmatch(text); m != nil {
		receiver, key := m[1], m[2]
		if isMapContainsKeyGuardedFlat(file, idx, receiver, key) {
			return
		}
		if experiment.Enabled("map-get-bang-skip-contains-key-filter") &&
			isInsideContainsKeyFilterChainFlat(file, idx, receiver) {
			return
		}
	} else if m := mapGetBangRe.FindStringSubmatch(text); m != nil {
		receiver, key := m[1], m[2]
		if isMapContainsKeyGuardedFlat(file, idx, receiver, key) {
			return
		}
		if experiment.Enabled("map-get-bang-skip-contains-key-filter") &&
			isInsideContainsKeyFilterChainFlat(file, idx, receiver) {
			return
		}
	}
	// If resolver is available, verify the receiver is actually a Map type
	if ctx.Resolver != nil {
		var receiverName string
		if m := mapBracketBangRe.FindStringSubmatch(text); m != nil {
			receiverName = m[1]
		} else if m := mapGetBangRe.FindStringSubmatch(text); m != nil {
			receiverName = m[1]
		}
		if receiverName != "" {
			simpleName := receiverName
			if dotIdx := strings.LastIndex(simpleName, "."); dotIdx >= 0 {
				simpleName = simpleName[dotIdx+1:]
			}
			resolved := ctx.Resolver.ResolveByNameFlat(simpleName, idx, file)
			if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
				isMap := resolved.Name == "Map" || resolved.Name == "MutableMap" ||
					resolved.Name == "HashMap" || resolved.Name == "LinkedHashMap" ||
					strings.HasSuffix(resolved.FQN, ".Map") ||
					strings.HasSuffix(resolved.FQN, ".MutableMap") ||
					strings.HasSuffix(resolved.FQN, ".HashMap") ||
					strings.HasSuffix(resolved.FQN, ".LinkedHashMap")
				if !isMap {
					return // receiver is not a Map, skip
				}
			}
		}
	}

	startByte := int(file.FlatStartByte(idx))
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Map access with not-null assertion operator (!!). Use getValue() or getOrDefault() instead.")
	// Try bracket syntax: map[key]!!
	if loc := mapBracketBangRe.FindStringSubmatchIndex(text); loc != nil {
		receiver := text[loc[2]:loc[3]]
		key := text[loc[4]:loc[5]]
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte + loc[0],
			EndByte:     startByte + loc[1],
			Replacement: receiver + ".getValue(" + key + ")",
		}
	} else if loc := mapGetBangRe.FindStringSubmatchIndex(text); loc != nil {
		receiver := text[loc[2]:loc[3]]
		key := text[loc[4]:loc[5]]
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte + loc[0],
			EndByte:     startByte + loc[1],
			Replacement: receiver + ".getValue(" + key + ")",
		}
	}
	ctx.Emit(f)
}

