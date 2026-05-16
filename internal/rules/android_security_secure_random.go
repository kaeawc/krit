package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SecureRandomRule detects insecure random usage: java.util.Random where
// SecureRandom should be used, and deterministic SecureRandom.setSeed(long)
// calls that Android lint reports as SecureRandom issues.
type SecureRandomRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *SecureRandomRule) Confidence() float64 { return 0.85 }

func (r *SecureRandomRule) check(ctx *api.Context) {
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		r.checkKotlinCall(ctx, file, ctx.Idx)
	case "method_invocation":
		r.checkJavaMethodInvocation(ctx, file, ctx.Idx)
	}
}

func (r *SecureRandomRule) checkKotlinCall(ctx *api.Context, file *scanner.File, call uint32) {
	if r.isInsecureKotlinRandomCall(file, call) {
		ctx.Emit(r.Finding(file, file.FlatRow(call)+1, file.FlatCol(call)+1,
			"Using java.util.Random. Use java.security.SecureRandom for security-sensitive operations."))
		return
	}

	if !secureRandomIsKotlinSetSeedCall(file, call) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(call)+1, file.FlatCol(call)+1,
		"Calling SecureRandom.setSeed() with a fixed or time-based seed makes output predictable. Use the default SecureRandom seeding."))
}

func (r *SecureRandomRule) isInsecureKotlinRandomCall(file *scanner.File, call uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, call)
	insecure := false
	if navExpr != 0 {
		ids := flatNavigationIdentifierParts(file, navExpr)
		if len(ids) == 3 && ids[0] == "java" && ids[1] == "util" && ids[2] == "Random" {
			insecure = true
		}
	} else if flatCallExpressionName(file, call) == "Random" && secureRandomImportsJavaUtilRandom(file) {
		insecure = true
	}
	return insecure
}

func secureRandomIsKotlinSetSeedCall(file *scanner.File, call uint32) bool {
	navExpr, args := flatCallExpressionParts(file, call)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "setSeed" {
		return false
	}
	arg := singleKotlinValueArgument(file, args)
	if arg == 0 || !secureRandomIsDeterministicSeedArgument(file, arg) {
		return false
	}
	receiver := flatNavigationReceiver(file, navExpr)
	return secureRandomKotlinReceiverIsSecureRandom(file, receiver)
}

func (r *SecureRandomRule) checkJavaMethodInvocation(ctx *api.Context, file *scanner.File, call uint32) {
	if !secureRandomIsJavaSetSeedCall(file, call) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(call)+1, file.FlatCol(call)+1,
		"Calling SecureRandom.setSeed() with a fixed or time-based seed makes output predictable. Use the default SecureRandom seeding."))
}

func secureRandomImportsJavaUtilRandom(file *scanner.File) bool {
	javaUtil := false
	kotlinRandom := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		switch missingPermissionIdentifierPath(file, node) {
		case "java.util.Random":
			javaUtil = true
		case "kotlin.random.Random":
			kotlinRandom = true
		}
	})
	return javaUtil && !kotlinRandom
}

func singleKotlinValueArgument(file *scanner.File, args uint32) uint32 {
	if file == nil || args == 0 {
		return 0
	}
	var out uint32
	count := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 {
			continue
		}
		out = expr
		count++
	}
	if count != 1 {
		return 0
	}
	return out
}

func secureRandomIsDeterministicSeedArgument(file *scanner.File, arg uint32) bool {
	arg = flatUnwrapParenExpr(file, arg)
	switch file.FlatType(arg) {
	case "integer_literal", "long_literal", "decimal_integer_literal", "hex_integer_literal", "octal_integer_literal", "binary_integer_literal":
		return true
	case "call_expression":
		navExpr, args := flatCallExpressionParts(file, arg)
		return args != 0 && file.FlatNamedChildCount(args) == 0 && secureRandomIsSystemTimeNavigation(file, navExpr)
	case "method_invocation":
		return secureRandomIsJavaSystemTimeCall(file, arg)
	default:
		return false
	}
}

func secureRandomIsSystemTimeNavigation(file *scanner.File, navExpr uint32) bool {
	ids := flatNavigationIdentifierParts(file, navExpr)
	return len(ids) == 2 && ids[0] == "System" && (ids[1] == "currentTimeMillis" || ids[1] == "nanoTime")
}

func secureRandomKotlinReceiverIsSecureRandom(file *scanner.File, receiver uint32) bool {
	if file == nil || receiver == 0 {
		return false
	}
	switch file.FlatType(receiver) {
	case "call_expression":
		return secureRandomKotlinConstructorCall(file, receiver)
	case "simple_identifier":
		name := file.FlatNodeString(receiver, nil)
		return secureRandomKotlinIdentifierIsSecureRandom(file, name)
	case "navigation_expression":
		return secureRandomKotlinQualifiedConstructor(file, receiver)
	default:
		return false
	}
}

func secureRandomKotlinConstructorCall(file *scanner.File, call uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr != 0 {
		return secureRandomKotlinQualifiedConstructor(file, navExpr)
	}
	return flatCallExpressionName(file, call) == "SecureRandom" && secureRandomImportsJavaSecuritySecureRandom(file)
}

func secureRandomKotlinQualifiedConstructor(file *scanner.File, navExpr uint32) bool {
	ids := flatNavigationIdentifierParts(file, navExpr)
	return len(ids) == 3 && ids[0] == "java" && ids[1] == "security" && ids[2] == "SecureRandom"
}

