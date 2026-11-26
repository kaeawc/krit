package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Port of AnnotationRetentionDetector from androidx.annotation:annotation-experimental 1.6.0.

// axlExperimentalMarkers are the meta-annotations that make an annotation class an opt-in marker.
var axlExperimentalMarkers = map[string]bool{
	"kotlin.Experimental": true, "kotlin.RequiresOptIn": true,
	"androidx.annotation.experimental.Experimental": true, "androidx.annotation.RequiresOptIn": true,
}

const axlJavaRequiresOptIn = "androidx.annotation.RequiresOptIn"

// ExperimentalAnnotationRetentionRule ports ExperimentalAnnotationRetention: an opt-in marker whose retention keeps
// it out of the class files (Kotlin markers need BINARY, Java ones the default CLASS).
type ExperimentalAnnotationRetentionRule struct {
	FlatDispatchBase
	BaseRule
}

// WrongRequiresOptInRule ports WrongRequiresOptIn: a Kotlin opt-in marker declared with
// androidx.annotation.RequiresOptIn, which the Kotlin compiler does not enforce.
type WrongRequiresOptInRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WrongRequiresOptInRule) fileMentions() []string { return []string{"RequiresOptIn"} }

func (r *ExperimentalAnnotationRetentionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ExperimentalAnnotationRetention", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WrongRequiresOptInRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WrongRequiresOptIn", RuleSet: "android-lint", DefaultActive: true}
}

// axlAnnotationRef is an annotation applied to a declaration: its resolved name and its argument list node.
type axlAnnotationRef struct {
	fqn  string
	args uint32
}

// axlDeclarationAnnotations returns the annotations on a Kotlin class or Java annotation type declaration, with
// their names resolved through imports (aliases included), the file's package and Kotlin's default imports.
func axlDeclarationAnnotations(file *scanner.File, decl uint32) []axlAnnotationRef {
	var out []axlAnnotationRef
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return nil
	}
	for m := file.FlatFirstChild(mods); m != 0; m = file.FlatNextSib(m) {
		var nameText string
		var args uint32
		switch file.FlatType(m) {
		case "annotation":
			// Kotlin: @Name or @Name(args) (constructor_invocation).
			for c := file.FlatFirstChild(m); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "user_type":
					nameText = file.FlatNodeText(c)
				case "constructor_invocation":
					if ut, ok := file.FlatFindChild(c, "user_type"); ok {
						nameText = file.FlatNodeText(ut)
					}
					if va, ok := file.FlatFindChild(c, "value_arguments"); ok {
						args = va
					}
				}
			}
		case "marker_annotation", "annotation_java":
		}
		if file.Language == scanner.LangJava && (file.FlatType(m) == "marker_annotation" || file.FlatType(m) == "annotation") {
			for c := file.FlatFirstChild(m); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "identifier", "scoped_identifier":
					nameText = file.FlatNodeText(c)
				case "annotation_argument_list":
					args = c
				}
			}
		}
		if nameText == "" {
			continue
		}
		out = append(out, axlAnnotationRef{fqn: axlResolveAnnotationName(file, strings.Join(strings.Fields(nameText), "")), args: args})
	}
	return out
}

// axlResolveAnnotationName resolves a written annotation name to a qualified name.
func axlResolveAnnotationName(file *scanner.File, name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	if file.Language == scanner.LangJava {
		for _, fqn := range []string{axlJavaRequiresOptIn, "androidx.annotation.experimental.Experimental",
			"java.lang.annotation.Retention", "kotlin.RequiresOptIn"} {
			pkg, simple := fqn[:strings.LastIndex(fqn, ".")], fqn[strings.LastIndex(fqn, ".")+1:]
			if simple == name && acrJavaImports(file, pkg, simple) {
				return fqn
			}
		}
		return name
	}
	facts := fileFactsCache().Imports(file)
	if target, ok := facts.Aliases[name]; ok {
		return target
	}
	for w := range facts.Wildcards {
		pkg := strings.TrimSuffix(w, ".*")
		switch pkg + "." + name {
		case axlJavaRequiresOptIn, "androidx.annotation.experimental.Experimental", "java.lang.annotation.Retention":
			return pkg + "." + name
		}
	}
	if acrFileDeclaresTopLevel(file, name) {
		return filePackageName(file) + "." + name
	}
	// Kotlin's default imports: kotlin.* and kotlin.annotation.*.
	switch name {
	case "RequiresOptIn", "Experimental":
		return "kotlin." + name
	case "Retention":
		return "kotlin.annotation.Retention"
	}
	return name
}

