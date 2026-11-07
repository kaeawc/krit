package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidLintPortRules registers checks ported from Android Lint's own detectors, in Lint's semantics.
func registerAndroidLintPortRules() {
	registerUnspecifiedImmutableFlag()
	registerWeekBasedYear()
	registerUsableSpace()
	registerWorldFilePermissions()
	registerX509TrustManager()
	registerWakelockTimeout()
	registerSuspiciousImport()
	registerPropertyFileRules()
	registerManifestPermissionRules()
	registerManifestApplicationRules()
	registerManifestTvRules()
	registerTypographyRules()
	registerMiscLintChecks()
	registerLayoutLintChecks()
	registerManifestMiscChecks()
	registerAttributeChecks()
	registerNetworkSecurityChecks()
	registerVectorChecks()
	registerReceiverFlag()
	registerSecurityCallChecks()
	registerMissingInflatedId()
}

// lintRulesScanningTests lists the ports that also analyze test sources. Lint skips them by default for every detector
// (checkTestSources is off), so the set is empty.
var lintRulesScanningTests = map[string]bool{}

// registerLintCallRule registers a Lint port that fires on Kotlin and Java calls.
func registerLintCallRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context)) {
	registerLintRule(rule, impl, confidence, check, lintCallNodeTypes, 0, nil)
}

// registerLintRule registers a Lint port over Kotlin and Java nodes of the given types.
func registerLintRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context), nodeTypes []string,
	needs api.Capabilities, javaFacts *api.JavaFactProfile) {
	// Lint does not analyze test sources unless checkTestSources is on.
	inner := check
	if !lintRulesScanningTests[rule.RuleName] {
		check = func(ctx *api.Context) {
			if ctx.File != nil && scanner.IsTestFile(ctx.File.Path) {
				return
			}
			inner(ctx)
		}
	}
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		NodeTypes: nodeTypes, Needs: needs, JavaFacts: javaFacts,
		Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: confidence, Implementation: impl, Check: check,
		NeedsLibraryFacts: true,
	})
}

func registerUnspecifiedImmutableFlag() {
	r := &UnspecifiedImmutableFlagRule{AndroidRule: lintAndroidRule("UnspecifiedImmutableFlag",
		"Missing `PendingIntent` mutability flag", ALCSecurity, ALSFatal, 5)}
	registerLintCallRule(r.AndroidRule, r, r.Confidence(), r.check)
}

func registerWeekBasedYear() {
	r := &WeekBasedYearRule{AndroidRule: lintAndroidRule("WeekBasedYear", "Week Based Year", ALCI18N, ALSWarning, 6)}
	registerLintCallRule(r.AndroidRule, r, r.Confidence(), r.check)
}

func registerUsableSpace() {
	r := &UsableSpaceRule{AndroidRule: lintAndroidRule("UsableSpace", "Using getUsableSpace()", ALCPerformance, ALSWarning, 3)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check, []string{"call_expression", "method_invocation", "navigation_expression"},
		api.NeedsTypeInfo|api.NeedsOracleCallTargets, &api.JavaFactProfile{ReceiverTypesForCallees: []string{"getUsableSpace"}})
}

func registerWorldFilePermissions() {
	needs := api.NeedsTypeInfo | api.NeedsOracleCallTargets
	readable := &SetWorldReadableRule{AndroidRule: lintAndroidRule("SetWorldReadable",
		"`File.setReadable()` used to make file world-readable", ALCSecurity, ALSWarning, 6)}
	registerLintRule(readable.AndroidRule, readable, readable.Confidence(), readable.check, lintCallNodeTypes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"setReadable"}})
	writable := &SetWorldWritableRule{AndroidRule: lintAndroidRule("SetWorldWritable",
		"`File.setWritable()` used to make file world-writable", ALCSecurity, ALSWarning, 6)}
	registerLintRule(writable.AndroidRule, writable, writable.Confidence(), writable.check, lintCallNodeTypes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"setWritable"}})
}

