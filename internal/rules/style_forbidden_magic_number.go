package rules

import (
	"strconv"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/experiment"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// MagicNumberRule detects literal numbers in code.
type MagicNumberRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreAnnotated                          []string
	CustomPreviewWildcard                    bool
	CustomPreviewPrefixes                    []string
	IgnorePropertyDeclaration                bool     // if true, skip numbers in val/var declarations
	IgnoreComposeUnits                       bool     // if true, skip numbers followed by .dp, .sp, .px, .em
	IgnoreColorLiterals                      bool     // if true, skip hex color literals (0xAARRGGBB)
	IgnoreNumbers                            []string // numbers to ignore (default: -1, 0, 1, 2)
	IgnoreHashCodeFunction                   bool     // if true, skip numbers in hashCode()
	IgnoreConstantDeclaration                bool     // if true, skip numbers in const val
	IgnoreAnnotation                         bool     // if true, skip numbers inside annotations
	IgnoreNamedArgument                      bool     // if true, skip numbers in named arguments
	IgnoreEnums                              bool     // if true, skip numbers in enum entries
	IgnoreRanges                             bool     // if true, skip numbers in ranges (1..10)
	IgnoreCompanionObjectPropertyDeclaration bool
	IgnoreExtensionFunctions                 bool
	IgnoreLocalVariableDeclaration           bool
	// IgnoreIdioms exempts the Android, Compose and DSL idioms krit adds to detekt's rule; see kritIdiom.
	IgnoreIdioms bool

	ignoredNumbersMu  sync.Mutex
	ignoredNumbersKey string
	ignoredNumbersMap map[float64]bool
}

// ignoredNumberSet returns the configured ignoreNumbers as values, so `1f`, `1.0` and `1` are the same number.
func (r *MagicNumberRule) ignoredNumberSet() map[float64]bool {
	nums := r.IgnoreNumbers
	if len(nums) == 0 {
		nums = []string{"-1", "0", "1", "2"}
	}
	key := strings.Join(nums, "\x00")
	r.ignoredNumbersMu.Lock()
	defer r.ignoredNumbersMu.Unlock()
	if r.ignoredNumbersMap != nil && r.ignoredNumbersKey == key {
		return r.ignoredNumbersMap
	}
	m := make(map[float64]bool, len(nums))
	for _, n := range nums {
		if v, ok := parseMagicNumberValue(n); ok {
			m[v] = true
		}
	}
	r.ignoredNumbersKey, r.ignoredNumbersMap = key, m
	return m
}

// parseMagicNumberValue parses a Kotlin numeric literal, with an optional leading minus, as detekt does.
func parseMagicNumberValue(text string) (float64, bool) {
	t := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), "_", ""))
	negative := strings.HasPrefix(t, "-")
	t = strings.TrimPrefix(t, "-")
	var v float64
	switch {
	case strings.HasPrefix(t, "0x"):
		digits := strings.TrimRight(strings.TrimPrefix(t, "0x"), "lu")
		n, err := strconv.ParseUint(digits, 16, 64)
		if err != nil {
			return 0, false
		}
		v = float64(n)
	case strings.HasPrefix(t, "0b"):
		digits := strings.TrimRight(strings.TrimPrefix(t, "0b"), "lu")
		n, err := strconv.ParseUint(digits, 2, 64)
		if err != nil {
			return 0, false
		}
		v = float64(n)
	default:
		t = strings.TrimRight(t, "lfdu")
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, false
		}
		v = f
	}
	if negative {
		v = -v
	}
	return v, true
}

// isIgnoredNumber reports whether the literal's value, with a leading unary minus, is in ignoreNumbers.
func (r *MagicNumberRule) isIgnoredNumber(file *scanner.File, idx uint32, text string) bool {
	v, ok := parseMagicNumberValue(text)
	if !ok {
		return false
	}
	if magicNumberHasUnaryMinus(file, idx) {
		v = -v
	}
	return r.ignoredNumberSet()[v]
}

func magicNumberHasUnaryMinus(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "prefix_expression" {
		return false
	}
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if file.FlatNodeTextEquals(child, "-") {
			return true
		}
		if file.FlatNodeTextEquals(child, "+") || file.FlatNodeTextEquals(child, "!") {
			return false
		}
	}
	return false
}

