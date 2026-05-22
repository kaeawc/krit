package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// HiltEntryPointOnNonInterfaceRule detects Hilt entry points declared as a
// class or object instead of an interface.
type HiltEntryPointOnNonInterfaceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule. Detection uses annotation and import patterns for
// Dagger/Hilt/Anvil; project-specific DI aliases are not followed.
// Classified per roadmap/17.
func (r *HiltEntryPointOnNonInterfaceRule) Confidence() float64 { return api.ConfidenceMedium }

func hiltEntryPointDeclarationFlat(file *scanner.File, idx uint32) (kind, name string, line int, ok bool) {
	if file == nil || !hasAnnotationFlat(file, idx, "EntryPoint") {
		return "", "", 0, false
	}

	switch file.FlatType(idx) {
	case "class_declaration", "object_declaration":
		return hiltEntryPointDeclKindFlat(file, idx), extractIdentifierFlat(file, idx), file.FlatRow(idx) + 1, true
	case "prefix_expression":
		target := hiltEntryPointAnnotatedTargetFlat(file, idx)
		if target == 0 {
			return "", "", 0, false
		}
		switch file.FlatType(target) {
		case "class_declaration", "object_declaration":
			return hiltEntryPointDeclKindFlat(file, target), extractIdentifierFlat(file, target), file.FlatRow(idx) + 1, true
		case "infix_expression":
			return hiltEntryPointInfixDeclFlat(file, target)
		}
	}

	return "", "", 0, false
}

func hiltEntryPointAnnotatedTargetFlat(file *scanner.File, idx uint32) uint32 {
	current := idx
	for file != nil && file.FlatType(current) == "prefix_expression" {
		if file.FlatNamedChildCount(current) < 2 {
			return 0
		}
		current = file.FlatNamedChild(current, 1)
	}
	return current
}

func hiltEntryPointInfixDeclFlat(file *scanner.File, idx uint32) (kind, name string, line int, ok bool) {
	if file == nil || file.FlatNamedChildCount(idx) < 2 {
		return "", "", 0, false
	}

	kind = file.FlatNodeText(file.FlatNamedChild(idx, 0))
	if kind != "class" && kind != "interface" {
		return "", "", 0, false
	}

	name = file.FlatNodeText(file.FlatNamedChild(idx, 1))
	return kind, name, file.FlatRow(idx) + 1, true
}

func hiltEntryPointDeclKindFlat(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "object_declaration":
		return "object"
	case "class_declaration":
		if file.FlatHasChildOfType(idx, "interface") {
			return "interface"
		}
		return "class"
	default:
		return "class"
	}
}

// HiltInstallInMismatchRule detects @Module/@InstallIn classes whose
// `@Provides` functions are annotated with a Hilt scope that does not match
// the module's installed component.
type HiltInstallInMismatchRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *HiltInstallInMismatchRule) Confidence() float64 { return api.ConfidenceMedium }

// hiltComponentScopes maps Hilt component class names to the scope annotations
// they own. The scopes are stored as a set for fast membership checks.
var hiltComponentScopes = map[string]map[string]struct{}{
	"SingletonComponent":        {"Singleton": {}},
	"ActivityRetainedComponent": {"ActivityRetainedScoped": {}},
	"ActivityComponent":         {"ActivityScoped": {}},
	"FragmentComponent":         {"FragmentScoped": {}},
	"ViewComponent":             {"ViewScoped": {}},
	"ViewWithFragmentComponent": {"ViewScoped": {}},
	"ServiceComponent":          {"ServiceScoped": {}},
	"ViewModelComponent":        {"ViewModelScoped": {}},
}

// hiltScopeAnnotations enumerates the Hilt scope annotation names. A function
// annotated with one of these but a non-matching component is suspect.
var hiltScopeAnnotations = []string{
	"Singleton",
	"ActivityRetainedScoped",
	"ActivityScoped",
	"FragmentScoped",
	"ViewScoped",
	"ServiceScoped",
	"ViewModelScoped",
}

// HiltSingletonWithActivityDepRule detects @Singleton classes whose constructor
// takes an Activity-, Fragment-, View-, or LifecycleOwner-scoped parameter,
// which is a scope mismatch.
type HiltSingletonWithActivityDepRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule.
func (r *HiltSingletonWithActivityDepRule) Confidence() float64 { return api.ConfidenceMedium }

var hiltSingletonActivityScopedTypes = map[string]struct{}{
	"Activity":                  {},
	"AppCompatActivity":         {},
	"ComponentActivity":         {},
	"FragmentActivity":          {},
	"Fragment":                  {},
	"DialogFragment":            {},
	"BottomSheetDialogFragment": {},
	"View":                      {},
	"ViewGroup":                 {},
	"LifecycleOwner":            {},
}
