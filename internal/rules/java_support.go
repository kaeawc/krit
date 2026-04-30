package rules

import v2 "github.com/kaeawc/krit/internal/rules/v2"

const JavaLanguageSupportKey = "java"

// JavaSupportMatrix is the source-of-truth Java source support classification.
// It covers analyzer infrastructure, ruleset defaults, and per-rule overrides.
type JavaSupportMatrix struct {
	Version         int
	ClosureCriteria []string
	Infrastructure  map[string]v2.LanguageSupport
	RuleSetDefaults map[string]v2.LanguageSupport
	Rules           map[string]v2.LanguageSupport
}

var javaSupportReadiness = JavaSupportMatrix{
	Version: 1,
	ClosureCriteria: []string{
		"Java-only and mixed Java/Kotlin projects parse, dispatch, suppress, cache, report, and apply text fixes without requiring Kotlin files.",
		"Source-visible Java declarations and references participate in cross-file and module indexes.",
		"High-precision Java rules use source facts, the Java type profile, or javac facts instead of broad lexical lookalikes.",
		"Supported Java source rules have positive and local-lookalike coverage.",
		"Pending Java-applicable Kotlin rules remain linked to an open tracking issue instead of being counted as full support.",
	},
	Infrastructure: map[string]v2.LanguageSupport{
		"autofix":       v2.LanguageSupport{Status: v2.LanguageSupportPartial, Issue: 705, Evidence: []string{"internal/rules/fix_test.go", "cmd/krit/main_test.go"}},
		"pipeline":      v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"cmd/krit/java_support_fixture_test.go", "internal/pipeline/parse_test.go", "internal/pipeline/dispatch_test.go"}},
		"semanticFacts": v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"internal/javafacts/facts_test.go", "internal/javafacts/helper_test.go", "internal/rules/android_correctness_test.go"}},
		"sourceIndex":   v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"internal/scanner/index_test.go", "internal/rules/deadcode_module_test.go"}},
	},
	RuleSetDefaults: map[string]v2.LanguageSupport{
		"a11y":                v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Compose and source accessibility rules need Java/non-Java applicability classification.", Issue: 700},
		"accessibility":       v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Java applicability has not been classified for every source rule in this ruleset.", Issue: 700},
		"android-correctness": v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Several Android correctness source rules are supported, but remaining Kotlin-only or Java-applicable rules still need per-rule classification.", Issue: 700},
		"android-gradle":      v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"Gradle rules are source-language independent."}},
		"android-icons":       v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"Icon/resource rules are source-language independent."}},
		"android-lint":        v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "Many Android lint compatibility checks are XML/manifest/resource-only, but source-backed checks still need Java parity classification.", Issue: 700},
		"android-manifest":    v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"Manifest rules are source-language independent."}},
		"android-resource":    v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"XML/resource rules are source-language independent."}},
		"architecture":        v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"Architecture graph checks are source-language independent or use the mixed source index."}},
		"comments":            v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Source comment and documentation rules need Java-specific syntax review.", Issue: 700},
		"complexity":          v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Complexity rules need Java AST parity classification before being counted as supported.", Issue: 700},
		"compose":             v2.LanguageSupport{Status: v2.LanguageSupportNotApplicable, Reason: "Compose rules target Kotlin Compose source patterns and are not Java source rules."},
		"coroutines":          v2.LanguageSupport{Status: v2.LanguageSupportNotApplicable, Reason: "Coroutine rules target Kotlin coroutine syntax and APIs."},
		"database":            v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "High-value Java database rules are supported, but this ruleset still needs per-rule parity review.", Issue: 700},
		"dead-code":           v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "Mixed Java/Kotlin indexes participate in dead-code analysis, but full Java rule parity is still tracked separately.", Issue: 700},
		"di-hygiene":          v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "DI source rules need Java annotation and class-shape coverage before being counted as supported.", Issue: 700},
		"empty-blocks":        v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "Common Java block shapes are supported; Kotlin-only file/constructor/when rules remain non-Java or need design.", Issue: 700},
		"exceptions":          v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "Method-level Java throw checks and common catch/finally rules are supported; remaining exception rules still need Java-specific call/constructor argument analysis or broader false-positive review.", Issue: 700},
		"i18n":                v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"Resource and locale project checks are source-language independent unless overridden by per-rule entries."}},
		"library":             v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"Dependency and project model checks are source-language independent."}},
		"naming":              v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Naming rules need Java-specific syntax and convention review.", Issue: 700},
		"observability":       v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Logging source rules need Java receiver/import coverage before being counted as supported.", Issue: 700},
		"performance":         v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Some Java performance/resource rules are supported, but remaining rules need per-rule classification.", Issue: 700},
		"potential-bugs":      v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "Basic java.lang.System/Runtime lifecycle checks and Java package declaration checks are supported; many remaining potential-bugs rules encode Kotlin syntax or type semantics and need per-rule Java classification.", Issue: 700},
		"privacy":             v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Privacy source rules need Java receiver/import coverage before being counted as supported.", Issue: 700},
		"release-engineering": v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "Literal URL checks and project-shape checks have Java support; remaining source rules need Java parity review.", Issue: 700},
		"resource-cost":       v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "High-value Java resource-cost rules are supported, but remaining rules need per-rule parity review.", Issue: 700},
		"security":            v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "High-value Android security and literal credential Java rules are supported; remaining source rules need Java parity review.", Issue: 700},
		"style":               v2.LanguageSupport{Status: v2.LanguageSupportPartial, Reason: "Java comment/import style rules, line-based formatting checks, and decimal numeric literal readability are supported; most remaining style rules encode Kotlin syntax and require explicit per-rule review.", Issue: 700},
		"supply-chain":        v2.LanguageSupport{Status: v2.LanguageSupportSupported, Evidence: []string{"Supply-chain checks are project and dependency based rather than Kotlin-source specific."}},
		"testing":             v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Test-source rules need Java test-framework and syntax coverage before being counted as supported.", Issue: 700},
		"testing-quality":     v2.LanguageSupport{Status: v2.LanguageSupportPending, Reason: "Test-source rules need Java test-framework and syntax coverage before being counted as supported.", Issue: 700},
	},
	Rules: map[string]v2.LanguageSupport{
		"AddJavascriptInterface":              v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"cmd/krit/java_support_fixture_test.go"}},
		"BufferedReadWithoutBuffer":           v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"CheckResult":                         v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"CommitPrefEdits":                     v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"CommitTransaction":                   v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"CursorLoopWithColumnIndexInLoop":     v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"DatabaseInstanceRecreated":           v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"DatabaseQueryOnMainThread":           v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/database_test.go"}},
		"DefaultLocale":                       v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"EmptyCatchBlock":                     v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyClassBlock":                     v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyDoWhileBlock":                   v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyElseBlock":                      v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyFinallyBlock":                   v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyForBlock":                       v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyFunctionBlock":                  v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyIfBlock":                        v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyTryBlock":                       v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyWhileBlock":                     v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"ExceptionRaisedInUnexpectedLocation": v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"ExitOutsideMain":                     v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/potentialbugs_lifecycle_test.go", "internal/javafacts/source_test.go"}},
		"ExplicitGarbageCollectionCall":       v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/potentialbugs_lifecycle_test.go", "internal/javafacts/source_test.go"}},
		"ForbiddenComment":                    v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_forbidden_test.go"}},
		"ForbiddenImport":                     v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_forbidden_test.go"}},
		"GrantAllUris":                        v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"HandlerLeak":                         v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"HardcodedBearerToken":                v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/hardcoded_bearer_token_test.go"}},
		"HardcodedGcpServiceAccount":          v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/hardcoded_gcp_service_account_test.go"}},
		"HardcodedLocalhostUrl":               v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/release_engineering_test.go"}},
		"HttpClientNotReused":                 v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"InstanceOfCheckForException":         v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"MaxChainedCallsOnSameLine":           v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_format_test.go"}},
		"MaxLineLength":                       v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_format_test.go"}},
		"MissingPackageDeclaration":           v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/potentialbugs_lifecycle_test.go"}},
		"NewLineAtEndOfFile":                  v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"cmd/krit/main_test.go"}},
		"NoTabs":                              v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"tests/fixtures/fixable/per-rule/NoTabs.java"}},
		"OkHttpClientCreatedPerCall":          v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"RecyclerAdapterStableIdsDefault":     v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"RecyclerAdapterWithoutDiffUtil":      v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"RethrowCaughtException":              v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"RetrofitCreateInHotPath":             v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"ReturnFromFinally":                   v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"SecureRandom":                        v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"SetJavaScriptEnabled":                v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_base_test.go"}},
		"SimpleDateFormat":                    v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"SpacingAfterPackageAndImports":       v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"tests/fixtures/fixable/per-rule/SpacingAfterPackageAndImports.java"}},
		"ThrowingExceptionFromFinally":        v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"ThrowingExceptionInMain":             v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"ThrowingNewInstanceOfSameException":  v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"TrailingWhitespace":                  v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"tests/fixtures/fixable/per-rule/TrailingWhitespace.java"}},
		"UnderscoresInNumericLiterals":        v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_format_test.go"}},
		"UseValueOf":                          v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"WildcardImport":                      v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_forbidden_test.go"}},
		"WorldReadableFiles":                  v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"WorldWriteableFiles":                 v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"WrongViewCast":                       v2.LanguageSupport{Status: v2.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_checks_test.go"}},
	},
}