func registerX509TrustManager() {
	custom := &CustomX509TrustManagerRule{AndroidRule: lintAndroidRule("CustomX509TrustManager",
		"Implements custom TLS trust manager", ALCSecurity, ALSWarning, 5)}
	registerLintRule(custom.AndroidRule, custom, custom.Confidence(), custom.check, trustManagerClassNodes, 0, nil)
	trustAll := &TrustAllX509TrustManagerRule{AndroidRule: lintAndroidRule("TrustAllX509TrustManager",
		"Insecure TLS/SSL trust manager", ALCSecurity, ALSWarning, 6)}
	registerLintRule(trustAll.AndroidRule, trustAll, trustAll.Confidence(), trustAll.check, trustManagerClassNodes, 0, nil)
}

func registerWakelockTimeout() {
	r := &WakelockTimeoutRule{AndroidRule: lintAndroidRule("WakelockTimeout", "Using wakeLock without timeout", ALCPerformance, ALSWarning, 9)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check, lintCallNodeTypes,
		api.NeedsTypeInfo|api.NeedsOracleCallTargets, &api.JavaFactProfile{ReceiverTypesForCallees: []string{"acquire"}})
}

func registerSuspiciousImport() {
	r := &SuspiciousImportRule{AndroidRule: lintAndroidRule("SuspiciousImport", "'import android.R' statement", ALCCorrectness, ALSWarning, 9)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check, []string{"import_header", "import_declaration"}, 0, nil)
}

func registerPropertyFileRules() {
	http := &UsingHTTPRule{AndroidRule: lintAndroidRule("UsingHttp", "Using HTTP instead of HTTPS", ALCSecurity, ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: http.RuleName, Category: http.RuleSetName, Description: http.Description(), Sev: api.Severity(http.Sev),
		Needs: api.NeedsModuleIndex, Confidence: http.Confidence(), Implementation: http, Check: http.check,
	})
	password := &ProxyPasswordRule{AndroidRule: lintAndroidRule("ProxyPassword", "Proxy Password in Cleartext", ALCSecurity, ALSWarning, 2)}
	api.Register(&api.Rule{
		ID: password.RuleName, Category: password.RuleSetName, Description: password.Description(), Sev: api.Severity(password.Sev),
		Needs: api.NeedsModuleIndex, Confidence: password.Confidence(), Implementation: password, Check: password.check,
	})
}

func registerManifestPermissionRules() {
	exact := &ExactAlarmRule{AndroidRule: lintAndroidRule("ExactAlarm", "Invalid Usage of Exact Alarms", ALCCorrectness, ALSError, 6)}
	registerLintManifestRule(exact.AndroidRule, exact, exact.Confidence(), exact.check)
	photo := &SelectedPhotoAccessRule{AndroidRule: lintAndroidRule("SelectedPhotoAccess",
		"Behavior change when requesting photo library access", ALCCorrectness, ALSWarning, 5)}
	registerLintManifestRule(photo.AndroidRule, photo, photo.Confidence(), photo.check)
}

func registerManifestApplicationRules() {
	backup := &DataExtractionRulesRule{AndroidRule: lintAndroidRule("DataExtractionRules",
		"Missing data extraction rules", ALCSecurity, ALSWarning, 3)}
	registerLintManifestRule(backup.AndroidRule, backup, backup.Confidence(), backup.check)
	label := &RedundantLabelRule{AndroidRule: lintAndroidRule("RedundantLabel", "Redundant label on activity", ALCCorrectness, ALSWarning, 5)}
	registerLintManifestRule(label.AndroidRule, label, label.Confidence(), label.check)
}

