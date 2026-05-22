package rules

// Android Lint rules for Security, Performance, Accessibility, I18N, and RTL categories.
// Ported from AOSP Android Lint.
// Origin: https://android.googlesource.com/platform/tools/base/+/refs/heads/main/lint/libs/lint-checks/

import (
	neturl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	androidproject "github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/strutil"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// Additional category constants not in android.go
const (
	ALCRTL AndroidLintCategory = "rtl"
)

// =============================================================================
// Security Rules
// =============================================================================

// AddJavascriptInterfaceRule detects WebView.addJavascriptInterface() calls.
type AddJavascriptInterfaceRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *AddJavascriptInterfaceRule) check(ctx *api.Context) {
	file := ctx.File
	if file.FlatType(ctx.Idx) != "call_expression" && file.FlatType(ctx.Idx) != "method_invocation" {
		return
	}
	if javaAwareCallName(file, ctx.Idx) != "addJavascriptInterface" {
		return
	}
	confidence, ok := addJavascriptInterfaceCallConfidence(ctx, ctx.Idx)
	if !ok {
		return
	}
	line := file.FlatRow(ctx.Idx) + 1
	col := file.FlatCol(ctx.Idx) + 1
	sdk := addJavascriptInterfaceSDKContextForFile(file)
	if sdk.minSdk < 17 {
		f := r.Finding(file, line, col,
			"addJavascriptInterface called while minSdk is below 17. This exposes injected objects to reflection on older Android versions.")
		f.Confidence = confidence
		ctx.Emit(f)
	}
	if sdk.targetSdk >= 17 && addJavascriptInterfaceBridgeMissingAnnotation(file, ctx.Idx) {
		f := r.Finding(file, line, col,
			"Injected JavaScript interface has no @JavascriptInterface-annotated methods for targetSdk 17 or higher.")
		f.Confidence = confidence
		ctx.Emit(f)
	}
}

func addJavascriptInterfaceCallConfidence(ctx *api.Context, call uint32) (float64, bool) {
	file := ctx.File
	if file.FlatType(call) == "method_invocation" {
		return addJavascriptInterfaceJavaConfidence(file, call)
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return 0, false
	}
	return addJavascriptInterfaceReceiverConfidence(ctx, file.FlatNamedChild(navExpr, 0))
}

func addJavascriptInterfaceJavaConfidence(file *scanner.File, call uint32) (float64, bool) {
	if !sourceImportsOrMentions(file, "android.webkit.WebView") {
		return 0, false
	}
	receiver := javaMethodReceiverText(file, call)
	if receiver == "" {
		return 0, false
	}
	if strings.Contains(receiver, "getSettings") {
		return 0, false
	}
	name := wrongViewCastCallReceiverName(file, call)
	if name == "" {
		name = receiver
	}
	if name == "webView" || name == "wv" || strings.HasSuffix(name, ".webView") || strings.HasSuffix(name, ".wv") {
		return 0.85, true
	}
	return 0, false
}

type addJavascriptInterfaceSDKContext struct {
	minSdk    int
	targetSdk int
}

func addJavascriptInterfaceSDKContextForFile(file *scanner.File) addJavascriptInterfaceSDKContext {
	if file == nil {
		return addJavascriptInterfaceSDKContext{}
	}
	return filefacts.FileFact(fileFactsCache(), file, slotAddJSInterfaceSDK, func() addJavascriptInterfaceSDKContext {
		sdk := addJavascriptInterfaceSDKContext{}
		for _, dir := range ancestorDirs(filepath.Dir(file.Path)) {
			for _, name := range []string{"build.gradle.kts", "build.gradle"} {
				buildPath := filepath.Join(dir, name)
				data, err := os.ReadFile(buildPath)
				if err != nil {
					continue
				}
				cfg, err := androidproject.ParseBuildGradleContent(string(data))
				if err != nil {
					continue
				}
				if cfg.MinSdkVersion > 0 {
					sdk.minSdk = cfg.MinSdkVersion
				}
				if cfg.TargetSdkVersion > 0 {
					sdk.targetSdk = cfg.TargetSdkVersion
				}
				if sdk.minSdk > 0 || sdk.targetSdk > 0 {
					return sdk
				}
			}
			for _, rel := range []string{"src/main/AndroidManifest.xml", "AndroidManifest.xml"} {
				manifestPath := filepath.Join(dir, rel)
				manifest, err := androidproject.ParseManifest(manifestPath)
				if err != nil {
					continue
				}
				if manifest.UsesSdk.MinSdkVersion != "" {
					sdk.minSdk, _ = strconv.Atoi(manifest.UsesSdk.MinSdkVersion)
				}
				if manifest.UsesSdk.TargetSdkVersion != "" {
					sdk.targetSdk, _ = strconv.Atoi(manifest.UsesSdk.TargetSdkVersion)
				}
				if sdk.minSdk > 0 || sdk.targetSdk > 0 {
					return sdk
				}
			}
		}
		return sdk
	})
}

func ancestorDirs(dir string) []string {
	if dir == "" || dir == "." {
		return nil
	}
	dir = filepath.Clean(dir)
	var dirs []string
	for {
		dirs = append(dirs, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

func addJavascriptInterfaceBridgeMissingAnnotation(file *scanner.File, call uint32) bool {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return false
	}
	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		arg = flatNamedValueArgument(file, args, "object")
	}
	if arg == 0 {
		arg = flatNamedValueArgument(file, args, "obj")
	}
	expr := flatValueArgumentExpression(file, arg)
	className := addJavascriptInterfaceConstructedClassName(file, expr)
	if className == "" {
		return false
	}
	classDecl := addJavascriptInterfaceSameFileClass(file, className)
	return classDecl != 0 && !addJavascriptInterfaceClassHasAnnotatedMethod(file, classDecl)
}

func addJavascriptInterfaceConstructedClassName(file *scanner.File, expr uint32) string {
	if file == nil || expr == 0 {
		return ""
	}
	expr = flatUnwrapParenExpr(file, expr)
	if file.FlatType(expr) == "call_expression" {
		if name := flatCallExpressionName(file, expr); name != "" {
			return name
		}
	}
	var name string
	file.FlatWalkNodes(expr, "type_identifier", func(idx uint32) {
		if name == "" {
			name = file.FlatNodeText(idx)
		}
	})
	return name
}

func addJavascriptInterfaceSameFileClass(file *scanner.File, name string) uint32 {
	var classDecl uint32
	file.FlatWalkNodes(0, "class_declaration", func(candidate uint32) {
		if classDecl == 0 && extractIdentifierFlat(file, candidate) == name {
			classDecl = candidate
		}
	})
	return classDecl
}

func addJavascriptInterfaceClassHasAnnotatedMethod(file *scanner.File, classDecl uint32) bool {
	found := false
	file.FlatWalkNodes(classDecl, "function_declaration", func(fn uint32) {
		if found {
			return
		}
		owner, ok := flatEnclosingAncestor(file, fn, "class_declaration")
		if ok && owner == classDecl && hasAnnotationNamed(file, fn, "JavascriptInterface") {
			found = true
		}
	})
	file.FlatWalkNodes(classDecl, "method_declaration", func(method uint32) {
		if found {
			return
		}
		owner, ok := flatEnclosingAncestor(file, method, "class_declaration")
		if ok && owner == classDecl && strings.Contains(file.FlatNodeText(method), "@JavascriptInterface") {
			found = true
		}
	})
	return found
}

func addJavascriptInterfaceReceiverConfidence(ctx *api.Context, receiverExpr uint32) (float64, bool) {
	file := ctx.File
	receiver := flatUnwrapParenExpr(file, receiverExpr)
	if ctx.Resolver != nil {
		typ := ctx.Resolver.ResolveFlatNode(receiver, file)
		if (typ == nil || typ.Kind == typeinfer.TypeUnknown) && file.FlatType(receiver) == "simple_identifier" {
			typ = ctx.Resolver.ResolveByNameFlat(file.FlatNodeText(receiver), receiver, file)
		}
		if typ != nil && typ.Kind != typeinfer.TypeUnknown && (typ.Name != "" || typ.FQN != "") {
			if addJavascriptInterfaceTypeIsWebView(ctx.Resolver, typ) {
				return 1.0, true
			}
			return 0, false
		}
	}
	name := addJavascriptInterfaceReceiverName(file, receiver)
	if name == "" {
		return 0, false
	}
	if name == "webView" || name == "wv" {
		return 0.85, true
	}
	return 0, false
}

func addJavascriptInterfaceReceiverName(file *scanner.File, receiver uint32) string {
	switch file.FlatType(receiver) {
	case "simple_identifier":
		return file.FlatNodeText(receiver)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, receiver)
	}
	return ""
}

