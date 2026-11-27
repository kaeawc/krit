package rules

import (
	"bytes"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Port of Android Lint 32.4.1's ObjectAnimatorDetector AnimatorKeep issue: a property setter that an ObjectAnimator
// reaches by reflection, in a module that shrinks, without @Keep. The detector's other issue (ObjectAnimatorBinding,
// broken property names and signatures) is not part of this port, so a setter that is missing or does not match is
// left alone.

func registerPerformanceAnimatorChecks() {
	r := &AnimatorKeepRule{AndroidRule: lintAndroidRule("AnimatorKeep", "Missing `@Keep` for Animated Properties",
		ALCPerformance, ALSWarning, 4)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		Needs: api.NeedsParsedFiles, NeedsLibraryFacts: true,
		Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: r.Confidence(), Implementation: r, Check: r.check,
	})
}

// AnimatorKeepRule mirrors Lint's AnimatorKeep.
type AnimatorKeepRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: targets and setters are resolved within the project's
// sources, and ProGuard keep rules are not read.
func (r *AnimatorKeepRule) Confidence() float64 { return api.ConfidenceMedium }

// Meta keeps the rule opt-in: Lint also accepts a ProGuard/R8 keep rule for the setter, which krit does not read.
func (r *AnimatorKeepRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "AnimatorKeep", RuleSet: "android-lint", DefaultActive: false,
		OptInReason: api.OptInReasonProjectPolicy}
}

const animatorKeepMessage = "This method is accessed from an ObjectAnimator so it should be annotated with `@Keep` " +
	"to ensure that it is not discarded or renamed in release builds"

