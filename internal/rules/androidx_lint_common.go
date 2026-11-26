package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Shared plumbing for the ports of the Lint checks bundled with AndroidX artifacts (Compose material, animation,
// foundation, lifecycle, activity, startup, webkit and so on). Like the Compose runtime ports, Lint's scope for
// these issues includes test sources, so they also run on tests.

// axlRuleOptions tunes how registerAxlRule registers a port.
type axlRuleOptions struct {
	Fix               api.FixLevel
	Needs             api.Capabilities
	NeedsLibraryFacts bool
}

// registerAxlRule registers an AndroidX Lint port under its Lint issue ID, so lint.xml and Gradle `lint { }`
// severities reach it. The confidence is high unless impl declares Confidence(), and an impl declaring
// fileMentions() only runs on files whose text contains every listed string.
func registerAxlRule(rule BaseRule, impl any, nodeTypes []string, languages []scanner.Language, opts axlRuleOptions,
	check func(*api.Context)) {
	confidence := api.ConfidenceHigh
	if c, ok := impl.(interface{ Confidence() float64 }); ok {
		confidence = c.Confidence()
	}
	var gate func(*scanner.File) bool
	if g, ok := impl.(interface{ fileMentions() []string }); ok {
		texts := g.fileMentions()
		gate = func(file *scanner.File) bool {
			for _, text := range texts {
				if !fileFactsCache().Mentions(file, text) {
					return false
				}
			}
			return true
		}
	}
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Desc, Sev: api.Severity(rule.Sev),
		NodeTypes: nodeTypes, Languages: languages, Confidence: confidence, Fix: opts.Fix, Needs: opts.Needs,
		NeedsLibraryFacts: opts.NeedsLibraryFacts, Implementation: impl, Check: check, FileGate: gate, LintIssue: true,
	})
}

// axlLambdaParam is a lambda parameter Lint's findUnreferencedParameters can report: a named, non-destructured
// parameter, or the implicit `it` (Node is then the lambda itself).
type axlLambdaParam struct {
	Name string
	Node uint32
}

// axlUnreferencedLambdaParams ports compose lint-utils' ULambdaExpression.findUnreferencedParameters for a lambda
// literal whose function type has one value parameter: the implicit `it` when the lambda declares no parameters,
// otherwise every named, non-destructured parameter, that no name expression in the lambda refers
// to. A reference inside an inner lambda that declares a parameter of the same name (or, for `it`, has its own
// implicit `it`) does not count.
func axlUnreferencedLambdaParams(file *scanner.File, lambda uint32) []axlLambdaParam {
	var params []axlLambdaParam
	if axlLambdaHasParameterSpec(file, lambda) {
		if list, ok := file.FlatFindChild(lambda, "lambda_parameters"); ok {
			for p := file.FlatFirstChild(list); p != 0; p = file.FlatNextSib(p) {
				if file.FlatType(p) != "variable_declaration" {
					continue
				}
				if name := file.FlatChildTextOrEmpty(p, "simple_identifier"); name != "" {
					params = append(params, axlLambdaParam{Name: strings.Trim(name, "`"), Node: p})
				}
			}
		}
	} else {
		params = []axlLambdaParam{{Name: "it", Node: lambda}}
	}
	if len(params) == 0 {
		return nil
	}
	var out []axlLambdaParam
	for _, p := range params {
		if !axlLambdaReferences(file, lambda, p.Name) {
			out = append(out, p)
		}
	}
	return out
}

// axlLambdaHasParameterSpec mirrors KtFunctionLiteral.hasParameterSpecification: the lambda has an arrow, with or
// without parameters before it.
func axlLambdaHasParameterSpec(file *scanner.File, lambda uint32) bool {
	for child := file.FlatFirstChild(lambda); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "lambda_parameters", "->":
			return true
		case "statements":
			return false
		}
	}
	return false
}

// axlLambdaReferences reports whether a name expression in the lambda's body refers to name and is not shadowed by
// an inner lambda.
func axlLambdaReferences(file *scanner.File, lambda uint32, name string) bool {
	found := false
	var walk func(n uint32)
	walk = func(n uint32) {
		for child := file.FlatFirstChild(n); child != 0 && !found; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "lambda_parameters":
				continue
			case "lambda_literal":
				if axlLambdaShadows(file, child, name) {
					continue
				}
			case "simple_identifier", "type_identifier", "interpolated_identifier":
				if strings.Trim(file.FlatNodeText(child), "`") == name {
					found = true
				}
				continue
			}
			walk(child)
		}
	}
	walk(lambda)
	return found
}