func addJavascriptInterfaceTypeIsWebView(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		if name == "WebView" || name == "android.webkit.WebView" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "WebView" || info.FQN == "android.webkit.WebView" {
			return true
		}
		for _, supertype := range info.Supertypes {
			if visit(supertype) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN) || visit(typ.Name)
}

// GetInstanceRule detects Cipher.getInstance with insecure algorithms (ECB, DES).
type GetInstanceRule struct {
	FlatDispatchBase
	AndroidRule
}

// WeakMessageDigestRule detects MessageDigest.getInstance calls that request
// collision-broken digest algorithms.
type WeakMessageDigestRule struct {
	FlatDispatchBase
	AndroidRule
}

// WeakMacAlgorithmRule detects Mac.getInstance calls that request HMAC
// algorithms backed by broken digest functions.
type WeakMacAlgorithmRule struct {
	FlatDispatchBase
	AndroidRule
}

// WeakKeySizeRule detects crypto key generators initialized with literal key
// sizes below conservative per-algorithm thresholds.
type WeakKeySizeRule struct {
	FlatDispatchBase
	AndroidRule
}

// StaticIvRule detects IV/GCM parameter specs built from inline literal bytes.
type StaticIvRule struct {
	FlatDispatchBase
	AndroidRule
}

type HardcodedSecretKeyRule struct {
	FlatDispatchBase
	AndroidRule
}

type HardcodedHTTPURLRule struct {
	FlatDispatchBase
	AndroidRule
}

type StartActivityWithUntrustedIntentRule struct {
	FlatDispatchBase
	AndroidRule
}

type RsaNoPaddingRule struct {
	FlatDispatchBase
	AndroidRule
}

