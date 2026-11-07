package rules

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/strutil"
)

// XMLExternalEntityRule detects XML parser factories created without obvious
// XXE-disabling hardening in the same callable scope.
type XMLExternalEntityRule struct {
	FlatDispatchBase
	BaseRule
}

// JavaObjectInputStreamRule detects direct Java serialization input streams in
// production source.
type JavaObjectInputStreamRule struct {
	FlatDispatchBase
	BaseRule
}

// JacksonDefaultTypingRule detects Jackson default typing APIs that can enable
// polymorphic deserialization gadget attacks.
type JacksonDefaultTypingRule struct {
	FlatDispatchBase
	BaseRule
}

// GsonPolymorphicFromJSONRule detects Gson deserialization into Object/Any.
type GsonPolymorphicFromJSONRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *XMLExternalEntityRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *JavaObjectInputStreamRule) Confidence() float64 { return api.ConfidenceMediumHigh }

func (r *JacksonDefaultTypingRule) Confidence() float64 { return api.ConfidenceMediumHigh }

func (r *GsonPolymorphicFromJSONRule) Confidence() float64 { return api.ConfidenceMediumHigh }

var xxeFactoryReceivers = map[string]bool{
	"DocumentBuilderFactory": true,
	"SAXParserFactory":       true,
	"XMLInputFactory":        true,
	"TransformerFactory":     true,
	"SchemaFactory":          true,
}

var xxeFactoryCreationNames = map[string]bool{"newInstance": true, "newFactory": true}

func xmlExternalEntityFactoryCall(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 || !xxeFactoryCreationNames[javaAwareCallName(file, call)] {
		return false
	}
	receiver := databaseCallReceiverName(file, call)
	if dot := strings.LastIndex(receiver, "."); dot >= 0 {
		receiver = receiver[dot+1:]
	}
	if !xxeFactoryReceivers[receiver] {
		return false
	}
	return sourceImportsOrMentions(file, receiver)
}

var xxeParseCalls = map[string]bool{
	"parse": true, "createXMLStreamReader": true, "createXMLEventReader": true,
	"createFilteredReader": true, "newSchema": true,
}

// xmlExternalEntityUsedToParse reports whether the callable that creates the factory also parses XML with it.
// Building documents or transforming a DOMSource reads no external input.
func xmlExternalEntityUsedToParse(file *scanner.File, call uint32) bool {
	scope := xxeFactoryScope(file, call)
	callStart := file.FlatStartByte(call)
	callEnd := file.FlatEndByte(call)
	found := false
	file.FlatWalkAllNodes(scope, func(n uint32) {
		// A call chained on the factory, such as newInstance().newDocumentBuilder().parse(x), starts at the factory call.
		chained := file.FlatStartByte(n) == callStart && file.FlatEndByte(n) > callEnd
		if found || (file.FlatStartByte(n) <= callStart && !chained) {
			return
		}
		switch file.FlatType(n) {
		case "call_expression", "method_invocation":
			name := javaAwareCallName(file, n)
			if xxeParseCalls[name] && !xxeParseReceiverIsTypeName(file, n) {
				found = true
			} else if name == "transform" {
				text := file.FlatNodeText(n)
				found = strings.Contains(text, "StreamSource") || strings.Contains(text, "SAXSource")
			}
		}
	})
	return found
}

// xxeFactoryScope is the callable that creates the factory, or the enclosing
// class when the factory is a property initializer, or the whole file.
func xxeFactoryScope(file *scanner.File, call uint32) uint32 {
	if scope, ok := flatEnclosingCallable(file, call); ok {
		return scope
	}
	if cls, ok := flatEnclosingAncestor(file, call, "class_declaration", "object_declaration"); ok {
		return cls
	}
	return 0
}