func (r *AnimatorKeepRule) check(ctx *api.Context) {
	// Lint checks only modules that shrink. Without a Gradle model it assumes they do.
	if facts := librarymodel.EnsureFacts(ctx.LibraryFacts); facts != nil && facts.Profile.HasGradle &&
		facts.Profile.Android.Minify != librarymodel.PresencePresent {
		return
	}
	files := append([]*scanner.File(nil), ctx.ParsedFiles...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	a := &animatorAnalysis{files: files, warned: map[[2]uintptr]bool{}}
	for _, file := range files {
		if file == nil || file.FlatTree == nil || isProjectTestFile(ctx, file.Path) ||
			!bytes.Contains(file.Content, []byte("ObjectAnimator")) {
			continue
		}
		for _, kind := range []string{"call_expression", "method_invocation"} {
			file.FlatWalkNodes(0, kind, func(idx uint32) { a.visitCall(file, idx) })
		}
	}
	for _, f := range a.out {
		ctx.Emit(r.Finding(f.file, f.line, f.col, animatorKeepMessage))
	}
}

type animatorFinding struct {
	file      *scanner.File
	line, col int
}

type animatorClass struct {
	file *scanner.File
	decl uint32
	fqn  string
}

type animatorAnalysis struct {
	files   []*scanner.File
	classes map[string][]animatorClass // simple name -> declarations
	warned  map[[2]uintptr]bool
	out     []animatorFinding
}

// animatorExpected maps an ObjectAnimator/PropertyValuesHolder factory to the setter parameter type it needs.
var animatorExpected = map[string]string{
	"ofInt": "int", "ofArgb": "int", "ofFloat": "float", "ofMultiInt": "int[]", "ofMultiFloat": "float[]",
}

// animatorEvaluatorTypes maps a TypeEvaluator class to the value type of ofObject.
var animatorEvaluatorTypes = map[string]string{
	"FloatEvaluator": "float", "FloatArrayEvaluator": "float[]", "IntEvaluator": "int", "ArgbEvaluator": "int",
	"IntArrayEvaluator": "int[]", "PointFEvaluator": "android.graphics.PointF",
}

func (a *animatorAnalysis) visitCall(file *scanner.File, idx uint32) {
	if file.Language == scanner.LangKotlin && perfIsTrailingLambdaInner(file, idx) {
		return
	}
	call, ok := lintCallAt(file, idx)
	if !ok || len(call.Args) < 2 || call.ReceiverNode == 0 {
		return
	}
	switch call.Name {
	case "ofInt", "ofArgb", "ofFloat", "ofMultiInt", "ofMultiFloat", "ofObject", "ofPropertyValuesHolder":
	default:
		return
	}
	if !lintTypeRefMatches(file, call.ReceiverText, "android.animation.ObjectAnimator") {
		return
	}
	target, ok := a.targetClass(file, call.Args[0], idx)
	if !ok {
		return
	}
	if call.Name == "ofPropertyValuesHolder" {
		for _, arg := range call.Args[1:] {
			holder, ok := a.holderConstruction(file, arg, idx)
			if !ok {
				return
			}
			hc, _ := lintCallAt(file, holder)
			expected, ok := animatorExpectedType(file, hc, 1)
			if !ok || len(hc.Args) < 2 {
				return
			}
			a.checkProperty(file, hc.Args[0], target, expected)
		}
		return
	}
	expected, ok := animatorExpectedType(file, call, 2)
	if !ok {
		return
	}
	a.checkProperty(file, call.Args[1], target, expected)
}

// animatorExpectedType is Lint's getExpectedType: the factory's value type, or for ofObject the type its
// evaluator (at evaluatorIndex) produces.
func animatorExpectedType(file *scanner.File, call lintCall, evaluatorIndex int) (string, bool) {
	if t, ok := animatorExpected[call.Name]; ok {
		return t, true
	}
	if call.Name == "ofKeyframe" {
		return "android.animation.Keyframe", true
	}
	if call.Name != "ofObject" || len(call.Args) <= evaluatorIndex {
		return "", false
	}
	eval, ok := lintCallAt(file, call.Args[evaluatorIndex])
	if !ok {
		return "", false
	}
	t, ok := animatorEvaluatorTypes[eval.Name]
	if !ok || !lintTypeRefMatches(file, eval.TypeText, "android.animation."+eval.Name) {
		return "", false
	}
	return t, true
}

// holderConstruction mirrors Lint's findHolderConstruction: a PropertyValuesHolder factory call (other than
// ofKeyframe), directly or as the last value of a variable.
func (a *animatorAnalysis) holderConstruction(file *scanner.File, arg, at uint32) (uint32, bool) {
	for depth := 0; depth < 4 && arg != 0; depth++ {
		switch file.FlatType(arg) {
		case "call_expression", "method_invocation":
			c, ok := lintCallAt(file, arg)
			if ok && strings.HasPrefix(c.Name, "of") && c.Name != "ofKeyframe" && c.ReceiverNode != 0 &&
				lintTypeRefMatches(file, c.ReceiverText, "android.animation.PropertyValuesHolder") {
				return arg, true
			}
			return 0, false
		case "simple_identifier", "identifier":
			arg = lintInitializerOf(file, at, file.FlatNodeText(arg), true)
		default:
			return 0, false
		}
	}
	return 0, false
}

// targetClass resolves the class an animator's target expression is: `this`, a constructor call, or a variable
// declared or initialized as a class of the project's sources.
func (a *animatorAnalysis) targetClass(file *scanner.File, target, at uint32) (animatorClass, bool) {
	name := ""
	switch file.FlatType(target) {
	case "this_expression", "this":
		cls, ok := flatEnclosingAncestor(file, at, "class_declaration", "object_declaration")
		if !ok {
			return animatorClass{}, false
		}
		return a.classAt(file, cls)
	case "call_expression", "object_creation_expression":
		c, ok := lintCallAt(file, target)
		if !ok || c.Name == "" || c.Name[0] < 'A' || c.Name[0] > 'Z' {
			return animatorClass{}, false
		}
		name = c.TypeText
	case "simple_identifier", "identifier":
		ident := file.FlatNodeText(target)
		name = lintTrimGenerics(strings.TrimSuffix(strings.TrimSpace(lintDeclaredTypeText(file, at, ident)), "?"))
		if name == "" {
			if init := lintInitializerOf(file, at, ident, true); init != 0 {
				return a.targetClass(file, init, init)
			}
		}
	}
	if name == "" {
		return animatorClass{}, false
	}
	return a.resolveClass(file, name)
}

func (a *animatorAnalysis) index() {
	if a.classes != nil {
		return
	}
	a.classes = map[string][]animatorClass{}
	for _, f := range a.files {
		if f == nil || f.FlatTree == nil {
			continue
		}
		pkg := sourcePackageName(f)
		for _, kind := range []string{"class_declaration", "object_declaration"} {
			f.FlatWalkNodes(0, kind, func(decl uint32) {
				n := lintClassNameNode(f, decl)
				if n == 0 {
					return
				}
				simple := f.FlatNodeText(n)
				fqn := simple
				for cur, ok := f.FlatParent(decl); ok; cur, ok = f.FlatParent(cur) {
					if t := f.FlatType(cur); t == "class_declaration" || t == "object_declaration" {
						if outer := lintClassNameNode(f, cur); outer != 0 {
							fqn = f.FlatNodeText(outer) + "." + fqn
						}
					}
				}
				if pkg != "" {
					fqn = pkg + "." + fqn
				}
				a.classes[simple] = append(a.classes[simple], animatorClass{file: f, decl: decl, fqn: fqn})
			})
		}
	}
}

func (a *animatorAnalysis) classAt(file *scanner.File, decl uint32) (animatorClass, bool) {
	a.index()
	n := lintClassNameNode(file, decl)
	if n == 0 {
		return animatorClass{}, false
	}
	for _, c := range a.classes[file.FlatNodeText(n)] {
		if c.file == file && c.decl == decl {
			return c, true
		}
	}
	return animatorClass{}, false
}

// resolveClass resolves a type name written in file to one source class: a declaration of the same file, the
// imported class, or a class of the same package; a qualified name must match exactly.
func (a *animatorAnalysis) resolveClass(file *scanner.File, name string) (animatorClass, bool) {
	a.index()
	simple := name[strings.LastIndex(name, ".")+1:]
	cands := a.classes[simple]
	pick := func(match func(animatorClass) bool) (animatorClass, bool) {
		var found []animatorClass
		for _, c := range cands {
			if match(c) {
				found = append(found, c)
			}
		}
		if len(found) == 1 {
			return found[0], true
		}
		return animatorClass{}, false
	}
	if strings.Contains(name, ".") {
		if c, ok := pick(func(c animatorClass) bool { return c.fqn == name || strings.HasSuffix(c.fqn, "."+name) }); ok {
			return c, true
		}
		return animatorClass{}, false
	}
	if c, ok := pick(func(c animatorClass) bool { return c.file == file }); ok {
		return c, true
	}
	if imported, ok := lintImportsOf(file).bySimple[simple]; ok {
		return pick(func(c animatorClass) bool { return c.fqn == imported })
	}
	pkg := sourcePackageName(file)
	return pick(func(c animatorClass) bool { return c.fqn == pkg+"."+simple || pkg == "" && c.fqn == simple })
}

// animatorSetter is a declaration that compiles to a setter: a Java method, a Kotlin function, or a Kotlin var
// property (or constructor property).
type animatorSetter struct {
	file      *scanner.File
	decl      uint32 // method, function or property declaration
	nameNode  uint32
	params    int
	paramType string // JVM type of the single parameter
	static    bool
	keep      bool // @Keep on the setter itself
	owner     animatorClass
}

// checkProperty mirrors Lint's checkProperty for the AnimatorKeep outcome.
func (a *animatorAnalysis) checkProperty(file *scanner.File, nameExpr uint32, target animatorClass, expected string) {
	prop, ok := lintEvalString(file, nameExpr)
	if !ok || !strings.Contains(target.fqn, ".") {
		return
	}
	setter := "set" + strings.ToUpper(prop[:min(1, len(prop))]) + prop[min(1, len(prop)):]
	var best *animatorSetter
	exact := false
	for _, m := range a.setters(target, setter, 0) {
		m := m
		if m.params == 1 {
			if best == nil {
				best = &m
			}
			if m.paramType == expected {
				best, exact = &m, true
				break
			}
		} else if best == nil {
			best = &m
		}
	}
	// A setter that is missing, mismatched or static is ObjectAnimatorBinding's finding, not this issue's.
	if best == nil || !exact || best.static {
		return
	}
	if best.keep || animatorEnclosingKeep(best.file, best.decl) {
		return
	}
	key := [2]uintptr{uintptr(best.decl), uintptrOf(best.file)}
	if a.warned[key] {
		return
	}
	a.warned[key] = true
	a.out = append(a.out, animatorFinding{file: best.file, line: best.file.FlatRow(best.nameNode) + 1,
		col: best.file.FlatCol(best.nameNode) + 1})
}

func uintptrOf(f *scanner.File) uintptr {
	if f == nil {
		return 0
	}
	h := uintptr(0)
	for i := 0; i < len(f.Path); i++ {
		h = h*31 + uintptr(f.Path[i])
	}
	return h
}

// setters lists the setter-like declarations named name in a class and then in its source superclasses, as
// findMethodsByName(name, checkBases = true) does. A superclass outside the project's sources ends the walk.
func (a *animatorAnalysis) setters(cls animatorClass, name string, depth int) []animatorSetter {
	if depth > 6 {
		return nil
	}
	file := cls.file
	var out []animatorSetter
	prop := strings.ToLower(name[3:4]) + name[4:]
	if body, ok := file.FlatFindChild(cls.decl, "class_body"); ok {
		for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "method_declaration":
				if id, ok := file.FlatFindChild(c, "identifier"); ok && file.FlatNodeText(id) == name {
					out = append(out, javaAnimatorSetter(file, c, id, cls))
				}
			case "function_declaration":
				if id, ok := file.FlatFindChild(c, "simple_identifier"); ok && file.FlatNodeText(id) == name {
					out = append(out, kotlinAnimatorFunction(file, c, id, cls))
				}
			case "property_declaration":
				if s, ok := kotlinAnimatorProperty(file, c, prop, cls); ok {
					out = append(out, s)
				}
			}
		}
	}
	if file.Language == scanner.LangKotlin {
		if ctor, ok := file.FlatFindChild(cls.decl, "primary_constructor"); ok {
			file.FlatWalkNodes(ctor, "class_parameter", func(p uint32) {
				if s, ok := kotlinAnimatorProperty(file, p, prop, cls); ok {
					out = append(out, s)
				}
			})
		}
	}
	for _, ref := range lintDirectSupertypeRefs(file, cls.decl) {
		if super, ok := a.resolveClass(file, ref.name); ok {
			out = append(out, a.setters(super, name, depth+1)...)
		}
	}
	return out
}

