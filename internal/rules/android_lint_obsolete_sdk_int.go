package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ObsoleteSdkInt mirrors the source half of Lint's ObsoleteSdkInt (ApiDetector): an `SDK_INT` comparison whose result
// the module's minSdk, an enclosing version check, or an enclosing @RequiresApi/@TargetApi already decides, and an
// @RequiresApi/@TargetApi no higher than the minSdk or the annotation of the enclosing declaration.

// ObsoleteSdkIntRule reports obsolete SDK_INT comparisons and API annotations in Kotlin and Java.
type ObsoleteSdkIntRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the module's minSdk comes from its build file or manifest.
func (r *ObsoleteSdkIntRule) Confidence() float64 { return api.ConfidenceMedium }

// sdkInf is the open upper bound of an SDK_INT range.
const sdkInf = 1 << 20

// sdkMaxLevel is the first level treated as a preview (Build.VERSION_CODES.CUR_DEVELOPMENT is 10000).
const sdkMaxLevel = 10000

const sdkIntFQN = "android.os.Build.VERSION.SDK_INT"

// sdkRange is an inclusive range of SDK_INT values.
type sdkRange struct{ lo, hi int }

// sdkSet is a sorted list of disjoint SDK_INT ranges, the values a version check allows.
type sdkSet []sdkRange

var sdkAll = sdkSet{{0, sdkInf}}

func sdkFrom(lo int) sdkSet { return sdkSet{{lo, sdkInf}} }

func (s sdkSet) isEmpty() bool { return len(s) == 0 }

// openEnded reports whether the set has no upper bound.
func (s sdkSet) openEnded() bool { return len(s) > 0 && s[len(s)-1].hi == sdkInf }

func (s sdkSet) equal(o sdkSet) bool {
	if len(s) != len(o) {
		return false
	}
	for i := range s {
		if s[i] != o[i] {
			return false
		}
	}
	return true
}

func (s sdkSet) and(o sdkSet) sdkSet {
	var out sdkSet
	for i, j := 0, 0; i < len(s) && j < len(o); {
		lo, hi := max(s[i].lo, o[j].lo), min(s[i].hi, o[j].hi)
		if lo <= hi {
			out = append(out, sdkRange{lo, hi})
		}
		if s[i].hi < o[j].hi {
			i++
		} else {
			j++
		}
	}
	return out
}

func (s sdkSet) not() sdkSet {
	var out sdkSet
	next := 0
	for _, r := range s {
		if r.lo > next {
			out = append(out, sdkRange{next, r.lo - 1})
		}
		next = r.hi + 1
	}
	if next <= sdkInf {
		out = append(out, sdkRange{next, sdkInf})
	}
	return out
}

func (s sdkSet) or(o sdkSet) sdkSet { return s.not().and(o.not()).not() }

// describe renders a set the way Lint prints a constraint: "= 23", "≥ 21 and < 23" or "= 21 or = 23".
func (s sdkSet) describe() string {
	parts := make([]string, len(s))
	for i, r := range s {
		switch {
		case r.lo == r.hi:
			parts[i] = fmt.Sprintf("= %d", r.lo)
		case r.hi == sdkInf:
			parts[i] = fmt.Sprintf("≥ %d", r.lo)
		default:
			parts[i] = fmt.Sprintf("≥ %d and < %d", r.lo, r.hi+1)
		}
	}
	return strings.Join(parts, " or ")
}

// sdkVersionCodes are the Build.VERSION_CODES constants keyed for lintEvalIntLocals.
var sdkVersionCodes = func() map[string]int64 {
	out := map[string]int64{}
	for i, name := range []string{"BASE", "BASE_1_1", "CUPCAKE", "DONUT", "ECLAIR", "ECLAIR_0_1", "ECLAIR_MR1", "FROYO",
		"GINGERBREAD", "GINGERBREAD_MR1", "HONEYCOMB", "HONEYCOMB_MR1", "HONEYCOMB_MR2", "ICE_CREAM_SANDWICH",
		"ICE_CREAM_SANDWICH_MR1", "JELLY_BEAN", "JELLY_BEAN_MR1", "JELLY_BEAN_MR2", "KITKAT", "KITKAT_WATCH", "LOLLIPOP",
		"LOLLIPOP_MR1", "M", "N", "N_MR1", "O", "O_MR1", "P", "Q", "R", "S", "S_V2", "TIRAMISU", "UPSIDE_DOWN_CAKE",
		"VANILLA_ICE_CREAM", "BAKLAVA"} {
		out["android.os.Build.VERSION_CODES."+name] = int64(i + 1)
	}
	return out
}()

