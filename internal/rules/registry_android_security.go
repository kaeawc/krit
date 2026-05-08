package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerAndroidSecurityRules() {

	// --- from android_security.go ---
	{
		r := &FieldGetterRule{FlatDispatchBase: FlatDispatchBase{}, AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "FieldGetter", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "FieldGetter", Brief: "Using getter instead of field access",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 4,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(),
			Sev: api.Severity(r.Sev), NodeTypes: []string{"call_expression"},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &AddJavascriptInterfaceRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "AddJavascriptInterface", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "AddJavascriptInterface", Brief: "addJavascriptInterface called",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 9,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Needs: api.NeedsTypeInfo,
			Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Implementation: r,
			Oracle:            &api.OracleFilter{Identifiers: []string{"addJavascriptInterface"}},
			OracleCallTargets: &api.OracleCallTargetFilter{CalleeNames: []string{"addJavascriptInterface"}},
			// Traverses the class hierarchy to confirm the receiver is a WebView subtype;
			// only ClassShell+Supertypes needed, no member inspection.
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{ClassShell: true, Supertypes: true},
			Check:                  r.check,
		})
	}
	{
		r := &GetInstanceRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "GetInstance", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "GetInstance", Brief: "Cipher.getInstance with insecure algorithm",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 9,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
	{
		r := &WeakMessageDigestRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WeakMessageDigest", RuleSetName: "security", Sev: "warning"},
			IssueID:  "WeakMessageDigest", Brief: "Weak MessageDigest algorithm",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &RsaNoPaddingRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "RsaNoPadding", RuleSetName: "security", Sev: "warning"},
			IssueID:  "RsaNoPadding", Brief: "RSA cipher without padding",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &PrngFromSystemTimeRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "PrngFromSystemTime", RuleSetName: "security", Sev: "warning"},
			IssueID:  "PrngFromSystemTime", Brief: "Predictable Random seed from system time",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "object_creation_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &OkHTTPDisableSslValidationRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "OkHttpDisableSslValidation", RuleSetName: "security", Sev: "warning"},
			IssueID:  "OkHttpDisableSslValidation", Brief: "OkHttp TLS validation disabled",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &DisableCertificatePinningRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "DisableCertificatePinning", RuleSetName: "security", Sev: "warning"},
			IssueID:  "DisableCertificatePinning", Brief: "CertificatePinner has no pins",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &InsecureTrustManagerRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "InsecureTrustManager", RuleSetName: "security", Sev: "warning"},
			IssueID:  "InsecureTrustManager", Brief: "Trust manager accepts all certificates",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 8,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"class_declaration", "object_literal", "object_creation_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &ImplicitPendingIntentRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ImplicitPendingIntent", RuleSetName: "security", Sev: "warning"},
			IssueID:  "ImplicitPendingIntent", Brief: "PendingIntent missing mutability flag",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &WeakMacAlgorithmRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WeakMacAlgorithm", RuleSetName: "security", Sev: "warning"},
			IssueID:  "WeakMacAlgorithm", Brief: "Weak Mac algorithm",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &WeakKeySizeRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WeakKeySize", RuleSetName: "security", Sev: "warning"},
			IssueID:  "WeakKeySize", Brief: "Weak crypto key size",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &StaticIvRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "StaticIv", RuleSetName: "security", Sev: "warning"},
			IssueID:  "StaticIv", Brief: "Static IV literal",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "object_creation_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &HardcodedSecretKeyRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "HardcodedSecretKey", RuleSetName: "security", Sev: "warning"},
			IssueID:  "HardcodedSecretKey", Brief: "SecretKeySpec uses literal key bytes",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "object_creation_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &HardcodedHTTPURLRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "HardcodedHttpUrl", RuleSetName: "security", Sev: "warning"},
			IssueID:  "HardcodedHttpUrl", Brief: "Hardcoded HTTP URL in network API",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation", "object_creation_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &StartActivityWithUntrustedIntentRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "StartActivityWithUntrustedIntent", RuleSetName: "security", Sev: "warning"},
			IssueID:  "StartActivityWithUntrustedIntent", Brief: "Parsed Intent launched without package or component guard",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &EasterEggRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "EasterEgg", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "EasterEgg", Brief: "Code contains easter egg references",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), Needs: api.NeedsLinePass, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
	{
		r := &WebViewAllowContentAccessRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WebViewAllowContentAccess", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "WebViewAllowContentAccess", Brief: "WebSettings.allowContentAccess explicitly enabled",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation", "assignment"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &WebViewAllowFileAccessRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WebViewAllowFileAccess", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "WebViewAllowFileAccess", Brief: "WebSettings.allowFileAccess explicitly enabled",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation", "assignment"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &WebViewMixedContentAllowAllRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WebViewMixedContentAllowAll", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "WebViewMixedContentAllowAll", Brief: "WebSettings.mixedContentMode set to MIXED_CONTENT_ALWAYS_ALLOW",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 8,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation", "assignment"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &WebViewUniversalAccessFromFileUrlsRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WebViewUniversalAccessFromFileUrls", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:  "WebViewUniversalAccessFromFileUrls", Brief: "WebSettings.allowUniversalAccessFromFileURLs explicitly enabled",
			Category: ALCSecurity, ALSeverity: ALSError, Priority: 9,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation", "assignment"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &WebViewFileAccessFromFileUrlsRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WebViewFileAccessFromFileUrls", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "WebViewFileAccessFromFileUrls", Brief: "WebSettings.allowFileAccessFromFileURLs explicitly enabled",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 8,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation", "assignment"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &WebViewDebuggingEnabledRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WebViewDebuggingEnabled", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "WebViewDebuggingEnabled", Brief: "WebView remote debugging explicitly enabled",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &ExportedContentProviderRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ExportedContentProvider", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ExportedContentProvider", Brief: "Exported content provider does not require permission",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
	{
		r := &ExportedReceiverRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ExportedReceiver", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ExportedReceiver", Brief: "Exported receiver does not require permission",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
	{
		r := &GrantAllUrisRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "GrantAllUris", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "GrantAllUris", Brief: "Overly broad URI permissions",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Needs: api.NeedsTypeInfo,
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r,
			// Traverses the class hierarchy to confirm the receiver is a Context subtype;
			// only ClassShell+Supertypes needed, no member inspection.
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{ClassShell: true, Supertypes: true},
			Check:                  r.check,
		})
	}
	{
		r := &UnprotectedDynamicReceiverRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "UnprotectedDynamicReceiver", RuleSetName: "security", Sev: "info"},
			IssueID:  "UnprotectedDynamicReceiver", Brief: "Dynamic receiver registered without broadcast permission",
			Category: ALCSecurity, ALSeverity: ALSInformational, Priority: 5,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &AllowAllHostnameVerifierRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "AllowAllHostnameVerifier", RuleSetName: "security", Sev: "warning"},
			IssueID:  "AllowAllHostnameVerifier", Brief: "HostnameVerifier accepts all hostnames",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"class_declaration"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &BroadcastReceiverExportedFlagMissingRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "BroadcastReceiverExportedFlagMissing", RuleSetName: "security", Sev: "warning"},
			IssueID:  "BroadcastReceiverExportedFlagMissing", Brief: "Dynamic receiver registration missing exported flag",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 6,
			Origin: "Krit",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &SecureRandomRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "SecureRandom", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "SecureRandom", Brief: "Insecure random source or deterministic SecureRandom seed",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 9,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &TrustedServerRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "TrustedServer", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "TrustedServer", Brief: "Trusting all certificates",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), NodeTypes: []string{"simple_identifier", "type_identifier", "class_declaration", "object_declaration", "object_literal"}, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
	{
		r := &WorldReadableFilesRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WorldReadableFiles", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "WorldReadableFiles", Brief: "Using MODE_WORLD_READABLE",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"simple_identifier", "identifier"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &WorldWriteableFilesRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WorldWriteableFiles", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "WorldWriteableFiles", Brief: "Using MODE_WORLD_WRITEABLE",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"simple_identifier", "identifier"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r, Check: r.check,
		})
	}
	{
		r := &DrawAllocationRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "DrawAllocation", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "DrawAllocation", Brief: "Memory allocations within drawing code",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 9,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &FloatMathRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "FloatMath", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "FloatMath", Brief: "Using FloatMath instead of Math",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 3,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), NodeTypes: []string{"navigation_expression"}, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
	{
		r := &HandlerLeakRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "HandlerLeak", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "HandlerLeak", Brief: "Handler reference leaks",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 7,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), NodeTypes: []string{"class_declaration", "object_literal", "object_creation_expression"}, Needs: api.NeedsResolver, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, TypeInfo: api.TypeInfoHint{PreferBackend: api.PreferResolver, Required: true}, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
	{
		r := &RecycleRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "Recycle", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "Recycle", Brief: "Missing recycle() calls",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 7,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), NodeTypes: []string{"property_declaration"}, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
	{
		r := &ByteOrderMarkRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ByteOrderMark", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ByteOrderMark", Brief: "Byte order mark (BOM) found in file",
			Category: ALCI18N, ALSeverity: ALSWarning, Priority: 8,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), Needs: api.NeedsLinePass, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
}
