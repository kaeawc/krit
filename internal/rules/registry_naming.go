package rules

import (
	"fmt"
	"github.com/kaeawc/krit/internal/experiment"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"path/filepath"
	"regexp"
	"strings"
)

func registerNamingRules() {

	// --- from naming.go ---
	{
		r := &ClassNamingRule{BaseRule: BaseRule{RuleName: "ClassNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects class names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				if strings.HasPrefix(name, "`") {
					return
				}
				if !r.Pattern.MatchString(name) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Class name '%s' does not match pattern: %s", name, r.Pattern.String()))
				}
			},
		})
	}
	{
		r := &FunctionNamingRule{
			BaseRule:           BaseRule{RuleName: "FunctionNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects function names that do not match the expected naming pattern."},
			Pattern:            regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`),
			AllowBacktickNames: true,
			IgnoreAnnotated:    DefaultIgnoreAnnotated,
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				text := file.FlatNodeText(idx)
				if strings.HasPrefix(strings.TrimLeft(text, " \t"), "fun interface ") ||
					strings.Contains(text, " fun interface ") {
					return
				}
				for i := 0; i < file.FlatChildCount(idx); i++ {
					c := file.FlatChild(idx, i)
					if file.FlatType(c) == "interface" {
						return
					}
				}
				if HasIgnoredAnnotation(text, r.IgnoreAnnotated) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				if r.AllowBacktickNames && strings.HasPrefix(name, "`") {
					return
				}
				operators := map[string]bool{
					"get": true, "set": true, "invoke": true, "plus": true,
					"minus": true, "times": true, "div": true, "rem": true,
					"compareTo": true, "equals": true, "hashCode": true, "toString": true,
				}
				if operators[name] {
					return
				}
				if !r.Pattern.MatchString(name) {
					if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' &&
						functionDeclarationHasExplicitReturnTypeFlat(file, idx) {
						return
					}
					if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' &&
						isTopLevelFunctionFlat(file, idx) &&
						functionHasExpressionBodyReturningCallFlat(file, idx) {
						return
					}
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Function name '%s' does not match pattern: %s", name, r.Pattern.String()))
				}
			},
		})
	}
	{
		r := &VariableNamingRule{BaseRule: BaseRule{RuleName: "VariableNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects local variable names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasAncestorOfType(idx, "function_body") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if strings.HasPrefix(name, "`") {
					return
				}
				if name != "" && !r.Pattern.MatchString(name) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Variable name '%s' does not match pattern: %s", name, r.Pattern.String()))
				}
			},
		})
	}
	{
		// Allow digits and underscores in both the first and subsequent
		// package segments. Detekt's default pattern disallows digits in the
		// first segment, which false-flags real-world packages that carry a
		// version suffix (`coil3`, `androidx.core2`, `ktor3.client`) — 373
		// findings on coil alone came from `package coil3.util`. The JVM
		// language spec allows digits and underscores in any package segment
		// as long as it starts with a letter.
		r := &PackageNamingRule{BaseRule: BaseRule{RuleName: "PackageNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects package names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z][a-zA-Z0-9_]*(\.[a-z][a-zA-Z0-9_]*)*$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"package_header"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var pkg string
				for i := 0; i < file.FlatNamedChildCount(idx); i++ {
					c := file.FlatNamedChild(idx, i)
					t := file.FlatType(c)
					if t == "identifier" || t == "simple_identifier" || t == "navigation_expression" ||
						t == "qualified_identifier" || t == "type_identifier" {
						pkg = strings.TrimSpace(file.FlatNodeText(c))
						break
					}
				}
				if pkg == "" {
					text := file.FlatNodeText(idx)
					if nl := strings.IndexByte(text, '\n'); nl >= 0 {
						text = text[:nl]
					}
					pkg = strings.TrimSpace(strings.TrimPrefix(text, "package "))
				}
				if pkg != "" && !r.Pattern.MatchString(pkg) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Package name '%s' does not match pattern: %s", pkg, r.Pattern.String()))
				}
			},
		})
	}
	{
		r := &EnumNamingRule{BaseRule: BaseRule{RuleName: "EnumNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects enum entry names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[A-Z][_a-zA-Z0-9]*$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"enum_entry"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name != "" && !r.Pattern.MatchString(name) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Enum entry '%s' does not match pattern: %s", name, r.Pattern.String()))
				}
			},
		})
	}
	{
		r := &BooleanPropertyNamingRule{BaseRule: BaseRule{RuleName: "BooleanPropertyNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects Boolean properties that do not start with an allowed prefix like is, has, or are."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, Fix: v2.FixSemantic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isBooleanPropertyFlat(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				if !strings.HasPrefix(name, "is") && !strings.HasPrefix(name, "has") && !strings.HasPrefix(name, "are") {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Boolean property '%s' should start with 'is', 'has', or 'are'", name))
					for i := 0; i < file.FlatChildCount(idx); i++ {
						child := file.FlatChild(idx, i)
						if file.FlatType(child) == "simple_identifier" {
							newName := "is" + strings.ToUpper(name[:1]) + name[1:]
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   int(file.FlatStartByte(child)),
								EndByte:     int(file.FlatEndByte(child)),
								Replacement: newName,
							}
							break
						}
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &ConstructorParameterNamingRule{BaseRule: BaseRule{RuleName: "ConstructorParameterNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects constructor val/var parameter names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"primary_constructor"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				for i := 0; i < file.FlatChildCount(idx); i++ {
					paramNode := file.FlatChild(idx, i)
					if file.FlatType(paramNode) != "class_parameter" {
						continue
					}
					if !file.FlatHasChildOfType(paramNode, "binding_pattern_kind") {
						continue
					}
					name := extractIdentifierFlat(file, paramNode)
					if strings.HasPrefix(name, "`") {
						continue
					}
					if name != "" && !r.Pattern.MatchString(name) {
						ctx.EmitAt(file.FlatRow(paramNode)+1, 1, fmt.Sprintf("Constructor parameter name '%s' does not match pattern: %s", name, r.Pattern.String()))
					}
				}
			},
		})
	}
	{
		r := &ForbiddenClassNameRule{BaseRule: BaseRule{RuleName: "ForbiddenClassName", RuleSetName: "naming", Sev: "warning", Desc: "Detects class names that match a configured list of disallowed names."}, ForbiddenNames: []string{"Manager", "Helper", "Util", "Utils"}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if len(r.ForbiddenNames) == 0 {
					return
				}
				forbidden := make(map[string]bool, len(r.ForbiddenNames))
				for _, n := range r.ForbiddenNames {
					forbidden[n] = true
				}
				name := extractIdentifierFlat(file, idx)
				if name != "" && forbidden[name] {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Class name '%s' is forbidden", name))
				}
			},
		})
	}
	{
		r := &FunctionNameMaxLengthRule{BaseRule: BaseRule{RuleName: "FunctionNameMaxLength", RuleSetName: "naming", Sev: "warning", Desc: "Detects function names that exceed the configured maximum length."}, MaxLength: 30}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name != "" && len(name) > r.MaxLength {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Function name '%s' exceeds maximum length of %d (length: %d)", name, r.MaxLength, len(name)))
				}
			},
		})
	}
	{
		r := &FunctionNameMinLengthRule{BaseRule: BaseRule{RuleName: "FunctionNameMinLength", RuleSetName: "naming", Sev: "warning", Desc: "Detects function names that are shorter than the configured minimum length."}, MinLength: 3}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name != "" && len(name) < r.MinLength {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Function name '%s' is below minimum length of %d (length: %d)", name, r.MinLength, len(name)))
				}
			},
		})
	}
	{
		r := &FunctionParameterNamingRule{BaseRule: BaseRule{RuleName: "FunctionParameterNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects function parameter names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				paramsNode, _ := file.FlatFindChild(idx, "function_value_parameters")
				if paramsNode == 0 {
					return
				}
				walkFunctionParametersFlat(file, paramsNode, func(paramNode uint32) {
					name := extractIdentifierFlat(file, paramNode)
					if strings.HasPrefix(name, "`") {
						return
					}
					if name != "" && !r.Pattern.MatchString(name) {
						ctx.EmitAt(file.FlatRow(paramNode)+1, 1, fmt.Sprintf("Function parameter name '%s' does not match pattern: %s", name, r.Pattern.String()))
					}
				})
			},
		})
	}
	{
		r := &InvalidPackageDeclarationRule{BaseRule: BaseRule{RuleName: "InvalidPackageDeclaration", RuleSetName: "naming", Sev: "warning", Desc: "Detects package declarations that do not match the file directory structure."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"package_header"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var pkg string
				idNode, _ := file.FlatFindChild(idx, "identifier")
				if idNode != 0 {
					pkg = strings.TrimSpace(file.FlatNodeText(idNode))
				} else {
					text := file.FlatNodeText(idx)
					pkg = strings.TrimSpace(strings.TrimPrefix(text, "package "))
					if i := strings.Index(pkg, "\n"); i >= 0 {
						pkg = strings.TrimSpace(pkg[:i])
					}
				}
				if pkg == "" {
					return
				}
				expectedSuffix := strings.ReplaceAll(pkg, ".", string(filepath.Separator))
				dir := filepath.Dir(file.Path)
				dirNorm := filepath.ToSlash(dir)
				expectedNorm := filepath.ToSlash(expectedSuffix)
				if !strings.HasSuffix(dirNorm, expectedNorm) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Package declaration '%s' does not match the file's directory structure", pkg))
				}
			},
		})
	}
	{
		r := &LambdaParameterNamingRule{BaseRule: BaseRule{RuleName: "LambdaParameterNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects lambda parameter names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$|^_$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"lambda_literal"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				paramsNode, _ := file.FlatFindChild(idx, "lambda_parameters")
				if paramsNode == 0 {
					return
				}
				file.FlatForEachChild(paramsNode, func(child uint32) {
					if file.FlatType(child) != "variable_declaration" && file.FlatType(child) != "simple_identifier" {
						return
					}
					name := ""
					if file.FlatType(child) == "simple_identifier" {
						name = file.FlatNodeText(child)
					} else {
						name = extractIdentifierFlat(file, child)
					}
					if strings.HasPrefix(name, "`") {
						return
					}
					if name != "" && !r.Pattern.MatchString(name) {
						ctx.EmitAt(file.FlatRow(child)+1, 1, fmt.Sprintf("Lambda parameter name '%s' does not match pattern: %s", name, r.Pattern.String()))
					}
				})
			},
		})
	}
	{
		r := &MatchingDeclarationNameRule{
			BaseRule:    BaseRule{RuleName: "MatchingDeclarationName", RuleSetName: "naming", Sev: "warning", Desc: "Detects files where the single top-level declaration name does not match the filename."},
			MustBeFirst: true,
			MultiplatformTargets: []string{
				"ios", "android", "js", "jvm", "native",
				"iosArm64", "iosX64", "macosX64", "mingwX64", "linuxX64",
			},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"source_file"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if strings.HasSuffix(file.Path, ".kts") {
					return
				}
				type classDecl struct {
					name string
					idx  uint32
				}
				var nonPrivateClasses []classDecl
				var typeAliasNames []string
				var firstDeclNode uint32
				hasComposableFunc := false
				hasExtensionFunc := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					switch file.FlatType(child) {
					case "class_declaration", "object_declaration":
						if firstDeclNode == 0 {
							firstDeclNode = child
						}
						if file.FlatHasModifier(child, "private") {
							continue
						}
						name := extractIdentifierFlat(file, child)
						if name != "" {
							nonPrivateClasses = append(nonPrivateClasses, classDecl{name, child})
						}
					case "type_alias":
						if firstDeclNode == 0 {
							firstDeclNode = child
						}
						name := extractIdentifierFlat(file, child)
						if name != "" {
							typeAliasNames = append(typeAliasNames, name)
						}
					case "function_declaration":
						if firstDeclNode == 0 {
							firstDeclNode = child
						}
						if flatHasAnnotationNamed(file, child, "Composable") {
							hasComposableFunc = true
						}
						if isExtensionFunctionDeclFlat(file, child) {
							hasExtensionFunc = true
						}
					case "property_declaration":
						if firstDeclNode == 0 {
							firstDeclNode = child
						}
					}
				}
				if len(nonPrivateClasses) != 1 {
					return
				}
				if hasComposableFunc {
					return
				}
				if hasExtensionFunc {
					return
				}
				decl := nonPrivateClasses[0]
				if r.MustBeFirst && firstDeclNode != 0 && decl.idx != firstDeclNode {
					return
				}
				fileName := fileNameWithoutSuffix(file.Path, r.MultiplatformTargets)
				for _, ta := range typeAliasNames {
					if ta == fileName {
						return
					}
				}
				bareFileName := fileName
				hadDotQualifier := false
				if dot := strings.Index(bareFileName, "."); dot > 0 {
					bareFileName = bareFileName[:dot]
					hadDotQualifier = true
				}
				if strings.EqualFold(fileName, "main") {
					return
				}
				if len(fileName) > 0 && fileName[0] >= 'a' && fileName[0] <= 'z' {
					return
				}
				if hadDotQualifier && len(bareFileName) >= 3 &&
					strings.HasPrefix(decl.name, bareFileName) {
					return
				}
				for _, suffix := range []string{"Android", "Jvm", "Js", "WasmJs", "Native"} {
					if decl.name == fileName+suffix || decl.name == bareFileName+suffix {
						return
					}
				}
				if strings.HasSuffix(fileName, "Test") && decl.name == fileName+"s" {
					return
				}
				if strings.Contains(filepath.ToSlash(file.Path), "/src/") &&
					strings.Contains(filepath.ToSlash(file.Path), "Main/") &&
					decl.name != fileName {
					return
				}
				if fileName != decl.name {
					ctx.EmitAt(file.FlatRow(decl.idx)+1, 1, fmt.Sprintf("File name '%s' does not match the single top-level declaration '%s'", fileName, decl.name))
				}
			},
		})
	}
	{
		r := &MemberNameEqualsClassNameRule{BaseRule: BaseRule{RuleName: "MemberNameEqualsClassName", RuleSetName: "naming", Sev: "warning", Desc: "Detects class members whose name is the same as the containing class name."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				className := extractIdentifierFlat(file, idx)
				if className == "" {
					return
				}
				classBody, _ := file.FlatFindChild(idx, "class_body")
				if classBody == 0 {
					return
				}
				for i := 0; i < file.FlatChildCount(classBody); i++ {
					child := file.FlatChild(classBody, i)
					switch file.FlatType(child) {
					case "function_declaration", "property_declaration":
						memberName := extractIdentifierFlat(file, child)
						if memberName == className {
							ctx.EmitAt(file.FlatRow(child)+1, 1, fmt.Sprintf("Member '%s' has the same name as the containing class", memberName))
						}
					}
				}
			},
		})
	}
	{
		r := &NoNameShadowingRule{BaseRule: BaseRule{RuleName: "NoNameShadowing", RuleSetName: "naming", Sev: "warning", Desc: "Detects inner declarations that shadow an outer declaration with the same name."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"source_file"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var findings []scanner.Finding
				sctx := &shadowScanCtx{
					file:     file,
					findings: &findings,
					seen:     make(map[noNameShadowFindingKey]bool),
				}
				r.walkScopeFlat(idx, sctx, nil, nil)
				for _, f := range findings {
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &NonBooleanPropertyPrefixedWithIsRule{BaseRule: BaseRule{RuleName: "NonBooleanPropertyPrefixedWithIs", RuleSetName: "naming", Sev: "warning", Desc: "Detects non-Boolean properties whose name starts with the is prefix."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name == "" || !strings.HasPrefix(name, "is") {
					return
				}
				if isBooleanPropertyFlat(file, idx) {
					return
				}
				text := file.FlatNodeText(idx)
				if strings.Contains(text, ": ") {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Non-Boolean property '%s' should not be prefixed with 'is'", name))
				}
			},
		})
	}
	{
		r := &ObjectPropertyNamingRule{BaseRule: BaseRule{RuleName: "ObjectPropertyNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects property names inside object declarations that do not match the expected naming pattern."}, ConstPattern: regexp.MustCompile(`^[A-Z][_A-Z0-9]*$`), PropertyPattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"object_declaration", "companion_object"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				classBody, _ := file.FlatFindChild(idx, "class_body")
				if classBody == 0 {
					return
				}
				allowBacking := experiment.Enabled("naming-allow-backing-properties")
				file.FlatForEachChild(classBody, func(propNode uint32) {
					if file.FlatType(propNode) != "property_declaration" {
						return
					}
					name := extractIdentifierFlat(file, propNode)
					if name == "" {
						return
					}
					if allowBacking && strings.HasPrefix(name, "_") &&
						file.FlatHasModifier(propNode, "private") {
						return
					}
					if file.FlatHasModifier(propNode, "const") {
						if !r.ConstPattern.MatchString(name) {
							ctx.EmitAt(file.FlatRow(propNode)+1, 1, fmt.Sprintf("Object const property '%s' does not match pattern: %s", name, r.ConstPattern.String()))
						}
					} else {
						if !r.PropertyPattern.MatchString(name) {
							ctx.EmitAt(file.FlatRow(propNode)+1, 1, fmt.Sprintf("Object property '%s' does not match pattern: %s", name, r.PropertyPattern.String()))
						}
					}
				})
			},
		})
	}
	{
		r := &TopLevelPropertyNamingRule{BaseRule: BaseRule{RuleName: "TopLevelPropertyNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects top-level property names that do not match the expected naming pattern."}, ConstPattern: regexp.MustCompile(`^[A-Z][_A-Z0-9]*$`), PropertyPattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				parent, ok := file.FlatParent(idx)
				if !ok || file.FlatType(parent) != "source_file" {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				if experiment.Enabled("naming-allow-backing-properties") &&
					strings.HasPrefix(name, "_") &&
					file.FlatHasModifier(idx, "private") {
					return
				}
				if file.FlatHasModifier(idx, "const") {
					if !r.ConstPattern.MatchString(name) {
						ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Top-level const property '%s' does not match pattern: %s", name, r.ConstPattern.String()))
					}
				} else {
					if !r.PropertyPattern.MatchString(name) {
						ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Top-level property '%s' does not match pattern: %s", name, r.PropertyPattern.String()))
					}
				}
			},
		})
	}
	{
		r := &VariableMaxLengthRule{BaseRule: BaseRule{RuleName: "VariableMaxLength", RuleSetName: "naming", Sev: "warning", Desc: "Detects variable names that exceed the configured maximum length."}, MaxLength: 64}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasAncestorOfType(idx, "function_body") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name != "" && len(name) > r.MaxLength {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Variable name '%s' exceeds maximum length of %d (length: %d)", name, r.MaxLength, len(name)))
				}
			},
		})
	}
	{
		r := &VariableMinLengthRule{BaseRule: BaseRule{RuleName: "VariableMinLength", RuleSetName: "naming", Sev: "warning", Desc: "Detects variable names that are shorter than the configured minimum length."}, MinLength: 2}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasAncestorOfType(idx, "function_body") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				if name == "_" {
					return
				}
				if len(name) < r.MinLength {
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Variable name '%s' is below minimum length of %d (length: %d)", name, r.MinLength, len(name)))
				}
			},
		})
	}
}
