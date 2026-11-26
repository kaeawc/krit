package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *TrimLambdaRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TrimLambda", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Without type information the receiver must be a string literal or a variable or parameter declared as String or CharSequence; a file that declares its own trim or isWhitespace function is skipped.",
		}}
}

func (r *TextConcatSpaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TextConcatSpace", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WrongCommentTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WrongCommentType", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Doc tags are recognized textually (a lowercase `@tag` at the start of a comment line, a `[text](url)` link in a KDoc description, `{@link}`, `{@linkplain}` or `{@inheritDoc}` in a Javadoc description) rather than with the KDoc and Javadoc parsers Lint uses.",
		}}
}
