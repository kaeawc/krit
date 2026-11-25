package rules

// Ports of the API-misuse checks bundled in androidx.fragment 1.9.1 (fragment-lint): UseRequireInsteadOfGet,
// UseGetLayoutInflater, OnCreateDialogIncorrectCallbackDetector (DialogFragmentCallbacksDetector) and
// AttachAndDetachInSameTransactionDetector (DetachAndAttachSameFragment).

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UseRequireInsteadOfGetRule mirrors UseRequireInsteadOfGet: a Fragment getter such as context or getActivity()
// asserted non-null with !!, checkNotNull or requireNonNull.
type UseRequireInsteadOfGetRule struct {
	FlatDispatchBase
	AndroidRule
}

// UseGetLayoutInflaterRule mirrors UseGetLayoutInflater: LayoutInflater.from(context) in a DialogFragment.
type UseGetLayoutInflaterRule struct {
	FlatDispatchBase
	AndroidRule
}

// DialogFragmentCallbacksDetectorRule mirrors DialogFragmentCallbacksDetector: setOnCancelListener or
// setOnDismissListener called from DialogFragment.onCreateDialog.
type DialogFragmentCallbacksDetectorRule struct {
	FlatDispatchBase
	AndroidRule
}

// DetachAndAttachSameFragmentRule mirrors DetachAndAttachSameFragment: detach and attach of the same fragment in one
// FragmentTransaction.
type DetachAndAttachSameFragmentRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a high base confidence: the getter must be a member of a class proven to extend Fragment.
func (r *UseRequireInsteadOfGetRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the receiver is the imported LayoutInflater class.
func (r *UseGetLayoutInflaterRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: like Lint, the check matches names.
func (r *DialogFragmentCallbacksDetectorRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a tier-2 (medium) base confidence: the transaction is followed through chains and locals of one
// function, and fragments are compared by their source text.
func (r *DetachAndAttachSameFragmentRule) Confidence() float64 { return api.ConfidenceMedium }

func registerAndroidXFragmentMiscChecks() {
	require := &UseRequireInsteadOfGetRule{AndroidRule: lintAndroidRule("UseRequireInsteadOfGet",
		"Use the 'require_____()' API rather than 'get____()' API for more descriptive error messages when it's null.",
		ALCCorrectness, ALSError, 6)}
	registerLintRule(require.AndroidRule, require, require.Confidence(), require.check,
		[]string{"postfix_expression", "directly_assignable_expression", "call_expression", "method_invocation"}, api.NeedsResolver, nil)

	inflater := &UseGetLayoutInflaterRule{AndroidRule: lintAndroidRule("UseGetLayoutInflater",
		"Use getLayoutInflater() to get the LayoutInflater instead of calling LayoutInflater.from(Context).",
		ALCCorrectness, ALSWarning, 9)}
	registerLintRule(inflater.AndroidRule, inflater, inflater.Confidence(), inflater.check,
		[]string{"call_expression", "method_invocation"}, 0, nil)

	callbacks := &DialogFragmentCallbacksDetectorRule{AndroidRule: lintAndroidRule("DialogFragmentCallbacksDetector",
		"Use onCancel() and onDismiss() instead of calling setOnCancelListener() and setOnDismissListener() from onCreateDialog()",
		ALCCorrectness, ALSWarning, 5)}
	registerLintRule(callbacks.AndroidRule, callbacks, callbacks.Confidence(), callbacks.check, []string{"class_declaration"}, 0, nil)

	detach := &DetachAndAttachSameFragmentRule{AndroidRule: lintAndroidRule("DetachAndAttachSameFragment",
		"Separate attach() and detach() into separate FragmentTransactions", ALCCorrectness, ALSWarning, 5)}
	registerLintRule(detach.AndroidRule, detach, detach.Confidence(), detach.check,
		[]string{"call_expression", "method_invocation"}, 0, nil)
}

// requirableFragmentGetters maps each Fragment getter, and its Kotlin property, to the require variant.
var requirableFragmentGetters = map[string]string{
	"getArguments": "requireArguments", "getContext": "requireContext", "getActivity": "requireActivity",
	"getFragmentManager": "requireFragmentManager", "getHost": "requireHost",
	"getParentFragment": "requireParentFragment", "getView": "requireView",
	"arguments": "requireArguments", "context": "requireContext", "activity": "requireActivity",
	"fragmentManager": "requireFragmentManager", "host": "requireHost",
	"parentFragment": "requireParentFragment", "view": "requireView",
}

// requireNullChecks are the null-check calls Lint rewrites; Kotlin's requireNotNull is not among them.
var requireNullChecks = map[string]bool{"checkNotNull": true, "requireNonNull": true}

func (r *UseRequireInsteadOfGetRule) check(ctx *api.Context) {
	file := ctx.File
	if file.FlatType(ctx.Idx) == "directly_assignable_expression" {
		r.checkAssignable(ctx)
		return
	}
	if file.FlatType(ctx.Idx) == "postfix_expression" {
		if file.FlatNodeText(fragmentLastChild(file, ctx.Idx)) != "!!" {
			return
		}
		operand := flatUnwrapParenExpr(file, file.FlatFirstChild(ctx.Idx))
		target, require, ok := requirableFragmentGetter(ctx, operand)
		if !ok {
			return
		}
		source := file.FlatNodeText(ctx.Idx)
		fixed := strings.ReplaceAll(removeSingleParentheses(source), target+"!!", require+"()")
		if fixed == removeSingleParentheses(source) {
			// The assertion is on something else built from the getter, such as `context?.x!!`, which Lint does not
			// report either.
			return
		}
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Use "+fixed+" instead of "+source))
		return
	}
	if fragmentIsLambdaWrapper(file, ctx.Idx) {
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || !requireNullChecks[call.Name] || len(call.Args) != 1 || fragmentHasTrailingLambda(file, ctx.Idx) {
		return
	}
	arg := flatUnwrapParenExpr(file, call.Args[0])
	target, require, ok := requirableFragmentGetter(ctx, arg)
	if !ok {
		return
	}
	fixed := strings.ReplaceAll(removeSingleParentheses(file.FlatNodeText(arg)), target, require+"()")
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Use "+fixed+" instead of "+file.FlatNodeText(ctx.Idx)))
}

// checkAssignable reports `view!!.x = …` and `child.view!!.x = …`: in an assignment target the grammar lists the
// receiver, the navigation suffixes and `!!` as siblings instead of nesting a postfix expression.
func (r *UseRequireInsteadOfGetRule) checkAssignable(ctx *api.Context) {
	file := ctx.File
	var kids []uint32
	for child := file.FlatFirstChild(ctx.Idx); child != 0; child = file.FlatNextSib(child) {
		kids = append(kids, child)
	}
	bang := -1
	for i, k := range kids {
		if file.FlatNodeText(k) == "!!" && !file.FlatIsNamed(k) {
			bang = i
			break
		}
	}
	var name string
	var receiver uint32
	switch {
	case bang == 1 && file.FlatType(kids[0]) == "simple_identifier":
		name = file.FlatNodeText(kids[0])
	case bang == 2 && file.FlatType(kids[1]) == "navigation_suffix" &&
		(file.FlatType(kids[0]) == "simple_identifier" || file.FlatType(kids[0]) == "this_expression"):
		id, ok := file.FlatFindChild(kids[1], "simple_identifier")
		if !ok || strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(kids[1])), "?.") {
			return
		}
		name, receiver = file.FlatNodeText(id), kids[0]
	default:
		return
	}
	target, require, ok := requirableFragmentMember(ctx, kids[0], name, receiver, false)
	if !ok {
		return
	}
	source := string(file.Content[file.FlatStartByte(kids[0]):file.FlatEndByte(kids[bang])])
	fixed := strings.ReplaceAll(removeSingleParentheses(source), target+"!!", require+"()")
	ctx.Emit(lintFindingAtNode(ctx, kids[0], "", "Use "+fixed+" instead of "+source))
}

// requirableFragmentGetter reports whether expr reads a requirable Fragment getter: `context`, `x.context`,
// `getContext()` or `x.getContext()` on a Fragment. target is the text Lint replaces (`context` or `getContext()`).
func requirableFragmentGetter(ctx *api.Context, expr uint32) (target, require string, ok bool) {
	file := ctx.File
	var name string
	var receiver uint32
	isCall := false
	switch file.FlatType(expr) {
	case "simple_identifier":
		name = file.FlatNodeText(expr)
	case "navigation_expression":
		suffix := fragmentLastChild(file, expr)
		if file.FlatType(suffix) != "navigation_suffix" || strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(suffix)), "?.") {
			return "", "", false
		}
		id, found := file.FlatFindChild(suffix, "simple_identifier")
		if !found {
			return "", "", false
		}
		name, receiver = file.FlatNodeText(id), file.FlatFirstChild(expr)
	case "call_expression", "method_invocation":
		call, found := lintCallAt(file, expr)
		if !found || len(call.Args) != 0 || !strings.HasPrefix(call.Name, "get") {
			return "", "", false
		}
		name, receiver, isCall = call.Name, call.ReceiverNode, true
	default:
		return "", "", false
	}
	return requirableFragmentMember(ctx, expr, name, receiver, isCall)
}