// xxeParseReceiverIsTypeName reports a parse call on a plain type name such as
// `Uri.parse(s)` or `LocalDate.parse(s)`, which is not an XML parser instance.
func xxeParseReceiverIsTypeName(file *scanner.File, call uint32) bool {
	receiver := strings.TrimSpace(databaseCallReceiverName(file, call))
	if receiver == "" {
		return false
	}
	for _, r := range receiver {
		if r != '.' && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	last := receiver[strings.LastIndex(receiver, ".")+1:]
	return last != "" && unicode.IsUpper(rune(last[0]))
}

// xxeFeatureHardening maps each XXE-related feature URL to the boolean
// value that disables the unsafe behavior when passed to
// `factory.setFeature(URL, BOOL)`.
var xxeFeatureHardening = map[string]bool{
	"http://apache.org/xml/features/disallow-doctype-decl":           true,
	"http://xml.org/sax/features/external-general-entities":          false,
	"http://xml.org/sax/features/external-parameter-entities":        false,
	"http://apache.org/xml/features/nonvalidating/load-external-dtd": false,
}

func xmlExternalEntityHasHardeningAfter(file *scanner.File, call uint32) bool {
	scope := xxeFactoryScope(file, call)
	callStart := file.FlatStartByte(call)
	factory := xxeFactoryName(file, call)
	found := false
	sawXInclude := false
	sawExpandEntities := false

	file.FlatWalkAllNodes(scope, func(n uint32) {
		if found || file.FlatStartByte(n) <= callStart {
			return
		}
		switch file.FlatType(n) {
		case "call_expression", "method_invocation":
			if xxeCallIsHardening(file, n, factory) {
				found = true
			}
		case "assignment":
			switch xxePropertyAssignmentName(file, n) {
			case "isXIncludeAware":
				sawXInclude = true
			case "isExpandEntityReferences":
				sawExpandEntities = true
			}
			if sawXInclude && sawExpandEntities {
				found = true
			}
		}
	})
	return found
}

// xxeCallIsHardening reports whether the call_expression / method_invocation
// is one of the XXE-disabling setFeature / setProperty invocations with the
// expected literal arguments. Match is AST-based so formatting, whitespace,
// or interleaved comments do not break detection.
func xxeCallIsHardening(file *scanner.File, call uint32, factory string) bool {
	name := javaAwareCallName(file, call)
	if name != "setFeature" && name != "setProperty" && name != "setAttribute" {
		return false
	}
	keyExpr, valExpr := xxeCallTwoArgExprs(file, call)
	if keyExpr == 0 || valExpr == 0 {
		return false
	}
	key := xxeConfigKey(file, keyExpr)
	if key == "" {
		return false
	}
	switch {
	case name == "setFeature" && key == "FEATURE_SECURE_PROCESSING":
		// Secure processing is the documented hardening for these two factories only.
		got, ok := xxeLiteralBool(file, valExpr)
		return ok && got && (factory == "TransformerFactory" || factory == "SchemaFactory")
	case name == "setFeature":
		expected, known := xxeFeatureHardening[key]
		if !known {
			return false
		}
		got, ok := xxeLiteralBool(file, valExpr)
		return ok && got == expected
	case xxeAccessExternalKeys[key]:
		// ACCESS_EXTERNAL_DTD, _SCHEMA and _STYLESHEET are cleared with an empty string.
		value, ok := xxeLiteralStringContent(file, valExpr)
		if !ok {
			text := strings.TrimSpace(file.FlatNodeText(valExpr))
			return text == `""`
		}
		return value == ""
	case xxeXMLInputFactoryKeys[key]:
		got, ok := xxeLiteralBool(file, valExpr)
		return ok && !got
	}
	return false
}

// xxeAccessExternalKeys are the XMLConstants properties that restrict external access.
var xxeAccessExternalKeys = map[string]bool{
	"ACCESS_EXTERNAL_DTD":                                             true,
	"ACCESS_EXTERNAL_SCHEMA":                                          true,
	"ACCESS_EXTERNAL_STYLESHEET":                                      true,
	"http://javax.xml.XMLConstants/property/accessExternalDTD":        true,
	"http://javax.xml.XMLConstants/property/accessExternalSchema":     true,
	"http://javax.xml.XMLConstants/property/accessExternalStylesheet": true,
}

// xxeXMLInputFactoryKeys are the StAX properties that disable DTDs or external entities.
var xxeXMLInputFactoryKeys = map[string]bool{
	"SUPPORT_DTD":                                   true,
	"IS_SUPPORTING_EXTERNAL_ENTITIES":               true,
	"javax.xml.stream.supportDTD":                   true,
	"javax.xml.stream.isSupportingExternalEntities": true,
}

// xxeConfigKey returns a string literal's content, or the trailing identifier
// of a constant reference such as XMLConstants.ACCESS_EXTERNAL_DTD.
func xxeConfigKey(file *scanner.File, expr uint32) string {
	if value, ok := xxeLiteralStringContent(file, expr); ok {
		if strings.HasPrefix(value, "http://javax.xml.XMLConstants/feature/secure-processing") {
			return "FEATURE_SECURE_PROCESSING"
		}
		return value
	}
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		return strings.TrimSpace(file.FlatNodeText(expr))
	case "navigation_expression":
		segs := flatNavigationChainIdentifiers(file, expr)
		if len(segs) > 0 {
			return segs[len(segs)-1]
		}
	case "field_access":
		if count := file.FlatNamedChildCount(expr); count > 0 {
			return strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(expr, count-1)))
		}
	}
	return ""
}

