package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// HardcodedRandomSeedRule detects SecureRandom(seed) constructor calls (Kotlin and
// Java) where any seed argument is supplied. A hardcoded seed defeats
// cryptographic randomness; the default constructor should be used instead.
//
// This is a krit-only rule. It was named TrulyRandom, which is the id Android
// Lint uses for a different check (weak SecureRandom initialization on
// Android 4.3 and older, see TrulyRandomRule); the old id stays a suppression alias.
//
// SecureRandom.setSeed() with a deterministic argument is already covered by
// SecureRandomRule. This rule covers the constructor form on both languages.
type HardcodedRandomSeedRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection is anchored
// to an imported or fully-qualified java.security.SecureRandom reference, so
// local lookalikes no longer fire; project-specific wrapper APIs named
// SecureRandom can still produce false negatives when not imported.
func (r *HardcodedRandomSeedRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *HardcodedRandomSeedRule) check(ctx *api.Context) {
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		r.checkKotlinCall(ctx, file, ctx.Idx)
	case "object_creation_expression":
		r.checkJavaConstruction(ctx, file, ctx.Idx)
	}
}

func (r *HardcodedRandomSeedRule) checkKotlinCall(ctx *api.Context, file *scanner.File, call uint32) {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 || file.FlatNamedChildCount(args) == 0 {
		return
	}
	if !secureRandomKotlinConstructorCall(file, call) {
		return
	}
	r.emit(ctx, file, call)
}

func (r *HardcodedRandomSeedRule) checkJavaConstruction(ctx *api.Context, file *scanner.File, node uint32) {
	if !secureRandomJavaObjectCreationIsSecureRandom(file, node) {
		return
	}
	args, ok := file.FlatFindChild(node, "argument_list")
	if !ok || file.FlatNamedChildCount(args) == 0 {
		return
	}
	r.emit(ctx, file, node)
}

func (r *HardcodedRandomSeedRule) emit(ctx *api.Context, file *scanner.File, node uint32) {
	ctx.Emit(r.Finding(file, file.FlatRow(node)+1, file.FlatCol(node)+1,
		"SecureRandom constructed with an explicit seed can be predictable. Use the default constructor so the platform supplies the entropy."))
}
