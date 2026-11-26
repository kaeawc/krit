package rules

import (
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of ScaffoldPaddingDetector and ColorsDetector from androidx.compose.material:material-android 1.13.0, and
// ScaffoldPaddingDetector and MaterialImportDetector from androidx.compose.material3:material3-android 1.5.0.

const (
	axlMaterialPackage  = "androidx.compose.material"
	axlMaterial3Package = "androidx.compose.material3"
)

var axlMaterial3Coordinates = []librarymodel.Coordinate{
	{Group: "androidx.compose.material3", Name: "material3"},
	{Group: "androidx.compose.material3", Name: "material3-android"},
}

// UnusedMaterialScaffoldPaddingParameterRule ports UnusedMaterialScaffoldPaddingParameter: the content lambda of
// material's Scaffold ignores the padding it is given, so app bars can cover the content.
type UnusedMaterialScaffoldPaddingParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// UnusedMaterial3ScaffoldPaddingParameterRule ports UnusedMaterial3ScaffoldPaddingParameter, the material3
// counterpart of UnusedMaterialScaffoldPaddingParameter.
type UnusedMaterial3ScaffoldPaddingParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// ConflictingOnColorRule ports ConflictingOnColor: background colors of a material Colors with the same value must
// share one 'on' color.
type ConflictingOnColorRule struct {
	FlatDispatchBase
	BaseRule
}

// UsingMaterialAndMaterial3LibrariesRule ports UsingMaterialAndMaterial3Libraries: an androidx.compose.material
// import in code that also uses material3.
type UsingMaterialAndMaterial3LibrariesRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *UnusedMaterialScaffoldPaddingParameterRule) fileMentions() []string {
	return []string{axlMaterialPackage}
}
func (r *UnusedMaterial3ScaffoldPaddingParameterRule) fileMentions() []string {
	return []string{axlMaterial3Package}
}
func (r *ConflictingOnColorRule) fileMentions() []string { return []string{axlMaterialPackage} }
func (r *UsingMaterialAndMaterial3LibrariesRule) fileMentions() []string {
	return []string{axlMaterialPackage}
}

var axlScaffoldLimitations = []string{
	"The content lambda must be passed as the trailing lambda or as `content = { }`; a lambda at the content " +
		"position without a label is not mapped.",
	"An inner lambda without parameters hides an outer `it` only when it is passed to a known one-parameter " +
		"lambda function (forEach, let, map, LazyListScope.items, ...) or a same-file function whose parameter type " +
		"takes one value; Lint resolves every inner lambda.",
	"Scaffold resolves through imports (including `import ... as Alias`) and the file's own package; a Scaffold " +
		"declared in another file of the same package is not seen as shadowing a star import.",
}

func (r *UnusedMaterialScaffoldPaddingParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnusedMaterialScaffoldPaddingParameter", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: axlScaffoldLimitations}
}

func (r *UnusedMaterial3ScaffoldPaddingParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnusedMaterial3ScaffoldPaddingParameter", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: axlScaffoldLimitations}
}

func (r *ConflictingOnColorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ConflictingOnColor", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"A color argument that names a val or var is followed to its same-file initializer (or, for a local var, " +
				"its last assignment before the call); values declared in other files compare by their reference text.",
		}}
}

func (r *UsingMaterialAndMaterial3LibrariesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UsingMaterialAndMaterial3Libraries", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"material3 use is proven by a material3 import in the same file or a material3 dependency declared by the " +
				"build file of the file's own module; one that only arrives through another project module is not seen.",
		}}
}

// axlCheckScaffoldPadding mirrors ScaffoldPaddingDetector for the Scaffold in pkg: each unreferenced parameter of
// the content lambda is reported at the parameter, or at the lambda for an implicit `it`.
func axlCheckScaffoldPadding(ctx *api.Context, pkg string) {
	file, lambda := ctx.File, ctx.Idx
	call, ok := axlContentLambda(file, lambda, "content", true)
	if !ok || !axlCallTargets(file, call, pkg, "Scaffold") {
		return
	}
	for _, p := range axlUnreferencedLambdaParams(file, lambda, 1) {
		ctx.EmitAtNode(p.Node, "Content padding parameter "+p.Name+" is not used")
	}
}

// axlColorsParameters lists the parameters of material's Colors constructor; lightColors and darkColors take the
// same parameters without isLight.
var axlColorsParameters = []string{
	"primary", "primaryVariant", "secondary", "secondaryVariant", "background", "surface", "error", "onPrimary",
	"onSecondary", "onBackground", "onSurface", "onError", "isLight",
}