// axlLambdaShadows reports whether an inner lambda rebinds name: one of its named, non-destructured parameters has
// that name, or name is `it` and the inner lambda has its own implicit `it`.
func axlLambdaShadows(file *scanner.File, lambda uint32, name string) bool {
	if !axlLambdaHasParameterSpec(file, lambda) {
		return name == "it" && axlLambdaImplicitItArity(file, lambda) == 1
	}
	list, ok := file.FlatFindChild(lambda, "lambda_parameters")
	if !ok {
		return false
	}
	for p := file.FlatFirstChild(list); p != 0; p = file.FlatNextSib(p) {
		if file.FlatType(p) == "variable_declaration" &&
			strings.Trim(file.FlatChildTextOrEmpty(p, "simple_identifier"), "`") == name {
			return true
		}
	}
	return false
}

// axlOneParameterLambdaFunctions are stdlib and Compose functions whose trailing lambda takes exactly one value
// parameter, so a parameterless lambda passed to them has an implicit `it`.
var axlOneParameterLambdaFunctions = map[string]bool{
	"let": true, "also": true, "takeIf": true, "takeUnless": true, "forEach": true, "onEach": true, "map": true,
	"mapNotNull": true, "filter": true, "filterNot": true, "flatMap": true, "any": true, "all": true, "none": true,
	"count": true, "first": true, "firstOrNull": true, "last": true, "lastOrNull": true, "find": true,
	"findLast": true, "single": true, "singleOrNull": true, "sortedBy": true, "sortedByDescending": true,
	"groupBy": true, "associateBy": true, "associateWith": true, "sumOf": true, "maxByOrNull": true,
	"minByOrNull": true, "partition": true, "items": true, "Crossfade": true, "AnimatedContent": true,
}

// axlLambdaImplicitItArity returns 1 when a parameterless lambda provably has an implicit `it`: it is the trailing
// lambda of a one-parameter-lambda function, or the argument for a same-file function's one-parameter function
// type. Otherwise it returns 0, so an unproven inner lambda never hides a reference to an outer `it`.
func axlLambdaImplicitItArity(file *scanner.File, lambda uint32) int {
	arg, ok := acrLambdaArgumentOf(file, lambda)
	if !ok {
		return 0
	}
	name, _, _ := acrCallCallee(file, arg.Call)
	if decl := acrSameFileFunction(file, name); decl != 0 {
		if param := acrParameterForArgument(file, decl, arg); param != 0 {
			return axlFunctionTypeArity(file, param)
		}
		return 0
	}
	if arg.Trailing && axlOneParameterLambdaFunctions[name] {
		return 1
	}
	return 0
}

// axlFunctionTypeArity returns the number of value parameters of a parameter's declared function type, or 0 when
// the type is not a function type.
func axlFunctionTypeArity(file *scanner.File, param uint32) int {
	var ft uint32
	file.FlatWalkNodes(param, "function_type", func(n uint32) {
		if ft == 0 {
			ft = n
		}
	})
	if ft == 0 {
		return 0
	}
	params, ok := file.FlatFindChild(ft, "function_type_parameters")
	if !ok {
		return 0
	}
	return file.FlatNamedChildCount(params)
}

// axlContentLambda returns the lambda literal passed for the call's parameter named param, as a trailing lambda
// (when param is the last parameter) or as `param = { }`, and the call.
func axlContentLambda(file *scanner.File, lambda uint32, param string, paramIsLast bool) (call uint32, ok bool) {
	arg, ok := acrLambdaArgumentOf(file, lambda)
	if !ok {
		return 0, false
	}
	if (arg.Trailing && paramIsLast) || arg.Label == param {
		return arg.Call, true
	}
	return 0, false
}

// axlCallTargets reports whether call is a call of pkg.name: a simple callee resolving to it (directly or through
// an import alias such as `import pkg.name as Other`), or a fully qualified `pkg.name(...)`.
func axlCallTargets(file *scanner.File, call uint32, pkg, name string) bool {
	if acrCallTargets(file, call, pkg, name) {
		return true
	}
	callee := acrCalleeExpression(file, call)
	if file.FlatType(callee) != "simple_identifier" {
		return false
	}
	alias := file.FlatNodeText(callee)
	return alias != name && fileFactsCache().Imports(file).Aliases[alias] == pkg+"."+name &&
		!acrFunctionNameShadowed(file, call, alias) && !acrFileDeclaresTopLevel(file, alias)
}
