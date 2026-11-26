package rules

import (
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Port of ExperimentalDetector's usage checks (UnsafeOptInUsageError / UnsafeOptInUsageWarning) from
// androidx.annotation:annotation-experimental 1.6.0, for opt-in markers declared in the project. It runs once per
// scan over every parsed file (markers, marked declarations and their usages live in different files), so its
// results are cached on the whole source set.

const (
	axlKotlinOptIn          = "kotlin.OptIn"
	axlKotlinUseExperiment  = "kotlin.UseExperimental"
	axlJavaOptIn            = "androidx.annotation.OptIn"
	axlJavaUseExperimental  = "androidx.annotation.experimental.UseExperimental"
	axlJavaExperimentalMark = "androidx.annotation.experimental.Experimental"
)

// UnsafeOptInUsageErrorRule ports UnsafeOptInUsageError: a use of an API marked with an error-level opt-in marker
// without opting in.
type UnsafeOptInUsageErrorRule struct {
	BaseRule
}

// UnsafeOptInUsageWarningRule ports UnsafeOptInUsageWarning: a use of an API marked with a warning-level opt-in
// marker without opting in.
type UnsafeOptInUsageWarningRule struct {
	BaseRule
}

var axlOptInUsageLimitations = []string{
	"Only opt-in markers declared in the project are known; APIs marked in libraries (CameraX, Media3, ...) need " +
		"their class files, which source analysis does not read.",
	"Usages are calls, constructor calls and member calls whose target resolves through imports, the file's package " +
		"and same-file declarations; the lint.xml `opt-in` option is not read.",
	"As in Lint, Kotlin code is only checked against androidx (Java) markers, since the Kotlin compiler enforces " +
		"kotlin.RequiresOptIn markers itself.",
}

func (r *UnsafeOptInUsageErrorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsafeOptInUsageError", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: axlOptInUsageLimitations}
}

func (r *UnsafeOptInUsageWarningRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsafeOptInUsageWarning", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: axlOptInUsageLimitations}
}

// axlOptInMarker is an opt-in marker annotation declared in the project.
type axlOptInMarker struct {
	fqn     string
	java    bool // androidx.annotation.RequiresOptIn / Experimental, rather than the kotlin ones
	warning bool
	message string
}

// axlOptInModel is the project's markers and the declarations they mark, by FQN.
type axlOptInModel struct {
	markers map[string]*axlOptInMarker
	// marked maps a declaration FQN (pkg.fn, pkg.Class, pkg.Class.member, pkg.FileKt.fn) to its markers.
	marked map[string][]string
	// names holds the simple names of the marked declarations, so a call with another name is skipped unresolved.
	names map[string]bool
}

// axlOptInAnnotated is a declaration with annotations, its own or an enclosing class's, by FQN.
type axlOptInAnnotated struct {
	fqns        []string // pkg.fn, pkg.Class, pkg.Class.member, and pkg.FileKt.fn for a Kotlin top-level declaration
	annotations []string // resolved annotation FQNs, the enclosing classes' included
}

const (
	slotAxlOptInMarkers    = "axlOptInMarkers"
	slotAxlOptInAnnotated  = "axlOptInAnnotated"
	slotAxlJavaImportNames = "axlJavaImportNames"
)

// axlBuildOptInModel merges the files' cached marker and annotated-declaration facts, so the Error and Warning rules
// share the per-file work and each fact depends only on its own file.
func axlBuildOptInModel(files []*scanner.File) *axlOptInModel {
	m := &axlOptInModel{markers: map[string]*axlOptInMarker{}, marked: map[string][]string{}, names: map[string]bool{}}
	for _, file := range files {
		if !fileFactsCache().Mentions(file, "RequiresOptIn") && !fileFactsCache().Mentions(file, "Experimental") {
			continue
		}
		for _, marker := range axlFileOptInMarkers(file) {
			m.markers[marker.fqn] = marker
		}
	}
	if len(m.markers) == 0 {
		return m
	}
	for _, file := range files {
		for _, decl := range axlFileOptInAnnotated(file) {
			var marks []string
			for _, a := range decl.annotations {
				if m.markers[a] != nil {
					marks = append(marks, a)
				}
			}
			if len(marks) == 0 {
				continue
			}
			for _, fqn := range decl.fqns {
				m.marked[fqn] = marks
				m.names[fqn[strings.LastIndex(fqn, ".")+1:]] = true
			}
		}
	}
	return m
}

// axlFileOptInMarkers returns the opt-in markers a file declares: annotation classes meta-annotated with a
// RequiresOptIn / Experimental annotation.
func axlFileOptInMarkers(file *scanner.File) []*axlOptInMarker {
	return filefacts.FileFact(fileFactsCache(), file, slotAxlOptInMarkers, func() []*axlOptInMarker {
		var out []*axlOptInMarker
		pkg := filePackageName(file)
		for _, kind := range []string{"class_declaration", "annotation_type_declaration"} {
			file.FlatWalkNodes(0, kind, func(decl uint32) {
				nameNode, ok := axlAnnotationTypeDecl(file, decl)
				if !ok {
					return
				}
				for _, a := range axlDeclarationAnnotations(file, decl) {
					if !axlExperimentalMarkers[a.fqn] {
						continue
					}
					marker := &axlOptInMarker{fqn: axlQualify(pkg, file.FlatNodeText(nameNode)),
						java: a.fqn == axlJavaRequiresOptIn || a.fqn == axlJavaExperimentalMark}
					marker.warning = axlAnnotationArgumentName(file, a.args, "level") == "WARNING"
					marker.message = axlAnnotationStringArgument(file, a.args, "message")
					out = append(out, marker)
				}
			})
		}
		return out
	})
}

// axlFileOptInAnnotated returns a file's declarations that carry annotations, their own or an enclosing class's.
func axlFileOptInAnnotated(file *scanner.File) []axlOptInAnnotated {
	return filefacts.FileFact(fileFactsCache(), file, slotAxlOptInAnnotated, func() []axlOptInAnnotated {
		var out []axlOptInAnnotated
		pkg := filePackageName(file)
		facade := axlFileFacade(file)
		own := func(decl uint32) []string {
			var names []string
			for _, a := range axlDeclarationAnnotations(file, decl) {
				names = append(names, axlResolveProjectName(file, a.fqn))
			}
			return names
		}
		var walk func(node uint32, owner string, inherited []string)
		walk = func(node uint32, owner string, inherited []string) {
			for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
				t := file.FlatType(c)
				switch t {
				case "class_declaration", "object_declaration", "interface_declaration":
					name := lintClassNameNode(file, c)
					if name == 0 {
						if n, ok := file.FlatFindChild(c, "type_identifier"); ok {
							name = n
						}
					}
					if name == 0 {
						continue
					}
					fqn := axlQualify(owner, file.FlatNodeText(name))
					annotations := append(append([]string(nil), inherited...), own(c)...)
					if len(annotations) > 0 {
						out = append(out, axlOptInAnnotated{fqns: []string{fqn}, annotations: annotations})
					}
					if body, ok := file.FlatFindChild(c, "class_body"); ok {
						walk(body, fqn, annotations)
					}
				case "function_declaration", "property_declaration", "method_declaration":
					var name string
					switch t {
					case "function_declaration":
						name = acrDeclarationName(file, c)
					case "property_declaration":
						name = acrPropertyName(file, c)
					default:
						name = file.FlatChildTextOrEmpty(c, "identifier")
					}
					annotations := append(append([]string(nil), inherited...), own(c)...)
					if name == "" || len(annotations) == 0 {
						continue
					}
					fqns := []string{axlQualify(owner, name)}
					if owner == pkg && facade != "" {
						fqns = append(fqns, axlQualify(pkg, facade)+"."+name)
					}
					out = append(out, axlOptInAnnotated{fqns: fqns, annotations: annotations})
				case "class_body", "interface_body", "program":
					walk(c, owner, inherited)
				}
			}
		}
		walk(0, pkg, nil)
		return out
	})
}

