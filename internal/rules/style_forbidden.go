package rules

import (
	"regexp"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/scanner"
)

// WildcardImportRule detects import x.y.* statements.
type WildcardImportRule struct {
	FlatDispatchBase
	BaseRule
	ExcludeImports []string // wildcard imports matching these prefixes are allowed
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *WildcardImportRule) Confidence() float64 { return 0.75 }



// ForbiddenCommentRule detects TODO:, FIXME:, STOPSHIP: markers.
type ForbiddenCommentRule struct {
	FlatDispatchBase
	BaseRule
	Comments        []string       // forbidden comment markers
	AllowedPatterns *regexp.Regexp // regex; comments matching this are allowed
}

var defaultForbiddenCommentMarkers = []string{"TODO:", "FIXME:", "STOPSHIP:"}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenCommentRule) Confidence() float64 { return 0.75 }



// ForbiddenVoidRule detects Void type usage.
type ForbiddenVoidRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreOverridden      bool
	IgnoreUsageInGenerics bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenVoidRule) Confidence() float64 { return 0.75 }


// javaInteropGenericTypes are Java generic types where Void is the canonical
// way to say "no result" and Unit is not substitutable.
var javaInteropGenericTypes = map[string]bool{
	"AsyncTask":         true,
	"Callable":          true,
	"CompletableFuture": true,
	"Future":            true,
	"ListenableFuture":  true,
	"Supplier":          true,
	"Function":          true,
	"BiFunction":        true,
	"Single":            true, // RxJava
	"Maybe":             true,
	"Observable":        true,
	"Flowable":          true,
	"Completable":       true,
}


// ForbiddenImportRule detects banned import patterns.
type ForbiddenImportRule struct {
	FlatDispatchBase
	BaseRule
	Patterns         []string // kept for backward compat; same as ForbiddenImports
	ForbiddenImports []string
	AllowedImports   []string
}

var defaultForbiddenImports = []string{
	"sun.",
	"jdk.internal.",
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenImportRule) Confidence() float64 { return 0.75 }



// ForbiddenEntry pairs a forbidden value with an optional reason.
type ForbiddenEntry struct {
	Value  string
	Reason string
}

// ForbiddenMethodCallRule detects banned method calls.
type ForbiddenMethodCallRule struct {
	FlatDispatchBase
	BaseRule
	Methods []string // simple list kept for backward compat
}

var defaultForbiddenMethods = []string{"print(", "println("}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenMethodCallRule) Confidence() float64 { return 0.75 }



// ForbiddenAnnotationRule detects annotations that should not be used.
type ForbiddenAnnotationRule struct {
	FlatDispatchBase
	BaseRule
	Annotations []string
}

var defaultForbiddenAnnotations = []string{"SuppressWarnings"}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenAnnotationRule) Confidence() float64 { return 0.75 }



// ForbiddenNamedParamRule detects named parameters in certain function calls.
type ForbiddenNamedParamRule struct {
	FlatDispatchBase
	BaseRule
	Methods []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenNamedParamRule) Confidence() float64 { return 0.75 }



// ForbiddenOptInRule detects @OptIn annotations.
type ForbiddenOptInRule struct {
	FlatDispatchBase
	BaseRule
	MarkerClasses []string // specific marker classes to forbid; empty = all @OptIn
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenOptInRule) Confidence() float64 { return 0.75 }



// ForbiddenSuppressRule detects @Suppress annotations.
type ForbiddenSuppressRule struct {
	FlatDispatchBase
	BaseRule
	Rules []string // specific suppressed rules to forbid; empty = all @Suppress
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenSuppressRule) Confidence() float64 { return 0.75 }



// MagicNumberRule detects literal numbers in code.
type MagicNumberRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreAnnotated                          []string
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

	ignoredNumbersOnce sync.Once
	ignoredNumbersMap  map[string]bool
}

