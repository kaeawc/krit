package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// FileFromUntrustedPathRule detects File(parent, child) construction inside
// extract/upload/download-style functions where child is either a literal with
// parent traversal (`..`) or a non-literal path segment without an obvious
// canonical-path containment check.
type FileFromUntrustedPathRule struct {
	FlatDispatchBase
	BaseRule
}

// TempFileWorldReadableRule detects setReadable/setWritable/setExecutable(true,
// false) on a File that came from File.createTempFile or
// Files.createTempFile, which makes the temporary file world-accessible.
type TempFileWorldReadableRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TempFileWorldReadableRule) Confidence() float64 { return api.ConfidenceMedium }

var tempFilePermissionSetterNames = map[string]bool{
	"setReadable":   true,
	"setWritable":   true,
	"setExecutable": true,
}

func tempFileSetterAndReceiver(file *scanner.File, call uint32) (string, uint32, bool) {
	switch file.FlatType(call) {
	case "call_expression":
		if !tempFilePermissionSetterNames[flatCallExpressionName(file, call)] {
			return "", 0, false
		}
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return "", 0, false
		}
		receiver := flatReceiverNameFromCall(file, call)
		if receiver == "" {
			return "", 0, false
		}
		return receiver, args, true
	case "method_invocation":
		if !tempFilePermissionSetterNames[javaMethodInvocationName(file, call)] {
			return "", 0, false
		}
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok {
			return "", 0, false
		}
		receiver := javaMethodReceiverText(file, call)
		// Only accept a bare identifier receiver — chained receivers like
		// File.createTempFile(...).setReadable(...) wouldn't have a binding
		// to look up anyway, and we'd risk matching unrelated setReadable.
		if receiver == "" || strings.Contains(receiver, ".") {
			return "", 0, false
		}
		return receiver, args, true
	}
	return "", 0, false
}

func tempFileSetterArgIsLiteral(file *scanner.File, args uint32, index int, want string) bool {
	if args == 0 {
		return false
	}
	switch file.FlatType(args) {
	case "value_arguments":
		arg := flatPositionalValueArgument(file, args, index)
		if arg == 0 {
			return false
		}
		inner := arg
		if c := file.FlatNamedChild(arg, 0); c != 0 {
			inner = c
		}
		inner = flatUnwrapParenExpr(file, inner)
		return strings.TrimSpace(file.FlatNodeText(inner)) == want
	case "argument_list":
		var current int
		for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if current == index {
				return strings.TrimSpace(file.FlatNodeText(child)) == want
			}
			current++
		}
	}
	return false
}

func receiverBoundToCreateTempFile(file *scanner.File, call uint32, receiver string) bool {
	fn, ok := flatEnclosingCallable(file, call)
	if !ok {
		return false
	}
	targetRow := file.FlatRow(call)
	declType, identExtractor := "", (func(uint32) string)(nil)
	switch file.FlatType(fn) {
	case "function_declaration":
		declType = "property_declaration"
		identExtractor = func(decl uint32) string { return propertyDeclarationName(file, decl) }
	case "method_declaration":
		declType = "local_variable_declaration"
		identExtractor = func(decl uint32) string { return javaLocalDeclaratorName(file, decl) }
	default:
		return false
	}
	found := false
	file.FlatWalkNodes(fn, declType, func(decl uint32) {
		if found || file.FlatRow(decl) > targetRow {
			return
		}
		if identExtractor(decl) != receiver {
			return
		}
		if expressionContainsCreateTempFileCall(file, decl) {
			found = true
		}
	})
	return found
}

func javaLocalDeclaratorName(file *scanner.File, decl uint32) string {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "variable_declarator" {
			continue
		}
		if ident, ok := file.FlatFindChild(child, "identifier"); ok {
			return file.FlatNodeText(ident)
		}
	}
	return ""
}

func expressionContainsCreateTempFileCall(file *scanner.File, root uint32) bool {
	if root == 0 {
		return false
	}
	for _, nodeType := range [2]string{"call_expression", "method_invocation"} {
		found := false
		file.FlatWalkNodes(root, nodeType, func(call uint32) {
			if found {
				return
			}
			if isCreateTempFileCall(file, call) {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

func isCreateTempFileCall(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		if flatCallExpressionName(file, call) != "createTempFile" {
			return false
		}
		receiver := flatReceiverNameFromCall(file, call)
		return receiver == "File" || receiver == "Files"
	case "method_invocation":
		if javaMethodInvocationName(file, call) != "createTempFile" {
			return false
		}
		receiver := javaMethodReceiverText(file, call)
		return receiver == "File" || receiver == "Files"
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *FileFromUntrustedPathRule) Confidence() float64 { return api.ConfidenceMedium }

// ZipSlipUncheckedRule detects classic Zip-slip vulnerabilities where a zip
// extraction loop builds a destination File from a zip entry name without a
// subsequent canonical-path containment check.
type ZipSlipUncheckedRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Security rule. Detection pattern-matches known-insecure API shapes and
// argument literals without confirming the receiver type. Classified per
// roadmap/17.
func (r *ZipSlipUncheckedRule) Confidence() float64 { return api.ConfidenceMedium }

var zipSlipAPIMarkers = []string{
	"ZipInputStream",
	"ZipFile",
	"JarFile",
	"JarInputStream",
	"ZipEntry",
}

func zipSlipChildArgIsEntryName(text string) bool {
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, "="); idx >= 0 {
		text = strings.TrimSpace(text[idx+1:])
	}
	return strings.HasSuffix(text, ".name")
}

func zipSlipFunctionMentionsZipAPI(file *scanner.File, fn uint32) bool {
	text := file.FlatNodeText(fn)
	for _, marker := range zipSlipAPIMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func zipSlipFunctionHasGuard(file *scanner.File, fn uint32) bool {
	text := file.FlatNodeText(fn)
	if strings.Contains(text, "canonicalPath.startsWith(") {
		return true
	}
	if strings.Contains(text, ".normalize()") && strings.Contains(text, ".startsWith(") {
		return true
	}
	if strings.Contains(text, ".toRealPath(") && strings.Contains(text, ".startsWith(") {
		return true
	}
	return false
}

func zipSlipInsideExtractionLoop(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "while_statement", "for_statement", "do_while_statement":
			return true
		case "lambda_literal":
			if call, found := flatEnclosingAncestor(file, cur, "call_expression"); found {
				switch flatCallExpressionName(file, call) {
				case "use", "forEach", "forEachIndexed", "onEach":
					return true
				}
			}
		}
	}
	return false
}

func isRiskyFileFromPathFunction(name string) bool {
	for _, fragment := range []string{"upload", "extract", "unzip", "download"} {
		if strings.Contains(name, fragment) {
			return true
		}
	}
	return false
}

func valueArgumentExpressionTextFlat(file *scanner.File, arg uint32) string {
	text := strings.TrimSpace(file.FlatNodeText(arg))
	if idx := strings.Index(text, "="); idx >= 0 {
		return strings.TrimSpace(text[idx+1:])
	}
	return text
}

func isStringLiteralExpr(text string) bool {
	return strings.HasPrefix(text, "\"") || strings.HasPrefix(text, "\"\"\"")
}

func hasCanonicalPathContainmentGuardFlat(file *scanner.File, fn uint32, parentExpr string) bool {
	if file == nil || parentExpr == "" {
		return false
	}
	fnText := file.FlatNodeText(fn)
	return strings.Contains(fnText, ".canonicalPath.startsWith(") &&
		strings.Contains(fnText, parentExpr+".canonicalPath") &&
		strings.Contains(fnText, "File.separator")
}