type PrngFromSystemTimeRule struct {
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

type BroadcastReceiverExportedFlagMissingRule struct {
	FlatDispatchBase
	AndroidRule
}

type InsecureTrustManagerRule struct {
	FlatDispatchBase
	AndroidRule
}

type ImplicitPendingIntentRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. AST-based
// detection resolves the call shape structurally (call_expression →
// navigation_expression(Cipher.getInstance) → string_literal arg) and
// confirms the receiver is javax.crypto.Cipher via import presence or
// the absence of a same-file user-defined Cipher class. Algorithm
// inspection uses the literal's parsed content, not regex slicing.
func (r *GetInstanceRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *WeakMessageDigestRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *WeakMacAlgorithmRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *WeakKeySizeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StaticIvRule) Confidence() float64 { return api.ConfidenceMediumHigh }

func (r *HardcodedSecretKeyRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *HardcodedHTTPURLRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *StartActivityWithUntrustedIntentRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RsaNoPaddingRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *PrngFromSystemTimeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *OkHTTPDisableSslValidationRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DisableCertificatePinningRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *AllowAllHostnameVerifierRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *BroadcastReceiverExportedFlagMissingRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *InsecureTrustManagerRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *ImplicitPendingIntentRule) Confidence() float64 { return api.ConfidenceHigh }

var getInstanceInsecureAlgoTokens = []string{"ECB", "DES", "RC2", "RC4"}

func (r *GetInstanceRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 || args == 0 {
		return
	}
	if flatNavigationExpressionLastIdentifier(file, navExpr) != "getInstance" {
		return
	}
	if !getInstanceReceiverIsJavaxCipher(file, navExpr) {
		return
	}
	algo, ok := getInstanceFirstStringArg(file, args)
	if !ok {
		return
	}
	upper := strings.ToUpper(algo)
	hit := false
	for _, tok := range getInstanceInsecureAlgoTokens {
		if strings.Contains(upper, tok) {
			hit = true
			break
		}
	}
	if !hit {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Cipher.getInstance uses insecure algorithm. Avoid ECB mode and DES/RC2/RC4.")
}

func (r *RsaNoPaddingRule) check(ctx *api.Context) {
	file := ctx.File
	if javaAwareCallName(file, ctx.Idx) != "getInstance" {
		return
	}
	if !rsaNoPaddingReceiverIsJavaxCipher(file, ctx.Idx) {
		return
	}
	algo, ok := weakGetInstanceFirstStringArg(file, ctx.Idx)
	if !ok || !rsaNoPaddingAlgorithm(algo) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"RSA cipher uses NoPadding. Use OAEPWithSHA-256AndMGF1Padding or at minimum PKCS1Padding instead of textbook RSA.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func (r *PrngFromSystemTimeRule) check(ctx *api.Context) {
	file := ctx.File
	if !prngFromSystemTimeCryptoFile(file) || prngFromSystemTimeTestPath(file.Path) {
		return
	}
	if !prngFromSystemTimeRandomConstructor(file, ctx.Idx) {
		return
	}
	seed := prngFromSystemTimeSeedArg(file, ctx.Idx)
	if seed == 0 || !prngFromSystemTimeSeedExpr(file.FlatNodeText(seed)) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"java.util.Random seeded from system time is predictable in security-sensitive code. Use SecureRandom without a deterministic seed.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

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

func (r *BroadcastReceiverExportedFlagMissingRule) check(ctx *api.Context) {
	file := ctx.File
	if scanner.IsTestFile(file.Path) || strings.Contains(filepath.ToSlash(file.Path), "/androidTest/") {
		return
	}
	if javaAwareCallName(file, ctx.Idx) != "registerReceiver" {
		return
	}
	if !dynamicReceiverHasContextReceiver(file, ctx.Idx) {
		return
	}
	if !broadcastReceiverExportedFlagMissing(file, ctx.Idx) {
		return
	}
	sdk := addJavascriptInterfaceSDKContextForFile(file)
	confidence := r.Confidence()
	if sdk.targetSdk > 0 && sdk.targetSdk < 34 {
		return
	}
	if sdk.targetSdk == 0 {
		confidence = 0.65
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Dynamic broadcast receiver registration omits RECEIVER_EXPORTED or RECEIVER_NOT_EXPORTED. Add an explicit exported flag for targetSdk 34+.")
	f.Confidence = confidence
	ctx.Emit(f)
}

func (r *InsecureTrustManagerRule) check(ctx *api.Context) {
	file := ctx.File
	if !insecureTrustManagerDecl(file, ctx.Idx) {
		return
	}
	for _, method := range insecureTrustManagerTrivialChecks(file, ctx.Idx) {
		f := r.Finding(file, file.FlatRow(method)+1, file.FlatCol(method)+1,
			"Trust manager check method accepts certificates without validation. Perform certificate validation or remove the trust-all manager.")
		f.Confidence = r.Confidence()
		ctx.Emit(f)
	}
}

func (r *ImplicitPendingIntentRule) check(ctx *api.Context) {
	file := ctx.File
	if !implicitPendingIntentCall(file, ctx.Idx) {
		return
	}
	sdk := addJavascriptInterfaceSDKContextForFile(file)
	confidence := r.Confidence()
	if sdk.targetSdk > 0 && sdk.targetSdk < 31 {
		return
	}
	if sdk.targetSdk == 0 {
		confidence = 0.65
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"PendingIntent flags omit FLAG_IMMUTABLE or FLAG_MUTABLE. Add an explicit mutability flag for Android 12+.")
	f.Confidence = confidence
	ctx.Emit(f)
}

var weakMessageDigestAlgorithms = map[string]bool{
	"MD2":   true,
	"MD4":   true,
	"MD5":   true,
	"SHA-1": true,
	"SHA1":  true,
}

func (r *WeakMessageDigestRule) check(ctx *api.Context) {
	file := ctx.File
	if javaAwareCallName(file, ctx.Idx) != "getInstance" {
		return
	}
	if !weakMessageDigestReceiverIsJavaSecurity(file, ctx.Idx) {
		return
	}
	algo, ok := weakGetInstanceFirstStringArg(file, ctx.Idx)
	if !ok || !weakMessageDigestAlgorithms[strings.ToUpper(strings.TrimSpace(algo))] {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"MessageDigest.getInstance uses a weak digest algorithm. Use SHA-256, SHA-384, SHA-512, or SHA-3 for security-sensitive hashing.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

var weakMacAlgorithms = map[string]bool{
	"HMACMD2":  true,
	"HMACMD5":  true,
	"HMACSHA0": true,
	"HMACSHA1": true,
}

func (r *WeakMacAlgorithmRule) check(ctx *api.Context) {
	file := ctx.File
	if javaAwareCallName(file, ctx.Idx) != "getInstance" {
		return
	}
	if !weakMacReceiverIsJavaxCrypto(file, ctx.Idx) {
		return
	}
	algo, ok := weakGetInstanceFirstStringArg(file, ctx.Idx)
	if !ok || !weakMacAlgorithms[strings.ToUpper(strings.TrimSpace(algo))] {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Mac.getInstance uses an HMAC algorithm backed by a weak digest. Use HmacSHA256, HmacSHA384, HmacSHA512, or SHA-3-based alternatives.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func (r *WeakKeySizeRule) check(ctx *api.Context) {
	file := ctx.File
	callName := javaAwareCallName(file, ctx.Idx)
	if callName != "initialize" && callName != "init" {
		return
	}
	receiver := weakKeySizeInitReceiver(file, ctx.Idx)
	if receiver == "" {
		return
	}
	size, ok := weakKeySizeFirstIntegerArg(file, ctx.Idx)
	if !ok {
		return
	}
	algo, ok := weakKeySizeFindGeneratorAlgorithm(file, ctx.Idx, receiver)
	if !ok {
		return
	}
	threshold, ok := weakKeySizeThreshold(algo)
	if !ok || size >= threshold {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Crypto key generator initialized with a weak literal key size. Use a size that meets the algorithm's current minimum strength.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func (r *StaticIvRule) check(ctx *api.Context) {
	file := ctx.File
	ctor, byteArg := staticIvConstructorAndByteArg(file, ctx.Idx)
	if ctor == "" || byteArg == 0 {
		return
	}
	if !staticIvImportsOrQualifiesSpec(file, ctor, ctx.Idx) {
		return
	}
	if !isLiteralByteArray(file, byteArg) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"IV parameter spec is built from literal bytes. Generate a fresh random IV for each encryption operation.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func (r *HardcodedSecretKeyRule) check(ctx *api.Context) {
	file := ctx.File
	keyArg := secretKeySpecLiteralKeyArg(file, ctx.Idx)
	if keyArg == 0 || !isLiteralByteArray(file, keyArg) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"SecretKeySpec is constructed from hardcoded bytes. Load keys from Android Keystore or a secret manager instead.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
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

func (r *StartActivityWithUntrustedIntentRule) check(ctx *api.Context) {
	file := ctx.File
	if !startActivityLaunchNames[javaAwareCallName(file, ctx.Idx)] {
		return
	}
	if !sourceImportsOrMentions(file, "android.content.Intent") {
		return
	}
	intentVar := startActivityIntentArgument(file, ctx.Idx)
	if intentVar == "" {
		return
	}
	parseStart, ok := startActivityFindParseURIAssignment(file, ctx.Idx, intentVar)
	if !ok {
		return
	}
	if startActivityHasGuardBetween(file, ctx.Idx, intentVar, parseStart) {
		return
	}
	f := r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Launching an Intent parsed from an untrusted URI without setPackage or component guard can enable intent redirection.")
	f.Confidence = r.Confidence()
	ctx.Emit(f)
}

func weakMessageDigestReceiverIsJavaSecurity(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		navExpr, _ := flatCallExpressionParts(file, call)
		if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "getInstance" {
			return false
		}
		if file.FlatNamedChildCount(navExpr) == 0 {
			return false
		}
		return weakMessageDigestReceiverTextIsJavaSecurity(file, file.FlatNodeText(file.FlatNamedChild(navExpr, 0)))
	case "method_invocation":
		return weakMessageDigestReceiverTextIsJavaSecurity(file, javaMethodReceiverText(file, call))
	default:
		return false
	}
}

func weakMessageDigestReceiverTextIsJavaSecurity(file *scanner.File, receiver string) bool {
	receiver = strings.TrimSpace(receiver)
	if receiver == "java.security.MessageDigest" {
		return true
	}
	if receiver != "MessageDigest" {
		return false
	}
	if weakMessageDigestFileDeclaresMessageDigest(file) {
		return false
	}
	return sourceImportsOrMentions(file, "java.security.MessageDigest")
}

func weakMessageDigestFileDeclaresMessageDigest(file *scanner.File) bool {
	found := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "type_alias"} {
		file.FlatWalkNodes(0, nodeType, func(node uint32) {
			if found {
				return
			}
			if extractIdentifierFlat(file, node) == "MessageDigest" {
				found = true
			}
		})
	}
	return found
}

func staticIvConstructorAndByteArg(file *scanner.File, idx uint32) (string, uint32) {
	switch file.FlatType(idx) {
	case "call_expression":
		name := flatCallExpressionName(file, idx)
		if name != "IvParameterSpec" && name != "GCMParameterSpec" {
			return "", 0
		}
		_, args := flatCallExpressionParts(file, idx)
		argIndex := 0
		if name == "GCMParameterSpec" {
			argIndex = 1
		}
		arg := flatPositionalValueArgument(file, args, argIndex)
		return name, flatValueArgumentExpression(file, arg)
	case "object_creation_expression":
		text := file.FlatNodeText(idx)
		name := ""
		switch {
		case strings.Contains(text, "IvParameterSpec"):
			name = "IvParameterSpec"
		case strings.Contains(text, "GCMParameterSpec"):
			name = "GCMParameterSpec"
		default:
			return "", 0
		}
		args, ok := file.FlatFindChild(idx, "argument_list")
		if !ok {
			return "", 0
		}
		argIndex := 0
		if name == "GCMParameterSpec" {
			argIndex = 1
		}
		if file.FlatNamedChildCount(args) <= argIndex {
			return "", 0
		}
		return name, file.FlatNamedChild(args, argIndex)
	default:
		return "", 0
	}
}

func secretKeySpecLiteralKeyArg(file *scanner.File, idx uint32) uint32 {
	if !secretKeySpecImportsOrQualifies(file, idx) {
		return 0
	}
	switch file.FlatType(idx) {
	case "call_expression":
		if flatCallExpressionName(file, idx) != "SecretKeySpec" {
			return 0
		}
		_, args := flatCallExpressionParts(file, idx)
		arg := flatPositionalValueArgument(file, args, 0)
		return flatValueArgumentExpression(file, arg)
	case "object_creation_expression":
		text := file.FlatNodeText(idx)
		if !strings.Contains(text, "SecretKeySpec") {
			return 0
		}
		args, ok := file.FlatFindChild(idx, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return 0
		}
		return file.FlatNamedChild(args, 0)
	default:
		return 0
	}
}

func secretKeySpecImportsOrQualifies(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "javax.crypto.spec.SecretKeySpec") {
		return true
	}
	if staticIvFileDeclaresType(file, "SecretKeySpec") {
		return false
	}
	return sourceImportsOrMentions(file, "javax.crypto.spec.SecretKeySpec") ||
		sourceImportsOrMentions(file, "javax.crypto.spec.*")
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

func staticIvImportsOrQualifiesSpec(file *scanner.File, ctor string, idx uint32) bool {
	text := file.FlatNodeText(idx)
	fqn := "javax.crypto.spec." + ctor
	if strings.Contains(text, fqn) {
		return true
	}
	if staticIvFileDeclaresType(file, ctor) {
		return false
	}
	return sourceImportsOrMentions(file, fqn) || sourceImportsOrMentions(file, "javax.crypto.spec.*")
}

func staticIvFileDeclaresType(file *scanner.File, name string) bool {
	found := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "type_alias"} {
		file.FlatWalkNodes(0, nodeType, func(node uint32) {
			if found {
				return
			}
			if extractIdentifierFlat(file, node) == name {
				found = true
			}
		})
	}
	return found
}

func isLiteralByteArray(file *scanner.File, expr uint32) bool {
	if file == nil || expr == 0 {
		return false
	}
	expr = flatUnwrapParenExpr(file, expr)
	text := strings.TrimSpace(file.FlatNodeText(expr))
	return staticIvLiteralKotlinByteArray(text) ||
		staticIvLiteralJavaByteArray(text) ||
		staticIvLiteralStringBytes(text) ||
		staticIvLiteralDecodeBytes(text)
}

func staticIvLiteralKotlinByteArray(text string) bool {
	if !strings.HasPrefix(text, "byteArrayOf(") && !strings.HasPrefix(text, "kotlin.byteArrayOf(") {
		return false
	}
	inside := text[strings.Index(text, "(")+1:]
	inside = strings.TrimSuffix(strings.TrimSpace(inside), ")")
	return staticIvLiteralList(inside)
}

func staticIvLiteralJavaByteArray(text string) bool {
	if !strings.HasPrefix(text, "new byte[]") {
		return false
	}
	open := strings.Index(text, "{")
	closeBrace := strings.LastIndex(text, "}")
	if open < 0 || closeBrace <= open {
		return false
	}
	return staticIvLiteralList(text[open+1 : closeBrace])
}

func staticIvLiteralList(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, part := range strings.Split(text, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "(byte)")
		part = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(part, "u"), "U"))
		part = strings.TrimSuffix(strings.TrimSuffix(part, "L"), "l")
		if part == "" {
			return false
		}
		if len(part) >= 3 && part[0] == '\'' && part[len(part)-1] == '\'' {
			continue
		}
		part = strings.ReplaceAll(part, "_", "")
		base := 10
		if strings.HasPrefix(part, "0x") || strings.HasPrefix(part, "0X") {
			base = 16
			part = part[2:]
		}
		if _, err := strconv.ParseInt(part, base, 64); err != nil {
			return false
		}
	}
	return true
}

func staticIvLiteralStringBytes(text string) bool {
	return strings.HasPrefix(text, "\"") &&
		(strings.Contains(text, "\".toByteArray(") ||
			strings.Contains(text, "\".encodeToByteArray(") ||
			strings.Contains(text, "\".getBytes(") ||
			strings.HasSuffix(text, "\".bytes"))
}

func staticIvLiteralDecodeBytes(text string) bool {
	return ((strings.Contains(text, "Base64.decode(") || strings.Contains(text, "Base64.getDecoder().decode(")) && strings.Contains(text, "\"")) ||
		(strings.HasPrefix(text, "\"") && strings.Contains(text, "\".hexToByteArray("))
}

var startActivityLaunchNames = map[string]bool{
	"startActivity":          true,
	"startActivities":        true,
	"startActivityForResult": true,
}

func startActivityIntentArgument(file *scanner.File, call uint32) string {
	var arg uint32
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		arg = flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return ""
		}
		arg = file.FlatNamedChild(args, 0)
	default:
		return ""
	}
	arg = flatUnwrapParenExpr(file, arg)
	switch file.FlatType(arg) {
	case "simple_identifier", "identifier":
		return strings.TrimSpace(file.FlatNodeText(arg))
	default:
		return ""
	}
}

func startActivityFindParseURIAssignment(file *scanner.File, launch uint32, intentVar string) (uint32, bool) {
	scope, ok := flatEnclosingCallable(file, launch)
	if !ok {
		return 0, false
	}
	launchStart := file.FlatStartByte(launch)
	var parseStart uint32
	for _, nodeType := range []string{"call_expression", "method_invocation"} {
		file.FlatWalkNodes(scope, nodeType, func(call uint32) {
			if parseStart != 0 || call == launch || file.FlatStartByte(call) >= launchStart {
				return
			}
			if !sameEnclosingCallable(file, call, scope) || javaAwareCallName(file, call) != "parseUri" {
				return
			}
			if !startActivityParseURIReceiverIsIntent(file, call) {
				return
			}
			container := startActivityAssignmentContainer(file, call)
			if container == 0 || !startActivityContainerAssignsReceiver(file, container, intentVar) {
				return
			}
			parseStart = file.FlatStartByte(container)
		})
	}
	return parseStart, parseStart != 0
}

func sameEnclosingCallable(file *scanner.File, idx, scope uint32) bool {
	got, ok := flatEnclosingCallable(file, idx)
	return ok && got == scope
}

func startActivityParseURIReceiverIsIntent(file *scanner.File, call uint32) bool {
	receiver := ""
	switch file.FlatType(call) {
	case "call_expression":
		nav, _ := flatCallExpressionParts(file, call)
		if nav == 0 || file.FlatNamedChildCount(nav) == 0 {
			return false
		}
		receiver = file.FlatNodeText(file.FlatNamedChild(nav, 0))
	case "method_invocation":
		receiver = javaMethodReceiverText(file, call)
	}
	receiver = strings.TrimSpace(receiver)
	return receiver == "android.content.Intent" ||
		(receiver == "Intent" && !startActivityFileDeclaresIntent(file) && sourceImportsOrMentions(file, "android.content.Intent"))
}

func startActivityFileDeclaresIntent(file *scanner.File) bool {
	found := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "type_alias"} {
		file.FlatWalkNodes(0, nodeType, func(node uint32) {
			if found {
				return
			}
			if extractIdentifierFlat(file, node) == "Intent" {
				found = true
			}
		})
	}
	return found
}

func startActivityAssignmentContainer(file *scanner.File, call uint32) uint32 {
	for cur, ok := file.FlatParent(call); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "property_declaration", "variable_declaration", "local_variable_declaration", "assignment":
			return cur
		case "function_declaration", "method_declaration", "lambda_literal", "class_declaration", "source_file":
			return 0
		}
	}
	return 0
}