// axlQualify joins a package (or owner) and a name.
func axlQualify(owner, name string) string {
	name = strings.Trim(name, "`")
	if owner == "" {
		return name
	}
	return owner + "." + name
}

// axlFileFacade returns the JVM facade class name of a Kotlin file's top-level declarations (Api.kt -> ApiKt), or "".
func axlFileFacade(file *scanner.File) string {
	if file.Language != scanner.LangKotlin {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(file.Path), ".kt")
	if base == "" {
		return ""
	}
	return strings.ToUpper(base[:1]) + base[1:] + "Kt"
}

// axlResolveProjectName qualifies a simple name the way axlResolveAnnotationName leaves it (unqualified when it is
// not a known annotation): through explicit imports, then the file's package.
func axlResolveProjectName(file *scanner.File, name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	if file.Language == scanner.LangJava {
		if found, ok := axlJavaImportNames(file)[name]; ok {
			return found
		}
		return axlQualify(filePackageName(file), name)
	}
	if target, ok := fileFactsCache().Imports(file).Aliases[name]; ok {
		return target
	}
	return axlQualify(filePackageName(file), name)
}

// axlJavaImportNames maps the last segment of each Java import to the imported name (the last such import wins).
func axlJavaImportNames(file *scanner.File) map[string]string {
	return filefacts.FileFact(fileFactsCache(), file, slotAxlJavaImportNames, func() map[string]string {
		out := map[string]string{}
		file.FlatWalkNodes(0, "import_declaration", func(n uint32) {
			text := strings.Join(strings.Fields(file.FlatNodeText(axlImportReference(file, n))), "")
			if dot := strings.LastIndex(text, "."); dot >= 0 {
				out[text[dot+1:]] = text
			}
		})
		return out
	})
}

