package rules

import (
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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

func (r *WeakMessageDigestRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *WeakMacAlgorithmRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *WeakKeySizeRule) Confidence() float64 { return api.ConfidenceMedium }

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