func magicNumberIsBitOperationLiteral(file *scanner.File, idx uint32) bool {
	if strings.HasPrefix(file.FlatNodeText(idx), "0b") || strings.HasPrefix(file.FlatNodeText(idx), "0B") {
		return true
	}
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		t := file.FlatType(cur)
		if t == "infix_expression" {
			text := file.FlatNodeText(cur)
			return strings.Contains(text, " shl ") ||
				strings.Contains(text, " shr ") ||
				strings.Contains(text, " ushr ") ||
				strings.Contains(text, " and ") ||
				strings.Contains(text, " or ") ||
				strings.Contains(text, " xor ")
		}
		if t == "function_body" || t == "statements" || t == "property_declaration" ||
			t == "function_declaration" || t == "source_file" {
			return false
		}
	}
	return false
}

func magicNumberIsBinaryProtocolLiteral(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "call_expression":
			switch flatCallNameAny(file, cur) {
			case "rangeEquals", "request":
				return true
			}
			return false
		case "function_body", "statements", "property_declaration", "function_declaration", "source_file":
			return false
		}
	}
	return false
}

func magicNumberIsAndroidAPILevelLiteral(file *scanner.File, idx uint32) bool {
	row := file.FlatRow(idx)
	if row >= 0 && row < len(file.Lines) {
		line := file.Lines[row]
		if strings.Contains(line, "RequiresApi(") ||
			strings.Contains(line, "TargetApi(") ||
			strings.Contains(line, "ChecksSdkIntAtLeast(") {
			return true
		}
	}
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "annotation":
			text := file.FlatNodeText(cur)
			return strings.Contains(text, "RequiresApi") ||
				strings.Contains(text, "TargetApi") ||
				strings.Contains(text, "ChecksSdkIntAtLeast")
		case "call_expression":
			switch flatCallNameAny(file, cur) {
			case "RequiresApi", "TargetApi", "ChecksSdkIntAtLeast", "minSdkLessThan", "checkApi":
				return true
			}
		case "function_body", "statements", "property_declaration", "function_declaration", "source_file":
			return false
		}
	}
	return false
}

func magicNumberIsHTTPStatusLiteral(file *scanner.File, idx uint32) bool {
	text := strings.TrimRight(file.FlatNodeText(idx), "lLuU")
	switch text {
	case "200", "201", "202", "204", "206", "299", "300", "304", "308", "400", "401", "403", "404", "416", "429", "499", "500", "502", "503", "504":
	default:
		return false
	}
	row := file.FlatRow(idx)
	if row < 0 || row >= len(file.Lines) {
		return false
	}
	line := file.Lines[row]
	hasStatusReceiver := strings.Contains(line, "response.code") ||
		strings.Contains(line, "statusCode") ||
		strings.Contains(line, "responseStatus") ||
		strings.Contains(line, "status ")
	if !hasStatusReceiver {
		return false
	}
	return strings.Contains(line, "..") ||
		strings.Contains(line, " until ") ||
		strings.Contains(line, ">=") ||
		strings.Contains(line, "<") ||
		strings.Contains(line, "==") ||
		strings.Contains(line, "!=")
}

// Confidence reports a tier-2 (medium) base confidence. MagicNumber is
// structurally accurate but highly context-dependent: whether a
// literal is "magic" depends on call context, domain, and convention,
// and several of its heuristics (IgnoreComposeUnits, IgnoreRanges,
// IgnoreCompanionObjectPropertyDeclaration) are best-effort. Medium
// confidence lets strict pipelines filter it out while keeping it
// available for default-severity scans.
func (r *MagicNumberRule) Confidence() float64 { return api.ConfidenceMedium }

// magicNumberLiteralTypes is the set of node types dispatched by MagicNumberRule.
// Used to deduplicate when tree-sitter nests e.g. integer_literal inside long_literal.
var magicNumberLiteralTypes = map[string]bool{
	"integer_literal": true,
	"real_literal":    true,
	"long_literal":    true,
	"hex_literal":     true,
}

type magicNumberAncestorContext struct {
	nearestCallName  string
	ancestorCallName map[string]bool
	anyTimeUnitCall  bool
}

