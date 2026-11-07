package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerAndroidSourceExtraRules() {

	// --- from android_source_extra.go ---
	{
		r := &ViewConstructorRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "ViewConstructor", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "ViewConstructor", Brief: "Missing View constructors", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Needs: api.NeedsResources, Languages: []scanner.Language{scanner.LangKotlin},
			AndroidDeps: uint32(AndroidDepLayout),
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// Without a layout index we cannot tell whether XML inflates the view, so stay silent.
				if ctx.ResourceIndex == nil || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "private") || scanner.IsTestFile(file.Path) {
					return
				}
				if androidHasAnnotationFlat(file, idx, "SuppressLint") &&
					strings.Contains(declarationHeaderText(file, idx), "\"ViewConstructor\"") {
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
					if !viewConstructorSupertypeIsAndroidView(file, idx, child, typeName) {
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
				// Only XML inflation needs the (Context, AttributeSet) constructor, so skip a view the file builds in code.
				if viewConstructedInFile(file, extractIdentifierFlat(file, idx)) {
					return
				}
				// Only a class named in layout XML is inflated, and XML cannot supply extra required arguments.
				if !viewClassReferencedInLayouts(ctx.ResourceIndex, viewConstructorClassFQN(file, idx)) || viewConstructorNeedsExtraArguments(file, idx) {
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
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						"Custom View subclass is missing (Context) and (Context, AttributeSet) constructors.")
					return
				}
				if !hasAttrSetCtor {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						"Custom View subclass is missing (Context, AttributeSet) constructor needed for XML inflation.")
				}
			},
		})
	}
	{
		r := &WrongImportRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "WrongImport", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "WrongImport", Brief: "Importing android.R instead of app R", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 5, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"import_header"}, Confidence: r.Confidence(), Fix: api.FixIdiomatic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &LayoutInflationRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "LayoutInflation", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "InflateParams", Brief: "Layout inflation without parent", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 5, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"},
			Needs:     api.NeedsResources, Languages: []scanner.Language{scanner.LangKotlin},
			AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ViewTagRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "ViewTag", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "ViewTag", Brief: "Tagged object leaks (setTag with view, holder or cursor below API 14)", Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 6, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"},
			Needs: api.NeedsTypeInfo |
				api.NeedsOracleCallTargets |
				api.NeedsOracleSupertypes,
			Confidence: r.Confidence(), Implementation: r,
			NeedsLibraryFacts: true,
			Oracle:            &api.OracleFilter{Identifiers: []string{"setTag"}},
			Check:             r.check,
		})
	}
	{
		r := &CutPasteIDRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "CutPasteId", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "CutPasteId", Brief: "Likely cut & paste mistakes", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"function_declaration", "method_declaration", "constructor_declaration"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &HardcodedRandomSeedRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "HardcodedRandomSeed", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "", Brief: "Hardcoded seed defeats SecureRandom", Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 9, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "object_creation_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Aliases:    []string{"TrulyRandom"},
			Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &TrulyRandomRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "TrulyRandom", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "TrulyRandom", Brief: "Weak RNG", Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 9, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs:             api.NeedsParsedFiles,
			NeedsLibraryFacts: true,
			Languages:         []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence:        r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingPermissionRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "MissingPermission", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "MissingPermission", Brief: "Missing permission check before API call", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 9, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Confidence: r.Confidence(), Implementation: r,
			Oracle: &api.OracleFilter{Identifiers: missingPermissionOracleIdentifiers()},
			Check:  r.check,
		})
	}
	{
		r := &WrongConstantRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "WrongConstant", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "WrongConstant", Brief: "Wrong constant passed to API", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 6, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Oracle:     &api.OracleFilter{Identifiers: []string{"setVisibility", "setLayoutDirection", "setImportantForAccessibility", "setGravity", "setOrientation"}},
			TypeInfo:   api.TypeInfoHint{PreferBackend: api.PreferOracle, Required: true},
			Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &InstantiatableRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "Instantiatable", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "Instantiatable", Brief: "Registered class not instantiatable", Category: ALCCorrectness, ALSeverity: ALSFatal, Priority: 6, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceHigher, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				isComponent := false
				for _, base := range componentSuperclasses {
					if classExtendsAndroidClass(file, idx, base) {
						isComponent = true
						break
					}
				}
				if !isComponent {
					return
				}
				// The class itself carries `private` when the class is private.
				if file.FlatHasModifier(idx, "private") {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						"This class is registered as an Android component but cannot be instantiated. Remove the private constructor or add a public no-arg constructor.")
					return
				}
				// Primary constructor's `private` modifier (e.g.
				// `class Foo private constructor(...)`).
				if pc, ok := file.FlatFindChild(idx, "primary_constructor"); ok {
					if file.FlatHasModifier(pc, "private") {
						ctx.EmitAt(declLine(file, idx), declCol(file, idx),
							"This class is registered as an Android component but cannot be instantiated. Remove the private constructor or add a public no-arg constructor.")
					}
				}
			},
		})
	}
	{
		r := &RtlAwareRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "RtlAware", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "RtlAware", Brief: "Using non-RTL-aware View methods", Category: ALCRTL, ALSeverity: ALSWarning, Priority: 5, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets, Confidence: api.ConfidenceHigher, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if repl, ok := rtlAwareMethods[name]; ok {
					// With an oracle, only View's own getters count; Rect.getLeft() and app classes do not.
					if cr, isComposite := ctx.Resolver.(*oracle.CompositeResolver); isComposite && cr.Oracle() != nil {
						if !strings.HasPrefix(oracleLookupCallTargetFlat(cr.Oracle(), file, idx), "android.view.View.") {
							return
						}
					}
					if repl == "" {
						ctx.EmitAt(callLine(file, idx), callCol(file, idx),
							name+" returns a physical left/right position and is not RTL-aware. Check View.getLayoutDirection() or use start/end based values.")
						return
					}
					ctx.EmitAt(callLine(file, idx), callCol(file, idx),
						"Use RTL-aware "+repl+" instead of "+name+" for bidirectional layout support.")
				}
			},
		})
	}
	{
		r := &RtlFieldAccessRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "RtlFieldAccess", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "RtlFieldAccess", Brief: "Direct field access of non-RTL-aware View fields", Category: ALCRTL, ALSeverity: ALSWarning, Priority: 5, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Confidence: api.ConfidenceHigher, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatContainsStringInterpolation(file, idx) {
					return
				}
				content := stringLiteralContent(file, idx)
				for _, field := range rtlFieldNames {
					if content == field {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							"Direct access to View."+field+" via reflection is not RTL-aware. Use the corresponding getter method instead.")
						return
					}
				}
			},
		})
	}
	{
		r := &GridLayoutRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "GridLayout", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "GridLayout", Brief: "GridLayout without columnCount", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 4, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "GridLayout" || !gridLayoutIsPlatformClass(file) {
					return
				}
				if gridLayoutConfiguredInPlace(file, idx) {
					return
				}
				var container uint32
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					t := file.FlatType(p)
					if t == "statements" || t == "function_body" || t == "class_body" || t == "source_file" {
						container = p
						break
					}
				}
				if container != 0 {
					receiver := initializerAssignedName(file, idx)
					if receiver != "" && gridLayoutColumnCountAssignedAfter(file, container, idx, receiver) {
						return
					}
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx), "GridLayout should specify a columnCount. Without it, all children will be in a single row.")
			},
		})
	}
	{
		r := &MangledCRLFRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "MangledCRLF", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "MangledCRLF", Brief: "Mixed line endings in file", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 3, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ResourceNameRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "ResourceName", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "ResourceName", Brief: "Resource name not in snake_case", Category: ALCCorrectness, ALSeverity: ALSFatal, Priority: 4, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: api.ConfidenceHigher, Implementation: r,
			Check: func(ctx *api.Context) {
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
				// Style names are conventionally CamelCase with dots (Theme.MyApp becomes R.style.Theme_MyApp).
				if !androidResourceTypes[segments[1]] || segments[1] == "style" {
					return
				}
				if !snakeCaseRe.MatchString(segments[2]) {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Resource name `R."+segments[1]+"."+segments[2]+"` should use snake_case.")
				}
			},
		})
	}
	{
		r := &ProguardRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "Proguard", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "Proguard", Brief: "Obsolete proguard.cfg reference", Category: ALCCorrectness, ALSeverity: ALSFatal, Priority: 4, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ProguardSplitRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "ProguardSplit", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "ProguardSplit", Brief: "Proguard config should be split", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 3, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &NfcTechWhitespaceRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "NfcTechWhitespace", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "NfcTechWhitespace", Brief: "Whitespace in NFC tech-list", Category: ALCCorrectness, ALSeverity: ALSFatal, Priority: 6, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &InvalidUsesTagAttributeRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "InvalidUsesTagAttribute", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "InvalidUsesTagAttribute", Brief: "Invalid name attribute for uses element", Category: ALCCorrectness, ALSeverity: ALSError, Priority: 6, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &LibraryCustomViewRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "LibraryCustomView", RuleSetName: androidRuleSet, Sev: "error"}, IssueID: "LibraryCustomView", Brief: "Custom view using hardcoded namespace", Category: ALCCorrectness, ALSeverity: ALSFatal, Priority: 6, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnknownIDInLayoutRule{AndroidRule: AndroidRule{BaseRule: BaseRule{RuleName: "UnknownIdInLayout", RuleSetName: androidRuleSet, Sev: "warning"}, IssueID: "UnknownIdInLayout", Brief: "Reference to an id that is not in the current layout", Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 5, Origin: "AOSP Android Lint"}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}

