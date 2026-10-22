package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/sourceheader"
)

// WildcardImportRule detects import x.y.* statements.
type WildcardImportRule struct {
	FlatDispatchBase
	BaseRule
	ExcludeImports []string // wildcard imports matching these prefixes are allowed
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *WildcardImportRule) Confidence() float64 { return api.ConfidenceMedium }

func wildcardImportExcluded(imp, excl string) bool {
	imp = strings.TrimSpace(imp)
	excl = strings.TrimSpace(excl)
	if imp == "" || excl == "" {
		return false
	}
	if imp == excl {
		return true
	}
	if strings.HasSuffix(excl, ".**") {
		prefix := strings.TrimSuffix(excl, "**")
		return strings.HasPrefix(imp, prefix)
	}
	return false
}

func importStatementPath(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	// FirstHeaderLine handles tree-sitter trailing trivia (comments,
	// trailing semicolon). Java imports may carry a leading `static`
	// modifier; strip it after the keyword/trivia pass.
	text := sourceheader.FirstHeaderLine(file.FlatNodeText(idx), "import")
	text = strings.TrimSpace(strings.TrimPrefix(text, "static"))
	return strings.TrimSpace(text)
}

// ForbiddenCommentRule detects TODO:, FIXME:, STOPSHIP: markers.
type ForbiddenCommentRule struct {
	FlatDispatchBase
	BaseRule
	Comments        []string       // forbidden comment markers
	AllowedPatterns *regexp.Regexp // regex; comments matching this are allowed
}

var defaultForbiddenCommentMarkers = []string{"TODO:", "FIXME:", "STOPSHIP:"}
var trackedBugTodoCommentPattern = regexp.MustCompile(`\bTODO(?:\s*\(\s*b/\d+\s*\)|:\s*b/\d+\b)`)

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenCommentRule) Confidence() float64 { return api.ConfidenceMedium }

func forbiddenCommentAllowedByDefault(text, marker string) bool {
	if marker != "TODO:" {
		return false
	}
	return trackedBugTodoCommentPattern.MatchString(text)
}

// ForbiddenVoidRule detects Void type usage.
type ForbiddenVoidRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreOverridden      bool
	IgnoreUsageInGenerics bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenVoidRule) Confidence() float64 { return api.ConfidenceMedium }

// javaInteropGenericTypes are Java generic types where Void is the canonical
// way to say "no result" and Unit is not substitutable.
var javaInteropGenericTypes = map[string]bool{
	"AsyncTask":         true,
	"Callable":          true,
	"CompletableFuture": true,
	"Future":            true,
	"ListenableFuture":  true,
	"Supplier":          true,
	"Function":          true,
	"BiFunction":        true,
	"Single":            true, // RxJava
	"Maybe":             true,
	"Observable":        true,
	"Flowable":          true,
	"Completable":       true,
}

// ForbiddenImportRule detects banned import patterns.
type ForbiddenImportRule struct {
	FlatDispatchBase
	BaseRule
	Patterns         []string // kept for backward compat; same as ForbiddenImports
	ForbiddenImports []string
	AllowedImports   []string
}

var defaultForbiddenImports = []string{
	"sun.",
	"jdk.internal.",
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenImportRule) Confidence() float64 { return api.ConfidenceMedium }

// ForbiddenEntry pairs a forbidden value with an optional reason.
type ForbiddenEntry struct {
	Value  string
	Reason string
}

// ForbiddenMethodCallRule detects banned method calls.
type ForbiddenMethodCallRule struct {
	FlatDispatchBase
	BaseRule
	Methods []string // simple list kept for backward compat
}

var defaultForbiddenMethods = []string{"print(", "println("}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenMethodCallRule) Confidence() float64 { return api.ConfidenceMedium }