// axlOnColorFor maps each background color to its 'on' color.
var axlOnColorFor = map[string]string{
	"primary": "onPrimary", "primaryVariant": "onPrimary", "secondary": "onSecondary",
	"secondaryVariant": "onSecondary", "background": "onBackground", "surface": "onSurface", "error": "onError",
}

// check mirrors ColorsDetector: background colors are grouped by their value text, and when one group's 'on'
// colors disagree each explicitly passed 'on' color is reported. A parameter left to its default in the compiled
// library has unknown text, which still differs from every explicit 'on' color.
func (r *ConflictingOnColorRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	name, _, _ := acrCallCallee(file, node)
	params := axlColorsParameters
	switch name {
	case "Colors":
	case "lightColors", "darkColors":
		params = params[:len(params)-1]
	default:
		return
	}
	if !acrCallTargets(file, node, axlMaterialPackage, name) {
		return
	}
	call, ok := lintCallAt(file, node)
	if !ok {
		return
	}
	args := map[string]uint32{}
	texts := map[string]string{}
	for i, p := range params {
		arg := call.Arg(i, p)
		if arg == 0 {
			continue
		}
		args[p] = arg
		texts[p] = axlColorValueText(file, arg, node, 0)
	}
	// Group the explicitly passed background colors by value, in parameter order.
	var groupKeys []string
	groups := map[string][]string{}
	for _, p := range params {
		if _, background := axlOnColorFor[p]; !background {
			continue
		}
		text, ok := texts[p]
		if !ok {
			continue
		}
		if _, seen := groups[text]; !seen {
			groupKeys = append(groupKeys, text)
		}
		groups[text] = append(groups[text], p)
	}
	reported := map[string]bool{}
	for _, key := range groupKeys {
		var onColors []string
		seen := map[string]bool{}
		for _, background := range groups[key] {
			on := axlOnColorFor[background]
			if !seen[on] {
				seen[on] = true
				onColors = append(onColors, on)
			}
		}
		values := map[string]bool{}
		for _, on := range onColors {
			text, explicit := texts[on]
			if !explicit {
				text = "\x00default"
			}
			values[text] = true
		}
		if len(values) < 2 {
			continue
		}
		for _, on := range onColors {
			if arg, explicit := args[on]; explicit && !reported[on] {
				reported[on] = true
				ctx.EmitAtNode(arg, "Conflicting 'on' color for a given background")
			}
		}
	}
}

// axlColorValueText mirrors ParameterWithArgument.resolveToDeclarationText: a reference to a same-file property
// or local variable is replaced by the text of its value (a local var's last assignment before the call, otherwise
// the initializer), recursively; anything else is its own source text.
func axlColorValueText(file *scanner.File, expr, call uint32, depth int) string {
	expr = acrUnwrapParens(file, expr)
	text := file.FlatNodeText(expr)
	if depth > 6 || file.FlatType(expr) != "simple_identifier" {
		return text
	}
	decl := acrValueDeclaration(file, expr, strings.Trim(text, "`"))
	if decl == 0 || file.FlatType(decl) != "property_declaration" {
		return text
	}
	value := acrInitializer(file, decl)
	if acrIsLocalDeclaration(file, decl) && axlPropertyIsVar(file, decl) {
		if last := axlLastAssignmentBefore(file, decl, strings.Trim(text, "`"), call); last != 0 {
			value = last
		}
	}
	if value == 0 {
		return text
	}
	return axlColorValueText(file, value, call, depth+1)
}

// axlPropertyIsVar reports whether a property declaration uses `var`.
func axlPropertyIsVar(file *scanner.File, decl uint32) bool {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "binding_pattern_kind":
			return file.FlatHasChildOfType(child, "var")
		case "variable_declaration":
			return false
		}
	}
	return false
}

// axlLastAssignmentBefore returns the value of the last `name = value` statement after the local declaration and
// before node, in the declaration's own block.
func axlLastAssignmentBefore(file *scanner.File, decl uint32, name string, node uint32) uint32 {
	limit := file.FlatStartByte(node)
	var value uint32
	for stmt := file.FlatNextSib(decl); stmt != 0 && file.FlatStartByte(stmt) < limit; stmt = file.FlatNextSib(stmt) {
		if file.FlatType(stmt) != "assignment" {
			continue
		}
		target := file.FlatFirstChild(stmt)
		if target == 0 || strings.TrimSpace(file.FlatNodeText(target)) != name {
			continue
		}
		value = flatLastNamedChild(file, stmt)
	}
	return value
}

