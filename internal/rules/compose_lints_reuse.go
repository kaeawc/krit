package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Messages of compose-lint-checks 1.6.1's SlotReusedDetector and ModifierReusedDetector, as Lint reports them.
const (
	composeSlotReusedMessage = "Slots should be invoked at most once on each execution path to meet lifecycle " +
		"expectations. For example, calling a slot inside a branch and again afterward can compose two copies of the " +
		"slot's content and internal state. Calling it once in each mutually exclusive branch is allowed because only " +
		"one branch runs. See https://slackhq.github.io/compose-lints/rules/#do-not-invoke-slots-in-more-than-once-place " +
		"for more information."
	composeModifierReusedMessage = "The modifier is passed to more than one composable on this execution path. Pass " +
		"it only to the top-level layout, and use `Modifier` for child composables. See " +
		"https://slackhq.github.io/compose-lints/rules/#dont-re-use-modifiers for more information."
	composeModifierNotForwardedMessage = "One branch passes the modifier parameter, but this branch does not. Pass it " +
		"to every branch whose composable accepts a modifier. See " +
		"https://slackhq.github.io/compose-lints/rules/#dont-re-use-modifiers for more information."
)

// SlotReusedRule ports compose-lints' SlotReused: a composable slot parameter invoked (or passed to another
// composable's slot) more than once on some execution path.
type SlotReusedRule struct {
	BaseRule
}

// ComposeModifierReusedRule ports compose-lints' ComposeModifierReused: the modifier parameter passed to more
// than one composable on an execution path, and a root if/when where one branch forwards it and another does not.
type ComposeModifierReusedRule struct {
	BaseRule
}

// composeLintsInlineHigherOrder are inline library functions (kotlin-stdlib higher-order functions and the lazy
// item builders) whose lambdas run during the call, as compose-lints' isPossiblyExecutedBy accepts for inline
// callees.
var composeLintsInlineHigherOrder = map[string]bool{
	"let": true, "run": true, "apply": true, "also": true, "with": true, "takeIf": true, "takeUnless": true,
	"repeat": true, "forEach": true, "forEachIndexed": true, "map": true, "mapIndexed": true, "mapNotNull": true,
	"filter": true, "filterNot": true, "flatMap": true, "any": true, "all": true, "none": true, "first": true,
	"firstOrNull": true, "last": true, "lastOrNull": true, "find": true, "fold": true, "reduce": true, "sumOf": true,
	"count": true, "onEach": true, "buildList": true, "buildString": true, "buildMap": true, "buildSet": true,
	"synchronized": true, "require": true, "check": true, "requireNotNull": true, "checkNotNull": true, "use": true,
	"getOrElse": true, "getOrPut": true, "ifEmpty": true, "ifBlank": true, "associate": true, "associateBy": true,
	"associateWith": true, "groupBy": true, "sortedBy": true, "sortedByDescending": true, "partition": true,
	"maxOf": true, "minOf": true, "maxByOrNull": true, "minByOrNull": true, "measureTimeMillis": true,
	// The list overloads of the lazy-layout item builders are inline in Compose foundation and Glance.
	"items": true, "itemsIndexed": true,
}

// composeLintsCfa is the simplified control-flow structure compose-lints builds (toSimplifiedCfaAst): tracked
// calls, switches for if/when, jumps for return and throw, blocks.
type composeLintsCfa struct {
	kind     int8 // cfaCall, cfaSwitch, cfaJump, cfaBlock
	call     uint32
	weight   int
	nodes    []*composeLintsCfa   // block
	branches [][]*composeLintsCfa // switch
	target   uint32               // jump: the lambda or function its block belongs to, 0 for the end
	owner    uint32               // block: the lambda or function node it is the body of, or 0
}

const (
	cfaCall int8 = iota
	cfaSwitch
	cfaJump
	cfaBlock
)

// composeLintsCfaBuilder walks a function body the way compose-lints' UAST visitor does.
type composeLintsCfaBuilder struct {
	file      *scanner.File
	fn        uint32
	weight    func(call uint32) int           // >0 for a tracked call
	visitable func(lambda, owner uint32) bool // whether a lambda argument runs during its owner call
	loops     func(body []*composeLintsCfa)   // called with the nodes of each loop body
}

func (b *composeLintsCfaBuilder) build(body uint32) *composeLintsCfa {
	root := &composeLintsCfa{kind: cfaBlock, owner: b.fn}
	b.visit(body, &root.nodes)
	return root
}