func buildMagicNumberAncestorContext(file *scanner.File, idx uint32) *magicNumberAncestorContext {
	ctx := &magicNumberAncestorContext{ancestorCallName: make(map[string]bool, 8)}
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			name := flatCallExpressionName(file, p)
			if name != "" {
				ctx.ancestorCallName[name] = true
				if ctx.nearestCallName == "" {
					ctx.nearestCallName = name
				}
			}
			if callExpressionHasDurationUnitArg(file, p) {
				ctx.anyTimeUnitCall = true
			}
		case "function_declaration", "class_declaration":
			return ctx
		}
	}
	return ctx
}

func magicNumberInsideNamedMethodCall(file *scanner.File, idx uint32, names map[string]bool, ctx *magicNumberAncestorContext) bool {
	if ctx == nil {
		return isInsideNamedMethodCall(file, idx, names)
	}
	for name := range names {
		if ctx.ancestorCallName[name] {
			return true
		}
	}
	return false
}

func magicNumberInsideComposeCall(file *scanner.File, idx uint32, calleeName string, ctx *magicNumberAncestorContext) bool {
	if ctx == nil {
		return isInsideComposeCall(file, idx, calleeName)
	}
	return ctx.nearestCallName == calleeName
}

func magicNumberInsideGeometryDslCall(file *scanner.File, idx uint32, ctx *magicNumberAncestorContext) bool {
	if ctx == nil {
		return isInsideGeometryDslCall(file, idx)
	}
	return geometryDslMethods[ctx.nearestCallName]
}

func magicNumberDurationLiteralWithTimeUnit(file *scanner.File, idx uint32, ctx *magicNumberAncestorContext) bool {
	if ctx == nil {
		return isDurationLiteralWithTimeUnit(file, idx)
	}
	return ctx.anyTimeUnitCall
}

// semanticUIProperties lists View/Compose/animation property names where
// `property = literal` is self-documenting and the literal is not a magic
// number — the property name supplies the semantic label.
var semanticUIProperties = map[string]bool{
	"duration": true, "startDelay": true, "endDelay": true,
	"alpha": true, "rotation": true, "rotationX": true, "rotationY": true,
	"scaleX": true, "scaleY": true, "pivotX": true, "pivotY": true,
	"translationX": true, "translationY": true, "translationZ": true,
	"elevation": true, "cornerRadius": true, "radius": true,
	"strokeWidth": true, "lineHeight": true, "letterSpacing": true,
	"textSize": true, "padding": true, "margin": true,
	"minWidth": true, "maxWidth": true, "minHeight": true, "maxHeight": true,
	"minimumWidth": true, "minimumHeight": true,
	"threshold": true, "progress": true, "max": true, "min": true,
}

// isSemanticPropertyAssignment returns true if the literal is the RHS of an
// assignment whose LHS identifier is a well-known UI/animation property.
func isSemanticPropertyAssignment(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" {
			return false
		}
		if file.FlatType(p) != "assignment" {
			continue
		}
		// LHS is the first named child; extract its text and get the final
		// identifier segment (after any `.`).
		if file.FlatNamedChildCount(p) == 0 {
			return false
		}
		first := file.FlatNamedChild(p, 0)
		lhs := file.FlatNodeText(first)
		if idx := strings.LastIndex(lhs, "."); idx >= 0 {
			lhs = lhs[idx+1:]
		}
		lhs = strings.TrimSpace(lhs)
		return semanticUIProperties[lhs]
	}
	return false
}

// isHTTPStatusExceptionArg returns true if the node is an integer literal
// argument to a constructor whose class name ends in `Exception` or `Error`
// and the literal falls in the HTTP status range 100..599.
func isHTTPStatusExceptionArg(file *scanner.File, idx uint32) bool {
	text := file.FlatNodeText(idx)
	val := 0
	for _, c := range text {
		if c < '0' || c > '9' {
			return false
		}
		val = val*10 + int(c-'0')
	}
	if val < 100 || val > 599 {
		return false
	}
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "call_expression" {
			callee := flatCallExpressionName(file, p)
			if callee == "" {
				return false
			}
			if dot := strings.LastIndex(callee, "."); dot >= 0 {
				callee = callee[dot+1:]
			}
			return strings.HasSuffix(callee, "Exception") || strings.HasSuffix(callee, "Error")
		}
		if file.FlatType(p) == "constructor_invocation" || file.FlatType(p) == "delegation_specifier" {
			// Get the type reference (first named child) and check its name suffix.
			for child := file.FlatFirstChild(p); child != 0; child = file.FlatNextSib(child) {
				if !file.FlatIsNamed(child) {
					continue
				}
				name := file.FlatNodeText(child)
				return strings.HasSuffix(name, "Exception") || strings.HasSuffix(name, "Error")
			}
			return false
		}
		if file.FlatType(p) == "function_declaration" {
			return false
		}
	}
	return false
}

