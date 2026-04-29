package rules

// Android Lint source-only rules: 13 checks that analyze Kotlin/Java source
// using tree-sitter AST or line scanning. No XML infrastructure needed.
//
// Ported from AOSP Android Lint detectors:
//   FragmentDetector, GetSignaturesDetector, JavaPerformance (SparseArray, ValueOf),
//   LogDetector (LONG_TAG, WRONG_TAG), NonInternationalizedSmsDetector,
//   ServiceCastDetector, ToastDetector, ViewConstructorDetector, ViewTagDetector,
//   WrongImportDetector, LayoutInflationDetector.

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// =====================================================================
// 1. FragmentConstructorRule
// =====================================================================

// FragmentConstructorRule flags Fragment subclasses that have non-default
// (parameterized) constructors without a no-arg constructor. The Android
// framework re-instantiates fragments via reflection using the no-arg ctor.
type FragmentConstructorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *FragmentConstructorRule) Confidence() float64 { return 0.75 }

// fragmentSuperclasses covers common Fragment base classes.
var fragmentSuperclasses = []string{
	"Fragment", "DialogFragment", "ListFragment", "PreferenceFragment",
	"PreferenceFragmentCompat", "BottomSheetDialogFragment",
}

type fragmentConstructorState struct {
	hasNoArgCtor bool
	hasParamCtor bool
}

func fragmentConstructorStateFlat(file *scanner.File, classDecl uint32) fragmentConstructorState {
	state := fragmentConstructorState{hasNoArgCtor: true}
	for child := file.FlatFirstChild(classDecl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "primary_constructor":
			count, defaults := constructorParameterDefaultsFlat(file, child, "class_parameter")
			if count == 0 {
				state.hasNoArgCtor = true
				continue
			}
			state.hasParamCtor = true
			state.hasNoArgCtor = defaults == count
		case "class_body":
			file.FlatWalkNodes(child, "secondary_constructor", func(ctor uint32) {
				count, defaults := constructorParameterDefaultsFlat(file, ctor, "parameter")
				if count == 0 {
					state.hasNoArgCtor = true
					return
				}
				state.hasParamCtor = true
				if defaults == count {
					state.hasNoArgCtor = true
				}
			})
		}
	}
	return state
}

func constructorParameterDefaultsFlat(file *scanner.File, ctor uint32, paramType string) (count int, defaults int) {
	file.FlatWalkNodes(ctor, paramType, func(param uint32) {
		if nearestConstructorAncestorFlat(file, param) != ctor {
			return
		}
		count++
		if parameterHasDefaultValueFlat(file, param) {
			defaults++
		}
	})
	return count, defaults
}

func nearestConstructorAncestorFlat(file *scanner.File, idx uint32) uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "primary_constructor", "secondary_constructor":
			return cur
		case "class_declaration", "class_body":
			return 0
		}
	}
	return 0
}

func parameterHasDefaultValueFlat(file *scanner.File, param uint32) bool {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatNodeTextEquals(child, "=") {
			return true
		}
	}
	return false
}

// =====================================================================
// 2. GetSignaturesRule
// =====================================================================

// GetSignaturesRule flags use of the deprecated PackageManager.GET_SIGNATURES
// constant. GET_SIGNING_CERTIFICATES should be used instead (API 28+).
type GetSignaturesRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GetSignaturesRule) Confidence() float64 { return 0.75 }

const getSignaturesFlagValue int64 = 0x40

func getSignaturesCallUsesDeprecatedFlag(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 || flatCallExpressionName(file, call) != "getPackageInfo" {
		return false
	}
	if apiGuardedByVersionCheckFlat(file, call) {
		return false
	}
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	flagArg := flatPositionalValueArgument(file, args, 1)
	if flagArg == 0 {
		flagArg = flatNamedValueArgument(file, args, "flags")
	}
	if flagArg == 0 {
		return false
	}
	expr := flatValueArgumentExpression(file, flagArg)
	if getSignaturesExprContainsFlag(file, expr) {
		return true
	}
	name := flatReferenceSimpleName(file, expr)
	if name == "" {
		return false
	}
	fn, ok := flatEnclosingFunction(file, call)
	if !ok {
		return false
	}
	return functionLocalInitializerContainsGetSignatures(file, fn, name, call)
}