func (b *composeLintsCfaBuilder) visit(n uint32, out *[]*composeLintsCfa) {
	file := b.file
	switch file.FlatType(n) {
	case "if_expression":
		var bodies []uint32
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "control_structure_body" {
				bodies = append(bodies, child)
			}
		}
		sw := &composeLintsCfa{kind: cfaSwitch}
		for i := 0; i < 2; i++ {
			var branch []*composeLintsCfa
			if i < len(bodies) {
				b.visit(bodies[i], &branch)
			}
			sw.branches = append(sw.branches, branch)
		}
		*out = append(*out, sw)
		return
	case "when_expression":
		sw := &composeLintsCfa{kind: cfaSwitch}
		for entry := file.FlatFirstChild(n); entry != 0; entry = file.FlatNextSib(entry) {
			if file.FlatType(entry) != "when_entry" {
				continue
			}
			var branch []*composeLintsCfa
			if body, ok := file.FlatFindChild(entry, "control_structure_body"); ok {
				b.visit(body, &branch)
			}
			sw.branches = append(sw.branches, branch)
		}
		sw.branches = append(sw.branches, nil) // the "branch" that skips the when
		*out = append(*out, sw)
		return
	case "jump_expression":
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			b.visit(child, out)
		}
		text := file.FlatNodeText(n)
		switch {
		case strings.HasPrefix(text, "throw"):
			*out = append(*out, &composeLintsCfa{kind: cfaJump})
		case strings.HasPrefix(text, "return"):
			*out = append(*out, &composeLintsCfa{kind: cfaJump, target: b.returnTarget(n)})
		}
		return
	case "lambda_literal":
		owner := composeLintsLambdaOwner(file, n)
		if owner == 0 || !b.visitable(n, owner) {
			return
		}
		block := &composeLintsCfa{kind: cfaBlock, owner: n}
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			b.visit(child, &block.nodes)
		}
		*out = append(*out, block)
		return
	case "for_statement", "while_statement", "do_while_statement":
		var body []*composeLintsCfa
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			b.visit(child, &body)
		}
		if b.loops != nil {
			b.loops(body)
		}
		*out = append(*out, &composeLintsCfa{kind: cfaBlock, nodes: body})
		return
	case "call_expression":
		if w := b.weight(n); w > 0 {
			*out = append(*out, &composeLintsCfa{kind: cfaCall, call: n, weight: w})
		}
		callee := file.FlatFirstChild(n)
		if file.FlatType(callee) == "call_expression" {
			// f<T>(x) { }: tree-sitter nests the call; it is one call, so visit the inner one's children only.
			for child := file.FlatFirstChild(callee); child != 0; child = file.FlatNextSib(child) {
				b.visit(child, out)
			}
			for child := file.FlatNextSib(callee); child != 0; child = file.FlatNextSib(child) {
				b.visit(child, out)
			}
			return
		}
	case "function_body", "statements", "control_structure_body":
		block := &composeLintsCfa{kind: cfaBlock}
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			b.visit(child, &block.nodes)
		}
		*out = append(*out, block)
		return
	}
	for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
		b.visit(child, out)
	}
}

// returnTarget returns the lambda a labeled `return@label` targets, or 0 for a return from the function.
func (b *composeLintsCfaBuilder) returnTarget(jump uint32) uint32 {
	file := b.file
	text := file.FlatNodeText(jump)
	if !strings.HasPrefix(text, "return@") {
		return 0
	}
	label := strings.TrimPrefix(text, "return@")
	if i := strings.IndexFunc(label, func(r rune) bool {
		return r != '_' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
	}); i >= 0 {
		label = label[:i]
	}
	for p, ok := file.FlatParent(jump); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" {
			return 0
		}
		if file.FlatType(p) != "lambda_literal" {
			continue
		}
		if owner := composeLintsLambdaOwner(file, p); owner != 0 && composeLintsCalleeName(file, owner) == label {
			return p
		}
		if parent, ok := file.FlatParent(p); ok && file.FlatType(parent) == "annotated_lambda" {
			if l, ok := file.FlatFindChild(parent, "label"); ok && strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(l)), "@") == label {
				return p
			}
		}
	}
	return 0
}

// composeLintsLambdaOwner returns the call a lambda literal is an argument of (trailing or in parentheses), or 0.
func composeLintsLambdaOwner(file *scanner.File, lambda uint32) uint32 {
	p, ok := file.FlatParent(lambda)
	for ok && file.FlatType(p) == "parenthesized_expression" {
		p, ok = file.FlatParent(p)
	}
	if !ok {
		return 0
	}
	switch file.FlatType(p) {
	case "annotated_lambda":
		suffix, ok := file.FlatParent(p)
		if !ok || file.FlatType(suffix) != "call_suffix" {
			return 0
		}
		call, _ := file.FlatParent(suffix)
		return call
	case "value_argument":
		return composeLintsArgCall(file, p)
	}
	return 0
}

// composeLintsCfaGraph is compose-lints' CodeFlowGraph: tracked call nodes and switch nodes linked toward the end.
type composeLintsCfaGraph struct {
	weight []int
	call   []uint32
	succ   [][]int
	start  int
}

