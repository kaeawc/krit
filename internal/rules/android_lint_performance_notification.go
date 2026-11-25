package rules

import (
	"bytes"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's NotificationTrampolineDetector: NotificationTrampoline (a notification whose
// PendingIntent starts a BroadcastReceiver or Service that launches an activity) and LaunchActivityFromNotification
// (a content or full-screen intent that starts a receiver or service at all).
//
// The detector follows the PendingIntent and its Intent through the last assignment of local variables, reads the
// class literal of the Intent, and inspects that class's onReceive/onStartCommand. Its class lookup only succeeds for
// Java source classes: a Kotlin receiver or service is not found, so Lint reports LaunchActivityFromNotification
// there instead of NotificationTrampoline (and nothing for addAction). The port keeps those verdicts.

const (
	notificationBuilderFQN = "androidx.core.app.NotificationCompat.Builder"
	pendingIntentFQN       = "android.app.PendingIntent"
	intentFQN              = "android.content.Intent"
)

func registerPerformanceNotificationChecks() {
	trampoline := &NotificationTrampolineRule{AndroidRule: lintAndroidRule("NotificationTrampoline",
		"Notification Trampolines", ALCPerformance, ALSError, 6)}
	activity := &LaunchActivityFromNotificationRule{AndroidRule: lintAndroidRule("LaunchActivityFromNotification",
		"Notification Launches Services or BroadcastReceivers", ALCPerformance, ALSWarning, 3)}
	for _, r := range []struct {
		rule  AndroidRule
		impl  any
		check func(*api.Context)
	}{{trampoline.AndroidRule, trampoline, trampoline.check}, {activity.AndroidRule, activity, activity.check}} {
		api.Register(&api.Rule{
			ID: r.rule.RuleName, Category: r.rule.RuleSetName, Description: r.rule.Description(), Sev: api.Severity(r.rule.Sev),
			Needs:      api.NeedsParsedFiles,
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: api.ConfidenceMedium, Implementation: r.impl, Check: r.check,
		})
	}
}

// NotificationTrampolineRule mirrors Lint's NotificationTrampoline.
type NotificationTrampolineRule struct {
	FlatDispatchBase
	AndroidRule
}

// LaunchActivityFromNotificationRule mirrors Lint's LaunchActivityFromNotification.
type LaunchActivityFromNotificationRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: values are traced through same-callable assignments.
func (r *NotificationTrampolineRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence: values are traced through same-callable assignments.
func (r *LaunchActivityFromNotificationRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NotificationTrampolineRule) check(ctx *api.Context) {
	for _, f := range notificationTrampolineFindings(ctx.ParsedFiles) {
		if f.trampoline {
			ctx.Emit(r.Finding(f.file, f.line, f.col, f.msg))
		}
	}
}

func (r *LaunchActivityFromNotificationRule) check(ctx *api.Context) {
	for _, f := range notificationTrampolineFindings(ctx.ParsedFiles) {
		if !f.trampoline {
			ctx.Emit(r.Finding(f.file, f.line, f.col, f.msg))
		}
	}
}

type notificationFinding struct {
	file       *scanner.File
	line, col  int
	msg        string
	trampoline bool
}

// notificationTrampolineFindings runs the detector over every non-test source file.
func notificationTrampolineFindings(parsed []*scanner.File) []notificationFinding {
	files := append([]*scanner.File(nil), parsed...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	a := &notificationAnalysis{files: files}
	for _, file := range files {
		if file == nil || file.FlatTree == nil || scanner.IsTestFile(file.Path) ||
			!bytes.Contains(file.Content, []byte("NotificationCompat")) {
			continue
		}
		for _, kind := range []string{"call_expression", "method_invocation"} {
			file.FlatWalkNodes(0, kind, func(idx uint32) { a.visitCall(file, idx) })
		}
	}
	return a.out
}

type notificationAnalysis struct {
	files     []*scanner.File
	javaIndex map[string][]javaClassRef
	out       []notificationFinding
}

// javaClassRef is a class declared in a source file; java is false for Kotlin classes.
type javaClassRef struct {
	file *scanner.File
	decl uint32
	java bool
}

// trampolineKind is the component a PendingIntent factory starts.
type trampolineKind struct {
	className string // BroadcastReceiver or Service
	handler   string // onReceive or onStartCommand
	params    []string
}

var trampolineKinds = map[string]trampolineKind{
	"getBroadcast": {"BroadcastReceiver", "onReceive", []string{"Context", "Intent"}},
	"getService":   {"Service", "onStartCommand", []string{"Intent", "int", "int"}},
}

func (a *notificationAnalysis) visitCall(file *scanner.File, idx uint32) {
	if file.Language == scanner.LangKotlin && perfIsTrailingLambdaInner(file, idx) {
		return
	}
	call, ok := lintCallAt(file, idx)
	if !ok {
		return
	}
	argIndex := 0
	switch call.Name {
	case "setContentIntent", "setFullScreenIntent":
	case "addAction":
		argIndex = 2
	default:
		return
	}
	if !notificationBuilderReceiver(file, call) {
		return
	}
	arg := call.Arg(argIndex, "")
	if arg == 0 {
		return
	}
	pfile, pending := a.pendingIntentConstruction(file, arg, idx, 0)
	if pending == 0 {
		return
	}
	pcall, _ := lintCallAt(pfile, pending)
	kind, ok := trampolineKinds[pcall.Name]
	if !ok {
		return
	}
	found := a.checkNonActivityIntent(file, idx, pfile, pcall, kind)
	if !found && call.Name != "addAction" {
		line, col := notificationCallNamePos(file, idx)
		a.out = append(a.out, notificationFinding{file: file, line: line, col: col,
			msg: "Notifications should only launch a `" + kind.className + "` from notification actions (`addAction`)"})
	}
}

// notificationCallNamePos is the position of the called method's name, where Lint anchors the report.
func notificationCallNamePos(file *scanner.File, idx uint32) (line, col int) {
	if first := file.FlatFirstChild(idx); file.FlatType(idx) == "call_expression" && file.FlatType(first) == "call_expression" {
		// `f(args) { lambda }`: the name is on the inner call.
		idx = first
	}
	name := lintCallNameNode(file, idx)
	if name == 0 {
		name = idx
	}
	return file.FlatRow(name) + 1, file.FlatCol(name) + 1
}

// notificationBuilderReceiver reports whether the call is made on an androidx NotificationCompat.Builder: an explicit
// receiver proven to be one, or the implicit receiver of an apply/run/with block on one.
func notificationBuilderReceiver(file *scanner.File, call lintCall) bool {
	if call.ReceiverNode != 0 {
		return notificationIsBuilder(file, call.ReceiverNode, call.Idx, 0)
	}
	if file.Language != scanner.LangKotlin {
		return false
	}
	for cur, ok := file.FlatParent(call.Idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "class_body", "source_file":
			return false
		case "lambda_literal":
			owner := notificationLambdaOwner(file, cur)
			if owner == 0 {
				return false
			}
			if first := file.FlatFirstChild(owner); file.FlatType(first) == "call_expression" {
				// `with(builder) { ... }`: the arguments are on the inner call.
				owner = first
			}
			scope, ok := lintCallAt(file, owner)
			if !ok {
				return false
			}
			switch scope.Name {
			case "apply", "run":
				return scope.ReceiverNode != 0 && notificationIsBuilder(file, scope.ReceiverNode, owner, 0)
			case "with":
				return len(scope.Args) > 0 && notificationIsBuilder(file, scope.Args[0], owner, 0)
			}
			return false
		}
	}
	return false
}

// notificationLambdaOwner returns the call a trailing lambda belongs to.
func notificationLambdaOwner(file *scanner.File, lambda uint32) uint32 {
	p, ok := file.FlatParent(lambda)
	if ok && file.FlatType(p) == "annotated_lambda" {
		p, ok = file.FlatParent(p)
	}
	if !ok || file.FlatType(p) != "call_suffix" {
		return 0
	}
	owner, ok := file.FlatParent(p)
	if !ok || file.FlatType(owner) != "call_expression" {
		return 0
	}
	return owner
}

// notificationIsBuilder reports whether the expression at idx is a NotificationCompat.Builder: its constructor, a
// Builder method that returns the builder (set*, add*, extend, apply, also), or a variable declared or initialized as
// one.
func notificationIsBuilder(file *scanner.File, idx, scope uint32, depth int) bool {
	if idx == 0 || depth > 8 {
		return false
	}
	switch file.FlatType(idx) {
	case "parenthesized_expression":
		kids := perfNamedChildren(file, idx)
		return len(kids) == 1 && notificationIsBuilder(file, kids[0], scope, depth+1)
	case "call_expression", "object_creation_expression", "method_invocation":
		call, ok := lintCallAt(file, idx)
		if !ok {
			return false
		}
		if call.Name == "Builder" {
			return lintTypeRefMatches(file, call.TypeText, notificationBuilderFQN)
		}
		if call.ReceiverNode != 0 && (strings.HasPrefix(call.Name, "set") || strings.HasPrefix(call.Name, "add") ||
			call.Name == "extend" || call.Name == "apply" || call.Name == "also") {
			return notificationIsBuilder(file, call.ReceiverNode, scope, depth+1)
		}
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(idx)
		if t := lintDeclaredTypeText(file, scope, name); t != "" {
			return lintTypeRefMatches(file, lintTrimGenerics(strings.TrimSuffix(t, "?")), notificationBuilderFQN)
		}
		if init := lintInitializerOf(file, scope, name, true); init != 0 {
			return notificationIsBuilder(file, init, init, depth+1)
		}
	}
	return false
}

// pendingIntentConstruction mirrors Lint's findPendingIntentConstruction: a direct PendingIntent factory call, the
// last value assigned to a variable, or the value a same-class Java helper method returns.
func (a *notificationAnalysis) pendingIntentConstruction(file *scanner.File, arg, at uint32, depth int) (*scanner.File, uint32) {
	if arg == 0 || depth > 4 {
		return nil, 0
	}
	switch file.FlatType(arg) {
	case "parenthesized_expression":
		if kids := perfNamedChildren(file, arg); len(kids) == 1 {
			return a.pendingIntentConstruction(file, kids[0], at, depth)
		}
	case "simple_identifier", "identifier":
		value := notificationLastAssignment(file, file.FlatNodeText(arg), at)
		if sel := notificationSelectorCall(file, value); sel != 0 {
			return file, sel
		}
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, arg)
		if !ok {
			return nil, 0
		}
		if call.ReceiverNode != 0 {
			if lintTypeRefMatches(file, call.ReceiverText, pendingIntentFQN) {
				return file, arg
			}
			return nil, 0
		}
		// Lint follows Java source methods only; a Kotlin helper does not resolve to a method it can read.
		if file.Language == scanner.LangJava {
			return a.javaHelperReturn(file, call, depth)
		}
	}
	return nil, 0
}

// javaHelperReturn follows an unqualified Java call into the method of the enclosing class with that name and
// arity, returning the last returned PendingIntent factory call that names a receiver or service.
func (a *notificationAnalysis) javaHelperReturn(file *scanner.File, call lintCall, depth int) (*scanner.File, uint32) {
	class, ok := flatEnclosingAncestor(file, call.Idx, "class_declaration")
	if !ok {
		return nil, 0
	}
	var method uint32
	file.FlatWalkNodes(class, "method_declaration", func(m uint32) {
		if id, ok := file.FlatFindChild(m, "identifier"); ok && file.FlatNodeText(id) == call.Name &&
			len(javaMethodParams(file, m)) == len(call.Args) {
			method = m
		}
	})
	if method == 0 {
		return nil, 0
	}
	var found uint32
	file.FlatWalkNodes(method, "return_statement", func(ret uint32) {
		kids := perfNamedChildren(file, ret)
		if len(kids) == 0 {
			return
		}
		if pf, p := a.pendingIntentConstruction(file, kids[0], ret, depth+1); p != 0 {
			if pc, ok := lintCallAt(pf, p); ok {
				if _, isKind := trampolineKinds[pc.Name]; isKind {
					found = p
				}
			}
		}
	})
	return file, found
}

// javaMethodParams returns the formal parameter nodes of a Java method.
func javaMethodParams(file *scanner.File, method uint32) []uint32 {
	params, ok := file.FlatFindChild(method, "formal_parameters")
	if !ok {
		return nil
	}
	var out []uint32
	for c := file.FlatFirstChild(params); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "formal_parameter" || t == "spread_parameter" {
			out = append(out, c)
		}
	}
	return out
}