// ---- syntax adapters over the Kotlin and Java trees ----

// sdkChildren returns the child nodes of idx in source order, comments excluded.
func sdkChildren(file *scanner.File, idx uint32) []uint32 {
	var out []uint32
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "line_comment", "multiline_comment", "block_comment":
		default:
			out = append(out, c)
		}
	}
	return out
}

// sdkUnwrap steps through parentheses.
func sdkUnwrap(file *scanner.File, idx uint32) uint32 {
	for file.FlatType(idx) == "parenthesized_expression" {
		inner := uint32(0)
		for _, c := range sdkChildren(file, idx) {
			if t := file.FlatType(c); t != "(" && t != ")" {
				inner = c
				break
			}
		}
		if inner == 0 {
			break
		}
		idx = inner
	}
	return idx
}

// sdkBinary splits a binary-shaped node into its operands and operator token.
func sdkBinary(file *scanner.File, idx uint32) (left uint32, op string, right uint32, ok bool) {
	switch file.FlatType(idx) {
	case "comparison_expression", "equality_expression", "check_expression", "conjunction_expression",
		"disjunction_expression", "binary_expression":
	default:
		return 0, "", 0, false
	}
	kids := sdkChildren(file, idx)
	if len(kids) != 3 {
		return 0, "", 0, false
	}
	return kids[0], file.FlatType(kids[1]), kids[2], true
}

// sdkIsSdkInt reports whether idx names android.os.Build.VERSION.SDK_INT.
func sdkIsSdkInt(file *scanner.File, idx uint32) bool {
	idx = sdkUnwrap(file, idx)
	switch file.FlatType(idx) {
	case "simple_identifier", "identifier", "navigation_expression", "field_access":
		text := strings.Join(strings.Fields(file.FlatNodeText(idx)), "")
		return strings.HasSuffix(text, "SDK_INT") && lintTypeRefMatches(file, text, sdkIntFQN)
	}
	return false
}

// sdkLiteralTypes are the integer literal node types of the Kotlin and Java grammars.
var sdkLiteralTypes = map[string]bool{
	"integer_literal": true, "decimal_integer_literal": true, "hex_integer_literal": true, "hex_literal": true,
}

// sdkConstant evaluates an operand the way Lint's version-check detection does: an Int literal, or a reference to a
// constant such as Build.VERSION_CODES.KITKAT or a never-reassigned variable. Arithmetic on literals does not count.
func sdkConstant(file *scanner.File, idx uint32) (int, bool) {
	idx = sdkUnwrap(file, idx)
	var v int64
	var ok bool
	switch t := file.FlatType(idx); {
	case sdkLiteralTypes[t]:
		text := file.FlatNodeText(idx)
		if strings.HasSuffix(text, "L") || strings.HasSuffix(text, "l") {
			return 0, false
		}
		v, ok = parseKotlinIntegerLiteral(text)
	case t == "simple_identifier" || t == "identifier" || t == "navigation_expression" || t == "field_access":
		v, ok = lintEvalIntLocals(file, idx, sdkVersionCodes)
	case t == "call_expression":
		// Lint folds `21.toInt()` to 21.
		if text := file.FlatNodeText(idx); strings.HasSuffix(text, ".toInt()") {
			v, ok = parseKotlinIntegerLiteral(strings.TrimSuffix(text, ".toInt()"))
		}
	}
	if !ok || v < 0 || v >= sdkMaxLevel {
		return 0, false
	}
	return int(v), true
}

// sdkRangeOperand reads a Kotlin `a..b`, `a..<b` or `a until b` operand into the values it spans.
func sdkRangeOperand(file *scanner.File, idx uint32) (sdkSet, bool) {
	idx = sdkUnwrap(file, idx)
	kids := sdkChildren(file, idx)
	if len(kids) != 3 {
		return nil, false
	}
	lo, okLo := sdkConstant(file, kids[0])
	hi, okHi := sdkConstant(file, kids[2])
	if !okLo || !okHi {
		return nil, false
	}
	switch op := strings.TrimSpace(file.FlatNodeText(kids[1])); {
	case file.FlatType(idx) == "range_expression" && op == "..":
	case file.FlatType(idx) == "range_expression" && op == "..<", file.FlatType(idx) == "infix_expression" && op == "until":
		hi--
	default:
		return nil, false
	}
	if hi < lo {
		return nil, false
	}
	return sdkSet{{lo, hi}}, true
}