func composeLintsBuildGraph(root *composeLintsCfa) *composeLintsCfaGraph {
	g := &composeLintsCfaGraph{}
	add := func(weight int, call uint32) int {
		g.weight = append(g.weight, weight)
		g.call = append(g.call, call)
		g.succ = append(g.succ, nil)
		return len(g.weight) - 1
	}
	end := add(0, 0)
	exits := map[uint32]int{} // block owner -> exit node
	following := end
	var visit func(n *composeLintsCfa)
	visit = func(n *composeLintsCfa) {
		switch n.kind {
		case cfaCall:
			node := add(n.weight, n.call)
			g.succ[node] = append(g.succ[node], following)
			following = node
		case cfaSwitch:
			node := add(0, 0)
			join := following
			for _, branch := range n.branches {
				following = join
				for i := len(branch) - 1; i >= 0; i-- {
					visit(branch[i])
				}
				g.succ[node] = append(g.succ[node], following)
			}
			following = node
		case cfaJump:
			if exit, ok := exits[n.target]; ok && n.target != 0 {
				following = exit
			} else {
				following = end
			}
		case cfaBlock:
			if n.owner != 0 {
				exits[n.owner] = following
			}
			for i := len(n.nodes) - 1; i >= 0; i-- {
				visit(n.nodes[i])
			}
		}
	}
	visit(root)
	g.start = add(0, 0)
	g.succ[g.start] = append(g.succ[g.start], following)
	return g
}

// heavyCalls returns the tracked calls lying on a start-to-end path whose tracked weight is at least 2, and the
// largest path weight. The graph is acyclic (jumps only go to later block exits).
func (g *composeLintsCfaGraph) heavyCalls() (map[uint32]bool, int) {
	n := len(g.weight)
	suffix := make([]int, n)
	done := make([]bool, n)
	var down func(i int) int
	down = func(i int) int {
		if done[i] {
			return suffix[i]
		}
		done[i] = true
		best := 0
		for _, s := range g.succ[i] {
			if v := down(s); v > best {
				best = v
			}
		}
		suffix[i] = g.weight[i] + best
		return suffix[i]
	}
	down(g.start)
	// prefix: max weight from start to the node, inclusive, by relaxation in reverse post-order.
	order := make([]int, 0, n)
	seen := make([]bool, n)
	var post func(i int)
	post = func(i int) {
		seen[i] = true
		for _, s := range g.succ[i] {
			if !seen[s] {
				post(s)
			}
		}
		order = append(order, i)
	}
	post(g.start)
	prefix := make([]int, n)
	for i := range prefix {
		prefix[i] = -1
	}
	prefix[g.start] = g.weight[g.start]
	for k := len(order) - 1; k >= 0; k-- {
		i := order[k]
		if prefix[i] < 0 {
			continue
		}
		for _, s := range g.succ[i] {
			if v := prefix[i] + g.weight[s]; v > prefix[s] {
				prefix[s] = v
			}
		}
	}
	heavy := map[uint32]bool{}
	for i := 0; i < n; i++ {
		if g.call[i] != 0 && prefix[i] >= 0 && prefix[i]+suffix[i]-g.weight[i] >= 2 {
			heavy[g.call[i]] = true
		}
	}
	return heavy, suffix[g.start]
}

// composeLintsProjectFunctions resolves a callee name to the project's function declarations.
type composeLintsProjectFunctions struct {
	index *scanner.CodeIndex
	files map[string]*scanner.File
	nodes map[*scanner.File]map[int]uint32
}

func newComposeLintsProjectFunctions(index *scanner.CodeIndex) *composeLintsProjectFunctions {
	files := make(map[string]*scanner.File, len(index.Files))
	for _, f := range index.Files {
		if f != nil {
			files[f.Path] = f
		}
	}
	return &composeLintsProjectFunctions{index: index, files: files, nodes: map[*scanner.File]map[int]uint32{}}
}

type composeLintsFunctionRef struct {
	file *scanner.File
	fn   uint32
}

func (p *composeLintsProjectFunctions) named(name string) []composeLintsFunctionRef {
	var out []composeLintsFunctionRef
	for _, sym := range p.index.SymbolsNamed(name) {
		if sym.Kind != "function" {
			continue
		}
		file := p.files[sym.File]
		if file == nil || file.FlatTree == nil {
			continue
		}
		byStart, ok := p.nodes[file]
		if !ok {
			byStart = map[int]uint32{}
			file.FlatWalkNodes(0, "function_declaration", func(fn uint32) { byStart[int(file.FlatStartByte(fn))] = fn })
			p.nodes[file] = byStart
		}
		if fn := byStart[sym.StartByte]; fn != 0 {
			out = append(out, composeLintsFunctionRef{file: file, fn: fn})
		}
	}
	return out
}

