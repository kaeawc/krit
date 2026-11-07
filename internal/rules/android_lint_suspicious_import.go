package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
)

// SuspiciousImportRule mirrors Lint's SuspiciousImport: `import android.R` (or `android.R.*`) without an alias.
type SuspiciousImportRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a high base confidence: the check is a literal import path.
func (r *SuspiciousImportRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *SuspiciousImportRule) check(ctx *api.Context) {
	file := ctx.File
	text := strings.TrimSpace(file.FlatNodeText(ctx.Idx))
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "import"), ";"))
	text = strings.TrimSpace(strings.TrimPrefix(text, "static "))
	if text != "android.R" && text != "android.R.*" {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Don't include `android.R` here; use a fully qualified name for each usage instead"))
}