// gridLayoutIsPlatformClass reports whether the file imports a GridLayout view (platform or AndroidX).
func gridLayoutIsPlatformClass(file *scanner.File) bool {
	for _, marker := range []string{
		"android.widget.GridLayout", "androidx.gridlayout.widget.GridLayout",
		"android.widget.*", "androidx.gridlayout.widget.*",
	} {
		if bytes.Contains(file.Content, []byte(marker)) {
			return true
		}
	}
	return false
}

// gridLayoutConfiguredInPlace reports whether the constructed GridLayout is configured by a scope
// function such as `GridLayout(c).apply { columnCount = 2 }`.
func gridLayoutConfiguredInPlace(file *scanner.File, call uint32) bool {
	parent, ok := file.FlatParent(call)
	if !ok || file.FlatType(parent) != "navigation_expression" {
		return false
	}
	outer, ok := file.FlatParent(parent)
	if !ok || file.FlatType(outer) != "call_expression" {
		return false
	}
	switch flatCallExpressionName(file, outer) {
	case "apply", "also", "run", "let":
		text := file.FlatNodeText(outer)
		return strings.Contains(text, "columnCount") || strings.Contains(text, "ColumnCount")
	}
	return false
}

func gridLayoutColumnCountAssignedAfter(file *scanner.File, container, call uint32, receiver string) bool {
	if file == nil || container == 0 || call == 0 || receiver == "" {
		return false
	}
	callStart := file.FlatStartByte(call)
	found := false
	file.FlatWalkNodes(container, "call_expression", func(setter uint32) {
		if found || file.FlatStartByte(setter) <= callStart {
			return
		}
		if flatCallExpressionName(file, setter) == "setColumnCount" && flatReceiverNameFromCall(file, setter) == receiver {
			found = true
		}
	})
	file.FlatWalkNodes(container, "assignment", func(assign uint32) {
		if found || file.FlatStartByte(assign) <= callStart {
			return
		}
		left := firstNamedChild(file, assign)
		if gridLayoutColumnCountTarget(file, left, receiver) {
			found = true
		}
	})
	return found
}