// composeLintsCallArguments returns a call's value arguments (including those tree-sitter nests in an inner call
// for `f<T>(x) { }`) and its trailing lambdas.
func composeLintsCallArguments(file *scanner.File, call uint32) (args, trailing []uint32) {
	collect := func(c uint32) {
		suffix, ok := file.FlatFindChild(c, "call_suffix")
		if !ok {
			return
		}
		if va, ok := file.FlatFindChild(suffix, "value_arguments"); ok {
			for a := file.FlatFirstChild(va); a != 0; a = file.FlatNextSib(a) {
				if file.FlatType(a) == "value_argument" {
					args = append(args, a)
				}
			}
		}
		if al, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
			if l, ok := file.FlatFindChild(al, "lambda_literal"); ok {
				trailing = append(trailing, l)
			}
		}
	}
	if callee := file.FlatFirstChild(call); file.FlatType(callee) == "call_expression" {
		collect(callee)
	}
	collect(call)
	return args, trailing
}

// composeLintsArgumentParam maps an argument expression of a call to the callee parameter it binds (by name, by
// position, or as the trailing lambda), or 0.
func composeLintsArgumentParam(file *scanner.File, call, expr uint32, target composeLintsFunctionRef) uint32 {
	params := composeLintsParams(target.file, target.fn)
	args, trailing := composeLintsCallArguments(file, call)
	for _, l := range trailing {
		if l == expr && len(params) > 0 {
			return params[len(params)-1].node
		}
	}
	for i, a := range args {
		value := file.FlatFirstChild(a)
		name := ""
		if value != 0 && file.FlatType(value) == "simple_identifier" {
			if eq := file.FlatNextSib(value); eq != 0 && file.FlatType(eq) == "=" {
				name = file.FlatNodeText(value)
				value = file.FlatNextSib(eq)
			}
		}
		if composeLintsUnwrapParens(file, value) != expr && value != expr {
			continue
		}
		if name != "" {
			for _, p := range params {
				if target.file.FlatChildTextOrEmpty(p.node, "simple_identifier") == name {
					return p.node
				}
			}
			return 0
		}
		if i < len(params) {
			return params[i].node
		}
	}
	return 0
}

// composeLintsSlotTarget reports whether expr, an argument or lambda of call, binds a @Composable function-typed
// parameter of a project composable Unit function. Library callees never count: the Lint CLI does not see the
// annotations on their parameter types.
func composeLintsSlotTarget(file *scanner.File, call, expr uint32, project *composeLintsProjectFunctions) bool {
	name := composeLintsCalleeName(file, call)
	if name == "" {
		return false
	}
	for _, target := range project.named(name) {
		if !acrFunctionIsComposable(target.file, target.fn) || acrFunctionReturnKind(target.file, target.fn) != acrReturnUnit {
			continue
		}
		if param := composeLintsArgumentParam(file, call, expr, target); param != 0 && acrComposableFunctionType(target.file, param) != 0 {
			return true
		}
	}
	return false
}

func (r *SlotReusedRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	var project *composeLintsProjectFunctions
	for _, file := range ctx.CodeIndex.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("Composable")) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) {
				return
			}
			body, _ := file.FlatFindChild(fn, "function_body")
			if body == 0 {
				return
			}
			for _, p := range composeLintsParams(file, fn) {
				if acrComposableFunctionType(file, p.node) == 0 {
					continue
				}
				if project == nil {
					project = newComposeLintsProjectFunctions(ctx.CodeIndex)
				}
				slot := strings.Trim(file.FlatChildTextOrEmpty(p.node, "simple_identifier"), "`")
				if composeLintsReusesSlot(file, fn, body, slot, project) {
					line, col := composeLintsByteLineCol(file, p.start)
					ctx.Emit(r.Finding(file, line, col, composeSlotReusedMessage))
				}
			}
		})
	}
}

// composeLintsReusesSlot reports whether some execution path of the function uses the slot twice.
func composeLintsReusesSlot(file *scanner.File, fn, body uint32, slot string, project *composeLintsProjectFunctions) bool {
	repeated := false
	b := &composeLintsCfaBuilder{file: file, fn: fn,
		weight: func(call uint32) int { return composeLintsSlotUses(file, call, slot, project) },
		visitable: func(lambda, owner uint32) bool {
			return composeLintsSlotTarget(file, owner, lambda, project)
		},
		loops: func(nodes []*composeLintsCfa) {
			if composeLintsCfaWeight(nodes) > 0 {
				repeated = true // a loop can run its body again
			}
		},
	}
	g := composeLintsBuildGraph(b.build(body))
	if repeated {
		return true
	}
	_, most := g.heavyCalls()
	return most >= 2
}

func composeLintsCfaWeight(nodes []*composeLintsCfa) int {
	total := 0
	for _, n := range nodes {
		total += n.weight + composeLintsCfaWeight(n.nodes)
		for _, b := range n.branches {
			total += composeLintsCfaWeight(b)
		}
	}
	return total
}

