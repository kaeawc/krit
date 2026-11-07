package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	sslSocketFactoryClass = "android.net.SSLCertificateSocketFactory"
	inetAddressClass      = "java.net.InetAddress"
)

// SSLCertificateSocketFactoryGetInsecureRule mirrors Lint's SSLCertificateSocketFactoryGetInsecure.
type SSLCertificateSocketFactoryGetInsecureRule struct {
	FlatDispatchBase
	AndroidRule
}

// SSLCertificateSocketFactoryCreateSocketRule mirrors Lint's SSLCertificateSocketFactoryCreateSocket: createSocket
// with an InetAddress skips hostname verification.
type SSLCertificateSocketFactoryCreateSocketRule struct {
	FlatDispatchBase
	AndroidRule
}

// DeletedProviderRule mirrors Lint's DeletedProvider: Cipher or SecureRandom requested from the removed Crypto provider.
type DeletedProviderRule struct {
	FlatDispatchBase
	AndroidRule
}

// WebViewClientOnReceivedSslErrorRule mirrors Lint's WebViewClientOnReceivedSslError: onReceivedSslError that proceeds.
type WebViewClientOnReceivedSslErrorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: receiver types are inferred from declarations.
func (r *SSLCertificateSocketFactoryGetInsecureRule) Confidence() float64 {
	return api.ConfidenceMedium
}

// Confidence reports a tier-2 (medium) base confidence: receiver types are inferred from declarations.
func (r *SSLCertificateSocketFactoryCreateSocketRule) Confidence() float64 {
	return api.ConfidenceMedium
}

// Confidence reports a tier-2 (medium) base confidence: receiver types are inferred from declarations.
func (r *DeletedProviderRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence: receiver types are inferred from declarations.
func (r *WebViewClientOnReceivedSslErrorRule) Confidence() float64 { return api.ConfidenceMedium }

// callOnClass reports whether the call targets a member of fqn: a static call spelled with the class, or an instance call
// on a receiver of that type.
func callOnClass(ctx *api.Context, call lintCall, fqn string) bool {
	if lintTypeRefMatches(ctx.File, call.ReceiverText, fqn) {
		return true
	}
	return lintReceiverIsType(ctx, call, fqn)
}

func (r *SSLCertificateSocketFactoryGetInsecureRule) check(ctx *api.Context) {
	call, ok := lintCallAt(ctx.File, ctx.Idx)
	if !ok || call.Name != "getInsecure" || !callOnClass(ctx, call, sslSocketFactoryClass) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "",
		"Use of `SSLCertificateSocketFactory.getInsecure()` can cause insecure network traffic due to trusting arbitrary TLS/SSL certificates presented by peers"))
}

// inetAddressFactories are the java.net.InetAddress methods that return a single InetAddress.
var inetAddressFactories = map[string]bool{
	"getByName": true, "getLocalHost": true, "getLoopbackAddress": true, "getByAddress": true,
}

// argIsInetAddress reports whether an argument expression has type InetAddress or a subclass declared in java.net.
func argIsInetAddress(ctx *api.Context, arg uint32) bool {
	file := ctx.File
	switch file.FlatType(arg) {
	case "simple_identifier", "identifier":
		return lintInetTypeName(lintDeclaredTypeText(file, arg, file.FlatNodeText(arg)), file)
	case "call_expression", "method_invocation":
		if c, ok := lintCallAt(file, arg); ok && inetAddressFactories[c.Name] {
			return lintTypeRefMatches(file, c.ReceiverText, inetAddressClass)
		}
	case "object_creation_expression":
		if c, ok := lintCallAt(file, arg); ok {
			return lintInetTypeName(c.TypeText, file)
		}
	}
	return false
}

func lintInetTypeName(text string, file *scanner.File) bool {
	for _, fqn := range []string{inetAddressClass, "java.net.Inet4Address", "java.net.Inet6Address"} {
		if lintTypeRefMatches(file, text, fqn) {
			return true
		}
	}
	return false
}

func (r *SSLCertificateSocketFactoryCreateSocketRule) check(ctx *api.Context) {
	call, ok := lintCallAt(ctx.File, ctx.Idx)
	if !ok || call.Name != "createSocket" || len(call.Args) == 0 || call.Args[0] == 0 || !lintReceiverIsType(ctx, call, sslSocketFactoryClass) {
		return
	}
	if !argIsInetAddress(ctx, call.Args[0]) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "",
		"Use of `SSLCertificateSocketFactory.createSocket()` with an InetAddress parameter can cause insecure network traffic due to trusting arbitrary hostnames in TLS/SSL certificates presented by peers"))
}

func (r *DeletedProviderRule) check(ctx *api.Context) {
	call, ok := lintCallAt(ctx.File, ctx.Idx)
	if !ok || call.Name != "getInstance" || len(call.Args) != 2 || call.Args[1] == 0 {
		return
	}
	if !lintTypeRefMatches(ctx.File, call.ReceiverText, "javax.crypto.Cipher") && !lintTypeRefMatches(ctx.File, call.ReceiverText, "java.security.SecureRandom") {
		return
	}
	if provider, ok := lintEvalString(ctx.File, call.Args[1]); ok && provider == "Crypto" {
		ctx.Emit(lintFindingAtNode(ctx, call.Args[1], "", "The Crypto provider has been deleted in Android P (and was deprecated in Android N), so the code will crash"))
	}
}

func (r *WebViewClientOnReceivedSslErrorRule) check(ctx *api.Context) {
	file := ctx.File
	if !lintExtendsWebViewClient(file, ctx.Idx, 0) {
		return
	}
	body, _ := file.FlatFindChild(ctx.Idx, "class_body")
	if body == 0 {
		return
	}
	for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
		var name uint32
		switch file.FlatType(member) {
		case "function_declaration":
			name, _ = file.FlatFindChild(member, "simple_identifier")
		case "method_declaration":
			name, _ = file.FlatFindChild(member, "identifier")
		default:
			continue
		}
		if file.FlatNodeText(name) != "onReceivedSslError" || file.FlatFindModifierNode(member, "abstract") != 0 {
			continue
		}
		for _, kind := range lintCallNodeTypes[:2] {
			file.FlatWalkNodes(member, kind, func(n uint32) {
				c, ok := lintCallAt(file, n)
				if !ok || c.Name != "proceed" || c.ReceiverNode == 0 {
					return
				}
				if !lintTypeRefMatches(file, lintReceiverTypeText(file, c), "android.webkit.SslErrorHandler") {
					return
				}
				ctx.Emit(lintFindingAtNode(ctx, n, "",
					"Permitting connections with SSL-related errors could allow eavesdroppers to intercept data sent by your app, which impacts the privacy of your users. Consider canceling the connections by invoking `SslErrorHandler#cancel()`."))
			})
		}
	}
}

func lintExtendsWebViewClient(file *scanner.File, class uint32, depth int) bool {
	if depth > 4 {
		return false
	}
	switch file.FlatType(class) {
	case "class_declaration", "object_literal", "object_creation_expression":
	default:
		return false
	}
	for _, super := range lintSupertypeNames(file, class) {
		if lintTypeRefMatches(file, super, "android.webkit.WebViewClient") {
			return true
		}
		simple := super[strings.LastIndex(super, ".")+1:]
		if decl := lintSameFileClass(file, simple); decl != 0 && decl != class && lintExtendsWebViewClient(file, decl, depth+1) {
			return true
		}
	}
	return false
}
