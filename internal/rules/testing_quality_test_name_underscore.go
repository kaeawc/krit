package rules

// Testing-quality rule: TestNameContainsUnderscore. Flags test
// function names that contain underscores when a backtick-quoted
// human-readable name would convey intent better, and offers a
// cosmetic autofix that rewrites the name as `with spaces`.

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type TestNameContainsUnderscoreRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestNameContainsUnderscoreRule) Confidence() float64 { return api.ConfidenceMediumLow }

func registerTestingQualityTestNameContainsUnderscore() {
	r := &TestNameContainsUnderscoreRule{
		BaseRule: BaseRule{RuleName: "TestNameContainsUnderscore", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects test function names using underscores where backtick-quoted names are preferred."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMediumLow, Fix: api.FixCosmetic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityIsTestFunction(file, idx) {
				return
			}
			name := testingQualityFunctionName(file, idx)
			if name == "" || !strings.Contains(name, "_") {
				return
			}
			if strings.HasPrefix(name, "`") {
				return
			}
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				"Test name uses underscores; consider backtick-quoted names.")
			if fix := testNameUnderscoreFix(file, idx, name); fix != nil {
				f.Fix = fix
			}
			ctx.Emit(f)
		},
	})
}

// testNameUnderscoreFix returns a byte-mode Fix that replaces the
// function's simple_identifier child with the same name rewritten as a
// backtick-quoted identifier with underscores swapped for spaces. Returns
// nil when the identifier byte range cannot be located.
func testNameUnderscoreFix(file *scanner.File, fnIdx uint32, name string) *scanner.Fix {
	for child := file.FlatFirstChild(fnIdx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier", "identifier":
			replacement := "`" + strings.ReplaceAll(name, "_", " ") + "`"
			return &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(child)),
				EndByte:     int(file.FlatEndByte(child)),
				Replacement: replacement,
			}
		}
	}
	return nil
}