func registerManifestTvRules() {
	banner := &MissingTvBannerRule{AndroidRule: lintAndroidRule("MissingTvBanner", "TV Missing Banner", ALCCorrectness, ALSError, 7)}
	registerLintManifestRule(banner.AndroidRule, banner, banner.Confidence(), banner.check)
	hardware := &UnsupportedTvHardwareRule{AndroidRule: lintAndroidRule("UnsupportedTvHardware",
		"Unsupported TV Hardware Feature", ALCCorrectness, ALSError, 6)}
	registerLintManifestRule(hardware.AndroidRule, hardware, hardware.Confidence(), hardware.check)
	touch := &ImpliedTouchscreenHardwareRule{AndroidRule: lintAndroidRule("ImpliedTouchscreenHardware",
		"Touchscreen not optional", ALCCorrectness, ALSError, 6)}
	registerLintManifestRule(touch.AndroidRule, touch, touch.Confidence(), touch.check)
}

func registerTypographyRules() {
	mk := func(id, brief string, prio int) AndroidRule {
		r := lintAndroidRule(id, brief, ALCUsability, ALSWarning, prio)
		return r
	}
	dashes := &TypographyDashesRule{AndroidRule: mk("TypographyDashes", "Hyphen can be replaced with dash", 5)}
	registerLintResourceRule(dashes.AndroidRule, dashes, dashes.Confidence(), dashes.check)
	quotes := &TypographyQuotesRule{AndroidRule: mk("TypographyQuotes",
		"Straight quotes can be replaced with curvy quotes, and apostrophes with typographic apostrophes.", 5)}
	registerLintResourceRule(quotes.AndroidRule, quotes, quotes.Confidence(), quotes.check)
	fractions := &TypographyFractionsRule{AndroidRule: mk("TypographyFractions", "Fraction string can be replaced with fraction character", 5)}
	registerLintResourceRule(fractions.AndroidRule, fractions, fractions.Confidence(), fractions.check)
	ellipsis := &TypographyEllipsisRule{AndroidRule: mk("TypographyEllipsis", "Ellipsis string can be replaced with ellipsis character", 5)}
	registerLintResourceRule(ellipsis.AndroidRule, ellipsis, ellipsis.Confidence(), ellipsis.check)
	other := &TypographyOtherRule{AndroidRule: mk("TypographyOther", "Other typographical problems", 3)}
	registerLintResourceRule(other.AndroidRule, other, other.Confidence(), other.check)
}

func registerMiscLintChecks() {
	sig := &SignatureOrSystemPermissionsRule{AndroidRule: lintAndroidRule("SignatureOrSystemPermissions",
		"Declaring signatureOrSystem permissions", ALCSecurity, ALSWarning, 5)}
	registerLintManifestRule(sig.AndroidRule, sig, sig.Confidence(), sig.check)
	sampling := &HighSamplingRateRule{AndroidRule: lintAndroidRule("HighSamplingRate", "High sensor sampling rate", ALCCorrectness, ALSWarning, 5)}
	registerLintManifestRule(sampling.AndroidRule, sampling, sampling.Confidence(), sampling.check)
	keyboard := &KeyboardInaccessibleWidgetRule{AndroidRule: lintAndroidRule("KeyboardInaccessibleWidget",
		"Keyboard inaccessible widget", ALCAccessibility, ALSWarning, 3)}
	api.Register(&api.Rule{
		ID: keyboard.RuleName, Category: keyboard.RuleSetName, Description: keyboard.Description(), Sev: api.Severity(keyboard.Sev),
		Needs: api.NeedsResources, AndroidDeps: uint32(keyboard.AndroidDependencies()), Confidence: keyboard.Confidence(), Implementation: keyboard,
		NeedsLibraryFacts: true, Check: keyboard.check,
	})
}

func registerLayoutLintChecks() {
	bar := &BottomAppBarRule{AndroidRule: lintAndroidRule("BottomAppBar", "BottomAppBar Problems", ALCCorrectness, ALSError, 9)}
	registerLintResourceRule(bar.AndroidRule, bar, bar.Confidence(), bar.check)
	ime := &InvalidImeActionIDRule{AndroidRule: lintAndroidRule("InvalidImeActionId", "Invalid imeActionId declaration", ALCCorrectness, ALSError, 6)}
	registerLintResourceRule(ime.AndroidRule, ime, ime.Confidence(), ime.check)
	title := &MenuTitleRule{AndroidRule: lintAndroidRule("MenuTitle", "Missing menu title", ALCUsability, ALSError, 5)}
	registerLintResourceRule(title.AndroidRule, title, title.Confidence(), title.check)
}

