package rules

import (
	androidproject "github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules/api"
)

const pendingIntentClass = "android.app.PendingIntent"

// pendingIntentConsts holds the PendingIntent flag values, keyed by qualified name for lintEvalInt.
var pendingIntentConsts = map[string]int64{
	pendingIntentClass + ".FLAG_ONE_SHOT":                     1 << 30,
	pendingIntentClass + ".FLAG_NO_CREATE":                    1 << 29,
	pendingIntentClass + ".FLAG_CANCEL_CURRENT":               1 << 28,
	pendingIntentClass + ".FLAG_UPDATE_CURRENT":               1 << 27,
	pendingIntentClass + ".FLAG_IMMUTABLE":                    1 << 26,
	pendingIntentClass + ".FLAG_MUTABLE":                      1 << 25,
	pendingIntentClass + ".FLAG_ALLOW_UNSAFE_IMPLICIT_INTENT": 1 << 24,
}

// pendingIntentFlagMask is the bit set Lint tests to decide a flags argument already carries a mutability decision.
const pendingIntentFlagMask = 234881024

// UnspecifiedImmutableFlagRule mirrors Lint's UnspecifiedImmutableFlag: a PendingIntent created without
// FLAG_IMMUTABLE or FLAG_MUTABLE.
type UnspecifiedImmutableFlagRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver is matched by name and imports.
func (r *UnspecifiedImmutableFlagRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnspecifiedImmutableFlagRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok {
		return
	}
	switch call.Name {
	case "getActivity", "getActivities", "getBroadcast", "getService":
	default:
		return
	}
	if !lintTypeRefMatches(file, call.ReceiverText, pendingIntentClass) {
		return
	}
	arg := call.Arg(3, "flags")
	if arg == 0 {
		return
	}
	flags, ok := lintEvalInt(file, arg, pendingIntentConsts)
	if !ok || flags&pendingIntentFlagMask != 0 {
		return
	}
	// Lint drops the finding below API 23 and demotes it to a warning below API 31.
	severity := ""
	if sdk := lintFileTargetSdk(ctx); sdk > 0 {
		if sdk < 23 {
			return
		}
		if sdk < 31 {
			severity = "warning"
		}
	}
	ctx.Emit(lintFindingAtNode(ctx, arg, severity, "Missing `PendingIntent` mutability flag"))
}

// lintFileTargetSdk mirrors Lint's per-module target SDK: the declared targetSdk, else the minSdk, read from the
// nearest build file or manifest above the file. It falls back to the project-wide Gradle profile, and returns 0
// when nothing is known.
func lintFileTargetSdk(ctx *api.Context) int {
	if ctx.File != nil {
		levels := androidproject.ResolveSDKLevels(ctx.File.Path)
		if levels.TargetSdk > 0 {
			return levels.TargetSdk
		}
		if levels.MinSdk > 0 {
			return levels.MinSdk
		}
	}
	return lintTargetSdk(ctx)
}