// requirableFragmentMember reports whether name, read at expr with the given receiver (0 for an implicit one), is a
// requirable getter of a Fragment.
func requirableFragmentMember(ctx *api.Context, expr uint32, name string, receiver uint32, isCall bool) (target, require string, ok bool) {
	file := ctx.File
	require, ok = requirableFragmentGetters[name]
	if !ok || isCall != strings.HasPrefix(name, "get") {
		return "", "", false
	}
	if receiver == 0 {
		if !isCall && fragmentNameIsLocal(file, expr, name) {
			// Lint skips local variables that shadow the property; krit also skips parameters.
			return "", "", false
		}
		cls, found := fragmentEnclosingClass(file, expr)
		if !found || !lintClassExtends(ctx, cls, fragmentClass, fragmentKnown) {
			return "", "", false
		}
	} else if !fragmentExpressionIsFragment(ctx, expr, receiver) {
		return "", "", false
	}
	if isCall {
		return name + "()", require, true
	}
	return name, require, true
}

// fragmentExpressionIsFragment reports whether expr is a Fragment instance: `this` in a Fragment, a Fragment-typed
// variable, or a parent-fragment accessor.
func fragmentExpressionIsFragment(ctx *api.Context, scope, expr uint32) bool {
	file := ctx.File
	expr = flatUnwrapParenExpr(file, expr)
	text := strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(expr)), "!!")
	switch text {
	case "this":
		cls, ok := fragmentEnclosingClass(file, expr)
		return ok && lintClassExtends(ctx, cls, fragmentClass, fragmentKnown)
	case "requireParentFragment()", "getParentFragment()", "parentFragment":
		return true
	}
	if !lintIdentOnly(text) {
		return false
	}
	typ := lintTrimGenerics(strings.TrimSuffix(lintDeclaredTypeText(file, scope, text), "?"))
	return typ != "" && lintTypeNameReaches(ctx, typ, fragmentClass, fragmentKnown, 0)
}