// composeLintsSlotUses counts the uses of slot by a call: invoking it (`slot()`, `slot.invoke()`,
// `slot?.invoke()`), plus each project composable slot argument that receives it.
func composeLintsSlotUses(file *scanner.File, call uint32, slot string, project *composeLintsProjectFunctions) int {
	uses := 0
	callee := file.FlatFirstChild(call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		if file.FlatNodeTextEquals(callee, slot) && composeLintsNameIsParam(file, callee, slot) {
			uses++
		}
	case "navigation_expression":
		if recv := file.FlatFirstChild(callee); file.FlatType(recv) == "simple_identifier" && file.FlatNodeTextEquals(recv, slot) &&
			composeLintsCalleeName(file, call) == "invoke" && composeLintsNameIsParam(file, recv, slot) {
			uses++
		}
	}
	args, _ := composeLintsCallArguments(file, call)
	for _, a := range args {
		value := file.FlatFirstChild(a)
		if value != 0 && file.FlatType(value) == "simple_identifier" {
			if eq := file.FlatNextSib(value); eq != 0 && file.FlatType(eq) == "=" {
				value = file.FlatNextSib(eq)
			}
		}
		if value != 0 && file.FlatType(value) == "simple_identifier" && file.FlatNodeTextEquals(value, slot) &&
			composeLintsNameIsParam(file, value, slot) && composeLintsSlotTarget(file, call, value, project) {
			uses++
		}
	}
	return uses
}

// composeLintsNameIsParam reports whether a name at node still refers to the function parameter: no lambda
// parameter or earlier local in between rebinds it.
func composeLintsNameIsParam(file *scanner.File, node uint32, name string) bool {
	return !composeLintsNameShadowed(file, node, name)
}

func (r *ComposeModifierReusedRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	var project *composeLintsProjectFunctions
	var usage *composeLintsUsageClassifier
	for _, file := range ctx.CodeIndex.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte("Modifier")) || !bytes.Contains(file.Content, []byte("Composable")) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) {
				return
			}
			modifier := composeLintsModifierParam(file, fn)
			body, _ := file.FlatFindChild(fn, "function_body")
			if modifier == 0 || body == 0 {
				return
			}
			if project == nil {
				project = newComposeLintsProjectFunctions(ctx.CodeIndex)
				usage = newComposeLintsUsageClassifier(ctx.CodeIndex)
			}
			name := strings.Trim(file.FlatChildTextOrEmpty(modifier, "simple_identifier"), "`")
			refs := composeLintsModifierRefs(file, body, name)
			slots := map[string]bool{}
			for _, p := range composeLintsParams(file, fn) {
				if acrComposableFunctionType(file, p.node) != 0 {
					slots[strings.Trim(file.FlatChildTextOrEmpty(p.node, "simple_identifier"), "`")] = true
				}
			}
			b := &composeLintsCfaBuilder{file: file, fn: fn,
				weight: func(call uint32) int {
					if len(composeLintsModifierArgRefs(file, call, refs)) == 0 {
						return 0
					}
					if composeLintsIsSlotInvocation(file, call, slots) || composeLintsCallIsComposable(file, call, usage) {
						return 1
					}
					return 0
				},
				visitable: func(_, owner uint32) bool {
					return composeLintsCallIsComposable(file, owner, usage) || composeLintsCallIsInline(file, owner, project)
				},
			}
			heavy, _ := composeLintsBuildGraph(b.build(body)).heavyCalls()
			reported := map[uint32]bool{}
			file.FlatWalkNodes(body, "call_expression", func(call uint32) {
				if !heavy[call] {
					return
				}
				for _, ref := range composeLintsModifierArgRefs(file, call, refs) {
					if !reported[ref] {
						reported[ref] = true
						ctx.Emit(r.Finding(file, file.FlatRow(ref)+1, file.FlatCol(ref)+1, composeModifierReusedMessage))
					}
				}
			})
			if acrFunctionReturnKind(file, fn) == acrReturnUnit {
				for _, at := range composeLintsBranchesMissingModifier(file, body, name, project, usage) {
					ctx.Emit(r.Finding(file, file.FlatRow(at)+1, file.FlatCol(at)+1, composeModifierNotForwardedMessage))
				}
			}
		})
	}
}

// composeLintsModifierParam picks the modifier parameter compose-lints tracks: the Modifier-typed parameter named
// `modifier`, else the first Modifier-typed one.
func composeLintsModifierParam(file *scanner.File, fn uint32) uint32 {
	var first uint32
	for _, p := range composeLintsParams(file, fn) {
		if !composeLintsParamIsModifier(file, p.node) {
			continue
		}
		if strings.Trim(file.FlatChildTextOrEmpty(p.node, "simple_identifier"), "`") == "modifier" {
			return p.node
		}
		if first == 0 {
			first = p.node
		}
	}
	return first
}

