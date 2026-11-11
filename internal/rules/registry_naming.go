package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerNamingRules() {
	registerNamingClassNaming()
	registerNamingFunctionNaming()
	registerNamingVariableNaming()
	registerNamingPackageNaming()
	registerNamingEnumNaming()
	registerNamingBooleanPropertyNaming()
	registerNamingConstructorParameterNaming()
	registerNamingForbiddenClassName()
	registerNamingFunctionNameMaxLength()
	registerNamingFunctionNameMinLength()
	registerNamingFunctionParameterNaming()
	registerNamingInvalidPackageDeclaration()
	registerNamingLambdaParameterNaming()
	registerNamingMatchingDeclarationName()
	registerNamingMemberNameEqualsClassName()
	registerNamingNoNameShadowing()
	registerNamingNonBooleanPropertyPrefixedWithIs()
	registerNamingObjectPropertyNaming()
	registerNamingTopLevelPropertyNaming()
	registerNamingVariableMaxLength()
	registerNamingVariableMinLength()
}

func registerNamingClassNaming() {
	r := &ClassNamingRule{BaseRule: BaseRule{RuleName: "ClassNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects class names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration", "object_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if scanner.IsTestFile(file.Path) {
				return
			}
			name := extractIdentifierFlat(file, idx)
			if name == "" {
				return
			}
			// detekt checks the name without its backticks, so `my class` is reported and `Valid` is not.
			if !r.Pattern.MatchString(strings.TrimSuffix(strings.TrimPrefix(name, "`"), "`")) {
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Class and Object names should match the pattern: "+displayPattern(r.Pattern))
			}
		},
	})
}

func registerNamingFunctionNaming() {
	r := &FunctionNamingRule{
		BaseRule: BaseRule{RuleName: "FunctionNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects function names that do not match the expected naming pattern."},
		Pattern:  regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`),
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if funInterfaceMisparsedAsFunction(file, idx) {
				return
			}
			// detekt skips overrides, which keep the name their supertype chose, and factory functions named
			// after their explicit return type. Everything else, backticks and interface members included,
			// is checked.
			if file.FlatHasModifier(idx, "override") || containingClassExcluded(file, idx, r.ExcludeClassPattern) {
				return
			}
			if HasIgnoredAnnotation(file.FlatNodeText(idx), r.IgnoreAnnotated) ||
				(r.CustomPreviewWildcard || len(r.CustomPreviewPrefixes) > 0) &&
					textHasCustomPreviewAnnotation(file.FlatNodeText(idx), customPreviewConfigFromFields(r.CustomPreviewWildcard, r.CustomPreviewPrefixes)) {
				return
			}
			name := extractIdentifierFlat(file, idx)
			if name == "" || r.Pattern.MatchString(name) {
				return
			}
			if functionReturnTypeName(file, idx) == name {
				return
			}
			ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Function names should match the pattern: "+displayPattern(r.Pattern))
		},
	})
}

func registerNamingVariableNaming() {
	r := &VariableNamingRule{BaseRule: BaseRule{RuleName: "VariableNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects variable and member property names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`), PrivateVariablePattern: regexp.MustCompile(`^(_)?[a-z][A-Za-z0-9]*$`), IgnoreOverridden: true}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !variableNamingIsFunctionLocalPropertyFlat(file, idx) {
				return
			}
			name := extractIdentifierFlat(file, idx)
			if name == "" || name == "_" {
				return
			}
			if containingClassExcluded(file, idx, r.ExcludeClassPattern) || (r.IgnoreOverridden && file.FlatHasModifier(idx, "override")) {
				return
			}
			if file.FlatHasModifier(idx, "private") && r.PrivateVariablePattern != nil {
				if !r.PrivateVariablePattern.MatchString(name) {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Private variable names should match the pattern: "+displayPattern(r.PrivateVariablePattern))
				}
				return
			}
			if !r.Pattern.MatchString(name) {
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Variable names should match the pattern: "+displayPattern(r.Pattern))
			}
		},
	})
}

