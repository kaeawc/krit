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
	registerInvalidWakeLockTag()
	registerApplySharedPref()
	registerLintSecurityChecks()
	registerStaticFieldLeak()
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
	registerMissingInflatedID()
	registerTranslationRules()
	registerResourceCycle()
	registerWearChecks()
	registerLintGradleScriptChecks()
	registerSuspiciousIndentation()
	registerAccessibilityLintChecks()
	registerParcelBinderLintChecks()
	registerObsoleteSdkInt()
	registerSourceHygieneLintChecks()
	registerDiffUtilEquals()
	registerManifestPermissionNameChecks()
	registerManifestOrientationChecks()
	registerManifestAppLinkChecks()
	registerManifestForegroundServiceChecks()
	registerIntentLintChecks()
	registerAndroidLintSecurityPorts()
	registerAndroidLintSecurityProjectPorts()
	registerAndroidLintSecurityContentPorts()
	registerAndroidLintSecurityPendingIntentPorts()
	registerAndroidXAppCompatXMLChecks()
	registerAndroidXFragmentBuildChecks()
	registerAndroidXActivityChecks()
	registerAndroidXAppCompatCallChecks()
	registerAndroidXFragmentLifecycleChecks()
	registerPerformanceAssertChecks()
	registerPerformanceGradleChecks()
	registerPerformanceNotificationChecks()
	registerPerformanceXMLChecks()
	registerPerformanceClassChecks()
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

func registerInvalidWakeLockTag() {
	r := &InvalidWakeLockTagRule{AndroidRule: lintAndroidRule("InvalidWakeLockTag", "Invalid Wake Lock Tag", ALCCorrectness, ALSWarning, 6)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check, lintCallNodeTypes,
		api.NeedsTypeInfo|api.NeedsOracleCallTargets, &api.JavaFactProfile{ReceiverTypesForCallees: []string{"newWakeLock"}})
}