// axlAnnotationTypeDecl returns the declaration and its name node when node is a Kotlin annotation class or a Java
// annotation type.
func axlAnnotationTypeDecl(file *scanner.File, node uint32) (nameNode uint32, ok bool) {
	switch file.FlatType(node) {
	case "class_declaration":
		if file.Language != scanner.LangKotlin || !acrFunctionHasModifier(file, node, "annotation") {
			return 0, false
		}
		n := lintClassNameNode(file, node)
		return n, n != 0
	case "annotation_type_declaration":
		n, found := file.FlatFindChild(node, "identifier")
		return n, found
	}
	return 0, false
}

// check mirrors AnnotationRetentionDetector's retention check on an opt-in marker: Kotlin needs an explicit
// kotlin.annotation.Retention(BINARY) (the default is RUNTIME); Java needs CLASS, its default.
func (r *ExperimentalAnnotationRetentionRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	nameNode, ok := axlAnnotationTypeDecl(file, node)
	if !ok {
		return
	}
	anns := axlDeclarationAnnotations(file, node)
	marker := false
	for _, a := range anns {
		if axlExperimentalMarkers[a.fqn] {
			marker = true
		}
	}
	if !marker {
		return
	}
	retentionClass, expected, def := "java.lang.annotation.Retention", "CLASS", "CLASS"
	if file.Language == scanner.LangKotlin {
		retentionClass, expected, def = "kotlin.annotation.Retention", "BINARY", "RUNTIME"
	}
	actual := def
	for _, a := range anns {
		if a.fqn == retentionClass {
			if v := axlAnnotationValueName(file, a.args); v != "" {
				actual = v
			}
		}
	}
	if actual == expected {
		return
	}
	format := func(v string) string {
		if v == def {
			return "default (`" + v + "`)"
		}
		return "`" + v + "`"
	}
	ctx.EmitAtNode(nameNode, "Experimental annotation has "+format(actual)+" retention, should use "+format(expected))
}

// axlAnnotationValueName returns the constant an annotation's single (or `value =`) argument names, such as BINARY
// for AnnotationRetention.BINARY, or "" when it is not a name.
func axlAnnotationValueName(file *scanner.File, args uint32) string {
	if args == 0 {
		return ""
	}
	var expr uint32
	for c := file.FlatFirstChild(args); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "value_argument":
			if label := flatValueArgumentLabel(file, c); label == "" || label == "value" {
				expr = flatValueArgumentExpression(file, c)
			}
		case "element_value_pair":
			if file.FlatChildTextOrEmpty(c, "identifier") == "value" {
				expr = flatLastNamedChild(file, c)
			}
		case "field_access", "identifier":
			expr = c
		}
		if expr != 0 {
			break
		}
	}
	if expr == 0 {
		return ""
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
	if !strings.Contains(text, ".") && file.Language == scanner.LangKotlin {
		// An imported constant may be aliased: `import ...AnnotationRetention.BINARY as B` then @Retention(B).
		if target, ok := fileFactsCache().Imports(file).Aliases[text]; ok {
			text = target
		}
	}
	return text[strings.LastIndex(text, ".")+1:]
}

// check mirrors AnnotationRetentionDetector's Kotlin-usage check: a Kotlin annotation class marked with
// androidx.annotation.RequiresOptIn.
func (r *WrongRequiresOptInRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	nameNode, ok := axlAnnotationTypeDecl(file, node)
	if !ok || file.Language != scanner.LangKotlin {
		return
	}
	for _, a := range axlDeclarationAnnotations(file, node) {
		if a.fqn == axlJavaRequiresOptIn {
			ctx.EmitAtNode(nameNode, "Experimental annotation should use kotlin.RequiresOptIn")
			return
		}
	}
}
