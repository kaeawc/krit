package rules

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/lazyre"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's RecyclerViewDetector (RecyclerView), ViewTypeDetector (FindViewByIdCast),
// RemoteViewDetector (RemoteViewLayout), UncaughtExceptionHandlerDetector (DefaultUncaughtExceptionDelegation) and
// WrongGradleMethodDetector (WrongGradleMethod).

// RecyclerViewRule mirrors Lint's RecyclerView: an adapter's onBindViewHolder position kept in a field or used in an
// inner class.
type RecyclerViewRule struct {
	AndroidRule
}

// FindViewByIDCastRule mirrors Lint's FindViewByIdCast: findViewById passed to a generic method whose result is
// dereferenced, which needs a cast to compile with Java 8.
type FindViewByIDCastRule struct {
	AndroidRule
}

// RemoteViewLayoutRule mirrors Lint's RemoteViewLayout: a RemoteViews layout with views RemoteViews cannot inflate.
type RemoteViewLayoutRule struct {
	AndroidRule
}

// DefaultUncaughtExceptionDelegationRule mirrors Lint's DefaultUncaughtExceptionDelegation: a default uncaught
// exception handler installed in a module that never reads the existing one.
type DefaultUncaughtExceptionDelegationRule struct {
	AndroidRule
}

// WrongGradleMethodRule mirrors the command-line half of Lint's WrongGradleMethod: a dependencies block inside a build
// type or product flavor, and firebaseAppDistribution without its import.
type WrongGradleMethodRule struct {
	GradleBase
	AndroidRule
}

// Confidence reports a medium-high base confidence: the adapter supertype and the escapes are found in source.
func (r *RecyclerViewRule) Confidence() float64 { return api.ConfidenceMediumHigh }

// Confidence reports a medium-high base confidence: the generic signature must be known or declared in the file.
func (r *FindViewByIDCastRule) Confidence() float64 { return api.ConfidenceMediumHigh }

// Confidence reports a high base confidence: the layout files are read directly.
func (r *RemoteViewLayoutRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a medium-high base confidence: the receiver must be written as Thread.
func (r *DefaultUncaughtExceptionDelegationRule) Confidence() float64 {
	return api.ConfidenceMediumHigh
}

// Confidence reports a high base confidence: the patterns are Lint's own command-line patterns.
func (r *WrongGradleMethodRule) Confidence() float64 { return api.ConfidenceHigh }

// ---------------------------------------------------------------------------
// RecyclerView
// ---------------------------------------------------------------------------

var recyclerAdapterSupertypes = []string{
	"androidx.recyclerview.widget.RecyclerView.Adapter", "android.support.v7.widget.RecyclerView.Adapter",
}

var recyclerAdapterKnown = lintKnownSet("ListAdapter", "PagingDataAdapter", "PagedListAdapter")

func (r *RecyclerViewRule) check(ctx *api.Context) {
	file := ctx.File
	cls := ctx.Idx
	if !strings.Contains(file.FlatNodeText(cls), "onBindViewHolder") || !recyclerIsAdapter(ctx, cls) {
		return
	}
	body, ok := file.FlatFindChild(cls, "class_body")
	if !ok {
		return
	}
	for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
		if t := file.FlatType(member); (t != "function_declaration" && t != "method_declaration") || recyclerName(file, member) != "onBindViewHolder" {
			continue
		}
		params := recyclerParams(file, member)
		if len(params) != 2 && len(params) != 3 {
			continue
		}
		fnBody := recyclerFunctionBody(file, member)
		if fnBody == 0 {
			continue
		}
		holder, position := params[0], params[1]
		if recyclerPositionEscapes(file, cls, fnBody, recyclerName(file, position)) {
			// Lint locates a Kotlin parameter at its name and a Java one at its start (modifiers or type).
			at := recyclerParamNameNode(file, position)
			if file.Language == scanner.LangJava {
				at = position
			}
			ctx.Emit(lintFindingAtNode(ctx, at, "",
				"Do not treat position as fixed; only use immediately and call `"+recyclerName(file, holder)+
					".getAdapterPosition()` to look it up later"))
		}
	}
}