func (r *MagicNumberRule) ignoredNumberSet() map[string]bool {
	r.ignoredNumbersOnce.Do(func() {
		nums := r.IgnoreNumbers
		if len(nums) == 0 {
			nums = []string{"-1", "0", "1", "2"}
		}
		m := make(map[string]bool, len(nums)*2)
		for _, n := range nums {
			m[n] = true
			// Also store the stripped form so that configured values like
			// "0.5f" / "1000L" match the cleaned literal text used at lookup.
			clean := strings.TrimRight(n, "fFdDlLuU")
			clean = strings.ReplaceAll(clean, "_", "")
			m[clean] = true
		}
		r.ignoredNumbersMap = m
	})
	return r.ignoredNumbersMap
}


// Confidence reports a tier-2 (medium) base confidence. MagicNumber is
// structurally accurate but highly context-dependent: whether a
// literal is "magic" depends on call context, domain, and convention,
// and several of its heuristics (IgnoreComposeUnits, IgnoreRanges,
// IgnoreCompanionObjectPropertyDeclaration) are best-effort. Medium
// confidence lets strict pipelines filter it out while keeping it
// available for default-severity scans.
func (r *MagicNumberRule) Confidence() float64 { return 0.75 }

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
			text := file.FlatNodeText(p)
			if strings.Contains(text, "TimeUnit.") || strings.Contains(text, "Duration.") {
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

// isHttpStatusExceptionArg returns true if the node is an integer literal
// argument to a constructor whose class name ends in `Exception` or `Error`
// and the literal falls in the HTTP status range 100..599.
func isHttpStatusExceptionArg(file *scanner.File, idx uint32) bool {
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
			if idx := strings.LastIndex(callee, "."); idx >= 0 {
				callee = callee[idx+1:]
			}
			return strings.HasSuffix(callee, "Exception") || strings.HasSuffix(callee, "Error")
		}
		if file.FlatType(p) == "constructor_invocation" || file.FlatType(p) == "delegation_specifier" {
			text := file.FlatNodeText(p)
			return strings.Contains(text, "Exception(") || strings.Contains(text, "Error(")
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
	// Walk up to find the enclosing call_expression.
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "call_expression" {
			text := file.FlatNodeText(p)
			if strings.Contains(text, "TimeUnit.") || strings.Contains(text, "Duration.") {
				return true
			}
			return false
		}
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" {
			return false
		}
	}
	return false
}

// isInsidePreviewOrSampleFunction returns true if the node is inside a
// function whose name or annotation marks it as a preview / sample / fake
// / mock / stub — UI tooling scaffolding rather than production code.
func isInsidePreviewOrSampleFunctionFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "function_declaration" {
			continue
		}
		name := extractIdentifierFlat(file, p)
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "preview") || strings.HasPrefix(lower, "sample") ||
			strings.HasPrefix(lower, "fake") || strings.HasPrefix(lower, "mock") ||
			strings.HasPrefix(lower, "stub") || strings.HasPrefix(lower, "fixture") ||
			strings.HasSuffix(lower, "preview") || strings.HasSuffix(lower, "sample") ||
			strings.HasSuffix(lower, "fixture") {
			return true
		}
		// Also check for @Preview / @SignalPreview annotation.
		mods, _ := file.FlatFindChild(p, "modifiers")
		if mods != 0 {
			modText := file.FlatNodeText(mods)
			if strings.Contains(modText, "@Preview") || strings.Contains(modText, "@SignalPreview") ||
				strings.Contains(modText, "@DarkPreview") || strings.Contains(modText, "@LightPreview") ||
				strings.Contains(modText, "@DayNightPreviews") {
				return true
			}
		}
		return false
	}
	return false
}

// dimensionConversionMethods are methods where a numeric argument is a dp/sp
// design value, not a magic number.
var dimensionConversionMethods = map[string]bool{
	"dpToPx": true, "spToPx": true, "pxToDp": true, "pxToSp": true,
	"toPixels": true, "toDp": true, "toSp": true, "toPx": true,
	// Compose / extension syntax
	"dp": true, "sp": true, "em": true, "px": true,
}