func registerManifestMiscChecks() {
	c2dm := &UsingC2DMRule{AndroidRule: lintAndroidRule("UsingC2DM", "Using C2DM", ALCSecurity, ALSError, 8)}
	registerLintManifestRule(c2dm.AndroidRule, c2dm, c2dm.Confidence(), c2dm.check)
	perm := &InvalidPermissionRule{AndroidRule: lintAndroidRule("InvalidPermission", "Invalid Permission Attribute", ALCSecurity, ALSError, 5)}
	registerLintManifestRule(perm.AndroidRule, perm, perm.Confidence(), perm.check)
	location := &CoarseFineLocationRule{AndroidRule: lintAndroidRule("CoarseFineLocation",
		"Cannot use `ACCESS_FINE_LOCATION` without `ACCESS_COARSE_LOCATION`", ALCCorrectness, ALSError, 5)}
	registerLintManifestRule(location.AndroidRule, location, location.Confidence(), location.check)
	wifi := &LeanbackUsesWifiRule{AndroidRule: lintAndroidRule("LeanbackUsesWifi", "Using android.hardware.wifi on TV", ALCCorrectness, ALSWarning, 5)}
	registerLintManifestRule(wifi.AndroidRule, wifi, wifi.Confidence(), wifi.check)
	feature := &UnnecessaryRequiredFeatureRule{AndroidRule: lintAndroidRule("UnnecessaryRequiredFeature",
		"Unnecessary required feature", ALCUsability, ALSWarning, 2)}
	registerLintManifestRule(feature.AndroidRule, feature, feature.Confidence(), feature.check)
}

func registerAttributeChecks() {
	alias := &InvalidManifestAttributeRule{AndroidRule: lintAndroidRule("InvalidManifestAttribute", "Invalid manifest attribute", ALCCorrectness, ALSWarning, 5)}
	registerLintManifestRule(alias.AndroidRule, alias, alias.Confidence(), alias.check)
	ellipsize := &EllipsizeMaxLinesRule{AndroidRule: lintAndroidRule("EllipsizeMaxLines", "Combining Ellipsize and Maxlines", ALCCorrectness, ALSError, 4)}
	api.Register(&api.Rule{
		ID: ellipsize.RuleName, Category: ellipsize.RuleSetName, Description: ellipsize.Description(), Sev: api.Severity(ellipsize.Sev),
		Needs: api.NeedsResources, AndroidDeps: uint32(ellipsize.AndroidDependencies()), Confidence: ellipsize.Confidence(),
		Implementation: ellipsize, NeedsLibraryFacts: true, Check: ellipsize.check,
	})
}

func registerNetworkSecurityChecks() {
	nsc := &NetworkSecurityConfigRule{AndroidRule: lintAndroidRule("NetworkSecurityConfig", "Valid Network Security Config File", ALCCorrectness, ALSFatal, 5)}
	registerLintManifestRule(nsc.AndroidRule, nsc, nsc.Confidence(), nsc.check)
	expiry := &PinSetExpiryRule{AndroidRule: lintAndroidRule("PinSetExpiry", "Validate `<pin-set>` expiration attribute", ALCCorrectness, ALSWarning, 6)}
	registerLintManifestRule(expiry.AndroidRule, expiry, expiry.Confidence(), expiry.check)
	backup := &MissingBackupPinRule{AndroidRule: lintAndroidRule("MissingBackupPin", "Missing Backup Pin", ALCCorrectness, ALSWarning, 6)}
	registerLintManifestRule(backup.AndroidRule, backup, backup.Confidence(), backup.check)
	user := &AcceptsUserCertificatesRule{AndroidRule: lintAndroidRule("AcceptsUserCertificates", "Allowing User Certificates", ALCSecurity, ALSWarning, 5)}
	registerLintManifestRule(user.AndroidRule, user, user.Confidence(), user.check)
}