// composeLintsModifierRefs returns the names that carry the modifier: the parameter and, like Lint's data flow,
// locals initialized from an expression that flows from one of them (`val m = modifier.fillMaxWidth()`).
func composeLintsModifierRefs(file *scanner.File, body uint32, name string) map[string]bool {
	refs := map[string]bool{name: true}
	file.FlatWalkNodes(body, "property_declaration", func(prop uint32) {
		v, ok := file.FlatFindChild(prop, "variable_declaration")
		if !ok {
			return
		}
		local := file.FlatChildTextOrEmpty(v, "simple_identifier")
		if init := composeLintsPropertyInitializer(file, prop); local != "" && init != 0 && composeLintsFlowsFrom(file, init, refs) {
			refs[local] = true
		}
	})
	return refs
}

// composeLintsFlowsFrom mirrors compose-lints' flowsFrom: a reference to a tracked name, the last statement of a
// block, either branch of an if or when, or a call whose receiver or non-lambda argument flows from one.
func composeLintsFlowsFrom(file *scanner.File, expr uint32, refs map[string]bool) bool {
	expr = composeLintsUnwrapParens(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier":
		return refs[file.FlatNodeText(expr)]
	case "if_expression", "when_expression":
		found := false
		file.FlatWalkNodes(expr, "control_structure_body", func(b uint32) {
			if !found {
				last := flatLastNamedChild(file, b)
				if file.FlatType(last) == "statements" {
					last = flatLastNamedChild(file, last)
				}
				found = last != 0 && composeLintsFlowsFrom(file, last, refs)
			}
		})
		return found
	case "navigation_expression":
		return composeLintsFlowsFrom(file, file.FlatFirstChild(expr), refs)
	case "call_expression":
		callee := file.FlatFirstChild(expr)
		if file.FlatType(callee) == "navigation_expression" && composeLintsFlowsFrom(file, file.FlatFirstChild(callee), refs) {
			return true
		}
		args, _ := composeLintsCallArguments(file, expr)
		for _, a := range args {
			value := flatLastNamedChild(file, a)
			if value != 0 && file.FlatType(value) != "lambda_literal" && composeLintsFlowsFrom(file, value, refs) {
				return true
			}
		}
	}
	return false
}

// composeLintsModifierArgRefs returns the references to tracked modifier names in a call's value arguments,
// skipping lambdas (compose-lints' visitReferencesSkipLambdas).
func composeLintsModifierArgRefs(file *scanner.File, call uint32, refs map[string]bool) []uint32 {
	var out []uint32
	args, _ := composeLintsCallArguments(file, call)
	for _, a := range args {
		var walk func(n uint32)
		walk = func(n uint32) {
			for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
				switch file.FlatType(c) {
				case "lambda_literal", "annotated_lambda":
					continue
				case "simple_identifier":
					if refs[file.FlatNodeText(c)] {
						if next := file.FlatNextSib(c); next != 0 && file.FlatType(next) == "=" {
							continue // a named-argument label
						}
						if parent, ok := file.FlatParent(c); ok && file.FlatType(parent) == "navigation_suffix" {
							continue
						}
						out = append(out, c)
					}
				}
				walk(c)
			}
		}
		walk(a)
	}
	return out
}

func composeLintsIsSlotInvocation(file *scanner.File, call uint32, slots map[string]bool) bool {
	callee := file.FlatFirstChild(call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		return slots[file.FlatNodeText(callee)]
	case "navigation_expression":
		recv := file.FlatFirstChild(callee)
		return file.FlatType(recv) == "simple_identifier" && slots[file.FlatNodeText(recv)] && composeLintsCalleeName(file, call) == "invoke"
	}
	return false
}

// composeLintsCallIsComposable reports whether a call resolves to a composable (read-only or not).
func composeLintsCallIsComposable(file *scanner.File, call uint32, usage *composeLintsUsageClassifier) bool {
	call = composeLintsInnermostCall(file, call)
	scope := composeLintsScope{params: map[string]uint32{}}
	switch usage.callUsage(file, scope, call) {
	case composeLintsUsageOther, composeLintsUsageReadOnly:
		return true
	}
	return false
}

// composeLintsCallIsInline reports whether a call targets an inline function: a known kotlin-stdlib higher-order
// function or a project function declared inline.
func composeLintsCallIsInline(file *scanner.File, call uint32, project *composeLintsProjectFunctions) bool {
	name := composeLintsCalleeName(file, call)
	refs := project.named(name)
	if len(refs) == 0 {
		return composeLintsInlineHigherOrder[name]
	}
	for _, ref := range refs {
		if !acrFunctionHasModifier(ref.file, ref.fn, "inline") {
			return false
		}
	}
	return true
}

