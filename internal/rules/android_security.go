package rules

// Android Lint rules for Security, Performance, Accessibility, I18N, and RTL categories.
// Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	neturl "net/url"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Additional category constants not in android.go
const (
	ALCRTL AndroidLintCategory = "rtl"
)

// =============================================================================
// Security Rules
// =============================================================================
//
// AddJavascriptInterfaceRule and its helpers live in
// android_security_javascript_interface.go.

type HardcodedHTTPURLRule struct {
	FlatDispatchBase
	AndroidRule
}

type OkHTTPDisableSslValidationRule struct {
	FlatDispatchBase
	AndroidRule
}

type DisableCertificatePinningRule struct {
	FlatDispatchBase
	AndroidRule
}

type AllowAllHostnameVerifierRule struct {
	FlatDispatchBase
	AndroidRule
}

type InsecureTrustManagerRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *HardcodedHTTPURLRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *OkHTTPDisableSslValidationRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DisableCertificatePinningRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *AllowAllHostnameVerifierRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *InsecureTrustManagerRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *OkHTTPDisableSslValidationRule) check(ctx *api.Context) {
	file := ctx.File
	name := javaAwareCallName(file, ctx.Idx)
	if name != "hostnameVerifier" && name != "sslSocketFactory" {
		return
	}
	if !sourceImportsOrMentions(file, "okhttp3.OkHttpClient") {
		return
	}
	chainText := okHTTPDisableSslValidationChainText(file, ctx.Idx)
	if !strings.Contains(chainText, "OkHttpClient.Builder") {
		return
	}
	if name == "hostnameVerifier" && !okHTTPDisableSslValidationAlwaysTrueVerifier(chainText) {
		return
	}
	if name == "sslSocketFactory" && !okHTTPDisableSslValidationUnsafeTrustManager(chainText) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"OkHttpClient.Builder disables TLS validation. Do not install always-true hostname verifiers or trust-all managers.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func (r *DisableCertificatePinningRule) check(ctx *api.Context) {
	file := ctx.File
	if !disableCertificatePinningEmptyBuilder(file, ctx.Idx) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"CertificatePinner.Builder builds without pins. Add certificate pins or remove the empty pinner.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func (r *AllowAllHostnameVerifierRule) check(ctx *api.Context) {
	file := ctx.File
	if !allowAllHostnameVerifierClass(file, ctx.Idx) {
		return
	}
	method := allowAllHostnameVerifierVerifyMethod(file, ctx.Idx)
	if method == 0 {
		return
	}
	f := r.Finding(file, file.FlatRow(method)+1, file.FlatCol(method)+1,
		"HostnameVerifier.verify always returns true. Validate the SSLSession hostname instead of accepting every host.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func (r *InsecureTrustManagerRule) check(ctx *api.Context) {
	file := ctx.File
	if !insecureTrustManagerDecl(ctx, file, ctx.Idx) {
		return
	}
	for _, method := range insecureTrustManagerTrivialChecks(file, ctx.Idx) {
		f := r.Finding(file, file.FlatRow(method)+1, file.FlatCol(method)+1,
			"Trust manager check method accepts certificates without validation. Perform certificate validation or remove the trust-all manager.")
		f.Confidence = r.Confidence()
		ctx.Emit(f)
	}
}

func (r *HardcodedHTTPURLRule) check(ctx *api.Context) {
	file := ctx.File
	raw, ok := hardcodedHTTPURLLiteralArg(file, ctx.Idx)
	if !ok || !hardcodedHTTPURLInsecure(raw) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Hardcoded HTTP URL passed to network API. Use HTTPS or load environment-specific endpoints from configuration.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func hardcodedHTTPURLLiteralArg(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 {
		return "", false
	}
	name := javaAwareCallName(file, idx)
	switch file.FlatType(idx) {
	case "call_expression":
		if name != "baseUrl" && name != "url" && name != "URL" {
			return "", false
		}
		if !hardcodedHTTPURLCallLooksReal(file, idx, name) {
			return "", false
		}
		_, args := flatCallExpressionParts(file, idx)
		arg := flatPositionalValueArgument(file, args, 0)
		if arg == 0 {
			return "", false
		}
		return weakSecurityStringLiteralValue(file, flatValueArgumentExpression(file, arg))
	case "object_creation_expression":
		if !hardcodedHTTPURLURLConstructorLooksReal(file, idx) {
			return "", false
		}
		args, ok := file.FlatFindChild(idx, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return "", false
		}
		return weakSecurityStringLiteralValue(file, file.FlatNamedChild(args, 0))
	case "method_invocation":
		if name != "baseUrl" && name != "url" {
			return "", false
		}
		if !hardcodedHTTPURLCallLooksReal(file, idx, name) {
			return "", false
		}
		args, ok := file.FlatFindChild(idx, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return "", false
		}
		return weakSecurityStringLiteralValue(file, file.FlatNamedChild(args, 0))
	default:
		return "", false
	}
}

func hardcodedHTTPURLCallLooksReal(file *scanner.File, idx uint32, name string) bool {
	text := file.FlatNodeText(idx)
	switch name {
	case "baseUrl":
		return sourceImportsOrMentions(file, "retrofit2.Retrofit") &&
			(strings.Contains(text, "Retrofit.Builder") || strings.Contains(text, "retrofit2.Retrofit.Builder"))
	case "url":
		return sourceImportsOrMentions(file, "okhttp3.Request") &&
			(strings.Contains(text, "Request.Builder") || strings.Contains(text, "okhttp3.Request.Builder"))
	case "URL":
		return hardcodedHTTPURLURLConstructorLooksReal(file, idx)
	default:
		return false
	}
}

func hardcodedHTTPURLURLConstructorLooksReal(file *scanner.File, idx uint32) bool {
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "java.net.URL") {
		return true
	}
	if staticIvFileDeclaresType(file, "URL") {
		return false
	}
	return sourceImportsOrMentions(file, "java.net.URL")
}

func hardcodedHTTPURLInsecure(raw string) bool {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "http://") {
		return false
	}
	u, err := neturl.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "http") {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "10.0.2.2", "0.0.0.0":
		return false
	default:
		return true
	}
}
