package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/analyzers/nullflow"
	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
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

func isViewBindingReceiver(receiver string) bool {
	if strings.HasPrefix(receiver, "_") && !strings.ContainsAny(receiver, "().") {
		return true
	}
	return receiver == "binding" || receiver == "viewBinding" || receiver == "_binding"
}

func isFragmentLifecycleReceiver(receiver string) bool {
	switch receiver {
	case "instance", "INSTANCE",
		"context", "activity", "arguments", "window",
		"dialog", "parentFragment", "serializedData":
		return true
	}
	if strings.HasSuffix(receiver, ".window") {
		if strings.Contains(strings.ToLower(receiver), "dialog") {
			return true
		}
	}
	return false
}

func isAndroidResourceReceiver(receiver string) bool {
	androidGetters := []string{
		"getDrawable(", "getColorStateList(", "getBundleExtra(",
		"getParcelableExtra(", "getParcelableExtraCompat(",
		"getParcelableArrayExtraCompat(", "getParcelableArrayListExtraCompat(",
		"getStringExtra(", "getIntExtra(", "getSystemService",
		"modelClass.cast(", ".cast(",
	}
	for _, g := range androidGetters {
		if strings.Contains(receiver, g) {
			return true
		}
	}
	if strings.HasSuffix(receiver, ".extras") ||
		strings.HasSuffix(receiver, "intent.extras") {
		return true
	}
	return false
}

func isWireProtobufReceiver(receiver string) bool {
	wireDecoders := []string{
		".ADAPTER.decode(", "cursor.requireBlob(", "requireBlob(", "requireNonNullBlob(",
	}
	for _, d := range wireDecoders {
		if strings.Contains(receiver, d) {
			return true
		}
	}
	wireProtoFields := []string{
		".timestamp", ".serverTimestamp", ".sourceDevice", ".sourceServiceId",
		".destination", ".destinationServiceId", ".groupId", ".masterKey",
		".content", ".dataMessage", ".syncMessage", ".sent", ".message",
		".type", ".serverGuid", ".ciphertextHash",
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
		".id", ".data_", ".targetSentTimestamp", ".latestRevisionId",
		".direction", ".conversationId", ".event", ".peekInfo",
		".ringUpdate", ".acknowledgedReceipt", ".observedReceipt",
		".flags", ".delete", ".edit", ".reaction", ".thread", ".groupV2",
		".sticker", ".preview", ".attachments", ".quote",
	}
	for _, field := range wireProtoFields {
		if strings.HasSuffix(receiver, field) {
			return true
		}
	}
	return false
}

func isBundleOrParcelReceiver(receiver string) bool {
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
	parcelMethods := []string{
		".readString(", ".readStringArray(", ".readStringList(",
		".readInt(", ".readLong(", ".readFloat(", ".readDouble(",
		".readByte(", ".readByteArray(", ".readBundle(",
		".readParcelable(", ".readParcelableArray(", ".readParcelableList(",
		".readParcelableCompat(", ".readParcelableArrayCompat(",
		".readSerializable(",
		".readSerializableCompat(",
	}
	for _, m := range parcelMethods {
		if strings.Contains(receiver, m) {
			return true
		}
	}
	return false
}

func isKspReceiver(receiver string, file *scanner.File) bool {
	if fileImportsKsp(file) && strings.HasSuffix(receiver, ".qualifiedName") {
		return true
	}
	if fileImportsKsp(file) && receiver == "creatorOrConstructor" {
		return true
	}
	return false
}

func isUnqualifiedWireProtoField(receiver string) bool {
	if receiver == "" || strings.ContainsAny(receiver, ".()[]") {
		return false
	}
	switch receiver {
	case "accessControl", "address", "amount", "attachments", "badge", "body",
		"callEvent", "callLinkUpdate", "callLogEvent", "callMessage", "cdn",
		"ciphertextHash", "configuration", "contacts", "content", "dataMessage",
		"data_", "delete", "deleteForMe", "destination", "destinationServiceId",
		"direction", "edit", "editMessage", "event", "failureReason", "fetchLatest",
		"flags", "giftBadge", "groupId", "groupV2", "groupChange", "hangup",
		"id", "inAppPayment", "keys", "latestRevisionId", "length", "masterKey",
		"message", "messageRequestResponse", "metadata", "needsReceipt", "opaque",
		"outgoingPayment", "paymentMethod", "paymentNotification", "preview",
		"query", "reaction", "receiptCredentialPresentation", "recipient",
		"redemption", "remoteDigest", "ringUpdate", "senderDevice", "sent",
		"serverGuid", "serverReceivedTimestamp", "serverTimestamp", "sourceDevice",
		"sourceServiceId", "start", "sticker", "storageService", "storyMessage",
		"syncMessage", "targetSentTimestamp", "thread", "timestamp", "type",
		"uploadSpec", "uri", "value", "verified", "viewOnceOpen":
		return true
	default:
		return false
	}
}

