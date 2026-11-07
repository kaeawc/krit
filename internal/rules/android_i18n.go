package rules

// Android Lint I18N rules. Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	"bytes"

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
	// A BOM at the start of a file is legal; Lint reports one after the first character, where files were joined.
	for i := 1; i+2 < len(file.Content); i++ {
		if file.Content[i] == 0xEF && file.Content[i+1] == 0xBB && file.Content[i+2] == 0xBF {
			line := bytes.Count(file.Content[:i], []byte{'\n'}) + 1
			col := i - (bytes.LastIndexByte(file.Content[:i], '\n') + 1) + 1
			ctx.Emit(r.Finding(file, line, col, "Found byte-order-mark in the middle of a file"))
			return
		}
	}
}