// axlAnnotationArgumentName returns the rightmost name of an annotation argument (`level = RequiresOptIn.Level.WARNING`
// gives WARNING), or "".
func axlAnnotationArgumentName(file *scanner.File, args uint32, label string) string {
	expr := axlAnnotationArgument(file, args, label)
	if expr == 0 {
		return ""
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
	return text[strings.LastIndex(text, ".")+1:]
}

// axlAnnotationStringArgument returns a string literal annotation argument's value, or "".
func axlAnnotationStringArgument(file *scanner.File, args uint32, label string) string {
	expr := axlAnnotationArgument(file, args, label)
	if expr == 0 {
		return ""
	}
	if v, ok := lintEvalStringStrict(file, expr); ok {
		return v
	}
	return ""
}

// axlAnnotationArgument returns the expression of a labeled Kotlin or Java annotation argument.
func axlAnnotationArgument(file *scanner.File, args uint32, label string) uint32 {
	if args == 0 {
		return 0
	}
	for c := file.FlatFirstChild(args); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "value_argument":
			if flatValueArgumentLabel(file, c) == label {
				return flatValueArgumentExpression(file, c)
			}
		case "element_value_pair":
			if file.FlatChildTextOrEmpty(c, "identifier") == label {
				return flatLastNamedChild(file, c)
			}
		}
	}
	return 0
}

// axlOptInCheck runs the usage check for one level.
func axlOptInCheck(ctx *api.Context, rule BaseRule, warning bool) {
	var files []*scanner.File
	for _, f := range ctx.ParsedFiles {
		if f != nil && f.FlatTree != nil && (f.Language == scanner.LangKotlin || f.Language == scanner.LangJava) {
			files = append(files, f)
		}
	}
	mentions := false
	for _, f := range files {
		if fileFactsCache().Mentions(f, "RequiresOptIn") || fileFactsCache().Mentions(f, "Experimental") {
			mentions = true
			break
		}
	}
	if !mentions {
		return
	}
	m := axlBuildOptInModel(files)
	if len(m.marked) == 0 {
		return
	}
	for _, file := range files {
		if scanner.IsTestFile(file.Path) || !axlFileMentionsAny(file, m.names) {
			continue
		}
		for _, kind := range []string{"call_expression", "method_invocation", "object_creation_expression"} {
			file.FlatWalkNodes(0, kind, func(call uint32) {
				if kind == "call_expression" && acrIsInnerCallOfTrailingLambda(file, call) {
					return
				}
				if !m.names[axlOptInCallName(file, call)] {
					return
				}
				target, nameNode := axlOptInCallTarget(file, call)
				for _, marker := range m.marked[target] {
					mk := m.markers[marker]
					if mk.warning != warning || (file.Language == scanner.LangKotlin && !mk.java) {
						continue
					}
					if axlOptInAccepted(file, call, mk) {
						continue
					}
					msg := mk.message
					if msg == "" {
						msg = "This declaration is opt-in and its usage should be marked with `@" + mk.fqn +
							"` or `@OptIn(markerClass = " + mk.fqn + ".class)`"
					}
					line, col := file.FlatRow(nameNode)+1, file.FlatCol(nameNode)+1
					ctx.Emit(rule.Finding(file, line, col, msg))
				}
			})
		}
	}
}