func registerNamingPackageNaming() {
	// Allow digits and underscores in both the first and subsequent
	// package segments. Stricter package patterns disallow digits in the
	// first segment, which false-flags real-world packages that carry a
	// version suffix (`coil3`, `androidx.core2`, `ktor3.client`). The JVM
	// language spec allows digits and underscores in any package segment.
	// as long as it starts with a letter.
	r := &PackageNamingRule{BaseRule: BaseRule{RuleName: "PackageNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects package names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z]+(\.[a-z][A-Za-z0-9]*)*$`)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"package_header"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
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
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Package name should match the pattern: "+displayPattern(r.Pattern))
			}
		},
	})
}

func registerNamingEnumNaming() {
	r := &EnumNamingRule{BaseRule: BaseRule{RuleName: "EnumNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects enum entry names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[A-Z][_a-zA-Z0-9]*$`)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"enum_entry"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := extractIdentifierFlat(file, idx)
			if name != "" && !r.Pattern.MatchString(name) {
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Enum entry names should match the pattern: "+displayPattern(r.Pattern))
			}
		},
	})
}

func registerNamingBooleanPropertyNaming() {
	r := &BooleanPropertyNamingRule{BaseRule: BaseRule{RuleName: "BooleanPropertyNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects Boolean properties that do not start with an allowed prefix like is, has, or are."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixSemantic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if scanner.IsTestFile(file.Path) || !isPublicDeclarationFlat(file, idx) {
				return
			}
			if !isBooleanPropertyFlat(file, idx) {
				return
			}
			name := extractIdentifierFlat(file, idx)
			if name == "" {
				return
			}
			if r.AllowedPattern != nil && r.AllowedPattern.MatchString(name) {
				return
			}
			if !strings.HasPrefix(name, "is") && !strings.HasPrefix(name, "has") && !strings.HasPrefix(name, "are") {
				f := r.Finding(file, declLine(file, idx), declCol(file, idx),
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

func registerNamingConstructorParameterNaming() {
	r := &ConstructorParameterNamingRule{BaseRule: BaseRule{RuleName: "ConstructorParameterNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects constructor val/var parameter names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"primary_constructor", "secondary_constructor"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if containingClassExcluded(file, idx, r.ExcludeClassPattern) {
				return
			}
			check := func(paramNode uint32) {
				if namingParameterHasWireName(file, paramNode) || file.FlatHasModifier(paramNode, "override") {
					return
				}
				name := extractIdentifierFlat(file, paramNode)
				if strings.HasPrefix(name, "`") || name == "" {
					return
				}
				if file.FlatHasModifier(paramNode, "private") && r.PrivateParameterPattern != nil {
					if !r.PrivateParameterPattern.MatchString(name) {
						ctx.EmitAt(file.FlatRow(paramNode)+1, file.FlatCol(paramNode)+1, "Constructor private parameter names should match the pattern: "+displayPattern(r.PrivateParameterPattern))
					}
					return
				}
				if !r.Pattern.MatchString(name) {
					ctx.EmitAt(file.FlatRow(paramNode)+1, file.FlatCol(paramNode)+1, "Constructor parameter names should match the pattern: "+displayPattern(r.Pattern))
				}
			}
			// detekt checks every constructor parameter, with or without val/var.
			if file.FlatType(idx) == "secondary_constructor" {
				if params, _ := file.FlatFindChild(idx, "function_value_parameters"); params != 0 {
					walkFunctionParametersFlat(file, params, check)
				}
				return
			}
			for i := 0; i < file.FlatChildCount(idx); i++ {
				if paramNode := file.FlatChild(idx, i); file.FlatType(paramNode) == "class_parameter" {
					check(paramNode)
				}
			}
		},
	})
}

func registerNamingForbiddenClassName() {
	r := &ForbiddenClassNameRule{BaseRule: BaseRule{RuleName: "ForbiddenClassName", RuleSetName: "naming", Sev: "warning", Desc: "Detects class names that match a configured list of disallowed names."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
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
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), fmt.Sprintf("Class name '%s' is forbidden", name))
			}
		},
	})
}

