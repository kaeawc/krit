package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerAndroidSourceExtraRules() {

	// --- from android_source_extra.go ---
	{
		r := &ViewConstructorRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "ViewConstructor", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "ViewConstructor", Brief: "Missing View constructors", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatHasModifier(idx, "abstract") {
					return
				}
				isView := false
				for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) != "delegation_specifier" {
						continue
					}
					typeName := viewConstructorSupertypeNameFlat(file, child)
					if typeName == "" {
						continue
					}
					for _, base := range viewSuperclasses {
						if typeName == base {
							isView = true
							break
						}
					}
					if isView {
						break
					}
				}
				if !isView {
					return
				}
				hasContextCtor, hasAttrSetCtor := false, false
				accumulate := func(ctor uint32) {
					types := constructorParameterTypeFlags(file, ctor, "Context", "AttributeSet")
					if types["Context"] {
						hasContextCtor = true
					}
					if types["Context"] && types["AttributeSet"] {
						hasAttrSetCtor = true
					}
				}
				for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
					switch file.FlatType(child) {
					case "primary_constructor":
						accumulate(child)
					case "class_body":
						file.FlatWalkNodes(child, "secondary_constructor", accumulate)
					}
				}
				if !hasContextCtor {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Custom View subclass is missing (Context) and (Context, AttributeSet) constructors.")
					return
				}
				if !hasAttrSetCtor {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Custom View subclass is missing (Context, AttributeSet) constructor needed for XML inflation.")
				}
			},
		})
	}
	{
		r := &WrongImportRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "WrongImport", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "WrongImport", Brief: "Importing android.R instead of app R", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 5, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"import_header"}, Confidence: r.Confidence(), Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &LayoutInflationRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "LayoutInflation", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "InflateParams", Brief: "Layout inflation without parent", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 5, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"},
			Needs:     v2.NeedsResources, Languages: []scanner.Language{scanner.LangKotlin},
			AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ViewTagRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "ViewTag", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "ViewTag", Brief: "Tagged object may leak", Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 6, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Needs:      v2.NeedsTypeInfo,
			Confidence: r.Confidence(), OriginalV1: r,
			Oracle:            &v2.OracleFilter{Identifiers: []string{"setTag"}},
			OracleCallTargets: &v2.OracleCallTargetFilter{CalleeNames: []string{"setTag"}},
			// Checks whether the receiver extends View (class hierarchy) to
			// confirm setTag is a View.setTag call; no member signatures needed.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{ClassShell: true, Supertypes: true},
			Check:                  r.check,
		})
	}
	{
		r := &TrulyRandomRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "TrulyRandom", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "TrulyRandom", Brief: "Hardcoded seed defeats SecureRandom", Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 9, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MissingPermissionRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "MissingPermission", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "MissingPermission", Brief: "Missing permission check before API call", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 9, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo, Confidence: r.Confidence(), OriginalV1: r,
			Oracle: &v2.OracleFilter{Identifiers: missingPermissionOracleIdentifiers()},
			OracleCallTargets: &v2.OracleCallTargetFilter{
				CalleeNames:          missingPermissionCandidateCalleeNames(),
				AnnotatedIdentifiers: missingPermissionAnnotatedIdentifiers,
			},
			// Uses LookupCallTarget for FQN verification and AST for
			// @RequiresPermission guards; no oracle member annotations needed.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check:                  r.check,
		})
	}
	{
		r := &WrongConstantRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "WrongConstant", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "WrongConstant", Brief: "Wrong constant passed to API", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 6, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: v2.NeedsTypeInfo,
			Oracle: &v2.OracleFilter{Identifiers: []string{"setVisibility", "setLayoutDirection", "setImportantForAccessibility", "setGravity", "setOrientation"}},
			OracleCallTargets: &v2.OracleCallTargetFilter{
				CalleeNames: []string{"setVisibility", "setLayoutDirection", "setImportantForAccessibility", "setGravity", "setOrientation"},
			},
			TypeInfo: v2.TypeInfoHint{PreferBackend: v2.PreferOracle, Required: true},
			// Uses LookupCallTarget to verify framework target; allowed constant
			// sets come from hardcoded tables, not oracle member annotations.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Confidence:             r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &InstantiatableRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "Instantiatable", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "Instantiatable", Brief: "Registered class not instantiatable", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 6, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				isComponent := false
				for _, base := range componentSuperclasses {
					if classHasSupertypeNamed(file, idx, base) {
						isComponent = true
						break
					}
				}
				if !isComponent {
					return
				}
				// The class itself carries `private` when the class is private.
				if file.FlatHasModifier(idx, "private") {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"This class is registered as an Android component but cannot be instantiated. Remove the private constructor or add a public no-arg constructor.")
					return
				}
				// Primary constructor's `private` modifier (e.g.
				// `class Foo private constructor(...)`).
				if pc, ok := file.FlatFindChild(idx, "primary_constructor"); ok {
					if file.FlatHasModifier(pc, "private") {
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							"This class is registered as an Android component but cannot be instantiated. Remove the private constructor or add a public no-arg constructor.")
					}
				}
			},
		})
	}
	{
		r := &RtlAwareRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "RtlAware", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "RtlAware", Brief: "Using non-RTL-aware View methods", Category: ALCRTL, ALSeverity: ALSWarning, Priority: 5, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if repl, ok := rtlAwareMethods[name]; ok {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Use RTL-aware "+repl+" instead of "+name+" for bidirectional layout support.")
				}
			},
		})
	}
	{
		r := &RtlFieldAccessRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "RtlFieldAccess", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "RtlFieldAccess", Brief: "Direct field access of non-RTL-aware View fields", Category: ALCRTL, ALSeverity: ALSWarning, Priority: 5, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatContainsStringInterpolation(file, idx) {
					return
				}
				content := stringLiteralContent(file, idx)
				for _, field := range rtlFieldNames {
					if content == field {
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							"Direct access to View."+field+" via reflection is not RTL-aware. Use the corresponding getter method instead.")
						return
					}
				}
			},
		})
	}
	{
		r := &GridLayoutRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "GridLayout", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "GridLayout", Brief: "GridLayout without columnCount", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 4, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.85, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "GridLayout" {
					return
				}
				// Prefer a structural check against the enclosing statement
				// group: if any sibling or descendant expression names
				// `columnCount`, treat it as configured. Fall back to a
				// bounded byte-window scan when we can't find a statement
				// container (e.g. the call appears at top-level).
				var container uint32
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					t := file.FlatType(p)
					if t == "statements" || t == "function_body" || t == "class_body" || t == "source_file" {
						container = p
						break
					}
				}
				if container != 0 {
					hasColumnCount := false
					file.FlatWalkNodes(container, "simple_identifier", func(ident uint32) {
						if !hasColumnCount && file.FlatNodeText(ident) == "columnCount" {
							hasColumnCount = true
						}
					})
					if hasColumnCount {
						return
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "GridLayout should specify a columnCount. Without it, all children will be in a single row.")
			},
		})
	}
	{
		r := &LocaleFolderRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "LocaleFolder", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "LocaleFolder", Brief: "Wrong locale folder naming", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 6, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &UseAlpha2Rule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "UseAlpha2", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "UseAlpha2", Brief: "3-letter ISO code in locale folder", Category: ALCI18N, ALSeverity: ALSWarning, Priority: 6, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Confidence: 0.75, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MangledCRLFRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "MangledCRLF", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "MangledCRLF", Brief: "Mixed line endings in file", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 3, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Confidence: 0.75, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &ResourceNameRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "ResourceName", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "ResourceName", Brief: "Resource name not in snake_case", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 4, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Structural R.<kind>.<name> decomposition. Require the nav
				// chain to be exactly three segments rooted at `R`, with
				// the middle segment in the resource-type allow-list. The
				// previous regex matched anywhere in the text, including
				// across unrelated chains and string content.
				segments := flatNavigationChainIdentifiers(file, idx)
				if len(segments) != 3 || segments[0] != "R" {
					return
				}
				if !androidResourceTypes[segments[1]] {
					return
				}
				if !snakeCaseRe.MatchString(segments[2]) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Resource name `R."+segments[1]+"."+segments[2]+"` should use snake_case.")
				}
			},
		})
	}
	{
		r := &ProguardRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "Proguard", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "Proguard", Brief: "Obsolete proguard.cfg reference", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 4, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatContainsStringInterpolation(file, idx) {
					return
				}
				if strings.Contains(stringLiteralContent(file, idx), "proguard.cfg") {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Reference to obsolete `proguard.cfg`. Use `proguard-rules.pro` instead.")
				}
			},
		})
	}
	{
		r := &ProguardSplitRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "ProguardSplit", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "ProguardSplit", Brief: "Proguard config should be split", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 3, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &NfcTechWhitespaceRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "NfcTechWhitespace", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "NfcTechWhitespace", Brief: "Whitespace in NFC tech-list", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 6, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatContainsStringInterpolation(file, idx) {
					return
				}
				content := stringLiteralContent(file, idx)
				if strings.Contains(content, "<tech>") && nfcTechRe.MatchString(content) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Whitespace in <tech> element value. NFC tech names must not have leading/trailing whitespace.")
				}
			},
		})
	}
	{
		r := &LibraryCustomViewRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "LibraryCustomView", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "LibraryCustomView", Brief: "Custom view using hardcoded namespace", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatContainsStringInterpolation(file, idx) {
					return
				}
				content := stringLiteralContent(file, idx)
				if hardcodedNsRe.MatchString(content) && !strings.Contains(content, "apk/res-auto") && !strings.Contains(content, "apk/res/android") {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Use `http://schemas.android.com/apk/res-auto` instead of a hardcoded package namespace. Hardcoded namespaces don't work in library projects.")
				}
			},
		})
	}
	{
		r := &UnknownIdInLayoutRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "UnknownIdInLayout", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "UnknownIdInLayout", Brief: "Reference to unknown @id in layout", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6, Origin: "AOSP Android Lint"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				segments := flatNavigationChainIdentifiers(file, idx)
				if len(segments) != 3 || segments[0] != "R" || segments[1] != "id" {
					return
				}
				name := segments[2]
				if strings.Contains(name, "__") || strings.HasPrefix(name, "_") {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, "Suspicious ID reference `R.id."+name+"`. Verify this ID exists in your layout resources.")
				}
			},
		})
	}
}