func isInsideWireProtoReceiverExtensionFlat(file *scanner.File, receiverIdx uint32) bool {
	if file == nil || receiverIdx == 0 {
		return false
	}
	if fn, ok := flatEnclosingFunction(file, receiverIdx); ok {
		typeName, _ := flatFunctionReceiverTypeInfo(file, fn)
		return isWireProtoReceiverTypeName(typeName)
	}
	prop, ok := flatEnclosingAncestor(file, receiverIdx, "property_declaration")
	if !ok {
		return false
	}
	typeName := flatPropertyReceiverTypeName(file, prop)
	return isWireProtoReceiverTypeName(typeName)
}

func isWireProtoReceiverTypeName(typeName string) bool {
	switch typeName {
	case "DataMessage", "GroupContextV2", "SyncMessage", "Content", "Envelope",
		"PendingOneTimeDonation", "FiatValue", "DecimalValue", "DecryptedGroup":
		return true
	default:
		return false
	}
}

func flatPropertyReceiverTypeName(file *scanner.File, prop uint32) string {
	if file == nil || prop == 0 || file.FlatType(prop) != "property_declaration" {
		return ""
	}
	var name string
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "nullable_type", "user_type", "type_identifier":
			if name == "" {
				name = flatLastIdentifierInNode(file, child)
			}
		case ".":
			return name
		case "property_delegate", "property_declaration_body", "=", "getter":
			return ""
		}
	}
	return ""
}

// isCompilerLookupReceiver reports compiler-plugin symbol lookups where `!!`
// is the conventional "this lookup must exist" assertion. This keeps the rule
// focused on application code while avoiding noisy compiler/IR codegen paths.
func isCompilerLookupReceiver(receiver string) bool {
	if strings.Contains(receiver, "referenceClass(") ||
		strings.Contains(receiver, "primaryConstructor") ||
		strings.Contains(receiver, "classFqName") ||
		strings.Contains(receiver, "getter") ||
		strings.Contains(receiver, "resolveKSClassDeclaration(") ||
		receiver == "classId" || strings.HasSuffix(receiver, ".classId") ||
		receiver == "creatorOrConstructor" || strings.HasSuffix(receiver, ".creatorOrConstructor") ||
		strings.Contains(receiver, "companionObject()") {
		return true
	}
	return isCompilerPluginInvariantReceiver(receiver)
}

