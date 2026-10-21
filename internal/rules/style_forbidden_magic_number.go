package rules

import (
	"strings"
	"sync"

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