func startActivityContainerAssignsReceiver(file *scanner.File, container uint32, name string) bool {
	text := file.FlatNodeText(container)
	assign := strings.Index(text, "=")
	if assign < 0 {
		return false
	}
	before := text[:assign]
	for _, token := range strings.FieldsFunc(before, func(r rune) bool {
		return r != '_' && r != '$' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
	}) {
		if token == name {
			return true
		}
	}
	return false
}

func startActivityHasGuardBetween(file *scanner.File, launch uint32, intentVar string, parseStart uint32) bool {
	scope, ok := flatEnclosingCallable(file, launch)
	if !ok {
		return false
	}
	launchStart := file.FlatStartByte(launch)
	guarded := false
	for _, nodeType := range []string{"call_expression", "method_invocation", "assignment"} {
		file.FlatWalkNodes(scope, nodeType, func(node uint32) {
			if guarded || node == launch || !sameEnclosingCallable(file, node, scope) {
				return
			}
			start := file.FlatStartByte(node)
			if start <= parseStart || start >= launchStart {
				return
			}
			text := strings.TrimSpace(file.FlatNodeText(node))
			if strings.Contains(text, intentVar+".setPackage(") ||
				strings.Contains(text, intentVar+".setComponent(") ||
				strings.Contains(text, intentVar+".setClassName(") ||
				strings.Contains(text, intentVar+".component") {
				guarded = true
			}
		})
	}
	return guarded
}

func weakKeySizeInitReceiver(file *scanner.File, call uint32) string {
	switch file.FlatType(call) {
	case "call_expression":
		receiver := kotlinCallReceiverChain(file, call)
		if strings.Contains(receiver, ".") {
			return ""
		}
		return receiver
	case "method_invocation":
		receiver := javaMethodReceiverText(file, call)
		if strings.Contains(receiver, ".") {
			return ""
		}
		return receiver
	default:
		return ""
	}
}

func weakKeySizeFirstIntegerArg(file *scanner.File, call uint32) (int, bool) {
	var expr uint32
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		arg := flatPositionalValueArgument(file, args, 0)
		expr = flatValueArgumentExpression(file, arg)
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return 0, false
		}
		expr = file.FlatNamedChild(args, 0)
	default:
		return 0, false
	}
	expr = flatUnwrapParenExpr(file, expr)
	text := strings.TrimSpace(file.FlatNodeText(expr))
	text = strings.TrimSuffix(strings.TrimSuffix(text, "L"), "l")
	text = strings.ReplaceAll(text, "_", "")
	size, err := strconv.Atoi(text)
	if err != nil {
		return 0, false
	}
	return size, true
}

func weakKeySizeFindGeneratorAlgorithm(file *scanner.File, initCall uint32, receiver string) (string, bool) {
	scope, ok := flatEnclosingCallable(file, initCall)
	if !ok {
		return "", false
	}
	initStart := file.FlatStartByte(initCall)
	var algo string
	var found bool
	for _, nodeType := range []string{"call_expression", "method_invocation"} {
		file.FlatWalkNodes(scope, nodeType, func(call uint32) {
			if found || call == initCall || file.FlatStartByte(call) >= initStart {
				return
			}
			if javaAwareCallName(file, call) != "getInstance" {
				return
			}
			if !weakKeySizeGeneratorReceiverIsKnown(file, call) {
				return
			}
			value, ok := weakGetInstanceFirstStringArg(file, call)
			if !ok {
				return
			}
			container := weakKeySizeAssignmentContainer(file, call)
			if container == 0 || !weakKeySizeContainerAssignsReceiver(file, container, receiver) {
				return
			}
			algo = value
			found = true
		})
	}
	return algo, found
}

