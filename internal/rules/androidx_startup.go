package rules

import (
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of InitializerConstructorDetector and EnsureInitializerMetadataDetector from
// androidx.startup:startup-runtime 1.2.0. Both see every main-source class at once (an Initializer may extend a base
// class declared in another file), so they run in the project scopes, whose results are cached on the whole source
// set.

const (
	axlStartupInitializer         = "androidx.startup.Initializer"
	axlInitializerNoArgMessage    = "Missing Initializer no-arg constructor"
	axlInitializerMetadataMessage = "Every Initializer needs to be accompanied by a corresponding <meta-data> entry in " +
		"the AndroidManifest.xml file."
)

// EnsureInitializerNoArgConstrRule ports EnsureInitializerNoArgConstr: an androidx.startup Initializer that the
// startup runtime cannot instantiate because it has no no-argument constructor.
type EnsureInitializerNoArgConstrRule struct {
	BaseRule
}

// EnsureInitializerMetadataRule ports EnsureInitializerMetadata: an Initializer that no manifest <meta-data> entry
// and no other Initializer's dependencies() reaches, so it never runs.
type EnsureInitializerMetadataRule struct {
	BaseRule
}

func (r *EnsureInitializerNoArgConstrRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "EnsureInitializerNoArgConstr", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Initializer hierarchies are followed through the project's own classes; one extending a library base " +
				"class other than Initializer is not seen.",
		}}
}

func (r *EnsureInitializerMetadataRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "EnsureInitializerMetadata", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"As in Lint, a <meta-data> name must be the fully qualified class name (a relative `.Name` does not count), " +
				"and the value must be androidx.startup or @string/androidx_startup.",
		}}
}

// axlStartupClasses returns the project's Initializer classes in Lint's report order.
func axlStartupClasses(ctx *api.Context) []*workClass {
	_, classes := axlStartupProject(ctx)
	return classes
}

// axlStartupProject returns the project's class index and its Initializer classes in Lint's report order.
func axlStartupProject(ctx *api.Context) (*workProject, []*workClass) {
	files := workProjectFiles(ctx)
	mentions := false
	for _, f := range files {
		if fileFactsCache().Mentions(f, "androidx.startup") {
			mentions = true
			break
		}
	}
	if !mentions {
		return nil, nil
	}
	p := buildWorkProject(files)
	var out []*workClass
	for _, c := range p.classes {
		if c.fqn != axlStartupInitializer && p.inheritsFrom(c, axlStartupInitializer) {
			out = append(out, c)
		}
	}
	return p, out
}

// check mirrors InitializerConstructorDetector: a non-interface Initializer with constructors, none of them without
// parameters (a Java class without constructors and a Kotlin class without a primary constructor have the default
// one; a Kotlin primary constructor whose parameters all have defaults also compiles to a no-argument one).
func (r *EnsureInitializerNoArgConstrRule) check(ctx *api.Context) {
	classes := axlStartupClasses(ctx)
	for _, c := range classes {
		if c.iface || axlClassHasNoArgConstructor(c.file, c.idx) {
			continue
		}
		ctx.Emit(r.finding(c.file, c.idx, axlInitializerNoArgMessage))
	}
}

// finding builds a finding at the start of a declaration.
func (r *EnsureInitializerNoArgConstrRule) finding(file *scanner.File, idx uint32, msg string) scanner.Finding {
	return axlDeclarationFinding(r.BaseRule, file, idx, msg)
}

func axlDeclarationFinding(rule BaseRule, file *scanner.File, idx uint32, msg string) scanner.Finding {
	line, col := file.FlatRow(idx)+1, file.FlatCol(idx)+1
	return rule.Finding(file, line, col, msg)
}

// axlClassHasNoArgConstructor reports whether a Kotlin or Java class can be constructed without arguments.
func axlClassHasNoArgConstructor(file *scanner.File, cls uint32) bool {
	if file.Language == scanner.LangJava {
		body, ok := file.FlatFindChild(cls, "class_body")
		if !ok {
			return true
		}
		ctors := flatDirectChildrenOfType(file, body, "constructor_declaration")
		if len(ctors) == 0 {
			return true
		}
		for _, ctor := range ctors {
			if params, ok := file.FlatFindChild(ctor, "formal_parameters"); ok &&
				len(flatDirectChildrenOfType(file, params, "formal_parameter"))+
					len(flatDirectChildrenOfType(file, params, "spread_parameter")) == 0 {
				return true
			}
		}
		return false
	}
	primary, hasPrimary := file.FlatFindChild(cls, "primary_constructor")
	secondaries := []uint32{}
	if body, ok := file.FlatFindChild(cls, "class_body"); ok {
		secondaries = flatDirectChildrenOfType(file, body, "secondary_constructor")
	}
	if !hasPrimary && len(secondaries) == 0 {
		return true
	}
	if hasPrimary {
		params := acrClassParameters(file, cls)
		allDefault := true
		for _, p := range params {
			if !file.FlatHasChildOfType(p, "=") {
				allDefault = false
			}
		}
		if allDefault {
			return true
		}
		_ = primary
	}
	for _, ctor := range secondaries {
		params, ok := file.FlatFindChild(ctor, "function_value_parameters")
		if !ok {
			continue
		}
		ps := flatDirectChildrenOfType(file, params, "parameter")
		if len(ps) == 0 {
			return true
		}
		// @JvmOverloads with every parameter defaulted compiles to a no-argument overload too.
		if hasAnnotationNamed(file, ctor, "JvmOverloads") {
			allDefault := true
			for _, p := range ps {
				if next := file.FlatNextSib(p); next == 0 || file.FlatType(next) != "=" {
					if !file.FlatHasChildOfType(p, "=") {
						allDefault = false
					}
				}
			}
			if allDefault {
				return true
			}
		}
	}
	return false
}