func getSignaturesExprContainsFlag(file *scanner.File, expr uint32) bool {
	if file == nil || expr == 0 {
		return false
	}
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier":
		return file.FlatNodeText(expr) == "GET_SIGNATURES"
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, expr) == "GET_SIGNATURES"
	case "integer_literal", "long_literal", "hex_literal":
		v, ok := parseIntegerLiteralForFlag(file.FlatNodeText(expr))
		return ok && (v&getSignaturesFlagValue) != 0
	}
	found := false
	file.FlatWalkAllNodes(expr, func(node uint32) {
		if found || node == expr {
			return
		}
		if getSignaturesExprContainsFlag(file, node) {
			found = true
		}
	})
	return found
}

func flatReferenceSimpleName(file *scanner.File, expr uint32) string {
	if file == nil || expr == 0 {
		return ""
	}
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier":
		return file.FlatNodeText(expr)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, expr)
	default:
		return ""
	}
}

func functionLocalInitializerContainsGetSignatures(file *scanner.File, fn uint32, name string, before uint32) bool {
	if file == nil || fn == 0 || name == "" {
		return false
	}
	found := false
	targetRow := file.FlatRow(before)
	file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
		if found || file.FlatRow(decl) > targetRow || propertyDeclarationName(file, decl) != name {
			return
		}
		if getSignaturesExprContainsFlag(file, propertyInitializerExpression(file, decl)) {
			found = true
		}
	})
	return found
}

func parseIntegerLiteralForFlag(text string) (int64, bool) {
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(strings.TrimSuffix(text, "L"), "l")
	text = strings.ReplaceAll(text, "_", "")
	if text == "" {
		return 0, false
	}
	base := 10
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		base = 16
		text = text[2:]
	} else if strings.HasPrefix(text, "0b") || strings.HasPrefix(text, "0B") {
		base = 2
		text = text[2:]
	}
	v, err := strconv.ParseInt(text, base, 64)
	return v, err == nil
}

// =====================================================================
// 3. SparseArrayRule
// =====================================================================

// SparseArrayRule flags HashMap<Integer, ...> / HashMap<Int, ...> usage where
// SparseArray or SparseIntArray would be more efficient on Android.
type SparseArrayRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SparseArrayRule) Confidence() float64 { return 0.9 }

// =====================================================================
// 4. UseValueOfRule
// =====================================================================

// UseValueOfRule flags direct constructor calls for boxed primitive types
// (e.g., Integer(42), Boolean(true)) where valueOf() should be used instead
// for object reuse.
type UseValueOfRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *UseValueOfRule) Confidence() float64 { return 0.9 }

var boxedPrimitiveConstructors = map[string]bool{
	"Integer":   true,
	"Long":      true,
	"Float":     true,
	"Double":    true,
	"Short":     true,
	"Byte":      true,
	"Boolean":   true,
	"Character": true,
}

// =====================================================================
// 5. LogTagLengthRule (LogDetector.LONG_TAG)
// =====================================================================

// LogTagLengthRule flags Log.x() calls where the tag string literal exceeds
// 23 characters (the maximum enforced by older Android versions).
type LogTagLengthRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LogTagLengthRule) Confidence() float64 { return 0.9 }

// resolveLogTagStringValue resolves the string value of a Log tag
// argument expression. It handles direct string literals as well as
// references to a `const val` / `val` declaration whose initializer
// is a non-interpolated string literal. Returns ("", false) when the
// value cannot be resolved.
func resolveLogTagStringValue(file *scanner.File, expr uint32) (string, bool) {
	if file == nil || expr == 0 {
		return "", false
	}
	expr = flatUnwrapParenExpr(file, expr)
	if file.FlatType(expr) == "string_literal" {
		if flatContainsStringInterpolation(file, expr) {
			return "", false
		}
		return stringLiteralContent(file, expr), true
	}
	name := flatReferenceSimpleName(file, expr)
	if name == "" {
		return "", false
	}
	return findConstStringPropertyValue(file, name)
}

// findConstStringPropertyValue scans every property_declaration in the
// file for one named `name` whose initializer is a non-interpolated
// string literal, and returns its content.
func findConstStringPropertyValue(file *scanner.File, name string) (string, bool) {
	if file == nil || name == "" {
		return "", false
	}
	value := ""
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(node uint32) {
		if found {
			return
		}
		if propertyDeclarationName(file, node) != name {
			return
		}
		init := propertyInitializerExpression(file, node)
		if init == 0 || file.FlatType(init) != "string_literal" {
			return
		}
		if flatContainsStringInterpolation(file, init) {
			return
		}
		value = stringLiteralContent(file, init)
		found = true
	})
	return value, found
}

