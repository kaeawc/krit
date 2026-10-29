package rules

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// StaticIvRule detects IV/GCM parameter specs built from inline literal bytes.
type StaticIvRule struct {
	FlatDispatchBase
	AndroidRule
}

type HardcodedSecretKeyRule struct {
	FlatDispatchBase
	AndroidRule
}

type PrngFromSystemTimeRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *StaticIvRule) Confidence() float64 { return api.ConfidenceMediumHigh }

func (r *HardcodedSecretKeyRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *PrngFromSystemTimeRule) Confidence() float64 { return api.ConfidenceMedium }

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
	return fileDeclaredTypeNames(file)[name]
}

// fileDeclaredTypeNames is the names of the file's classes, objects and type
// aliases, collected once per file (#944) instead of a whole-file walk for
// every visited call.
func fileDeclaredTypeNames(file *scanner.File) map[string]bool {
	return filefacts.FileFact(fileFactsCache(), file, slotDeclaredTypeNames, func() map[string]bool {
		names := map[string]bool{}
		for _, nodeType := range []string{"class_declaration", "object_declaration", "type_alias"} {
			file.FlatWalkNodes(0, nodeType, func(node uint32) {
				names[extractIdentifierFlat(file, node)] = true
			})
		}
		return names
	})
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

func prngFromSystemTimeCryptoFile(file *scanner.File) bool {
	if file == nil {
		return false
	}
	// Once per file (#944): the rule asks from every node it visits.
	return filefacts.FileFact(fileFactsCache(), file, slotPrngCryptoFile, func() bool {
		text := string(file.Content)
		return strings.Contains(text, "import javax.crypto") ||
			strings.Contains(text, "import java.security") ||
			strings.Contains(text, "import javax.net.ssl")
	})
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