// notificationSelectorCall returns the call an expression ends with: the expression itself, or the last call of a
// qualified chain (`PendingIntent.getBroadcast(...)`).
func notificationSelectorCall(file *scanner.File, idx uint32) uint32 {
	if idx == 0 {
		return 0
	}
	switch file.FlatType(idx) {
	case "parenthesized_expression":
		if kids := perfNamedChildren(file, idx); len(kids) == 1 {
			return notificationSelectorCall(file, kids[0])
		}
	case "call_expression", "method_invocation", "object_creation_expression":
		return idx
	}
	return 0
}

// notificationLastAssignment returns the value last assigned to the local, parameter-free variable name before at,
// within the enclosing function, or the initializer of a property or field of the file.
func notificationLastAssignment(file *scanner.File, name string, at uint32) uint32 {
	fn, hasFn := flatEnclosingCallable(file, at)
	before := file.FlatStartByte(at)
	var best uint32
	var bestStart uint32
	consider := func(stmt, value uint32) {
		if value == 0 || file.FlatStartByte(stmt) >= before {
			return
		}
		if s := file.FlatStartByte(stmt); best == 0 || s > bestStart {
			best, bestStart = value, s
		}
	}
	if hasFn {
		file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
			if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
				consider(decl, lintInitializerAfterEquals(file, decl))
			}
		})
		file.FlatWalkNodes(fn, "variable_declarator", func(decl uint32) {
			if id, ok := file.FlatFindChild(decl, "identifier"); ok && file.FlatNodeText(id) == name {
				consider(decl, lintInitializerAfterEquals(file, decl))
			}
		})
		file.FlatWalkNodes(fn, "assignment", func(asg uint32) {
			kids := perfNamedChildren(file, asg)
			if len(kids) == 2 && strings.TrimSpace(file.FlatNodeText(kids[0])) == name && perfHasToken(file, asg, "=") {
				consider(asg, kids[1])
			}
		})
		file.FlatWalkNodes(fn, "assignment_expression", func(asg uint32) {
			kids := perfNamedChildren(file, asg)
			if len(kids) == 2 && file.FlatNodeText(kids[0]) == name && perfHasToken(file, asg, "=") {
				consider(asg, kids[1])
			}
		})
	}
	if best != 0 || lintNameIsParameterAt(file, at, name) {
		return best
	}
	return lintInitializerOf(file, at, name, true)
}