func registerApplySharedPref() {
	r := &ApplySharedPrefRule{AndroidRule: lintAndroidRule("ApplySharedPref", "Use `apply()` on `SharedPreferences`", ALCPerformance, ALSWarning, 6)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check, lintCallNodeTypes,
		api.NeedsTypeInfo|api.NeedsOracleCallTargets, &api.JavaFactProfile{ReceiverTypesForCallees: []string{"edit"}})
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

func registerMissingInflatedID() {
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

func registerStaticFieldLeak() {
	r := &StaticFieldLeakRule{AndroidRule: lintAndroidRule("StaticFieldLeak", "Static Field Leaks", ALCPerformance, ALSWarning, 6)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration", "object_declaration", "class_declaration", "object_literal"},
		Needs:     api.NeedsTypeInfo, Languages: []scanner.Language{scanner.LangKotlin},
		Confidence: r.Confidence(), Implementation: r, NeedsLibraryFacts: true,
		Check: func(ctx *api.Context) {
			if !scanner.IsTestFile(ctx.File.Path) {
				r.check(ctx)
			}
		},
	})
}

func registerLintSecurityChecks() {
	load := &UnsafeDynamicallyLoadedCodeRule{AndroidRule: lintAndroidRule("UnsafeDynamicallyLoadedCode",
		"`load` used to dynamically load code", ALCSecurity, ALSWarning, 6)}
	registerLintRule(load.AndroidRule, load, load.Confidence(), load.check, lintCallNodeTypes[:2], 0, nil)
	verifier := &BadHostnameVerifierRule{AndroidRule: lintAndroidRule("BadHostnameVerifier",
		"Insecure HostnameVerifier", ALCSecurity, ALSWarning, 6)}
	registerLintRule(verifier.AndroidRule, verifier, verifier.Confidence(), verifier.check, append(append([]string{}, trustManagerClassNodes...), "call_expression"), 0, nil)
	ids := &HardwareIDsRule{AndroidRule: lintAndroidRule("HardwareIds", "Hardware Id Usage", ALCSecurity, ALSWarning, 6)}
	registerLintRule(ids.AndroidRule, ids, ids.Confidence(), ids.check,
		[]string{"call_expression", "method_invocation", "navigation_expression", "field_access"}, 0, nil)
	provider := &DeprecatedProviderRule{AndroidRule: lintAndroidRule("DeprecatedProvider", "Using BC Provider", ALCSecurity, ALSWarning, 9)}
	registerLintRule(provider.AndroidRule, provider, provider.Confidence(), provider.check, lintCallNodeTypes[:2], 0, nil)
}

func registerWearChecks() {
	feature := &InvalidWearFeatureAttributeRule{AndroidRule: lintAndroidRule("InvalidWearFeatureAttribute",
		"Invalid attribute for Wear uses-feature", ALCCorrectness, ALSError, 6)}
	registerLintManifestRule(feature.AndroidRule, feature, feature.Confidence(), feature.check)
	standalone := &WearStandaloneAppFlagRule{AndroidRule: lintAndroidRule("WearStandaloneAppFlag",
		"Invalid or missing Wear standalone app flag", ALCCorrectness, ALSError, 6)}
	registerLintManifestRule(standalone.AndroidRule, standalone, standalone.Confidence(), standalone.check)
	recents := &WearRecentsRule{AndroidRule: lintAndroidRule("WearRecents", "Wear OS: Recents and app resume", ALCUsability, ALSWarning, 5)}
	registerLintManifestRule(recents.AndroidRule, recents, recents.Confidence(), recents.check)
	splash := &WearSplashScreenRule{AndroidRule: lintAndroidRule("WearSplashScreen", "Wear: Use `SplashScreen` library", ALCUsability, ALSWarning, 5)}
	registerLintManifestRule(splash.AndroidRule, splash, splash.Confidence(), splash.check)
	editor := &WatchFaceEditorRule{AndroidRule: lintAndroidRule("WatchFaceEditor", "Watch face editor must use launchMode=\"standard\"", ALCCorrectness, ALSWarning, 5)}
	registerLintManifestRule(editor.AndroidRule, editor, editor.Confidence(), editor.check)
	noCode := &WatchFaceFormatDeclaresHasNoCodeRule{AndroidRule: lintAndroidRule("WatchFaceFormatDeclaresHasNoCode",
		"The `hasCode` attribute should be set to `false`", ALCCorrectness, ALSError, 7)}
	registerLintManifestRule(noCode.AndroidRule, noCode, noCode.Confidence(), noCode.check)
	missing := &WatchFaceFormatMissingVersionRule{AndroidRule: lintAndroidRule("WatchFaceFormatMissingVersion",
		"The Watch Face Format version is missing", ALCCorrectness, ALSError, 7)}
	registerLintManifestRule(missing.AndroidRule, missing, missing.Confidence(), missing.check)
	invalid := &WatchFaceFormatInvalidVersionRule{AndroidRule: lintAndroidRule("WatchFaceFormatInvalidVersion",
		"The Watch Face Format version is invalid", ALCCorrectness, ALSError, 7)}
	registerLintManifestRule(invalid.AndroidRule, invalid, invalid.Confidence(), invalid.check)
	password := &WearPasswordInputRule{AndroidRule: lintAndroidRule("WearPasswordInput", "Wear: Using password input", ALCUsability, ALSError, 5)}
	registerLintResourceRule(password.AndroidRule, password, password.Confidence(), password.check)
}

func registerSuspiciousIndentation() {
	r := &SuspiciousIndentationRule{AndroidRule: lintAndroidRule("SuspiciousIndentation", "Suspicious indentation", ALCCorrectness, ALSError, 6)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check, []string{"statements", "block", "constructor_body"}, 0, nil)
}

func registerAccessibilityLintChecks() {
	needs := api.NeedsTypeInfo | api.NeedsOracleCallTargets | api.NeedsResolver
	notificationID := &NotificationID0Rule{AndroidRule: lintAndroidRule("NotificationId0", "Notification Id is 0", ALCCorrectness, ALSError, 6)}
	registerLintRule(notificationID.AndroidRule, notificationID, notificationID.Confidence(), notificationID.check, lintCallNodeTypes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"startForeground"}})

	contentDescription := &GetContentDescriptionOverrideRule{AndroidRule: lintAndroidRule("GetContentDescriptionOverride",
		"Overriding `getContentDescription()` on a View", ALCAccessibility, ALSError, 9)}
	registerLintRule(contentDescription.AndroidRule, contentDescription, contentDescription.Confidence(), contentDescription.check,
		[]string{"function_declaration", "method_declaration"}, api.NeedsResolver, nil)

	windowState := &AccessibilityWindowStateChangedEventRule{AndroidRule: lintAndroidRule("AccessibilityWindowStateChangedEvent",
		"Use of accessibility window state change events", ALCAccessibility, ALSWarning, 5)}
	registerLintRule(windowState.AndroidRule, windowState, windowState.Confidence(), windowState.check,
		[]string{"call_expression", "method_invocation", "function_declaration", "method_declaration"}, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"sendAccessibilityEvent"}})

	focus := &AccessibilityFocusRule{AndroidRule: lintAndroidRule("AccessibilityFocus", "Forcing accessibility focus", ALCAccessibility, ALSWarning, 5)}
	registerLintRule(focus.AndroidRule, focus, focus.Confidence(), focus.check, lintCallNodeTypes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"performAccessibilityAction"}})
}