func isCompilerPluginInvariantReceiver(receiver string) bool {
	if receiver == "" {
		return false
	}
	for _, part := range []string{
		"constArgumentOfTypeAt<",
		"CompilerMessageLocationWithRange.create(",
		"findRequiredConstructor(",
		"getAnnotation(",
		"getRequiredAnnotationString()",
		"scopeOrNull()",
		"typeArguments.single()",
		"RequiredMapKeyAnnotation()",
		"findRequiredMapValueType()",
		"requiredClassId",
	} {
		if strings.Contains(receiver, part) {
			return true
		}
	}
	for _, suffix := range []string{
		".backingField",
		".callee",
		".dispatchReceiverParameter",
		".extensionReceiverParameterCompat",
		".factoryTargetMetadata",
		".generatedGraphMetadata",
		".graphParam",
		".ir",
		".irElement",
		".irProperty",
		".dependencyGraph",
		".packageFqName",
		".receiverParameterSymbol",
		".scope",
		".switchingId",
		".targetConstructor",
		".thisReceiver",
		".typeOrNull",
	} {
		if strings.HasSuffix(receiver, suffix) {
			return true
		}
	}
	switch receiver {
	case "backingField", "containerParameter", "diagnostic", "dispatchReceiverParameter",
		"functionReceiver", "generatedGraphMetadata", "innerRequiredClassId", "injectorMetadata",
		"mapKey", "parentClassOrNull", "requiredClassId", "requiredType", "sizeSymbol",
		"targetConstructor", "thisReceiver":
		return true
	default:
		return false
	}
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
func (r *MapGetWithNotNullAssertionRule) Confidence() float64 { return api.ConfidenceMedium }

type mapGetBangAccess struct {
	access   uint32
	receiver uint32
	key      uint32
	call     bool
	safeCall bool
}

func (r *MapGetWithNotNullAssertionRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	// Skip test files — fail-fast `map[key]!!` is idiomatic in tests.
	if scanner.IsTestFile(file.Path) {
		return
	}
	access, ok := flatMapGetBangAccess(file, idx)
	if !ok {
		return
	}
	receiverType, ok := mapGetReceiverMapType(ctx, access.receiver)
	if !ok || !mapGetAccessMatchesMapGet(ctx, access, receiverType) {
		return
	}
	// Skip when the access is guarded by `map.containsKey(key)` in an
	// enclosing if or earlier statement, or by a preceding filter.
	if nullflow.IsMapContainsKeyGuarded(file, idx, access.receiver, access.key) ||
		nullflow.IsEarlyReturnMapContainsKeyGuarded(file, idx, access.receiver, access.key, bodyAlwaysExitsFlat) {
		return
	}
	if experiment.Enabled("map-get-bang-skip-contains-key-filter") &&
		nullflow.IsInsideContainsKeyFilterChain(file, idx, access.receiver) {
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
		return mapGetBangAccess{access: access, receiver: receiver, key: key, call: true, safeCall: safeCall}, true
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

func mapGetAccessMatchesMapGet(ctx *api.Context, access mapGetBangAccess, receiverType *typeinfer.ResolvedType) bool {
	if access.call {
		target, ok := semantics.ResolveCallTarget(ctx, access.access)
		if !ok || target.CalleeName != "get" {
			return false
		}
		if target.Resolved && !mapGetResolvedTargetIsMapGet(target.QualifiedName) {
			return false
		}
	}
	return mapGetKeyCompatible(ctx, receiverType, access.key)
}

func mapGetResolvedTargetIsMapGet(target string) bool {
	return target == "kotlin.collections.Map.get" ||
		target == "kotlin.collections.MutableMap.get" ||
		target == "java.util.Map.get" ||
		strings.HasSuffix(target, ".Map.get") ||
		strings.HasSuffix(target, ".MutableMap.get")
}

func mapGetReceiverMapType(ctx *api.Context, receiver uint32) (*typeinfer.ResolvedType, bool) {
	if ctx.File == nil || ctx.Resolver == nil || receiver == 0 {
		return nil, false
	}
	receiver = flatUnwrapParenExpr(ctx.File, receiver)
	resolved := mapResolveExpressionType(ctx, receiver, nil)
	if mapResolvedTypeIsMap(ctx.Resolver, resolved, nil) {
		return resolved, true
	}
	return nil, false
}

func mapResolveExpressionType(ctx *api.Context, expr uint32, seen map[uint32]bool) *typeinfer.ResolvedType {
	if ctx.File == nil || ctx.Resolver == nil || expr == 0 {
		return nil
	}
	expr = flatUnwrapParenExpr(ctx.File, expr)
	if seen == nil {
		seen = make(map[uint32]bool)
	}
	if seen[expr] {
		return nil
	}
	seen[expr] = true

	switch ctx.File.FlatType(expr) {
	case "simple_identifier":
		if resolved := ctx.Resolver.ResolveFlatNode(expr, ctx.File); resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
			return resolved
		}
		return ctx.Resolver.ResolveByNameFlat(ctx.File.FlatNodeString(expr, nil), expr, ctx.File)
	case "navigation_expression":
		name := flatNavigationExpressionLastIdentifier(ctx.File, expr)
		if name != "" {
			if resolved := ctx.Resolver.ResolveByNameFlat(name, expr, ctx.File); resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
				return resolved
			}
			base := flatNavigationExpressionReceiver(ctx.File, expr)
			if baseType := mapResolveExpressionType(ctx, base, seen); baseType != nil {
				return mapMemberType(ctx, baseType, name)
			}
		}
	}
	resolved := ctx.Resolver.ResolveFlatNode(expr, ctx.File)
	if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
		return resolved
	}
	return nil
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

func mapGetKeyCompatible(ctx *api.Context, receiverType *typeinfer.ResolvedType, key uint32) bool {
	if receiverType == nil || len(receiverType.TypeArgs) == 0 {
		return true
	}
	want := receiverType.TypeArgs[0]
	if mapTypeArgIsWildcard(want) {
		return true
	}
	got := mapResolveExpressionType(ctx, key, nil)
	if got == nil || got.Kind == typeinfer.TypeUnknown {
		return false
	}
	return mapTypesCompatible(want, *got)
}

func mapTypeArgIsWildcard(t typeinfer.ResolvedType) bool {
	name := simpleTypeName(t.Name)
	return name == "" || name == "*" || name == "Any"
}

func mapTypesCompatible(want, got typeinfer.ResolvedType) bool {
	if mapTypeArgIsWildcard(want) {
		return true
	}
	wantName := simpleTypeName(firstNonEmpty(want.FQN, want.Name))
	gotName := simpleTypeName(firstNonEmpty(got.FQN, got.Name))
	if wantName == "" || gotName == "" {
		return false
	}
	if wantName == gotName {
		return true
	}
	return typeinfer.MapJavaToKotlin(firstNonEmpty(got.FQN, got.Name)) == firstNonEmpty(want.FQN, want.Name)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func mapMemberType(ctx *api.Context, owner *typeinfer.ResolvedType, member string) *typeinfer.ResolvedType {
	if ctx.File == nil || ctx.Resolver == nil || owner == nil || member == "" {
		return nil
	}
	for _, ownerName := range []string{owner.FQN, owner.Name} {
		if ownerName == "" {
			continue
		}
		if info := ctx.Resolver.ClassHierarchy(ownerName); info != nil {
			for _, m := range info.Members {
				if m.Name == member && m.Type != nil && m.Type.Kind != typeinfer.TypeUnknown {
					return m.Type
				}
			}
		}
		if typ := mapMemberTypeFromSameFileDeclaration(ctx, simpleTypeName(ownerName), member); typ != nil {
			return typ
		}
	}
	return nil
}

func mapMemberTypeFromSameFileDeclaration(ctx *api.Context, ownerName, member string) *typeinfer.ResolvedType {
	if ctx.File == nil || ctx.Resolver == nil || ownerName == "" || member == "" {
		return nil
	}
	file := ctx.File
	var classDecl uint32
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if classDecl != 0 || file.FlatType(candidate) != "class_declaration" {
			return
		}
		if extractIdentifierFlat(file, candidate) == ownerName {
			classDecl = candidate
		}
	})
	if classDecl == 0 {
		return nil
	}
	var out *typeinfer.ResolvedType
	file.FlatWalkAllNodes(classDecl, func(candidate uint32) {
		if out != nil || candidate == classDecl {
			return
		}
		if !mapClassMemberCandidate(file, classDecl, candidate) || extractIdentifierFlat(file, candidate) != member {
			return
		}
		if typeNode := mapExplicitTypeNode(file, candidate); typeNode != 0 {
			out = ctx.Resolver.ResolveFlatNode(typeNode, file)
			if out == nil || out.Kind == typeinfer.TypeUnknown {
				out = &typeinfer.ResolvedType{Name: simpleTypeName(file.FlatNodeText(typeNode)), Kind: typeinfer.TypeClass}
			}
		}
	})
	return out
}

func mapClassMemberCandidate(file *scanner.File, classDecl, candidate uint32) bool {
	switch file.FlatType(candidate) {
	case "class_parameter", "property_declaration":
	default:
		return false
	}
	for p, ok := file.FlatParent(candidate); ok; p, ok = file.FlatParent(p) {
		if p == classDecl {
			return true
		}
		switch file.FlatType(p) {
		case "function_declaration", "lambda_literal", "object_declaration":
			return false
		case "class_declaration":
			return p == classDecl
		}
	}
	return false
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