// checkNonActivityIntent mirrors Lint's checkNonActivityIntent. It reports trampolines and returns whether the
// intent's target class was found (in which case Lint does not report LaunchActivityFromNotification).
func (a *notificationAnalysis) checkNonActivityIntent(setFile *scanner.File, setCall uint32, pfile *scanner.File,
	pending lintCall, kind trampolineKind) bool {
	if len(pending.Args) != 4 {
		return false
	}
	intentArg := pending.Args[2]
	if t := pfile.FlatType(intentArg); t != "simple_identifier" && t != "identifier" {
		return false
	}
	construction := notificationSelectorCall(pfile, notificationLastAssignment(pfile, pfile.FlatNodeText(intentArg), pending.Idx))
	if construction == 0 {
		return false
	}
	intent, ok := lintCallAt(pfile, construction)
	if !ok || intent.Name != "Intent" || intent.ReceiverNode != 0 && pfile.FlatType(construction) != "object_creation_expression" ||
		!lintTypeRefMatches(pfile, intent.TypeText, intentFQN) {
		return false
	}
	var classArg uint32
	switch len(intent.Args) {
	case 2:
		classArg = intent.Args[1]
	case 4:
		classArg = intent.Args[3]
	default:
		return false
	}
	className := notificationClassLiteralName(pfile, classArg)
	if className == "" {
		return false
	}
	target := a.resolveJavaClass(pfile, className)
	if target.file == nil || !target.java {
		return false
	}
	handler := javaDeclaredMethod(target.file, target.decl, kind.handler, kind.params)
	if handler == 0 {
		return true
	}
	simple := className[strings.LastIndex(className, ".")+1:]
	target.file.FlatWalkNodes(handler, "method_invocation", func(inv uint32) {
		switch javaMethodInvocationName(target.file, inv) {
		case "startActivity", "startActivities":
			// Lint reports the notification call once for every launch in the handler, lambdas included.
			line, col := notificationCallNamePos(setFile, setCall)
			a.out = append(a.out, notificationFinding{file: setFile, line: line, col: col, trampoline: true,
				msg: "This intent launches a `" + kind.className + "` (`" + simple + "`) which launches activities; " +
					"this indirection is bad for performance, and activities should be launched directly from the notification"})
		}
	})
	return true
}