// check mirrors EnsureInitializerMetadataDetector: every non-interface Initializer must be named by a manifest
// <meta-data android:value="androidx.startup"> entry or by a class literal in some Initializer's dependencies().
func (r *EnsureInitializerMetadataRule) check(ctx *api.Context) {
	project, classes := axlStartupProject(ctx)
	if len(classes) == 0 {
		return
	}
	reachable := map[string]bool{}
	for _, m := range ctx.Manifests {
		// Lint does not visit test-artifact (androidTest, test) or generated build/ manifests.
		if m == nil || isProjectTestFile(ctx, m.Path) || strings.Contains(filepath.ToSlash(projectPath(ctx, m.Path)), "/build/") {
			continue
		}
		axlStartupMetadataNames(m, reachable)
	}
	for _, c := range classes {
		axlDependencyLiterals(project, c, reachable)
	}
	for _, c := range classes {
		if c.iface || reachable[c.fqn] {
			continue
		}
		ctx.Emit(axlDeclarationFinding(r.BaseRule, c.file, c.idx, axlInitializerMetadataMessage))
	}
}

// axlStartupMetadataNames adds the android:name of every <meta-data> whose android:value is androidx.startup.
func axlStartupMetadataNames(m *manifest.Manifest, out map[string]bool) {
	if m == nil || m.Root == nil {
		return
	}
	for _, meta := range workDescendants(m.Root, "meta-data") {
		switch meta.AttrValue("android:value") {
		case "androidx.startup", "@string/androidx_startup":
			out[meta.AttrValue("android:name")] = true
		}
	}
}

// axlDependencyLiterals adds the project class named by each class literal (`X::class.java`, `X.class`) in the
// class's own no-argument dependencies() method (a Java interface's default one included).
func axlDependencyLiterals(p *workProject, c *workClass, out map[string]bool) {
	file := c.file
	body, ok := file.FlatFindChild(c.idx, "class_body")
	if !ok {
		body, ok = file.FlatFindChild(c.idx, "interface_body")
	}
	if !ok {
		return
	}
	pkg := filePackageName(file)
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		var name string
		switch file.FlatType(child) {
		case "function_declaration":
			name = acrDeclarationName(file, child)
		case "method_declaration":
			name = file.FlatChildTextOrEmpty(child, "identifier")
		default:
			continue
		}
		if name != "dependencies" || !axlHasNoParameters(file, child) {
			continue
		}
		file.FlatWalkAllNodes(child, func(n uint32) {
			var typeText string
			switch file.FlatType(n) {
			case "callable_reference":
				// X::class
				if file.FlatNodeTextEquals(flatLastNamedChild(file, n), "class") ||
					strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(n)), "::class") {
					typeText = strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(n)), "::class")
				}
			case "class_literal":
				typeText = strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(n)), ".class")
				typeText = strings.TrimSuffix(typeText, "::class")
			case "navigation_expression":
				// a.b.Child::class parses as a navigation ending in ::class.
				if text := strings.Join(strings.Fields(file.FlatNodeText(n)), ""); strings.HasSuffix(text, "::class") {
					typeText = strings.TrimSuffix(text, "::class")
				}
			}
			if typeText == "" {
				return
			}
			if fqn := axlResolveProjectClass(p, c, file, pkg, typeText); fqn != "" {
				out[fqn] = true
			}
		})
	}
}

// axlHasNoParameters reports whether a Kotlin function or Java method declares no parameters.
func axlHasNoParameters(file *scanner.File, fn uint32) bool {
	if params, ok := file.FlatFindChild(fn, "function_value_parameters"); ok {
		return len(flatDirectChildrenOfType(file, params, "parameter")) == 0
	}
	if params, ok := file.FlatFindChild(fn, "formal_parameters"); ok {
		return len(flatDirectChildrenOfType(file, params, "formal_parameter"))+
			len(flatDirectChildrenOfType(file, params, "spread_parameter")) == 0
	}
	return true
}

// axlResolveProjectClass resolves a class literal written inside class c to the one project class it names, or "":
// a type nested in c or an enclosing class first, then the file's imports, package and on-demand imports, in the
// order workResolveTypeRef lists them (Java prefers a same-package class to an on-demand import).
func axlResolveProjectClass(p *workProject, c *workClass, file *scanner.File, pkg, text string) string {
	var candidates []string
	for owner := c.fqn; len(owner) > len(pkg); {
		candidates = append(candidates, owner+"."+text)
		i := strings.LastIndexByte(owner, '.')
		if i < 0 {
			break
		}
		owner = owner[:i]
	}
	candidates = append(candidates, workResolveTypeRef(file, pkg, text)...)
	for _, fqn := range candidates {
		if p.byFQN[fqn] != nil {
			return fqn
		}
	}
	return ""
}
