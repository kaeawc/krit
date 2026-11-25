package rules

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/kaeawc/krit/internal/lazyre"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// BidiSpoofingRule mirrors Lint's BidiSpoofing (BidirectionalTextDetector): a comment or string literal whose source
// text opens a Unicode bidirectional embedding, override or isolate that is not closed before the end of its line.
type BidiSpoofingRule struct {
	FlatDispatchBase
	AndroidRule
}

// AuthLeakRule mirrors Lint's AuthLeak (StringAuthLeakDetector): a string literal holding a URL with a user name and
// password.
type AuthLeakRule struct {
	FlatDispatchBase
	AndroidRule
}

// SecretInSourceRule mirrors Lint's SecretInSource (SecretDetector): a literal Gemini API key passed to the
// com.google.ai.client.generativeai.GenerativeModel constructor.
type SecretInSourceRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-3 (high) base confidence: the check reads only the literal's own text.
func (r *BidiSpoofingRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a tier-3 (high) base confidence: the check reads only the literal's own text.
func (r *AuthLeakRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a tier-2 (medium) base confidence: the constructor is matched through imports.
func (r *SecretInSourceRule) Confidence() float64 { return api.ConfidenceMedium }

// lintSecurityTextNodeTypes are the Kotlin and Java comment and string literal nodes the text checks visit.
var lintSecurityTextNodeTypes = []string{"string_literal", "line_comment", "multiline_comment", "block_comment"}

// lintLiteralPart is one literal entry of a string: what Lint's UAST sees as a ULiteralExpression. A Kotlin string
// template splits into one entry per run of text between interpolations and escape sequences, and a raw string also
// splits at each line break. A Java literal is a single entry spanning the quotes.
type lintLiteralPart struct {
	// start is the byte offset of the entry's source text.
	start int
	// source is the entry's source text; value is what the literal evaluates to.
	source, value string
}

// lintLiteralParts returns the literal entries of the string literal at idx, in source order.
func lintLiteralParts(file *scanner.File, idx uint32) []lintLiteralPart {
	if file.Language == scanner.LangJava {
		value, ok := javaStringLiteralValue(file, idx)
		if !ok {
			return nil
		}
		return []lintLiteralPart{{start: int(file.FlatStartByte(idx)), source: file.FlatNodeText(idx), value: value}}
	}
	raw := stringLiteralIsRaw(file, idx)
	var parts []lintLiteralPart
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "string_content" {
			continue
		}
		text := file.FlatNodeText(child)
		base := int(file.FlatStartByte(child))
		begin := 0
		flush := func(end int) {
			if end > begin {
				parts = append(parts, lintLiteralPart{start: base + begin, source: text[begin:end], value: text[begin:end]})
			}
		}
		for i := 0; i < len(text); {
			switch {
			case raw && text[i] == '\n':
				flush(i)
				i++
				begin = i
			case !raw && text[i] == '\\' && i+1 < len(text):
				flush(i)
				n := 2
				if text[i+1] == 'u' && i+6 <= len(text) {
					n = 6
				}
				i += n
				begin = i
			default:
				i++
			}
		}
		flush(len(text))
	}
	return parts
}

// javaStringLiteralValue evaluates a Java string literal or text block, decoding escape sequences. It reports false
// when the literal holds an escape it does not understand.
func javaStringLiteralValue(file *scanner.File, idx uint32) (string, bool) {
	text := file.FlatNodeText(idx)
	if strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`) && len(text) >= 6 {
		return javaUnescape(javaTextBlockStrip(text[3 : len(text)-3]))
	}
	if len(text) < 2 || text[0] != '"' || text[len(text)-1] != '"' {
		return "", false
	}
	return javaUnescape(text[1 : len(text)-1])
}

// javaTextBlockStrip applies the text block indentation rules to the text between the delimiters: the opening line
// break is dropped, the common leading whitespace of the content lines and the closing delimiter line is removed,
// and trailing spaces are stripped.
func javaTextBlockStrip(body string) string {
	_, body, _ = strings.Cut(body, "\n")
	lines := strings.Split(body, "\n")
	indent := -1
	for i, line := range lines {
		last := i == len(lines)-1
		if strings.TrimSpace(line) == "" && !last {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	for i, line := range lines {
		if len(line) >= indent && indent > 0 {
			line = line[indent:]
		} else {
			line = strings.TrimLeft(line, " \t")
		}
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

// javaUnescape decodes Java string escapes.
func javaUnescape(s string) (string, bool) {
	if !strings.Contains(s, `\`) {
		return s, true
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			sb.WriteByte(s[i])
			continue
		}
		i++
		switch c := s[i]; c {
		case 'n':
			sb.WriteByte('\n')
		case 't':
			sb.WriteByte('\t')
		case 'r':
			sb.WriteByte('\r')
		case 'b':
			sb.WriteByte('\b')
		case 'f':
			sb.WriteByte('\f')
		case 's':
			sb.WriteByte(' ')
		case '\n':
			// A line continuation in a text block.
		case '"', '\'', '\\':
			sb.WriteByte(c)
		case 'u':
			for i < len(s) && s[i] == 'u' {
				i++
			}
			if i+4 > len(s) {
				return "", false
			}
			v, err := strconv.ParseUint(s[i:i+4], 16, 32)
			if err != nil {
				return "", false
			}
			sb.WriteRune(rune(v))
			i += 3
		default:
			if c < '0' || c > '7' {
				return "", false
			}
			end := i + 1
			for end < len(s) && end < i+3 && s[end] >= '0' && s[end] <= '7' {
				end++
			}
			v, _ := strconv.ParseUint(s[i:end], 8, 32)
			sb.WriteRune(rune(v))
			i = end - 1
		}
	}
	return sb.String(), true
}

// lintIsComment reports whether the node is a comment Lint's UFile.allCommentsInFile returns. KDoc and Javadoc are
// not among them.
func lintIsComment(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "line_comment":
		return true
	case "multiline_comment", "block_comment":
		text := file.FlatNodeText(idx)
		return !strings.HasPrefix(text, "/**") || text == "/**/"
	}
	return false
}

// bidiUnterminated ports BidirectionalTextDetector.containsUnterminatedBidiSegment: embeddings (LRE, RLE, LRO, RLO)
// and isolates (LRI, RLI, FSI) that the matching PDF or PDI does not close before a line break.
func bidiUnterminated(s string) bool {
	embedding, isolate := 0, 0
	for _, r := range s {
		switch r {
		case '\n', '\v', '\f', '\r', 0x85, 0x2028, 0x2029:
			embedding, isolate = 0, 0
		case 0x202A, 0x202B, 0x202D, 0x202E:
			embedding++
		case 0x202C:
			if embedding > 0 {
				embedding--
			}
		case 0x2066, 0x2067, 0x2068:
			isolate++
		case 0x2069:
			if isolate > 0 {
				isolate--
			}
		}
	}
	return embedding+isolate > 0
}

func (r *BidiSpoofingRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if lintIsComment(file, idx) {
		if bidiUnterminated(file.FlatNodeText(idx)) {
			ctx.Emit(lintFindingAtNode(ctx, idx, "", "Comment contains misleading Unicode bidirectional text"))
		}
		return
	}
	if file.FlatType(idx) != "string_literal" {
		return
	}
	for _, part := range lintLiteralParts(file, idx) {
		if bidiUnterminated(part.source) {
			ctx.Emit(lintSecurityFindingInRange(file, part.start, part.start+len(part.source),
				"String contains misleading Unicode bidirectional text"))
		}
	}
}

// authLeakRe is StringAuthLeakDetector.AUTH_REGEXP: scheme://user:password@host.
var authLeakRe = lazyre.New(`([\w+.-]+)://([\w_.!~*'()%;&=+$,-]+):([\w_.!~*'()%;&=+$,-]+)@([\w_.!~*'()%;&=+$,-]+)`)

// lintFormatSpecRe is StringFormatDetector.FORMAT, anchored because Lint calls matches().
var lintFormatSpecRe = lazyre.New(`^%(\d+\$)?([-+#, 0(<]*)?(\d+)?(\.\d+)?([tT])?([a-zA-Z%])$`)

func (r *AuthLeakRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if file.FlatType(idx) != "string_literal" {
		return
	}
	for _, part := range lintLiteralParts(file, idx) {
		if len(utf16.Encode([]rune(part.value))) > 512 {
			continue
		}
		m := authLeakRe.FindStringSubmatchIndex(part.value)
		if m == nil {
			continue
		}
		password := part.value[m[6]:m[7]]
		if strings.HasPrefix(password, "%") && strings.HasSuffix(password, "s") && lintFormatSpecRe.MatchString(password) {
			continue
		}
		// Lint offsets the range by one from the start of the literal element, to step over a Java opening quote; a
		// Kotlin entry has no quote, so the range starts one character into the URL.
		start := part.start + m[0] + 1
		end := min(start+m[1]-m[0], len(file.Content))
		ctx.Emit(lintSecurityFindingInRange(file, start, end, "Possible credential leak"))
	}
}

// lintSecurityFindingInRange builds a finding for the byte range [start,end) of the file.
func lintSecurityFindingInRange(file *scanner.File, start, end int, msg string) scanner.Finding {
	start = min(max(start, 0), len(file.Content))
	before := file.Content[:start]
	line := 1 + bytes.Count(before, []byte{'\n'})
	col := start - (bytes.LastIndexByte(before, '\n') + 1) + 1
	return scanner.Finding{Line: line, Col: col, StartByte: start, EndByte: end, Message: msg}
}

const generativeModelFQN = "com.google.ai.client.generativeai.GenerativeModel"

func (r *SecretInSourceRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "GenerativeModel" {
		return
	}
	switch file.FlatType(ctx.Idx) {
	case "object_creation_expression":
	case "call_expression":
		if call.ReceiverNode != 0 && call.TypeText != generativeModelFQN {
			return
		}
	default:
		return
	}
	if !lintTypeRefMatches(file, call.TypeText, generativeModelFQN) {
		return
	}
	// A class of the same name declared in the file can shadow the import inside its scope.
	if call.TypeText != generativeModelFQN && lintSameFileClassOrObject(file, "GenerativeModel") != 0 {
		return
	}
	arg := lintSkipParens(file, call.Arg(1, "apiKey"))
	if arg == 0 || !secretKeyExpression(file, arg) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, arg, "",
		"This argument looks like an API key that has come from source code; API keys should not be included in source code"))
}

