package rules

import "github.com/kaeawc/krit/internal/rules/api"

// SetWorldReadableRule mirrors Lint's SetWorldReadable: File.setReadable(true, false).
type SetWorldReadableRule struct {
	FlatDispatchBase
	AndroidRule
}

// SetWorldWritableRule mirrors Lint's SetWorldWritable: File.setWritable(true, false).
type SetWorldWritableRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver type is inferred from declarations.
func (r *SetWorldReadableRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence: the receiver type is inferred from declarations.
func (r *SetWorldWritableRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SetWorldReadableRule) check(ctx *api.Context) {
	worldPermissionCheck(ctx, "setReadable", "Setting file permissions to world-readable can be risky, review carefully")
}

func (r *SetWorldWritableRule) check(ctx *api.Context) {
	worldPermissionCheck(ctx, "setWritable", "Setting file permissions to world-writable can be risky, review carefully")
}

// worldPermissionCheck reports `file.<method>(true, false)`, which grants the permission to everyone.
func worldPermissionCheck(ctx *api.Context, method, msg string) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != method || len(call.Args) != 2 {
		return
	}
	if v, ok := lintEvalBool(file, call.Args[0]); !ok || !v {
		return
	}
	if v, ok := lintEvalBool(file, call.Args[1]); !ok || v {
		return
	}
	if !lintReceiverIsType(ctx, call, javaIoFile) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", msg))
}