func forbiddenMethodCallMatch(ctx *api.Context, call uint32, methods []string) (string, bool) {
	if ctx.File == nil || len(methods) == 0 {
		return "", false
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || target.CalleeName == "" {
		return "", false
	}
	for _, spec := range methods {
		name := forbiddenMethodSpecName(spec)
		if name == "" || target.CalleeName != name {
			continue
		}
		if target.Resolved && forbiddenMethodResolvedTargetMatches(spec, target.QualifiedName, name) {
			return name, true
		}
		if !target.Resolved && forbiddenMethodImplicitKotlinStdlibMatch(ctx, call, name) {
			return name, true
		}
		if !target.Resolved && forbiddenMethodSameFileDeclarationMatch(ctx, call, name) {
			if forbiddenMethodImplicitKotlinStdlibName(name) {
				continue
			}
			return name, true
		}
		if !target.Resolved && forbiddenMethodQualifiedReferenceMatches(ctx.File, call, spec, name) {
			return name, true
		}
	}
	return "", false
}

func forbiddenMethodImplicitKotlinStdlibMatch(ctx *api.Context, call uint32, name string) bool {
	if !forbiddenMethodImplicitKotlinStdlibName(name) || ctx == nil || ctx.File == nil {
		return false
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || target.Receiver.Node != 0 {
		return false
	}
	return !forbiddenMethodSameFileDeclarationMatch(ctx, call, name)
}

func forbiddenMethodImplicitKotlinStdlibName(name string) bool {
	return name == "print" || name == "println"
}

func forbiddenMethodSpecName(spec string) string {
	spec = strings.TrimSpace(strings.TrimSuffix(spec, "("))
	if spec == "" {
		return ""
	}
	if idx := strings.Index(spec, "("); idx >= 0 {
		spec = spec[:idx]
	}
	if idx := strings.LastIndex(spec, "."); idx >= 0 {
		return spec[idx+1:]
	}
	return spec
}

func forbiddenMethodResolvedTargetMatches(spec, qualifiedName, simpleName string) bool {
	spec = strings.TrimSpace(strings.TrimSuffix(spec, "("))
	if idx := strings.Index(spec, "("); idx >= 0 {
		spec = spec[:idx]
	}
	qualifiedName = strings.ReplaceAll(strings.TrimSpace(qualifiedName), "#", ".")
	if strings.Contains(spec, ".") {
		return qualifiedName == spec || strings.HasSuffix(qualifiedName, "."+spec)
	}
	switch simpleName {
	case "print", "println":
		return qualifiedName == "kotlin.io."+simpleName
	default:
		return strings.HasSuffix(qualifiedName, "."+simpleName)
	}
}

func forbiddenMethodSameFileDeclarationMatch(ctx *api.Context, call uint32, name string) bool {
	if ctx.File == nil || name == "" {
		return false
	}
	found := false
	ctx.File.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || extractIdentifierFlat(ctx.File, fn) != name {
			return
		}
		found = semantics.SameFileDeclarationMatch(ctx, fn, call)
	})
	return found
}

func forbiddenMethodQualifiedReferenceMatches(file *scanner.File, call uint32, spec string, name string) bool {
	if file == nil || !strings.Contains(spec, ".") {
		return false
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 {
		return false
	}
	segments := flatNavigationChainIdentifiers(file, navExpr)
	if len(segments) == 0 || segments[len(segments)-1] != name {
		return false
	}
	got := strings.Join(segments, ".")
	spec = strings.TrimSuffix(strings.TrimSpace(spec), "(")
	if idx := strings.Index(spec, "("); idx >= 0 {
		spec = spec[:idx]
	}
	return got == spec
}

// ForbiddenAnnotationRule detects annotations that should not be used.
type ForbiddenAnnotationRule struct {
	FlatDispatchBase
	BaseRule
	Annotations []string
}

var defaultForbiddenAnnotations = []string{"SuppressWarnings"}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenAnnotationRule) Confidence() float64 { return api.ConfidenceMedium }