// =====================================================================
// 6. LogTagMismatchRule (LogDetector.WRONG_TAG)
// =====================================================================

// LogTagMismatchRule flags Log.x(TAG, ...) calls where the TAG companion
// constant value doesn't match the enclosing class name.
type LogTagMismatchRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LogTagMismatchRule) Confidence() float64 { return 0.75 }

// findDirectCompanionTag searches the direct children of a class_declaration
// for a companion_object containing a TAG constant, returning its string value.
// This avoids matching TAG constants defined in nested classes.
func findDirectCompanionTagFlat(file *scanner.File, classNode uint32) string {
	for i := 0; i < file.FlatChildCount(classNode); i++ {
		child := file.FlatChild(classNode, i)
		if file.FlatType(child) == "class_body" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				member := file.FlatChild(child, j)
				if file.FlatType(member) == "companion_object" {
					if tag := directCompanionTagValueFlat(file, member); tag != "" {
						return tag
					}
				}
			}
			break
		}
	}
	return ""
}

func directCompanionTagValueFlat(file *scanner.File, companion uint32) string {
	tag := ""
	file.FlatWalkNodes(companion, "property_declaration", func(prop uint32) {
		if tag != "" || extractIdentifierFlat(file, prop) != "TAG" {
			return
		}
		if !nodeIsDirectlyInsideFlat(file, prop, companion) {
			return
		}
		expr := propertyInitializerExpression(file, prop)
		if expr == 0 {
			return
		}
		switch file.FlatType(expr) {
		case "string_literal":
			if flatContainsStringInterpolation(file, expr) {
				return
			}
			tag = stringLiteralContent(file, expr)
		case "navigation_expression":
			// `Foo::class.java.simpleName` — treat the referenced class
			// name as the effective tag value so it can be compared to
			// the enclosing class.
			if name := classLiteralSimpleNameRef(file, expr); name != "" {
				tag = name
			}
		}
	})
	return tag
}

// classLiteralSimpleNameRef extracts `Foo` from an expression of the form
// `Foo::class.java.simpleName` (or the Kotlin-only `Foo::class.simpleName`).
// Returns "" if the expression is anything else.
func classLiteralSimpleNameRef(file *scanner.File, expr uint32) string {
	if file.FlatType(expr) != "navigation_expression" {
		return ""
	}
	if flatNavigationExpressionLastIdentifier(file, expr) != "simpleName" {
		return ""
	}
	var ref uint32
	file.FlatWalkNodes(expr, "callable_reference", func(n uint32) {
		if ref == 0 {
			ref = n
		}
	})
	if ref == 0 {
		return ""
	}
	for c := file.FlatFirstChild(ref); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "type_identifier", "simple_identifier":
			return file.FlatNodeText(c)
		}
	}
	return ""
}

func nodeIsDirectlyInsideFlat(file *scanner.File, node uint32, container uint32) bool {
	for cur, ok := file.FlatParent(node); ok; cur, ok = file.FlatParent(cur) {
		if cur == container {
			return true
		}
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "function_declaration", "lambda_literal":
			return false
		}
	}
	return false
}

// =====================================================================
// 7. NonInternationalizedSmsRule
// =====================================================================

// NonInternationalizedSmsRule flags SmsManager.sendTextMessage() calls that
// may not handle internationalization of phone numbers or message encoding.
type NonInternationalizedSmsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NonInternationalizedSmsRule) Confidence() float64 { return 0.75 }

func nonInternationalizedSmsCallFlat(file *scanner.File, call uint32) bool {
	name := flatCallExpressionName(file, call)
	if name != "sendTextMessage" && name != "sendMultipartTextMessage" {
		return false
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	if !smsManagerReceiverFlat(file, receiver, call) {
		return false
	}
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		return false
	}
	expr := flatUnwrapParenExpr(file, flatValueArgumentExpression(file, arg))
	if expr == 0 || file.FlatType(expr) != "string_literal" {
		return false
	}
	if flatContainsStringInterpolation(file, expr) {
		return false
	}
	return !strings.HasPrefix(stringLiteralContent(file, expr), "+")
}

