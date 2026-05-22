package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerAndroidRules() {

	// --- from android.go ---
	{
		r := &ContentDescriptionRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ContentDescription", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ContentDescription", Brief: "Image without contentDescription",
			Category: ALCAccessibility, ALSeverity: ALSWarning, Priority: 3,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if name != "Image" && name != "Icon" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if flatNamedValueArgument(file, args, "contentDescription") != 0 {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Image/Icon without contentDescription. Provide a description for accessibility.")
			},
		})
	}
	{
		r := &ObsoleteLayoutParamsRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ObsoleteLayoutParam", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ObsoleteLayoutParam", Brief: "Obsolete layout params",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Confidence: api.ConfidenceMedium, Implementation: r,
			OracleCallTargets:      &api.OracleCallTargetFilter{CalleeNames: []string{"preferredWidth", "preferredHeight", "preferredSize"}},
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name, replacement, ok := obsoleteComposeModifierCall(ctx, idx)
				if !ok {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Obsolete Compose layout modifier '"+name+"' was renamed to '"+replacement+"' in Compose 1.0.")
			},
		})
	}
	{
		r := &ViewHolderRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ViewHolder", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ViewHolder", Brief: "View holder candidates",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Needs: api.NeedsResolver, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isRecyclerAdapterClassFlat(ctx, idx) {
					return
				}
				if classHasMemberFunctionFlat(file, idx, "onCreateViewHolder") || classHasNestedViewHolderFlat(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"RecyclerView.Adapter subclass should implement the ViewHolder pattern.")
			},
		})
	}
	{
		r := &HardcodedTextRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "HardcodedText", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "HardcodedText", Brief: "Hardcoded text",
			Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &LogDetectorRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "LogConditional", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "LogConditional", Brief: "Unconditional logging calls",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Needs:      api.NeedsTypeInfo,
			Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// Test sources frequently log without guards on purpose
				// (debugging assertion failures, JUnit/Robolectric output).
				// Skip them to match user expectations and AOSP lint behavior.
				if scanner.IsTestFile(file.Path) {
					return
				}
				if !logCallIsAndroidLog(ctx, idx) {
					return
				}
				if hasAndroidLogGuardFlat(ctx, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Unconditional logging call. Wrap in Log.isLoggable() or BuildConfig.DEBUG for performance.")
			},
		})
	}
	{
		r := &SdCardPathRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "SdCardPath", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "SdCardPath", Brief: "Hardcoded reference to /sdcard",
			Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatContainsStringInterpolation(file, idx) {
					return
				}
				content := stringLiteralContent(file, idx)
				switch {
				case strings.Contains(content, "/sdcard"), strings.Contains(content, "/mnt/sdcard"):
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Hardcoded /sdcard path. Use Environment.getExternalStorageDirectory() instead.")
				case strings.Contains(content, "/data/data/"), strings.Contains(content, "/data/user/"):
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Hardcoded /data/ path. Use Context.getFilesDir() instead; the per-user data directory can vary.")
				}
			},
		})
	}
	{
		r := &WakelockRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "Wakelock", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "Wakelock", Brief: "Incorrect WakeLock usage",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 9,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Confidence: api.ConfidenceMedium, Implementation: r,
			OracleCallTargets:      &api.OracleCallTargetFilter{CalleeNames: []string{"acquire", "release"}},
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !wakeLockAcquireCall(ctx, idx) {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				foundRelease := false
				file.FlatWalkNodes(fn, "call_expression", func(call uint32) {
					if foundRelease {
						return
					}
					if call == idx {
						return
					}
					if wakeLockReleaseCallOnSameReceiver(ctx, idx, call) {
						foundRelease = true
					}
				})
				if foundRelease {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"WakeLock acquired without release. Ensure WakeLock.release() is called.")
			},
		})
	}
	{
		r := &SetJavaScriptEnabledRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "SetJavaScriptEnabled", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "SetJavaScriptEnabled", Brief: "Using setJavaScriptEnabled",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation", "assignment"},
			Needs:     api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceHigher, Implementation: r,
			JavaFacts:              &api.JavaFactProfile{ReceiverTypesForCallees: []string{"setJavaScriptEnabled"}},
			NeedsLibraryFacts:      true,
			OracleCallTargets:      &api.OracleCallTargetFilter{CalleeNames: []string{"setJavaScriptEnabled"}},
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				switch file.FlatType(idx) {
				case "call_expression":
					if !setJavaScriptEnabledCall(ctx, idx) {
						return
					}
					args := flatCallKeyArguments(file, idx)
					firstArg := flatPositionalValueArgument(file, args, 0)
					if firstArg == 0 {
						return
					}
					expr := flatValueArgumentExpression(file, firstArg)
					if !isBooleanLiteralTrue(file, expr) {
						return
					}
				case "method_invocation":
					if !setJavaScriptEnabledJavaCall(ctx, idx) {
						return
					}
				case "assignment":
					if !webSettingsAssignmentTarget(ctx, idx) {
						return
					}
					if !isBooleanLiteralTrue(file, assignmentRHS(file, idx)) {
						return
					}
				default:
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Using setJavaScriptEnabled(true). Review for XSS vulnerabilities.")
			},
		})
	}
	{
		r := &ExportedServiceRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ExportedService", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ExportedService", Brief: "Exported service does not require permission",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
	{
		r := &PrivateKeyRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "PackagedPrivateKey", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:  "PackagedPrivateKey", Brief: "Packaged private key",
			Category: ALCSecurity, ALSeverity: ALSFatal, Priority: 8,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev), Needs: api.NeedsLinePass, Confidence: r.Confidence(), Implementation: r, Check: r.check})
	}
}