// primitiveArrayBuilders are Kotlin stdlib primitive-array constructors.
// Literal values passed to these are bytes/ints in a sequence, not magic
// numbers that deserve extraction to named constants.
var primitiveArrayBuilders = map[string]bool{
	"byteArrayOf": true, "ubyteArrayOf": true,
	"intArrayOf": true, "uintArrayOf": true,
	"longArrayOf": true, "ulongArrayOf": true,
	"shortArrayOf": true, "ushortArrayOf": true,
	"floatArrayOf": true, "doubleArrayOf": true,
	"charArrayOf": true, "booleanArrayOf": true,
}

// jvmBuilderMethods consume a literal value that's self-documenting within
// the call, so there's no benefit to extracting a named constant.
var jvmBuilderMethods = map[string]bool{
	"valueOf":      true,
	"ofEpochMilli": true, "ofEpochSecond": true, "ofEpochDay": true,
	"ofSeconds": true, "ofMillis": true, "ofMinutes": true,
	"ofHours": true, "ofDays": true, "ofNanos": true,
	"ofYears": true, "ofMonths": true, "ofWeeks": true,
}

// isDurationLiteralWithTimeUnit returns true if the node is a numeric
// literal argument in a call_expression whose argument list contains a
// TimeUnit.X reference — the pair makes the value self-documenting.
func isDurationLiteralWithTimeUnit(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "call_expression" {
			return callExpressionHasDurationUnitArg(file, p)
		}
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" {
			return false
		}
	}
	return false
}

// callExpressionHasDurationUnitArg returns true if any value_argument in the
// call carries canonical duration-unit evidence such as
// `java.util.concurrent.TimeUnit.SECONDS`, imported `TimeUnit.SECONDS`, or
// `kotlin.time.Duration.*`. The import/FQN check keeps local TimeUnit
// lookalikes from suppressing otherwise magic literals.
func callExpressionHasDurationUnitArg(file *scanner.File, callIdx uint32) bool {
	callText := strings.Join(strings.Fields(file.FlatNodeText(callIdx)), "")
	if strings.Contains(callText, "java.util.concurrent.TimeUnit.") ||
		strings.Contains(callText, "kotlin.time.Duration.") ||
		strings.Contains(callText, "java.time.Duration.") {
		return true
	}
	if strings.Contains(callText, "TimeUnit.") &&
		sourceImportsOrMentions(file, "java.util.concurrent.TimeUnit") {
		return true
	}
	if strings.Contains(callText, "Duration.") &&
		(sourceImportsOrMentions(file, "kotlin.time.Duration") ||
			sourceImportsOrMentions(file, "java.time.Duration")) {
		return true
	}
	return false
}

// kritIdiom reports literals that krit exempts on top of what detekt does: common Android, Compose,
// networking and DSL idioms where the number documents itself. detekt reports these, so
// ignoreIdioms: false turns the whole group off.
func (r *MagicNumberRule) kritIdiom(file *scanner.File, idx uint32, text string) bool {
	line := file.Lines[file.FlatRow(idx)]
	if magicNumberLiteralKindIdiom(file, idx, text) {
		return true
	}

	var ancestorCtx *magicNumberAncestorContext
	if experiment.Enabled("magic-number-ancestor-scan") {
		ancestorCtx = buildMagicNumberAncestorContext(file, idx)
	}

	if magicNumberPositionIdiom(file, idx, text, ancestorCtx) {
		return true
	}
	if r.magicNumberCallContextIdiom(file, idx, ancestorCtx) {
		return true
	}
	return r.magicNumberConfiguredIdiom(file, idx, text, line)
}