func recyclerIsAdapter(ctx *api.Context, cls uint32) bool {
	for _, fqn := range recyclerAdapterSupertypes {
		if lintClassExtends(ctx, cls, fqn, recyclerAdapterKnown) {
			return true
		}
	}
	return false
}

// recyclerParams lists the parameter nodes of a Kotlin function or Java method.
func recyclerParams(file *scanner.File, fn uint32) []uint32 {
	var out []uint32
	listType := "function_value_parameters"
	paramType := "parameter"
	if file.Language == scanner.LangJava {
		listType, paramType = "formal_parameters", "formal_parameter"
	}
	list, ok := file.FlatFindChild(fn, listType)
	if !ok {
		return nil
	}
	for c := file.FlatFirstChild(list); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == paramType || file.FlatType(c) == "spread_parameter" {
			out = append(out, c)
		}
	}
	return out
}

// recyclerName returns the declared name of a Kotlin or Java function, method or parameter.
func recyclerName(file *scanner.File, node uint32) string {
	if n := recyclerParamNameNode(file, node); n != node {
		return file.FlatNodeText(n)
	}
	return ""
}

func recyclerParamNameNode(file *scanner.File, param uint32) uint32 {
	for _, t := range []string{"simple_identifier", "identifier"} {
		if n, ok := file.FlatFindChild(param, t); ok {
			return n
		}
	}
	return param
}

func recyclerFunctionBody(file *scanner.File, fn uint32) uint32 {
	for _, t := range []string{"function_body", "block"} {
		if n, ok := file.FlatFindChild(fn, t); ok {
			return n
		}
	}
	return 0
}

// recyclerPositionEscapes ports ParameterEscapesVisitor: the position escapes when it (or a local it was assigned to)
// is assigned to a field, or is read inside an anonymous or inner class. Lambdas are not classes, so they do not count.
// Names are resolved to their declarations, so a local that redeclares the position (or a copy of it) is not it.
func recyclerPositionEscapes(file *scanner.File, bindClass, body uint32, position string) bool {
	// tracked maps each name that holds the position to the declaration it resolves to: 0 for the parameter, else
	// the local the position was assigned to.
	tracked := map[string]uint32{position: 0}
	holds := func(name string, at uint32) bool {
		decl, ok := tracked[name]
		return ok && recyclerResolve(file, at, body, name) == decl
	}
	fields := recyclerFieldNames(file, bindClass)
	foundInner := false
	escapes := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if escapes {
			return
		}
		switch t := file.FlatType(n); t {
		case "object_literal", "class_declaration", "class_body":
			if t == "class_body" {
				// A Java anonymous class body sits under its `new` expression.
				if p, ok := file.FlatParent(n); !ok || file.FlatType(p) != "object_creation_expression" {
					return
				}
			}
			foundInner = true
		case "assignment", "assignment_expression":
			lhs, rhs, field := recyclerAssignSides(file, n)
			if lhs == "" {
				return
			}
			if rhs != "" && holds(rhs, n) {
				if field {
					escapes = true
					return
				}
				switch decl := recyclerResolve(file, n, body, lhs); {
				case recyclerIsLocal(file, decl):
					tracked[lhs] = decl
				case decl == 0 && fields[lhs]:
					escapes = true
				}
				return
			}
			if !field && holds(lhs, n) {
				delete(tracked, lhs)
			}
		case "simple_identifier", "identifier":
			name := file.FlatNodeText(n)
			if !foundInner || !recyclerIsRead(file, n) || !holds(name, n) {
				return
			}
			if inner := recyclerEnclosingClass(file, n); inner != 0 && inner != bindClass {
				escapes = true
			}
		}
	})
	return escapes
}

// recyclerIsRead reports an identifier in a position that reads a variable: not a declaration's name, a member
// selector or an argument label.
func recyclerIsRead(file *scanner.File, n uint32) bool {
	p, ok := file.FlatParent(n)
	if !ok {
		return false
	}
	switch file.FlatType(p) {
	case "navigation_suffix", "parameter", "formal_parameter", "variable_declaration", "variable_declarator",
		"class_parameter", "function_declaration", "method_declaration", "value_argument_label", "type_identifier":
		return false
	case "field_access":
		// Java `holder.position`: only the receiver reads a variable.
		if file.FlatFirstChild(p) != n {
			return false
		}
	case "value_argument":
		if next := file.FlatNextSib(n); next != 0 && file.FlatNodeText(next) == "=" {
			return false
		}
	}
	return true
}

