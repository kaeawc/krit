package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/semantics"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func semanticTargetOwnerMatches(target string, owners ...string) bool {
	target = strings.TrimSpace(strings.ReplaceAll(target, "#", "."))
	if cut := strings.Index(target, "("); cut >= 0 {
		target = target[:cut]
	}
	for _, owner := range owners {
		owner = strings.TrimSpace(strings.ReplaceAll(owner, "#", "."))
		if target == owner || strings.HasPrefix(target, owner+".") {
			return true
		}
	}
	return false
}

func semanticResolvedTargetMatches(ctx *v2.Context, call uint32, fqns ...string) bool {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || !target.Resolved {
		return false
	}
	got := strings.TrimSpace(strings.ReplaceAll(target.QualifiedName, "#", "."))
	for _, want := range fqns {
		want = strings.TrimSpace(strings.ReplaceAll(want, "#", "."))
		if got == want || strings.HasSuffix(got, "."+want) {
			return true
		}
	}
	return false
}

func semanticCallReceiverTypeMatches(ctx *v2.Context, call uint32, types ...string) bool {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || !target.Receiver.Valid() {
		return false
	}
	typ, ok := semantics.ExpressionType(ctx, target.Receiver.Node)
	if !ok {
		return false
	}
	return semanticTypeNameMatches(typ.Name, types...) || semanticTypeNameMatches(typ.FQN, types...)
}

func semanticTypeNameMatches(got string, wants ...string) bool {
	got = strings.TrimSpace(strings.ReplaceAll(got, "$", "."))
	if got == "" {
		return false
	}
	for _, want := range wants {
		want = strings.TrimSpace(strings.ReplaceAll(want, "$", "."))
		if got == want || strings.HasSuffix(got, "."+want) {
			return true
		}
	}
	return false
}

func semanticCallTargetOrReceiverType(ctx *v2.Context, call uint32, owners []string, receiverTypes []string) bool {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return false
	}
	if target.Resolved {
		return semanticTargetOwnerMatches(target.QualifiedName, owners...)
	}
	return semanticCallReceiverTypeMatches(ctx, call, receiverTypes...)
}

func obsoleteComposeModifierCall(ctx *v2.Context, call uint32) (name string, replacement string, ok bool) {
	file := ctx.File
	name = flatCallExpressionName(file, call)
	replacements := map[string]string{
		"preferredWidth":  "width",
		"preferredHeight": "height",
		"preferredSize":   "size",
	}
	replacement, ok = replacements[name]
	if !ok {
		return "", "", false
	}
	target, targetOK := semantics.ResolveCallTarget(ctx, call)
	if targetOK && target.Resolved {
		if semanticTargetOwnerMatches(target.QualifiedName, "androidx.compose.ui.Modifier", "androidx.compose.foundation.layout") {
			return name, replacement, true
		}
		return "", "", false
	}
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return "", "", false
	}
	navSegments := flatNavigationChainIdentifiers(file, nav)
	if len(navSegments) >= 2 && navSegments[0] == "Modifier" && navSegments[len(navSegments)-1] == name {
		return name, replacement, true
	}
	receiver := file.FlatNamedChild(nav, 0)
	if receiver != 0 {
		segments := flatNavigationChainIdentifiers(file, receiver)
		if len(segments) > 0 && segments[0] == "Modifier" {
			return name, replacement, true
		}
		if semanticTypeNameMatches(semanticReferenceTypeName(ctx, receiver), "androidx.compose.ui.Modifier", "Modifier") {
			return name, replacement, true
		}
	}
	return "", "", false
}

func semanticReferenceTypeName(ctx *v2.Context, node uint32) string {
	typ, ok := semantics.ExpressionType(ctx, node)
	if !ok {
		return ""
	}
	if typ.FQN != "" {
		return typ.FQN
	}
	return typ.Name
}

