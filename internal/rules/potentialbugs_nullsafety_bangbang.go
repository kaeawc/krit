package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/analyzers/nullflow"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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
	// The exemption is intended for `fun requireX(): T = expr!!` — the
	// expression-body form where the function name documents the precondition.
	// Detect this structurally via the AST: the function_body must start with
	// `=` (expression body), not `{` (block body). A scan of `fnText` for `=`
	// also matches default args like `requireX(x: Int = 0)` and block-body
	// `val y = ...` assignments, silently skipping real `!!` bugs.
	return flatFunctionHasExpressionBody(file, fn)
}

// flatFunctionHasExpressionBody reports whether the function_declaration at fn
// uses an expression body (`fun f(): T = expr`) rather than a block body
// (`fun f(): T { ... }`). A function with no body at all (abstract / interface)
// returns false.
func flatFunctionHasExpressionBody(file *scanner.File, fn uint32) bool {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return false
	}
	body, ok := file.FlatFindChild(fn, "function_body")
	if !ok || body == 0 {
		return false
	}
	// The function_body's first non-named child is either `=` (expression
	// body) or `{` (block body).
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "=":
			return true
		case "{":
			return false
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// UnsafeCallOnNullableTypeRule detects !! operator usage.
// ---------------------------------------------------------------------------
type UnsafeCallOnNullableTypeRule struct {
	FlatDispatchBase
	BaseRule
	CustomPreviewWildcard bool
	CustomPreviewPrefixes []string
}

func (r *UnsafeCallOnNullableTypeRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File

	// Gate on the !! operator child — avoids false positives from string
	// literals like "use !! to force" that contain the text but aren't the operator.
	if !flatPostfixHasBangBang(file, idx) {
		return
	}

	// Skip test sources — tests use `!!` freely on setup fixtures;
	// a NullPointerException there is just a failed test, not a runtime
	// bug affecting production.
	if isAndroidTestSupportArtifactSource(file.Path) {
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
	if isInsidePreviewOrSampleFunctionFlatWithConfig(file, idx, customPreviewConfigFromFields(r.CustomPreviewWildcard, r.CustomPreviewPrefixes)) {
		return
	}

	// Derive receiver from the first named child of the postfix_expression.
	receiverIdx := flatFirstNamedChild(file, idx)
	receiverText := file.FlatNodeText(receiverIdx)
	if isNamedKClassSimpleNameLiteralReceiver(receiverText) {
		return
	}

	// Skip proto-processor files: any Kotlin file importing Wire /
	// protobuf packages is treated as a "proto processor". Generated fields
	// are often nullable by type but required at runtime, and `!!` is the
	// idiomatic unwrap.
	// Skip only pure dotted field-chain receivers (2+ segments, no
	// parentheses), preserving checks on single-identifier locals and
	// method-call chains.
	normalized := strings.ReplaceAll(receiverText, "!!", "")
	normalized = strings.TrimPrefix(normalized, "this.")
	if fileImportsProto(file) && isDottedFieldChain(normalized) {
		return
	}
	// Skip idiomatic Android patterns where !! is the canonical way to
	// consume platform-typed APIs:
	//   - Bundle.getX(...)!!, requireArguments().getX()!!
	//   - Parcel.readX()!! in Parcelable constructors
	//   - Intent.getX(...)!! / Intent.extras!!
	//
	// De-dup with MapGetWithNotNullAssertionOperator: map[key]!! / foo.get(k)!!
	// is the sibling rule's concern.
	if strings.HasSuffix(receiverText, "]") {
		return
	}
	if isIdiomaticNullAssertionReceiver(receiverText, file, receiverIdx) {
		return
	}
	// Normalize the receiver so that `dialog!!.window` and `this.window`
	// match the plain `window` in the allowlist.
	if normalized != receiverText && isIdiomaticNullAssertionReceiver(normalized, file, receiverIdx) {
		return
	}

	// Flow-sensitive guard: if the receiver expression (or its leading
	// safe-call chain) is proven non-null by an enclosing `if (x != null)`
	// or `if (x?.y != null)` branch, the `!!` is a smart-cast workaround
	// rather than an unsafe assertion.
	if nullflow.IsGuardedNonNull(file, idx, receiverIdx) {
		return
	}
	// Early-return guard: `if (x == null) return` earlier in the same block
	// proves non-null for any subsequent `x!!` in the same statements scope.
	if nullflow.IsEarlyReturnGuarded(file, idx, receiverIdx, bodyAlwaysExitsFlat) {
		return
	}
	// Same-expression short-circuit guard: `x != null && x!!.y` proves the
	// right-hand `!!` safe without relying on Kotlin smart casts.
	if nullflow.IsShortCircuitGuardedNonNull(file, idx, receiverIdx) {
		return
	}
	// Same-block assignment guard: `if (x == null) x = create(); x!!.y` and
	// `x = create(); x!!.y` prove simple mutable fields/locals non-null in
	// the current statement sequence.
	if nullflow.IsSameBlockAssignedNonNullBeforeUse(file, idx, receiverIdx, nil) {
		return
	}
	// Post-filter smart cast: `.filter { it.x != null }.map { it.x!! }` —
	// if an enclosing lambda is inside a `.map` / `.forEach` / `.let` call
	// whose chain has a preceding `.filter { it.<field> != null }`, the
	// subsequent `!!` on that field is safe.
	if nullflow.IsPostFilterSmartCast(file, idx, receiverText) {
		return
	}
	// `fun requireXxx(): T = field!!` — the function name explicitly
	// documents the precondition ("the caller must have verified this").
	// The `!!` is the idiomatic implementation.
	if isRequireFunctionBangBodyFlat(file, idx) {
		return
	}

	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Not-null assertion operator (!!) used. Consider using safe calls (?.) instead."))
}

// fileImportsProto returns true if the Kotlin file imports Wire or protobuf
// packages. Proto fields are structurally nullable but conventionally required;
// `!!` is idiomatic.
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
		strings.Contains(header, ".protos.") ||
		strings.Contains(header, ".databaseprotos.") ||
		strings.Contains(header, ".storageservice.protos.") ||
		strings.Contains(header, ".api.crypto.protos.") ||
		strings.Contains(header, ".internal.serialize.protos.")
}

