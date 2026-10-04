package dev.jasonpearson.krit.fir.checkers.potentialbugs

import com.intellij.lang.LighterASTNode
import com.intellij.psi.tree.IElementType
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import dev.jasonpearson.krit.fir.support.unwrapLightParens
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirCheckNotNullCallChecker
import org.jetbrains.kotlin.fir.analysis.checkers.processOverriddenFunctionsSafe
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.argument
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Flags a not-null assertion on a map lookup: `map[key]!!`,
 * `map.get(key)!!` or `map?.get(key)!!`, where `getValue()` or
 * `getOrDefault()` states the intent and fails with a useful message.
 *
 * Mirrors the Go rule's scope and exemptions:
 * - the operand of `!!` (through parentheses) must be written as an index
 *   access with one index or as a qualified `get` call with one argument on
 *   an explicit receiver; a bare `get(key)!!` on an implicit receiver is not
 *   reported, as Go does not report it;
 * - files krit classifies as test files are skipped;
 * - an access guarded by `receiver.containsKey(key)` is skipped: the access
 *   sits in the then branch of an `if` whose condition proves the key is
 *   present (or in the else branch of one proving it absent), or it follows,
 *   in the same block, an `if` without else whose condition proves the key
 *   absent and whose body always leaves (its last statement is a
 *   `return`/`throw`/`break`/`continue`, or an if/else whose both branches
 *   do). The receiver and key are matched by their source text, as Go
 *   matches them. The if-walk stops at the nearest named function or lambda,
 *   the block search at the nearest block, exactly where Go stops.
 *
 * The map proof comes from resolution instead of Go's source type inference:
 * the call must resolve to `kotlin.collections.Map.get`, to a member that
 * overrides it (`HashMap.get`, `TreeMap.get`, a project `Map`
 * implementation, a delegated `Map`), or to the stdlib
 * `Map<out K, V>.get(key)` extension.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: Go accepts any receiver whose type name is `Map`, `HashMap`,
 *   `LinkedHashMap`, `TreeMap` (or `Mutable`/qualified spellings), and
 *   does not check which `get` the call resolves to. A project class that
 *   reuses one of those names, a project `get` extension on a map whose key
 *   type Go does not compare (`Map<*, *>`), and a map subclass's own `get`
 *   overload with another parameter type are not map lookups, so they are
 *   not reported.
 * - Guards: a containsKey guard only exempts an access when the condition
 *   proves the key is present through `!`, `&&` / `and`, `||` / `or`,
 *   comparisons with a boolean literal and `also` / `apply`, each step
 *   sound, and Go also accepts it. Go also accepts a `containsKey` call
 *   anywhere in the condition (`containsKey(k) == false`, an argument of
 *   another call, inside a lambda or an if expression, the receiver of
 *   `let`), a negated conjunction (`!(a && !containsKey(k))`), a top-level
 *   `a || containsKey(k)` (then branch) or `a && !containsKey(k)` (else
 *   branch, early return), and the infix `a or containsKey(k)` /
 *   `a and !containsKey(k)` at any depth, none of which proves the key is
 *   present, so those accesses are reported.
 * - Recall: resolution sees map receivers Go cannot type (a `super`
 *   receiver, a companion object's property, a scope-function `it`, a type
 *   parameter bounded by Map, a Java method's result, the receiver
 *   `a[k]!!` of a second lookup, a JDK map such as `Properties` or
 *   `ConcurrentHashMap`, an anonymous object) and keys whose type Go cannot
 *   prove equal to the map's key type (a subtype), plus the stdlib
 *   `Map<out K, V>.get(key)` extension, and lookups that are the target of
 *   an assignment (`map[k]!!.count += 1`), which Go's tree-sitter view does
 *   not parse as a `!!` expression.
 */