func classExtendsAnyFlat(file *scanner.File, class uint32, names ...string) bool {
	for child := file.FlatFirstChild(class); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		typeName := viewConstructorSupertypeNameFlat(file, child)
		if semanticTypeNameMatches(typeName, names...) {
			return true
		}
	}
	return false
}

func classHasMemberFunctionFlat(file *scanner.File, class uint32, name string) bool {
	body, _ := file.FlatFindChild(class, "class_body")
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "function_declaration", func(fn uint32) {
		if found {
			return
		}
		if extractIdentifierFlat(file, fn) == name {
			found = true
		}
	})
	return found
}

func classHasNestedViewHolderFlat(file *scanner.File, class uint32) bool {
	body, _ := file.FlatFindChild(class, "class_body")
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "class_declaration", func(child uint32) {
		if found || child == class {
			return
		}
		name := extractIdentifierFlat(file, child)
		if strings.Contains(name, "ViewHolder") || classExtendsAnyFlat(file, child, "RecyclerView.ViewHolder", "ViewHolder") {
			found = true
		}
	})
	return found
}

func isRecyclerAdapterClassFlat(ctx *v2.Context, class uint32) bool {
	file := ctx.File
	if ctx.Resolver != nil {
		name := extractIdentifierFlat(file, class)
		if info := ctx.Resolver.ClassHierarchy(name); info != nil {
			for _, st := range info.Supertypes {
				if semanticTypeNameMatches(st,
					"androidx.recyclerview.widget.RecyclerView.Adapter",
					"android.support.v7.widget.RecyclerView.Adapter",
					"android.widget.BaseAdapter",
					"RecyclerView.Adapter",
					"BaseAdapter",
					"Adapter",
				) {
					return true
				}
			}
		}
	}
	return classExtendsAnyFlat(file, class, "RecyclerView.Adapter", "BaseAdapter", "Adapter")
}

func logCallIsAndroidLog(ctx *v2.Context, call uint32) bool {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || target.CalleeName == "" || !logLevelNames[target.CalleeName] {
		return false
	}
	if target.Resolved {
		return semanticTargetOwnerMatches(target.QualifiedName, "android.util.Log")
	}
	receiver := flatReceiverNameFromCall(ctx.File, call)
	if receiver != "Log" && receiver != "android.util.Log" {
		return false
	}
	if ctx.Resolver != nil {
		if fqn := ctx.Resolver.ResolveImport("Log", ctx.File); fqn != "" {
			return fqn == "android.util.Log"
		}
		return false
	}
	return receiver == "Log" || receiver == "android.util.Log"
}

func hasAndroidLogGuardFlat(ctx *v2.Context, call uint32) bool {
	file := ctx.File
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "if_expression":
			found := false
			file.FlatWalkNodes(p, "call_expression", func(c uint32) {
				if found || c == call || flatCallExpressionName(file, c) != "isLoggable" {
					return
				}
				if semanticResolvedTargetMatches(ctx, c, "android.util.Log.isLoggable") ||
					(flatReceiverNameFromCall(file, c) == "Log" && (ctx.Resolver == nil || ctx.Resolver.ResolveImport("Log", file) == "android.util.Log")) {
					found = true
				}
			})
			if found {
				return true
			}
		case "function_declaration", "class_declaration", "lambda_literal", "source_file":
			return false
		}
	}
	return false
}

func wakeLockAcquireCall(ctx *v2.Context, call uint32) bool {
	if flatCallExpressionName(ctx.File, call) != "acquire" {
		return false
	}
	if semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.os.PowerManager.WakeLock"},
		[]string{"android.os.PowerManager.WakeLock", "WakeLock"},
	) {
		return true
	}
	return receiverDeclaredFromCall(ctx.File, call, "newWakeLock")
}