// xxeFactoryName returns the simple factory type name of a factory creation call.
func xxeFactoryName(file *scanner.File, call uint32) string {
	receiver := databaseCallReceiverName(file, call)
	if dot := strings.LastIndex(receiver, "."); dot >= 0 {
		receiver = receiver[dot+1:]
	}
	return receiver
}

// xxeCallTwoArgExprs returns the first two positional argument expressions
// of a two-arg call, regardless of whether it parsed as a Kotlin
// call_expression (value_arguments) or Java method_invocation
// (argument_list).
func xxeCallTwoArgExprs(file *scanner.File, call uint32) (uint32, uint32) {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 {
			return 0, 0
		}
		a0 := flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
		a1 := flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 1))
		return a0, a1
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) < 2 {
			return 0, 0
		}
		return file.FlatNamedChild(args, 0), file.FlatNamedChild(args, 1)
	}
	return 0, 0
}

// xxeLiteralStringContent returns the unquoted content of a string literal
// expression with no interpolation, or "", false.
func xxeLiteralStringContent(file *scanner.File, expr uint32) (string, bool) {
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		if flatContainsStringInterpolation(file, expr) {
			return "", false
		}
		if value := stringLiteralContent(file, expr); value != "" {
			return value, true
		}
		text := strings.TrimSpace(file.FlatNodeText(expr))
		if unq, err := strconv.Unquote(text); err == nil {
			return unq, true
		}
	}
	return "", false
}

