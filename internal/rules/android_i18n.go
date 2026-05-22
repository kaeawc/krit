package rules

// Android Lint I18N rules. Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ByteOrderMarkRule detects BOM (byte order mark) in files.
type ByteOrderMarkRule struct{ AndroidRule }

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — the BOM check is a literal three-byte compare at the start
// of the file content. No heuristic path.
func (r *ByteOrderMarkRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *ByteOrderMarkRule) check(ctx *api.Context) {
	file := ctx.File
	if len(file.Content) >= 3 &&
		file.Content[0] == 0xEF && file.Content[1] == 0xBB && file.Content[2] == 0xBF {
		ctx.Emit(r.Finding(file, 1, 1,
			"File contains a UTF-8 byte order mark (BOM). Remove the BOM for consistency."))
		return
	}
}