// recyclerResolve returns the declaration that name, used at node at, resolves to within the bind body: the scope of
// a function, lambda, loop or catch parameter, the nearest earlier local of an enclosing block (destructured too), a
// `when` subject, or the body of a Java class that declares it as a field. It returns 0 when none does: name is then
// the bind method's parameter or a member. Lint 32.4.1 resolves a Kotlin object's own property of the position's name
// to the position (recorded in RecyclerView/shadowing), so Kotlin members do not count.
func recyclerResolve(file *scanner.File, at, body uint32, name string) uint32 {
	offset := file.FlatStartByte(at)
	for cur, ok := file.FlatParent(at); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "method_declaration", "lambda_literal", "lambda_expression", "for_statement",
			"enhanced_for_statement", "catch_clause", "anonymous_function":
			if cur != body && recyclerDeclaresName(file, cur, name) {
				return cur
			}
		case "statements", "block":
			if decl := recyclerLocalBefore(file, cur, name, offset); decl != 0 {
				return decl
			}
		case "when_expression":
			if subject, ok := file.FlatFindChild(cur, "when_subject"); ok && file.FlatEndByte(subject) <= offset {
				if vd, ok := file.FlatFindChild(subject, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
					return subject
				}
			}
		case "class_body":
			if file.Language == scanner.LangJava && recyclerMemberNames(file, cur, nil)[name] {
				return cur
			}
		}
		if cur == body {
			break
		}
	}
	return 0
}

// recyclerIsLocal reports a declaration recyclerResolve returned for a local variable.
func recyclerIsLocal(file *scanner.File, decl uint32) bool {
	if decl == 0 {
		return false
	}
	t := file.FlatType(decl)
	return t == "property_declaration" || t == "local_variable_declaration"
}

// recyclerLocalBefore returns the local variable declaration of the block, before offset, that declares name: a
// Kotlin property (destructured too) or a Java local. A declaration that contains offset (its own initializer) does
// not count. It returns 0 when there is none.
func recyclerLocalBefore(file *scanner.File, block uint32, name string, offset uint32) uint32 {
	var found uint32
	for c := file.FlatFirstChild(block); c != 0 && file.FlatEndByte(c) <= offset; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "property_declaration":
			if vd, ok := file.FlatFindChild(c, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
				found = c
			}
			if multi, ok := file.FlatFindChild(c, "multi_variable_declaration"); ok {
				for vd := file.FlatFirstChild(multi); vd != 0; vd = file.FlatNextSib(vd) {
					if file.FlatType(vd) == "variable_declaration" && lintDeclaredName(file, vd) == name {
						found = c
					}
				}
			}
		case "local_variable_declaration":
			for d := file.FlatFirstChild(c); d != 0; d = file.FlatNextSib(d) {
				if file.FlatType(d) == "variable_declarator" {
					if id, ok := file.FlatFindChild(d, "identifier"); ok && file.FlatNodeText(id) == name {
						found = c
					}
				}
			}
		}
	}
	return found
}

// recyclerDeclaresName reports a parameter (of a function, lambda, loop or catch) declared directly by scope.
func recyclerDeclaresName(file *scanner.File, scope uint32, name string) bool {
	found := false
	var visit func(c uint32, depth int)
	visit = func(c uint32, depth int) {
		for ch := file.FlatFirstChild(c); ch != 0 && !found; ch = file.FlatNextSib(ch) {
			switch file.FlatType(ch) {
			case "function_value_parameters", "formal_parameters", "lambda_parameters", "parameter", "formal_parameter",
				"variable_declaration", "catch_formal_parameter", "inferred_parameters":
				if depth < 3 {
					visit(ch, depth+1)
				}
			case "simple_identifier", "identifier":
				if depth > 0 && file.FlatNodeText(ch) == name {
					found = true
				}
			}
		}
	}
	visit(scope, 0)
	if !found {
		switch file.FlatType(scope) {
		case "for_statement", "enhanced_for_statement":
			// Kotlin `for (x in xs)` and Java `for (T x : xs)` declare the loop variable directly.
			for ch := file.FlatFirstChild(scope); ch != 0; ch = file.FlatNextSib(ch) {
				if (file.FlatType(ch) == "variable_declaration" || file.FlatType(ch) == "identifier") && file.FlatNodeText(ch) == name {
					return true
				}
			}
		}
	}
	return found
}