func registerNamingFunctionNameMaxLength() {
	r := &FunctionNameMaxLengthRule{BaseRule: BaseRule{RuleName: "FunctionNameMaxLength", RuleSetName: "naming", Sev: "warning", Desc: "Detects function names that exceed the configured maximum length."}, MaxLength: 30}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if funInterfaceMisparsedAsFunction(file, idx) {
				return
			}
			// detekt's visitor skips override and operator functions and never descends into function bodies.
			if file.FlatHasModifier(idx, "override") || file.FlatHasModifier(idx, "operator") || hasEnclosingFunction(file, idx) {
				return
			}
			name := strings.Trim(extractIdentifierFlat(file, idx), "`")
			if name != "" && nameLength(name) > r.MaxLength {
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), fmt.Sprintf("Function names should be at most %d characters long.", r.MaxLength))
			}
		},
	})
}

func registerNamingFunctionNameMinLength() {
	r := &FunctionNameMinLengthRule{BaseRule: BaseRule{RuleName: "FunctionNameMinLength", RuleSetName: "naming", Sev: "warning", Desc: "Detects function names that are shorter than the configured minimum length."}, MinLength: 3}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if funInterfaceMisparsedAsFunction(file, idx) {
				return
			}
			// detekt's visitor skips override and operator functions and never descends into function bodies.
			if file.FlatHasModifier(idx, "override") || file.FlatHasModifier(idx, "operator") || hasEnclosingFunction(file, idx) {
				return
			}
			name := strings.Trim(extractIdentifierFlat(file, idx), "`")
			if name != "" && nameLength(name) < r.MinLength {
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), fmt.Sprintf("Function names should be at least %d characters long.", r.MinLength))
			}
		},
	})
}

func registerNamingFunctionParameterNaming() {
	r := &FunctionParameterNamingRule{BaseRule: BaseRule{RuleName: "FunctionParameterNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects function parameter names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if funInterfaceMisparsedAsFunction(file, idx) {
				return
			}
			paramsNode, _ := file.FlatFindChild(idx, "function_value_parameters")
			// An override cannot rename parameters; the supertype owns the names.
			if paramsNode == 0 || file.FlatHasModifier(idx, "override") || containingClassExcluded(file, idx, r.ExcludeClassPattern) {
				return
			}
			walkFunctionParametersFlat(file, paramsNode, func(paramNode uint32) {
				name := extractIdentifierFlat(file, paramNode)
				if strings.HasPrefix(name, "`") {
					return
				}
				// A trailing underscore (`value_`) is the generated-adapter way to avoid a keyword clash.
				if strings.HasSuffix(name, "_") && r.Pattern.MatchString(strings.TrimRight(name, "_")) {
					return
				}
				if name != "" && !r.Pattern.MatchString(name) {
					ctx.EmitAt(file.FlatRow(paramNode)+1, file.FlatCol(paramNode)+1, "Function parameter names should match the pattern: "+displayPattern(r.Pattern))
				}
			})
		},
	})
}

func registerNamingInvalidPackageDeclaration() {
	r := &InvalidPackageDeclarationRule{BaseRule: BaseRule{RuleName: "InvalidPackageDeclaration", RuleSetName: "naming", Sev: "warning", Desc: "Detects package declarations that do not match the file directory structure."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"package_header"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if invalidPackageDeclarationIgnoredPath(file.Path) {
				return
			}
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
			if r.RequireRootInDeclaration && r.RootPackage != "" {
				if pkg != r.RootPackage && !strings.HasPrefix(pkg, r.RootPackage+".") {
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("Package declaration '%s' must start with the configured root package '%s'", pkg, r.RootPackage))
					return
				}
			}
			// Compute the directory-suffix the package implies. When
			// RootPackage is configured, strip it from the package
			// declaration before forming the suffix — the project root
			// maps to that package, so a file at `<root>/foo/Bar.kt`
			// declaring `package <root>.foo` should pass even though
			// `<root>` doesn't appear in the directory layout.
			pkgForPath := pkg
			if r.RootPackage != "" {
				if pkg == r.RootPackage {
					pkgForPath = ""
				} else if strings.HasPrefix(pkg, r.RootPackage+".") {
					pkgForPath = strings.TrimPrefix(pkg, r.RootPackage+".")
				}
			}
			if pkgForPath == "" {
				return
			}
			expectedSuffix := strings.ReplaceAll(pkgForPath, ".", string(filepath.Separator))
			dir := filepath.Dir(file.Path)
			dirNorm := filepath.ToSlash(dir)
			expectedNorm := filepath.ToSlash(expectedSuffix)
			if !strings.HasSuffix(dirNorm, expectedNorm) {
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "The package declaration does not match the actual file location.")
			}
		},
	})
}