internal object MapGetWithNotNullAssertionOperator : FirCheckNotNullCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "MapGetWithNotNullAssertionOperator"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val checkNotNullCallCheckers = setOf(MapGetWithNotNullAssertionOperator)
    }

    private const val MESSAGE = "Map access with not-null assertion operator (!!). Use getValue() or getOrDefault() instead."
    private val GET = Name.identifier("get")
    private val COLLECTIONS = FqName("kotlin.collections")
    private val MAP_GET = CallableId(ClassId(COLLECTIONS, Name.identifier("Map")), GET)
    private val STDLIB_MAP_GET_EXTENSION = CallableId(COLLECTIONS, GET)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirCheckNotNullCall) {
        val source = expression.source ?: return
        if (source.kind is KtFakeSourceElementKind) return
        val call = lookupCall(expression.argument) ?: return
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (callee.name != GET || !isMapGet(callee)) return
        val access = writtenAccess(source) ?: return
        if (isInTestFile()) return
        if (isContainsKeyGuarded(source, access) || isEarlyReturnGuarded(source, access)) return
        report(source, MESSAGE)
    }

    private fun lookupCall(argument: FirExpression): FirFunctionCall? = when (argument) {
        is FirFunctionCall -> argument
        is FirSafeCallExpression -> argument.selector as? FirFunctionCall
        else -> null
    }

    // Map.get itself, a member overriding it, or the stdlib extension that
    // forwards to it.
    context(context: CheckerContext)
    private fun isMapGet(callee: FirNamedFunctionSymbol): Boolean {
        val original = callee.unwrapFakeOverrides()
        val id = original.callableId
        if (id == MAP_GET || id == STDLIB_MAP_GET_EXTENSION) return true
        if (original.receiverParameterSymbol != null) return false
        var overridesMapGet = false
        original.processOverriddenFunctionsSafe {
            if (it.unwrapFakeOverrides().callableId == MAP_GET) overridesMapGet = true
        }
        return overridesMapGet
    }

    // ---- Written shape (the Go rule's tree-sitter view) -------------------

    /** The access `receiver[key]` / `receiver.get(key)` under the `!!`. */
    private class Access(val receiver: LighterASTNode, val key: LighterASTNode)

    private fun writtenAccess(source: KtSourceElement): Access? {
        val postfix = source.lighterASTNode
        if (postfix.tokenType != KtNodeTypes.POSTFIX_EXPRESSION) return null
        val operand = significant(source, postfix).firstOrNull() ?: return null
        val access = unwrapLightParens(source, operand)
        return when (access.tokenType) {
            KtNodeTypes.ARRAY_ACCESS_EXPRESSION -> indexAccess(source, access)
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION -> getCallAccess(source, access)
            else -> null
        }
    }

    // `receiver[key]`: Go takes the indexing suffix's only named child, so a
    // comment inside the brackets makes it give up, as it does here.
    private fun indexAccess(source: KtSourceElement, access: LighterASTNode): Access? {
        val parts = significant(source, access)
        val receiver = parts.firstOrNull() ?: return null
        val indices = parts.lastOrNull()?.takeIf { it.tokenType == KtNodeTypes.INDICES } ?: return null
        val keys = lightChildren(source, indices).filter {
            it.tokenType != KtTokens.WHITE_SPACE && it.tokenType != KtTokens.LBRACKET &&
                it.tokenType != KtTokens.RBRACKET && it.tokenType != KtTokens.COMMA
        }
        return keys.singleOrNull()?.let { Access(receiver, it) }
    }

    // `receiver.get(key)` / `receiver?.get(key)` with one (possibly named)
    // argument.
    private fun getCallAccess(source: KtSourceElement, access: LighterASTNode): Access? {
        val parts = significant(source, access)
        if (parts.size != 2) return null
        val (receiver, selector) = parts
        if (selector.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val key = getCallKey(source, selector) ?: return null
        return Access(receiver, key)
    }

    private fun getCallKey(source: KtSourceElement, call: LighterASTNode): LighterASTNode? {
        val parts = significant(source, call)
        val name = parts.firstOrNull()?.takeIf { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION } ?: return null
        if (lightText(source, name) != "get") return null
        // `get<K, V>(key)`: explicit type arguments select the stdlib extension.
        val arguments = if (parts.getOrNull(1)?.tokenType == KtNodeTypes.TYPE_ARGUMENT_LIST) 2 else 1
        if (parts.size != arguments + 1) return null
        val args = parts[arguments].takeIf { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return null
        val argument = lightChildren(source, args).singleOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT } ?: return null
        return significant(source, argument).lastOrNull()
    }

    // ---- containsKey guards (Go: nullflow.IsMapContainsKeyGuarded) --------

    // The access sits in a branch of an enclosing `if` whose condition proves
    // the key is present in that branch. Go walks up to the nearest named
    // function or lambda.
    private fun isContainsKeyGuarded(source: KtSourceElement, access: Access): Boolean {
        val tree = source.treeStructure
        var node = source.lighterASTNode
        while (true) {
            val parent = tree.getParent(node) ?: return false
            if (isWalkBoundary(source, parent)) return false
            if (parent.tokenType == KtNodeTypes.IF) {
                val condition = conditionOf(source, parent)
                when (node.tokenType) {
                    KtNodeTypes.THEN -> if (condition != null && proves(source, condition, access, whenTrue = true)) return true
                    KtNodeTypes.ELSE -> if (condition != null && proves(source, condition, access, whenTrue = false)) return true
                }
            }
            node = parent
        }
    }

    // Go stops at a `function_declaration` (a named function; an anonymous
    // `fun() {}` is not one) and at a `lambda_literal`.
    private fun isWalkBoundary(source: KtSourceElement, node: LighterASTNode): Boolean = when (node.tokenType) {
        KtNodeTypes.FUNCTION_LITERAL, KtNodeTypes.LAMBDA_EXPRESSION -> true
        KtNodeTypes.FUN -> lightChildren(source, node).any { it.tokenType == KtTokens.IDENTIFIER }
        else -> false
    }

    // ---- Early-return guards (Go: nullflow.IsEarlyReturnMapContainsKeyGuarded)

    // An earlier statement of the nearest enclosing block is
    // `if (!receiver.containsKey(key)) <leave>` without an else branch.
    private fun isEarlyReturnGuarded(source: KtSourceElement, access: Access): Boolean {
        val tree = source.treeStructure
        var anchor = source.lighterASTNode
        var block: LighterASTNode? = null
        while (true) {
            val parent = tree.getParent(anchor) ?: return false
            if (isWalkBoundary(source, parent)) return false
            if (parent.tokenType == KtNodeTypes.BLOCK) {
                block = parent
                break
            }
            anchor = parent
        }
        for (statement in significant(source, block ?: return false)) {
            if (statement == anchor || statement.startOffset >= anchor.startOffset) return false
            if (statement.tokenType != KtNodeTypes.IF) continue
            val parts = lightChildren(source, statement)
            if (parts.any { it.tokenType == KtNodeTypes.ELSE }) continue
            val condition = conditionOf(source, statement) ?: continue
            val then = parts.firstOrNull { it.tokenType == KtNodeTypes.THEN } ?: continue
            if (!bodyAlwaysLeaves(source, then)) continue
            if (proves(source, condition, access, whenTrue = false)) return true
        }
        return false
    }

    // Go's bodyAlwaysExitsFlat: the body's last statement is a jump, or an
    // if/else whose both branches always leave. A trailing comment is the
    // last statement to Go, so it ends the body without leaving.
    private fun bodyAlwaysLeaves(source: KtSourceElement, body: LighterASTNode): Boolean {
        val statement = significantWithComments(source, body).lastOrNull() ?: return false
        return statementAlwaysLeaves(source, statement)
    }

    private fun statementAlwaysLeaves(source: KtSourceElement, statement: LighterASTNode): Boolean =
        when (statement.tokenType) {
            KtNodeTypes.BLOCK -> {
                val last = significantWithComments(source, statement)
                    .lastOrNull { it.tokenType != KtTokens.LBRACE && it.tokenType != KtTokens.RBRACE }
                last != null && statementAlwaysLeaves(source, last)
            }
            KtNodeTypes.RETURN, KtNodeTypes.THROW, KtNodeTypes.BREAK, KtNodeTypes.CONTINUE -> true
            KtNodeTypes.IF -> {
                val parts = lightChildren(source, statement)
                val then = parts.firstOrNull { it.tokenType == KtNodeTypes.THEN }
                val otherwise = parts.firstOrNull { it.tokenType == KtNodeTypes.ELSE }
                then != null && otherwise != null && bodyAlwaysLeaves(source, then) && bodyAlwaysLeaves(source, otherwise)
            }
            else -> false
        }

    // ---- Condition proofs (Go: mapContainsKeyConditionProves) -------------

    /**
     * Whether [condition] evaluating to [whenTrue] proves
     * `receiver.containsKey(key)` is true, both soundly and by Go's test.
     *
     * Sound: only parentheses, `!`, `&&` / `and`, `||` / `or`, a comparison
     * with a boolean literal (`== true`, `!= false`, ...) and the receiver of
     * `also` / `apply` (which return it) are looked through. A conjunction
     * proves the key when it is true and either operand proves it, or when it
     * is false and both do; a disjunction when it is false and either operand
     * proves it, or when it is true and both do.
     *
     * Go: Go accepts a `containsKey` call anywhere in the condition when the
     * number of `!` operators above it is even (then branch) or odd (else
     * branch, early return), and no `||` (then branch) or `&&` (otherwise)
     * is nested between it and the condition (the condition's own top-level
     * operator is not checked, and the infix `and` / `or` never are). It
     * ignores `== false`, so the two tests are applied together and an
     * access is skipped only when both hold; where Go's test alone is
     * unsound, the sound test rejects it.
     */
    private fun proves(source: KtSourceElement, condition: LighterASTNode, access: Access, whenTrue: Boolean): Boolean =
        provesAt(source, condition, access, Proof(condition, value = whenTrue, rootValue = whenTrue, negations = 0, goAccepts = true))

    /**
     * [root]: the whole condition. [value]: what the current node must
     * evaluate to. [rootValue]: what the whole condition evaluates to.
     * [negations]: the `!` operators crossed, Go's parity count.
     * [goAccepts]: no `||` (then branch) or `&&` (otherwise) that Go rejects
     * has been crossed.
     */
    private data class Proof(
        val root: LighterASTNode,
        val value: Boolean,
        val rootValue: Boolean,
        val negations: Int,
        val goAccepts: Boolean,
    )

    private fun provesAt(source: KtSourceElement, node: LighterASTNode, access: Access, proof: Proof): Boolean =
        when (node.tokenType) {
            KtNodeTypes.PARENTHESIZED -> significant(source, node).singleOrNull()
                ?.let { provesAt(source, it, access, proof) } ?: false
            KtNodeTypes.PREFIX_EXPRESSION -> {
                val parts = significant(source, node)
                if (parts.size == 2 && operationToken(source, parts[0]) == KtTokens.EXCL) {
                    provesAt(source, parts[1], access, proof.copy(value = !proof.value, negations = proof.negations + 1))
                } else {
                    false
                }
            }
            KtNodeTypes.BINARY_EXPRESSION -> provesThroughBinary(source, node, access, proof)
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION -> {
                val goParity = if (proof.rootValue) proof.negations % 2 == 0 else proof.negations % 2 == 1
                if (proof.value && proof.goAccepts && goParity && isContainsKeyCall(source, node, access)) {
                    true
                } else {
                    val receiver = receiverOfSelfReturningCall(source, node)
                    receiver != null && provesAt(source, receiver, access, proof)
                }
            }
            else -> false
        }

    private fun provesThroughBinary(source: KtSourceElement, node: LighterASTNode, access: Access, proof: Proof): Boolean {
        val parts = significant(source, node)
        if (parts.size != 3) return false
        val (left, operation, right) = parts
        val token = operationToken(source, operation)
        val infix = if (token == KtTokens.IDENTIFIER) lightText(source, operation) else null
        val conjunction = token == KtTokens.ANDAND || infix == "and"
        val disjunction = token == KtTokens.OROR || infix == "or"
        if (conjunction || disjunction) {
            // Go rejects an `&&` / `||` nested below the condition: an `||`
            // for a then branch, an `&&` for an else branch or early return.
            // It never rejects the infix `and` / `or`.
            val nested = node != proof.root
            val goRejects = nested &&
                ((token == KtTokens.OROR && proof.rootValue) || (token == KtTokens.ANDAND && !proof.rootValue))
            val next = proof.copy(goAccepts = proof.goAccepts && !goRejects)
            // A true conjunction or a false disjunction fixes both operands,
            // so either one proving the key is enough; otherwise only one
            // operand is known to hold the value, so both must prove it.
            return if (conjunction == proof.value) {
                provesAt(source, left, access, next) || provesAt(source, right, access, next)
            } else {
                provesAt(source, left, access, next) && provesAt(source, right, access, next)
            }
        }
        if (token != KtTokens.EQEQ && token != KtTokens.EXCLEQ) return false
        val (literal, operand) = when {
            booleanLiteral(source, right) != null -> booleanLiteral(source, right) to left
            booleanLiteral(source, left) != null -> booleanLiteral(source, left) to right
            else -> return false
        }
        // `c == true` / `c != false` keep the value, `c == false` /
        // `c != true` flip it. Go's parity ignores them.
        val keeps = (literal == true) == (token == KtTokens.EQEQ)
        return provesAt(source, operand, access, proof.copy(value = if (keeps) proof.value else !proof.value))
    }

    // The receiver of `receiver.also { ... }` / `receiver.apply { ... }`,
    // which evaluate to their receiver. Go walks into it like any other
    // subexpression of the condition.
    private fun receiverOfSelfReturningCall(source: KtSourceElement, node: LighterASTNode): LighterASTNode? {
        val parts = significant(source, node)
        if (parts.size != 2) return null
        val (receiver, selector) = parts
        if (selector.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val name = significant(source, selector).firstOrNull()
            ?.takeIf { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION } ?: return null
        return receiver.takeIf { lightText(source, name) == "also" || lightText(source, name) == "apply" }
    }

    private fun booleanLiteral(source: KtSourceElement, node: LighterASTNode): Boolean? {
        val literal = unwrapLightParens(source, node)
        if (literal.tokenType != KtNodeTypes.BOOLEAN_CONSTANT) return null
        return when (lightText(source, literal)) {
            "true" -> true
            "false" -> false
            else -> null
        }
    }

    // `receiver.containsKey(key)` (not a safe call), matched on the source
    // text of the receiver and the key, as Go matches them.
    private fun isContainsKeyCall(source: KtSourceElement, node: LighterASTNode, access: Access): Boolean {
        val parts = significant(source, node)
        if (parts.size != 2) return false
        val (receiver, selector) = parts
        if (selector.tokenType != KtNodeTypes.CALL_EXPRESSION) return false
        val callParts = significant(source, selector)
        val name = callParts.firstOrNull()?.takeIf { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION } ?: return false
        if (lightText(source, name) != "containsKey" || callParts.size != 2) return false
        val args = callParts[1].takeIf { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return false
        val argument = lightChildren(source, args).singleOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT } ?: return false
        val key = significant(source, argument).lastOrNull() ?: return false
        return equivalent(source, receiver, access.receiver) && equivalent(source, key, access.key)
    }

    // ---- Light-tree helpers -----------------------------------------------

    private fun equivalent(source: KtSourceElement, a: LighterASTNode, b: LighterASTNode): Boolean {
        val left = unwrapLightParens(source, a)
        val right = unwrapLightParens(source, b)
        if (left == right) return true
        return left.tokenType == right.tokenType && lightText(source, left).trim() == lightText(source, right).trim()
    }

    // The condition expression of an `if`. Go takes the first named child of
    // the `if` as its condition, so a comment ahead of the condition, before
    // or after `(`, hides it from Go and the `if` proves nothing.
    private fun conditionOf(source: KtSourceElement, ifNode: LighterASTNode): LighterASTNode? {
        val children = lightChildren(source, ifNode)
        val lpar = children.indexOfFirst { it.tokenType == KtTokens.LPAR }
        if (lpar < 0 || children.take(lpar).any { it.tokenType in KtTokens.COMMENTS }) return null
        val afterParen = children.drop(lpar + 1)
            .firstOrNull { it.tokenType != KtTokens.WHITE_SPACE } ?: return null
        if (afterParen.tokenType != KtNodeTypes.CONDITION) return null
        val first = lightChildren(source, afterParen).firstOrNull { it.tokenType != KtTokens.WHITE_SPACE } ?: return null
        return first.takeIf { it.tokenType !in KtTokens.COMMENTS }
    }

    private fun operationToken(source: KtSourceElement, operation: LighterASTNode): IElementType? {
        if (operation.tokenType != KtNodeTypes.OPERATION_REFERENCE) return null
        return lightChildren(source, operation).firstOrNull()?.tokenType
    }

    // Children other than whitespace, comments and punctuation.
    private fun significant(source: KtSourceElement, node: LighterASTNode): List<LighterASTNode> =
        significantChildren(source, node, trivia)

    // Children other than whitespace and punctuation; comments are kept.
    private fun significantWithComments(source: KtSourceElement, node: LighterASTNode): List<LighterASTNode> =
        lightChildren(source, node).filter { it.tokenType !in trivia }

    private val trivia = setOf(
        KtTokens.WHITE_SPACE,
        KtTokens.LPAR,
        KtTokens.RPAR,
        KtTokens.DOT,
        KtTokens.SAFE_ACCESS,
        KtTokens.EXCLEXCL,
        KtTokens.SEMICOLON,
        KtTokens.EQ,
    )
}