// animationMethods consume literal durations/delays.
var animationMethods = map[string]bool{
	"setDuration": true, "setStartDelay": true, "duration": true,
	"startDelay": true, "animateTo": true, "withStartAction": true,
	"setRepeatCount": true, "setRepeatMode": true,
	// Kotlin stdlib numeric clamping — the literal is a domain bound.
	"coerceAtMost": true, "coerceAtLeast": true, "coerceIn": true,
	// JobManager / WorkManager builders — literals are config values.
	"setMaxAttempts": true, "setMaxInstancesForQueue": true,
	"setInitialDelay": true, "setBackoffCriteria": true,
	"setLifespan": true, "setMinimumLatency": true,
	"setOverrideDeadline": true, "setRequiresCharging": true,
	"setPeriodic": true,
	// SQL fluent builders — row limits/offsets are query-shape constants.
	"limit": true, "offset": true, "take": true, "drop": true,
	"chunked": true, "windowed": true,
	// View fade/slide helpers — the numeric arg is a millis duration.
	"fadeIn": true, "fadeOut": true, "fadeInOut": true,
	"slideIn": true, "slideOut": true, "crossFade": true,
	"animateAlpha": true, "animateVisibility": true,
	// Compose semantic token wrappers — the integer IS the semantic label.
	"FontWeight": true,
	// Numeric radix / base conversions — the integer is the numeric base.
	"toString": true, "parseInt": true, "parseLong": true,
	"toInt": true, "toLong": true,
}

// geometryDslMethods are methods where numeric literal arguments represent
// coordinates, angles, scales, or alphas — semantic values inherent to the
// API and not magic numbers.
var coordinateConstructors = map[string]bool{
	"PointF": true, "Point": true, "RectF": true, "Rect": true,
	"Offset": true, "Size": true, "Vector": true, "Vector2": true,
	"set": true, "setTo": true, "setValues": true,
	"PathDashPathEffect": true, "DashPathEffect": true,
	"HSVToColor": true, "HSLToColor": true,
	// Material motion / bezier interpolator control points.
	"PathInterpolator": true, "PathInterpolatorCompat": true,
	"CubicBezierEasing": true,
	// Signal-specific UI helpers.
	"GridDividerDecoration":   true,
	"appendCenteredImageSpan": true,
	// QR / image data builders — sizes are domain constants.
	"forData": true,
	// Credit card / phone-number grouping DSL.
	"applyGrouping": true,
	// Callbacks where a literal is the dispatched event data (keypad
	// digit, menu index, etc.) — the call name carries the meaning.
	"onKeyPress": true, "onDigitPress": true, "onItemClick": true,
	"onPageSelected": true, "onTabSelected": true,
}

var geometryDslMethods = map[string]bool{
	// Canvas/Path
	"moveTo": true, "lineTo": true, "cubicTo": true, "quadTo": true,
	"rMoveTo": true, "rLineTo": true, "rCubicTo": true, "rQuadTo": true,
	"arcTo": true, "rArcTo": true, "addArc": true, "addOval": true,
	"addRect": true, "addRoundRect": true, "addCircle": true,
	"drawRect": true, "drawRoundRect": true, "drawCircle": true,
	"drawLine": true, "drawPoint": true, "drawOval": true, "drawArc": true,
	"rotate": true, "rotateX": true, "rotateY": true, "rotateZ": true,
	"scale": true, "scaleX": true, "scaleY": true,
	"translate": true, "translationX": true, "translationY": true, "translationZ": true,
	"alpha": true, "setAlpha": true,
	"setX": true, "setY": true, "setZ": true,
	// Compose ImageVector / PathBuilder DSL — all coordinates are raw
	// vector-drawable data and are never meaningful constants to extract.
	"moveToRelative": true, "lineToRelative": true,
	"curveTo": true, "curveToRelative": true,
	"reflectiveCurveTo": true, "reflectiveCurveToRelative": true,
	"horizontalLineTo": true, "horizontalLineToRelative": true,
	"verticalLineTo": true, "verticalLineToRelative": true,
	"arcToRelative": true, "quadToRelative": true,
	"reflectiveQuadTo": true, "reflectiveQuadToRelative": true,
	"materialPath": true, "path": true, "group": true,
	"rewind": true,
	// Brush/gradient
	"verticalGradient": true, "horizontalGradient": true, "linearGradient": true,
	"radialGradient": true, "sweepGradient": true,
	// Compose layout
	"offset": true, "padding": true, "size": true, "width": true, "height": true,
}

