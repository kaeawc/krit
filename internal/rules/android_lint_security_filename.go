package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnsanitizedFilenameFromContentProviderRule mirrors Lint's UnsanitizedFilenameFromContentProvider
// (UnsafeFilenameDetector): a ContentProvider display name read from a Cursor and passed to a java.io.File
// constructor before anything could have sanitized it.
type UnsanitizedFilenameFromContentProviderRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the Cursor receiver is proven from declarations, and the
// data flow is followed through local variables of one function.
func (r *UnsanitizedFilenameFromContentProviderRule) Confidence() float64 {
	return api.ConfidenceMedium
}

const (
	cursorFQN          = "android.database.Cursor"
	contentResolverFQN = "android.content.ContentResolver"
	displayNameColumn  = "_display_name"
)

func (r *UnsanitizedFilenameFromContentProviderRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || (call.Name != "getColumnIndex" && call.Name != "getColumnIndexOrThrow") || len(call.Args) != 1 {
		return
	}
	fn := filenameFlowScope(file, ctx.Idx)
	if fn == 0 || !cursorReceiver(file, call) || !displayNameArgument(file, call.Args[0]) {
		return
	}
	// The column index flows into Cursor.getString(int); each such call starts a flow of the filename.
	lintTrackFlow(file, fn, ctx.Idx, func(ev lintFlowEvent) {
		if ev.receiver {
			return
		}
		get, ok := lintCallAt(file, ev.call)
		if !ok || get.Name != "getString" || len(get.Args) != 1 || !cursorReceiver(file, get) {
			return
		}
		r.checkFilename(ctx, fn, ev.call)
	})
}

// checkFilename follows the value of one getString call: it reports the first uses that reach a File constructor,
// until the value is used in any other call, which Lint takes as a possible sanitization.
func (r *UnsanitizedFilenameFromContentProviderRule) checkFilename(ctx *api.Context, fn, getString uint32) {
	file := ctx.File
	sanitized := false
	lintTrackFlow(file, fn, getString, func(ev lintFlowEvent) {
		if !ev.receiver && !sanitized && fileConstructorCall(file, ev.call) {
			ref := strings.TrimSpace(file.FlatNodeText(ev.ref))
			ctx.Emit(lintFindingAtNode(ctx, ev.ref, "", "Using `"+ref+"` is unsafe as it is a filename obtained "+
				"directly from a `ContentProvider`. You should sanitize it before using it for creating a `File`."))
			return
		}
		sanitized = true
	})
}

// filenameFlowScope returns the function, method or constructor whose body Lint's data flow walks.
func filenameFlowScope(file *scanner.File, idx uint32) uint32 {
	fn, _ := flatEnclosingAncestor(file, idx, "function_declaration", "method_declaration", "constructor_declaration",
		"secondary_constructor")
	return fn
}

// displayNameArgument reports whether the column argument evaluates to "_display_name": a literal, a same-file
// constant, or the DISPLAY_NAME column constants of android.provider.
func displayNameArgument(file *scanner.File, arg uint32) bool {
	if v, ok := lintEvalString(file, arg); ok && v == displayNameColumn {
		return true
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(arg)), "")
	last := text[strings.LastIndex(text, ".")+1:]
	if last != "DISPLAY_NAME" && last != "COLUMN_DISPLAY_NAME" {
		return false
	}
	owner := strings.TrimSuffix(text, "."+last)
	if owner == text {
		// A bare constant needs a static import of one of the column classes.
		full, ok := lintImportsOf(file).bySimple[last]
		if !ok {
			return false
		}
		owner = strings.TrimSuffix(full, "."+last)
		text = full
	}
	for _, cls := range []string{"android.provider.OpenableColumns", "android.provider.MediaStore",
		"android.provider.DocumentsContract"} {
		head := owner
		if i := strings.Index(owner, "."); i >= 0 && !strings.HasPrefix(owner, "android.") {
			head = owner[:i]
		}
		if owner == cls || strings.HasPrefix(owner, cls+".") || lintTypeRefMatches(file, head, cls) {
			return true
		}
	}
	return false
}

// cursorReceiver reports whether the call's receiver is an android.database.Cursor: declared with that type, or the
// parameter of a use/let/also lambda on such a value or on ContentResolver.query.
func cursorReceiver(file *scanner.File, call lintCall) bool {
	if call.ReceiverNode == 0 {
		return false
	}
	if typ := lintReceiverTypeText(file, call); typ != "" {
		return lintTypeRefMatches(file, strings.TrimSuffix(strings.TrimSpace(typ), "?"), cursorFQN)
	}
	name := strings.TrimSpace(call.ReceiverText)
	if !lintIdentOnly(name) {
		return false
	}
	lambda, ok := lintLambdaBinding(file, call.Idx, name)
	if !ok {
		return false
	}
	scopeCall, ok := flatEnclosingAncestor(file, lambda, "call_expression")
	if !ok {
		return false
	}
	scope, ok := lintCallAt(file, scopeCall)
	if !ok || (scope.Name != "use" && scope.Name != "let" && scope.Name != "also") || scope.ReceiverNode == 0 {
		return false
	}
	recv := lintSkipParensJS(file, scope.ReceiverNode)
	if inner, ok := lintCallAt(file, recv); ok && inner.Name == "query" {
		return contentResolverReceiver(file, inner)
	}
	return cursorReceiver(file, lintCall{File: file, Idx: scopeCall, ReceiverNode: scope.ReceiverNode,
		ReceiverText: scope.ReceiverText})
}

