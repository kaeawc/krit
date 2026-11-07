package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// GradleGetterRule ports Android Lint's GradleGetter: inside `defaultConfig`, a
// property assigned from a call to getVersionCode() or getVersionName()
// resolves to the implicit getter of the DSL property instead of the script's
// own method, so the build gets the wrong value. Lint asks for another name,
// such as computeVersionCode(). It applies to Groovy and Kotlin build scripts.
type GradleGetterRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the scan is line based.
func (r *GradleGetterRule) Confidence() float64 { return api.ConfidenceMedium }

const gradleGetterMessage = "Bad method name: pick a unique method name which does not conflict with the implicit " +
	"getters for the defaultConfig properties. For example, try using the prefix compute- instead of get-."

// gradleGetterAssignRe matches `property getVersionCode()`, `property = getVersionName()` and
// `property(getVersionCode())`.
var gradleGetterAssignRe = regexp.MustCompile(`^\s*[A-Za-z_]\w*\s*(?:=\s*|\(\s*)?(?:getVersionCode|getVersionName)\s*(?:\(\s*\))?\s*\)?\s*;?\s*$`)

func (r *GradleGetterRule) check(ctx *api.Context) {
	var blocks []string
	for i, raw := range strings.Split(ctx.GradleContent, "\n") {
		line := stripGradleLineComment(raw)
		if len(blocks) > 0 && blocks[len(blocks)-1] == "defaultConfig" && gradleGetterAssignRe.MatchString(line) {
			ctx.Emit(baseFinding(ctx.GradlePath, i+1, r.BaseRule, gradleGetterMessage))
		}
		blocks = trackGradleBlocks(blocks, line)
	}
}

// trackGradleBlocks updates the stack of open block names for the braces on one comment-free line. A block's name
// is the identifier before its `{`; anonymous braces push an empty name.
func trackGradleBlocks(blocks []string, line string) []string {
	var inQuote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuote != 0 {
			switch c {
			case '\\':
				i++
			case inQuote:
				inQuote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			inQuote = c
		case '{':
			blocks = append(blocks, gradleBlockName(line[:i]))
		case '}':
			if len(blocks) > 0 {
				blocks = blocks[:len(blocks)-1]
			}
		}
	}
	return blocks
}

// gradleBlockName returns the last identifier of text, the name of the block opened right after it.
func gradleBlockName(text string) string {
	text = strings.TrimRight(text, " \t(")
	end := len(text)
	start := end
	for start > 0 {
		c := text[start-1]
		if c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			start--
			continue
		}
		break
	}
	return text[start:end]
}