// isInsideComposeCall returns true if the node is an argument inside a call
// to a function with the given simple name.
func isInsideComposeCall(file *scanner.File, idx uint32, calleeName string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "call_expression" {
			name := flatCallExpressionName(file, p)
			return name == calleeName
		}
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" {
			return false
		}
	}
	return false
}

// isInsideNamedMethodCall returns true if the node is an argument to a call
// whose callee simple name is in the given set.
func isInsideNamedMethodCall(file *scanner.File, idx uint32, names map[string]bool) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "call_expression" {
			name := flatCallExpressionName(file, p)
			if names[name] {
				return true
			}
			// Continue walking outward through nested calls — a literal
			// inside `listOf(...)` inside `applyGrouping(...)` should still
			// match on the outer call.
			continue
		}
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" {
			return false
		}
	}
	return false
}

// isInsideGeometryDslCall returns true if the node is an argument to a known
// geometry/Compose DSL method where raw numeric literals are semantic.
func isInsideGeometryDslCall(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "call_expression" {
			name := flatCallExpressionName(file, p)
			if geometryDslMethods[name] {
				return true
			}
			return false
		}
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" {
			return false
		}
	}
	return false
}

// isInsideToInfixMap returns true if the literal appears as an operand of
// a `to` infix expression, typically used in mapOf() / listOf() pair builders
// for lookup tables where numeric constants are semantically named.
func isInsideToInfixMap(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "infix_expression" {
			text := file.FlatNodeText(p)
			// Check if it contains ` to ` as the infix operator
			if strings.Contains(text, " to ") {
				return true
			}
		}
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" {
			return false
		}
	}
	return false
}

// bitmapBuilderMethods take pixel dimensions, quality percentages, and
// capacity hints that are self-documenting at the call site. Also includes
// time-unit converters and byte-array constructors whose literals are
// inherently semantic.
var bitmapBuilderMethods = map[string]bool{
	"createScaledBitmap": true,
	"createBitmap":       true,
	"compress":           true,
	"decodeResource":     true,
	"decodeByteArray":    true,
	// Collection capacity
	"ArrayList":       true,
	"HashMap":         true,
	"HashSet":         true,
	"LinkedHashMap":   true,
	"LinkedHashSet":   true,
	"ArrayDeque":      true,
	"LruCache":        true,
	"SparseArray":     true,
	"SparseIntArray":  true,
	"SparseLongArray": true,
	// Time-unit converters (TimeUnit.MINUTES.toMillis(30) etc.)
	"toMillis": true, "toSeconds": true, "toMinutes": true,
	"toHours": true, "toDays": true, "toMicros": true, "toNanos": true,
	// Byte-array / buffer sizes
	"readNBytes": true, "readNBytesOrThrow": true,
	"allocate": true, "allocateDirect": true,
	// Duration constructors (Kotlin stdlib)
	"milliseconds": true, "seconds": true, "minutes": true,
	"hours": true, "days": true, "nanoseconds": true, "microseconds": true,
}

// isHttpStatusComparison returns true if the literal is on the RHS of a
// comparison against a variable/property whose name suggests an HTTP status.
func isHttpStatusComparison(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "equality_expression", "comparison_expression":
			text := file.FlatNodeText(p)
			lower := strings.ToLower(text)
			if strings.Contains(lower, "status") ||
				strings.Contains(lower, "statuscode") ||
				strings.Contains(lower, "httpcode") ||
				strings.Contains(lower, ".code") {
				return true
			}
			return false
		case "function_declaration", "class_declaration":
			return false
		}
	}
	return false
}