// magicNumberLiteralKindIdiom reports literals whose spelling or role marks them as self-documenting.
func magicNumberLiteralKindIdiom(file *scanner.File, idx uint32, text string) bool {
	// Numeric literals written with underscore separators
	// (e.g., 25_000L, 1_000_000) have explicit author-intended grouping;
	// these aren't magic numbers, the author already made the value legible.
	if strings.Contains(text, "_") {
		return true
	}
	if magicNumberIsBitOperationLiteral(file, idx) {
		return true
	}
	if magicNumberIsBinaryProtocolLiteral(file, idx) {
		return true
	}
	if magicNumberIsAndroidAPILevelLiteral(file, idx) {
		return true
	}
	return magicNumberIsHTTPStatusLiteral(file, idx)
}

// magicNumberPositionIdiom reports literals whose syntactic position (when branch, index, cardinality
// comparison, SDK check, constant declaration, migration, crypto call) explains them.
func magicNumberPositionIdiom(file *scanner.File, idx uint32, text string, ancestorCtx *magicNumberAncestorContext) bool {
	// Skip literals that are the direct value of a when branch mapping an
	// enum/constant to discrete numeric values — this is a lookup-table
	// idiom, not a magic number. Example: `SIZE.LARGE -> 0.8f`.
	if isWhenBranchValue(file, idx) {
		return true
	}
	// Skip literals inside array/list index access: `parts[1]`, `bytes[3]`.
	// These are positional indices, not magic numbers.
	if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "indexing_suffix" {
		return true
	}
	// Skip literals that are the RHS of a `.size` / `.length` / `.count`
	// equality/comparison. These are intrinsic collection-cardinality
	// checks (e.g. `parts.size == 7`, `daysEnabled.size == 7`), not
	// extractable constants.
	if isSizeCardinalityComparison(file, idx) {
		return true
	}
	// Skip regex-group accessor arguments: `matcher.group(3)`,
	// `match.groupValues[1]`, etc. Regex capture indices are intrinsic
	// to the pattern.
	if experiment.Enabled("magic-number-skip-regex-group-indices") &&
		isInsideRegexGroupAccessor(file, idx) {
		return true
	}
	// Skip SDK_INT comparisons: `Build.VERSION.SDK_INT < 24`, etc.
	// API-level literals are semantic constants.
	if isNearSdkIntComparison(file, idx) {
		return true
	}
	// Skip literals inside SDK-version annotations: @RequiresApi(26),
	// @TargetApi(31), @ChecksSdkIntAtLeast(N), @RequiresExtension(N).
	if isInsideSdkAnnotation(file, idx) {
		return true
	}
	// Skip literals that are the RHS of an ALL_CAPS named constant
	// declaration (e.g., `private val MAX_SIZE = 1024`). Flagging these
	// defeats the "extract to named constant" guidance.
	if isInsideAllCapsConstantDecl(file, idx) {
		return true
	}
	// Skip literals inside database migration methods (`onUpgrade`,
	// `onDowngrade`, `onCreate`) where version comparisons reference
	// historical schema versions.
	if isInsideDbMigrationMethod(file, idx) {
		return true
	}
	// Skip literals inside crypto/KDF calls — output lengths, key sizes
	// are dictated by the algorithm, not arbitrary.
	if magicNumberInsideNamedMethodCall(file, idx, cryptoMethods, ancestorCtx) {
		return true
	}
	// Skip hex literals (0x...) — hex notation is already self-documenting
	// as a color/mask/byte pattern.
	return strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X")
}