// fragmentNameIsLocal reports whether name at idx refers to a local variable or parameter of an enclosing function or
// lambda declared before idx.
func fragmentNameIsLocal(file *scanner.File, idx uint32, name string) bool {
	if lintNameIsParameterAt(file, idx, name) {
		return true
	}
	start := file.FlatStartByte(idx)
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_body", "class_declaration", "object_literal", "source_file":
			return false
		case "statements", "block":
			for stmt := file.FlatFirstChild(cur); stmt != 0 && file.FlatStartByte(stmt) < start; stmt = file.FlatNextSib(stmt) {
				if file.FlatType(stmt) != "property_declaration" {
					continue
				}
				if vd, ok := file.FlatFindChild(stmt, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
					return true
				}
			}
		case "lambda_literal":
			if params, ok := file.FlatFindChild(cur, "lambda_parameters"); ok {
				found := false
				file.FlatWalkNodes(params, "variable_declaration", func(vd uint32) {
					found = found || lintDeclaredName(file, vd) == name
				})
				if found {
					return true
				}
			}
		}
	}
	return false
}

// removeSingleParentheses mirrors the Lint helper of that name: it drops each `(` not directly followed by `)`, then
// each `)` not directly preceded by `(`, so `(context)!!` becomes `context!!` and `getContext()` keeps its `()`.
func removeSingleParentheses(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '(' && (i+1 >= len(s) || s[i+1] == ')') {
			b.WriteByte(s[i])
		} else if s[i] != '(' {
			b.WriteByte(s[i])
		}
	}
	t := b.String()
	b.Reset()
	for i := 0; i < len(t); i++ {
		if t[i] == ')' && (i == 0 || t[i-1] != '(') {
			continue
		}
		b.WriteByte(t[i])
	}
	return b.String()
}