func weakKeySizeGeneratorReceiverIsKnown(file *scanner.File, call uint32) bool {
	receiver := ""
	switch file.FlatType(call) {
	case "call_expression":
		navExpr, _ := flatCallExpressionParts(file, call)
		if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
			return false
		}
		receiver = file.FlatNodeText(file.FlatNamedChild(navExpr, 0))
	case "method_invocation":
		receiver = javaMethodReceiverText(file, call)
	default:
		return false
	}
	receiver = strings.TrimSpace(receiver)
	switch receiver {
	case "java.security.KeyPairGenerator":
		return true
	case "javax.crypto.KeyGenerator":
		return true
	case "KeyPairGenerator":
		return !weakKeySizeFileDeclaresType(file, "KeyPairGenerator") &&
			sourceImportsOrMentions(file, "java.security.KeyPairGenerator")
	case "KeyGenerator":
		return !weakKeySizeFileDeclaresType(file, "KeyGenerator") &&
			sourceImportsOrMentions(file, "javax.crypto.KeyGenerator")
	default:
		return false
	}
}

func weakKeySizeFileDeclaresType(file *scanner.File, name string) bool {
	found := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "type_alias"} {
		file.FlatWalkNodes(0, nodeType, func(node uint32) {
			if found {
				return
			}
			if extractIdentifierFlat(file, node) == name {
				found = true
			}
		})
	}
	return found
}

func weakKeySizeAssignmentContainer(file *scanner.File, call uint32) uint32 {
	for cur, ok := file.FlatParent(call); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "property_declaration", "variable_declaration", "local_variable_declaration", "assignment":
			return cur
		case "function_declaration", "method_declaration", "source_file":
			return 0
		}
	}
	return 0
}

func weakKeySizeContainerAssignsReceiver(file *scanner.File, container uint32, receiver string) bool {
	text := file.FlatNodeText(container)
	assign := strings.Index(text, "=")
	if assign < 0 {
		return false
	}
	before := text[:assign]
	for _, token := range strings.FieldsFunc(before, func(r rune) bool {
		return r != '_' && r != '$' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
	}) {
		if token == receiver {
			return true
		}
	}
	return false
}

func weakKeySizeThreshold(algo string) (int, bool) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(algo), "-", ""))
	switch normalized {
	case "RSA", "DSA":
		return 2048, true
	case "EC", "ECDSA":
		return 224, true
	case "AES":
		return 128, true
	}
	if strings.HasPrefix(normalized, "HMACSHA") {
		return 256, true
	}
	return 0, false
}

func weakMacReceiverIsJavaxCrypto(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		navExpr, _ := flatCallExpressionParts(file, call)
		if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "getInstance" {
			return false
		}
		if file.FlatNamedChildCount(navExpr) == 0 {
			return false
		}
		return weakMacReceiverTextIsJavaxCrypto(file, file.FlatNodeText(file.FlatNamedChild(navExpr, 0)))
	case "method_invocation":
		return weakMacReceiverTextIsJavaxCrypto(file, javaMethodReceiverText(file, call))
	default:
		return false
	}
}

func weakMacReceiverTextIsJavaxCrypto(file *scanner.File, receiver string) bool {
	receiver = strings.TrimSpace(receiver)
	if receiver == "javax.crypto.Mac" {
		return true
	}
	if receiver != "Mac" {
		return false
	}
	if weakMacFileDeclaresMac(file) {
		return false
	}
	return sourceImportsOrMentions(file, "javax.crypto.Mac")
}

func weakMacFileDeclaresMac(file *scanner.File) bool {
	found := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "type_alias"} {
		file.FlatWalkNodes(0, nodeType, func(node uint32) {
			if found {
				return
			}
			if extractIdentifierFlat(file, node) == "Mac" {
				found = true
			}
		})
	}
	return found
}

func weakGetInstanceFirstStringArg(file *scanner.File, call uint32) (string, bool) {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		arg := flatPositionalValueArgument(file, args, 0)
		if arg == 0 {
			return "", false
		}
		return weakSecurityStringLiteralValue(file, flatValueArgumentExpression(file, arg))
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return "", false
		}
		return weakSecurityStringLiteralValue(file, file.FlatNamedChild(args, 0))
	default:
		return "", false
	}
}

func weakSecurityStringLiteralValue(file *scanner.File, expr uint32) (string, bool) {
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		if flatContainsStringInterpolation(file, expr) {
			return "", false
		}
		text := strings.TrimSpace(file.FlatNodeText(expr))
		if value := stringLiteralContent(file, expr); value != "" || text == `""` {
			return value, true
		}
		value, err := strconv.Unquote(text)
		if err != nil {
			return "", false
		}
		return value, true
	default:
		return "", false
	}
}

// getInstanceReceiverIsJavaxCipher returns true when the navigation
// expression's receiver is javax.crypto.Cipher — either explicitly
// spelled `javax.crypto.Cipher.getInstance(...)` or a bare `Cipher`
// reference backed by an import of `javax.crypto.Cipher` with no
// conflicting user-defined Cipher class in the same file.
func getInstanceReceiverIsJavaxCipher(file *scanner.File, navExpr uint32) bool {
	if file == nil || navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	text := strings.TrimSpace(file.FlatNodeText(receiver))
	if text == "javax.crypto.Cipher" {
		return true
	}
	if text != "Cipher" {
		return false
	}
	if getInstanceFileDeclaresCipherType(file) {
		return false
	}
	return getInstanceFileImportsJavaxCipher(file)
}

func getInstanceFileImportsJavaxCipher(file *scanner.File) bool {
	found := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		if found {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(node))
		text = strings.TrimPrefix(text, "import ")
		text = strings.TrimSuffix(text, ";")
		text = strings.TrimSpace(text)
		if text == "javax.crypto.Cipher" || text == "javax.crypto.*" {
			found = true
		}
	})
	return found
}

func getInstanceFileDeclaresCipherType(file *scanner.File) bool {
	found := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "type_alias"} {
		file.FlatWalkNodes(0, nodeType, func(node uint32) {
			if found {
				return
			}
			if extractIdentifierFlat(file, node) == "Cipher" {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

func rsaNoPaddingReceiverIsJavaxCipher(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		navExpr, _ := flatCallExpressionParts(file, call)
		return getInstanceReceiverIsJavaxCipher(file, navExpr)
	case "method_invocation":
		receiver := strings.TrimSpace(javaMethodReceiverText(file, call))
		if receiver == "javax.crypto.Cipher" {
			return true
		}
		if receiver != "Cipher" || getInstanceFileDeclaresCipherType(file) {
			return false
		}
		return sourceImportsOrMentions(file, "javax.crypto.Cipher")
	default:
		return false
	}
}

func rsaNoPaddingAlgorithm(algo string) bool {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(algo)), "/")
	return len(parts) == 3 && parts[0] == "RSA" && parts[1] != "" && parts[2] == "NOPADDING"
}

func prngFromSystemTimeCryptoFile(file *scanner.File) bool {
	if file == nil {
		return false
	}
	text := string(file.Content)
	return strings.Contains(text, "import javax.crypto") ||
		strings.Contains(text, "import java.security") ||
		strings.Contains(text, "import javax.net.ssl")
}

func prngFromSystemTimeTestPath(path string) bool {
	path = strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(path, "/tests/fixtures/") {
		return false
	}
	return strings.Contains(path, "/src/test/") || strings.Contains(path, "/src/androidtest/")
}

func prngFromSystemTimeRandomConstructor(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "call_expression":
		name := flatCallExpressionName(file, idx)
		if name != "Random" {
			return false
		}
		return sourceImportsOrMentions(file, "java.util.Random") ||
			sourceImportsOrMentions(file, "kotlin.random.Random") ||
			strings.Contains(file.FlatNodeText(idx), "java.util.Random(")
	case "object_creation_expression":
		text := strings.TrimSpace(file.FlatNodeText(idx))
		if !strings.Contains(text, "Random(") {
			return false
		}
		if strings.Contains(text, "SecureRandom") {
			return false
		}
		return sourceImportsOrMentions(file, "java.util.Random") || strings.Contains(text, "java.util.Random")
	default:
		return false
	}
}