func secureRandomKotlinIdentifierIsSecureRandom(file *scanner.File, name string) bool {
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		if found {
			return
		}
		decl, ok := file.FlatFindChild(prop, "variable_declaration")
		if !ok || !flatDeclarationContainsIdentifier(file, decl, name) {
			return
		}
		if secureRandomKotlinPropertyDeclaresSecureRandom(file, prop) {
			found = true
		}
	})
	return found
}

func secureRandomKotlinPropertyDeclaresSecureRandom(file *scanner.File, prop uint32) bool {
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type":
			if secureRandomTypeNodeNamesSecureRandom(file, child) {
				return true
			}
		case "call_expression":
			if secureRandomKotlinConstructorCall(file, child) {
				return true
			}
		}
	}
	return false
}

func flatDeclarationContainsIdentifier(file *scanner.File, node uint32, name string) bool {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" && file.FlatNodeString(child, nil) == name {
			return true
		}
	}
	return false
}

func secureRandomImportsJavaSecuritySecureRandom(file *scanner.File) bool {
	importedSecureRandom := false
	importedKotlinRandom := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		switch missingPermissionIdentifierPath(file, node) {
		case "java.security.SecureRandom":
			importedSecureRandom = true
		case "kotlin.random.Random":
			importedKotlinRandom = true
		}
	})
	return importedSecureRandom && !importedKotlinRandom
}

func secureRandomTypeNodeNamesSecureRandom(file *scanner.File, node uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(node))
	return text == "SecureRandom" || text == "java.security.SecureRandom"
}

func secureRandomIsJavaSetSeedCall(file *scanner.File, call uint32) bool {
	if secureRandomJavaMethodName(file, call) != "setSeed" {
		return false
	}
	arg := singleJavaArgument(file, call)
	if arg == 0 || !secureRandomIsDeterministicSeedArgument(file, arg) {
		return false
	}
	receiver := secureRandomJavaMethodReceiver(file, call)
	if receiver == 0 {
		return false
	}
	if file.FlatType(receiver) == "object_creation_expression" {
		return secureRandomJavaObjectCreationIsSecureRandom(file, receiver)
	}
	if file.FlatType(receiver) == "identifier" {
		return secureRandomJavaIdentifierIsSecureRandom(file, file.FlatNodeString(receiver, nil))
	}
	return false
}

func secureRandomJavaMethodName(file *scanner.File, call uint32) string {
	last := ""
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "identifier" {
			last = file.FlatNodeString(child, nil)
		}
	}
	return last
}

func secureRandomJavaMethodReceiver(file *scanner.File, call uint32) uint32 {
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "identifier", "object_creation_expression", "method_invocation", "field_access":
			return child
		case "argument_list":
			return 0
		}
	}
	return 0
}

func singleJavaArgument(file *scanner.File, call uint32) uint32 {
	args, ok := file.FlatFindChild(call, "argument_list")
	if !ok {
		return 0
	}
	var out uint32
	count := 0
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		out = child
		count++
	}
	if count != 1 {
		return 0
	}
	return out
}

func secureRandomIsJavaSystemTimeCall(file *scanner.File, call uint32) bool {
	if name := secureRandomJavaMethodName(file, call); name != "currentTimeMillis" && name != "nanoTime" {
		return false
	}
	if args, ok := file.FlatFindChild(call, "argument_list"); !ok || file.FlatNamedChildCount(args) != 0 {
		return false
	}
	receiver := secureRandomJavaMethodReceiver(file, call)
	return receiver != 0 && file.FlatType(receiver) == "identifier" && file.FlatNodeString(receiver, nil) == "System"
}

func secureRandomJavaIdentifierIsSecureRandom(file *scanner.File, name string) bool {
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "local_variable_declaration", func(decl uint32) {
		if found {
			return
		}
		if !secureRandomJavaLocalDeclarationNames(file, decl, name) {
			return
		}
		if secureRandomJavaLocalDeclarationIsSecureRandom(file, decl) {
			found = true
		}
	})
	return found
}

func secureRandomJavaLocalDeclarationNames(file *scanner.File, decl uint32, name string) bool {
	var found bool
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "variable_declarator" {
			continue
		}
		if ident, ok := file.FlatFindChild(child, "identifier"); ok && file.FlatNodeString(ident, nil) == name {
			found = true
		}
	}
	return found
}

func secureRandomJavaLocalDeclarationIsSecureRandom(file *scanner.File, decl uint32) bool {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier":
			if secureRandomJavaTypeNodeNamesSecureRandom(file, child) {
				return true
			}
		case "variable_declarator":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatType(gc) == "object_creation_expression" && secureRandomJavaObjectCreationIsSecureRandom(file, gc) {
					return true
				}
			}
		}
	}
	return false
}

func secureRandomJavaObjectCreationIsSecureRandom(file *scanner.File, node uint32) bool {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "scoped_identifier":
			if secureRandomJavaTypeNodeNamesSecureRandom(file, child) {
				return true
			}
		}
	}
	return false
}

func secureRandomJavaTypeNodeNamesSecureRandom(file *scanner.File, node uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(node))
	if text == "java.security.SecureRandom" {
		return true
	}
	return text == "SecureRandom" && secureRandomJavaImportsSecureRandom(file)
}

func secureRandomJavaImportsSecureRandom(file *scanner.File) bool {
	found := false
	file.FlatWalkNodes(0, "import_declaration", func(node uint32) {
		if strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(node)), "import "), ";") == "java.security.SecureRandom" {
			found = true
		}
	})
	return found
}