// cryptoMethods consume sizes/lengths that are dictated by crypto primitives.
var cryptoMethods = map[string]bool{
	"deriveSecrets": true, "hkdf": true, "HKDF": true,
	"pbkdf2": true, "PBKDF2": true, "scrypt": true, "argon2": true,
	"generateKey": true, "generateKeyPair": true,
	"hash": true, "digest": true,
	// Cryptographic buffer/IV/salt/nonce helpers — sizes are dictated
	// by the primitive, not the author.
	"getSecretBytes": true, "getSecretBytesInt": true,
	"ByteArray": true, "ByteBuffer": true, "allocate": true,
	"getIv": true, "getNonce": true, "getSalt": true, "getKeyBytes": true,
	"generateIv": true, "generateNonce": true, "generateSalt": true,
	"randomBytes": true, "secureRandomBytes": true, "nextBytes": true,
	// Byte-slice operations on crypto-derived buffers (HKDF outputs,
	// key material, MAC keys). The numeric bounds are structural
	// offsets dictated by the primitive's output layout, not magic
	// numbers. Example: `extendedKey.copyOfRange(32, 64)` slices the
	// MAC key out of an HKDF-derived buffer.
	"copyOfRange": true, "sliceArray": true,
	// Android Handler/View delay APIs — the millis is the intended
	// delay value, already documented by the method name.
	"postDelayed": true, "postAtTime": true, "sendMessageDelayed": true,
	"delay": true, "delayMillis": true, "schedule": true,
}

// dbMigrationMethods are Android SQLite lifecycle method names where version
// integers are historical constants, not magic numbers.
var dbMigrationMethods = map[string]bool{
	"onUpgrade": true, "onDowngrade": true, "onCreate": true,
	"migrate": true,
}

// isInsideDbMigrationMethod returns true if the node is inside a function
// named onUpgrade/onDowngrade/onCreate/migrate. Schema version comparisons
// reference historical constants, not magic numbers.
func isInsideDbMigrationMethod(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" {
			name := extractIdentifierFlat(file, p)
			return dbMigrationMethods[name]
		}
	}
	return false
}

// isInsideAllCapsConstantDecl returns true if the node is inside a
// property_declaration whose identifier is ALL_CAPS (e.g., MAX_SIZE,
// TIMEOUT_MS). These are the extracted constants MagicNumber asks us to
// create — flagging their RHS is backwards.
func isInsideAllCapsConstantDecl(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "property_declaration" {
			name := extractIdentifierFlat(file, p)
			if name == "" {
				return false
			}
			// Check all chars are upper or underscore or digit, and at least
			// one is a letter (not e.g. `_` or `123`).
			hasLetter := false
			for _, c := range name {
				if c >= 'A' && c <= 'Z' {
					hasLetter = true
					continue
				}
				if c == '_' || (c >= '0' && c <= '9') {
					continue
				}
				return false
			}
			return hasLetter
		}
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" {
			return false
		}
	}
	return false
}

// sdkVersionAnnotations lists annotation names whose numeric args are API
// level constants, not magic numbers.
var sdkVersionAnnotations = map[string]bool{
	"RequiresApi": true, "TargetApi": true, "ChecksSdkIntAtLeast": true,
	"RequiresExtension": true, "SdkConstant": true,
}

// isInsideSdkAnnotation returns true if the node is inside an annotation
// argument list for a known SDK-version annotation.
func isInsideSdkAnnotation(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "annotation" {
			text := file.FlatNodeText(p)
			text = strings.TrimPrefix(text, "@")
			if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
				text = text[:parenIdx]
			}
			if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
				text = text[dotIdx+1:]
			}
			text = strings.TrimSpace(text)
			return sdkVersionAnnotations[text]
		}
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "class_declaration" {
			return false
		}
	}
	return false
}

// isNearSdkIntComparison returns true if the literal is a direct operand
// of a binary expression whose other operand references SDK_INT.
func isNearSdkIntComparison(file *scanner.File, idx uint32) bool {
	p, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	// Check binary/comparison expression parents
	switch file.FlatType(p) {
	case "comparison_expression", "equality_expression", "binary_expression":
		pText := file.FlatNodeText(p)
		return strings.Contains(pText, "SDK_INT") || strings.Contains(pText, "Build.VERSION")
	}
	return false
}