func javaAnimatorSetter(file *scanner.File, m, id uint32, owner animatorClass) animatorSetter {
	s := animatorSetter{file: file, decl: m, nameNode: id, owner: owner, static: file.FlatHasModifier(m, "static"),
		keep: animatorHasKeep(file, m)}
	if params, ok := file.FlatFindChild(m, "formal_parameters"); ok {
		for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
			if t := file.FlatType(p); t != "formal_parameter" && t != "spread_parameter" {
				continue
			}
			s.params++
			for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "integral_type", "floating_point_type", "boolean_type", "type_identifier", "scoped_type_identifier",
					"array_type", "generic_type":
					s.paramType = javaAnimatorType(file, strings.TrimSpace(file.FlatNodeText(c)))
				}
			}
		}
	}
	return s
}

func javaAnimatorType(file *scanner.File, t string) string {
	switch t {
	case "PointF", "android.graphics.PointF":
		if lintTypeRefMatches(file, t, "android.graphics.PointF") {
			return "android.graphics.PointF"
		}
	case "Keyframe":
		return "android.animation.Keyframe"
	}
	return strings.ReplaceAll(t, " ", "")
}

func kotlinAnimatorFunction(file *scanner.File, fn, id uint32, owner animatorClass) animatorSetter {
	s := animatorSetter{file: file, decl: fn, nameNode: id, owner: owner, keep: animatorHasKeep(file, fn)}
	if params, ok := file.FlatFindChild(fn, "function_value_parameters"); ok {
		for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
			if file.FlatType(p) != "parameter" {
				continue
			}
			s.params++
			for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "user_type", "nullable_type":
					s.paramType = kotlinAnimatorType(file, strings.TrimSpace(file.FlatNodeText(c)))
				}
			}
		}
	}
	// A companion or object member annotated @JvmStatic would be static; the class's own functions are not.
	s.static = file.FlatType(owner.decl) == "object_declaration" && animatorHasAnnotation(file, fn, "JvmStatic")
	return s
}