// ForbiddenNamedParamRule detects named parameters in certain function calls.
type ForbiddenNamedParamRule struct {
	FlatDispatchBase
	BaseRule
	Methods []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenNamedParamRule) Confidence() float64 { return api.ConfidenceMedium }

// ForbiddenOptInRule detects @OptIn annotations.
type ForbiddenOptInRule struct {
	FlatDispatchBase
	BaseRule
	MarkerClasses []string // specific marker classes to forbid; empty = all @OptIn
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenOptInRule) Confidence() float64 { return api.ConfidenceMedium }

// ForbiddenSuppressRule detects @Suppress annotations.
type ForbiddenSuppressRule struct {
	FlatDispatchBase
	BaseRule
	Rules []string // specific suppressed rules to forbid; empty = all @Suppress
}

// Confidence reports a tier-2 (medium) base confidence. Style/forbidden rule. Detection flags configured forbidden
// imports/methods/annotations via literal string/regex match; false
// positives arise when project-local names collide with forbidden list
// entries. Classified per roadmap/17.
func (r *ForbiddenSuppressRule) Confidence() float64 { return api.ConfidenceMedium }

// isInsidePreviewOrSampleFunctionFlatWithConfig returns true if the node is
// inside a function whose name or annotation marks it as a preview / sample /
// fake / mock / stub — UI tooling scaffolding rather than production code.
func isInsidePreviewOrSampleFunctionFlatWithConfig(file *scanner.File, idx uint32, cfg customPreviewConfig) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		nodeType := file.FlatType(p)
		if nodeType != "function_declaration" {
			if nodeType != "source_file" && previewOrSampleFunctionLikeTextWithConfig(file.FlatNodeText(p), cfg) {
				return true
			}
			continue
		}
		name := extractIdentifierFlat(file, p)
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "preview") || strings.HasPrefix(lower, "sample") ||
			strings.HasPrefix(lower, "fake") || strings.HasPrefix(lower, "mock") ||
			strings.HasPrefix(lower, "stub") || strings.HasPrefix(lower, "fixture") ||
			strings.HasSuffix(lower, "preview") || strings.HasSuffix(lower, "sample") ||
			strings.HasSuffix(lower, "fixture") {
			return true
		}
		// Also check for common preview annotations.
		mods, _ := file.FlatFindChild(p, "modifiers")
		if mods != 0 {
			modText := file.FlatNodeText(mods)
			if textHasCustomPreviewAnnotation(modText, cfg) {
				return true
			}
		}
		return false
	}
	return false
}

func previewOrSampleFunctionLikeTextWithConfig(text string, cfg customPreviewConfig) bool {
	if text == "" || !strings.Contains(text, "fun ") {
		return false
	}
	signature := text
	if open := strings.Index(signature, "{"); open >= 0 {
		signature = signature[:open]
	}
	if len(signature) > 500 {
		signature = signature[:500]
	}
	if textHasCustomPreviewAnnotation(signature, cfg) {
		return true
	}
	name := functionNameFromSignature(signature)
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "preview") || strings.HasPrefix(lower, "sample") ||
		strings.HasPrefix(lower, "fake") || strings.HasPrefix(lower, "mock") ||
		strings.HasPrefix(lower, "stub") || strings.HasPrefix(lower, "fixture") ||
		strings.HasSuffix(lower, "preview") || strings.HasSuffix(lower, "sample") ||
		strings.HasSuffix(lower, "fixture")
}

func functionNameFromSignature(signature string) string {
	i := strings.Index(signature, "fun ")
	if i < 0 {
		return ""
	}
	rest := strings.TrimLeft(signature[i+len("fun "):], " \t\r\n")
	start := 0
	for start < len(rest) {
		c := rest[start]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' {
			break
		}
		start++
	}
	end := start
	for end < len(rest) {
		c := rest[end]
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			break
		}
		end++
	}
	if end <= start {
		return ""
	}
	return rest[start:end]
}