// isWhenBranchValue reports whether the node is either the result
// expression OR the match pattern of a `when` entry (e.g. `5 -> "five"`
// or `CASE -> 0.8f`). Both forms are part of a lookup table, not magic
// numbers.
func isWhenBranchValue(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "when_entry" || t == "when_condition" {
			return true
		}
		// Stop walking at expression boundaries
		if t == "statements" || t == "function_body" || t == "class_body" ||
			t == "lambda_literal" || t == "if_expression" || t == "try_expression" {
			return false
		}
	}
	return false
}

// isInsideRegexGroupAccessor reports whether the given literal is an
// argument to a `Matcher` / `MatchResult` group accessor (`group(N)`,
// `groupValues[N]`, `range(N)`, `start(N)`, `end(N)`). These capture
// group indices are intrinsic to the regex pattern.
func isInsideRegexGroupAccessor(file *scanner.File, idx uint32) bool {
	// Walk up looking for an enclosing call_expression whose
	// navigation_expression ends in one of the group accessor names,
	// OR an indexing_suffix whose base navigation ends in
	// `groupValues`.
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			navExpr, _ := flatCallExpressionParts(file, p)
			if navExpr == 0 {
				continue
			}
			last := flatNavigationExpressionLastIdentifier(file, navExpr)
			switch last {
			case "group", "range", "start", "end":
				return true
			}
		case "navigation_expression":
			t := strings.TrimSpace(file.FlatNodeText(p))
			if strings.HasSuffix(t, ".groupValues") ||
				strings.HasSuffix(t, ".groups") {
				return true
			}
		case "function_declaration", "class_body", "source_file":
			return false
		}
	}
	return false
}

// isSizeCardinalityComparison reports whether the node is an integer
// literal that is the RHS of an equality/comparison whose other operand
// ends in `.size`, `.length`, or `.count`. These represent intrinsic
// collection shape checks, not magic numbers.
func isSizeCardinalityComparison(file *scanner.File, idx uint32) bool {
	p, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	t := file.FlatType(p)
	if t != "equality_expression" && t != "comparison_expression" {
		return false
	}
	for i := 0; i < file.FlatChildCount(p); i++ {
		c := file.FlatChild(p, i)
		if c == idx {
			continue
		}
		txt := strings.TrimSpace(file.FlatNodeText(c))
		if strings.HasSuffix(txt, ".size") ||
			strings.HasSuffix(txt, ".length") ||
			strings.HasSuffix(txt, ".count") ||
			strings.HasSuffix(txt, ".size()") ||
			strings.HasSuffix(txt, ".length()") ||
			strings.HasSuffix(txt, ".count()") {
			return true
		}
	}
	return false
}

// isLocalProperty checks if the property_declaration ancestor is inside a function body.
func (r *MagicNumberRule) isLocalProperty(file *scanner.File, idx uint32) bool {
	// Walk up to find the property_declaration, then check if it's inside a function_body
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "property_declaration" {
			// Now check if this property is inside a function_body
			for pp, ok := file.FlatParent(p); ok; pp, ok = file.FlatParent(pp) {
				if file.FlatType(pp) == "function_body" || file.FlatType(pp) == "statements" {
					return true
				}
				if file.FlatType(pp) == "class_body" || file.FlatType(pp) == "source_file" {
					return false
				}
			}
			return false
		}
	}
	return false
}

// isPartOfInfixRange checks if a number is part of an infix range call like
// 1 downTo 0, 0 until 10, or step expressions.
func (r *MagicNumberRule) isPartOfInfixRange(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "infix_expression" {
			pText := file.FlatNodeText(p)
			if strings.Contains(pText, " downTo ") || strings.Contains(pText, " until ") ||
				strings.Contains(pText, " step ") {
				return true
			}
		}
	}
	return false
}