// JavaSupportReadiness returns the checked-in Java source support matrix.
func JavaSupportReadiness() JavaSupportMatrix {
	return JavaSupportMatrix{
		Version:         javaSupportReadiness.Version,
		ClosureCriteria: append([]string(nil), javaSupportReadiness.ClosureCriteria...),
		Infrastructure:  cloneLanguageSupportMap(javaSupportReadiness.Infrastructure),
		RuleSetDefaults: cloneLanguageSupportMap(javaSupportReadiness.RuleSetDefaults),
		Rules:           cloneLanguageSupportMap(javaSupportReadiness.Rules),
	}
}

// JavaSupportForRule returns the per-rule Java support entry, falling back to
// the rule's ruleset default when there is no rule-specific override.
func JavaSupportForRule(r *v2.Rule) (v2.LanguageSupport, bool) {
	if r == nil {
		return v2.LanguageSupport{}, false
	}
	if support, ok := javaSupportReadiness.Rules[r.ID]; ok {
		return support, true
	}
	if support, ok := javaSupportReadiness.RuleSetDefaults[r.Category]; ok {
		return support, true
	}
	return v2.LanguageSupport{}, false
}

func cloneLanguageSupportMap(in map[string]v2.LanguageSupport) map[string]v2.LanguageSupport {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]v2.LanguageSupport, len(in))
	for key, support := range in {
		support.Evidence = append([]string(nil), support.Evidence...)
		support.Fixtures = append([]string(nil), support.Fixtures...)
		out[key] = support
	}
	return out
}