// recyclerAssignSides returns the simple names of an assignment's target (a name, or the field of a Java
// `holder.name` or `this.name`) and of its value when that is a plain name. field reports a qualified Java target: it
// resolves to a field, never to a local of the same name, and Lint counts it as an escape. A qualified Kotlin target
// (`holder.name`, `this.name`) goes through the property's setter, which Lint does not count, so it returns no
// target (recorded in RecyclerView/field_escape).
func recyclerAssignSides(file *scanner.File, n uint32) (lhs, rhs string, field bool) {
	var parts []uint32
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			parts = append(parts, c)
		}
	}
	if len(parts) < 2 {
		return "", "", false
	}
	target := strings.TrimSpace(file.FlatNodeText(parts[0]))
	if !lintIdentOnly(target) {
		if file.FlatType(parts[0]) != "field_access" {
			return "", "", false
		}
		var last uint32
		for c := file.FlatFirstChild(parts[0]); c != 0; c = file.FlatNextSib(c) {
			last = c
		}
		if last == 0 || file.FlatType(last) != "identifier" {
			return "", "", false
		}
		target, field = file.FlatNodeText(last), true
	}
	op := strings.TrimSpace(string(file.Content[file.FlatEndByte(parts[0]):file.FlatStartByte(parts[len(parts)-1])]))
	if op != "=" {
		return "", "", false
	}
	value := strings.TrimSpace(file.FlatNodeText(parts[len(parts)-1]))
	if !lintIdentOnly(value) {
		value = ""
	}
	return target, value, field
}

// recyclerFieldNames lists the properties and fields declared directly in the class and its same-file superclasses.
func recyclerFieldNames(file *scanner.File, cls uint32) map[string]bool {
	out := map[string]bool{}
	for depth := 0; cls != 0 && depth < 8; depth++ {
		if body, ok := file.FlatFindChild(cls, "class_body"); ok {
			recyclerMemberNames(file, body, out)
		}
		if p, ok := file.FlatFindChild(cls, "primary_constructor"); ok {
			file.FlatWalkNodes(p, "class_parameter", func(cp uint32) {
				if _, ok := file.FlatFindChild(cp, "binding_pattern_kind"); ok {
					out[lintDeclaredName(file, cp)] = true
				}
			})
		}
		var next uint32
		for _, ref := range lintDirectSupertypeRefs(file, cls) {
			if decl := lintSameFileClass(file, ref.simple); decl != 0 && decl != cls {
				next = decl
				break
			}
		}
		cls = next
	}
	return out
}

// recyclerMemberNames adds the properties and fields declared directly in a class body to out (a new map when nil)
// and returns it.
func recyclerMemberNames(file *scanner.File, body uint32, out map[string]bool) map[string]bool {
	if out == nil {
		out = map[string]bool{}
	}
	for m := file.FlatFirstChild(body); m != 0; m = file.FlatNextSib(m) {
		switch file.FlatType(m) {
		case "property_declaration":
			if vd, ok := file.FlatFindChild(m, "variable_declaration"); ok {
				out[lintDeclaredName(file, vd)] = true
			}
		case "field_declaration":
			for c := file.FlatFirstChild(m); c != 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) == "variable_declarator" {
					if id, ok := file.FlatFindChild(c, "identifier"); ok {
						out[file.FlatNodeText(id)] = true
					}
				}
			}
		}
	}
	return out
}

