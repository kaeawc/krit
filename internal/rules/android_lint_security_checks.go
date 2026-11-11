package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnsafeDynamicallyLoadedCodeRule mirrors Lint's UnsafeDynamicallyLoadedCode: System.load or Runtime.load.
type UnsafeDynamicallyLoadedCodeRule struct {
	FlatDispatchBase
	AndroidRule
}

// BadHostnameVerifierRule mirrors Lint's BadHostnameVerifier: a HostnameVerifier whose verify only returns true.
type BadHostnameVerifierRule struct {
	FlatDispatchBase
	AndroidRule
}

// HardwareIDsRule mirrors Lint's HardwareIds: device identifier getters, the ANDROID_ID setting and Build.SERIAL.
type HardwareIDsRule struct {
	FlatDispatchBase
	AndroidRule
}

// DeprecatedProviderRule mirrors Lint's DeprecatedProvider: Cipher.getInstance with the "BC" provider.
type DeprecatedProviderRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: receiver types are inferred from declarations.
func (r *UnsafeDynamicallyLoadedCodeRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence: supertypes are matched by name and imports.
func (r *BadHostnameVerifierRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence: receiver types are inferred from declarations.
func (r *HardwareIDsRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence: the receiver type is inferred from imports.
func (r *DeprecatedProviderRule) Confidence() float64 { return api.ConfidenceMedium }

// javaLangTypeRef reports whether text names the java.lang class simple, which needs no import.
func javaLangTypeRef(file *scanner.File, text, simple string) bool {
	text = strings.TrimSpace(text)
	return text == simple || text == "java.lang."+simple || lintTypeRefMatches(file, text, "java.lang."+simple)
}

func (r *UnsafeDynamicallyLoadedCodeRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "load" || len(call.Args) != 1 || call.ReceiverNode == 0 {
		return
	}
	recv := strings.TrimSpace(call.ReceiverText)
	switch {
	case javaLangTypeRef(file, recv, "System"):
	case javaLangTypeRef(file, strings.TrimSuffix(recv, ".getRuntime()"), "Runtime") && strings.HasSuffix(recv, ".getRuntime()"):
	case javaLangTypeRef(file, lintReceiverTypeText(file, call), "Runtime"):
	default:
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "",
		"Dynamically loading code using `load` is risky, please use `loadLibrary` instead when possible"))
}

var hostnameVerifierTypes = []string{"javax.net.ssl.HostnameVerifier"}

func (r *BadHostnameVerifierRule) check(ctx *api.Context) {
	file := ctx.File
	if !lintHasSupertype(file, ctx.Idx, hostnameVerifierTypes, true, 0) {
		return
	}
	body, _ := file.FlatFindChild(ctx.Idx, "class_body")
	if body == 0 {
		return
	}
	for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
		name := verifyMethodName(file, member)
		if name == 0 {
			continue
		}
		if !verifierAlwaysTrue(file, ctx.Idx) {
			return
		}
		ctx.Emit(lintFindingAtNode(ctx, name, "",
			"`verify` always returns `true`, which could cause insecure network traffic due to trusting TLS/SSL server certificates for wrong hostnames"))
		return
	}
}

// verifyMethodName returns the name node of a `verify(String, SSLSession)` method declaration, or 0.
func verifyMethodName(file *scanner.File, member uint32) uint32 {
	var name, params uint32
	switch file.FlatType(member) {
	case "function_declaration":
		name, _ = file.FlatFindChild(member, "simple_identifier")
		params, _ = file.FlatFindChild(member, "function_value_parameters")
	case "method_declaration":
		name, _ = file.FlatFindChild(member, "identifier")
		params, _ = file.FlatFindChild(member, "formal_parameters")
	default:
		return 0
	}
	if name == 0 || params == 0 || file.FlatNodeText(name) != "verify" {
		return 0
	}
	var types []string
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		if t := file.FlatType(p); t == "parameter" || t == "formal_parameter" {
			types = append(types, lintChildTypeText(file, p))
		}
	}
	if len(types) != 2 || !javaLangTypeRef(file, types[0], "String") || !lintTypeRefMatches(file, types[1], "javax.net.ssl.SSLSession") {
		return 0
	}
	return name
}

// verifierState tracks Lint's ComplexVisitor over a class: a throw or call makes it complex, and each return sets or
// clears the flag by whether it returns the literal true, so the last return decides.
type verifierState struct {
	file    *scanner.File
	complex bool
}

func (v *verifierState) visit(n uint32) {
	file := v.file
	switch file.FlatType(n) {
	case "call_expression", "method_invocation", "object_creation_expression", "throw_statement":
		v.complex = true
	case "jump_expression", "return_statement":
		v.complex = jumpIsComplex(file, n, v.complex)
	case "function_body":
		if expr := expressionBodyValue(file, n); expr != 0 {
			v.complex = !isTrueLiteral(file, expr)
		}
	}
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		v.visit(c)
	}
}