// notificationClassLiteralName returns the class named by `X::class.java` or `X.class`, as written.
func notificationClassLiteralName(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "class_literal":
		for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "type_identifier", "scoped_type_identifier":
				return file.FlatNodeText(c)
			}
		}
	case "navigation_expression":
		kids := perfNamedChildren(file, idx)
		if len(kids) != 2 || strings.TrimSpace(strings.TrimPrefix(file.FlatNodeText(kids[1]), ".")) != "java" {
			return ""
		}
		ref := kids[0]
		switch file.FlatType(ref) {
		case "callable_reference":
			// `X::class`
			if !perfHasToken(file, ref, "class") {
				return ""
			}
		case "navigation_expression":
			// `Outer.Inner::class` parses as a navigation ending in a `::class` suffix.
			parts := perfNamedChildren(file, ref)
			if len(parts) != 2 || strings.ReplaceAll(file.FlatNodeText(parts[1]), " ", "") != "::class" {
				return ""
			}
		default:
			return ""
		}
		text := file.FlatNodeText(ref)
		return strings.ReplaceAll(strings.TrimSpace(text[:strings.Index(text, "::")]), " ", "")
	}
	return ""
}

// resolveJavaClass resolves a class name written in file to a source class of the project: a qualified name, an
// import, or a class of the same package.
func (a *notificationAnalysis) resolveJavaClass(file *scanner.File, name string) javaClassRef {
	if a.javaIndex == nil {
		a.javaIndex = map[string][]javaClassRef{}
		for _, f := range a.files {
			if f == nil || f.FlatTree == nil {
				continue
			}
			pkg := sourceFilePackage(f)
			for _, kind := range []string{"class_declaration", "object_declaration"} {
				f.FlatWalkNodes(0, kind, func(decl uint32) {
					n := lintClassNameNode(f, decl)
					if n == 0 {
						return
					}
					fqn := f.FlatNodeText(n)
					for cur, ok := f.FlatParent(decl); ok; cur, ok = f.FlatParent(cur) {
						if t := f.FlatType(cur); t == "class_declaration" || t == "object_declaration" ||
							t == "interface_declaration" || t == "enum_declaration" {
							if outer := lintClassNameNode(f, cur); outer != 0 {
								fqn = f.FlatNodeText(outer) + "." + fqn
							}
						}
					}
					if pkg != "" {
						fqn = pkg + "." + fqn
					}
					a.javaIndex[fqn] = append(a.javaIndex[fqn], javaClassRef{file: f, decl: decl, java: f.Language == scanner.LangJava})
				})
			}
		}
	}
	candidates := []string{name}
	simple := name
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		simple = name[:dot]
	}
	if imported, ok := lintImportsOf(file).bySimple[simple]; ok {
		candidates = append([]string{imported + strings.TrimPrefix(name, simple)}, candidates...)
	} else if pkg := sourceFilePackage(file); pkg != "" {
		candidates = append([]string{pkg + "." + name}, candidates...)
	}
	for _, c := range candidates {
		if refs := a.javaIndex[c]; len(refs) == 1 {
			return refs[0]
		}
	}
	return javaClassRef{}
}

