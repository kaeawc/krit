package rules

import (
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// GetInstanceRule detects Cipher.getInstance with insecure algorithms (ECB, DES).
type GetInstanceRule struct {
	FlatDispatchBase
	AndroidRule
}

type RsaNoPaddingRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. AST-based
// detection resolves the call shape structurally (call_expression →
// navigation_expression(Cipher.getInstance) → string_literal arg) and
// confirms the receiver is javax.crypto.Cipher via its import and the
// absence of a declaration named Cipher in scope at the call. Algorithm
// inspection uses the literal's parsed content, not regex slicing.
func (r *GetInstanceRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *RsaNoPaddingRule) Confidence() float64 { return api.ConfidenceHigh }

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

// getInstanceReceiverIsJavaxCipher returns true when the navigation
// expression's receiver is javax.crypto.Cipher — either explicitly
// spelled `javax.crypto.Cipher.getInstance(...)` or a bare `Cipher`
// that an import of javax.crypto.Cipher backs and that nothing in scope
// at the call shadows.
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
	if getInstanceCipherNameShadowedAt(file, navExpr) {
		return false
	}
	return getInstanceFileImportsJavaxCipher(file)
}

// getInstanceFileImportsJavaxCipher reports whether the file's imports
// make a bare `Cipher` javax.crypto.Cipher: an import of the class under
// its own name, or the javax.crypto star import with no explicit import
// binding another type to `Cipher`. It reads each import's identifier
// path, so a comment the parser attaches to the import_header, backticks,
// or spacing around the dots do not change the answer.
func getInstanceFileImportsJavaxCipher(file *scanner.File) bool {
	explicit, star, other := false, false, false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		path, alias, wildcard := getInstanceImportHeaderParts(file, node)
		switch {
		case wildcard:
			star = star || path == "javax.crypto"
		case path == "javax.crypto.Cipher":
			explicit = explicit || alias == "" || alias == "Cipher"
		case alias == "Cipher", alias == "" && strings.HasSuffix(path, ".Cipher"):
			other = true
		}
	})
	return !other && (explicit || star)
}

// getInstanceImportHeaderParts returns a Kotlin import's dotted path,
// its alias (empty when it has none), and whether it is a star import.
func getInstanceImportHeaderParts(file *scanner.File, header uint32) (path, alias string, wildcard bool) {
	var parts []string
	for child := file.FlatFirstChild(header); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "identifier":
			for seg := file.FlatFirstChild(child); seg != 0; seg = file.FlatNextSib(seg) {
				if file.FlatType(seg) == "simple_identifier" {
					parts = append(parts, getInstanceIdentifierName(file, seg))
				}
			}
		case "wildcard_import":
			wildcard = true
		case "import_alias":
			for name := file.FlatFirstChild(child); name != 0; name = file.FlatNextSib(name) {
				switch file.FlatType(name) {
				case "type_identifier", "simple_identifier":
					alias = getInstanceIdentifierName(file, name)
				}
			}
		}
	}
	return strings.Join(parts, "."), alias, wildcard
}

func getInstanceIdentifierName(file *scanner.File, idx uint32) string {
	return strings.Trim(file.FlatNodeString(idx, nil), "`")
}