// verifierAlwaysTrue reports whether Lint's visitor leaves the class non-complex. Lint's UAST order puts Kotlin
// superclass constructor calls, secondary constructors and nested classes after the methods, so they decide last.
func verifierAlwaysTrue(file *scanner.File, class uint32) bool {
	v := &verifierState{file: file}
	var deferred []uint32
	late := false
	for c := file.FlatFirstChild(class); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "delegation_specifier":
			if _, ok := file.FlatFindChild(c, "constructor_invocation"); ok {
				late = true
			}
		case "class_body":
			for m := file.FlatFirstChild(c); m != 0; m = file.FlatNextSib(m) {
				switch file.FlatType(m) {
				case "secondary_constructor":
					late = true
				case "companion_object", "class_declaration", "object_declaration":
					deferred = append(deferred, m)
				default:
					v.visit(m)
				}
			}
		}
	}
	for _, d := range deferred {
		v.visit(d)
	}
	return !v.complex && !late
}

// jumpIsComplex updates the complex flag for a Kotlin jump or Java return statement.
func jumpIsComplex(file *scanner.File, n uint32, prev bool) bool {
	text := strings.TrimSpace(file.FlatNodeText(n))
	switch {
	case strings.HasPrefix(text, "throw"):
		return true
	case !strings.HasPrefix(text, "return"):
		return prev
	}
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) && file.FlatType(c) != "label" {
			return !isTrueLiteral(file, c)
		}
	}
	return prev
}

// expressionBodyValue returns the expression of a Kotlin `= expr` function body, or 0 for a block body.
func expressionBodyValue(file *scanner.File, body uint32) uint32 {
	if !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(body)), "=") {
		return 0
	}
	for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			return c
		}
	}
	return 0
}

func isTrueLiteral(file *scanner.File, n uint32) bool {
	return strings.TrimSpace(file.FlatNodeText(n)) == "true"
}

// hardwareIDMethods maps the identifier getters Lint reports to the class that declares them.
var hardwareIDMethods = map[string]string{
	"getAddress":         "android.bluetooth.BluetoothAdapter",
	"getMacAddress":      "android.net.wifi.WifiInfo",
	"getDeviceId":        "android.telephony.TelephonyManager",
	"getLine1Number":     "android.telephony.TelephonyManager",
	"getSimSerialNumber": "android.telephony.TelephonyManager",
	"getSubscriberId":    "android.telephony.TelephonyManager",
}

// hardwareIDProperties maps Kotlin synthetic property names to the getter they call.
var hardwareIDProperties = map[string]string{
	"address": "getAddress", "macAddress": "getMacAddress", "deviceId": "getDeviceId", "line1Number": "getLine1Number",
	"simSerialNumber": "getSimSerialNumber", "subscriberId": "getSubscriberId",
}

func hardwareIDMessage(name string) string {
	return "Using `" + name + "` to get device identifiers is not recommended"
}

func (r *HardwareIDsRule) check(ctx *api.Context) {
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "navigation_expression", "field_access":
		r.checkMember(ctx)
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.ReceiverNode == 0 {
		return
	}
	if call.Name == "getString" {
		r.checkSecureString(ctx, call)
		return
	}
	if owner := hardwareIDMethods[call.Name]; owner != "" && hardwareIDReceiver(ctx, call, owner) {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", hardwareIDMessage(call.Name)))
	}
}

// checkSecureString reports Settings.Secure.getString(resolver, "android_id").
func (r *HardwareIDsRule) checkSecureString(ctx *api.Context, call lintCall) {
	file := ctx.File
	if len(call.Args) != 2 || call.Args[1] == 0 || !lintTypeRefMatches(file, call.ReceiverText, "android.provider.Settings.Secure") {
		return
	}
	arg := call.Args[1]
	text := strings.TrimSpace(file.FlatNodeText(arg))
	if value, ok := lintEvalStringStrict(file, arg); (ok && value == "android_id") ||
		text == "ANDROID_ID" || strings.HasSuffix(text, "Secure.ANDROID_ID") {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", hardwareIDMessage("getString")))
	}
}

// checkMember reports Kotlin synthetic property reads of the identifier getters and Build.SERIAL.
func (r *HardwareIDsRule) checkMember(ctx *api.Context) {
	file := ctx.File
	recvNode, name, nameNode := memberAccessParts(file, ctx.Idx)
	if recvNode == 0 || name == "" {
		return
	}
	recv := strings.TrimSpace(file.FlatNodeText(recvNode))
	if name == "SERIAL" {
		if lintTypeRefMatches(file, recv, "android.os.Build") {
			ctx.Emit(lintFindingAtNode(ctx, nameNode, "", hardwareIDMessage("SERIAL")))
		}
		return
	}
	getter := hardwareIDProperties[name]
	if getter == "" || file.Language != scanner.LangKotlin || isCalleeOfCall(file, ctx.Idx) {
		return
	}
	call := lintCall{File: file, Idx: ctx.Idx, Name: getter, ReceiverText: recv, ReceiverNode: recvNode}
	if hardwareIDReceiver(ctx, call, hardwareIDMethods[getter]) {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", hardwareIDMessage(getter)))
	}
}