// xxeLiteralBool returns the boolean value of a `true` / `false` literal,
// or false, false for anything else.
func xxeLiteralBool(file *scanner.File, expr uint32) (bool, bool) {
	expr = flatUnwrapParenExpr(file, expr)
	switch strings.TrimSpace(file.FlatNodeText(expr)) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// xxePropertyAssignmentName returns the trailing property name of a Kotlin
// property assignment whose right-hand side is the literal `false`, or "".
// Used to detect the `factory.isXIncludeAware = false` /
// `factory.isExpandEntityReferences = false` hardening pair.
func xxePropertyAssignmentName(file *scanner.File, assignment uint32) string {
	_, tail := chainSplitTrailing(kotlinAssignmentTargetChain(file, assignment))
	if tail == "" {
		return ""
	}
	value := findKotlinAssignmentValue(file, assignment)
	if value == 0 {
		return ""
	}
	if got, ok := xxeLiteralBool(file, value); !ok || got {
		return ""
	}
	return tail
}

func javaObjectInputStreamConstructor(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || scanner.IsTestFile(file.Path) {
		return false
	}
	if !sourceImportsOrMentions(file, "java.io.ObjectInputStream") {
		return false
	}
	if javaObjectInputStreamDuplicateNestedCall(file, idx) {
		return false
	}
	if javaObjectInputStreamSafeSubclassScope(file, idx) {
		return false
	}
	return javaObjectInputStreamCallShape(file, idx)
}

func javaObjectInputStreamCallShape(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "call_expression":
		// Kotlin: the callee is the first named child of the call_expression —
		// either a bare simple_identifier or a navigation_expression. Inspect
		// it directly so string-literal arguments that mention the class
		// cannot satisfy the shape check.
		navExpr, _ := flatCallExpressionParts(file, idx)
		if navExpr != 0 {
			chain := flatNavigationChainIdentifiers(file, navExpr)
			if len(chain) == 3 && chain[0] == "java" && chain[1] == "io" && chain[2] == "ObjectInputStream" {
				return true
			}
			return false
		}
		return flatCallExpressionName(file, idx) == "ObjectInputStream"
	case "object_creation_expression":
		// Java: `new X(args)` parses with the type as a named child —
		// type_identifier for `ObjectInputStream`, scoped_type_identifier
		// whose tail identifier is `ObjectInputStream` for the FQN form.
		for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
			if !file.FlatIsNamed(c) {
				continue
			}
			switch file.FlatType(c) {
			case "type_identifier":
				return file.FlatNodeTextEquals(c, "ObjectInputStream")
			case "scoped_type_identifier":
				return file.FlatNodeTextEquals(c, "java.io.ObjectInputStream")
			default:
				return false
			}
		}
		return false
	default:
		return false
	}
}

func javaObjectInputStreamDuplicateNestedCall(file *scanner.File, idx uint32) bool {
	if file.FlatType(idx) != "call_expression" {
		return false
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "call_expression":
			return file.FlatRow(parent) == file.FlatRow(idx) &&
				file.FlatCol(parent) == file.FlatCol(idx) &&
				javaObjectInputStreamCallShape(file, parent)
		case "function_declaration", "method_declaration", "class_declaration", "source_file":
			return false
		}
	}
	return false
}

func javaObjectInputStreamSafeSubclassScope(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration":
			text := file.FlatNodeText(cur)
			if strings.Contains(text, "ObjectInputStream") && strings.Contains(text, "resolveClass") {
				return true
			}
			return false
		case "source_file":
			return false
		}
	}
	return false
}

func jacksonDefaultTypingCall(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	name := javaAwareCallName(file, idx)
	if name != "enableDefaultTyping" && name != "activateDefaultTyping" {
		return false
	}
	if !jacksonFileImportsDatabind(file) {
		return false
	}
	// activateDefaultTyping(validator) is the supported API; only a validator that allows every type is unsafe.
	if name == "activateDefaultTyping" && !jacksonValidatorAllowsAnyType(okHTTPCallArgumentsText(file, idx)) {
		return false
	}
	return jacksonDefaultTypingReceiverLooksMapper(file, idx)
}

// jacksonValidatorAllowsAnyType reports a PolymorphicTypeValidator argument
// that trusts every type: LaissezFaireSubTypeValidator or an Object base type.
func jacksonValidatorAllowsAnyType(args string) bool {
	compact := strings.Join(strings.Fields(args), "")
	return strings.Contains(compact, "LaissezFaire") ||
		strings.Contains(compact, "allowIfBaseType(Object.class") ||
		strings.Contains(compact, "allowIfSubType(Object.class") ||
		strings.Contains(compact, "allowIfBaseType(Any::class") ||
		strings.Contains(compact, "allowIfSubType(Any::class") ||
		strings.Contains(compact, "allowIfBaseType(\"\")") ||
		strings.Contains(compact, "allowIfSubType(\"\")")
}

func jacksonFileImportsDatabind(file *scanner.File) bool {
	if file == nil {
		return false
	}
	text := string(file.Content)
	return strings.Contains(text, "com.fasterxml.jackson.databind") ||
		strings.Contains(text, "com.fasterxml.jackson.dataformat.xml.XmlMapper")
}

