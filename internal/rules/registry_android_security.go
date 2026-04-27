package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(),
			Sev: v2.Severity(r.Sev), NodeTypes: []string{"call_expression"},
			Confidence: r.Confidence(), OriginalV1: r, Check: r.check,
		})
	}
	{
		r := &AddJavascriptInterfaceRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "AddJavascriptInterface", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "AddJavascriptInterface", Brief: "addJavascriptInterface called",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 9,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, OriginalV1: r,
			Oracle:            &v2.OracleFilter{Identifiers: []string{"addJavascriptInterface"}},
			OracleCallTargets: &v2.OracleCallTargetFilter{CalleeNames: []string{"addJavascriptInterface"}},
			// Traverses the class hierarchy to confirm the receiver is a WebView subtype;
			// only ClassShell+Supertypes needed, no member inspection.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{ClassShell: true, Supertypes: true},
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
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &EasterEggRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "EasterEgg", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "EasterEgg", Brief: "Code contains easter egg references",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), Needs: v2.NeedsLinePass, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &ExportedContentProviderRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ExportedContentProvider", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ExportedContentProvider", Brief: "Exported content provider does not require permission",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &ExportedReceiverRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ExportedReceiver", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ExportedReceiver", Brief: "Exported receiver does not require permission",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &GrantAllUrisRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "GrantAllUris", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "GrantAllUris", Brief: "Overly broad URI permissions",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 7,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: r.Confidence(), OriginalV1: r,
			// Traverses the class hierarchy to confirm the receiver is a Context subtype;
			// only ClassShell+Supertypes needed, no member inspection.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{ClassShell: true, Supertypes: true},
			Check:                  r.check,
		})
	}
	{
		r := &SecureRandomRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "SecureRandom", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "SecureRandom", Brief: "Insecure random source or deterministic SecureRandom seed",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 9,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), NodeTypes: []string{"simple_identifier", "type_identifier", "class_declaration", "object_declaration", "object_literal"}, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &WorldReadableFilesRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WorldReadableFiles", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "WorldReadableFiles", Brief: "Using MODE_WORLD_READABLE",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), NodeTypes: []string{"simple_identifier"}, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &WorldWriteableFilesRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "WorldWriteableFiles", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "WorldWriteableFiles", Brief: "Using MODE_WORLD_WRITEABLE",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), NodeTypes: []string{"simple_identifier"}, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &DrawAllocationRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "DrawAllocation", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "DrawAllocation", Brief: "Memory allocations within drawing code",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 9,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), OriginalV1: r,
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
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), NodeTypes: []string{"navigation_expression"}, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &HandlerLeakRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "HandlerLeak", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "HandlerLeak", Brief: "Handler reference leaks",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 7,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), NodeTypes: []string{"class_declaration", "object_literal", "object_creation_expression"}, Needs: v2.NeedsResolver, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, TypeInfo: v2.TypeInfoHint{PreferBackend: v2.PreferResolver, Required: true}, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &RecycleRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "Recycle", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "Recycle", Brief: "Missing recycle() calls",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 7,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), NodeTypes: []string{"property_declaration"}, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &ByteOrderMarkRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ByteOrderMark", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ByteOrderMark", Brief: "Byte order mark (BOM) found in file",
			Category: ALCI18N, ALSeverity: ALSWarning, Priority: 8,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), Needs: v2.NeedsLinePass, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
}