func registerNamingLambdaParameterNaming() {
	r := &LambdaParameterNamingRule{BaseRule: BaseRule{RuleName: "LambdaParameterNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects lambda parameter names that do not match the expected naming pattern."}, Pattern: regexp.MustCompile(`^[a-z][A-Za-z0-9]*$|^_$`)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"lambda_literal"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			paramsNode, _ := file.FlatFindChild(idx, "lambda_parameters")
			if paramsNode == 0 {
				return
			}
			check := func(child uint32) {
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
					ctx.EmitAt(file.FlatRow(child)+1, file.FlatCol(child)+1, "Lambda parameter names should match the pattern: "+displayPattern(r.Pattern))
				}
			}
			file.FlatForEachChild(paramsNode, func(child uint32) {
				// Entries of a destructured parameter `{ (a, b) -> }` are named parameters too.
				if file.FlatType(child) == "multi_variable_declaration" {
					file.FlatForEachChild(child, check)
					return
				}
				check(child)
			})
		},
	})
}

func registerNamingMatchingDeclarationName() {
	r := &MatchingDeclarationNameRule{
		BaseRule:    BaseRule{RuleName: "MatchingDeclarationName", RuleSetName: "naming", Sev: "warning", Desc: "Detects files where the single top-level declaration name does not match the filename."},
		MustBeFirst: true,
		MultiplatformTargets: []string{
			"ios", "android", "js", "jvm", "native",
			"iosArm64", "iosX64", "macosX64", "mingwX64", "linuxX64",
		},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"source_file"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			type classDecl struct {
				name string
				idx  uint32
			}
			var nonPrivateClasses []classDecl
			var typeAliasNames []string
			var firstDeclNode uint32
			totalClasses := 0
			for i := 0; i < file.FlatChildCount(idx); i++ {
				child := file.FlatChild(idx, i)
				switch file.FlatType(child) {
				case "ERROR", "lambda_literal", "call_expression", "prefix_expression", "infix_expression":
					// Error recovery (e.g. on `fun interface`) can hide declarations as statements.
					return
				case "class_declaration", "object_declaration":
					if firstDeclNode == 0 {
						firstDeclNode = child
					}
					if file.FlatHasModifier(child, "private") {
						continue
					}
					totalClasses++
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
				case "property_declaration":
					if firstDeclNode == 0 {
						firstDeclNode = child
					}
				}
			}
			// detekt looks at the non-private classes and objects only; private helpers do not count.
			if len(nonPrivateClasses) != 1 || totalClasses != 1 {
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
				ctx.EmitAt(declLine(file, decl.idx), declCol(file, decl.idx), fmt.Sprintf("The file name '%s' does not match the name of the single top-level declaration '%s'.", fileName, decl.name))
			}
		},
	})
}

func registerNamingMemberNameEqualsClassName() {
	r := &MemberNameEqualsClassNameRule{BaseRule: BaseRule{RuleName: "MemberNameEqualsClassName", RuleSetName: "naming", Sev: "warning", Desc: "Detects class members whose name is the same as the containing class name."}, IgnoreOverridden: true}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration", "object_declaration", "companion_object"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			const classMessage = "A member is named after the class. This might result in confusion. Either rename the member or change it to a constructor."
			const objectMessage = "A member is named after the object. This might result in confusion. Please rename the member."
			name, message := extractIdentifierFlat(file, idx), classMessage
			companion := file.FlatType(idx) == "companion_object"
			switch {
			case file.FlatType(idx) == "class_declaration" && file.FlatHasChildOfType(idx, "interface"):
				return
			case companion:
				// A companion's members are compared with the class that holds it.
				name = ""
				if p, ok := file.FlatParent(idx); ok {
					if cls, ok := file.FlatParent(p); ok && file.FlatType(cls) == "class_declaration" {
						name = extractIdentifierFlat(file, cls)
					}
				}
			case file.FlatType(idx) == "object_declaration":
				message = objectMessage
			}
			if name == "" {
				return
			}
			body, _ := file.FlatFindChild(idx, "class_body")
			if body == 0 {
				body, _ = file.FlatFindChild(idx, "enum_class_body")
			}
			if body == 0 {
				return
			}
			forEachDirectClassBodyNodeFlat(file, body, func(member uint32) {
				t := file.FlatType(member)
				if t != "function_declaration" && t != "property_declaration" {
					return
				}
				if !strings.EqualFold(extractIdentifierFlat(file, member), name) {
					return
				}
				if r.IgnoreOverridden && file.FlatHasModifier(member, "override") {
					return
				}
				// A companion factory `fun Foo(): Foo` is the idiom the rule points to.
				if companion && t == "function_declaration" && functionReturnTypeText(file, member) == name {
					return
				}
				ctx.EmitAt(file.FlatRow(member)+1, file.FlatCol(member)+1, message)
			})
		},
	})
}