// kotlinAnimatorProperty describes the setter a `var` property (or constructor `var` parameter) called prop compiles to.
func kotlinAnimatorProperty(file *scanner.File, decl uint32, prop string, owner animatorClass) (animatorSetter, bool) {
	var nameNode uint32
	switch file.FlatType(decl) {
	case "property_declaration":
		kind, _ := file.FlatFindChild(decl, "binding_pattern_kind")
		if strings.TrimSpace(file.FlatNodeText(kind)) != "var" {
			return animatorSetter{}, false
		}
		vd, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || lintDeclaredName(file, vd) != prop {
			return animatorSetter{}, false
		}
		nameNode, _ = file.FlatFindChild(vd, "simple_identifier")
	case "class_parameter":
		// The val/var keyword sits in a binding_pattern_kind child.
		kind, ok := file.FlatFindChild(decl, "binding_pattern_kind")
		if !ok || strings.TrimSpace(file.FlatNodeText(kind)) != "var" {
			return animatorSetter{}, false
		}
		id, ok := file.FlatFindChild(decl, "simple_identifier")
		if !ok || file.FlatNodeText(id) != prop {
			return animatorSetter{}, false
		}
		nameNode = id
	default:
		return animatorSetter{}, false
	}
	if nameNode == 0 {
		return animatorSetter{}, false
	}
	typ := ""
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "user_type", "nullable_type":
			typ = strings.TrimSpace(file.FlatNodeText(c))
		case "variable_declaration":
			for t := file.FlatFirstChild(c); t != 0; t = file.FlatNextSib(t) {
				if tt := file.FlatType(t); tt == "user_type" || tt == "nullable_type" {
					typ = strings.TrimSpace(file.FlatNodeText(t))
				}
			}
		}
	}
	if typ == "" {
		return animatorSetter{}, false
	}
	// Only a setter-targeted annotation reaches the setter: `@set:Keep` or @Keep on the `set` accessor. Lint does
	// not see a property-level @Keep, which annotates the property.
	keep := strings.Contains(file.FlatNodeText(decl), "@set:Keep") || strings.Contains(file.FlatNodeText(decl), "@set:androidx.annotation.Keep")
	if setterNode, ok := file.FlatFindChild(decl, "setter"); ok && animatorHasKeep(file, setterNode) {
		keep = true
	}
	return animatorSetter{file: file, decl: decl, nameNode: nameNode, params: 1, paramType: kotlinAnimatorType(file, typ),
		owner: owner, keep: keep}, true
}