// magicNumberCallContextIdiom reports literals passed to builder, DSL, geometry and similar calls whose API
// documents the value.
func (r *MagicNumberRule) magicNumberCallContextIdiom(file *scanner.File, idx uint32, ancestorCtx *magicNumberAncestorContext) bool {
	// Skip Bitmap/collection/cache builder constructors.
	if magicNumberInsideNamedMethodCall(file, idx, bitmapBuilderMethods, ancestorCtx) {
		return true
	}
	// Skip HTTP status comparisons: `x.status == 200`, `code >= 500`.
	if isHTTPStatusComparison(file, idx) {
		return true
	}
	// Skip literals inside Compose `Color(...)` calls — component values
	// 0f-1f are semantic color channels, not magic numbers.
	if magicNumberInsideComposeCall(file, idx, "Color", ancestorCtx) {
		return true
	}
	// Skip literals inside Compose/Canvas/Path/animator DSL methods where
	// raw coordinates are inherent to the API.
	if magicNumberInsideGeometryDslCall(file, idx, ancestorCtx) {
		return true
	}
	// Skip literals inside coordinate/size constructors like `PointF(x, y)`
	// or setters like `point.set(x, y)`.
	if magicNumberInsideNamedMethodCall(file, idx, coordinateConstructors, ancestorCtx) {
		return true
	}
	// Skip literals inside dimension/animation DSL calls.
	if magicNumberInsideNamedMethodCall(file, idx, dimensionConversionMethods, ancestorCtx) ||
		magicNumberInsideNamedMethodCall(file, idx, animationMethods, ancestorCtx) {
		return true
	}
	// Skip literals inside range expressions / IntRange constants
	// (e.g., `0x0000..0x024F` Unicode ranges).
	if file.FlatHasAncestorOfType(idx, "range_expression") {
		return true
	}
	// Skip literals that are part of a `to` infix expression used in a
	// collection builder (mapOf lookup tables, version maps).
	if isInsideToInfixMap(file, idx) {
		return true
	}
	// Skip literals inside preview/sample/fake/mock/stub functions — these
	// are scaffolding for UI tooling, not production constants.
	if isInsidePreviewOrSampleFunctionFlatWithConfig(file, idx, customPreviewConfigFromFields(r.CustomPreviewWildcard, r.CustomPreviewPrefixes)) {
		return true
	}
	// Skip literals that are the duration argument of a call whose sibling
	// is a `TimeUnit.X` reference (e.g., `Single.timer(200, TimeUnit.MILLISECONDS)`).
	if magicNumberDurationLiteralWithTimeUnit(file, idx, ancestorCtx) {
		return true
	}
	// Skip literals that are arguments to java.math / java.time builder
	// calls where the literal is documentational (BigDecimal.valueOf(3),
	// Instant.ofEpochMilli(0), Duration.ofSeconds(30), etc.).
	if magicNumberInsideNamedMethodCall(file, idx, jvmBuilderMethods, ancestorCtx) {
		return true
	}
	// Skip literals inside primitive-array builders `byteArrayOf(1, 2, 3)` —
	// these are data payloads (test fixtures, magic bytes, handshake
	// sequences), never meaningful constants to extract.
	if magicNumberInsideNamedMethodCall(file, idx, primitiveArrayBuilders, ancestorCtx) {
		return true
	}
	// Skip HTTP status code literals inside exception constructor calls
	// (e.g., `NonSuccessfulResponseCodeException(404)`). The exception
	// type name together with the known HTTP status range is
	// self-documenting.
	if isHTTPStatusExceptionArg(file, idx) {
		return true
	}
	// Skip literals that are the RHS of an assignment to a semantic UI /
	// animation / layout property — the property name itself documents
	// the value (`duration = 250`, `textSize = 14f`, `elevation = 4f`).
	return isSemanticPropertyAssignment(file, idx)
}

// magicNumberConfiguredIdiom reports color literals and Compose units when the matching options are on.
func (r *MagicNumberRule) magicNumberConfiguredIdiom(file *scanner.File, idx uint32, text, line string) bool {
	trimmed := strings.TrimSpace(line)
	// Skip color literals: hex numbers inside Color() calls or in color-named properties
	if r.IgnoreColorLiterals && strings.HasPrefix(text, "0x") {
		if strings.Contains(line, "Color(") {
			return true
		}
		lowerLine := strings.ToLower(trimmed)
		if strings.Contains(lowerLine, "color") || strings.Contains(lowerLine, "background") ||
			strings.Contains(lowerLine, "tint") || strings.Contains(lowerLine, "palette") {
			return true
		}
	}

	// Skip Compose dimension units: N.dp, N.sp, N.px, N.em
	if r.IgnoreComposeUnits {
		endByte := int(file.FlatEndByte(idx))
		if endByte+3 <= len(file.Content) {
			after := string(file.Content[endByte:min(endByte+4, len(file.Content))])
			if strings.HasPrefix(after, ".dp") || strings.HasPrefix(after, ".sp") ||
				strings.HasPrefix(after, ".px") || strings.HasPrefix(after, ".em") {
				return true
			}
		}
	}
	return false
}