// sdkCompareSet returns the SDK_INT values for which the comparison at idx is true.
func sdkCompareSet(file *scanner.File, idx uint32) (sdkSet, bool) {
	left, op, right, ok := sdkBinary(file, idx)
	if !ok {
		return nil, false
	}
	if op == "in" && file.Language != scanner.LangJava {
		if !sdkIsSdkInt(file, left) {
			return nil, false
		}
		return sdkRangeOperand(file, right)
	}
	if !sdkIsSdkInt(file, left) {
		// `19 <= SDK_INT` reads as `SDK_INT >= 19`.
		if !sdkIsSdkInt(file, right) {
			return nil, false
		}
		right = left
		switch op {
		case "<":
			op = ">"
		case "<=":
			op = ">="
		case ">":
			op = "<"
		case ">=":
			op = "<="
		}
	}
	c, ok := sdkConstant(file, right)
	// Lint treats `SDK_INT < 1` as always true and `SDK_INT > 0` as no check, so both are left alone.
	if !ok || c < 1 || (c == 1 && op == "<") {
		return nil, false
	}
	switch op {
	case ">=":
		return sdkFrom(c), true
	case ">":
		return sdkFrom(c + 1), true
	case "<=":
		return sdkSet{{0, c}}, true
	case "<":
		return sdkSet{{0, c - 1}}, true
	case "==":
		return sdkSet{{c, c}}, true
	case "!=":
		return sdkSet{{c, c}}.not(), true
	}
	return nil, false
}

// sdkCons is Lint's ApiConstraint for a condition: the SDK_INT values that make it true, and whether `!` may
// negate it. An `&&` with an operand that is not a version check yields a constraint that cannot be negated.
type sdkCons struct {
	set sdkSet
	neg bool
	// none marks Lint's ApiConstraint.NONE, which intersecting with an empty constraint produces. It reads as no
	// constraint at all once the result is used.
	none bool
}

// sdkMax intersects two constraints, ignoring a missing one (Lint's ApiConstraint.maxNullable).
func sdkMax(a, b *sdkCons) *sdkCons {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	if a.set.isEmpty() || b.set.isEmpty() {
		return &sdkCons{neg: true, none: true}
	}
	return &sdkCons{set: a.set.and(b.set), neg: true}
}

// not negates the constraint. Lint turns a constraint that cannot be negated into an empty one.
func (c *sdkCons) not() *sdkCons {
	switch {
	case c == nil:
		return nil
	case c.none:
		return &sdkCons{set: sdkAll, neg: true}
	case !c.neg:
		return &sdkCons{}
	}
	return &sdkCons{set: c.set.not(), neg: true}
}

func (c *sdkCons) nonNegatable() *sdkCons {
	if c == nil {
		return nil
	}
	return &sdkCons{set: c.set, neg: false, none: c.none}
}

// sdkLogical splits `a && b` or `a || b` into its operator and operands.
func sdkLogical(file *scanner.File, idx uint32) (op string, left, right uint32) {
	switch file.FlatType(idx) {
	case "conjunction_expression", "disjunction_expression", "binary_expression":
		l, o, r, ok := sdkBinary(file, idx)
		if ok && (o == "&&" || o == "||") {
			return o, l, r
		}
	}
	return "", 0, 0
}

// sdkChain reports whether idx is an `a && b && c` chain, which Lint's UAST holds as one polyadic expression rather
// than a binary one.
func sdkChain(file *scanner.File, idx uint32) bool {
	op, left, _ := sdkLogical(file, idx)
	if op == "" {
		return false
	}
	lop, _, _ := sdkLogical(file, left)
	return lop == op
}

// sdkNotOperand returns the operand of a logical negation, or 0.
func sdkNotOperand(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "prefix_expression", "unary_expression":
		kids := sdkChildren(file, idx)
		if len(kids) == 2 && file.FlatType(kids[0]) == "!" {
			return kids[1]
		}
	}
	return 0
}

func sdkIsLogicalType(t string) bool {
	return t == "conjunction_expression" || t == "disjunction_expression"
}