// contentResolverReceiver reports whether a query call is made on a ContentResolver.
func contentResolverReceiver(file *scanner.File, call lintCall) bool {
	recv := strings.TrimSpace(call.ReceiverText)
	recv = strings.TrimSuffix(recv, "!!")
	if recv == "contentResolver" || recv == "getContentResolver()" || strings.HasSuffix(recv, ".contentResolver") ||
		strings.HasSuffix(recv, ".getContentResolver()") {
		return true
	}
	if typ := lintReceiverTypeText(file, call); typ != "" {
		return lintTypeRefMatches(file, strings.TrimSuffix(strings.TrimSpace(typ), "?"), contentResolverFQN)
	}
	return false
}

// lintLambdaBinding returns the innermost Kotlin lambda enclosing idx that binds name, as its declared parameter or
// as the implicit `it`.
func lintLambdaBinding(file *scanner.File, idx uint32, name string) (uint32, bool) {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "lambda_literal" {
			continue
		}
		params, hasParams := file.FlatFindChild(cur, "lambda_parameters")
		if !hasParams {
			if name == "it" {
				return cur, true
			}
			continue
		}
		if lintParamsDeclare(file, params, name) {
			return cur, true
		}
	}
	return 0, false
}

// fileConstructorCall reports whether the call constructs a java.io.File or a class in the file that extends it.
func fileConstructorCall(file *scanner.File, idx uint32) bool {
	call, ok := lintCallAt(file, idx)
	if !ok {
		return false
	}
	switch file.FlatType(idx) {
	case "object_creation_expression":
	case "call_expression":
		if call.ReceiverNode != 0 && !strings.Contains(call.TypeText, ".") {
			return false
		}
	default:
		return false
	}
	return lintTypeExtendsFile(file, call.TypeText, 0)
}

func lintTypeExtendsFile(file *scanner.File, typ string, depth int) bool {
	typ = strings.TrimSpace(typ)
	if typ == "" || depth > 6 {
		return false
	}
	if !strings.Contains(typ, ".") {
		if cls := lintSameFileClass(file, typ); cls != 0 {
			for _, super := range javascriptInterfaceSuperclassNames(file, cls) {
				if lintTypeExtendsFile(file, super, depth+1) {
					return true
				}
			}
			return false
		}
	}
	return lintTypeRefMatches(file, typ, "java.io.File")
}

// lintFlowEvent is a use of a tracked value: as the receiver of a call, or as one of its arguments.
type lintFlowEvent struct {
	call, ref uint32
	receiver  bool
}

// lintTrackFlow is a small port of Lint's DataFlowAnalyzer over one function body. It tracks the start expression
// and the local variables it is assigned to (reassigning a variable drops it), and calls fn, in source order, for
// every call that has a tracked value as its receiver or as an argument. Lambdas and local functions are walked, as
// Lint walks them; local classes are not entered.
func lintTrackFlow(file *scanner.File, root, start uint32, fn func(lintFlowEvent)) {
	f := &lintFlow{file: file, root: root, start: start, tracked: map[string]bool{}, fn: fn}
	f.visit(root)
}

type lintFlow struct {
	file        *scanner.File
	root, start uint32
	tracked     map[string]bool
	fn          func(lintFlowEvent)
}

func (f *lintFlow) isTracked(n uint32) bool {
	n = lintSkipParensJS(f.file, n)
	if n == f.start {
		return true
	}
	switch f.file.FlatType(n) {
	case "simple_identifier", "identifier":
		return f.tracked[f.file.FlatNodeText(n)]
	}
	return false
}

func (f *lintFlow) visit(n uint32) {
	file := f.file
	for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
		// Lint's analyzer also walks local functions, which can capture the value; local classes are separate
		// declarations.
		switch file.FlatType(child) {
		case "class_declaration", "object_declaration", "local_class_declaration":
			if n != f.root {
				continue
			}
		}
		f.visit(child)
	}
	switch file.FlatType(n) {
	case "property_declaration":
		if vd, ok := file.FlatFindChild(n, "variable_declaration"); ok {
			f.tracked[lintDeclaredName(file, vd)] = f.isTracked(lintInitializerAfterEquals(file, n))
		}
	case "variable_declarator":
		if id, ok := file.FlatFindChild(n, "identifier"); ok {
			f.tracked[file.FlatNodeText(id)] = f.isTracked(lintInitializerAfterEquals(file, n))
		}
	case "assignment", "assignment_expression":
		f.assign(n)
	case "call_expression", "method_invocation", "object_creation_expression":
		f.call(n)
	}
}

// assign drops a variable that is reassigned, or tracks it when the new value is tracked.
func (f *lintFlow) assign(n uint32) {
	file := f.file
	lhs := file.FlatNamedChild(n, 0)
	rhs := file.FlatNamedChild(n, file.FlatNamedChildCount(n)-1)
	switch file.FlatType(lhs) {
	case "directly_assignable_expression", "identifier", "simple_identifier":
		if name := strings.TrimSpace(file.FlatNodeText(lhs)); lintIdentOnly(name) {
			f.tracked[name] = f.isTracked(rhs)
		}
	}
}

// call reports the tracked values a call uses as its receiver and as its arguments.
func (f *lintFlow) call(n uint32) {
	call, ok := lintCallAt(f.file, n)
	if !ok {
		return
	}
	if call.ReceiverNode != 0 && f.isTracked(call.ReceiverNode) {
		f.fn(lintFlowEvent{call: n, ref: call.ReceiverNode, receiver: true})
	}
	for _, arg := range call.Args {
		if arg != 0 && f.isTracked(arg) {
			f.fn(lintFlowEvent{call: n, ref: lintSkipParensJS(f.file, arg)})
		}
	}
}
