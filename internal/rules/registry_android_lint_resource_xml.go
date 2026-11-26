package rules

// registerAndroidLintResourceXMLChecks registers the Android Lint 32.4.1 StringEscaping, InvalidNavigation,
// ValidRestrictions and RelativeClassResolution checks.
func registerAndroidLintResourceXMLChecks() {
	escaping := &StringEscapingRule{AndroidRule: lintAndroidRule("StringEscaping", "Invalid string escapes", ALCMessages, ALSError, 9)}
	registerLintResourceRule(escaping.AndroidRule, escaping, escaping.Confidence(), escaping.check)

	navigation := &InvalidNavigationRule{AndroidRule: lintAndroidRule("InvalidNavigation", "No start destination specified",
		ALCCorrectness, ALSWarning, 3)}
	registerLintResourceRule(navigation.AndroidRule, navigation, navigation.Confidence(), navigation.check)

	restrictions := &ValidRestrictionsRule{AndroidRule: lintAndroidRule("ValidRestrictions", "Invalid Restrictions Descriptor",
		ALCCorrectness, ALSFatal, 5)}
	registerLintResourceRule(restrictions.AndroidRule, restrictions, restrictions.Confidence(), restrictions.check)

	relative := &RelativeClassResolutionRule{AndroidRule: lintAndroidRule("RelativeClassResolution",
		"Relative class names resolve to applicationId, not namespace", ALCCorrectness, ALSFatal, 8)}
	registerLintResourceRule(relative.AndroidRule, relative, relative.Confidence(), relative.check)
}