func wakeLockReleaseCallOnSameReceiver(ctx *v2.Context, acquire, release uint32) bool {
	if flatCallExpressionName(ctx.File, release) != "release" {
		return false
	}
	if !semanticCallTargetOrReceiverType(ctx, release,
		[]string{"android.os.PowerManager.WakeLock"},
		[]string{"android.os.PowerManager.WakeLock", "WakeLock"},
	) {
		if !receiverDeclaredFromCall(ctx.File, release, "newWakeLock") {
			return false
		}
	}
	ar := semantics.ReferenceName(ctx.File, semanticsReceiverNode(ctx, acquire))
	rr := semantics.ReferenceName(ctx.File, semanticsReceiverNode(ctx, release))
	return ar != "" && ar == rr
}

func semanticsReceiverNode(ctx *v2.Context, call uint32) uint32 {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || !target.Receiver.Valid() {
		return 0
	}
	return target.Receiver.Node
}

func astReceiverNodeFromCall(file *scanner.File, call uint32) uint32 {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return 0
	}
	var receiver uint32
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "navigation_suffix" {
			break
		}
		receiver = child
	}
	return receiver
}

func receiverContainsCallName(file *scanner.File, call uint32, names ...string) bool {
	receiver := semanticsReceiverNode(&v2.Context{File: file}, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(file, call)
	}
	if receiver == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(receiver, "call_expression", func(candidate uint32) {
		if found {
			return
		}
		callee := flatCallExpressionName(file, candidate)
		for _, name := range names {
			if callee == name {
				found = true
				return
			}
		}
	})
	return found
}

func setJavaScriptEnabledCall(ctx *v2.Context, call uint32) bool {
	if flatCallExpressionName(ctx.File, call) != "setJavaScriptEnabled" {
		return false
	}
	if semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.webkit.WebSettings"},
		[]string{"android.webkit.WebSettings", "WebSettings"},
	) {
		return true
	}
	receiver := semanticsReceiverNode(ctx, call)
	return receiver != 0 && navigationReceiverHasTypedRoot(ctx.File, receiver, "WebView") && navigationChainContainsSegment(ctx.File, receiver, "settings")
}

func webSettingsAssignmentTarget(ctx *v2.Context, assignment uint32) bool {
	file := ctx.File
	target, _ := file.FlatFindChild(assignment, "directly_assignable_expression")
	if target == 0 || finalSimpleIdentifier(file, target) != "javaScriptEnabled" {
		return false
	}
	if directlyAssignableWebSettingsTarget(file, assignment, target) {
		return true
	}
	receiver := assignmentReceiverBeforeFinal(file, target)
	if receiver == 0 {
		return false
	}
	if semanticTypeNameMatches(semanticReferenceTypeName(ctx, receiver), "android.webkit.WebSettings", "WebSettings") {
		return true
	}
	if file.FlatType(receiver) == "navigation_expression" {
		return navigationReceiverHasTypedRoot(file, receiver, "WebView") && navigationChainContainsSegment(file, receiver, "settings")
	}
	foundWebSettingsReceiver := false
	file.FlatWalkNodes(target, "navigation_expression", func(nav uint32) {
		if foundWebSettingsReceiver {
			return
		}
		if semanticTypeNameMatches(semanticReferenceTypeName(ctx, nav), "android.webkit.WebSettings", "WebSettings") ||
			(navigationReceiverHasTypedRoot(file, nav, "WebView") && navigationChainContainsSegment(file, nav, "settings")) {
			foundWebSettingsReceiver = true
		}
	})
	if foundWebSettingsReceiver {
		return true
	}
	name := semantics.ReferenceName(file, receiver)
	fn, ok := flatEnclosingFunction(file, assignment)
	return ok && parameterHasTypeFlat(file, fn, name, "WebSettings")
}

func directlyAssignableWebSettingsTarget(file *scanner.File, assignment uint32, target uint32) bool {
	segments := directlyAssignableSegments(file, target)
	if len(segments) < 2 || segments[len(segments)-1] != "javaScriptEnabled" {
		return false
	}
	fn, ok := flatEnclosingFunction(file, assignment)
	if !ok {
		return false
	}
	if len(segments) == 2 {
		return parameterHasTypeFlat(file, fn, segments[0], "WebSettings")
	}
	return navigationSegmentsContain(segments[:len(segments)-1], "settings") && parameterHasTypeFlat(file, fn, segments[0], "WebView")
}

