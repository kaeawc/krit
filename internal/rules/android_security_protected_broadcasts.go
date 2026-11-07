package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// protectedBroadcastConstants lists system broadcast action constants that only the system can send.
// A receiver that listens only to these needs no exported flag. Names are unique across framework classes.
var protectedBroadcastConstants = map[string]bool{
	"ACTION_SCREEN_ON": true, "ACTION_SCREEN_OFF": true, "ACTION_USER_PRESENT": true,
	"ACTION_BATTERY_CHANGED": true, "ACTION_BATTERY_LOW": true, "ACTION_BATTERY_OKAY": true,
	"ACTION_POWER_CONNECTED": true, "ACTION_POWER_DISCONNECTED": true,
	"ACTION_TIMEZONE_CHANGED": true, "ACTION_TIME_CHANGED": true, "ACTION_TIME_TICK": true, "ACTION_DATE_CHANGED": true,
	"ACTION_LOCALE_CHANGED": true, "ACTION_CONFIGURATION_CHANGED": true,
	"ACTION_BOOT_COMPLETED": true, "ACTION_LOCKED_BOOT_COMPLETED": true, "ACTION_SHUTDOWN": true, "ACTION_REBOOT": true,
	"ACTION_AIRPLANE_MODE_CHANGED": true, "ACTION_CLOSE_SYSTEM_DIALOGS": true, "ACTION_HEADSET_PLUG": true,
	"ACTION_DEVICE_STORAGE_LOW": true, "ACTION_DEVICE_STORAGE_OK": true,
	"ACTION_PACKAGE_ADDED": true, "ACTION_PACKAGE_REMOVED": true, "ACTION_PACKAGE_REPLACED": true,
	"ACTION_PACKAGE_CHANGED": true, "ACTION_PACKAGE_FULLY_REMOVED": true, "ACTION_PACKAGE_DATA_CLEARED": true,
	"ACTION_MY_PACKAGE_REPLACED": true, "ACTION_USER_UNLOCKED": true,
	"ACTION_DEVICE_IDLE_MODE_CHANGED": true, "ACTION_POWER_SAVE_MODE_CHANGED": true,
	"ACTION_LOW_POWER_STANDBY_ENABLED_CHANGED": true,
	"CONNECTIVITY_ACTION":                      true, "ACTION_RESTRICT_BACKGROUND_CHANGED": true,
	"WIFI_STATE_CHANGED_ACTION": true, "NETWORK_STATE_CHANGED_ACTION": true, "SCAN_RESULTS_AVAILABLE_ACTION": true,
	"ACTION_AUDIO_BECOMING_NOISY": true, "RINGER_MODE_CHANGED_ACTION": true, "ACTION_PHONE_STATE_CHANGED": true,
}

// protectedBroadcastQualifiers are the framework classes that declare the constants above.
var protectedBroadcastQualifiers = map[string]bool{
	"Intent": true, "PowerManager": true, "ConnectivityManager": true, "WifiManager": true,
	"AudioManager": true, "TelephonyManager": true, "BluetoothAdapter": true,
	"android.content.Intent": true, "android.os.PowerManager": true, "android.net.ConnectivityManager": true,
	"android.net.wifi.WifiManager": true, "android.media.AudioManager": true,
	"android.telephony.TelephonyManager": true,
}

var (
	intentFilterCtorArg   = regexp.MustCompile(`IntentFilter\(\s*([^,()]+(?:\([^()]*\))?[^,()]*)`)
	intentFilterAddAction = regexp.MustCompile(`addAction\(\s*([^,()]+)\s*\)`)
	actionStringLiteral   = regexp.MustCompile(`^"(android\.intent\.action\.([A-Z0-9_]+))"$`)
)

func isProtectedBroadcastAction(expr string) bool {
	expr = strings.TrimSpace(expr)
	if m := actionStringLiteral.FindStringSubmatch(expr); m != nil {
		return protectedBroadcastActions[m[1]] || protectedBroadcastConstants["ACTION_"+m[2]]
	}
	qualifier, name := "", expr
	if i := strings.LastIndex(expr, "."); i >= 0 {
		qualifier, name = expr[:i], expr[i+1:]
	}
	return protectedBroadcastConstants[name] && (qualifier == "" || protectedBroadcastQualifiers[qualifier])
}

// broadcastFilterActions returns the actions added to the filter argument, or nil when they cannot be resolved.
func broadcastFilterActions(file *scanner.File, call uint32, filterArg string) []string {
	filterArg = strings.TrimSpace(filterArg)
	if strings.Contains(filterArg, "IntentFilter(") {
		return collectFilterActions(filterArg)
	}
	if filterArg == "" || strings.ContainsAny(filterArg, "().") {
		return nil
	}
	scope, ok := flatEnclosingCallable(file, call)
	if !ok {
		return nil
	}
	text := file.FlatNodeText(scope)
	decl := regexp.MustCompile(`\b` + regexp.QuoteMeta(filterArg) + `\b\s*(?::\s*IntentFilter\s*)?=\s*(?:new\s+)?IntentFilter\(`).FindStringIndex(text)
	if decl == nil {
		return nil
	}
	return collectFilterActions(text[decl[0]:])
}

// collectFilterActions gathers the constructor action and every addAction argument in text. Extra
// addAction calls from unrelated filters only add actions, which keeps the result conservative.
func collectFilterActions(text string) []string {
	var actions []string
	if i := strings.Index(text, "IntentFilter("); i >= 0 {
		if m := intentFilterCtorArg.FindStringSubmatch(text[i:]); m != nil && !strings.HasPrefix(text[i:], "IntentFilter()") {
			actions = append(actions, strings.TrimSpace(m[1]))
		}
	}
	for _, m := range intentFilterAddAction.FindAllStringSubmatch(text, -1) {
		actions = append(actions, strings.TrimSpace(m[1]))
	}
	return actions
}

// broadcastFilterOnlyProtectedActions reports a registration whose filter lists only protected system broadcasts.
func broadcastFilterOnlyProtectedActions(file *scanner.File, call uint32, positional []string, compat bool) bool {
	idx := 1
	if compat {
		idx = 2
	}
	if idx >= len(positional) {
		return false
	}
	actions := broadcastFilterActions(file, call, positional[idx])
	if len(actions) == 0 {
		return false
	}
	for _, action := range actions {
		if !isProtectedBroadcastAction(action) {
			return false
		}
	}
	return true
}