// sdkMisgroupedNot returns X when idx is the Kotlin grammar's reading of `!a || b` as `!(a || b)`, where the `!`
// really applies to the first operand only. It returns 0 for any other negation.
func sdkMisgroupedNot(file *scanner.File, idx uint32) uint32 {
	if file.Language == scanner.LangJava {
		return 0
	}
	if inner := sdkNotOperand(file, idx); inner != 0 && sdkIsLogicalType(file.FlatType(inner)) {
		return inner
	}
	return 0
}

// sdkLeafNegated reports whether idx is the first operand of a misgrouped `!a || b`, so the negation belongs to it.
func sdkLeafNegated(file *scanner.File, idx uint32) bool {
	for cur := idx; ; {
		p, ok := file.FlatParent(cur)
		if !ok {
			return false
		}
		switch {
		case sdkNotOperand(file, p) == cur:
			return cur != idx && sdkIsLogicalType(file.FlatType(cur))
		case sdkIsLogicalType(file.FlatType(p)) && file.FlatFirstChild(p) == cur:
			cur = p
		default:
			return false
		}
	}
}

// sdkCvc is Lint's getVersionCheckConstraint: what a true condition says about SDK_INT, or nil when unknown. A part
// that is not a version check is dropped from an `&&` or `||` instead of making the whole condition unknown.
func sdkCvc(file *scanner.File, idx uint32) *sdkCons {
	c := sdkCvcCore(file, sdkUnwrap(file, idx))
	if !sdkIsLogicalType(file.FlatType(idx)) && sdkLeafNegated(file, idx) {
		if c == nil || !c.neg {
			return nil
		}
		return c.not()
	}
	return c
}

func sdkCvcCore(file *scanner.File, idx uint32) *sdkCons {
	if set, ok := sdkCompareSet(file, idx); ok {
		return &sdkCons{set: set, neg: true}
	}
	if inner := sdkMisgroupedNot(file, idx); inner != 0 {
		return sdkCvcCore(file, inner)
	}
	switch op, _, _ := sdkLogical(file, idx); op {
	case "&&":
		return sdkAndWith(file, idx, 0)
	case "||":
		return sdkOrWith(file, idx, 0).not()
	}
	if inner := sdkNotOperand(file, idx); inner != 0 {
		if c := sdkCvc(file, inner); c != nil && c.neg {
			return c.not()
		}
	}
	return nil
}

// sdkAndWith is Lint's getAndedWithConstraint: the constraint on the operand after prev of an `&&`, or on the whole
// `&&` when prev is 0.
func sdkAndWith(file *scanner.File, idx, prev uint32) *sdkCons {
	op, left, right := sdkLogical(file, idx)
	if op != "&&" || prev == left {
		return nil
	}
	var l, r *sdkCons
	l = sdkCvc(file, left)
	if prev == right {
		return l.nonNegatable()
	}
	r = sdkCvc(file, right)
	res := sdkMax(l, r)
	if res != nil && (l == nil || r == nil || sdkChain(file, idx)) {
		res = res.nonNegatable()
	}
	return res
}

// sdkOrWith is Lint's getOredWithConstraint: what holds when an `||` reaches prev, which is that every operand before
// it was false.
func sdkOrWith(file *scanner.File, idx, prev uint32) *sdkCons {
	op, left, right := sdkLogical(file, idx)
	if op != "||" || prev == left {
		return nil
	}
	l := sdkCvc(file, left).not()
	if prev == right {
		return l
	}
	return sdkMax(l, sdkCvc(file, right).not())
}

// sdkUncertain is Lint's uncertainAnd (op "&&") and uncertainOr (op "||"): a logical expression of that operator with
// an operand that is neither itself uncertain nor a version check. A chain of three operands always counts.
func sdkUncertain(file *scanner.File, idx uint32, op string) bool {
	idx = sdkUnwrap(file, idx)
	if inner := sdkMisgroupedNot(file, idx); inner != 0 {
		idx = inner
	}
	got, left, right := sdkLogical(file, idx)
	if got != op {
		return false
	}
	if sdkChain(file, idx) {
		return true
	}
	for _, operand := range []uint32{left, right} {
		if !sdkUncertain(file, operand, op) && sdkCvc(file, operand) == nil {
			return true
		}
	}
	return false
}