func prngFromSystemTimeSeedArg(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, idx)
		return flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
	case "object_creation_expression":
		args, ok := file.FlatFindChild(idx, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return 0
		}
		return file.FlatNamedChild(args, 0)
	default:
		return 0
	}
}

func prngFromSystemTimeSeedExpr(text string) bool {
	text = strings.ReplaceAll(strings.TrimSpace(text), " ", "")
	return strings.Contains(text, "System.currentTimeMillis()") ||
		strings.Contains(text, "System.nanoTime()") ||
		strings.Contains(text, "Date().time") ||
		strings.Contains(text, "Date().getTime()") ||
		strings.Contains(text, "newDate().getTime()") ||
		strings.Contains(text, "Calendar.getInstance().timeInMillis") ||
		strings.Contains(text, "Calendar.getInstance().getTimeInMillis()") ||
		strings.Contains(text, "Instant.now().toEpochMilli()")
}

func okHTTPDisableSslValidationChainText(file *scanner.File, idx uint32) string {
	best := file.FlatNodeText(idx)
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		typ := file.FlatType(cur)
		if typ == "function_declaration" || typ == "method_declaration" || typ == "class_declaration" || typ == "source_file" {
			break
		}
		text := file.FlatNodeText(cur)
		if strings.Contains(text, "OkHttpClient.Builder") {
			best = text
		}
	}
	return best
}

func disableCertificatePinningEmptyBuilder(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || javaAwareCallName(file, idx) != "build" {
		return false
	}
	if !sourceImportsOrMentions(file, "okhttp3.CertificatePinner") &&
		!sourceImportsOrMentions(file, "okhttp3.CertificatePinner.Builder") {
		return false
	}
	chainText := disableCertificatePinningChainText(file, idx)
	compact := strings.Join(strings.Fields(chainText), "")
	if strings.Contains(compact, ".add(") {
		return false
	}
	return strings.Contains(compact, "CertificatePinner.Builder().build(") ||
		strings.Contains(compact, "newCertificatePinner.Builder().build(") ||
		(sourceImportsOrMentions(file, "okhttp3.CertificatePinner.Builder") &&
			(strings.Contains(compact, "Builder().build(") || strings.Contains(compact, "newBuilder().build(")))
}

func disableCertificatePinningChainText(file *scanner.File, idx uint32) string {
	best := file.FlatNodeText(idx)
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		typ := file.FlatType(cur)
		if typ == "function_declaration" || typ == "method_declaration" || typ == "class_declaration" || typ == "source_file" {
			break
		}
		text := file.FlatNodeText(cur)
		compact := strings.Join(strings.Fields(text), "")
		if strings.Contains(compact, "CertificatePinner.Builder") ||
			strings.Contains(compact, "Builder().") ||
			strings.Contains(compact, "newBuilder().") {
			best = text
		}
	}
	return best
}

func okHTTPDisableSslValidationAlwaysTrueVerifier(chainText string) bool {
	text := strings.ReplaceAll(chainText, " ", "")
	return strings.Contains(text, "->true") ||
		strings.Contains(text, "returntrue;") ||
		strings.Contains(text, "returntrue}")
}

func okHTTPDisableSslValidationUnsafeTrustManager(chainText string) bool {
	text := strings.ToLower(chainText)
	return strings.Contains(text, "trustall") ||
		strings.Contains(text, "unsafe") ||
		(strings.Contains(text, "x509trustmanager") &&
			(strings.Contains(text, "checkservertrusted") || strings.Contains(text, "checkclienttrusted")))
}

func allowAllHostnameVerifierClass(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "class_declaration" {
		return false
	}
	if !sourceImportsOrMentions(file, "javax.net.ssl.HostnameVerifier") {
		return false
	}
	if allowAllHostnameVerifierHasLocalShadow(file, idx) {
		return false
	}
	header := allowAllHostnameVerifierClassHeader(file, idx)
	if header == "" {
		return false
	}
	return insecureTrustManagerTextHasTypeToken(header, "HostnameVerifier") ||
		strings.Contains(header, "javax.net.ssl.HostnameVerifier")
}

func allowAllHostnameVerifierClassHeader(file *scanner.File, idx uint32) string {
	text := strings.TrimSpace(file.FlatNodeText(idx))
	if open := strings.Index(text, "{"); open >= 0 {
		return text[:open]
	}
	return text
}

func allowAllHostnameVerifierHasLocalShadow(file *scanner.File, owner uint32) bool {
	shadowed := false
	file.FlatWalkNodes(0, "class_declaration", func(candidate uint32) {
		if candidate != owner && extractIdentifierFlat(file, candidate) == "HostnameVerifier" {
			shadowed = true
		}
	})
	file.FlatWalkNodes(0, "interface_declaration", func(candidate uint32) {
		if candidate != owner && extractIdentifierFlat(file, candidate) == "HostnameVerifier" {
			shadowed = true
		}
	})
	return shadowed
}

func allowAllHostnameVerifierVerifyMethod(file *scanner.File, owner uint32) uint32 {
	var match uint32
	check := func(method uint32) {
		if match != 0 || !allowAllHostnameVerifierMethodOwnedBy(file, method, owner) {
			return
		}
		if allowAllHostnameVerifierMethodName(file, method) != "verify" {
			return
		}
		text := file.FlatNodeText(method)
		if allowAllHostnameVerifierParamCount(text) != 2 {
			return
		}
		if allowAllHostnameVerifierMethodReturnsTrue(text) {
			match = method
		}
	}
	file.FlatWalkNodes(owner, "function_declaration", check)
	file.FlatWalkNodes(owner, "method_declaration", check)
	return match
}

func allowAllHostnameVerifierMethodOwnedBy(file *scanner.File, method, owner uint32) bool {
	actual, ok := flatEnclosingAncestor(file, method, "class_declaration")
	return ok && actual == owner
}

func allowAllHostnameVerifierMethodName(file *scanner.File, method uint32) string {
	switch file.FlatType(method) {
	case "function_declaration":
		return flatFunctionName(file, method)
	case "method_declaration":
		text := file.FlatNodeText(method)
		if strings.Contains(text, "verify(") {
			return "verify"
		}
	}
	return ""
}

func allowAllHostnameVerifierParamCount(methodText string) int {
	name := strings.Index(methodText, "verify")
	if name < 0 {
		return -1
	}
	openRel := strings.Index(methodText[name:], "(")
	if openRel < 0 {
		return -1
	}
	open := name + openRel
	closeParen := matchingParenIndex(methodText, open)
	if closeParen < 0 {
		return -1
	}
	params := strings.TrimSpace(methodText[open+1 : closeParen])
	if params == "" {
		return 0
	}
	depth := 0
	count := 1
	for _, r := range params {
		switch r {
		case '(', '<', '[', '{':
			depth++
		case ')', '>', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}

func allowAllHostnameVerifierMethodReturnsTrue(methodText string) bool {
	cleaned := stripLineAndBlockComments(methodText)
	if body, ok := firstBraceBody(cleaned); ok {
		body = strings.TrimSpace(stripLineAndBlockComments(body))
		return body == "return true" || body == "return true;"
	}
	if eq := strings.LastIndex(cleaned, "="); eq >= 0 {
		expr := strings.TrimSpace(cleaned[eq+1:])
		return expr == "true"
	}
	return false
}

func matchingParenIndex(text string, open int) int {
	if open < 0 || open >= len(text) || text[open] != '(' {
		return -1
	}
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func insecureTrustManagerDecl(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	typ := file.FlatType(idx)
	if typ != "class_declaration" && typ != "object_literal" && typ != "object_creation_expression" {
		return false
	}
	if !sourceImportsOrMentions(file, "javax.net.ssl.X509TrustManager") &&
		!sourceImportsOrMentions(file, "javax.net.ssl.TrustManager") {
		return false
	}
	text := file.FlatNodeText(idx)
	if !insecureTrustManagerTextHasTypeToken(text, "X509TrustManager") &&
		!insecureTrustManagerTextHasTypeToken(text, "TrustManager") {
		return false
	}
	if strings.Contains(text, " by ") {
		return false
	}
	return true
}

func insecureTrustManagerTextHasTypeToken(text, token string) bool {
	return strutil.ContainsTokenWordBoundary(text, token)
}

func insecureTrustManagerTrivialChecks(file *scanner.File, owner uint32) []uint32 {
	var findings []uint32
	check := func(method uint32) {
		if !insecureTrustManagerMethodOwnedBy(file, method, owner) {
			return
		}
		name := insecureTrustManagerMethodName(file, method)
		if name != "checkServerTrusted" && name != "checkClientTrusted" {
			return
		}
		if insecureTrustManagerMethodBodyTrivial(file.FlatNodeText(method), name) {
			findings = append(findings, method)
		}
	}
	file.FlatWalkNodes(owner, "function_declaration", check)
	file.FlatWalkNodes(owner, "method_declaration", check)
	return findings
}

func insecureTrustManagerMethodOwnedBy(file *scanner.File, method, owner uint32) bool {
	actual, ok := flatEnclosingAncestor(file, method, "class_declaration", "object_literal", "object_creation_expression")
	return ok && actual == owner
}

func insecureTrustManagerMethodName(file *scanner.File, method uint32) string {
	switch file.FlatType(method) {
	case "function_declaration":
		return flatFunctionName(file, method)
	case "method_declaration":
		text := file.FlatNodeText(method)
		for _, name := range []string{"checkServerTrusted", "checkClientTrusted"} {
			if strings.Contains(text, name+"(") {
				return name
			}
		}
	}
	return ""
}

func insecureTrustManagerMethodBodyTrivial(methodText, name string) bool {
	open := strings.Index(methodText, name)
	if open < 0 {
		return false
	}
	body, ok := firstBraceBody(methodText[open:])
	if !ok {
		return false
	}
	body = stripLineAndBlockComments(body)
	body = strings.TrimSpace(body)
	return body == "" || body == "return" || body == "return;"
}

func firstBraceBody(text string) (string, bool) {
	start := strings.Index(text, "{")
	if start < 0 {
		return "", false
	}
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start+1 : i], true
			}
		}
	}
	return "", false
}