// composeLintsBranchesMissingModifier mirrors compose-lints' reportMissingBranchModifiers: for a root if/when
// whose branches call composables accepting a modifier, when one branch forwards the modifier, the callee of
// every branch call that does not.
func composeLintsBranchesMissingModifier(file *scanner.File, body uint32, name string,
	project *composeLintsProjectFunctions, usage *composeLintsUsageClassifier) []uint32 {
	decls := map[string]bool{name: true}
	file.FlatWalkNodes(body, "property_declaration", func(prop uint32) {
		if !composeLintsPropertyIsVal(file, prop) || !acrIsLocalDeclaration(file, prop) {
			return
		}
		v, _ := file.FlatFindChild(prop, "variable_declaration")
		local := file.FlatChildTextOrEmpty(v, "simple_identifier")
		if init := composeLintsPropertyInitializer(file, prop); init != 0 && composeLintsForwarding(file, init, decls) == composeLintsForwardYes {
			decls[local] = true
		}
	})
	cond := composeLintsRootConditional(file, body, usage)
	if cond == 0 {
		return nil
	}
	var calls []uint32
	composeLintsBranchCalls(file, cond, usage, &calls)
	type branch struct {
		at  uint32
		fwd int8
	}
	var branches []branch
	hasYes := false
	for _, call := range calls {
		if !composeLintsAcceptsModifier(file, call, project) {
			continue
		}
		fwd := composeLintsForwardNo
		args, _ := composeLintsCallArguments(file, call)
		for _, a := range args {
			value := flatLastNamedChild(file, a)
			if value == 0 || file.FlatType(value) == "lambda_literal" {
				continue
			}
			if f := composeLintsForwarding(file, value, decls); f == composeLintsForwardYes {
				fwd = composeLintsForwardYes
				break
			}
		}
		if fwd == composeLintsForwardYes {
			hasYes = true
		}
		at := file.FlatFirstChild(call)
		if file.FlatType(at) == "navigation_expression" {
			if s, ok := file.FlatFindChild(at, "navigation_suffix"); ok {
				if id, ok := file.FlatFindChild(s, "simple_identifier"); ok {
					at = id
				}
			}
		}
		branches = append(branches, branch{at: at, fwd: fwd})
	}
	if !hasYes {
		return nil
	}
	var out []uint32
	for _, b := range branches {
		if b.fwd == composeLintsForwardNo {
			out = append(out, b.at)
		}
	}
	return out
}

const (
	composeLintsForwardYes int8 = iota
	composeLintsForwardNo
	composeLintsForwardDoesNotComplete
)

// composeLintsForwarding mirrors compose-lints' forwardingFrom: whether an expression definitely evaluates to (or
// is built from) one of the modifier declarations.
func composeLintsForwarding(file *scanner.File, expr uint32, decls map[string]bool) int8 {
	expr = composeLintsUnwrapParens(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier":
		if decls[file.FlatNodeText(expr)] {
			return composeLintsForwardYes
		}
		return composeLintsForwardNo
	case "jump_expression":
		return composeLintsForwardDoesNotComplete
	case "if_expression", "when_expression":
		var results []int8
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			body := child
			if file.FlatType(child) == "when_entry" {
				body, _ = file.FlatFindChild(child, "control_structure_body")
			} else if file.FlatType(child) != "control_structure_body" {
				continue
			}
			last := flatLastNamedChild(file, body)
			if file.FlatType(last) == "statements" {
				last = flatLastNamedChild(file, last)
			}
			if last == 0 {
				results = append(results, composeLintsForwardNo)
				continue
			}
			results = append(results, composeLintsForwarding(file, last, decls))
		}
		if file.FlatType(expr) == "if_expression" && len(results) < 2 {
			results = append(results, composeLintsForwardNo)
		}
		completing := false
		all := true
		for _, r := range results {
			if r == composeLintsForwardDoesNotComplete {
				continue
			}
			completing = true
			if r != composeLintsForwardYes {
				all = false
			}
		}
		switch {
		case !completing:
			return composeLintsForwardDoesNotComplete
		case all:
			return composeLintsForwardYes
		}
		return composeLintsForwardNo
	case "navigation_expression":
		return composeLintsForwarding(file, file.FlatFirstChild(expr), decls)
	case "call_expression":
		result := composeLintsForwardNo
		callee := file.FlatFirstChild(expr)
		if file.FlatType(callee) == "navigation_expression" {
			if f := composeLintsForwarding(file, file.FlatFirstChild(callee), decls); f == composeLintsForwardDoesNotComplete {
				return f
			} else if f == composeLintsForwardYes {
				result = f
			}
		}
		args, _ := composeLintsCallArguments(file, expr)
		for _, a := range args {
			value := flatLastNamedChild(file, a)
			if value == 0 || file.FlatType(value) == "lambda_literal" {
				continue
			}
			switch composeLintsForwarding(file, value, decls) {
			case composeLintsForwardDoesNotComplete:
				return composeLintsForwardDoesNotComplete
			case composeLintsForwardYes:
				result = composeLintsForwardYes
			}
		}
		return result
	}
	return composeLintsForwardNo
}