// axlFileMentionsAny reports whether a file's text contains one of the names.
func axlFileMentionsAny(file *scanner.File, names map[string]bool) bool {
	for name := range names {
		if fileFactsCache().Mentions(file, name) {
			return true
		}
	}
	return false
}

// axlOptInCallName returns the simple name a call uses: the callee, the invoked method, or the constructed class.
func axlOptInCallName(file *scanner.File, call uint32) string {
	switch file.FlatType(call) {
	case "call_expression":
		name, _, _ := acrCallCallee(file, call)
		return name
	case "method_invocation", "object_creation_expression":
		c, ok := lintCallAt(file, call)
		if !ok {
			return ""
		}
		if file.FlatType(call) == "method_invocation" {
			return c.Name
		}
		return c.TypeText[strings.LastIndex(c.TypeText, ".")+1:]
	}
	return ""
}

// axlOptInCallTarget resolves a call to the FQN of the declaration it uses and its name node: a top-level function or
// class (constructor), a member of a class or object named by its receiver, or a member of a value whose type is
// a same-file-resolvable class.
func axlOptInCallTarget(file *scanner.File, call uint32) (string, uint32) {
	switch file.FlatType(call) {
	case "call_expression":
		name, nameNode, hasReceiver := acrCallCallee(file, call)
		if name == "" {
			return "", 0
		}
		if !hasReceiver {
			// Kotlin resolves an unqualified call to a member of an enclosing class before a top-level function.
			if owner := axlEnclosingMemberOwner(file, call, name); owner != "" {
				return owner + "." + name, nameNode
			}
			if acrSingleTopLevelFunction(file, name) != 0 && lintSameFileClass(file, name) == 0 {
				return axlQualify(filePackageName(file), name), nameNode
			}
			return axlResolveProjectName(file, name), nameNode
		}
		receiver := file.FlatFirstChild(acrCalleeExpression(file, call))
		owner := axlOptInReceiverType(file, receiver)
		if owner == "" {
			return "", 0
		}
		return owner + "." + name, nameNode
	case "method_invocation":
		c, ok := lintCallAt(file, call)
		if !ok || c.ReceiverNode == 0 {
			return "", 0
		}
		owner := axlOptInReceiverType(file, c.ReceiverNode)
		if owner == "" {
			return "", 0
		}
		nameNode := call
		for ch := file.FlatFirstChild(call); ch != 0; ch = file.FlatNextSib(ch) {
			if file.FlatType(ch) == "identifier" && file.FlatNodeTextEquals(ch, c.Name) {
				nameNode = ch
			}
		}
		return owner + "." + c.Name, nameNode
	case "object_creation_expression":
		c, ok := lintCallAt(file, call)
		if !ok {
			return "", 0
		}
		return axlResolveProjectName(file, c.TypeText), call
	}
	return "", 0
}

// axlEnclosingMemberOwner returns the FQN of the nearest enclosing class or object that declares a member function
// named name, or "" when there is none (or it is inside an anonymous object, whose members have no FQN). Only inner
// classes see their outer class's members.
func axlEnclosingMemberOwner(file *scanner.File, node uint32, name string) string {
	owner := uint32(0)
	for p, ok := file.FlatParent(node); ok && owner == 0; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration":
			if axlClassDeclaresFunction(file, p, name) {
				owner = p
			} else if !acrFunctionHasModifier(file, p, "inner") {
				return ""
			}
		case "object_literal":
			if body, ok := file.FlatFindChild(p, "class_body"); ok && axlBodyDeclaresFunction(file, body, name) {
				return ""
			}
		}
	}
	if owner == 0 {
		return ""
	}
	var names []string
	for p, ok := owner, true; ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration":
			n := lintClassNameNode(file, p)
			if n == 0 {
				return ""
			}
			names = append([]string{strings.Trim(file.FlatNodeText(n), "`")}, names...)
		case "object_literal", "function_declaration", "lambda_literal":
			return "" // a local or anonymous class has no FQN
		}
	}
	return axlQualify(filePackageName(file), strings.Join(names, "."))
}

// axlClassDeclaresFunction reports whether a class or object body declares a function named name.
func axlClassDeclaresFunction(file *scanner.File, class uint32, name string) bool {
	body, ok := file.FlatFindChild(class, "class_body")
	return ok && axlBodyDeclaresFunction(file, body, name)
}

