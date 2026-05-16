package rules

// Miscellaneous Android Lint security rules: EasterEgg, TrustedServer,
// WorldReadableFiles, WorldWriteableFiles. These rules are independent
// of the other security rule clusters and share no helpers with them.

import (
	"regexp"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// EasterEggRule detects comments containing easter egg references.
type EasterEggRule struct{ AndroidRule }

var easterEggRe = regexp.MustCompile(`(?i)\b(easter\s*egg|cheat\s*code|secret\s*(?:mode|menu|feature))\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *EasterEggRule) Confidence() float64 { return 0.75 }

func (r *EasterEggRule) check(ctx *api.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		// Only check comments
		if scanner.IsCommentLine(line) {
			if easterEggRe.MatchString(line) {
				ctx.Emit(r.Finding(file, i+1, 1,
					"Code contains easter egg / hidden feature reference. Review for security implications."))
			}
		}
	}
}

// TrustedServerRule detects trust-all certificate patterns.
type TrustedServerRule struct{ AndroidRule }

// Confidence bumps this rule to tier-1 (high). The AST dispatch inspects
// class/object declarations for X509TrustManager supertypes with empty
// override bodies, and matches a short allow-list of known trust-all
// hostname-verifier identifiers. Both paths avoid the comment/string
// false positives that the previous line scan was prone to.
func (r *TrustedServerRule) Confidence() float64 { return 0.95 }

var trustedServerInsecureIdentifiers = map[string]bool{
	"TrustAllCertificates":        true,
	"AllowAllHostnameVerifier":    true,
	"ALLOW_ALL_HOSTNAME_VERIFIER": true,
}

func (r *TrustedServerRule) check(ctx *api.Context) {
	file := ctx.File
	node := ctx.Idx
	switch file.FlatType(node) {
	case "simple_identifier", "type_identifier":
		name := file.FlatNodeText(node)
		if !trustedServerInsecureIdentifiers[name] {
			return
		}
		// Skip the declaring site: `class TrustAllCertificates` or
		// `val AllowAllHostnameVerifier = ...` are declarations, not
		// usages of a known insecure API.
		if parent, ok := file.FlatParent(node); ok {
			switch file.FlatType(parent) {
			case "class_declaration", "object_declaration", "interface_declaration",
				"variable_declaration":
				return
			}
		}
		ctx.Emit(r.Finding(file, file.FlatRow(node)+1, file.FlatCol(node)+1,
			"Trusting all certificates or hostnames is insecure. Use proper certificate validation."))
	case "class_declaration", "object_literal", "object_declaration":
		if !trustedServerDeclaresX509(file, node) {
			return
		}
		if !trustedServerHasEmptyTrustCheck(file, node) {
			return
		}
		ctx.Emit(r.Finding(file, file.FlatRow(node)+1, file.FlatCol(node)+1,
			"Trust manager overrides checkClientTrusted/checkServerTrusted with an empty body. Perform real certificate validation."))
	}
}

func trustedServerDeclaresX509(file *scanner.File, decl uint32) bool {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		userType, _ := file.FlatFindChild(child, "user_type")
		if userType == 0 {
			if ctor, ok := file.FlatFindChild(child, "constructor_invocation"); ok {
				userType, _ = file.FlatFindChild(ctor, "user_type")
			}
		}
		if userType == 0 {
			continue
		}
		ident := flatLastChildOfType(file, userType, "type_identifier")
		if ident != 0 && file.FlatNodeText(ident) == "X509TrustManager" {
			return true
		}
	}
	return false
}

func trustedServerHasEmptyTrustCheck(file *scanner.File, decl uint32) bool {
	var body uint32
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "class_body" {
			body = child
			break
		}
	}
	if body == 0 {
		return false
	}
	empty := false
	for fn := file.FlatFirstChild(body); fn != 0; fn = file.FlatNextSib(fn) {
		if file.FlatType(fn) != "function_declaration" {
			continue
		}
		name := ""
		var fnBody uint32
		for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "simple_identifier":
				if name == "" {
					name = file.FlatNodeText(child)
				}
			case "function_body":
				fnBody = child
			}
		}
		if name != "checkClientTrusted" && name != "checkServerTrusted" {
			continue
		}
		if fnBody != 0 && trustedServerFunctionBodyIsEmpty(file, fnBody) {
			empty = true
			break
		}
	}
	return empty
}

func trustedServerFunctionBodyIsEmpty(file *scanner.File, body uint32) bool {
	// function_body is either `{ stmts... }` or `= expr`. An empty block
	// has only `{` and `}` named-false children plus at most a statements
	// wrapper with no children.
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "statements":
			if file.FlatFirstChild(child) != 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// WorldReadableFilesRule detects MODE_WORLD_READABLE usage.
type WorldReadableFilesRule struct{ AndroidRule }

// Confidence bumps this rule to tier-1 (high). AST dispatch on
// simple_identifier nodes means matches inside comments or string
// literals can no longer occur — those live under line_comment /
// string_content, which tree-sitter does not treat as identifiers.
func (r *WorldReadableFilesRule) Confidence() float64 { return 0.95 }

func (r *WorldReadableFilesRule) check(ctx *api.Context) {
	if worldReadableIdentifierMatch(ctx, "MODE_WORLD_READABLE") {
		file := ctx.File
		ctx.Emit(r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
			"MODE_WORLD_READABLE is insecure. Use more restrictive file permissions."))
	}
}

// WorldWriteableFilesRule detects MODE_WORLD_WRITEABLE usage.
type WorldWriteableFilesRule struct{ AndroidRule }

// Confidence bumps this rule to tier-1 (high). AST dispatch on
// simple_identifier nodes means matches inside comments or string
// literals can no longer occur — those live under line_comment /
// string_content, which tree-sitter does not treat as identifiers.
func (r *WorldWriteableFilesRule) Confidence() float64 { return 0.95 }

func (r *WorldWriteableFilesRule) check(ctx *api.Context) {
	if worldReadableIdentifierMatch(ctx, "MODE_WORLD_WRITEABLE") ||
		worldReadableIdentifierMatch(ctx, "MODE_WORLD_WRITABLE") {
		file := ctx.File
		ctx.Emit(r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
			"MODE_WORLD_WRITEABLE is insecure. Use more restrictive file permissions."))
	}
}

func worldReadableIdentifierMatch(ctx *api.Context, want string) bool {
	if ctx.File == nil || ctx.Idx == 0 {
		return false
	}
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "simple_identifier", "identifier":
	default:
		return false
	}
	if file.FlatNodeText(ctx.Idx) != want {
		return false
	}
	// Skip declaration sites (unlikely but harmless): `val MODE_WORLD_READABLE = ...`.
	if parent, ok := file.FlatParent(ctx.Idx); ok {
		switch file.FlatType(parent) {
		case "variable_declaration", "variable_declarator":
			return false
		}
	}
	return true
}
