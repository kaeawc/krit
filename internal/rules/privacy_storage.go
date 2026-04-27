package rules

import (
	"regexp"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

var sensitiveStorageKeyPattern = regexp.MustCompile(`(?i)(token|secret|password|pin|auth|credential|private.?key)`)

var sensitiveFileNamePattern = regexp.MustCompile(`(?i)(credential|token|secret|auth|password|private.?key)`)

// SharedPreferencesForSensitiveKeyRule flags putString/putInt/putLong calls
// on SharedPreferences with a key literal matching sensitive patterns.
// Sensitive data should use EncryptedSharedPreferences or the Keystore.
type SharedPreferencesForSensitiveKeyRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SharedPreferencesForSensitiveKeyRule) Confidence() float64 { return 0.75 }

func (r *SharedPreferencesForSensitiveKeyRule) check(ctx *v2.Context) {
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

	if !sensitiveStorageKeyPattern.MatchString(body) {
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

func (r *PlainFileWriteOfSensitiveRule) Confidence() float64 { return 0.75 }

func (r *PlainFileWriteOfSensitiveRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	name := flatCallExpressionName(file, idx)
	if name != "writeText" && name != "writeBytes" {
		return
	}
	if !plainFileWriteCall(ctx, idx) || encryptedStorageReceiver(ctx, idx) {
		return
	}

	receiverLiteral := sensitiveFileReceiverLiteral(ctx, idx)
	if receiverLiteral == "" || !sensitiveFileNamePattern.MatchString(receiverLiteral) {
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

func (r *LogOfSharedPreferenceReadRule) Confidence() float64 { return 0.75 }

func (r *LogOfSharedPreferenceReadRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	name := flatCallExpressionName(file, idx)
	if !isLogMethod(name) {
		return
	}

	receiver := flatReceiverNameFromCall(file, idx)
	if !isLogReceiver(receiver) {
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
		if sensitiveStorageKeyPattern.MatchString(body) {
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
	case "putString", "putInt", "putLong", "putFloat", "putBoolean", "putStringSet":
		return true
	}
	return false
}

func isSharedPrefsGetMethod(name string) bool {
	switch name {
	case "getString", "getInt", "getLong", "getFloat", "getBoolean", "getStringSet":
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