// sdkParentConstraint is Lint's getVersionConditional: what the construct holding the expression says about SDK_INT
// inside the child it holds.
func sdkParentConstraint(file *scanner.File, parent, child uint32) *sdkCons {
	switch file.FlatType(parent) {
	case "if_expression", "if_statement":
		return sdkIfConstraint(file, parent, child)
	case "ternary_expression":
		kids := sdkChildren(file, parent)
		if len(kids) == 5 && (child == kids[2] || child == kids[4]) {
			return sdkBranchConstraint(file, kids[0], child == kids[2])
		}
	case "conjunction_expression", "disjunction_expression", "binary_expression":
		switch op, _, _ := sdkLogical(file, parent); op {
		case "&&":
			return sdkAndWith(file, parent, child)
		case "||":
			return sdkOrWith(file, parent, child)
		}
	case "when_entry":
		return sdkWhenConstraint(file, parent, file.FlatType(child) == "control_structure_body")
	}
	return nil
}

// sdkBranchConstraint is the constraint of the then (isThen) or else branch of a conditional on cond.
func sdkBranchConstraint(file *scanner.File, cond uint32, isThen bool) *sdkCons {
	if isThen && sdkUncertain(file, cond, "||") || !isThen && sdkUncertain(file, cond, "&&") {
		return nil
	}
	c := sdkCvc(file, cond)
	if isThen {
		return c
	}
	return c.not()
}

// sdkIfConstraint constrains the then and else branches of an if by its condition.
func sdkIfConstraint(file *scanner.File, ifNode, child uint32) *sdkCons {
	var cond uint32
	afterElse := false
	for _, c := range sdkChildren(file, ifNode) {
		switch t := file.FlatType(c); {
		case t == "if" || t == "(" || t == ")":
		case t == "else":
			afterElse = true
		case cond == 0:
			cond = c
		case c == child:
			return sdkBranchConstraint(file, cond, !afterElse)
		}
	}
	return nil
}

// sdkWhenConstraint is Lint's getCumulativeCaseConstraint for a subject-less `when`: only the first condition of an
// entry counts, every earlier entry's condition failed, and the entry's own condition holds in its body.
func sdkWhenConstraint(file *scanner.File, entry uint32, inBody bool) *sdkCons {
	when, ok := file.FlatParent(entry)
	if !ok {
		return nil
	}
	if _, subject := file.FlatFindChild(when, "when_subject"); subject {
		return nil
	}
	var acc *sdkCons
	for _, e := range sdkChildren(file, when) {
		if file.FlatType(e) != "when_entry" {
			continue
		}
		first := sdkFirstWhenCondition(file, e)
		var cc *sdkCons
		if first != 0 {
			cc = sdkCvc(file, first)
		}
		if e == entry {
			if !inBody || first != 0 && sdkUncertain(file, first, "||") {
				return acc
			}
			return sdkMax(cc, acc)
		}
		// Lint's behaviour is that of ignoring an earlier entry whose condition cannot be negated.
		if cc != nil && cc.neg && !sdkUncertain(file, first, "&&") {
			acc = sdkMax(cc.not(), acc)
		}
	}
	return nil
}

// sdkParenthesizedCaseValue adds the constraints Lint computes at each parenthesised layer of a case value: a node
// whose parent is parentheses around the clause's condition counts as being in the body unless it is an `||` reached
// from its right side.
func sdkParenthesizedCaseValue(file *scanner.File, cond, top uint32, below []uint32) *sdkCons {
	entry, ok := file.FlatParent(cond)
	if !ok {
		return nil
	}
	var acc *sdkCons
	p := top
	// below[0] is the checked expression itself, which Lint does not treat as one of its own parents.
	for j := len(below) - 1; j >= 1 && file.FlatType(p) == "parenthesized_expression"; j-- {
		p = below[j]
		prev := below[j-1]
		op, left, _ := sdkLogical(file, p)
		fromRight := op == "||" && prev != sdkUnwrap(file, left)
		acc = sdkMax(acc, sdkWhenConstraint(file, entry, !fromRight))
	}
	return acc
}

func sdkFirstWhenCondition(file *scanner.File, entry uint32) uint32 {
	for _, c := range sdkChildren(file, entry) {
		if file.FlatType(c) == "when_condition" {
			kids := sdkChildren(file, c)
			if len(kids) == 1 {
				return kids[0]
			}
			return c
		}
	}
	return 0
}