// recyclerEnclosingClass returns the innermost class-like node around n: a class, an object literal or a Java
// anonymous class.
func recyclerEnclosingClass(file *scanner.File, n uint32) uint32 {
	for cur, ok := file.FlatParent(n); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_literal", "object_declaration":
			return cur
		case "class_body":
			if p, ok := file.FlatParent(cur); ok && file.FlatType(p) == "object_creation_expression" {
				return p
			}
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// FindViewByIdCast
// ---------------------------------------------------------------------------

// findViewGenericIdentities are library methods `<T> T m(T, ...)`, keyed by class FQN and method name, with the
// argument indexes whose parameter is the type variable.
var findViewGenericIdentities = map[string]map[string][]int{
	"java.util.Objects":                                    {"requireNonNull": {0}, "requireNonNullElse": {0, 1}},
	"com.google.common.base.Preconditions":                 {"checkNotNull": {0}},
	"com.google.common.base.Verify":                        {"verifyNotNull": {0}},
	"androidx.core.util.Preconditions":                     {"checkNotNull": {0}},
	"androidx.core.util.ObjectsCompat":                     {"requireNonNull": {0}},
	"android.support.v4.util.Preconditions":                {"checkNotNull": {0}},
	"com.google.android.gms.common.internal.Preconditions": {"checkNotNull": {0}},
}

func (r *FindViewByIDCastRule) check(ctx *api.Context) {
	file := ctx.File
	call := ctx.Idx
	name := javaMethodInvocationName(file, call)
	if name != "findViewById" && name != "requireViewById" {
		return
	}
	// Only an unqualified call is the direct argument Lint looks at; `view.findViewById(x)` sits under its receiver.
	if inv, ok := lintCallAt(file, call); !ok || inv.ReceiverNode != 0 || len(inv.Args) != 1 {
		return
	}
	// Like Lint, a class's own findViewById does not change the verdict (recorded in FindViewByIdCast/lookalike).
	args, ok := file.FlatParent(call)
	if !ok || file.FlatType(args) != "argument_list" {
		return
	}
	surrounding, ok := file.FlatParent(args)
	if !ok || file.FlatType(surrounding) != "method_invocation" {
		return
	}
	// The surrounding call must be the receiver of a further call or field access.
	parent, ok := file.FlatParent(surrounding)
	if !ok || (file.FlatType(parent) != "method_invocation" && file.FlatType(parent) != "field_access") ||
		file.FlatNamedChild(parent, 0) != surrounding {
		return
	}
	inv, ok := lintCallAt(file, surrounding)
	if !ok {
		return
	}
	index := -1
	for i, a := range inv.Args {
		if a == call {
			index = i
		}
	}
	if index < 0 || !findViewGenericAt(file, inv, index) {
		return
	}
	f := lintFindingAtNode(ctx, call, "", "Add explicit cast here; won't compile with Java language level 1.8 without it")
	// Lint's fix inserts the cast and shortens the name when View is imported.
	cast := "(android.view.View) "
	if imported, ok := lintImportsOf(file).bySimple["View"]; ok && imported == "android.view.View" {
		cast = "(View) "
	}
	start := int(file.FlatStartByte(call))
	f.Fix = &scanner.Fix{ByteMode: true, StartByte: start, EndByte: start, Replacement: cast}
	ctx.Emit(f)
}

// findViewGenericAt reports whether the call's parameter at index is a type variable other than one bounded by View,
// and its return type is a type variable: a known library identity method, or a method declared in this file.
func findViewGenericAt(file *scanner.File, inv lintCall, index int) bool {
	for fqn, methods := range findViewGenericIdentities {
		indexes, ok := methods[inv.Name]
		if !ok {
			continue
		}
		owner := inv.ReceiverText
		if owner == "" {
			if imported := lintImportsOf(file).bySimple[inv.Name]; imported != fqn+"."+inv.Name {
				continue
			}
		} else if !lintTypeRefMatches(file, owner, fqn) {
			continue
		}
		for _, i := range indexes {
			if i == index && len(inv.Args) > i {
				return true
			}
		}
	}
	if inv.ReceiverText != "" && inv.ReceiverText != "this" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "method_declaration", func(m uint32) {
		if found || file.FlatChildTextOrEmpty(m, "identifier") != inv.Name {
			return
		}
		found = findViewGenericDeclaration(file, m, index, len(inv.Args))
	})
	return found
}

