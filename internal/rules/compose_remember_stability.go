package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// composeEnclosingLambdaArgumentName walks up from idx looking for the
// nearest lambda_literal ancestor that is itself a named value_argument, and
// returns the argument label (e.g. "onClick", "onValueChange"). Returns ""
// if no such enclosing lambda exists, or if the lambda is not a named
// argument. This powers callback/handler detection without needing type
// info about Compose itself.
func composeEnclosingLambdaArgumentName(file *scanner.File, idx uint32) string {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "lambda_literal" {
			continue
		}
		parent, pok := file.FlatParent(cur)
		if !pok {
			return ""
		}
		// Lambdas passed as trailing lambdas are wrapped in annotated_lambda
		// / lambda_argument, not value_argument — skip those.
		if file.FlatType(parent) != "value_argument" {
			return ""
		}
		if file.FlatNamedChildCount(parent) < 2 {
			return ""
		}
		label := file.FlatNamedChild(parent, 0)
		if label == 0 || file.FlatType(label) != "simple_identifier" {
			return ""
		}
		return file.FlatNodeText(label)
	}
	return ""
}

// composeLambdaReferencesEnclosingParam reports whether the lambda body at
// lambdaIdx references any simple_identifier that matches a parameter of
// the nearest enclosing function_declaration. Used to detect
// `remember { f(param) }` / `LaunchedEffect(Unit) { f(param) }` patterns
// where the lambda captures mutable outer state without listing it as a key.
func composeLambdaReferencesEnclosingParam(file *scanner.File, lambdaIdx uint32) (string, bool) {
	fn, ok := flatEnclosingFunction(file, lambdaIdx)
	if !ok {
		return "", false
	}
	paramNames := flatFunctionParameterNames(file, fn)
	if len(paramNames) == 0 {
		return "", false
	}
	paramSet := make(map[string]struct{}, len(paramNames))
	for _, n := range paramNames {
		paramSet[n] = struct{}{}
	}

	var hit string
	file.FlatWalkAllNodes(lambdaIdx, func(idx uint32) {
		if hit != "" {
			return
		}
		if file.FlatType(idx) != "simple_identifier" {
			return
		}
		text := file.FlatNodeText(idx)
		if _, match := paramSet[text]; !match {
			return
		}
		// Filter out identifiers that appear as a label/name (declaration or
		// named-argument label), not as a reference — those don't count as a
		// capture.
		parent, pok := file.FlatParent(idx)
		if !pok {
			return
		}
		switch file.FlatType(parent) {
		case "variable_declaration", "parameter", "navigation_suffix":
			return
		case "value_argument":
			// The first named child of a value_argument whose count is >= 2
			// is the argument label (e.g. `onClick = { ... }`), not a reference.
			if file.FlatNamedChildCount(parent) >= 2 && file.FlatNamedChild(parent, 0) == idx {
				return
			}
		}
		hit = text
	})
	return hit, hit != ""
}

// ComposeRememberWithoutKeyRule flags `remember { f(param) }` where the
// lambda body references an enclosing function parameter but the remember
// call has no key arguments. The cached value won't update when the
// parameter changes across recomposition.
type ComposeRememberWithoutKeyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeRememberWithoutKeyRule) Confidence() float64 { return api.ConfidenceMedium }

// ComposeLaunchedEffectWithoutKeysRule flags `LaunchedEffect(Unit) { f(param) }`
// where the effect body references an enclosing parameter but the keys are
// constant (Unit / true / false / null). The effect won't re-run when the
// parameter changes.
type ComposeLaunchedEffectWithoutKeysRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeLaunchedEffectWithoutKeysRule) Confidence() float64 { return api.ConfidenceMedium }

var composeConstantKeyTexts = map[string]struct{}{
	"Unit":  {},
	"true":  {},
	"false": {},
	"null":  {},
}

// ComposeUnstableParameterRule flags `@Composable fun X(users: List<User>)`
// whose parameters use the mutable Kotlin collection types (`List`, `Map`,
// `Set`, `MutableList`, etc.) directly. Compose considers these unstable
// for recomposition-skipping unless the author wraps them in a stable type
// (`@Immutable data class`, `ImmutableList` / `PersistentList` from
// kotlinx.collections.immutable).
type ComposeUnstableParameterRule struct {
	FlatDispatchBase
	BaseRule
}

var composeUnstableCollectionTypes = map[string]struct{}{
	"List":        {},
	"Map":         {},
	"Set":         {},
	"MutableList": {},
	"MutableMap":  {},
	"MutableSet":  {},
	"Collection":  {},
	"Iterable":    {},
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeUnstableParameterRule) Confidence() float64 { return api.ConfidenceMedium }

func composeUnstableParameterShouldSkipFunctionFlat(file *scanner.File, fn uint32) bool {
	if file == nil || fn == 0 {
		return false
	}
	if isAndroidTestSupportArtifactSource(file.Path) {
		return true
	}
	for cur := fn; cur != 0; {
		if file.FlatType(cur) == "function_declaration" && composeFunctionIsPreviewOrSampleFlat(file, cur) {
			return true
		}
		parent, ok := file.FlatParent(cur)
		if !ok {
			break
		}
		cur = parent
	}
	return false
}

func composeFunctionIsPreviewOrSampleFlat(file *scanner.File, fn uint32) bool {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return false
	}
	name := extractIdentifierFlat(file, fn)
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "preview") || strings.HasPrefix(lower, "sample") ||
		strings.HasPrefix(lower, "fixture") ||
		strings.HasSuffix(lower, "preview") || strings.HasSuffix(lower, "sample") ||
		strings.HasSuffix(lower, "fixture") {
		return true
	}
	mods, _ := file.FlatFindChild(fn, "modifiers")
	if mods == 0 {
		return false
	}
	return composeModifierTextHasPreviewAnnotation(file.FlatNodeText(mods))
}

func composeModifierTextHasPreviewAnnotation(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] != '@' {
			continue
		}
		start := i + 1
		end := start
		for end < len(text) {
			c := text[end]
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
				(c >= '0' && c <= '9') || c == '_' || c == '.' {
				end++
				continue
			}
			break
		}
		if end > start && strings.HasSuffix(text[start:end], "Preview") {
			return true
		}
	}
	return false
}

// ComposeRememberSaveableNonParcelableRule flags `rememberSaveable {
// SomeType() }` where SomeType() is a constructor call to a plain class
// with no `saver` / `stateSaver` argument passed. Without a saver,
// rememberSaveable falls back to AutoSaver which only handles primitives,
// Parcelables, and a few stdlib types.
//
// Since this rule is syntactic it can't verify Parcelable status — it
// warns on any constructor-looking call in the trailing lambda whose name
// doesn't match a well-known stdlib type. Users can suppress the finding
// locally when they know the type is Parcelable/Serializable.
type ComposeRememberSaveableNonParcelableRule struct {
	FlatDispatchBase
	BaseRule
}

var composeSaverSafeBuiltins = map[string]struct{}{
	"Int":     {},
	"Long":    {},
	"Float":   {},
	"Double":  {},
	"Boolean": {},
	"Char":    {},
	"String":  {},
	"Unit":    {},
	"Short":   {},
	"Byte":    {},
	"BigInt":  {},
	"Bundle":  {},
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeRememberSaveableNonParcelableRule) Confidence() float64 { return api.ConfidenceMedium }
