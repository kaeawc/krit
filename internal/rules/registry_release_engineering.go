package rules

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerReleaseEngineeringRules() {

	// --- from release_engineering.go ---
	{
		r := &BuildConfigDebugInLibraryRule{BaseRule: BaseRule{RuleName: "BuildConfigDebugInLibrary", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects BuildConfig.DEBUG references inside Android library modules where the value is always false in release."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isBuildConfigDebugReferenceFlat(file, idx) {
					return
				}
				if !isAndroidLibrarySourceFile(file.Path) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "BuildConfig.DEBUG in an Android library module is false in consumer release builds; this guard may silently drop its body.")
			},
		})
	}
	{
		r := &BuildConfigDebugInvertedRule{BaseRule: BaseRule{RuleName: "BuildConfigDebugInverted", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects negated BuildConfig.DEBUG guards wrapping logging calls that likely invert a debug-only check."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				condition, body := ifConditionAndThenBodyFlat(file, idx)
				if !isNegatedBuildConfigDebugConditionFlat(file, condition) {
					return
				}
				if !containsLoggingCallFlat(file, body) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Negated BuildConfig.DEBUG guard wraps logging; this likely inverts a debug-only log check.")
			},
		})
	}
	{
		r := &AllProjectsBlockRule{BaseRule: BaseRule{RuleName: "AllProjectsBlock", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects deprecated allprojects {} blocks in Gradle build scripts."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isGradleBuildScript(file.Path) {
					return
				}
				if flatCallExpressionName(file, idx) != "allprojects" {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, "`allprojects {}` is deprecated in Gradle 8.x; move shared configuration to settings-level repositories or convention plugins.")
			},
		})
	}
	{
		r := &HardcodedEnvironmentNameRule{BaseRule: BaseRule{RuleName: "HardcodedEnvironmentName", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects hardcoded environment names like 'dev', 'staging', or 'prod' passed to config APIs."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				funcName := flatCallExpressionName(file, idx)
				if !isEnvironmentConfigCallName(funcName) {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				for i := 0; i < file.FlatChildCount(args); i++ {
					arg := file.FlatChild(args, i)
					if arg == 0 || file.FlatType(arg) != "value_argument" {
						continue
					}
					literal := hardcodedEnvironmentLiteralFlat(file, arg)
					if literal == "" {
						continue
					}
					ctx.EmitAt(file.FlatRow(arg)+1, file.FlatCol(arg)+1,
						fmt.Sprintf("Hardcoded environment name %q passed to %s(); prefer a build- or runtime-provided environment value.", literal, funcName))
					return
				}
			},
		})
	}
	{
		r := &DebugToastInProductionRule{BaseRule: BaseRule{RuleName: "DebugToastInProduction", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects Toast.makeText calls whose message starts with debug-related prefixes in production code."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) {
					return
				}
				name := flatCallExpressionName(file, idx)
				if name != "makeText" {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if receiver != "Toast" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				// Second argument is the message (first is context)
				argCount := 0
				for i := 0; i < file.FlatChildCount(args); i++ {
					arg := file.FlatChild(args, i)
					if arg == 0 || file.FlatType(arg) != "value_argument" {
						continue
					}
					argCount++
					if argCount == 2 {
						text := strings.TrimSpace(file.FlatNodeText(arg))
						if debugToastPrefixRe.MatchString(text) {
							ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Toast message starts with a debug prefix; remove or guard behind BuildConfig.DEBUG.")
						}
						break
					}
				}
			},
		})
	}
	{
		r := &PrintlnInProductionRule{BaseRule: BaseRule{RuleName: "PrintlnInProduction", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects println or print calls in production code that should use a logging framework."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if printlnNonProductionPath(file.Path) {
					return
				}
				name := flatCallExpressionName(file, idx)
				receiver := flatReceiverNameFromCall(file, idx)
				if !isProductionPrintCallFlat(file, idx, name, receiver) {
					return
				}
				if printlnInsideGradleTaskAction(file, idx) {
					return
				}
				// Exclude if inside a top-level fun main()
				if enclosing, ok := flatEnclosingFunction(file, idx); ok && enclosing != 0 {
					funcText := flatFunctionName(file, enclosing)
					if funcText == "main" {
						if parent, ok := file.FlatParent(enclosing); ok {
							if file.FlatType(parent) == "source_file" {
								return
							}
						}
					}
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "println/print in production code; use a logging framework instead.")
			},
		})
	}
	{
		r := &PrintStackTraceInProductionRule{BaseRule: BaseRule{RuleName: "PrintStackTraceInProduction", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects printStackTrace() calls in code that has a logging framework available."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) {
					return
				}
				name := flatCallExpressionName(file, idx)
				if name != "printStackTrace" {
					return
				}
				if !hasLoggingImport(file) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "printStackTrace() in code with a logging framework; use the logger to record the exception.")
			},
		})
	}
	{
		r := &NonASCIIIdentifierRule{BaseRule: BaseRule{RuleName: "NonAsciiIdentifier", RuleSetName: releaseEngineeringRuleSet, Sev: "info", Desc: "Detects class, function, or property names containing non-ASCII characters."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "function_declaration", "property_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) {
					return
				}
				var name string
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) == "simple_identifier" || file.FlatType(child) == "type_identifier" {
						name = file.FlatNodeText(child)
						break
					}
				}
				if name == "" {
					return
				}
				for _, ch := range name {
					if ch > 127 && !unicode.IsControl(ch) {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							fmt.Sprintf("Non-ASCII character in identifier %q; this may cause issues in non-UTF-8 build environments.", name))
						return
					}
				}
			},
		})
	}
	{
		r := &HardcodedLogTagRule{BaseRule: BaseRule{RuleName: "HardcodedLogTag", RuleSetName: releaseEngineeringRuleSet, Sev: "info", Desc: "Detects Log tag string literals matching the enclosing class name instead of using a companion TAG constant."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMediumHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := flatCallExpressionName(file, idx)
				if !logLevelMethods[name] {
					return
				}
				receiver := flatReceiverNameFromCall(file, idx)
				if receiver != "Log" {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				// First argument is the tag
				for i := 0; i < file.FlatChildCount(args); i++ {
					arg := file.FlatChild(args, i)
					if arg == 0 || file.FlatType(arg) != "value_argument" {
						continue
					}
					text := strings.TrimSpace(file.FlatNodeText(arg))
					unquoted, err := strconv.Unquote(text)
					if err != nil {
						return
					}
					className := flatEnclosingClassName(file, idx)
					if className != "" && unquoted == className {
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							fmt.Sprintf("Log tag %q matches enclosing class name; hoist to a companion `TAG` constant.", unquoted))
					}
					return
				}
			},
		})
	}
	{
		r := &CommentedOutCodeBlockRule{BaseRule: BaseRule{RuleName: "CommentedOutCodeBlock", RuleSetName: releaseEngineeringRuleSet, Sev: "info", Desc: "Detects consecutive lines of commented-out Kotlin code that should be deleted or restored."}, MinLines: 3}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Fix: api.FixIdiomatic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &GradleBuildContainsTodoRule{BaseRule: BaseRule{RuleName: "GradleBuildContainsTodo", RuleSetName: releaseEngineeringRuleSet, Sev: "info", Desc: "Detects TODO comments in build.gradle(.kts) files that may block release readiness."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &CommentedOutImportRule{BaseRule: BaseRule{RuleName: "CommentedOutImport", RuleSetName: releaseEngineeringRuleSet, Sev: "info", Desc: "Detects commented-out import statements that are either dead code or incomplete refactors."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"line_comment"}, Confidence: r.Confidence(), Fix: api.FixIdiomatic, Implementation: r,
			Check: r.checkNode,
		})
	}
	{
		r := &MergeConflictMarkerLeftoverRule{BaseRule: BaseRule{RuleName: "MergeConflictMarkerLeftover", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects unresolved merge conflict markers (<<<, ===, >>>) left in source files."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &HardcodedLocalhostURLRule{BaseRule: BaseRule{RuleName: "HardcodedLocalhostUrl", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects hardcoded localhost or 10.0.2.2 URLs in non-test production source files."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"string_literal"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &TestOnlyImportInProductionRule{BaseRule: BaseRule{RuleName: "TestOnlyImportInProduction", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects test framework imports (JUnit, Mockito, MockK, etc.) in non-test source files."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"import_header"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ConventionPluginDeadCodeRule{BaseRule: BaseRule{RuleName: "ConventionPluginDeadCode", RuleSetName: releaseEngineeringRuleSet, Sev: "info", Desc: "Detects convention plugins under build-logic or buildSrc that are never applied by any module."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ModuleTemplateConformanceRule{BaseRule: BaseRule{RuleName: "ModuleTemplateConformance", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects feature modules that do not conform to the configured module template."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &VisibleForTestingCallerInNonTestRule{BaseRule: BaseRule{RuleName: "VisibleForTestingCallerInNonTest", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects calls to @VisibleForTesting-annotated functions from non-test source files."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &OpenForTestingCallerInNonTestRule{BaseRule: BaseRule{RuleName: "OpenForTestingCallerInNonTest", RuleSetName: releaseEngineeringRuleSet, Sev: "info", Desc: "Detects subclassing of @OpenForTesting types outside test source sets."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &TestFixtureAccessedFromProductionRule{BaseRule: BaseRule{RuleName: "TestFixtureAccessedFromProduction", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects usage of types declared under src/testFixtures/ from production source files."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile | api.NeedsParsedFiles | api.NeedsResolver, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &TimberTreeNotPlantedRule{BaseRule: BaseRule{RuleName: "TimberTreeNotPlanted", RuleSetName: releaseEngineeringRuleSet, Sev: "warning", Desc: "Detects Timber logging usage without any Timber.plant() call in the project."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile | api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			OracleCallTargets: &api.OracleCallTargetFilter{
				CalleeNames: []string{"v", "d", "i", "w", "e", "wtf", "plant"},
				LexicalHintsByCallee: map[string][]string{
					"v":     {"timber.log.Timber", "Timber"},
					"d":     {"timber.log.Timber", "Timber"},
					"i":     {"timber.log.Timber", "Timber"},
					"w":     {"timber.log.Timber", "Timber"},
					"e":     {"timber.log.Timber", "Timber"},
					"wtf":   {"timber.log.Timber", "Timber"},
					"plant": {"timber.log.Timber", "Timber"},
				},
			},
			TypeInfo: api.TypeInfoHint{PreferBackend: api.PreferOracle, Required: true},
			// Uses call target FQNs to detect Timber.v/d/i/w/e calls and
			// Timber.plant(); never reads class declarations.
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			Confidence:             r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