func directlyAssignableSegments(file *scanner.File, target uint32) []string {
	if file.FlatType(target) != "directly_assignable_expression" {
		return nil
	}
	var out []string
	for child := file.FlatFirstChild(target); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			out = append(out, file.FlatNodeText(child))
		case "navigation_suffix":
			if ident, ok := file.FlatFindChild(child, "simple_identifier"); ok {
				out = append(out, file.FlatNodeText(ident))
			}
		case "navigation_expression":
			out = append(out, flatNavigationChainIdentifiers(file, child)...)
		}
	}
	return out
}

func assignmentReceiverBeforeFinal(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "directly_assignable_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return assignmentReceiverBeforeFinal(file, child)
			}
		}
	case "navigation_expression":
		var receiver uint32
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if file.FlatType(child) == "navigation_suffix" {
				break
			}
			receiver = child
		}
		return receiver
	}
	return 0
}

func apiGuardedByVersionCheckFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if declarationHasApiGuardAnnotationFlat(file, p) {
			return true
		}
		switch file.FlatType(p) {
		case "annotation":
			name := annotationFinalName(file, p)
			if name == "RequiresApi" || name == "TargetApi" {
				return true
			}
		case "if_expression", "when_expression":
			found := false
			file.FlatWalkAllNodes(p, func(n uint32) {
				if found || n == idx {
					return
				}
				if file.FlatType(n) == "navigation_expression" {
					segments := flatNavigationChainIdentifiers(file, n)
					if len(segments) >= 3 && segments[0] == "Build" && segments[1] == "VERSION" && segments[2] == "SDK_INT" {
						found = true
					}
				}
			})
			if found {
				return true
			}
		case "source_file":
			return false
		}
	}
	return false
}

func declarationHasApiGuardAnnotationFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "function_declaration", "class_declaration", "object_declaration", "property_declaration", "variable_declaration":
	default:
		return false
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(mods, "annotation", func(ann uint32) {
		if found {
			return
		}
		name := annotationFinalName(file, ann)
		if name == "RequiresApi" || name == "TargetApi" {
			found = true
		}
	})
	return found
}

func nodeHasAncestorTypeFlat(file *scanner.File, idx uint32, types ...string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		pt := file.FlatType(p)
		for _, typ := range types {
			if pt == typ {
				return true
			}
		}
	}
	return false
}

func apiNodeNameFlat(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "call_expression":
		return flatCallExpressionName(file, idx)
	case "simple_identifier", "type_identifier":
		return file.FlatNodeText(idx)
	case "user_type", "navigation_expression":
		if name := flatNavigationExpressionLastIdentifier(file, idx); name != "" {
			return name
		}
		return strings.TrimSpace(file.FlatNodeText(idx))
	}
	return semantics.ReferenceName(file, idx)
}

func inlinedApiReferenceNameFlat(file *scanner.File, idx uint32) string {
	segments := flatNavigationChainIdentifiers(file, idx)
	if len(segments) > 0 {
		return strings.Join(segments, ".")
	}
	return apiNodeNameFlat(file, idx)
}

func sharedPrefsEditorCall(ctx *v2.Context, call uint32) bool {
	name := flatCallExpressionName(ctx.File, call)
	if !isSharedPrefsPutMethod(name) && !isSharedPrefsGetMethod(name) {
		return false
	}
	return semanticCallTargetOrReceiverType(ctx, call,
		[]string{"android.content.SharedPreferences.Editor", "android.content.SharedPreferences"},
		[]string{"android.content.SharedPreferences.Editor", "android.content.SharedPreferences", "Editor", "SharedPreferences"},
	) || sameFileFunctionDeclaredInOwner(ctx.File, call, "Editor", "SharedPreferences")
}

