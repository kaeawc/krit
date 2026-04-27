package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"strings"
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"preferredWidth", "preferredHeight", "preferredSize"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Needs: v2.NeedsResolver, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				for i := 0; i < file.FlatChildCount(args); i++ {
					child := file.FlatChild(args, i)
					if file.FlatType(child) != "value_argument" {
						continue
					}
					argText := strings.TrimSpace(file.FlatNodeText(child))
					eqIdx := strings.Index(argText, "=")
					if eqIdx < 0 {
						continue
					}
					label := strings.TrimSpace(argText[:eqIdx])
					if !hardcodedTextLabels[label] {
						continue
					}
					valueText := strings.TrimSpace(argText[eqIdx+1:])
					if valueText == "" || valueText[0] != '"' {
						continue
					}
					if strings.Contains(valueText, "stringResource(") || strings.Contains(valueText, "getString(") {
						continue
					}
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Hardcoded text. Use string resources for localization.")
					return
				}
			},
		})
	}
	{
		r := &LogDetectorRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "LogConditional", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "LogConditional", Brief: "Unconditional logging calls",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"v", "d", "i", "println", "isLoggable"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !logCallIsAndroidLog(ctx, idx) {
					return
				}
				if hasAndroidLogGuardFlat(ctx, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Unconditional logging call. Wrap in Log.isLoggable() for performance.")
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatContainsStringInterpolation(file, idx) {
					return
				}
				content := stringLiteralContent(file, idx)
				if !strings.Contains(content, "/sdcard") && !strings.Contains(content, "/mnt/sdcard") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Hardcoded /sdcard path. Use Environment.getExternalStorageDirectory() instead.")
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"acquire", "release"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "assignment"}, Needs: v2.NeedsTypeInfo, Confidence: 0.9, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"setJavaScriptEnabled"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
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
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), Needs: v2.NeedsLinePass, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
	{
		r := &PrivateKeyRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "PackagedPrivateKey", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:  "PackagedPrivateKey", Brief: "Packaged private key",
			Category: ALCSecurity, ALSeverity: ALSFatal, Priority: 8,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev), Needs: v2.NeedsLinePass, Confidence: r.Confidence(), OriginalV1: r, Check: r.check})
	}
}