func axlBodyDeclaresFunction(file *scanner.File, body uint32, name string) bool {
	for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "function_declaration" && acrDeclarationName(file, c) == name {
			return true
		}
	}
	return false
}

// axlOptInReceiverType returns the FQN of the class a receiver names (`Holder`, `ApiKt`) or constructs
// (`MarkedClass()`), or of a value's declared class type.
func axlOptInReceiverType(file *scanner.File, receiver uint32) string {
	receiver = acrUnwrapParens(file, receiver)
	switch file.FlatType(receiver) {
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(receiver)
		if decl := acrValueDeclaration(file, receiver, name); decl != 0 {
			if t := acrDeclaredType(file, decl); t != 0 {
				return axlResolveProjectName(file, acrTypeNodeName(file, t))
			}
			return ""
		}
		if acrFirstRuneIsUpper(name) {
			return axlResolveProjectName(file, name)
		}
	case "call_expression":
		name, _, hasReceiver := acrCallCallee(file, receiver)
		if !hasReceiver && acrFirstRuneIsUpper(name) {
			return axlResolveProjectName(file, name)
		}
	}
	return ""
}

// axlOptInAccepted mirrors isExperimentalityAccepted: an enclosing declaration (or the file, through @file:OptIn)
// is annotated with the marker itself, or opts in to it with an OptIn annotation the marker's kind accepts.
func axlOptInAccepted(file *scanner.File, node uint32, mk *axlOptInMarker) bool {
	accepted := map[string]bool{axlJavaOptIn: true, axlJavaUseExperimental: true}
	if !mk.java {
		accepted[axlKotlinOptIn] = true
		accepted[axlKotlinUseExperiment] = true
	}
	check := func(decl uint32) bool {
		for _, a := range axlDeclarationAnnotations(file, decl) {
			fqn := axlResolveOptInName(file, a.fqn)
			if axlResolveProjectName(file, fqn) == mk.fqn {
				return true
			}
			if accepted[fqn] && axlOptInNamesMarker(file, a.args, mk.fqn) {
				return true
			}
		}
		return false
	}
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "class_declaration", "object_declaration", "property_declaration",
			"method_declaration", "constructor_declaration", "interface_declaration", "secondary_constructor":
			if check(p) {
				return true
			}
		}
	}
	if file.Language == scanner.LangKotlin {
		for c := file.FlatFirstChild(0); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "file_annotation" && strings.Contains(file.FlatNodeText(c), "OptIn") &&
				axlOptInNamesMarker(file, c, mk.fqn) {
				return true
			}
		}
	}
	return false
}

// axlResolveOptInName resolves an annotation name for the opt-in annotations (OptIn defaults to kotlin.OptIn in
// Kotlin unless androidx.annotation.OptIn is imported).
func axlResolveOptInName(file *scanner.File, name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	switch name {
	case "OptIn", "UseExperimental":
		if file.Language == scanner.LangJava {
			if acrJavaImports(file, "androidx.annotation", name) {
				return "androidx.annotation." + name
			}
			if acrJavaImports(file, "androidx.annotation.experimental", name) {
				return "androidx.annotation.experimental." + name
			}
			return name
		}
		if target, ok := fileFactsCache().Imports(file).Aliases[name]; ok {
			return target
		}
		return "kotlin." + name
	}
	return name
}

// axlOptInNamesMarker reports whether an OptIn annotation's arguments name the marker class
// (`Marker::class`, `Marker.class`, `[Marker::class]`, `{Marker.class}`).
func axlOptInNamesMarker(file *scanner.File, args uint32, marker string) bool {
	if args == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(args, func(n uint32) {
		if found {
			return
		}
		var typeText string
		text := strings.Join(strings.Fields(file.FlatNodeText(n)), "")
		switch file.FlatType(n) {
		case "callable_reference", "class_literal":
			typeText = strings.TrimSuffix(strings.TrimSuffix(text, "::class"), ".class")
		case "navigation_expression":
			// a.Exp::class parses as a navigation ending in ::class.
			if !strings.HasSuffix(text, "::class") {
				return
			}
			typeText = strings.TrimSuffix(text, "::class")
		default:
			return
		}
		if typeText != "" && axlResolveProjectName(file, typeText) == marker {
			found = true
		}
	})
	return found
}