// getInstanceCipherNameShadowedAt reports whether a declaration named
// `Cipher` that is in scope at idx wins over an import: a top-level
// declaration of the file, a member of a class enclosing idx (or of that
// class's companion object), or a parameter or earlier local of an
// enclosing function, lambda, or loop. A `Cipher` nested in a class that
// does not enclose idx is out of scope and does not shadow.
func getInstanceCipherNameShadowedAt(file *scanner.File, idx uint32) bool {
	if file.Language == scanner.LangJava {
		return getInstanceJavaCipherNameShadowedAt(file, idx)
	}
	for scope, ok := file.FlatParent(idx); ok; scope, ok = file.FlatParent(scope) {
		switch file.FlatType(scope) {
		case "source_file":
			if getInstanceMembersDeclareCipher(file, scope) {
				return true
			}
		case "class_declaration", "object_declaration", "companion_object":
			if getInstanceClassDeclaresCipher(file, scope) {
				return true
			}
		case "statements":
			if getInstanceEarlierLocalDeclaresCipher(file, scope, idx) {
				return true
			}
		case "function_declaration", "anonymous_function", "secondary_constructor", "setter":
			if params, found := file.FlatFindChild(scope, "function_value_parameters"); found &&
				getInstanceChildrenDeclareCipher(file, params, "parameter") {
				return true
			}
			if getInstanceChildrenDeclareCipher(file, scope, "parameter_with_optional_type") {
				return true
			}
		case "lambda_literal":
			if params, found := file.FlatFindChild(scope, "lambda_parameters"); found &&
				getInstanceBindingsDeclareCipher(file, params) {
				return true
			}
		case "for_statement":
			if getInstanceBindingsDeclareCipher(file, scope) {
				return true
			}
		}
	}
	return false
}

// getInstanceClassDeclaresCipher reports whether a class declares a
// member named `Cipher`: a nested class, object, type alias, or property,
// a val/var constructor parameter, or the same in its companion object,
// whose members are in scope throughout the class.
func getInstanceClassDeclaresCipher(file *scanner.File, class uint32) bool {
	if ctor, ok := file.FlatFindChild(class, "primary_constructor"); ok {
		for param := file.FlatFirstChild(ctor); param != 0; param = file.FlatNextSib(param) {
			if file.FlatType(param) != "class_parameter" {
				continue
			}
			if _, isProperty := file.FlatFindChild(param, "binding_pattern_kind"); isProperty &&
				getInstanceDeclarationName(file, param) == "Cipher" {
				return true
			}
		}
	}
	for body := file.FlatFirstChild(class); body != 0; body = file.FlatNextSib(body) {
		switch file.FlatType(body) {
		case "class_body", "enum_class_body":
			if getInstanceMembersDeclareCipher(file, body) {
				return true
			}
		}
	}
	return false
}

func getInstanceMembersDeclareCipher(file *scanner.File, body uint32) bool {
	for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
		switch file.FlatType(member) {
		case "class_declaration", "object_declaration", "type_alias":
			if getInstanceDeclarationName(file, member) == "Cipher" {
				return true
			}
		case "property_declaration":
			if getInstanceBindingsDeclareCipher(file, member) {
				return true
			}
		case "companion_object":
			if getInstanceDeclarationName(file, member) == "Cipher" {
				return true
			}
			if companionBody, ok := file.FlatFindChild(member, "class_body"); ok &&
				getInstanceMembersDeclareCipher(file, companionBody) {
				return true
			}
		}
	}
	return false
}

// getInstanceEarlierLocalDeclaresCipher reports whether a block declares
// a local named `Cipher` in a statement that ends before idx starts; a
// local is not in scope in its own initializer or in earlier statements.
func getInstanceEarlierLocalDeclaresCipher(file *scanner.File, block, idx uint32) bool {
	start := file.FlatStartByte(idx)
	for stmt := file.FlatFirstChild(block); stmt != 0; stmt = file.FlatNextSib(stmt) {
		if file.FlatEndByte(stmt) > start {
			return false
		}
		switch file.FlatType(stmt) {
		case "property_declaration":
			if getInstanceBindingsDeclareCipher(file, stmt) {
				return true
			}
		case "class_declaration", "object_declaration":
			if getInstanceDeclarationName(file, stmt) == "Cipher" {
				return true
			}
		}
	}
	return false
}

// getInstanceBindingsDeclareCipher reports whether a property, lambda
// parameter list, or loop header binds `Cipher`, directly or as a
// component of a destructuring declaration.
func getInstanceBindingsDeclareCipher(file *scanner.File, owner uint32) bool {
	for child := file.FlatFirstChild(owner); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "variable_declaration":
			if getInstanceDeclarationName(file, child) == "Cipher" {
				return true
			}
		case "multi_variable_declaration":
			if getInstanceChildrenDeclareCipher(file, child, "variable_declaration") {
				return true
			}
		}
	}
	return false
}

