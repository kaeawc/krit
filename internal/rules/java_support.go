package rules

import api "github.com/kaeawc/krit/internal/rules/api"

const JavaLanguageSupportKey = "java"

// JavaSupportMatrix is the source-of-truth Java source support classification.
// It covers analyzer infrastructure, ruleset defaults, and per-rule overrides.
type JavaSupportMatrix struct {
	Version         int
	ClosureCriteria []string
	Infrastructure  map[string]api.LanguageSupport
	RuleSetDefaults map[string]api.LanguageSupport
	Rules           map[string]api.LanguageSupport
}

var javaSupportReadiness = JavaSupportMatrix{
	Version: 1,
	ClosureCriteria: []string{
		"Java-only and mixed Java/Kotlin projects parse, dispatch, suppress, cache, report, and apply text fixes without requiring Kotlin files.",
		"Source-visible Java declarations and references participate in cross-file and module indexes.",
		"High-precision Java rules use source facts, the Java type profile, or javac facts instead of broad lexical lookalikes.",
		"Supported Java source rules have positive and local-lookalike coverage.",
		"Pending Java-applicable Kotlin rules explain their remaining design or coverage gap instead of being counted as full support.",
	},
	Infrastructure: map[string]api.LanguageSupport{
		"autofix":       {Status: api.LanguageSupportPartial, Reason: "Java autofix support is intentionally limited to low-risk text fixes until semantic Java fixes have dedicated safety review.", Evidence: []string{"internal/rules/fix_test.go", "cmd/krit/main_test.go"}},
		"pipeline":      {Status: api.LanguageSupportSupported, Evidence: []string{"cmd/krit/java_support_fixture_test.go", "internal/pipeline/parse_test.go", "internal/pipeline/dispatch_test.go"}},
		"semanticFacts": {Status: api.LanguageSupportSupported, Evidence: []string{"internal/javafacts/facts_test.go", "internal/javafacts/helper_test.go", "internal/rules/android_correctness_test.go"}},
		"sourceIndex":   {Status: api.LanguageSupportSupported, Evidence: []string{"internal/scanner/index_test.go", "internal/rules/deadcode_module_test.go"}},
	},
	RuleSetDefaults: map[string]api.LanguageSupport{
		"a11y":                {Status: api.LanguageSupportPending, Reason: "Compose and source accessibility rules need Java/non-Java applicability classification."},
		"accessibility":       {Status: api.LanguageSupportPending, Reason: "Java applicability has not been classified for every source rule in this ruleset."},
		"android-correctness": {Status: api.LanguageSupportPending, Reason: "Several Android correctness source rules are supported, but remaining Kotlin-only or Java-applicable rules still need per-rule classification."},
		"android-gradle":      {Status: api.LanguageSupportSupported, Evidence: []string{"Gradle rules are source-language independent."}},
		"android-icons":       {Status: api.LanguageSupportSupported, Evidence: []string{"Icon/resource rules are source-language independent."}},
		"android-lint":        {Status: api.LanguageSupportPartial, Reason: "Many Android lint compatibility checks are XML/manifest/resource-only, but source-backed checks still need Java parity classification."},
		"android-manifest":    {Status: api.LanguageSupportSupported, Evidence: []string{"Manifest rules are source-language independent."}},
		"android-resource":    {Status: api.LanguageSupportSupported, Evidence: []string{"XML/resource rules are source-language independent."}},
		"architecture":        {Status: api.LanguageSupportSupported, Evidence: []string{"Architecture graph checks are source-language independent or use the mixed source index."}},
		"comments":            {Status: api.LanguageSupportPending, Reason: "Source comment and documentation rules need Java-specific syntax review."},
		"complexity":          {Status: api.LanguageSupportPending, Reason: "Complexity rules need Java AST parity classification before being counted as supported."},
		"compose":             {Status: api.LanguageSupportNotApplicable, Reason: "Compose rules target Kotlin Compose source patterns and are not Java source rules."},
		"coroutines":          {Status: api.LanguageSupportNotApplicable, Reason: "Coroutine rules target Kotlin coroutine syntax and APIs."},
		"database":            {Status: api.LanguageSupportPartial, Reason: "High-value Java database rules are supported, but this ruleset still needs per-rule parity review."},
		"dead-code":           {Status: api.LanguageSupportPartial, Reason: "Mixed Java/Kotlin indexes participate in dead-code analysis, but full Java rule parity is still tracked separately."},
		"di-hygiene":          {Status: api.LanguageSupportPending, Reason: "DI source rules need Java annotation and class-shape coverage before being counted as supported."},
		"empty-blocks":        {Status: api.LanguageSupportPartial, Reason: "Common Java block shapes are supported; Kotlin-only file/constructor/when rules remain non-Java or need design."},
		"exceptions":          {Status: api.LanguageSupportPartial, Reason: "Method-level Java throw checks and common catch/finally rules are supported; remaining exception rules still need Java-specific call/constructor argument analysis or broader false-positive review."},
		"i18n":                {Status: api.LanguageSupportSupported, Evidence: []string{"Resource and locale project checks are source-language independent unless overridden by per-rule entries."}},
		"library":             {Status: api.LanguageSupportSupported, Evidence: []string{"Dependency and project model checks are source-language independent."}},
		"naming":              {Status: api.LanguageSupportPending, Reason: "Naming rules need Java-specific syntax and convention review."},
		"observability":       {Status: api.LanguageSupportPending, Reason: "Logging source rules need Java receiver/import coverage before being counted as supported."},
		"performance":         {Status: api.LanguageSupportPending, Reason: "Some Java performance/resource rules are supported, but remaining rules need per-rule classification."},
		"potential-bugs":      {Status: api.LanguageSupportPartial, Reason: "Basic java.lang.System/Runtime lifecycle checks and Java package declaration checks are supported; many remaining potential-bugs rules encode Kotlin syntax or type semantics and need per-rule Java classification."},
		"privacy":             {Status: api.LanguageSupportPending, Reason: "Privacy source rules need Java receiver/import coverage before being counted as supported."},
		"release-engineering": {Status: api.LanguageSupportPartial, Reason: "Literal URL checks and project-shape checks have Java support; remaining source rules need Java parity review."},
		"resource-cost":       {Status: api.LanguageSupportPartial, Reason: "High-value Java resource-cost rules are supported, but remaining rules need per-rule parity review."},
		"security":            {Status: api.LanguageSupportPartial, Reason: "High-value Android security and literal credential Java rules are supported; remaining source rules need Java parity review."},
		"style":               {Status: api.LanguageSupportPartial, Reason: "Java comment/import style rules, line-based formatting checks, and decimal numeric literal readability are supported; most remaining style rules encode Kotlin syntax and require explicit per-rule review."},
		"supply-chain":        {Status: api.LanguageSupportSupported, Evidence: []string{"Supply-chain checks are project and dependency based rather than Kotlin-source specific."}},
		"testing":             {Status: api.LanguageSupportPending, Reason: "Test-source rules need Java test-framework and syntax coverage before being counted as supported."},
		"testing-quality":     {Status: api.LanguageSupportPending, Reason: "Test-source rules need Java test-framework and syntax coverage before being counted as supported."},
	},
	Rules: map[string]api.LanguageSupport{
		"AddJavascriptInterface":               {Status: api.LanguageSupportSupported, Fixtures: []string{"cmd/krit/java_support_fixture_test.go"}},
		"AllowAllHostnameVerifier":             {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"BroadcastReceiverExportedFlagMissing": {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"BufferedReadWithoutBuffer":            {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"CheckResult":                          {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"CollectInOnCreateWithoutLifecycle":    {Status: api.LanguageSupportNotApplicable, Reason: "Targets Kotlin coroutines `Flow.collect` calls in Android lifecycle callbacks; Java lifecycle code doesn't use the Kotlin Flow API."},
		"CommitPrefEdits":                      {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"CommitTransaction":                    {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"ComposeRememberWithoutKey":            {Status: api.LanguageSupportNotApplicable, Reason: "Targets Kotlin Compose `remember { … }` blocks; Compose is Kotlin-only."},
		"CursorLoopWithColumnIndexInLoop":      {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"DatabaseInstanceRecreated":            {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"DatabaseQueryOnMainThread":            {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/database_test.go"}},
		"DefaultLocale":                        {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"DisableCertificatePinning":            {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"EmptyCatchBlock":                      {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyClassBlock":                      {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyDoWhileBlock":                    {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyElseBlock":                       {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyFinallyBlock":                    {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyForBlock":                        {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyFunctionBlock":                   {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyIfBlock":                         {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyTryBlock":                        {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"EmptyWhileBlock":                      {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/emptyblocks_test.go"}},
		"ExceptionRaisedInUnexpectedLocation":  {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"ExitOutsideMain":                      {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/potentialbugs_lifecycle_test.go", "internal/javafacts/source_test.go"}},
		"ExplicitGarbageCollectionCall":        {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/potentialbugs_lifecycle_test.go", "internal/javafacts/source_test.go"}},
		"ForbiddenComment":                     {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_forbidden_test.go"}},
		"ForbiddenImport":                      {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_forbidden_test.go"}},
		"GrantAllUris":                         {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"GsonPolymorphicFromJson":              {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/security_test.go"}},
		"HandlerLeak":                          {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"HardcodedBearerToken":                 {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/hardcoded_bearer_token_test.go"}},
		"HardcodedGcpServiceAccount":           {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/hardcoded_gcp_service_account_test.go"}},
		"HardcodedHttpUrl":                     {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"HardcodedLocalhostUrl":                {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/release_engineering_test.go"}},
		"HardcodedSecretKey":                   {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"HttpClientNotReused":                  {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"ImplicitPendingIntent":                {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"InjectDispatcher":                     {Status: api.LanguageSupportNotApplicable, Reason: "Targets Kotlin coroutines `Dispatchers.IO/Default/Unconfined` references; Java callers of the coroutines runtime are out of scope for the heuristic."},
		"InsecureTrustManager":                 {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"InstanceOfCheckForException":          {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"JavaObjectInputStream":                {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/security_test.go"}},
		"JacksonDefaultTyping":                 {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/security_test.go"}},
		"JdbcStatementExecute":                 {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/security_test.go"}},
		"LogPii":                               {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/security_test.go"}},
		"MaxChainedCallsOnSameLine":            {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_format_test.go"}},
		"MaxLineLength":                        {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_format_test.go"}},
		"MissingPackageDeclaration":            {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/potentialbugs_lifecycle_test.go"}},
		"NewLineAtEndOfFile":                   {Status: api.LanguageSupportSupported, Fixtures: []string{"cmd/krit/main_test.go"}},
		"NoTabs":                               {Status: api.LanguageSupportSupported, Fixtures: []string{"tests/fixtures/fixable/per-rule/NoTabs.java"}},
		"OkHttpClientCreatedPerCall":           {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"OkHttpDisableSslValidation":           {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"OptInMarkerExposedPublicly":           {Status: api.LanguageSupportNotApplicable, Reason: "@OptIn is a Kotlin-only construct; Java has no equivalent annotation."},
		"ProcessBuilderShellArg":               {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/security_test.go"}},
		"PrngFromSystemTime":                   {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"RecyclerAdapterStableIdsDefault":      {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"RecyclerAdapterWithoutDiffUtil":       {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"RethrowCaughtException":               {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"RetrofitCreateInHotPath":              {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/resource_cost_test.go"}},
		"ReturnFromFinally":                    {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"RuntimeExecUnsafeShape":               {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/security_test.go"}},
		"RsaNoPadding":                         {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"SecureRandom":                         {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"SetJavaScriptEnabled":                 {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_base_test.go"}},
		"SetTextI18n":                          {Status: api.LanguageSupportPending, Reason: "Kotlin call_expression coverage only; Java method_invocation parity for TextView/Button receivers still needs implementation."},
		"SimpleDateFormat":                     {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_test.go"}},
		"SpacingAfterPackageAndImports":        {Status: api.LanguageSupportSupported, Fixtures: []string{"tests/fixtures/fixable/per-rule/SpacingAfterPackageAndImports.java"}},
		"StaticIv":                             {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"StartActivityWithUntrustedIntent":     {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"SqlInjectionRawQuery":                 {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/security_test.go"}},
		"SqliteCursorWithoutClose":             {Status: api.LanguageSupportNotApplicable, Reason: "Rule dispatches on Kotlin property_declaration and matches the val/var initializer chain; Java local variables are syntactically and structurally different."},
		"ThrowingExceptionFromFinally":         {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"ThrowingExceptionInMain":              {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"ThrowingNewInstanceOfSameException":   {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/exceptions_test.go"}},
		"TrailingWhitespace":                   {Status: api.LanguageSupportSupported, Fixtures: []string{"tests/fixtures/fixable/per-rule/TrailingWhitespace.java"}},
		"UnderscoresInNumericLiterals":         {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_format_test.go"}},
		"UnprotectedDynamicReceiver":           {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"UseValueOf":                           {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"WebViewDebuggingEnabled":              {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_webview_debugging_enabled_test.go"}},
		"WeakKeySize":                          {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"WeakMacAlgorithm":                     {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"WeakMessageDigest":                    {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_security_test.go"}},
		"WildcardImport":                       {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/style_forbidden_test.go"}},
		"WorldReadableFiles":                   {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"WorldWriteableFiles":                  {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_source_test.go"}},
		"WrongCall":                            {Status: api.LanguageSupportPending, Reason: "Kotlin call_expression coverage only; Java method_invocation parity for View subclass receivers still needs implementation."},
		"XmlExternalEntity":                    {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/security_test.go"}},
		"WrongViewCast":                        {Status: api.LanguageSupportSupported, Fixtures: []string{"internal/rules/android_correctness_checks_test.go"}},
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
func JavaSupportForRule(r *api.Rule) (api.LanguageSupport, bool) {
	if r == nil {
		return api.LanguageSupport{}, false
	}
	if support, ok := javaSupportReadiness.Rules[r.ID]; ok {
		return support, true
	}
	if support, ok := javaSupportReadiness.RuleSetDefaults[r.Category]; ok {
		return support, true
	}
	return api.LanguageSupport{}, false
}

func cloneLanguageSupportMap(in map[string]api.LanguageSupport) map[string]api.LanguageSupport {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]api.LanguageSupport, len(in))
	for key, support := range in {
		support.Evidence = append([]string(nil), support.Evidence...)
		support.Fixtures = append([]string(nil), support.Fixtures...)
		out[key] = support
	}
	return out
}
