package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const wakeLockClass = "android.os.PowerManager.WakeLock"

// WakelockTimeoutRule mirrors Lint's WakelockTimeout: WakeLock.acquire() without a timeout.
type WakelockTimeoutRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver type is inferred from declarations.
func (r *WakelockTimeoutRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WakelockTimeoutRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "acquire" || len(call.Args) != 0 || call.ReceiverNode == 0 {
		return
	}
	if !lintReceiverIsType(ctx, call, wakeLockClass) && !receiverDeclaredFromCall(file, ctx.Idx, "newWakeLock") &&
		!lintCallChainReturnsWakeLock(file, call) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "",
		"Provide a timeout when requesting a wakelock with `PowerManager.Wakelock.acquire(long timeout)`. This will ensure the OS will cleanup any wakelocks that last longer than you intend, and will save your user's battery."))
}

// lintCallChainReturnsWakeLock matches `powerManager.newWakeLock(...).acquire()`.
func lintCallChainReturnsWakeLock(file *scanner.File, call lintCall) bool {
	inner, ok := lintCallAt(file, call.ReceiverNode)
	return ok && inner.Name == "newWakeLock"
}
