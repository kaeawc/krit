package rules

import (
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// Batch 3: Flow / StateFlow rules
// ---------------------------------------------------------------------------

// StateFlowMutableLeakRule detects publicly exposed MutableStateFlow.
type StateFlowMutableLeakRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *StateFlowMutableLeakRule) Confidence() float64 { return api.ConfidenceMedium }

func stateFlowMutableLeakHasKotlinxEvidence(file *scanner.File, propText string) bool {
	content := string(file.Content)
	return strings.Contains(propText, "kotlinx.coroutines.flow.MutableStateFlow") ||
		strings.Contains(content, "import kotlinx.coroutines.flow.MutableStateFlow") ||
		strings.Contains(content, "import kotlinx.coroutines.flow.*")
}

// SharedFlowWithoutReplayRule detects MutableSharedFlow() with no buffer config.
type SharedFlowWithoutReplayRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SharedFlowWithoutReplayRule) Confidence() float64 { return api.ConfidenceMedium }

// StateFlowCompareByReferenceRule detects .map{}.distinctUntilChanged() on StateFlow.
type StateFlowCompareByReferenceRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *StateFlowCompareByReferenceRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// Batch 4: coroutine scope / context rules
// ---------------------------------------------------------------------------

// GlobalScopeLaunchInViewModelRule detects GlobalScope.launch in ViewModel/Presenter classes.
type GlobalScopeLaunchInViewModelRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *GlobalScopeLaunchInViewModelRule) Confidence() float64 { return api.ConfidenceMedium }

// SupervisorScopeInEventHandlerRule detects supervisorScope with a single child operation.
type SupervisorScopeInEventHandlerRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SupervisorScopeInEventHandlerRule) Confidence() float64 { return api.ConfidenceMedium }

// WithContextInSuspendFunctionNoopRule detects nested withContext with the same dispatcher.
type WithContextInSuspendFunctionNoopRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WithContextInSuspendFunctionNoopRule) Confidence() float64 { return api.ConfidenceMedium }

func extractWithContextDispatcher(ctx *api.Context, callIdx uint32) string {
	if ctx.File == nil {
		return ""
	}
	file := ctx.File
	args := flatCallKeyArguments(file, callIdx)
	if args == 0 {
		return ""
	}
	firstArg := flatPositionalValueArgument(file, args, 0)
	if firstArg == 0 {
		return ""
	}
	expr := flatValueArgumentExpression(file, firstArg)
	if expr == 0 || !injectDispatcherReferenceConfirmed(file, expr) {
		return ""
	}
	segments := flatNavigationChainIdentifiers(file, expr)
	if len(segments) == 2 {
		return strings.Join(segments, ".")
	}
	return ""
}

// withContextCallResolves returns true when a bare `withContext(...)` call in
// `file` can be attributed to `kotlinx.coroutines.withContext`. The rule must
// stay quiet when the file has no kotlinx.coroutines.withContext import (or
// wildcard) or when a top-level/local function named `withContext` shadows the
// stdlib symbol.
func withContextCallResolves(file *scanner.File, callIdx uint32) bool {
	if file == nil {
		return false
	}
	if !fileImportsFQN(file, "kotlinx.coroutines.withContext") {
		return false
	}
	if fileDeclaresFunctionNamed(file, "withContext") {
		return false
	}
	// Reject member calls like `scope.withContext(...)` — only the bare
	// identifier form resolves to the stdlib `kotlinx.coroutines.withContext`.
	// Handle both the direct call (`withContext(d) {}`) and the trailing-
	// lambda outer call shape (outer call_expression whose first child is the
	// inner `withContext(d)` call_expression).
	target := callIdx
	if file.FlatType(target) == "call_expression" {
		navExpr, args := flatCallExpressionParts(file, target)
		if navExpr == 0 && args == 0 {
			// Possibly the outer trailing-lambda wrapper; descend into the
			// first nested call_expression child.
			for child := file.FlatFirstChild(target); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "call_expression" {
					target = child
					navExpr, _ = flatCallExpressionParts(file, target)
					break
				}
			}
		}
		if navExpr != 0 {
			return false
		}
	}
	return true
}

// fileDeclaresFunctionNamed reports whether the file contains a
// function_declaration named `name` anywhere (top-level, inside a class, or
// nested). This is intentionally broad: any such shadow makes the bare
// `withContext` reference ambiguous, so the rule refuses to fire.
func fileDeclaresFunctionNamed(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found {
			return
		}
		if file.FlatType(idx) != "function_declaration" {
			return
		}
		if extractIdentifierFlat(file, idx) == name {
			found = true
		}
	})
	return found
}

// withContextSameScopeAncestor walks ancestors of `inner` looking for an
// enclosing `withContext(dispatcher)` call. It stops at scope boundaries that
// invalidate the comparison: function/class/object declarations and lambdas
// whose owning call is NOT another `withContext` (e.g. `launch`, `async`,
// `runBlocking`, `flow`, `coroutineScope`, `supervisorScope` — those create a
// new coroutine context where re-stating the dispatcher is meaningful).
//
// The returned dispatcher is the outer parent's dispatcher string, or "" if no
// matching ancestor was found within the same scope.
func withContextSameScopeAncestor(ctx *api.Context, inner uint32, innerDispatcher string) string {
	file := ctx.File
	if file == nil {
		return ""
	}
	for p, ok := file.FlatParent(inner); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "anonymous_function",
			"class_declaration", "object_declaration",
			"anonymous_initializer":
			return ""
		case "lambda_literal":
			// Crossing a lambda is fine ONLY when the lambda is the trailing
			// lambda of a `withContext(sameDispatcher)` call. Otherwise the
			// lambda belongs to a launch/async/coroutineScope/flow/etc. — a
			// separate coroutine context where re-stating the dispatcher is
			// meaningful, not a no-op.
			ownerCall := lambdaOwnerCall(file, p)
			if ownerCall == 0 {
				return ""
			}
			if flatCallNameAny(file, ownerCall) != "withContext" {
				return ""
			}
			if !withContextCallResolves(file, lambdaOwnerInnerCall(file, ownerCall)) {
				return ""
			}
			ownerDispatcher := extractWithContextDispatcher(ctx, ownerCall)
			if ownerDispatcher == "" {
				return ""
			}
			// Found the enclosing withContext via its trailing lambda. If the
			// dispatchers match, that's the redundant nesting we want to
			// flag. If not, the enclosing context is different, so the inner
			// switch is intentional.
			if ownerDispatcher == innerDispatcher {
				return ownerDispatcher
			}
			return ""
		case "call_expression":
			// A call_expression on the path (e.g. the trailing-lambda outer
			// wrapper of the inner call itself) is harmless; the lambda case
			// above is what actually proves real enclosure.
			continue
		}
	}
	return ""
}