func registerVectorChecks() {
	raster := &VectorRasterRule{AndroidRule: lintAndroidRule("VectorRaster", "Vector Image Generation", ALCCorrectness, ALSWarning, 5)}
	registerLintVectorRule(raster.AndroidRule, raster, raster.Confidence(), raster.check)
	path := &VectorPathRule{AndroidRule: lintAndroidRule("VectorPath", "Long vector paths", ALCPerformance, ALSWarning, 5)}
	registerLintVectorRule(path.AndroidRule, path, path.Confidence(), path.check)
	invalid := &InvalidVectorPathRule{AndroidRule: lintAndroidRule("InvalidVectorPath", "Invalid vector paths", ALCCorrectness, ALSError, 5)}
	registerLintVectorRule(invalid.AndroidRule, invalid, invalid.Confidence(), invalid.check)
}

func registerLintVectorRule(rule AndroidRule, impl interface{ AndroidDependencies() AndroidDataDependency }, confidence float64, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		Needs: api.NeedsResources, AndroidDeps: uint32(impl.AndroidDependencies()), Confidence: confidence,
		Implementation: impl, NeedsLibraryFacts: true, Check: check,
	})
}

func registerReceiverFlag() {
	r := &UnspecifiedRegisterReceiverFlagRule{AndroidRule: lintAndroidRule("UnspecifiedRegisterReceiverFlag",
		"Missing `registerReceiver()` exported flag", ALCCorrectness, ALSError, 5)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check, lintCallNodeTypes,
		api.NeedsTypeInfo|api.NeedsOracleCallTargets, &api.JavaFactProfile{ReceiverTypesForCallees: []string{"registerReceiver"}})
}

func registerSecurityCallChecks() {
	needs := api.NeedsTypeInfo | api.NeedsOracleCallTargets
	insecure := &SSLCertificateSocketFactoryGetInsecureRule{AndroidRule: lintAndroidRule("SSLCertificateSocketFactoryGetInsecure",
		"Call to `SSLCertificateSocketFactory.getInsecure()`", ALCSecurity, ALSWarning, 6)}
	registerLintRule(insecure.AndroidRule, insecure, insecure.Confidence(), insecure.check, lintCallNodeTypes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"getInsecure"}})
	socket := &SSLCertificateSocketFactoryCreateSocketRule{AndroidRule: lintAndroidRule("SSLCertificateSocketFactoryCreateSocket",
		"Insecure call to `SSLCertificateSocketFactory.createSocket()`", ALCSecurity, ALSWarning, 6)}
	registerLintRule(socket.AndroidRule, socket, socket.Confidence(), socket.check, lintCallNodeTypes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"createSocket"}})
	provider := &DeletedProviderRule{AndroidRule: lintAndroidRule("DeletedProvider", "Using Deleted Provider", ALCSecurity, ALSError, 9)}
	registerLintCallRule(provider.AndroidRule, provider, provider.Confidence(), provider.check)
	client := &WebViewClientOnReceivedSslErrorRule{AndroidRule: lintAndroidRule("WebViewClientOnReceivedSslError",
		"Proceeds with the HTTPS connection despite SSL errors", ALCSecurity, ALSWarning, 5)}
	registerLintRule(client.AndroidRule, client, client.Confidence(), client.check, trustManagerClassNodes, 0, nil)
}

func registerMissingInflatedId() {
	r := &MissingInflatedIDRule{AndroidRule: lintAndroidRule("MissingInflatedId", "ID not found in inflated resource", ALCCorrectness, ALSError, 5)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: lintCallNodeTypes[:2], Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Needs: api.NeedsResources, AndroidDeps: uint32(AndroidDepLayout), Confidence: r.Confidence(), Implementation: r,
		Check: func(ctx *api.Context) {
			if ctx.File != nil && scanner.IsTestFile(ctx.File.Path) {
				return
			}
			r.check(ctx)
		},
	})
}