func encryptedStorageReceiver(ctx *v2.Context, call uint32) bool {
	receiver := semanticsReceiverNode(ctx, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(ctx.File, call)
		if receiver == 0 {
			return false
		}
	}
	typ, ok := semantics.ExpressionType(ctx, receiver)
	if !ok {
		return receiverChainHasQualifiedRoot(ctx.File, receiver, "EncryptedSharedPreferences", "EncryptedFile")
	}
	return semanticTypeNameMatches(typ.Name, "EncryptedSharedPreferences", "EncryptedFile") ||
		semanticTypeNameMatches(typ.FQN, "androidx.security.crypto.EncryptedSharedPreferences", "androidx.security.crypto.EncryptedFile")
}

func plainFileWriteCall(ctx *v2.Context, call uint32) bool {
	name := flatCallExpressionName(ctx.File, call)
	if name != "writeText" && name != "writeBytes" {
		return false
	}
	return semanticCallTargetOrReceiverType(ctx, call,
		[]string{"java.io.File", "kotlin.io.FilesKt"},
		[]string{"java.io.File", "File"},
	) || receiverDeclaredFromCall(ctx.File, call, "File") || receiverContainsCallName(ctx.File, call, "File")
}

func sensitiveFileReceiverLiteral(ctx *v2.Context, call uint32) string {
	receiver := semanticsReceiverNode(ctx, call)
	if receiver == 0 {
		receiver = astReceiverNodeFromCall(ctx.File, call)
	}
	if receiver == 0 {
		return ""
	}
	if ctx.File.FlatType(receiver) != "call_expression" || flatCallExpressionName(ctx.File, receiver) != "File" {
		found := uint32(0)
		ctx.File.FlatWalkNodes(receiver, "call_expression", func(candidate uint32) {
			if found == 0 && flatCallExpressionName(ctx.File, candidate) == "File" {
				found = candidate
			}
		})
		receiver = found
		if receiver == 0 {
			return ""
		}
	}
	_, args := flatCallExpressionParts(ctx.File, receiver)
	if args == 0 {
		return ""
	}
	for arg := ctx.File.FlatFirstChild(args); arg != 0; arg = ctx.File.FlatNextSib(arg) {
		if ctx.File.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(ctx.File, arg)
		if expr != 0 && ctx.File.FlatType(expr) == "string_literal" {
			return stringLiteralContent(ctx.File, expr)
		}
	}
	return ""
}

func mainDispatcherReferenceFlat(ctx *v2.Context, idx uint32) bool {
	file := ctx.File
	if file.FlatType(idx) != "navigation_expression" {
		return false
	}
	segments := flatNavigationChainIdentifiers(file, idx)
	if len(segments) != 2 || segments[0] != "Dispatchers" || segments[1] != "Main" {
		return false
	}
	if ctx.Resolver != nil {
		if typ, ok := semantics.ExpressionType(ctx, idx); ok && semanticTypeNameMatches(typ.FQN, "kotlinx.coroutines.MainCoroutineDispatcher", "MainCoroutineDispatcher") {
			return true
		}
		if fqn := ctx.Resolver.ResolveImport("Dispatchers", file); fqn != "" {
			return fqn == "kotlinx.coroutines.Dispatchers"
		}
		return false
	}
	return false
}

func receiverDeclaredFromCall(file *scanner.File, call uint32, initializerCallee string) bool {
	receiver := semanticsReceiverNode(&v2.Context{File: file}, call)
	name := semantics.ReferenceName(file, receiver)
	if name == "" {
		return false
	}
	fn, ok := flatEnclosingFunction(file, call)
	if !ok {
		return false
	}
	found := false
	file.FlatWalkAllNodes(fn, func(n uint32) {
		if found || file.FlatStartByte(n) >= file.FlatStartByte(call) {
			return
		}
		if file.FlatType(n) != "property_declaration" && file.FlatType(n) != "variable_declaration" {
			return
		}
		if extractIdentifierFlat(file, n) == name && propertyInitializerCallCalleeName(file, n) == initializerCallee {
			found = true
		}
	})
	return found
}