func smsManagerReceiverFlat(file *scanner.File, receiver uint32, call uint32) bool {
	if path := contactsIdentifierPathFlat(file, receiver); len(path) > 0 && path[0] == "SmsManager" {
		return true
	}
	name := flatReferenceSimpleName(file, receiver)
	if name == "" {
		return false
	}
	if name == "smsManager" {
		return true
	}
	return contactsSameOwnerDeclarationHasTypeFlat(file, call, name, "SmsManager")
}

// =====================================================================
// 8. ServiceCastRule
// =====================================================================

// ServiceCastRule flags getSystemService() calls cast to the wrong type.
// For example, getSystemService(ALARM_SERVICE) as PowerManager.
type ServiceCastRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ServiceCastRule) Confidence() float64 { return 0.75 }

// serviceCastMap maps service constant names to their correct manager types.
var serviceCastMap = map[string]string{
	"ACCESSIBILITY_SERVICE":   "AccessibilityManager",
	"ACCOUNT_SERVICE":         "AccountManager",
	"ACTIVITY_SERVICE":        "ActivityManager",
	"ALARM_SERVICE":           "AlarmManager",
	"AUDIO_SERVICE":           "AudioManager",
	"BATTERY_SERVICE":         "BatteryManager",
	"BLUETOOTH_SERVICE":       "BluetoothManager",
	"CAMERA_SERVICE":          "CameraManager",
	"CLIPBOARD_SERVICE":       "ClipboardManager",
	"CONNECTIVITY_SERVICE":    "ConnectivityManager",
	"DEVICE_POLICY_SERVICE":   "DevicePolicyManager",
	"DISPLAY_SERVICE":         "DisplayManager",
	"DOWNLOAD_SERVICE":        "DownloadManager",
	"INPUT_METHOD_SERVICE":    "InputMethodManager",
	"JOB_SCHEDULER_SERVICE":   "JobScheduler",
	"KEYGUARD_SERVICE":        "KeyguardManager",
	"LAYOUT_INFLATER_SERVICE": "LayoutInflater",
	"LOCATION_SERVICE":        "LocationManager",
	"MEDIA_ROUTER_SERVICE":    "MediaRouter",
	"NOTIFICATION_SERVICE":    "NotificationManager",
	"NSD_SERVICE":             "NsdManager",
	"POWER_SERVICE":           "PowerManager",
	"SEARCH_SERVICE":          "SearchManager",
	"SENSOR_SERVICE":          "SensorManager",
	"STORAGE_SERVICE":         "StorageManager",
	"TELECOM_SERVICE":         "TelecomManager",
	"TELEPHONY_SERVICE":       "TelephonyManager",
	"USB_SERVICE":             "UsbManager",
	"VIBRATOR_SERVICE":        "Vibrator",
	"WALLPAPER_SERVICE":       "WallpaperManager",
	"WIFI_SERVICE":            "WifiManager",
	"WINDOW_SERVICE":          "WindowManager",
}

// =====================================================================
// 9. ToastRule (ShowToast)
// =====================================================================

// ToastRule flags Toast.makeText() calls where .show() is never called
// on the result. The Toast will not be displayed without show().
type ToastRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ToastRule) Confidence() float64 { return 0.85 }

func toastMakeTextIsShown(file *scanner.File, call uint32) bool {
	if ancestorCallNameMatches(file, call, "show") {
		return true
	}
	if ancestorApplyLambdaShowsToast(file, call) {
		return true
	}
	toastVar := initializerAssignedName(file, call)
	if toastVar == "" {
		return false
	}
	fn, ok := flatEnclosingFunction(file, call)
	if !ok {
		return false
	}
	return functionHasReceiverCallAfter(file, fn, call, toastVar, showCallName, nil)
}

func ancestorApplyLambdaShowsToast(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "call_expression":
			if flatCallExpressionName(file, parent) != "apply" {
				continue
			}
			lambda := flatCallTrailingLambda(file, parent)
			if lambda == 0 {
				continue
			}
			found := false
			file.FlatWalkNodes(lambda, "call_expression", func(call uint32) {
				if found || flatCallExpressionName(file, call) != "show" {
					return
				}
				if flatReceiverNameFromCall(file, call) == "" {
					found = true
				}
			})
			if found {
				return true
			}
		case "function_declaration", "source_file":
			return false
		}
	}
	return false
}