// sdkAnnotatedTypes are the declarations whose @RequiresApi/@TargetApi bounds the SDK_INT inside them.
var sdkAnnotatedTypes = map[string]bool{
	"function_declaration": true, "class_declaration": true, "object_declaration": true, "secondary_constructor": true,
	"companion_object": true, "getter": true, "setter": true, "method_declaration": true, "constructor_declaration": true,
	"interface_declaration": true, "enum_declaration": true, "record_declaration": true, "annotation_type_declaration": true,
}

// sdkAPIAnnotation describes a @RequiresApi or @TargetApi annotation.
type sdkAPIAnnotation struct {
	node  uint32
	name  string
	level int
}

var sdkAPIAnnotationFQNs = map[string][]string{
	"RequiresApi": {"androidx.annotation.RequiresApi", "android.support.annotation.RequiresApi"},
	"TargetApi":   {"android.annotation.TargetApi"},
}

// sdkParseAnnotation reads an annotation node as @RequiresApi(level) or @TargetApi(level).
func sdkParseAnnotation(file *scanner.File, ann uint32) (sdkAPIAnnotation, bool) {
	var typeText string
	var args []uint32
	switch file.Language {
	case scanner.LangJava:
		for _, c := range sdkChildren(file, ann) {
			switch file.FlatType(c) {
			case "identifier", "scoped_identifier":
				typeText = file.FlatNodeText(c)
			case "annotation_argument_list":
				args = sdkChildren(file, c)
			}
		}
	default:
		inv, ok := file.FlatFindChild(ann, "constructor_invocation")
		if !ok {
			return sdkAPIAnnotation{}, false
		}
		for _, c := range sdkChildren(file, inv) {
			switch file.FlatType(c) {
			case "user_type":
				typeText = file.FlatNodeText(c)
			case "value_arguments":
				args = sdkChildren(file, c)
			}
		}
	}
	typeText = strings.Join(strings.Fields(typeText), "")
	name := typeText[strings.LastIndex(typeText, ".")+1:]
	fqns, known := sdkAPIAnnotationFQNs[name]
	if !known || !sdkTypeMatchesAny(file, typeText, fqns) {
		return sdkAPIAnnotation{}, false
	}
	level, ok := sdkAnnotationLevel(file, args)
	if !ok {
		return sdkAPIAnnotation{}, false
	}
	return sdkAPIAnnotation{node: ann, name: name, level: level}, true
}

func sdkTypeMatchesAny(file *scanner.File, text string, fqns []string) bool {
	for _, fqn := range fqns {
		if lintTypeRefMatches(file, text, fqn) {
			return true
		}
	}
	return false
}

// sdkAnnotationLevel reads the first positional argument, or the `value`/`api` one, as an API level.
func sdkAnnotationLevel(file *scanner.File, args []uint32) (int, bool) {
	for _, a := range args {
		var expr uint32
		switch file.FlatType(a) {
		case "value_argument":
			expr = sdkValueArgumentExpr(file, a)
		case "element_value_pair":
			kids := sdkChildren(file, a)
			if len(kids) == 3 && sdkIsAPILabel(file.FlatNodeText(kids[0])) {
				expr = kids[2]
			}
		case "(", ")", ",":
		default:
			expr = a
		}
		if expr == 0 {
			continue
		}
		v, ok := lintEvalIntLocals(file, expr, sdkVersionCodes)
		if !ok || v < 1 || v >= sdkMaxLevel {
			return 0, false
		}
		return int(v), true
	}
	return 0, false
}

func sdkIsAPILabel(s string) bool { s = strings.TrimSpace(s); return s == "value" || s == "api" }

// sdkValueArgumentExpr returns the expression of a Kotlin argument that is positional or labelled value/api.
func sdkValueArgumentExpr(file *scanner.File, arg uint32) uint32 {
	kids := sdkChildren(file, arg)
	switch {
	case len(kids) == 1:
		return kids[0]
	case len(kids) == 3 && file.FlatType(kids[1]) == "=" && sdkIsAPILabel(file.FlatNodeText(kids[0])):
		return kids[2]
	}
	return 0
}

// sdkDeclAnnotation returns the @RequiresApi/@TargetApi annotation on a declaration node.
func sdkDeclAnnotation(file *scanner.File, decl uint32) (sdkAPIAnnotation, bool) {
	for _, mods := range sdkChildren(file, decl) {
		if file.FlatType(mods) != "modifiers" {
			continue
		}
		for _, ann := range sdkChildren(file, mods) {
			if file.FlatType(ann) != "annotation" {
				continue
			}
			if info, ok := sdkParseAnnotation(file, ann); ok {
				return info, true
			}
		}
	}
	return sdkAPIAnnotation{}, false
}