func gridLayoutColumnCountTarget(file *scanner.File, idx uint32, receiver string) bool {
	if file == nil || idx == 0 || receiver == "" {
		return false
	}
	if file.FlatType(idx) == "directly_assignable_expression" {
		ids := assignableIdentifierParts(file, idx)
		return len(ids) == 2 && ids[0] == receiver && ids[1] == "columnCount"
	}
	if file.FlatType(idx) == "navigation_expression" {
		ids := navigationIdentifierParts(file, idx)
		return len(ids) >= 2 && ids[0] == receiver && ids[len(ids)-1] == "columnCount"
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) && gridLayoutColumnCountTarget(file, child, receiver) {
			return true
		}
	}
	return false
}

func assignableIdentifierParts(file *scanner.File, idx uint32) []string {
	var ids []string
	file.FlatWalkNodes(idx, "simple_identifier", func(ident uint32) {
		ids = append(ids, file.FlatNodeText(ident))
	})
	return ids
}

func navigationIdentifierParts(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	var ids []string
	var walk func(uint32)
	walk = func(node uint32) {
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			switch file.FlatType(child) {
			case "simple_identifier":
				ids = append(ids, file.FlatNodeText(child))
			case "navigation_expression", "navigation_suffix":
				walk(child)
			}
		}
	}
	walk(idx)
	return ids
}

func firstNamedChild(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

// viewConstructorSupertypeIsAndroidView rejects interfaces and non-Android types that share a View class name.
func viewConstructorSupertypeIsAndroidView(file *scanner.File, classDecl, deleg uint32, typeName string) bool {
	// A class supertype is constructed, so a bare `: View` with a primary constructor is an interface.
	_, invoked := file.FlatFindChild(deleg, "constructor_invocation")
	if !invoked {
		if _, hasPrimary := file.FlatFindChild(classDecl, "primary_constructor"); hasPrimary {
			return false
		}
		hasSecondary := false
		if body, ok := file.FlatFindChild(classDecl, "class_body"); ok {
			file.FlatWalkNodes(body, "secondary_constructor", func(uint32) { hasSecondary = true })
		}
		if !hasSecondary {
			return false
		}
	}
	// Qualified names such as `Contract.View` refer to a nested type, not the Android class.
	text := strings.TrimSpace(file.FlatNodeText(deleg))
	if i := strings.IndexAny(text, "(<"); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)
	if strings.Contains(text, ".") && !strings.HasPrefix(text, "android.") && !strings.HasPrefix(text, "com.google.android.") {
		return false
	}
	// An explicit non-Android import of the same simple name is a different type.
	if imp, ok := handlerFileImportForSimple(file, typeName); ok {
		return strings.HasPrefix(imp, "android.") || strings.HasPrefix(imp, "com.google.android.")
	}
	return true
}