func isNamedKClassSimpleNameLiteralReceiver(receiver string) bool {
	const suffix = "::class.simpleName"
	if !strings.HasSuffix(receiver, suffix) {
		return false
	}
	qualifier := strings.TrimSuffix(receiver, suffix)
	if qualifier == "" {
		return false
	}
	parts := strings.Split(qualifier, ".")
	last := parts[len(parts)-1]
	if last == "" || last[0] < 'A' || last[0] > 'Z' {
		return false
	}
	for _, part := range parts {
		if !isKotlinIdentifierPartList(part) {
			return false
		}
	}
	return true
}

func isKotlinIdentifierPartList(text string) bool {
	if text == "" {
		return false
	}
	for i, r := range text {
		if i == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
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

func flatIsNullLiteral(file *scanner.File, idx uint32) bool {
	idx = flatUnwrapParenExpr(file, idx)
	return idx != 0 && (file.FlatType(idx) == "null" || file.FlatNodeTextEquals(idx, "null"))
}

// isIdiomaticNullAssertionReceiver returns true if the receiver text matches
// a known Android API where !! is the standard (and often only) consumption
// pattern.
func isIdiomaticNullAssertionReceiver(receiver string, file *scanner.File, receiverIdx uint32) bool {
	if isViewBindingReceiver(receiver) {
		return true
	}
	if isFragmentLifecycleReceiver(receiver) {
		return true
	}
	if isAndroidResourceReceiver(receiver) {
		return true
	}
	if isWireProtobufReceiver(receiver) {
		return true
	}
	if isBundleOrParcelReceiver(receiver) {
		return true
	}
	if isKspReceiver(receiver, file) {
		return true
	}
	if fileImportsCompilerApis(file) && isCompilerLookupReceiver(receiver) {
		return true
	}
	if fileImportsProto(file) && isUnqualifiedWireProtoField(receiver) &&
		isInsideWireProtoReceiverExtensionFlat(file, receiverIdx) {
		return true
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