func getInstanceChildrenDeclareCipher(file *scanner.File, owner uint32, childType string) bool {
	for child := file.FlatFirstChild(owner); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == childType && getInstanceDeclarationName(file, child) == "Cipher" {
			return true
		}
	}
	return false
}

func getInstanceDeclarationName(file *scanner.File, idx uint32) string {
	return strings.Trim(extractIdentifierFlat(file, idx), "`")
}

// getInstanceJavaCipherNameShadowedAt is the Java counterpart of
// getInstanceCipherNameShadowedAt: a top-level type, a member type or
// field of an enclosing class, a parameter or earlier local of an
// enclosing method, or a single-type import of another `Cipher`.
func getInstanceJavaCipherNameShadowedAt(file *scanner.File, idx uint32) bool {
	start := file.FlatStartByte(idx)
	for scope, ok := file.FlatParent(idx); ok; scope, ok = file.FlatParent(scope) {
		if getInstanceJavaTypeScopeDeclaresCipher(file, scope) ||
			getInstanceJavaLocalScopeDeclaresCipher(file, scope, start) {
			return true
		}
	}
	return false
}

// getInstanceJavaTypeScopeDeclaresCipher covers the scopes that declare
// types and fields: the compilation unit and class-like bodies.
func getInstanceJavaTypeScopeDeclaresCipher(file *scanner.File, scope uint32) bool {
	switch file.FlatType(scope) {
	case "program":
		return getInstanceJavaMembersDeclareCipher(file, scope) || getInstanceJavaImportsOtherCipher(file, scope)
	case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
		for body := file.FlatFirstChild(scope); body != 0; body = file.FlatNextSib(body) {
			switch file.FlatType(body) {
			case "class_body", "interface_body", "enum_body", "enum_body_declarations":
				if getInstanceJavaMembersDeclareCipher(file, body) {
					return true
				}
			case "formal_parameters":
				if getInstanceJavaParametersDeclareCipher(file, body) {
					return true
				}
			}
		}
	case "class_body", "enum_body_declarations":
		// Anonymous class and enum bodies have no declaration node.
		return getInstanceJavaMembersDeclareCipher(file, scope)
	}
	return false
}

// getInstanceJavaLocalScopeDeclaresCipher covers the scopes that bind
// parameters and locals: methods, lambdas, loops, catch and
// try-with-resources clauses, and statement lists.
func getInstanceJavaLocalScopeDeclaresCipher(file *scanner.File, scope, start uint32) bool {
	switch file.FlatType(scope) {
	case "method_declaration", "constructor_declaration":
		params, found := file.FlatFindChild(scope, "formal_parameters")
		return found && getInstanceJavaParametersDeclareCipher(file, params)
	case "lambda_expression":
		return getInstanceJavaLambdaDeclaresCipher(file, scope)
	case "enhanced_for_statement":
		// The loop variable is the identifier before the colon.
		return getInstanceJavaIdentifierBeforeTokenIsCipher(file, scope, ":")
	case "catch_clause":
		param, found := file.FlatFindChild(scope, "catch_formal_parameter")
		return found && getInstanceJavaIdentifierBeforeTokenIsCipher(file, param, "")
	case "try_with_resources_statement":
		resources, _ := file.FlatFindChild(scope, "resource_specification")
		for resource := file.FlatFirstChild(resources); resources != 0 && resource != 0; resource = file.FlatNextSib(resource) {
			// A resource's name follows its type and precedes the `=`.
			if file.FlatType(resource) == "resource" &&
				getInstanceJavaIdentifierBeforeTokenIsCipher(file, resource, "=") {
				return true
			}
		}
	case "block", "constructor_body", "switch_block_statement_group", "for_statement":
		return getInstanceJavaEarlierLocalDeclaresCipher(file, scope, start)
	}
	return false
}

