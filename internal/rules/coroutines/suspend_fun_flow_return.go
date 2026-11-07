package coroutines

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/base"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// SuspendFunWithFlowReturnTypeRule flags `suspend fun foo(): Flow<T>` and
// related Flow-returning suspend functions. Flow builders are cold and do
// not require the caller to be in a coroutine, so the suspend modifier is
// redundant at best and misleading at worst.
type SuspendFunWithFlowReturnTypeRule struct {
	base.FlatDispatchBase
	base.BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule.
func (r *SuspendFunWithFlowReturnTypeRule) Confidence() float64 { return api.ConfidenceHigh }

// IsFixable marks this rule as auto-fixable: stripping the suspend modifier.
func (r *SuspendFunWithFlowReturnTypeRule) IsFixable() bool { return true }

// Meta returns the descriptor metadata for the rule.
func (r *SuspendFunWithFlowReturnTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SuspendFunWithFlowReturnType",
		RuleSet:       "coroutines",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}

// flowReturnTypeNames is the recognized set of cold-Flow types whose
// suspending wrapper is structurally redundant.
var flowReturnTypeNames = map[string]bool{
	"Flow":              true,
	"StateFlow":         true,
	"SharedFlow":        true,
	"MutableStateFlow":  true,
	"MutableSharedFlow": true,
}

func init() {
	r := &SuspendFunWithFlowReturnTypeRule{
		BaseRule: base.BaseRule{
			RuleName:    "SuspendFunWithFlowReturnType",
			RuleSetName: "coroutines",
			Sev:         "warning",
			Desc:        "Detects suspend functions that return a Flow type, since Flow builders are cold and do not require suspend.",
		},
	}
	api.Register(&api.Rule{
		ID:             r.RuleName,
		Category:       r.RuleSetName,
		Description:    r.Desc,
		Sev:            api.Severity(r.Sev),
		NodeTypes:      []string{"function_declaration"},
		Confidence:     0.85,
		Fix:            api.FixIdiomatic,
		Implementation: r,
		// The resolver knows which project functions are declared suspend.
		Needs: api.NeedsResolver,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !file.FlatHasModifier(idx, "suspend") {
				return
			}
			// The signature is dictated by a supertype, left to subclasses, or declared without a body.
			// An open hook keeps suspend so overrides can suspend.
			if file.FlatHasModifier(idx, "override") || file.FlatHasModifier(idx, "abstract") || file.FlatHasModifier(idx, "open") ||
				file.FlatHasModifier(idx, "expect") || isInterfaceMember(file, idx) {
				return
			}
			body, hasBody := file.FlatFindChild(idx, "function_body")
			if !hasBody {
				return
			}
			// A suspend call before the flow is built is why the modifier is needed.
			if callsSuspendOutsideLambdas(ctx, body) {
				return
			}
			if !returnsFlowType(ctx, idx, body, hasBody) {
				return
			}
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
				"Suspend function returns a Flow type. A function that returns Flow should not be suspend. The flow builder is cold and does not require a coroutine.")
			suspendNode := file.FlatFindModifierNode(idx, "suspend")
			if suspendNode != 0 {
				endByte := int(file.FlatEndByte(suspendNode))
				if endByte < len(file.Content) && file.Content[endByte] == ' ' {
					endByte++
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(suspendNode)),
					EndByte:     endByte,
					Replacement: "",
				}
			}
			ctx.Emit(f)
		},
	})
}

// isInterfaceMember reports whether the function is declared directly in an interface.
func isInterfaceMember(file *scanner.File, fn uint32) bool {
	for p, ok := file.FlatParent(fn); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "class_declaration" {
			continue
		}
		for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "interface" {
				return true
			}
		}
		return false
	}
	return false
}

// knownSuspendCallees are coroutine APIs that always suspend.
var knownSuspendCallees = map[string]bool{
	"delay": true, "await": true, "awaitAll": true, "withContext": true, "coroutineScope": true,
	"supervisorScope": true, "yield": true, "withTimeout": true, "withTimeoutOrNull": true,
	"join": true, "joinAll": true, "suspendCoroutine": true, "suspendCancellableCoroutine": true,
}

type oracleProvider interface {
	Oracle() oracle.Lookup
}

// callsSuspendOutsideLambdas reports whether the body makes a provably suspending call outside any lambda.
// Calls inside lambdas, such as a flow builder, run later and do not need the modifier.
func callsSuspendOutsideLambdas(ctx *api.Context, body uint32) bool {
	file := ctx.File
	var lookup oracle.Lookup
	if p, ok := ctx.Resolver.(oracleProvider); ok {
		lookup = p.Oracle()
	}
	suspendFuns := map[string]bool{}
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if file.FlatHasModifier(fn, "suspend") {
			if id, ok := file.FlatFindChild(fn, "simple_identifier"); ok {
				suspendFuns[file.FlatNodeText(id)] = true
			}
		}
	})
	found := false
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if found || callIsNested(file, body, call) {
			return
		}
		name := calleeSimpleName(file, call)
		// Any project file may declare the callee `suspend`; so may a chained call with a dual-use name.
		if ProjectDeclaresSuspendFunction(ctx.Resolver, name) || chainedCallMaySuspend(file, call, name) {
			found = true
			return
		}
		if lookup != nil {
			if isSuspend, ok := callTargetSuspend(lookup, file, call); ok {
				found = isSuspend
				return
			}
		}
		found = knownSuspendCallees[name] || suspendFuns[name]
	})
	return found
}