func registerNamingNoNameShadowing() {
	r := &NoNameShadowingRule{BaseRule: BaseRule{RuleName: "NoNameShadowing", RuleSetName: "naming", Sev: "warning", Desc: "Detects inner declarations that shadow an outer declaration with the same name."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"source_file"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
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

func registerNamingNonBooleanPropertyPrefixedWithIs() {
	r := &NonBooleanPropertyPrefixedWithIsRule{BaseRule: BaseRule{RuleName: "NonBooleanPropertyPrefixedWithIs", RuleSetName: "naming", Sev: "warning", Desc: "Detects non-Boolean properties whose name starts with the is prefix."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
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
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), fmt.Sprintf("Non-Boolean property '%s' should not be prefixed with 'is'", name))
			}
		},
	})
}

func registerNamingObjectPropertyNaming() {
	r := &ObjectPropertyNamingRule{BaseRule: BaseRule{RuleName: "ObjectPropertyNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects property names inside object declarations that do not match the expected naming pattern."}, ConstPattern: regexp.MustCompile(`^[A-Za-z][_A-Za-z0-9]*$`), PropertyPattern: regexp.MustCompile(`^[A-Za-z][_A-Za-z0-9]*$`), PrivatePropertyPattern: regexp.MustCompile(`^(_)?[A-Za-z][_A-Za-z0-9]*$`)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"object_declaration", "companion_object"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
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
						ctx.EmitAt(declLine(file, propNode), declCol(file, propNode), "Object constant names should match the pattern: "+displayPattern(r.ConstPattern))
					}
				} else if file.FlatHasModifier(propNode, "private") && r.PrivatePropertyPattern != nil {
					if !r.PrivatePropertyPattern.MatchString(name) {
						ctx.EmitAt(declLine(file, propNode), declCol(file, propNode), "Private object property names should match the pattern: "+displayPattern(r.PrivatePropertyPattern))
					}
				} else {
					if !r.PropertyPattern.MatchString(name) {
						ctx.EmitAt(declLine(file, propNode), declCol(file, propNode), "Object property names should match the pattern: "+displayPattern(r.PropertyPattern))
					}
				}
			})
		},
	})
}

func registerNamingTopLevelPropertyNaming() {
	r := &TopLevelPropertyNamingRule{BaseRule: BaseRule{RuleName: "TopLevelPropertyNaming", RuleSetName: "naming", Sev: "warning", Desc: "Detects top-level property names that do not match the expected naming pattern."}, ConstPattern: regexp.MustCompile(`^[A-Z][_A-Z0-9]*$`), PropertyPattern: regexp.MustCompile(`^[A-Za-z][_A-Za-z0-9]*$`), PrivatePropertyPattern: regexp.MustCompile(`^_?[A-Za-z][_A-Za-z0-9]*$`)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			parent, ok := file.FlatParent(idx)
			if !ok || file.FlatType(parent) != "source_file" {
				return
			}
			name := extractIdentifierFlat(file, idx)
			if name == "" {
				return
			}
			if strings.HasPrefix(name, "_") &&
				file.FlatHasModifier(idx, "private") {
				return
			}
			if file.FlatHasModifier(idx, "const") {
				if !r.ConstPattern.MatchString(name) {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Top level constant names should match the pattern: "+displayPattern(r.ConstPattern))
				}
			} else if file.FlatHasModifier(idx, "private") && r.PrivatePropertyPattern != nil {
				if !r.PrivatePropertyPattern.MatchString(name) {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Private top level property names should match the pattern: "+displayPattern(r.PrivatePropertyPattern))
				}
			} else {
				if !r.PropertyPattern.MatchString(name) {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Top level property names should match the pattern: "+displayPattern(r.PropertyPattern))
				}
			}
		},
	})
}

