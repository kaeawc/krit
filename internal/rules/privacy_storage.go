package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// The patterns match whole snake_case words. Names are split into words first
// (see privacyWordMatch), so `pin` does not match `shipping` or `spinner`.
var sensitiveStorageKeyPattern = regexp.MustCompile(`(?i)(^|_)(tokens?|secrets?|passwords?|passwd|pin|pins|auth|oauth|credentials?|private_?keys?|api_?keys?)(_|$)`)

var sensitiveFileNamePattern = regexp.MustCompile(`(?i)(^|_)(credentials?|tokens?|secrets?|auth|oauth|passwords?|passwd|private_?keys?)(_|$)`)

var privacyNonWordChars = regexp.MustCompile(`[^A-Za-z0-9]+`)

// privacyWordMatch matches pattern against name as written and against name
// split into lowercase snake_case words at camelCase and punctuation boundaries.
func privacyWordMatch(pattern *regexp.Regexp, name string) bool {
	if pattern.MatchString(name) {
		return true
	}
	snake := logPiiCamelBoundary.ReplaceAllString(name, "${1}${3}_${2}${4}")
	snake = privacyNonWordChars.ReplaceAllString(snake, "_")
	return pattern.MatchString(snake)
}

// sensitiveStorageKeyBenignWords mark a key that names metadata about a secret
// (its expiry, count or flag) rather than the secret itself.
var sensitiveStorageKeyBenignWords = map[string]bool{
	"expires": true, "expiry": true, "expiration": true, "expired": true, "expire": true,
	"time": true, "timestamp": true, "at": true, "count": true, "counter": true, "attempts": true,
	"enabled": true, "disabled": true, "visible": true, "shown": true, "hint": true, "type": true,
	"state": true, "status": true, "version": true, "length": true, "label": true, "has": true, "is": true,
}

// sensitiveStorageKey reports whether a SharedPreferences key names a secret value.
func sensitiveStorageKey(key string) bool {
	if !privacyWordMatch(sensitiveStorageKeyPattern, key) {
		return false
	}
	snake := logPiiCamelBoundary.ReplaceAllString(key, "${1}${3}_${2}${4}")
	for _, w := range privacyNonWordChars.Split(strings.ToLower(snake), -1) {
		if sensitiveStorageKeyBenignWords[w] {
			return false
		}
	}
	return true
}

// SharedPreferencesForSensitiveKeyRule flags putString/putInt/putLong calls
// on SharedPreferences with a key literal matching sensitive patterns.
// Sensitive data should use EncryptedSharedPreferences or the Keystore.
type SharedPreferencesForSensitiveKeyRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SharedPreferencesForSensitiveKeyRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SharedPreferencesForSensitiveKeyRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	name := flatCallExpressionName(file, idx)
	if !isSharedPrefsPutMethod(name) {
		return
	}
	if !sharedPrefsEditorCall(ctx, idx) || encryptedStorageReceiver(ctx, idx) {
		return
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return
	}

	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		return
	}
	argExpr := flatValueArgumentExpression(file, arg)
	if argExpr == 0 {
		return
	}

	body, ok := kotlinStringLiteralBody(file.FlatNodeText(argExpr))
	if !ok {
		return
	}

	if !sensitiveStorageKey(body) {
		return
	}

	ctx.Emit(r.Finding(
		file,
		file.FlatRow(argExpr)+1,
		file.FlatCol(argExpr)+1,
		"SharedPreferences key \""+body+"\" looks sensitive. Use EncryptedSharedPreferences or the Android Keystore for sensitive data.",
	))
}

// PlainFileWriteOfSensitiveRule flags writeText/writeBytes calls on File
// objects whose filename contains sensitive patterns.
type PlainFileWriteOfSensitiveRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PlainFileWriteOfSensitiveRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *PlainFileWriteOfSensitiveRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	name := flatCallExpressionName(file, idx)
	if name != "writeText" && name != "writeBytes" {
		return
	}
	if !plainFileWriteCall(ctx, idx) || encryptedStorageReceiver(ctx, idx) {
		return
	}

	receiverLiteral := sensitiveFileReceiverLiteral(ctx, idx)
	if receiverLiteral == "" || !privacyWordMatch(sensitiveFileNamePattern, receiverLiteral) {
		return
	}

	ctx.Emit(r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Plain-file write to a path containing sensitive terms. Use EncryptedFile or the Android Keystore for sensitive data.",
	))
}

// LogOfSharedPreferenceReadRule flags logger calls whose argument directly
// passes a SharedPreferences getString/getInt value with a sensitive key.
type LogOfSharedPreferenceReadRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LogOfSharedPreferenceReadRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LogOfSharedPreferenceReadRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	name := flatCallExpressionName(file, idx)
	if !isLogMethod(name) {
		return
	}

	receiver := flatReceiverNameFromCall(file, idx)
	if !isLogReceiver(receiver) {
		return
	}
	if logReceiverIsLocalShadow(ctx, idx, receiver) {
		return
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return
	}

	file.FlatWalkNodes(args, "call_expression", func(innerCall uint32) {
		innerName := flatCallExpressionName(file, innerCall)
		if !isSharedPrefsGetMethod(innerName) {
			return
		}
		if !sharedPrefsEditorCall(ctx, innerCall) || encryptedStorageReceiver(ctx, innerCall) {
			return
		}

		_, innerArgs := flatCallExpressionParts(file, innerCall)
		if innerArgs == 0 {
			return
		}
		keyArg := flatPositionalValueArgument(file, innerArgs, 0)
		if keyArg == 0 {
			return
		}
		keyExpr := flatValueArgumentExpression(file, keyArg)
		if keyExpr == 0 {
			return
		}
		body, ok := kotlinStringLiteralBody(file.FlatNodeText(keyExpr))
		if !ok {
			return
		}
		if sensitiveStorageKey(body) {
			ctx.Emit(r.Finding(
				file,
				file.FlatRow(innerCall)+1,
				file.FlatCol(innerCall)+1,
				"Logging the value of SharedPreferences key \""+body+"\". Sensitive data from preferences should not be written to logs.",
			))
		}
	})
}

func isSharedPrefsPutMethod(name string) bool {
	switch name {
	case "putString", "putInt", "putStringSet":
		return true
	}
	return false
}

func isSharedPrefsGetMethod(name string) bool {
	switch name {
	case "getString", "getInt", "getStringSet":
		return true
	}
	return false
}

func isLogMethod(name string) bool {
	switch name {
	case "d", "i", "w", "e", "v", "wtf":
		return true
	}
	return false
}

func isLogReceiver(receiver string) bool {
	switch receiver {
	case "Log", "Timber":
		return true
	}
	return false
}

// logReceiverIsLocalShadow reports whether `receiver` at `callIdx` is a value
// (val/var/parameter) that happens to share the simple name of the Log/Timber
// class. Only consulted when a resolver is available (e.g. --depth=thorough).
// A class/object named Log or Timber resolves with the same simple name, so
// only a mismatched Name indicates a real value-shadow.
func logReceiverIsLocalShadow(ctx *api.Context, callIdx uint32, receiver string) bool {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return false
	}
	t := ctx.Resolver.ResolveByNameFlat(receiver, callIdx, ctx.File)
	if t == nil {
		return false
	}
	return t.Name != "" && t.Name != receiver
}