// lintSkipParens unwraps parenthesized expressions.
func lintSkipParens(file *scanner.File, idx uint32) uint32 {
	for idx != 0 && file.FlatType(idx) == "parenthesized_expression" {
		inner := uint32(0)
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				inner = child
				break
			}
		}
		idx = inner
	}
	return idx
}

// secretKeyExpression reports whether expr is a key literal, or a reference to a local variable or a same-file field
// whose initializer is one. Lint reads the initializer even when the variable is reassigned later.
func secretKeyExpression(file *scanner.File, expr uint32) bool {
	if secretKeyLiteral(file, expr) {
		return true
	}
	decl := secretKeyReferencedDeclaration(file, expr)
	if decl == 0 {
		return false
	}
	init := lintSkipParens(file, lintInitializerAfterEquals(file, decl))
	if init == 0 || !secretKeyLiteral(file, init) {
		return false
	}
	// Lint skips a field whose declaration suppresses the issue.
	if _, inFn := flatEnclosingCallable(file, decl); !inFn && file.SuppressionIdx != nil &&
		file.SuppressionIdx.IsSuppressed(int(file.FlatStartByte(decl)), "SecretInSource", "android-lint") {
		return false
	}
	return true
}

// secretKeyLiteral is SecretDetector.isLiteralStringKey: a string literal of one entry, longer than 30 characters,
// that starts with AIza.
func secretKeyLiteral(file *scanner.File, expr uint32) bool {
	if file.FlatType(expr) != "string_literal" {
		return false
	}
	var value string
	if file.Language == scanner.LangJava {
		v, ok := javaStringLiteralValue(file, expr)
		if !ok {
			return false
		}
		value = v
	} else {
		parts := lintLiteralParts(file, expr)
		if len(parts) != 1 || flatContainsStringInterpolation(file, expr) {
			return false
		}
		// An escape sequence is its own entry, so the one part must cover the whole literal body.
		body := file.FlatNodeText(expr)
		body = body[lintLiteralContentOffset(file, expr) : len(body)-lintLiteralContentOffset(file, expr)]
		if parts[0].source != body {
			return false
		}
		value = parts[0].value
	}
	return strings.HasPrefix(value, "AIza") && len(utf16.Encode([]rune(value))) > 30
}