// findViewGenericDeclaration checks a same-file declaration `<T ...> T name(..., T p, ...)`.
func findViewGenericDeclaration(file *scanner.File, m uint32, index, arity int) bool {
	tparams, ok := file.FlatFindChild(m, "type_parameters")
	if !ok {
		return false
	}
	bounds := map[string]string{}
	for tp := file.FlatFirstChild(tparams); tp != 0; tp = file.FlatNextSib(tp) {
		if file.FlatType(tp) != "type_parameter" {
			continue
		}
		name := ""
		bound := ""
		for c := file.FlatFirstChild(tp); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "type_identifier", "identifier":
				if name == "" {
					name = file.FlatNodeText(c)
				}
			case "type_bound":
				bound = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(c)), "extends"))
			}
		}
		if name != "" {
			bounds[name] = bound
		}
	}
	ret := ""
	for c := file.FlatFirstChild(m); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "type_identifier" {
			ret = file.FlatNodeText(c)
			break
		} else if t == "identifier" || t == "formal_parameters" {
			break
		}
	}
	if _, ok := bounds[ret]; !ok {
		return false
	}
	params := recyclerParams(file, m)
	if len(params) != arity || index >= len(params) {
		return false
	}
	typ := lintChildTypeText(file, params[index])
	bound, ok := bounds[typ]
	if !ok {
		return false
	}
	erasure := strings.TrimSpace(strings.Split(bound, "&")[0])
	return erasure != "View" && erasure != "android.view.View"
}

// ---------------------------------------------------------------------------
// RemoteViewLayout
// ---------------------------------------------------------------------------

const remoteViewsFQN = "android.widget.RemoteViews"

var layoutFolderVersionRe = lazyre.New(`-v(\d+)(?:-|$)`)

func (r *RemoteViewLayoutRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "RemoteViews" || len(call.Args) != 2 || call.ReceiverNode != 0 ||
		!lintTypeRefMatches(file, call.TypeText, remoteViewsFQN) || ctx.ResourceIndex == nil {
		return
	}
	text := strings.TrimSpace(file.FlatNodeText(call.Args[1]))
	if !strings.HasPrefix(text, "R.layout.") || strings.HasPrefix(text, "android.R.") {
		return
	}
	layoutName := strings.TrimPrefix(text, "R.layout.")
	minSdk := 1
	if facts := lintProjectMinSdk(ctx); facts > 0 {
		minSdk = facts
	}
	// The calling module's own layout overrides a library's of the same name, so a layout from another module is
	// read only when the calling module has none.
	caller := remoteViewModuleDir(projectPath(ctx, file.Path))
	var own, other []*android.Layout
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		if strings.TrimSuffix(filepath.Base(layout.FilePath), ".xml") != layoutName {
			continue
		}
		if caller != "" && remoteViewModuleDir(projectPath(ctx, layout.FilePath)) == caller {
			own = append(own, layout)
		} else {
			other = append(other, layout)
		}
	}
	layouts := own
	if len(layouts) == 0 {
		layouts = other
	}
	tags := map[string]bool{}
	for _, layout := range layouts {
		folderMin := minSdk
		if m := layoutFolderVersionRe.FindStringSubmatch(filepath.Base(filepath.Dir(layout.FilePath))); m != nil {
			if v, err := strconv.Atoi(m[1]); err == nil && v > folderMin {
				folderMin = v
			}
		}
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "" && !remoteViewSupportedTag(v.Type, folderMin) {
				tags[v.Type] = true
			}
		})
	}
	if len(tags) == 0 {
		return
	}
	sorted := make([]string, 0, len(tags))
	for t := range tags {
		sorted = append(sorted, t)
	}
	sort.Strings(sorted)
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", fmt.Sprintf("`@layout/%s` includes views not allowed in a `RemoteView`: %s",
		layoutName, strings.Join(sorted, ", "))))
}

// remoteViewModuleDir returns the module directory of a project-relative source or resource path, the directory
// above its src/<set>/{java,kotlin,res} (or above a flat <module>/res; "." for the root module), or "" when the path
// has neither.
func remoteViewModuleDir(path string) string {
	// projectPath spells a path under the project root with a leading slash (`/app/src/main/...`).
	parts := strings.Split(strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "/"), "/")
	dir := func(n int) string {
		if n == 0 {
			return "."
		}
		return strings.Join(parts[:n], "/")
	}
	// The path is relative to the project root, so the first src/<set>/<kind> is the module's, even when a package
	// below it has a directory named src.
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "src" {
			switch parts[i+2] {
			case "java", "kotlin", "res":
				return dir(i)
			}
		}
	}
	if n := len(parts); n >= 3 && parts[n-3] == "res" {
		return dir(n - 3)
	}
	return ""
}

