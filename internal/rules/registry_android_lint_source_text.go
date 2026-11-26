package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// wrongCommentTypeNodeTypes are the Kotlin and Java declarations Lint visits as UClass, UMethod and UField.
var wrongCommentTypeNodeTypes = []string{
	"class_declaration", "object_declaration", "companion_object", "function_declaration", "property_declaration",
	"class_parameter", "secondary_constructor", "primary_constructor", "getter", "setter", "enum_entry",
	"interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration",
	"method_declaration", "constructor_declaration", "field_declaration", "constant_declaration", "enum_constant",
}

// registerAndroidLintSourceTextChecks registers the Android Lint 32.4.1 TrimLambda, TextConcatSpace and
// WrongCommentType checks.
func registerAndroidLintSourceTextChecks() {
	trim := &TrimLambdaRule{AndroidRule: lintAndroidRule("TrimLambda", "Unnecessary lambda with `trim()`",
		ALCCorrectness, ALSInformational, 6)}
	api.Register(&api.Rule{
		ID: trim.RuleName, Category: trim.RuleSetName, Description: trim.Description(), Sev: api.Severity(trim.Sev),
		NodeTypes: []string{"call_expression"}, Languages: []scanner.Language{scanner.LangKotlin},
		Confidence: trim.Confidence(), Implementation: trim, Fix: api.FixSemantic, OptionalResolver: true,
		FileGate: lintFileMentionsGate(trim), Check: lintSkipTestSources(trim.check),
	})

	concat := &TextConcatSpaceRule{AndroidRule: lintAndroidRule("TextConcatSpace", "Missing space in text concatenation?",
		ALCCorrectness, ALSWarning, 6)}
	registerLintRule(concat.AndroidRule, concat, concat.Confidence(), concat.check,
		[]string{"additive_expression", "binary_expression"}, 0, nil)

	comment := &WrongCommentTypeRule{AndroidRule: lintAndroidRule("WrongCommentType", "Wrong Comment Type",
		ALCCorrectness, ALSWarning, 9)}
	api.Register(&api.Rule{
		ID: comment.RuleName, Category: comment.RuleSetName, Description: comment.Description(), Sev: api.Severity(comment.Sev),
		NodeTypes: wrongCommentTypeNodeTypes, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: comment.Confidence(), Implementation: comment, Fix: api.FixCosmetic,
		Check: lintSkipTestSources(comment.check),
	})
}

// lintSkipTestSources wraps a Lint port's check so it skips test sources, as Lint does unless checkTestSources is on.
func lintSkipTestSources(check func(*api.Context)) func(*api.Context) {
	return func(ctx *api.Context) {
		if ctx.File != nil && scanner.IsTestFile(ctx.File.Path) {
			return
		}
		check(ctx)
	}
}