func registerNamingVariableMaxLength() {
	r := &VariableMaxLengthRule{BaseRule: BaseRule{RuleName: "VariableMaxLength", RuleSetName: "naming", Sev: "warning", Desc: "Detects variable names that exceed the configured maximum length."}, MaxLength: 64}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := strings.Trim(extractIdentifierFlat(file, idx), "`")
			if name != "" && nameLength(name) > r.MaxLength {
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), fmt.Sprintf("Variable names should be at most %d characters long.", r.MaxLength))
			}
		},
	})
}

func registerNamingVariableMinLength() {
	r := &VariableMinLengthRule{BaseRule: BaseRule{RuleName: "VariableMinLength", RuleSetName: "naming", Sev: "warning", Desc: "Detects variable names that are shorter than the configured minimum length."}, MinLength: 1}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := strings.Trim(extractIdentifierFlat(file, idx), "`")
			if name == "" || name == "_" {
				return
			}
			if nameLength(name) < r.MinLength {
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), fmt.Sprintf("Variable names should be at least %d characters long.", r.MinLength))
			}
		},
	})
}

// displayPattern returns the pattern as the user wrote it, without the anchors the compiler added, as detekt
// prints it in messages.
func displayPattern(re *regexp.Regexp) string {
	if re == nil {
		return ""
	}
	p := strings.TrimSuffix(strings.TrimPrefix(re.String(), "^"), "$")
	if strings.HasPrefix(p, "(?:") && strings.HasSuffix(p, ")") {
		p = p[3 : len(p)-1]
	}
	return p
}

// containingClassExcluded reports a declaration whose nearest enclosing class or object has a name matching
// the excludeClassPattern option.
func containingClassExcluded(file *scanner.File, idx uint32, re *regexp.Regexp) bool {
	if re == nil {
		return false
	}
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration", "companion_object":
			name := extractIdentifierFlat(file, p)
			return name != "" && re.MatchString(name)
		}
	}
	return false
}

// functionReturnTypeName returns the referenced name of a plain (non-nullable) declared return type, the way
// detekt's KtUserType.referencedName does: the last segment of a qualified name, without type arguments.
func functionReturnTypeName(file *scanner.File, fn uint32) string {
	text := functionReturnTypeText(file, fn)
	if text == "" || strings.HasSuffix(text, "?") || strings.HasPrefix(text, "(") || strings.Contains(text, "->") {
		return ""
	}
	if i := strings.IndexByte(text, '<'); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)
	if i := strings.LastIndexByte(text, '.'); i >= 0 {
		text = strings.TrimSpace(text[i+1:])
	}
	return strings.Trim(text, "`")
}

// functionReturnTypeText returns the text of the declared return type of a function, or "".
func functionReturnTypeText(file *scanner.File, fn uint32) string {
	seenParams := false
	for c := file.FlatFirstChild(fn); c != 0; c = file.FlatNextSib(c) {
		t := file.FlatType(c)
		if t == "function_value_parameters" {
			seenParams = true
			continue
		}
		if !seenParams {
			continue
		}
		switch t {
		case "user_type", "nullable_type", "function_type", "parenthesized_type", "type_reference", "dynamic_type":
			return file.FlatNodeText(c)
		case "function_body":
			return ""
		}
	}
	return ""
}

// nameLength is the length of a name in UTF-16 units, which is what Kotlin's String.length counts.
func nameLength(name string) int {
	n := 0
	for _, r := range name {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// funInterfaceMisparsedAsFunction reports a `fun interface X { ... }` that the grammar parses as a function
// declaration with `interface` as its receiver type. Rules about functions must not treat it as one.
func funInterfaceMisparsedAsFunction(file *scanner.File, idx uint32) bool {
	if file.FlatType(idx) != "function_declaration" {
		return false
	}
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "simple_identifier":
			return false
		case "user_type":
			if strings.TrimSpace(file.FlatNodeText(c)) == "interface" {
				return true
			}
		}
	}
	return false
}
