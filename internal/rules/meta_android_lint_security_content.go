// Descriptor metadata for the Android Lint Security ports in android_lint_security_javascript.go and
// android_lint_security_filename.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *JavascriptInterfaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "JavascriptInterface", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UnsanitizedFilenameFromContentProviderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsanitizedFilenameFromContentProvider", RuleSet: "android-lint", DefaultActive: true}
}