// memberAccessParts splits `receiver.name` into the receiver node, the member name, and the name node.
func memberAccessParts(file *scanner.File, idx uint32) (recv uint32, name string, nameNode uint32) {
	if file.FlatType(idx) == "field_access" {
		n := file.FlatNamedChildCount(idx)
		if n < 2 {
			return 0, "", 0
		}
		nameNode = file.FlatNamedChild(idx, n-1)
		return file.FlatNamedChild(idx, 0), file.FlatNodeText(nameNode), nameNode
	}
	if file.FlatNamedChildCount(idx) != 2 {
		return 0, "", 0
	}
	suffix := file.FlatNamedChild(idx, 1)
	if file.FlatType(suffix) != "navigation_suffix" {
		return 0, "", 0
	}
	nameNode, _ = file.FlatFindChild(suffix, "simple_identifier")
	if nameNode == 0 {
		return 0, "", 0
	}
	return file.FlatNamedChild(idx, 0), file.FlatNodeText(nameNode), nameNode
}

// isCalleeOfCall reports whether the navigation expression is the callee of a call, not a property read.
func isCalleeOfCall(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	return ok && file.FlatType(parent) == "call_expression" && file.FlatNamedChild(parent, 0) == idx
}

// isSystemServiceReceiver reports whether the call's receiver is an instance of fqn by resolved or declared type, or
// through an initializer cast such as `getSystemService(X) as Type` or `(Type) getSystemService(X)`. Lint does not
// infer the type from `getSystemService(Type::class.java)`.
func isSystemServiceReceiver(ctx *api.Context, call lintCall, fqn string) bool {
	if lintReceiverIsType(ctx, call, fqn) {
		return true
	}
	file := ctx.File
	simple := fqn[strings.LastIndex(fqn, ".")+1:]
	recv := call.ReceiverNode
	switch file.FlatType(recv) {
	case "simple_identifier", "identifier":
		init := lintInitializerOf(file, call.Idx, file.FlatNodeText(recv), true)
		return init != 0 && initNamesSystemService(file.FlatNodeText(init), fqn, simple)
	case "parenthesized_expression", "as_expression", "cast_expression", "call_expression", "method_invocation":
		return initNamesSystemService(file.FlatNodeText(recv), fqn, simple)
	}
	return false
}

func initNamesSystemService(text, fqn, simple string) bool {
	return strings.Contains(text, " as "+simple) || strings.Contains(text, " as "+fqn) || strings.Contains(text, " as? "+simple) || strings.Contains(text, "("+simple+")") || strings.Contains(text, "("+fqn+")")
}

func (r *DeprecatedProviderRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "getInstance" || len(call.Args) != 2 || call.Args[1] == 0 || call.ReceiverNode == 0 ||
		!callOnClass(ctx, call, "javax.crypto.Cipher") {
		return
	}
	if !providerArgIsBC(file, call.Args[1]) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, call.Args[1], "",
		"The `BC` provider is deprecated and when `targetSdkVersion` is moved to `P` this method will throw a `NoSuchAlgorithmException`. To fix this you should stop specifying a provider and use the default implementation"))
}

// providerArgIsBC matches the provider argument "BC", or `Security.getProvider("BC")`.
func providerArgIsBC(file *scanner.File, arg uint32) bool {
	if provider, ok := lintEvalStringStrict(file, arg); ok {
		return provider == "BC"
	}
	inner, ok := lintCallAt(file, arg)
	if !ok || inner.Name != "getProvider" || len(inner.Args) != 1 || !lintTypeRefMatches(file, inner.ReceiverText, "java.security.Security") {
		return false
	}
	provider, ok := lintEvalStringStrict(file, inner.Args[0])
	return ok && provider == "BC"
}

const wifiManagerClass = "android.net.wifi.WifiManager"

// hardwareIDReceiver reports whether the receiver is an owner instance. A WifiInfo also comes from
// `wifiManager.connectionInfo`, whose type Lint reads from the declared return type.
func hardwareIDReceiver(ctx *api.Context, call lintCall, owner string) bool {
	if isSystemServiceReceiver(ctx, call, owner) {
		return true
	}
	file := ctx.File
	if owner != "android.net.wifi.WifiInfo" {
		return false
	}
	var base uint32
	switch file.FlatType(call.ReceiverNode) {
	case "navigation_expression":
		if recv, name, _ := memberAccessParts(file, call.ReceiverNode); name == "connectionInfo" {
			base = recv
		}
	case "call_expression", "method_invocation":
		if inner, ok := lintCallAt(file, call.ReceiverNode); ok && inner.Name == "getConnectionInfo" && len(inner.Args) == 0 {
			base = inner.ReceiverNode
		}
	}
	if base == 0 {
		return false
	}
	manager := lintCall{File: file, Idx: call.Idx, ReceiverNode: base, ReceiverText: strings.TrimSpace(file.FlatNodeText(base))}
	return isSystemServiceReceiver(ctx, manager, wifiManagerClass)
}