func jacksonDefaultTypingReceiverLooksMapper(file *scanner.File, idx uint32) bool {
	text := strings.Join(strings.Fields(file.FlatNodeText(idx)), "")
	if strings.Contains(text, "ObjectMapper().") ||
		strings.Contains(text, "XmlMapper().") ||
		strings.Contains(text, "JsonMapper.builder()") ||
		strings.Contains(text, "newObjectMapper().") ||
		strings.Contains(text, "newXmlMapper().") ||
		strings.Contains(text, "newJsonMapper") ||
		strings.Contains(text, "com.fasterxml.jackson.databind.ObjectMapper(") {
		return true
	}
	receiver := databaseCallReceiverName(file, idx)
	if receiver == "" {
		return false
	}
	if strings.Contains(receiver, "ObjectMapper") || strings.Contains(receiver, "XmlMapper") || strings.Contains(receiver, "JsonMapper") {
		return true
	}
	return receiverDeclaredFromCall(file, idx, "ObjectMapper") ||
		receiverDeclaredFromCall(file, idx, "XmlMapper") ||
		receiverDeclaredFromCall(file, idx, "JsonMapper") ||
		jacksonReceiverDeclaredAsMapper(file, idx, receiver) ||
		receiverNameDeclaredWithType(file, receiver, "ObjectMapper", "XmlMapper", "JsonMapper")
}

func jacksonReceiverDeclaredAsMapper(file *scanner.File, call uint32, receiver string) bool {
	scope, ok := flatEnclosingCallable(file, call)
	if !ok {
		return false
	}
	found := false
	file.FlatWalkAllNodes(scope, func(n uint32) {
		if found || file.FlatStartByte(n) >= file.FlatStartByte(call) {
			return
		}
		typ := file.FlatType(n)
		if typ != "property_declaration" && typ != "variable_declaration" && typ != "local_variable_declaration" {
			return
		}
		text := file.FlatNodeText(n)
		if !strings.Contains(text, receiver) {
			return
		}
		compact := strings.Join(strings.Fields(text), "")
		if strings.Contains(compact, "ObjectMapper") || strings.Contains(compact, "XmlMapper") || strings.Contains(compact, "JsonMapper") {
			found = true
		}
	})
	return found
}

func gsonPolymorphicFromJSONCall(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || javaAwareCallName(file, idx) != "fromJson" {
		return false
	}
	if !gsonFileImportsGson(file) || !gsonFromJSONReceiverLooksGson(file, idx) {
		return false
	}
	arg, ok := gsonFromJSONTypeArg(file, idx)
	return ok && gsonFromJSONPolymorphicTypeArg(file.FlatNodeText(arg))
}

func gsonFileImportsGson(file *scanner.File) bool {
	if file == nil {
		return false
	}
	text := string(file.Content)
	return strings.Contains(text, "com.google.gson.Gson") ||
		strings.Contains(text, "com.google.gson.GsonBuilder") ||
		strings.Contains(text, "com.google.gson.*")
}

func gsonFromJSONReceiverLooksGson(file *scanner.File, idx uint32) bool {
	text := strings.Join(strings.Fields(file.FlatNodeText(idx)), "")
	if strings.Contains(text, "com.google.gson.Gson(") ||
		strings.Contains(text, "Gson().fromJson(") ||
		strings.Contains(text, "newGson().fromJson(") ||
		strings.Contains(text, "com.google.gson.GsonBuilder(") ||
		strings.Contains(text, "GsonBuilder().create().fromJson(") ||
		strings.Contains(text, "newGsonBuilder().create().fromJson(") {
		return true
	}
	receiver := databaseCallReceiverName(file, idx)
	if receiver == "" {
		return false
	}
	if strings.Contains(receiver, "Gson") {
		return true
	}
	if staticIvFileDeclaresType(file, "Gson") || staticIvFileDeclaresType(file, "GsonBuilder") {
		return false
	}
	return receiverDeclaredFromCall(file, idx, "Gson") ||
		receiverDeclaredFromCall(file, idx, "GsonBuilder") ||
		gsonReceiverDeclaredAsGson(file, idx, receiver) ||
		receiverNameDeclaredWithType(file, receiver, "Gson")
}