// secretKeyReferencedDeclaration resolves a simple name, or a member reference on a same-file class or object, to
// the local variable or same-file field it names. It returns the declaration node, or 0 when the name does not
// resolve in the file.
func secretKeyReferencedDeclaration(file *scanner.File, expr uint32) uint32 {
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(expr)
		if local := lintLocalDeclarationBefore(file, expr, name); local != 0 {
			return local
		}
		if lintNameIsParameterAt(file, expr, name) {
			return 0
		}
		for cur, ok := file.FlatParent(expr); ok; cur, ok = file.FlatParent(cur) {
			switch file.FlatType(cur) {
			case "class_declaration", "object_declaration", "companion_object", "interface_declaration", "enum_declaration":
				if decl := lintMemberField(file, cur, name); decl != 0 {
					return decl
				}
			}
		}
		return lintMemberField(file, 0, name)
	case "navigation_expression", "field_access":
		var named []uint32
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				named = append(named, child)
			}
		}
		if len(named) != 2 {
			return 0
		}
		owner := strings.TrimSpace(file.FlatNodeText(named[0]))
		name := strings.TrimSpace(file.FlatNodeText(named[1]))
		if file.FlatType(named[1]) == "navigation_suffix" {
			name = strings.TrimPrefix(name, ".")
		}
		if owner == "this" {
			for cur, ok := file.FlatParent(expr); ok; cur, ok = file.FlatParent(cur) {
				if t := file.FlatType(cur); t == "class_declaration" || t == "object_declaration" {
					return lintMemberField(file, cur, name)
				}
			}
			return 0
		}
		if strings.Contains(owner, ".") {
			return 0
		}
		if cls := lintSameFileClassOrObject(file, owner); cls != 0 {
			return lintMemberField(file, cls, name)
		}
	}
	return 0
}