// getInstanceJavaEarlierLocalDeclaresCipher reports whether a statement
// list (or a for statement's initializer) declares a local variable or
// local class named `Cipher` in a statement that ends before start.
func getInstanceJavaEarlierLocalDeclaresCipher(file *scanner.File, scope, start uint32) bool {
	for stmt := file.FlatFirstChild(scope); stmt != 0 && file.FlatEndByte(stmt) <= start; stmt = file.FlatNextSib(stmt) {
		switch file.FlatType(stmt) {
		case "local_variable_declaration":
			if getInstanceChildrenDeclareCipher(file, stmt, "variable_declarator") {
				return true
			}
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
			if getInstanceDeclarationName(file, stmt) == "Cipher" {
				return true
			}
		}
	}
	return false
}

// getInstanceJavaLambdaDeclaresCipher reads a lambda's parameters in
// each of their spellings: `Cipher -> ...`, `(Cipher) -> ...`, and
// `(KeyCipher Cipher) -> ...`. It stops at the arrow, so an identifier
// that is the lambda's body is not read as a parameter.
func getInstanceJavaLambdaDeclaresCipher(file *scanner.File, lambda uint32) bool {
	for child := file.FlatFirstChild(lambda); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "->":
			return false
		case "identifier":
			if file.FlatNodeTextEquals(child, "Cipher") {
				return true
			}
		case "inferred_parameters":
			if getInstanceJavaIdentifierBeforeTokenIsCipher(file, child, "") {
				return true
			}
		case "formal_parameters":
			if getInstanceJavaParametersDeclareCipher(file, child) {
				return true
			}
		}
	}
	return false
}

// getInstanceJavaIdentifierBeforeTokenIsCipher reports whether owner has
// a direct identifier child spelled `Cipher` before its first stop token;
// an empty stop reads every direct child.
func getInstanceJavaIdentifierBeforeTokenIsCipher(file *scanner.File, owner uint32, stop string) bool {
	for child := file.FlatFirstChild(owner); child != 0; child = file.FlatNextSib(child) {
		childType := file.FlatType(child)
		if stop != "" && childType == stop {
			return false
		}
		if childType == "identifier" && file.FlatNodeTextEquals(child, "Cipher") {
			return true
		}
	}
	return false
}

// getInstanceJavaParametersDeclareCipher reads each parameter's name, the
// identifier that follows its type.
func getInstanceJavaParametersDeclareCipher(file *scanner.File, params uint32) bool {
	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		switch file.FlatType(param) {
		case "formal_parameter", "spread_parameter":
			if name, ok := file.FlatFindChild(param, "identifier"); ok && file.FlatNodeTextEquals(name, "Cipher") {
				return true
			}
		}
	}
	return false
}

func getInstanceJavaMembersDeclareCipher(file *scanner.File, body uint32) bool {
	for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
		switch file.FlatType(member) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration",
			"annotation_type_declaration":
			if getInstanceDeclarationName(file, member) == "Cipher" {
				return true
			}
		case "field_declaration":
			if getInstanceChildrenDeclareCipher(file, member, "variable_declarator") {
				return true
			}
		}
	}
	return false
}

// getInstanceJavaImportsOtherCipher reports whether a single-type import
// binds `Cipher` to a class other than javax.crypto.Cipher.
func getInstanceJavaImportsOtherCipher(file *scanner.File, program uint32) bool {
	for imp := file.FlatFirstChild(program); imp != 0; imp = file.FlatNextSib(imp) {
		if file.FlatType(imp) != "import_declaration" {
			continue
		}
		if _, wildcard := file.FlatFindChild(imp, "asterisk"); wildcard {
			continue
		}
		name, ok := file.FlatFindChild(imp, "scoped_identifier")
		if !ok {
			continue
		}
		path := strings.Join(strings.Fields(file.FlatNodeText(name)), "")
		if strings.HasSuffix(path, ".Cipher") && path != "javax.crypto.Cipher" {
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
		if receiver != "Cipher" || getInstanceCipherNameShadowedAt(file, call) {
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
