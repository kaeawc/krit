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
	"regexp"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
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

var secondaryCtorRe = regexp.MustCompile(`constructor\s*\([^)]+\)`)

// allParamsHaveDefaults checks if every parameter in the param list has a default value.
func allParamsHaveDefaults(params string) bool {
	// Split by comma, accounting for nested generics
	depth := 0
	start := 0
	var parts []string
	for i, c := range params {
		switch c {
		case '<', '(':
			depth++
		case '>', ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, params[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, params[start:])

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) == 0 {
			continue
		}
		if !strings.Contains(p, "=") {
			return false
		}
	}
	return true
}

// =====================================================================
// 2. GetSignaturesRule
// =====================================================================

// GetSignaturesRule flags use of the deprecated PackageManager.GET_SIGNATURES
// constant. GET_SIGNING_CERTIFICATES should be used instead (API 28+).
type GetSignaturesRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GetSignaturesRule) Confidence() float64 { return 0.75 }

func (r *GetSignaturesRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if strings.Contains(line, "GET_SIGNATURES") &&
			!strings.Contains(line, "GET_SIGNING_CERTIFICATES") {
			ctx.Emit(r.Finding(file, i+1, 1,
				"GET_SIGNATURES is deprecated and can be spoofed. Use GET_SIGNING_CERTIFICATES (API 28+) instead."))
		}
	}
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


var (
	tagConstRe  = regexp.MustCompile(`(?:const\s+val|val)\s+TAG\s*(?::\s*String)?\s*=\s*"([^"]*)"`)
	classNameRe = regexp.MustCompile(`(?:class|object)\s+(\w+)`)
)

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
					companionText := file.FlatNodeText(member)
					m := tagConstRe.FindStringSubmatch(companionText)
					if m != nil {
						return m[1]
					}
				}
			}
			break
		}
	}
	return ""
}

// =====================================================================
// 7. NonInternationalizedSmsRule
// =====================================================================

// NonInternationalizedSmsRule flags SmsManager.sendTextMessage() calls that
// may not handle internationalization of phone numbers or message encoding.
type NonInternationalizedSmsRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NonInternationalizedSmsRule) Confidence() float64 { return 0.75 }

func (r *NonInternationalizedSmsRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if strings.Contains(line, "sendTextMessage") || strings.Contains(line, "sendMultipartTextMessage") {
			if strings.Contains(line, "SmsManager") || strings.Contains(line, "smsManager") {
				ctx.Emit(r.Finding(file, i+1, 1,
					"SMS sending may not handle internationalization of phone numbers properly."))
			}
		}
	}
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

var serviceCastRe = regexp.MustCompile(`getSystemService\s*\(\s*(?:\w+\.)?(\w+_SERVICE)\s*\)\s*(?:as\s+(\w+))`)

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