// lintSameFileClassOrObject returns the class, interface or object declared in the file with the simple name.
func lintSameFileClassOrObject(file *scanner.File, name string) uint32 {
	if cls := lintSameFileClass(file, name); cls != 0 {
		return cls
	}
	var found uint32
	file.FlatWalkNodes(0, "object_declaration", func(decl uint32) {
		if id, ok := file.FlatFindChild(decl, "type_identifier"); ok && found == 0 && file.FlatNodeText(id) == name {
			found = decl
		}
	})
	return found
}

// lintLocalDeclarationBefore returns the local variable named name that is declared before use in a block enclosing
// it, within the same function, or 0.
func lintLocalDeclarationBefore(file *scanner.File, use uint32, name string) uint32 {
	fn, ok := flatEnclosingCallable(file, use)
	if !ok {
		return 0
	}
	useStart := file.FlatStartByte(use)
	var best uint32
	visit := func(decl, nameNode uint32) {
		if nameNode == 0 || file.FlatNodeText(nameNode) != name || file.FlatStartByte(decl) >= useStart {
			return
		}
		scope, ok := file.FlatParent(decl)
		if file.FlatType(scope) == "local_variable_declaration" {
			scope, ok = file.FlatParent(scope)
		}
		if !ok || file.FlatEndByte(scope) < file.FlatEndByte(use) || file.FlatStartByte(scope) > useStart {
			return
		}
		if best == 0 || file.FlatStartByte(decl) > file.FlatStartByte(best) {
			best = decl
		}
	}
	file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
		vd, _ := file.FlatFindChild(decl, "variable_declaration")
		if vd != 0 && lintDeclaredName(file, vd) == name {
			visit(decl, vd)
		}
	})
	file.FlatWalkNodes(fn, "variable_declarator", func(decl uint32) {
		parent, _ := file.FlatParent(decl)
		if file.FlatType(parent) != "local_variable_declaration" {
			return
		}
		id, _ := file.FlatFindChild(decl, "identifier")
		visit(decl, id)
	})
	return best
}

// lintMemberField returns the property or field named name declared directly in the body of owner (a class, object,
// or companion object; 0 means the top level of the file), or 0.
func lintMemberField(file *scanner.File, owner uint32, name string) uint32 {
	var found uint32
	check := func(decl uint32, declName string, container uint32) {
		if found != 0 || declName != name {
			return
		}
		if _, inFn := flatEnclosingCallable(file, decl); inFn {
			return
		}
		if lintDeclarationOwner(file, container) != owner {
			return
		}
		found = decl
	}
	file.FlatWalkNodes(owner, "property_declaration", func(decl uint32) {
		vd, _ := file.FlatFindChild(decl, "variable_declaration")
		if vd != 0 {
			check(decl, lintDeclaredName(file, vd), decl)
		}
	})
	file.FlatWalkNodes(owner, "variable_declarator", func(decl uint32) {
		parent, _ := file.FlatParent(decl)
		if file.FlatType(parent) != "field_declaration" && file.FlatType(parent) != "constant_declaration" {
			return
		}
		id, _ := file.FlatFindChild(decl, "identifier")
		if id != 0 {
			check(decl, file.FlatNodeText(id), parent)
		}
	})
	if found == 0 && owner != 0 && file.FlatType(owner) == "class_declaration" {
		// A companion object's members are reachable through the class name and from inside the class.
		file.FlatWalkNodes(owner, "companion_object", func(companion uint32) {
			if found == 0 && lintDeclarationOwner(file, companion) == owner {
				found = lintMemberField(file, companion, name)
			}
		})
	}
	return found
}

// lintDeclarationOwner returns the nearest enclosing class, interface, object or companion object of decl, or 0 for a
// top-level declaration. A companion object's members count as members of the companion.
func lintDeclarationOwner(file *scanner.File, decl uint32) uint32 {
	for cur, ok := file.FlatParent(decl); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "companion_object", "interface_declaration", "enum_declaration":
			return cur
		}
	}
	return 0
}