func navigationReceiverHasTypedRoot(file *scanner.File, nav uint32, typeName string) bool {
	segments := flatNavigationChainIdentifiers(file, nav)
	if len(segments) == 0 {
		return false
	}
	root := segments[0]
	fn, ok := flatEnclosingFunction(file, nav)
	if !ok {
		return false
	}
	return parameterHasTypeFlat(file, fn, root, typeName)
}

func parameterHasTypeFlat(file *scanner.File, owner uint32, name string, typeName string) bool {
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(owner, "parameter", func(param uint32) {
		if found || extractIdentifierFlat(file, param) != name {
			return
		}
		file.FlatWalkNodes(param, "user_type", func(typ uint32) {
			if found {
				return
			}
			if semanticTypeNameMatches(apiNodeNameFlat(file, typ), typeName) {
				found = true
			}
		})
	})
	return found
}

func navigationChainContainsSegment(file *scanner.File, nav uint32, name string) bool {
	return navigationSegmentsContain(flatNavigationChainIdentifiers(file, nav), name)
}

func navigationSegmentsContain(segments []string, name string) bool {
	for _, segment := range segments {
		if segment == name {
			return true
		}
	}
	return false
}

func receiverChainHasQualifiedRoot(file *scanner.File, receiver uint32, roots ...string) bool {
	segments := flatNavigationChainIdentifiers(file, receiver)
	if len(segments) == 0 {
		if file.FlatType(receiver) == "call_expression" {
			nav, _ := flatCallExpressionParts(file, receiver)
			segments = flatNavigationChainIdentifiers(file, nav)
		}
	}
	if len(segments) > 0 {
		for _, root := range roots {
			if segments[0] == root {
				return true
			}
		}
	}
	found := false
	file.FlatWalkAllNodes(receiver, func(n uint32) {
		if found {
			return
		}
		if file.FlatType(n) != "navigation_expression" && file.FlatType(n) != "simple_identifier" {
			return
		}
		name := semantics.ReferenceName(file, n)
		for _, root := range roots {
			if name == root {
				found = true
				return
			}
		}
	})
	return found
}

func sameFileFunctionDeclaredInOwner(file *scanner.File, call uint32, owners ...string) bool {
	name := flatCallExpressionName(file, call)
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || extractIdentifierFlat(file, fn) != name {
			return
		}
		for p, ok := file.FlatParent(fn); ok; p, ok = file.FlatParent(p) {
			if file.FlatType(p) != "class_declaration" && file.FlatType(p) != "object_declaration" {
				continue
			}
			if semanticTypeNameMatches(extractIdentifierFlat(file, p), owners...) {
				found = true
			}
			return
		}
	})
	return found
}

func importShortNameFlat(file *scanner.File, idx uint32) string {
	text := strings.TrimSpace(file.FlatNodeText(idx))
	if !strings.HasPrefix(text, "import ") {
		return ""
	}
	imp := strings.TrimSpace(strings.TrimPrefix(text, "import "))
	if i := strings.Index(imp, " as "); i >= 0 {
		return strings.TrimSpace(imp[i+4:])
	}
	parts := strings.Split(imp, ".")
	if len(parts) == 0 || parts[len(parts)-1] == "*" {
		return ""
	}
	return parts[len(parts)-1]
}

func fileHasReferenceNameOutsideNode(file *scanner.File, name string, exclude uint32) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(n uint32) {
		if found || n == exclude {
			return
		}
		if file.FlatStartByte(n) >= file.FlatStartByte(exclude) && file.FlatEndByte(n) <= file.FlatEndByte(exclude) {
			return
		}
		if nodeHasAncestorTypeFlat(file, n, "import_header", "package_header") {
			return
		}
		t := file.FlatType(n)
		if t != "simple_identifier" && t != "type_identifier" && t != "navigation_expression" && t != "user_type" {
			return
		}
		if semantics.ReferenceName(file, n) == name {
			found = true
		}
	})
	return found
}