func stripLineAndBlockComments(text string) string {
	for {
		start := strings.Index(text, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(text[start+2:], "*/")
		if end < 0 {
			return text[:start]
		}
		text = text[:start] + text[start+2+end+2:]
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

var implicitPendingIntentMethods = map[string]bool{
	"getActivity":   true,
	"getBroadcast":  true,
	"getService":    true,
	"getActivities": true,
}

func implicitPendingIntentCall(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	name := javaAwareCallName(file, idx)
	if !implicitPendingIntentMethods[name] {
		return false
	}
	if !sourceImportsOrMentions(file, "android.app.PendingIntent") {
		return false
	}
	text := file.FlatNodeText(idx)
	compact := strings.Join(strings.Fields(text), "")
	if !strings.Contains(compact, "PendingIntent."+name+"(") {
		return false
	}
	if strings.Contains(text, "PendingIntentCompat") {
		return false
	}
	flags, ok := implicitPendingIntentFlagsText(file, idx)
	if !ok {
		return false
	}
	return !strings.Contains(flags, "FLAG_IMMUTABLE") && !strings.Contains(flags, "FLAG_MUTABLE")
}

func implicitPendingIntentFlagsText(file *scanner.File, call uint32) (string, bool) {
	if file == nil || call == 0 {
		return "", false
	}
	switch file.FlatType(call) {
	case "call_expression":
		args := flatCallKeyArguments(file, call)
		if args == 0 {
			return "", false
		}
		if named := flatNamedValueArgument(file, args, "flags"); named != 0 {
			expr := flatValueArgumentExpression(file, named)
			if expr == 0 {
				return "", false
			}
			return file.FlatNodeText(expr), true
		}
		var last uint32
		for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatType(arg) != "value_argument" || flatHasValueArgumentLabel(file, arg) {
				continue
			}
			last = arg
		}
		if last == 0 {
			return "", false
		}
		expr := flatValueArgumentExpression(file, last)
		if expr == 0 {
			return "", false
		}
		return file.FlatNodeText(expr), true
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return "", false
		}
		return file.FlatNodeText(file.FlatNamedChild(args, file.FlatNamedChildCount(args)-1)), true
	default:
		return "", false
	}
}

func getInstanceFirstStringArg(file *scanner.File, args uint32) (string, bool) {
	if file == nil || args == 0 {
		return "", false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if !file.FlatIsNamed(arg) {
			continue
		}
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 {
			return "", false
		}
		switch file.FlatType(expr) {
		case "string_literal", "line_string_literal", "multi_line_string_literal":
			if flatContainsStringInterpolation(file, expr) {
				return "", false
			}
			text := file.FlatNodeText(expr)
			if strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`) {
				return strings.TrimSuffix(strings.TrimPrefix(text, `"""`), `"""`), true
			}
			value, err := strconv.Unquote(text)
			if err != nil {
				return "", false
			}
			return value, true
		}
		return "", false
	}
	return "", false
}

// ExportedContentProviderRule detects exported content providers without permission.
type ExportedContentProviderRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ExportedContentProviderRule) Confidence() float64 { return api.ConfidenceMedium }

func exportedPermissionEnforcedInClass(file *scanner.File, classIdx uint32) bool {
	body, _ := file.FlatFindChild(classIdx, "class_body")
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if found {
			return
		}
		switch flatCallExpressionName(file, call) {
		case "enforceCallingPermission",
			"enforceCallingOrSelfPermission",
			"checkCallingPermission",
			"checkCallingOrSelfPermission",
			"enforcePermission",
			"checkPermission",
			"enforceUriPermission",
			"checkUriPermission":
			found = true
		}
	})
	return found
}

func exportedClassExtendsAndroid(file *scanner.File, classIdx uint32, simpleName, fqn string) bool {
	if !privacyClassDirectlyExtendsFlat(file, classIdx, simpleName) {
		return false
	}
	return missingPermissionHasImport(file, fqn)
}

func (r *ExportedContentProviderRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if !exportedClassExtendsAndroid(file, idx, "ContentProvider", "android.content.ContentProvider") {
		return
	}
	if exportedPermissionEnforcedInClass(file, idx) {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"ContentProvider subclass may be exported without permission. Ensure permissions are enforced.")
}

// ExportedReceiverRule detects exported receivers without permission.
type ExportedReceiverRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ExportedReceiverRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ExportedReceiverRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if !exportedClassExtendsAndroid(file, idx, "BroadcastReceiver", "android.content.BroadcastReceiver") {
		return
	}
	if exportedPermissionEnforcedInClass(file, idx) {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"BroadcastReceiver subclass may be exported without permission. Ensure permissions are enforced.")
}

// GrantAllUrisRule detects overly broad URI permissions.
type GrantAllUrisRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GrantAllUrisRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GrantAllUrisRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	name := grantURIPermissionCallName(file, idx)
	if name != "grantUriPermission" && name != "grantUriPermissions" {
		return
	}
	confidence := grantURIPermissionConfidence(ctx, idx)
	if confidence <= 0 {
		return
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Overly broad URI permission grant. Consider restricting to specific URIs.")
	f.Confidence = confidence
	ctx.Emit(f)
}

func grantURIPermissionCallName(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "call_expression":
		return flatCallExpressionName(file, idx)
	case "method_invocation":
		return wrongViewCastCallName(file, idx)
	default:
		return ""
	}
}

func grantURIPermissionConfidence(ctx *api.Context, idx uint32) float64 {
	file := ctx.File
	if file.FlatType(idx) == "method_invocation" {
		return grantURIPermissionJavaConfidence(file, idx)
	}
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr != 0 && ctx.Resolver != nil {
		receiver := file.FlatNamedChild(navExpr, 0)
		if receiver != 0 {
			typ := ctx.Resolver.ResolveFlatNode(receiver, file)
			if typ == nil || typ.Kind == typeinfer.TypeUnknown {
				if file.FlatType(receiver) == "simple_identifier" {
					typ = ctx.Resolver.ResolveByNameFlat(file.FlatNodeText(receiver), receiver, file)
				}
			}
			if typ != nil && typ.Kind != typeinfer.TypeUnknown {
				if grantURITypeIsContext(ctx.Resolver, typ) {
					return 1.0
				}
				return 0
			}
		}
	}
	if navExpr != 0 {
		receiver := file.FlatNamedChild(navExpr, 0)
		if receiver != 0 && file.FlatType(receiver) == "simple_identifier" {
			recvName := file.FlatNodeText(receiver)
			if recvName == "this" || recvName == "context" || recvName == "ctx" {
				if missingPermissionHasImport(file, "android.content.Context") {
					return 0.85
				}
			}
		}
	} else if missingPermissionHasImport(file, "android.content.Context") {
		// Unqualified call in a file that imports Context — likely an Activity/Service.
		_ = args
		return 0.85
	}
	return 0.7
}

func grantURIPermissionJavaConfidence(file *scanner.File, idx uint32) float64 {
	receiver := wrongViewCastCallReceiverName(file, idx)
	switch receiver {
	case "context", "ctx", "this":
		if sourceImportsOrMentions(file, "android.content.Context") ||
			sourceImportsOrMentions(file, "android.app.Activity") ||
			sourceImportsOrMentions(file, "android.app.Service") {
			return 0.85
		}
	case "":
		if sourceImportsOrMentions(file, "android.content.Context") ||
			sourceImportsOrMentions(file, "android.app.Activity") ||
			sourceImportsOrMentions(file, "android.app.Service") {
			return 0.85
		}
	default:
		if strings.HasSuffix(receiver, ".Context") || strings.HasSuffix(receiver, ".Activity") || strings.HasSuffix(receiver, ".Service") {
			return 0.85
		}
	}
	return 0
}

func grantURITypeIsContext(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType) bool {
	if typ == nil {
		return false
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		if name == "Context" || name == "android.content.Context" {
			return true
		}
		if resolver == nil {
			return false
		}
		info := resolver.ClassHierarchy(name)
		if info == nil {
			return false
		}
		if info.Name == "Context" || info.FQN == "android.content.Context" {
			return true
		}
		for _, supertype := range info.Supertypes {
			if visit(supertype) {
				return true
			}
		}
		return false
	}
	return visit(typ.FQN) || visit(typ.Name)
}

// UnprotectedDynamicReceiverRule detects dynamic broadcast receivers
// registered for public actions without a broadcast permission.
type UnprotectedDynamicReceiverRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *UnprotectedDynamicReceiverRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnprotectedDynamicReceiverRule) check(ctx *api.Context) {
	file := ctx.File
	if javaAwareCallName(file, ctx.Idx) != "registerReceiver" {
		return
	}
	if !dynamicReceiverHasContextReceiver(file, ctx.Idx) {
		return
	}
	if !dynamicReceiverHasMissingPermission(file, ctx.Idx) {
		return
	}
	if !dynamicReceiverFilterMentionsPublicAction(file, ctx.Idx) {
		return
	}
	ctx.EmitAt(file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Dynamic broadcast receiver registered for a public action without a broadcast permission. Pass a non-null permission or restrict the receiver.")
}

func dynamicReceiverHasContextReceiver(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 {
		return false
	}
	text := file.FlatNodeText(call)
	if strings.Contains(text, "LocalBroadcastManager") {
		return false
	}
	importsContext := sourceImportsOrMentions(file, "android.content.Context") ||
		sourceImportsOrMentions(file, "android.app.Activity") ||
		sourceImportsOrMentions(file, "android.app.Service") ||
		sourceImportsOrMentions(file, "android.content.ContextWrapper")
	importsContextCompat := sourceImportsOrMentions(file, "androidx.core.content.ContextCompat") ||
		sourceImportsOrMentions(file, "android.support.v4.content.ContextCompat")
	switch file.FlatType(call) {
	case "call_expression":
		navExpr, _ := flatCallExpressionParts(file, call)
		if navExpr == 0 {
			return importsContext
		}
		receiver := file.FlatNamedChild(navExpr, 0)
		if receiver == 0 {
			return false
		}
		receiverText := strings.TrimSpace(file.FlatNodeText(receiver))
		switch receiverText {
		case "this", "context", "ctx", "activity", "service":
			return importsContext || importsContextCompat
		case "ContextCompat", "androidx.core.content.ContextCompat":
			return importsContextCompat
		default:
			return (strings.HasSuffix(receiverText, "Context") ||
				strings.HasSuffix(receiverText, "ContextWrapper") ||
				strings.HasSuffix(receiverText, "Activity") ||
				strings.HasSuffix(receiverText, "Service")) && importsContext
		}
	case "method_invocation":
		receiver := wrongViewCastCallReceiverName(file, call)
		switch receiver {
		case "":
			return importsContext
		case "this", "context", "ctx", "activity", "service":
			return importsContext || importsContextCompat
		case "ContextCompat", "androidx.core.content.ContextCompat":
			return importsContextCompat
		default:
			return (strings.HasSuffix(receiver, "Context") ||
				strings.HasSuffix(receiver, "ContextWrapper") ||
				strings.HasSuffix(receiver, "Activity") ||
				strings.HasSuffix(receiver, "Service")) && importsContext
		}
	default:
		return false
	}
}

func dynamicReceiverHasMissingPermission(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return false
		}
		positional := flatValueArgumentTexts(file, args)
		switch len(positional) {
		case 2:
			return true
		case 4:
			return strings.TrimSpace(positional[2]) == "null"
		default:
			return false
		}
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return false
		}
		positional := flatJavaArgumentTexts(file, args)
		switch len(positional) {
		case 2:
			return true
		case 4:
			return strings.TrimSpace(positional[2]) == "null"
		default:
			return false
		}
	default:
		return false
	}
}

func broadcastReceiverExportedFlagMissing(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return false
		}
		positional := flatValueArgumentTexts(file, args)
		if broadcastReceiverUsesContextCompat(file, call) {
			if len(positional) < 4 {
				return true
			}
			return !broadcastReceiverFlagTextContainsExportedConstant(positional[len(positional)-1])
		}
		if len(positional) < 3 {
			return true
		}
		return !broadcastReceiverFlagTextContainsExportedConstant(positional[2])
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return false
		}
		positional := flatJavaArgumentTexts(file, args)
		if broadcastReceiverUsesContextCompat(file, call) {
			if len(positional) < 4 {
				return true
			}
			return !broadcastReceiverFlagTextContainsExportedConstant(positional[len(positional)-1])
		}
		if len(positional) < 3 {
			return true
		}
		return !broadcastReceiverFlagTextContainsExportedConstant(positional[2])
	default:
		return false
	}
}

func broadcastReceiverUsesContextCompat(file *scanner.File, call uint32) bool {
	text := file.FlatNodeText(call)
	if strings.Contains(text, "ContextCompat.registerReceiver") ||
		strings.Contains(text, "androidx.core.content.ContextCompat.registerReceiver") ||
		strings.Contains(text, "android.support.v4.content.ContextCompat.registerReceiver") {
		return true
	}
	switch wrongViewCastCallReceiverName(file, call) {
	case "ContextCompat", "androidx.core.content.ContextCompat", "android.support.v4.content.ContextCompat":
		return true
	default:
		return false
	}
}

func broadcastReceiverFlagTextContainsExportedConstant(text string) bool {
	return strings.Contains(text, "RECEIVER_EXPORTED") || strings.Contains(text, "RECEIVER_NOT_EXPORTED")
}

func flatValueArgumentTexts(file *scanner.File, args uint32) []string {
	var out []string
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" || flatHasValueArgumentLabel(file, arg) {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 {
			continue
		}
		out = append(out, strings.TrimSpace(file.FlatNodeText(expr)))
	}
	return out
}

func flatJavaArgumentTexts(file *scanner.File, args uint32) []string {
	var out []string
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		out = append(out, strings.TrimSpace(file.FlatNodeText(child)))
	}
	return out
}

var dynamicReceiverPublicActionString = regexp.MustCompile(`"com\.example\.[A-Za-z0-9_.-]+"`)

func dynamicReceiverFilterMentionsPublicAction(file *scanner.File, call uint32) bool {
	filterText := dynamicReceiverFilterText(file, call)
	if filterText == "" {
		return false
	}
	publicActions := []string{
		"Intent.ACTION_SCREEN_ON",
		"Intent.ACTION_BATTERY_CHANGED",
		"Intent.ACTION_USER_PRESENT",
		"Intent.ACTION_BOOT_COMPLETED",
		"android.intent.action.SCREEN_ON",
		"android.intent.action.BATTERY_CHANGED",
		"android.intent.action.USER_PRESENT",
		"android.intent.action.BOOT_COMPLETED",
	}
	for _, action := range publicActions {
		if strings.Contains(filterText, action) {
			return true
		}
	}
	return dynamicReceiverPublicActionString.MatchString(filterText)
}

func dynamicReceiverFilterText(file *scanner.File, call uint32) string {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return ""
		}
		filter := flatPositionalValueArgument(file, args, 1)
		expr := flatValueArgumentExpression(file, filter)
		if expr == 0 {
			return ""
		}
		return file.FlatNodeText(expr)
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) < 2 {
			return ""
		}
		return file.FlatNodeText(file.FlatNamedChild(args, 1))
	default:
		return ""
	}
}