func registerParcelBinderLintChecks() {
	needs := api.NeedsTypeInfo | api.NeedsOracleCallTargets | api.NeedsResolver
	callNodes := []string{"call_expression", "method_invocation"}

	parcel := &ParcelClassLoaderRule{AndroidRule: lintAndroidRule("ParcelClassLoader", "Default Parcel Class Loader", ALCCorrectness, ALSWarning, 3)}
	registerLintRule(parcel.AndroidRule, parcel, parcel.Confidence(), parcel.check, callNodes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: sortedKeys(parcelMethodNames())})

	binder := &BinderGetCallingInMainThreadRule{AndroidRule: lintAndroidRule("BinderGetCallingInMainThread",
		"Incorrect usage of getCallingUid() or getCallingPid()", ALCSecurity, ALSError, 9)}
	registerLintRule(binder.AndroidRule, binder, binder.Confidence(), binder.check, callNodes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"getCallingUid", "getCallingPid"}})

	prefs := &MutatingSharedPrefsRule{AndroidRule: lintAndroidRule("MutatingSharedPrefs", "Mutating an Immutable SharedPrefs Set", ALCCorrectness, ALSWarning, 6)}
	registerLintRule(prefs.AndroidRule, prefs, prefs.Confidence(), prefs.check, callNodes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"getStringSet"}})
}

func registerObsoleteSdkInt() {
	r := &ObsoleteSdkIntRule{AndroidRule: lintAndroidRule("ObsoleteSdkInt", "Obsolete SDK_INT Version Check", ALCPerformance, ALSWarning, 6)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check,
		[]string{"comparison_expression", "equality_expression", "check_expression", "binary_expression", "annotation"}, 0, nil)
	res := &ObsoleteSdkIntResourceRule{AndroidRule: lintAndroidRule("ObsoleteSdkIntResource", "Obsolete SDK_INT Version Check", ALCPerformance, ALSWarning, 6)}
	registerLintResourceRule(res.AndroidRule, res, res.Confidence(), res.check)
	man := &ObsoleteSdkIntManifestRule{AndroidRule: lintAndroidRule("ObsoleteSdkIntManifest", "Obsolete SDK_INT Version Check", ALCPerformance, ALSWarning, 6)}
	registerLintManifestRule(man.AndroidRule, man, man.Confidence(), man.check)
}

func registerSourceHygieneLintChecks() {
	locale := &ConstantLocaleRule{AndroidRule: lintAndroidRule("ConstantLocale", "Constant Locale", ALCI18N, ALSWarning, 6)}
	registerLintCallRule(locale.AndroidRule, locale, locale.Confidence(), locale.check)

	notCtor := &NotConstructorRule{AndroidRule: lintAndroidRule("NotConstructor", "Not a Constructor", ALCCorrectness, ALSWarning, 4)}
	registerLintRule(notCtor.AndroidRule, notCtor, notCtor.Confidence(), notCtor.check,
		[]string{"function_declaration", "method_declaration"}, 0, nil)

	canvas := &CanvasSizeRule{AndroidRule: lintAndroidRule("CanvasSize", "Wrong Canvas Size", ALCCorrectness, ALSWarning, 6)}
	registerLintRule(canvas.AndroidRule, canvas, canvas.Confidence(), canvas.check,
		[]string{"call_expression", "method_invocation", "navigation_expression"}, api.NeedsTypeInfo|api.NeedsOracleCallTargets,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"getWidth", "getHeight"}})

	leak := &WifiManagerLeakRule{AndroidRule: lintAndroidRule("WifiManagerLeak", "WifiManager Leak", ALCCorrectness, ALSError, 6)}
	registerLintCallRule(leak.AndroidRule, leak, leak.Confidence(), leak.check)
	potential := &WifiManagerPotentialLeakRule{AndroidRule: lintAndroidRule("WifiManagerPotentialLeak", "WifiManager Potential Leak", ALCCorrectness, ALSWarning, 6)}
	registerLintCallRule(potential.AndroidRule, potential, potential.Confidence(), potential.check)
}

func registerDiffUtilEquals() {
	r := &DiffUtilEqualsRule{AndroidRule: lintAndroidRule("DiffUtilEquals", "Suspicious DiffUtil Equality", ALCCorrectness, ALSError, 4)}
	registerLintRule(r.AndroidRule, r, r.Confidence(), r.check,
		[]string{"equality_expression", "binary_expression", "call_expression", "method_invocation"}, 0, nil)
}