// kotlinAnimatorType maps a Kotlin parameter type to the JVM type of the compiled setter parameter.
func kotlinAnimatorType(file *scanner.File, t string) string {
	if strings.HasSuffix(t, "?") {
		return "boxed:" + t // a nullable primitive is boxed and never matches
	}
	switch t {
	case "Float":
		return "float"
	case "Int":
		return "int"
	case "Double":
		return "double"
	case "Long":
		return "long"
	case "Boolean":
		return "boolean"
	case "IntArray":
		return "int[]"
	case "FloatArray":
		return "float[]"
	case "PointF", "android.graphics.PointF":
		if lintTypeRefMatches(file, t, "android.graphics.PointF") {
			return "android.graphics.PointF"
		}
	case "Keyframe", "android.animation.Keyframe":
		return "android.animation.Keyframe"
	}
	return t
}

// animatorHasKeep reports @Keep (androidx or support library) among the declaration's annotations.
func animatorHasKeep(file *scanner.File, decl uint32) bool {
	return animatorHasAnnotation(file, decl, "Keep") &&
		(lintImportsOf(file).bySimple["Keep"] == "" || lintTypeRefMatches(file, "Keep", "androidx.annotation.Keep") ||
			lintTypeRefMatches(file, "Keep", "android.support.annotation.Keep")) ||
		animatorHasAnnotation(file, decl, "androidx.annotation.Keep") ||
		animatorHasAnnotation(file, decl, "android.support.annotation.Keep")
}

// animatorHasAnnotation reports whether the declaration's own modifiers carry the annotation name (with no use-site
// target such as @get: or @set:).
func animatorHasAnnotation(file *scanner.File, decl uint32, name string) bool {
	found := false
	for c := file.FlatFirstChild(decl); c != 0 && !found; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "modifiers":
			file.FlatWalkAllNodes(c, func(n uint32) {
				switch file.FlatType(n) {
				case "annotation", "marker_annotation":
					text := strings.TrimSpace(file.FlatNodeText(n))
					text = strings.TrimPrefix(text, "@")
					if i := strings.IndexByte(text, '('); i >= 0 {
						text = text[:i]
					}
					if strings.TrimSpace(text) == name {
						found = true
					}
				}
			})
		}
	}
	return found
}

// animatorEnclosingKeep reports @Keep on a class around the declaration.
func animatorEnclosingKeep(file *scanner.File, decl uint32) bool {
	for cur, ok := file.FlatParent(decl); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "companion_object":
			if animatorHasKeep(file, cur) {
				return true
			}
		}
	}
	return false
}