// lintProjectMinSdk returns the minSdk of the module that owns the file (the build's SDK levels, then the library
// facts), or 0 when unknown.
func lintProjectMinSdk(ctx *api.Context) int {
	if ctx.File == nil {
		return 0
	}
	return obsoleteSdkMinSdk(ctx)
}

// remoteViewSupportedTag mirrors isSupportedTag: the RemoteViews-safe layouts and widgets, the API 31 additions, and
// the inflater's meta tags.
func remoteViewSupportedTag(tag string, minSdk int) bool {
	if strings.HasPrefix(tag, "android.view.") || strings.HasPrefix(tag, "android.widget.") {
		return remoteViewSupportedTag(tag[strings.LastIndexByte(tag, '.')+1:], minSdk)
	}
	switch tag {
	case "AdapterViewFlipper", "FrameLayout", "GridLayout", "GridView", "LinearLayout", "ListView", "RelativeLayout",
		"StackView", "ViewFlipper", "AnalogClock", "Button", "Chronometer", "ImageButton", "ImageView", "ProgressBar",
		"TextClock", "TextView", "AbsoluteLayout", "ViewStub", "merge", "requestFocus", "include":
		return true
	case "CheckBox", "Switch", "RadioButton", "RadioGroup":
		return minSdk >= 31
	}
	return false
}

// ---------------------------------------------------------------------------
// DefaultUncaughtExceptionDelegation
// ---------------------------------------------------------------------------

func (r *DefaultUncaughtExceptionDelegationRule) check(ctx *api.Context) {
	type site struct {
		file *scanner.File
		call uint32
	}
	roots := map[string]string{}
	reads := map[string]bool{}
	var sets []site
	var setRoots []string
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil || isProjectTestFile(ctx, file.Path) ||
			!bytes.Contains(file.Content, uncaughtHandlerBytes) {
			continue
		}
		root := workModuleRoot(file.Path, roots)
		for _, t := range []string{"call_expression", "method_invocation"} {
			file.FlatWalkNodes(0, t, func(n uint32) {
				call, ok := lintCallAt(file, n)
				if !ok {
					return
				}
				switch call.Name {
				case "getDefaultUncaughtExceptionHandler":
					reads[root] = true
				case "setDefaultUncaughtExceptionHandler":
					if uncaughtArgCount(file, n, call) == 1 && uncaughtReceiverIsThread(file, call) {
						sets = append(sets, site{file, n})
						setRoots = append(setRoots, root)
					}
				}
			})
		}
	}
	for i, s := range sets {
		if reads[setRoots[i]] {
			continue
		}
		f := s.file
		ctx.Emit(scanner.Finding{File: f.Path, Line: f.FlatRow(s.call) + 1, Col: f.FlatCol(s.call) + 1,
			StartByte: int(f.FlatStartByte(s.call)), EndByte: int(f.FlatEndByte(s.call)),
			Message: "Must call `getDefaultUncaughtExceptionHandler()` to get the existing handler, and call " +
				"`existingHandler.uncaughtException(thread, throwable)` from your new handler"})
	}
}

var uncaughtHandlerBytes = []byte("DefaultUncaughtExceptionHandler")

// uncaughtArgCount counts a call's arguments, a Kotlin trailing lambda included.
func uncaughtArgCount(file *scanner.File, n uint32, call lintCall) int {
	count := len(call.Args)
	if file.Language != scanner.LangJava {
		file.FlatWalkNodes(n, "annotated_lambda", func(l uint32) {
			if p, ok := file.FlatParent(l); ok && file.FlatType(p) == "call_suffix" {
				if pp, ok := file.FlatParent(p); ok && pp == n {
					count++
				}
			}
		})
	}
	return count
}

// uncaughtReceiverIsThread reports a call written on java.lang.Thread.
func uncaughtReceiverIsThread(file *scanner.File, call lintCall) bool {
	recv := strings.TrimSpace(call.ReceiverText)
	if recv == "java.lang.Thread" {
		return true
	}
	return recv == "Thread" && lintSameFileClass(file, "Thread") == 0 &&
		(lintImportsOf(file).bySimple["Thread"] == "" || lintImportsOf(file).bySimple["Thread"] == "java.lang.Thread")
}