// callIsNested reports whether the call sits inside a lambda or nested function below root.
func callIsNested(file *scanner.File, root, call uint32) bool {
	for p, ok := file.FlatParent(call); ok && p != root; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal", "function_declaration", "anonymous_function":
			return true
		}
	}
	return false
}

func calleeSimpleName(file *scanner.File, call uint32) string {
	first := file.FlatFirstChild(call)
	if first == 0 {
		return ""
	}
	text := file.FlatNodeText(first)
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		text = text[dot+1:]
	}
	return strings.TrimSpace(text)
}

// callTargetSuspend reads the byte-range fact when the oracle has one, so a call starting where another
// call of its chain starts is not answered with that other call's fact.
func callTargetSuspend(lookup oracle.Lookup, file *scanner.File, call uint32) (bool, bool) {
	if flat, ok := lookup.(interface {
		LookupCallTargetSuspendFlat(file *scanner.File, idx uint32) (bool, bool)
	}); ok {
		return flat.LookupCallTargetSuspendFlat(file, call)
	}
	return lookup.LookupCallTargetSuspend(file.Path, file.FlatRow(call)+1, file.FlatCol(call)+1)
}

type suspendDeclarationLookup interface {
	DeclaresSuspendFunction(name string) bool
}

// ProjectDeclaresSuspendFunction reports whether any indexed project file declares a `suspend` function with this name.
func ProjectDeclaresSuspendFunction(resolver typeinfer.TypeResolver, name string) bool {
	for resolver != nil {
		if lookup, ok := resolver.(suspendDeclarationLookup); ok {
			return lookup.DeclaresSuspendFunction(name)
		}
		composite, ok := resolver.(*oracle.CompositeResolver)
		if !ok {
			return false
		}
		resolver = composite.Fallback()
	}
	return false
}

// DualUseChainNames are member names that exist both as suspend and non-suspend overloads.
var DualUseChainNames = map[string]bool{
	"first": true, "firstOrNull": true, "single": true, "singleOrNull": true, "last": true, "lastOrNull": true,
	"toList": true, "toSet": true, "count": true, "fold": true, "reduce": true, "collect": true,
	"collectLatest": true, "join": true, "receive": true, "send": true, "emit": true, "lock": true,
	"withLock": true, "acquire": true, "test": true,
}

// chainedCallMaySuspend reports a member call whose name is a dual-use suspend name or starts with `await`.
func chainedCallMaySuspend(file *scanner.File, call uint32, name string) bool {
	first := file.FlatFirstChild(call)
	if first == 0 || file.FlatType(first) != "navigation_expression" {
		return false
	}
	return DualUseChainNames[name] || strings.HasPrefix(name, "await")
}

// returnsFlowType reports a declared Flow return type, or an expression body the compiler typed as a Flow.
func returnsFlowType(ctx *api.Context, fn, body uint32, hasBody bool) bool {
	file := ctx.File
	if declared := declaredReturnTypeNode(file, fn); declared != 0 {
		if file.FlatType(declared) == "nullable_type" {
			if inner, ok := file.FlatFindChild(declared, "user_type"); ok {
				declared = inner
			}
		}
		if file.FlatType(declared) != "user_type" {
			return false
		}
		last := uint32(0)
		for c := file.FlatFirstChild(declared); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "type_identifier" {
				last = c
			}
		}
		return last != 0 && flowReturnTypeNames[file.FlatNodeText(last)]
	}
	// An inferred return type exists only for `= expression` bodies.
	if !hasBody {
		return false
	}
	if first := file.FlatFirstChild(body); first == 0 || file.FlatType(first) != "=" {
		return false
	}
	expr := uint32(0)
	for c := file.FlatFirstChild(body); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			expr = c
		}
	}
	if expr == 0 || ctx.Resolver == nil {
		return false
	}
	rt := ctx.Resolver.ResolveFlatNode(expr, file)
	if rt == nil || !rt.Resolved || rt.FQN == "" {
		return false
	}
	name := rt.FQN
	if i := strings.IndexByte(name, '<'); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimSuffix(name, "?")
	short := name[strings.LastIndexByte(name, '.')+1:]
	return strings.HasPrefix(name, "kotlinx.coroutines.flow.") && flowReturnTypeNames[short]
}

// declaredReturnTypeNode returns the type written after the parameter list, or 0.
func declaredReturnTypeNode(file *scanner.File, fn uint32) uint32 {
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return 0
	}
	next := file.FlatNextSib(params)
	if next == 0 || file.FlatType(next) != ":" {
		return 0
	}
	return file.FlatNextSib(next)
}