// sourceFilePackage returns the package a Kotlin or Java file declares, or "".
func sourceFilePackage(file *scanner.File) string {
	for _, kind := range []string{"package_header", "package_declaration"} {
		var pkg string
		file.FlatWalkNodes(0, kind, func(n uint32) {
			if pkg != "" {
				return
			}
			text := strings.TrimSpace(file.FlatNodeText(n))
			text = strings.TrimSuffix(strings.TrimPrefix(text, "package"), ";")
			pkg = strings.TrimSpace(text)
		})
		if pkg != "" {
			return pkg
		}
	}
	return ""
}

// javaDeclaredMethod returns the method of a Java class (not of its supertypes) with the given name and parameter
// types, matched by simple type name.
func javaDeclaredMethod(file *scanner.File, class uint32, name string, params []string) uint32 {
	body, ok := file.FlatFindChild(class, "class_body")
	if !ok {
		return 0
	}
	for m := file.FlatFirstChild(body); m != 0; m = file.FlatNextSib(m) {
		if file.FlatType(m) != "method_declaration" {
			continue
		}
		id, ok := file.FlatFindChild(m, "identifier")
		if !ok || file.FlatNodeText(id) != name {
			continue
		}
		formal := javaMethodParams(file, m)
		if len(formal) != len(params) {
			continue
		}
		match := true
		for i, p := range formal {
			typ := ""
			for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "type_identifier", "scoped_type_identifier", "integral_type":
					typ = file.FlatNodeText(c)
				}
			}
			typ = typ[strings.LastIndex(typ, ".")+1:]
			if typ != params[i] {
				match = false
			}
		}
		if match {
			return m
		}
	}
	return 0
}