// sdkEnclosingAnnotation finds the nearest annotated declaration strictly above idx.
func sdkEnclosingAnnotation(file *scanner.File, idx uint32) (sdkAPIAnnotation, bool) {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if !sdkAnnotatedTypes[file.FlatType(cur)] {
			continue
		}
		if info, found := sdkDeclAnnotation(file, cur); found {
			return info, true
		}
	}
	return sdkAPIAnnotation{}, false
}

// sdkIsMethodBoundary reports whether a version check outside the node no longer applies inside it: a method or a
// field initializer, unless it belongs to an anonymous class.
func sdkIsMethodBoundary(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "function_declaration":
		return !sdkIsLocalFunction(file, idx) && !sdkInAnonymousClass(file, idx)
	case "secondary_constructor", "getter", "setter", "method_declaration", "constructor_declaration":
		return !sdkInAnonymousClass(file, idx)
	case "property_declaration":
		parent, ok := file.FlatParent(idx)
		return ok && file.FlatType(parent) != "statements" && !sdkInAnonymousClass(file, idx)
	case "field_declaration":
		return !sdkInAnonymousClass(file, idx)
	}
	return false
}

func sdkIsLocalFunction(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	return ok && file.FlatType(parent) == "statements"
}

func sdkInAnonymousClass(file *scanner.File, idx uint32) bool {
	body, ok := file.FlatParent(idx)
	if !ok || file.FlatType(body) != "class_body" {
		return false
	}
	owner, ok := file.FlatParent(body)
	if !ok {
		return false
	}
	switch file.FlatType(owner) {
	case "object_literal", "object_creation_expression":
		return true
	}
	return false
}

// sdkCondTypes are the node types of a boolean expression whose operands are boolean expressions.
var sdkCondTypes = map[string]bool{
	"parenthesized_expression": true, "prefix_expression": true, "unary_expression": true, "conjunction_expression": true,
	"disjunction_expression": true, "comparison_expression": true, "equality_expression": true, "check_expression": true,
	"binary_expression": true, "infix_expression": true,
}

// sdkMisparsedNot reports whether the subtree holds a negation the grammar groups wrongly and sdkMisgroupedNot does not
// repair: `!a == b` and the like.
func sdkMisparsedNot(file *scanner.File, idx uint32) bool {
	if file.FlatType(idx) == "prefix_expression" {
		kids := sdkChildren(file, idx)
		if len(kids) == 2 && file.FlatType(kids[0]) == "!" && sdkCondTypes[file.FlatType(kids[1])] &&
			file.FlatType(kids[1]) != "parenthesized_expression" && !sdkIsLogicalType(file.FlatType(kids[1])) {
			return true
		}
	}
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if sdkMisparsedNot(file, c) {
			return true
		}
	}
	return false
}

// sdkConditionsMisparsed reports whether the boolean expression around idx, or the condition of a conditional that
// holds it, is one the Kotlin grammar groups wrongly. Lint's verdict cannot be reproduced from such a tree.
func sdkConditionsMisparsed(file *scanner.File, idx uint32) bool {
	if file.Language == scanner.LangJava {
		return false
	}
	root := idx
	for p, ok := file.FlatParent(root); ok && sdkCondTypes[file.FlatType(p)]; p, ok = file.FlatParent(p) {
		root = p
	}
	if sdkMisparsedNot(file, root) {
		return true
	}
	for cur, ok := file.FlatParent(root); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "if_expression", "when_entry", "when_expression":
			if sdkMisparsedNot(file, cur) {
				return true
			}
		}
		if sdkIsMethodBoundary(file, cur) {
			return false
		}
	}
	return false
}

// sdkOuterCondition is Lint's getOuterVersionCheckConstraint: the constraint the conditions around idx put on
// SDK_INT, or nil when none applies.
func sdkOuterCondition(file *scanner.File, idx uint32) *sdkCons {
	var acc *sdkCons
	var below []uint32
	for child := idx; ; {
		parent, ok := file.FlatParent(child)
		if !ok {
			return acc
		}
		if sdkIsMethodBoundary(file, parent) {
			return acc
		}
		if file.FlatType(parent) == "when_condition" {
			acc = sdkMax(acc, sdkParenthesizedCaseValue(file, parent, child, below))
		}
		acc = sdkMax(acc, sdkParentConstraint(file, parent, child))
		below = append(below, child)
		child = parent
	}
}