// composeLintsRootConditional returns the if or when that is a function body's root: the expression body, or the
// last statement after setup statements that emit nothing.
func composeLintsRootConditional(file *scanner.File, body uint32, usage *composeLintsUsageClassifier) uint32 {
	var candidate uint32
	if first := file.FlatFirstChild(body); first != 0 && file.FlatType(first) == "=" {
		candidate = file.FlatNextSib(first)
	} else if stmts, ok := file.FlatFindChild(body, "statements"); ok {
		candidate = composeLintsLastAfterSetup(file, stmts, usage)
	}
	candidate = composeLintsUnwrapParens(file, candidate)
	if candidate != 0 && file.FlatType(candidate) == "jump_expression" && strings.HasPrefix(file.FlatNodeText(candidate), "return") {
		candidate = composeLintsUnwrapParens(file, flatLastNamedChild(file, candidate))
	}
	if t := file.FlatType(candidate); candidate != 0 && (t == "if_expression" || t == "when_expression") {
		return candidate
	}
	return 0
}

// composeLintsLastAfterSetup returns a block's last statement when every earlier one is safe setup.
func composeLintsLastAfterSetup(file *scanner.File, stmts uint32, usage *composeLintsUsageClassifier) uint32 {
	var list []uint32
	for s := file.FlatFirstChild(stmts); s != 0; s = file.FlatNextSib(s) {
		if file.FlatIsNamed(s) && !composeLintsIsComment(file, s) {
			list = append(list, s)
		}
	}
	if len(list) == 0 {
		return 0
	}
	for _, s := range list[:len(list)-1] {
		target := s
		if file.FlatType(s) == "property_declaration" {
			target = composeLintsPropertyInitializer(file, s)
			if target == 0 {
				return 0
			}
		}
		emits := false
		file.FlatWalkNodes(target, "call_expression", func(call uint32) {
			if !emits && composeLintsCallIsComposable(file, call, usage) {
				emits = true
			}
		})
		if emits {
			return 0
		}
	}
	return list[len(list)-1]
}

// composeLintsBranchCalls collects compose-lints' branchCalls: the direct call each branch of an if/when ends in.
func composeLintsBranchCalls(file *scanner.File, expr uint32, usage *composeLintsUsageClassifier, out *[]uint32) {
	expr = composeLintsUnwrapParens(file, expr)
	switch file.FlatType(expr) {
	case "control_structure_body":
		if stmts, ok := file.FlatFindChild(expr, "statements"); ok {
			if last := composeLintsLastAfterSetup(file, stmts, usage); last != 0 {
				composeLintsBranchCalls(file, last, usage, out)
			}
			return
		}
		composeLintsBranchCalls(file, flatLastNamedChild(file, expr), usage, out)
	case "jump_expression":
		if strings.HasPrefix(file.FlatNodeText(expr), "return") {
			composeLintsBranchCalls(file, flatLastNamedChild(file, expr), usage, out)
		}
	case "if_expression":
		for c := file.FlatFirstChild(expr); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "control_structure_body" {
				composeLintsBranchCalls(file, c, usage, out)
			}
		}
	case "when_expression":
		for c := file.FlatFirstChild(expr); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "when_entry" {
				if body, ok := file.FlatFindChild(c, "control_structure_body"); ok {
					composeLintsBranchCalls(file, body, usage, out)
				}
			}
		}
	case "call_expression":
		*out = append(*out, expr)
	}
}

// composeLintsAcceptsModifier reports whether a branch call targets a non-read-only Unit composable taking
// exactly one Modifier: every project overload of the name, or an imported library composable in the table.
func composeLintsAcceptsModifier(file *scanner.File, call uint32, project *composeLintsProjectFunctions) bool {
	name := composeLintsCalleeName(file, call)
	if name == "" {
		return false
	}
	if refs := project.named(name); len(refs) > 0 {
		for _, ref := range refs {
			if composeLintsDeclUsage(ref.file, ref.fn) != composeLintsUsageOther || acrFunctionReturnKind(ref.file, ref.fn) != acrReturnUnit {
				return false
			}
			count := 0
			for _, p := range composeLintsParams(ref.file, ref.fn) {
				if composeLintsParamIsModifier(ref.file, p.node) {
					count++
				}
			}
			if count != 1 {
				return false
			}
		}
		return true
	}
	facts := fileFactsCache().Imports(file)
	if facts == nil {
		return false
	}
	return composeLintsLibraryModifierComposables[facts.Aliases[name]]
}

// composeLintsInnermostCall returns the call holding the callee: tree-sitter parses `f(args) { }` as a call whose
// callee is the call `f(args)`.
func composeLintsInnermostCall(file *scanner.File, call uint32) uint32 {
	for {
		callee := file.FlatFirstChild(call)
		if callee == 0 || file.FlatType(callee) != "call_expression" {
			return call
		}
		call = callee
	}
}
