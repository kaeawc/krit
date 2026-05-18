package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TrulyRandomRule detects SecureRandom(seed) constructor calls (Kotlin and
// Java) where any seed argument is supplied. Per Android lint's
// TrulyRandomDetector, a hardcoded seed defeats cryptographic randomness —
// the default constructor should be used instead.
//
// SecureRandom.setSeed() with a deterministic argument is already covered by
// SecureRandomRule. This rule covers the constructor form on both languages.
type TrulyRandomRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection is anchored
// to an imported or fully-qualified java.security.SecureRandom reference, so
// local lookalikes no longer fire; project-specific wrapper APIs named
// SecureRandom can still produce false negatives when not imported.
func (r *TrulyRandomRule) Confidence() float64 { return 0.85 }

func (r *TrulyRandomRule) check(ctx *api.Context) {
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		r.checkKotlinCall(ctx, file, ctx.Idx)
	case "object_creation_expression":
		r.checkJavaConstruction(ctx, file, ctx.Idx)
	}
}

func (r *TrulyRandomRule) checkKotlinCall(ctx *api.Context, file *scanner.File, call uint32) {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 || file.FlatNamedChildCount(args) == 0 {
		return
	}
	if !secureRandomKotlinConstructorCall(file, call) {
		return
	}
	r.emit(ctx, file, call)
}

func (r *TrulyRandomRule) checkJavaConstruction(ctx *api.Context, file *scanner.File, node uint32) {
	if !secureRandomJavaObjectCreationIsSecureRandom(file, node) {
		return
	}
	args, ok := file.FlatFindChild(node, "argument_list")
	if !ok || file.FlatNamedChildCount(args) == 0 {
		return
	}
	r.emit(ctx, file, node)
}

func (r *TrulyRandomRule) emit(ctx *api.Context, file *scanner.File, node uint32) {
	ctx.Emit(r.Finding(file, file.FlatRow(node)+1, file.FlatCol(node)+1,
		"SecureRandom with a hardcoded seed is not secure. Use the default constructor for cryptographic randomness."))
}