// ---- comparisons ----

func (r *ObsoleteSdkIntRule) check(ctx *api.Context) {
	file := ctx.File
	if file == nil {
		return
	}
	if file.FlatType(ctx.Idx) == "annotation" {
		r.checkAnnotation(ctx)
		return
	}
	check, ok := sdkCompareSet(file, ctx.Idx)
	if !ok || sdkConditionsMisparsed(file, ctx.Idx) {
		return
	}
	minSdk := obsoleteSdkMinSdk(ctx)
	if minSdk < 1 {
		return
	}
	combined := sdkFrom(minSdk)
	outer := sdkOuterCondition(file, ctx.Idx)
	if outer != nil && outer.none {
		outer = nil
	}
	hasContext := outer != nil
	if outer != nil && !outer.set.isEmpty() {
		combined = combined.and(outer.set)
	}
	if enclosing, found := sdkEnclosingAnnotation(file, ctx.Idx); found {
		combined = combined.and(sdkFrom(enclosing.level))
		hasContext = true
	}
	if msg, ok := sdkObsoleteMessage(file.FlatNodeText(ctx.Idx), check, combined, hasContext); ok {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", msg))
	}
}

// sdkObsoleteMessage words the finding Lint makes when the combined constraint decides the check.
func sdkObsoleteMessage(text string, check, combined sdkSet, hasContext bool) (string, bool) {
	both := check.and(combined)
	always, never := both.equal(combined), both.isEmpty()
	if !always && !never {
		return "", false
	}
	if (never && hasContext) || !combined.openEnded() {
		verdict, suffix := "always", ""
		if never {
			verdict = "never"
		} else {
			suffix = " (`SDK_INT` " + combined.describe() + ")"
		}
		return fmt.Sprintf("Unnecessary; `%s` is %s true here%s", text, verdict, suffix), true
	}
	if always {
		return fmt.Sprintf("Unnecessary; `SDK_INT` is always >= %d", combined[0].lo), true
	}
	return fmt.Sprintf("Unnecessary; `SDK_INT` is never < %d", combined[0].lo), true
}

// obsoleteSdkMinSdk returns the minSdk of the module owning the file, or 0 when no build file or manifest declares it.
func obsoleteSdkMinSdk(ctx *api.Context) int {
	if levels := android.ResolveSDKLevels(ctx.File.Path); levels.MinSdk > 0 {
		return levels.MinSdk
	}
	if facts := librarymodel.EnsureFacts(ctx.LibraryFacts); facts != nil {
		return facts.Profile.MinSdkVersion
	}
	return 0
}

// ---- annotations ----

// sdkAnnotationTarget returns the declaration an annotation is written on, or 0 when Lint does not visit it: type
// aliases, file annotations and local variables.
func sdkAnnotationTarget(file *scanner.File, ann uint32) uint32 {
	mods, ok := file.FlatParent(ann)
	if !ok {
		return 0
	}
	switch file.FlatType(mods) {
	case "modifiers", "parameter_modifiers":
	default:
		return 0
	}
	decl, ok := file.FlatParent(mods)
	if !ok {
		return 0
	}
	switch file.FlatType(decl) {
	case "type_alias", "local_variable_declaration", "file_annotation", "package_declaration", "import_declaration":
		return 0
	case "property_declaration":
		if parent, ok := file.FlatParent(decl); ok && file.FlatType(parent) == "statements" {
			return 0
		}
	}
	return decl
}

func (r *ObsoleteSdkIntRule) checkAnnotation(ctx *api.Context) {
	file := ctx.File
	info, ok := sdkParseAnnotation(file, ctx.Idx)
	if !ok {
		return
	}
	decl := sdkAnnotationTarget(file, ctx.Idx)
	if decl == 0 {
		return
	}
	if outer, found := sdkEnclosingAnnotation(file, decl); found && info.level < outer.level {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", fmt.Sprintf(
			"Unnecessary; `SDK_INT` is always >= %d from outer annotation (`@%s(%d)`)", outer.level, outer.name, outer.level)))
		return
	}
	if minSdk := obsoleteSdkMinSdk(ctx); minSdk >= info.level {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Unnecessary; `SDK_INT` is always >= "+strconv.Itoa(info.level)))
	}
}