// receiverNameDeclaredWithType reports whether the file declares a parameter,
// property, or local named name whose declaration text mentions one of typeNames
// as a whole word, such as `mapper: ObjectMapper` or `Gson gson`.
func receiverNameDeclaredWithType(file *scanner.File, name string, typeNames ...string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "parameter", "class_parameter", "property_declaration", "variable_declaration",
			"formal_parameter", "field_declaration", "local_variable_declaration", "spread_parameter":
		default:
			return
		}
		if extractIdentifierFlat(file, n) != name && !declaresNameInChildren(file, n, name) {
			return
		}
		text := file.FlatNodeText(n)
		for _, typeName := range typeNames {
			if strutil.ContainsTokenWordBoundary(text, typeName) {
				found = true
				return
			}
		}
	})
	return found
}

// declaresNameInChildren reports a Java declaration whose declarator or
// identifier child is spelled name.
func declaresNameInChildren(file *scanner.File, n uint32, name string) bool {
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "identifier":
			if file.FlatNodeTextEquals(c, name) {
				return true
			}
		case "variable_declarator":
			if id, ok := file.FlatFindChild(c, "identifier"); ok && file.FlatNodeTextEquals(id, name) {
				return true
			}
		}
	}
	return false
}

func gsonReceiverDeclaredAsGson(file *scanner.File, call uint32, receiver string) bool {
	scope, ok := flatEnclosingCallable(file, call)
	if !ok {
		return false
	}
	found := false
	file.FlatWalkAllNodes(scope, func(n uint32) {
		if found || file.FlatStartByte(n) >= file.FlatStartByte(call) {
			return
		}
		typ := file.FlatType(n)
		if typ != "property_declaration" && typ != "variable_declaration" && typ != "local_variable_declaration" {
			return
		}
		text := file.FlatNodeText(n)
		if !strings.Contains(text, receiver) {
			return
		}
		compact := strings.Join(strings.Fields(text), "")
		if strings.Contains(compact, "Gson()") || strings.Contains(compact, "newGson()") ||
			strings.Contains(compact, "GsonBuilder().create()") || strings.Contains(compact, "newGsonBuilder().create()") ||
			strings.Contains(compact, "Gson"+receiver+"=") || strings.Contains(compact, "Gson"+receiver+";") {
			found = true
		}
	})
	return found
}

func gsonFromJSONTypeArg(file *scanner.File, idx uint32) (uint32, bool) {
	switch file.FlatType(idx) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, idx)
		arg := flatPositionalValueArgument(file, args, 1)
		if arg == 0 {
			return 0, false
		}
		expr := flatValueArgumentExpression(file, arg)
		return expr, expr != 0
	case "method_invocation":
		args, ok := file.FlatFindChild(idx, "argument_list")
		if !ok || file.FlatNamedChildCount(args) < 2 {
			return 0, false
		}
		return file.FlatNamedChild(args, 1), true
	default:
		return 0, false
	}
}

func gsonFromJSONPolymorphicTypeArg(text string) bool {
	compact := strings.Join(strings.Fields(text), "")
	switch compact {
	case "Any::class.java", "kotlin.Any::class.java", "Object::class.java", "java.lang.Object::class.java",
		"Any::class", "kotlin.Any::class", "Object::class", "java.lang.Object::class",
		"Object.class", "java.lang.Object.class":
		return true
	default:
		return false
	}
}

var xmlExternalEntityExternalInputPattern = regexp.MustCompile(`\b(URL|HttpURLConnection|HttpClient|OkHttpClient|openConnection|openStream|Socket|curl)\b`)

// xmlExternalEntityScriptFetchesExternalInput reports a script that reads from the network, so its parsed XML
// may not be repository-local.
func xmlExternalEntityScriptFetchesExternalInput(file *scanner.File) bool {
	return xmlExternalEntityExternalInputPattern.Match(file.Content)
}
