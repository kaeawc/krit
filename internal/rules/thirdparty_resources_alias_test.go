package rules_test

import "testing"

// Review of #1808 (P2): krit does not read the module namespace, so the module's own R imported from a namespace
// that is not a prefix of the file's package is reported, where Lint, comparing with the namespace, does not. This
// pins the documented difference.
func TestMissingResourceImportAliasWithoutNamespace(t *testing.T) {
	const src = "package com.app.home\n\nimport slack.features.home.R\n\nfun f() = R.string.title\n"
	if got := runRuleByName(t, "MissingResourceImportAlias", src); len(got) != 1 || got[0].Line != 3 {
		t.Errorf("want the documented report at 3, got %v", got)
	}
	const prefix = "package slack.features.home.ui\n\nimport slack.features.home.R\n\nfun f() = R.string.title\n"
	if got := runRuleByName(t, "MissingResourceImportAlias", prefix); len(got) != 0 {
		t.Errorf("a namespace that is a prefix of the package is local, got %v", got)
	}
}