func (r *UseGetLayoutInflaterRule) check(ctx *api.Context) {
	file := ctx.File
	if fragmentIsLambdaWrapper(file, ctx.Idx) {
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "from" || len(call.Args) != 1 || !lintTypeRefMatches(file, call.ReceiverText, "android.view.LayoutInflater") {
		return
	}
	cls, ok := fragmentEnclosingClass(file, ctx.Idx)
	if !ok {
		return
	}
	// Lint compares the simple name of the direct superclass with DialogFragment. Its Kotlin path also asks whether the
	// class extends "DialogFragment", but that is not a qualified name and never matches, so a subclass such as
	// AppCompatDialogFragment is not reported in either language.
	direct := false
	for _, ref := range lintDirectSupertypeRefs(file, cls) {
		direct = direct || ref.simple == "DialogFragment"
	}
	if !direct {
		return
	}
	if file.Language == scanner.LangJava {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Use of LayoutInflater.from("+strings.TrimSpace(file.FlatNodeText(call.Args[0]))+
			") detected. Consider using getLayoutInflater() instead"))
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Use of LayoutInflater.from(Context) detected. Consider using layoutInflater instead"))
}

func (r *DialogFragmentCallbacksDetectorRule) check(ctx *api.Context) {
	file := ctx.File
	// Lint compares the simple name of the first declared supertype.
	refs := lintDirectSupertypeRefs(file, ctx.Idx)
	if len(refs) == 0 || refs[0].simple != "DialogFragment" {
		return
	}
	for _, fn := range fragmentClassMethods(file, ctx.Idx) {
		if fragmentFunctionName(file, fn) != "onCreateDialog" {
			continue
		}
		file.FlatWalkAllNodes(fragmentFunctionBody(file, fn), func(n uint32) {
			if t := file.FlatType(n); (t != "call_expression" && t != "method_invocation") || fragmentIsLambdaWrapper(file, n) {
				return
			}
			call, ok := lintCallAt(file, n)
			if !ok {
				return
			}
			switch call.Name {
			case "setOnCancelListener":
				ctx.Emit(lintFindingAtNode(ctx, n, "", "Use onCancel() instead of calling setOnCancelListener() from onCreateDialog()"))
			case "setOnDismissListener":
				ctx.Emit(lintFindingAtNode(ctx, n, "", "Use onDismiss() instead of calling setOnDismissListener() from onCreateDialog()"))
			}
		})
	}
}

// fragmentManagerAccessors name the receivers that return a FragmentManager.
var fragmentManagerAccessors = map[string]bool{
	"parentFragmentManager": true, "childFragmentManager": true, "supportFragmentManager": true, "fragmentManager": true,
	"getParentFragmentManager()": true, "getChildFragmentManager()": true, "getSupportFragmentManager()": true,
	"getFragmentManager()": true, "requireFragmentManager()": true,
}

// fragmentTransactionBuilders are the FragmentTransaction methods that return the transaction, so a chain keeps it.
var fragmentTransactionBuilders = map[string]bool{
	"add": true, "replace": true, "remove": true, "attach": true, "detach": true, "show": true, "hide": true,
	"addToBackStack": true, "disallowAddToBackStack": true, "setCustomAnimations": true, "setTransition": true,
	"setReorderingAllowed": true, "setPrimaryNavigationFragment": true, "setMaxLifecycle": true, "runOnCommit": true,
	"addSharedElement": true, "setBreadCrumbTitle": true, "setBreadCrumbShortTitle": true, "setTransitionStyle": true,
}

func (r *DetachAndAttachSameFragmentRule) check(ctx *api.Context) {
	file := ctx.File
	if fragmentIsLambdaWrapper(file, ctx.Idx) {
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "beginTransaction" || len(call.Args) != 0 || call.ReceiverNode == 0 {
		return
	}
	if !fragmentManagerAccessors[fragmentReceiverLastSegment(call)] &&
		!lintTypeRefMatches(file, lintReceiverClassText(file, call), "androidx.fragment.app.FragmentManager") {
		return
	}
	fn, ok := flatEnclosingAncestor(file, ctx.Idx, "function_declaration", "method_declaration")
	if !ok {
		return
	}
	attach, detach := fragmentTransactionOps(file, fragmentFunctionBody(file, fn), ctx.Idx)
	if attach == "" || attach != detach {
		return
	}
	name := fragmentCallNameNode(file, ctx.Idx)
	ctx.Emit(lintFindingAtNode(ctx, name, "", "Calling detach() and attach() in the same FragmentTransaction is a no-op, "+
		"meaning it does not recreate the Fragment's view. If you would like the view to be recreated, separate these "+
		"operations into separate transactions."))
}

// fragmentTransactionOps follows the transaction begun at begin through call chains and local variables in body, and
// returns the argument text of the last attach and the last detach on it.
func fragmentTransactionOps(file *scanner.File, body, begin uint32) (attach, detach string) {
	tracked := map[uint32]bool{begin: true}
	names := map[string]bool{}
	type op struct {
		at         uint32
		name, frag string
	}
	var ops []op
	seen := map[uint32]bool{}
	for changed := true; changed; {
		changed = false
		file.FlatWalkAllNodes(body, func(n uint32) {
			switch file.FlatType(n) {
			case "property_declaration", "local_variable_declaration", "variable_declarator":
				if init := flatUnwrapParenExpr(file, lintInitializerAfterEquals(file, n)); init != 0 && tracked[init] {
					if name := fragmentDeclaredVariable(file, n); name != "" && !names[name] {
						names[name] = true
						changed = true
					}
				}
				return
			case "call_expression", "method_invocation":
			default:
				return
			}
			if seen[n] || fragmentIsLambdaWrapper(file, n) {
				return
			}
			c, ok := lintCallAt(file, n)
			if !ok || c.ReceiverNode == 0 {
				return
			}
			recv := flatUnwrapParenExpr(file, c.ReceiverNode)
			if !tracked[recv] && !names[strings.TrimSpace(c.ReceiverText)] {
				return
			}
			seen[n] = true
			changed = true
			if fragmentTransactionBuilders[c.Name] {
				tracked[n] = true
			}
			if (c.Name == "attach" || c.Name == "detach") && len(c.Args) > 0 {
				ops = append(ops, op{at: n, name: c.Name, frag: strings.TrimSpace(file.FlatNodeText(c.Args[0]))})
			}
		})
	}
	// Lint keeps the last attach and the last detach the flow reaches, in source order.
	var lastAttach, lastDetach uint32
	for _, o := range ops {
		switch {
		case o.name == "attach" && file.FlatStartByte(o.at) >= file.FlatStartByte(lastAttach):
			attach, lastAttach = o.frag, o.at
		case o.name == "detach" && file.FlatStartByte(o.at) >= file.FlatStartByte(lastDetach):
			detach, lastDetach = o.frag, o.at
		}
	}
	return attach, detach
}

// fragmentDeclaredVariable names the variable a Kotlin property or Java declarator declares.
func fragmentDeclaredVariable(file *scanner.File, decl uint32) string {
	switch file.FlatType(decl) {
	case "property_declaration":
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
			return lintDeclaredName(file, vd)
		}
	case "variable_declarator":
		if id, ok := file.FlatFindChild(decl, "identifier"); ok {
			return file.FlatNodeText(id)
		}
	}
	return ""
}

// fragmentCallNameNode returns the method-name identifier of a call, which Lint's getNameLocation points at.
func fragmentCallNameNode(file *scanner.File, call uint32) uint32 {
	if file.FlatType(call) == "method_invocation" {
		var name uint32
		for child := file.FlatFirstChild(call); child != 0 && file.FlatType(child) != "argument_list"; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "identifier" {
				name = child
			}
		}
		if name != 0 {
			return name
		}
		return call
	}
	callee := file.FlatFirstChild(call)
	if file.FlatType(callee) == "navigation_expression" {
		if suffix := fragmentLastChild(file, callee); suffix != 0 {
			if id, ok := file.FlatFindChild(suffix, "simple_identifier"); ok {
				return id
			}
		}
	}
	return callee
}

// fragmentLastChild returns the last child of node, named or not.
func fragmentLastChild(file *scanner.File, node uint32) uint32 {
	var last uint32
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		last = child
	}
	return last
}