// ---------------------------------------------------------------------------
// WrongGradleMethod
// ---------------------------------------------------------------------------

const (
	firebaseAppDistributionName   = "firebaseAppDistribution"
	firebaseAppDistributionPrefix = "com.google.firebase.appdistribution.gradle."
)

// gradleBlock is one open closure in a build script: its name, the text before its brace, and where the name is.
type gradleBlock struct {
	name, header string
	line, col    int
}

var gradleContainerCallRe = lazyre.New(`^(getByName|create|maybeCreate|named|register)\s*\(\s*["']([^"']*)["']`)

func (r *WrongGradleMethodRule) check(ctx *api.Context) {
	content := ctx.GradleContent
	if !strings.Contains(content, "dependencies") && !strings.Contains(content, firebaseAppDistributionName) {
		return
	}
	kts := isKotlinDslScript(ctx.GradlePath)
	firebaseImported := strings.Contains(content, firebaseAppDistributionPrefix)
	var stack []gradleBlock
	inBlock := false
	for i, raw := range strings.Split(content, "\n") {
		var code string
		code, inBlock = blankGradleComments(strings.TrimSuffix(raw, "\r"), inBlock)
		start := 0
		var quote byte
		depth := 0
		for j := 0; j < len(code); j++ {
			c := code[j]
			if quote != 0 {
				switch c {
				case '\\':
					j++
				case quote:
					quote = 0
				}
				continue
			}
			switch c {
			case '"', '\'':
				quote = c
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			case ';':
				if depth == 0 {
					start = j + 1
				}
			case '{':
				if depth != 0 {
					continue
				}
				header := strings.TrimSpace(code[start:j])
				block := gradleBlock{name: lintGradleBlockName(code[start:j]), header: header, line: i + 1}
				if idx := strings.LastIndex(code[start:j], block.name); block.name != "" && idx >= 0 {
					block.col = utf8RuneCol(code, start+idx)
				}
				r.visitBlock(ctx, block, stack, kts, firebaseImported)
				stack = append(stack, block)
				start = j + 1
			case '}':
				if depth == 0 && len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				start = j + 1
			}
		}
	}
}

func utf8RuneCol(s string, byteOffset int) int {
	return len([]rune(s[:byteOffset])) + 1
}

// visitBlock checks a closure as checkCli does: statement is the block name, parent and grandparent the enclosing
// closures (control flow blocks have no name and are skipped).
func (r *WrongGradleMethodRule) visitBlock(ctx *api.Context, block gradleBlock, stack []gradleBlock, kts, firebaseImported bool) {
	if block.name == "" {
		return
	}
	var named []gradleBlock
	for i := len(stack) - 1; i >= 0 && len(named) < 2; i-- {
		if stack[i].name != "" {
			named = append(named, stack[i])
		}
	}
	switch block.name {
	case firebaseAppDistributionName:
		if kts && !firebaseImported {
			f := baseFinding(ctx.GradlePath, block.line, r.BaseRule,
				"This does not resolve to the right method; you need to explicitly add `import "+firebaseAppDistributionPrefix+
					firebaseAppDistributionName+"` to this file!")
			f.Col = block.col
			ctx.Emit(f)
		}
	case "dependencies":
		if len(named) < 2 {
			return
		}
		inBuildType := named[1].name == "buildTypes"
		inFlavor := named[1].name == "productFlavors"
		if !inBuildType && !inFlavor {
			return
		}
		container := named[0].name
		if m := gradleContainerCallRe.FindStringSubmatch(named[0].header); m != nil {
			container = m[2]
		}
		where, kind := "build types", "build type"
		if inFlavor {
			where, kind = "product flavors", "product flavor"
		}
		msg := "Suspicious receiver type; this does not apply to " + where + ". This will apply to a receiver of type " +
			"`Project`, found in one of the enclosing lambdas. Make sure it's declared in the right place in the file. " +
			"If you wanted a " + kind + " specific dependency, use `" + container + "Implementation` rather than " +
			"`implementation` in the top level `dependencies` block."
		f := baseFinding(ctx.GradlePath, block.line, r.BaseRule, msg)
		f.Col = block.col
		ctx.Emit(f)
	}
}