func registerManifestPermissionNameChecks() {
	reserved := &ReservedSystemPermissionRule{AndroidRule: lintAndroidRule("ReservedSystemPermission",
		"Permission name is a reserved Android permission", ALCSecurity, ALSError, 5)}
	registerLintManifestRule(reserved.AndroidRule, reserved, reserved.Confidence(), reserved.check)
	naming := &PermissionNamingConventionRule{AndroidRule: lintAndroidRule("PermissionNamingConvention",
		"Permission name does not follow recommended convention", ALCSecurity, ALSWarning, 5)}
	registerLintManifestRule(naming.AndroidRule, naming, naming.Confidence(), naming.check)
	known := &KnownPermissionErrorRule{AndroidRule: lintAndroidRule("KnownPermissionError",
		"Value specified for permission is a known error", ALCSecurity, ALSError, 5)}
	registerLintManifestRule(known.AndroidRule, known, known.Confidence(), known.check)
	system := &SystemPermissionTypoRule{AndroidRule: lintAndroidRule("SystemPermissionTypo",
		"Permission appears to be a standard permission with a typo", ALCSecurity, ALSWarning, 5)}
	registerLintManifestRule(system.AndroidRule, system, system.Confidence(), system.check)
	custom := &CustomPermissionTypoRule{AndroidRule: lintAndroidRule("CustomPermissionTypo",
		"Permission appears to be a custom permission with a typo", ALCSecurity, ALSWarning, 5)}
	registerLintManifestRule(custom.AndroidRule, custom, custom.Confidence(), custom.check)
}

func registerManifestOrientationChecks() {
	locked := &LockedOrientationActivityRule{AndroidRule: lintAndroidRule("LockedOrientationActivity",
		"Incompatible screenOrientation value", ALCCorrectness, ALSWarning, 4)}
	registerLintManifestRule(locked.AndroidRule, locked, locked.Confidence(), locked.check)
	resizeable := &NonResizeableActivityRule{AndroidRule: lintAndroidRule("NonResizeableActivity",
		"Activity is set to be non-resizeable", ALCCorrectness, ALSWarning, 4)}
	registerLintManifestRule(resizeable.AndroidRule, resizeable, resizeable.Confidence(), resizeable.check)
	unique := &IntentFilterUniqueDataAttributesRule{AndroidRule: lintAndroidRule("IntentFilterUniqueDataAttributes",
		"Data tags should only declare unique attributes", ALCCorrectness, ALSWarning, 5)}
	registerLintManifestRule(unique.AndroidRule, unique, unique.Confidence(), unique.check)
	discouraged := &DiscouragedAPIRule{AndroidRule: lintAndroidRule("DiscouragedApi", "Using discouraged APIs",
		ALCCorrectness, ALSWarning, 2)}
	registerLintManifestRule(discouraged.AndroidRule, discouraged, discouraged.Confidence(), discouraged.check)
}

func registerManifestAppLinkChecks() {
	urlError := &AppLinkURLErrorRule{AndroidRule: lintAndroidRule("AppLinkUrlError", "URI invalid",
		ALCCorrectness, ALSError, 5)}
	registerLintManifestRule(urlError.AndroidRule, urlError, urlError.Confidence(), urlError.check)
	warning := &AppLinkWarningRule{AndroidRule: lintAndroidRule("AppLinkWarning", "App Link warning",
		ALCCorrectness, ALSWarning, 5)}
	registerLintManifestRule(warning.AndroidRule, warning, warning.Confidence(), warning.check)
}

func registerIntentLintChecks() {
	reset := &IntentResetRule{AndroidRule: lintAndroidRule("IntentReset", "Suspicious mix of `setType` and `setData`", ALCCorrectness, ALSWarning, 6)}
	registerLintRule(reset.AndroidRule, reset, reset.Confidence(), reset.check,
		[]string{"call_expression", "object_creation_expression"}, 0, nil)

	nullAction := &IntentWithNullActionLaunchRule{AndroidRule: lintAndroidRule("IntentWithNullActionLaunch",
		"Unsafe intent launched with no action set", ALCSecurity, ALSWarning, 9)}
	registerLintRule(nullAction.AndroidRule, nullAction, nullAction.Confidence(), nullAction.check,
		[]string{"call_expression", "object_creation_expression"}, 0, nil)
}

func registerManifestForegroundServiceChecks() {
	permission := &ForegroundServicePermissionRule{AndroidRule: lintAndroidRule("ForegroundServicePermission",
		"Missing permissions required by foregroundServiceType", ALCCorrectness, ALSError, 5)}
	registerLintManifestRule(permission.AndroidRule, permission, permission.Confidence(), permission.check)
	queryAll := &QueryAllPackagesPermissionRule{AndroidRule: lintAndroidRule("QueryAllPackagesPermission",
		"Using the QUERY_ALL_PACKAGES permission", ALCCompliance, ALSError, 8)}
	registerLintManifestRule(queryAll.AndroidRule, queryAll, queryAll.Confidence(), queryAll.check)
}