// check mirrors MaterialImportDetector: an import whose reference is androidx.compose.material or contains
// `androidx.compose.material.`, outside the icons, ripple and pullrefresh packages and the ExperimentalMaterialApi
// import, once material3 is in use.
func (r *UsingMaterialAndMaterial3LibrariesRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	ref := axlImportReference(file, node)
	if ref == 0 {
		return
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(ref)), "")
	if text != axlMaterialPackage && !strings.Contains(text, axlMaterialPackage+".") {
		return
	}
	for _, sub := range []string{".icons", ".ripple", ".pullrefresh"} {
		if strings.Contains(text, axlMaterialPackage+sub) {
			return
		}
	}
	if text == axlMaterialPackage+".ExperimentalMaterialApi" || !axlUsesMaterial3(ctx) {
		return
	}
	ctx.EmitAtNode(ref, "Using a material import while also using the material3 library")
}

// axlImportReference returns the imported name of a Kotlin import_header or Java import_declaration, without the
// `.*` of a star import or the alias.
func axlImportReference(file *scanner.File, node uint32) uint32 {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "identifier", "scoped_identifier":
			return child
		}
	}
	return 0
}

// axlUsesMaterial3 reports whether the code uses material3: the file imports from it, or the file's module depends
// on it. Lint runs material3's checks only in modules that depend on material3, so in a multi-module build the
// dependency must come from the build file of the module the source file belongs to.
func axlUsesMaterial3(ctx *api.Context) bool {
	file := ctx.File
	if file.Language == scanner.LangJava {
		if axlJavaImportsFrom(file, axlMaterial3Package) {
			return true
		}
	} else if fileFactsCache().Imports(file).HasAnyPrefix(axlMaterial3Package + ".") {
		return true
	}
	return axlModuleHasDependency(librarymodel.EnsureFacts(ctx.LibraryFacts).Profile, file.Path, axlMaterial3Coordinates)
}

// axlModuleHasDependency reports whether the module owning path declares one of coords. Modules are the
// directories of the build files dependencies were read from; the owner is the deepest one containing path. With
// one module every dependency counts, as for a single-module build, and a dependency of unknown origin always does.
func axlModuleHasDependency(profile librarymodel.ProjectProfile, path string, coords []librarymodel.Coordinate) bool {
	modules := map[string]bool{}
	for _, dep := range profile.Dependencies {
		if dep.Source != "" {
			modules[filepath.Dir(filepath.Clean(dep.Source))] = true
		}
	}
	if len(modules) <= 1 {
		return profile.HasAnyDependency(coords...)
	}
	owner := ""
	clean := filepath.Clean(path)
	for dir := range modules {
		if len(dir) > len(owner) && strings.HasPrefix(clean, dir+string(filepath.Separator)) {
			owner = dir
		}
	}
	if owner == "" {
		return axlUnknownOriginDependency(profile, coords)
	}
	for _, dep := range profile.Dependencies {
		// A dependency of unknown origin still counts build-wide; one from a sibling module does not.
		if dep.Source != "" && filepath.Dir(filepath.Clean(dep.Source)) != owner {
			continue
		}
		for _, c := range coords {
			if dep.Group == c.Group && dep.Name == c.Name {
				return true
			}
		}
	}
	return false
}

// axlJavaImportsFrom reports whether a Java file has an import (single-type, on-demand or static) under pkg.
func axlJavaImportsFrom(file *scanner.File, pkg string) bool {
	refs := filefacts.FileFact(fileFactsCache(), file, slotAxlJavaImports, func() []string {
		var out []string
		file.FlatWalkNodes(0, "import_declaration", func(n uint32) {
			if ref := axlImportReference(file, n); ref != 0 {
				out = append(out, strings.Join(strings.Fields(file.FlatNodeText(ref)), ""))
			}
		})
		return out
	})
	for _, ref := range refs {
		if ref == pkg || strings.HasPrefix(ref, pkg+".") {
			return true
		}
	}
	return false
}

const slotAxlJavaImports = "axlJavaImports"

// axlUnknownOriginDependency reports whether a dependency with no recorded build file matches one of coords.
func axlUnknownOriginDependency(profile librarymodel.ProjectProfile, coords []librarymodel.Coordinate) bool {
	for _, dep := range profile.Dependencies {
		if dep.Source != "" {
			continue
		}
		for _, c := range coords {
			if dep.Group == c.Group && dep.Name == c.Name {
				return true
			}
		}
	}
	return false
}