// lambdaOwnerCall returns the call_expression whose trailing lambda is the
// given lambda_literal idx, or 0 when the lambda is not a trailing lambda of
// a call. Handles both `name { body }` (lambda is a child of `call_suffix`)
// and `name(args) { body }` (lambda is on the outer call's call_suffix while
// the args live on a nested inner call_expression).
func lambdaOwnerCall(file *scanner.File, lambda uint32) uint32 {
	if file == nil || lambda == 0 {
		return 0
	}
	p, ok := file.FlatParent(lambda)
	if !ok {
		return 0
	}
	// lambda_literal may be wrapped in annotated_lambda.
	if file.FlatType(p) == "annotated_lambda" {
		p, ok = file.FlatParent(p)
		if !ok {
			return 0
		}
	}
	if file.FlatType(p) != "call_suffix" {
		return 0
	}
	owner, ok := file.FlatParent(p)
	if !ok || file.FlatType(owner) != "call_expression" {
		return 0
	}
	return owner
}

// lambdaOwnerInnerCall returns the inner call_expression that carries
// `withContext(args)` when the outer trailing-lambda call wraps it; otherwise
// returns the outer call itself. This is the node `withContextCallResolves`
// expects (must point at the bare-identifier call).
func lambdaOwnerInnerCall(file *scanner.File, outer uint32) uint32 {
	if file == nil || outer == 0 {
		return 0
	}
	for child := file.FlatFirstChild(outer); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "call_expression" {
			return child
		}
	}
	return outer
}

// LaunchWithoutCoroutineExceptionHandlerRule detects launch{} with throw but no handler.
type LaunchWithoutCoroutineExceptionHandlerRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LaunchWithoutCoroutineExceptionHandlerRule) Confidence() float64 {
	return api.ConfidenceMedium
}

// ---------------------------------------------------------------------------
// Batch 5: cross-module rule
// ---------------------------------------------------------------------------

// MainDispatcherInLibraryCodeRule detects Dispatchers.Main in library modules
// without kotlinx-coroutines-android dependency.
type MainDispatcherInLibraryCodeRule struct {
	BaseRule
}

func (r *MainDispatcherInLibraryCodeRule) IsFixable() bool     { return false }
func (r *MainDispatcherInLibraryCodeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MainDispatcherInLibraryCodeRule) check(ctx *api.Context) {
	pmi := ctx.ModuleIndex
	if pmi == nil || pmi.Graph == nil {
		return
	}

	for modPath, mod := range pmi.Graph.Modules {
		if !isAndroidLibraryModule(mod) {
			continue
		}
		if hasCoroutinesAndroidDep(mod) {
			continue
		}
		files := pmi.ModuleFiles[modPath]
		for _, file := range files {
			if file == nil || file.FlatTree == nil {
				continue
			}
			localCtx := *ctx
			localCtx.File = file
			file.FlatWalkNodes(0, "navigation_expression", func(idx uint32) {
				if mainDispatcherReferenceFlat(&localCtx, idx) {
					ctx.Emit(scanner.Finding{
						File:     file.Path,
						Line:     file.FlatRow(idx) + 1,
						Col:      file.FlatCol(idx) + 1,
						RuleSet:  r.RuleSetName,
						Rule:     r.RuleName,
						Severity: r.Sev,
						Message:  "Dispatchers.Main used in a library module without kotlinx-coroutines-android dependency.",
					})
				}
			})
		}
	}
}

func (r *MainDispatcherInLibraryCodeRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{
		NeedsFiles:        true,
		NeedsDependencies: true,
		NeedsIndex:        false,
	}
}

func isAndroidLibraryModule(mod *module.Module) bool {
	buildFile := filepath.Join(mod.Dir, "build.gradle.kts")
	cfg, err := android.ParseBuildGradle(buildFile)
	if err != nil {
		buildFile = filepath.Join(mod.Dir, "build.gradle")
		cfg, err = android.ParseBuildGradle(buildFile)
		if err != nil {
			return false
		}
	}
	for _, plugin := range cfg.Plugins {
		if plugin == "com.android.library" {
			return true
		}
	}
	return false
}

func hasCoroutinesAndroidDep(mod *module.Module) bool {
	buildFile := filepath.Join(mod.Dir, "build.gradle.kts")
	cfg, err := android.ParseBuildGradle(buildFile)
	if err != nil {
		buildFile = filepath.Join(mod.Dir, "build.gradle")
		cfg, err = android.ParseBuildGradle(buildFile)
		if err != nil {
			return false
		}
	}
	for _, dep := range cfg.Dependencies {
		if dep.Name == "kotlinx-coroutines-android" {
			return true
		}
	}
	return false
}
