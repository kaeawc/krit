package dev.jasonpearson.krit.fir.checkers.testingquality

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isGoSpace
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirAnonymousObject
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.argument
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId

/**
 * Flags a not-null assertion (`!!`) in the arguments of an assertion-library call, where
 * `assertNotNull(x)` first and a smart cast say the same thing and fail with a message.
 *
 * Mirrors the Go rule: one finding per assertion call, reported on the first `!!` token, listing
 * every `!!` in the call's own arguments (a member receiver `x!!.ts` reads a member, not the value).
 * A `!!` inside a nested call's arguments or a lambda is not the assertion's subject. Go runs the rule
 * only on test-support files and skips an operand an earlier `assertNotNull`, `requireNotNull`,
 * `checkNotNull` or `assertThat(x).isNotNull()` call in the same function already checks, since a library
 * without a Kotlin contract does not smart-cast it; the checker reads those calls from the syntax tree as Go does.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: the call must resolve to a function of an assertion library (kotlin.test, JUnit,
 *   Truth, AssertJ, Hamcrest, Turbine). Go accepts the written name when the oracle cannot resolve
 *   the call, so a project function that reuses a library name is not reported.
 * - Precision: the operand's type comes from the compiler. A platform type is not nullable, and an
 *   operand the compiler smart-casts to non-null (an earlier `assertNotNull(x)` with a contract) makes
 *   the `!!` unnecessary rather than unsafe.
 * - Recall: the name is the declared name, so an import alias of an assertion function is reported.
 */
internal object AssertNullableWithNotNullAssertion : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "AssertNullableWithNotNullAssertion"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(AssertNullableWithNotNullAssertion)
    }

    // assertNotNull and assertNull take the nullable value itself, so they are not on the list.
    private val assertionNames = setOf(
        "assertEquals", "assertNotEquals", "assertTrue", "assertFalse", "assertSame", "assertNotSame",
        "assertThat", "assertThrows", "assertFailsWith", "assertIs", "assertIsNot", "assertContains",
        "assertContentEquals", "expectThat", "expect", "expectClean", "expectErrorCount", "expectWarningCount",
        "expectFixDiffs", "expectFixes", "expectMatches", "expectContains", "isEqualTo", "isNotEqualTo",
        "isSameInstanceAs", "isNotSameInstanceAs", "isNull", "isNotNull", "isTrue", "isFalse", "containsExactly",
        "containsAtLeast", "containsNoneOf", "hasSize", "isEmpty", "isNotEmpty", "shouldBe", "shouldNotBe",
        "shouldThrow", "shouldNotThrow",
    )

    private val libraryPrefixes = listOf(
        "kotlin.test.", "org.junit.", "junit.framework.", "com.google.common.truth.", "org.assertj.",
        "org.hamcrest.", "app.cash.turbine.",
    )

    private const val TRUTH_PREFIX = "com.google.common.truth."
    private const val MAX_OPERAND_LENGTH = 60

    // A written call that starts a new scope for `!!`: its own arguments or a lambda.
    private val scopeBoundaries = setOf(
        KtNodeTypes.VALUE_ARGUMENT_LIST,
        KtNodeTypes.LAMBDA_ARGUMENT,
        KtNodeTypes.LAMBDA_EXPRESSION,
        KtNodeTypes.FUNCTION_LITERAL,
        KtNodeTypes.FUN,
    )
    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)
    private val accessTokens = setOf(KtTokens.DOT, KtTokens.SAFE_ACCESS)

    private class Bang(
        val source: KtSourceElement,
        val postfix: LighterASTNode,
        val operandText: String,
        val isMemberReceiver: Boolean,
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        if (!isLibraryAssertion(callableId)) return
        if (!runsInFile()) return
        val argumentList = writtenArgumentList(source) ?: return
        val bangs = BangCollector().also { collector -> expression.arguments.forEach { it.accept(collector) } }.bangs
            .filter { it.source?.kind is KtRealSourceElementKind }
            .mapNotNull { subjectBang(it, argumentList) }
            .sortedBy { it.source.startOffset }
        if (bangs.isEmpty()) return
        val unchecked = bangs.filter { !isCheckedBefore(it, source) }
        if (unchecked.isEmpty()) return
        report(operatorAnchor(unchecked.first()), message(unchecked))
    }

    private fun isLibraryAssertion(id: CallableId): Boolean {
        if (id.callableName.asString() !in assertionNames) return false
        val qualified = id.asSingleFqName().asString()
        return libraryPrefixes.any { qualified.startsWith(it) }
    }

    // Go runs the rule on test-support files: test sources, testing and test-utils modules, `*Test.kt`, and
    // the testing-quality fixtures.
    context(context: CheckerContext)
    private fun runsInFile(): Boolean {
        if (isInTestFile()) return true
        val lower = containingScanPath()?.lowercase() ?: return false
        return "-testing/" in lower || "/testing/" in lower || "/test-utils/" in lower || "-test-utils/" in lower ||
            lower.endsWith("test.kt") || ("/tests/fixtures/" in lower && "/testing-quality/" in lower)
    }

    // The value argument list of the written call; null for operator, infix and implicit-invoke forms.
    private fun writtenArgumentList(source: KtSourceElement): LighterASTNode? {
        var call = source.lighterASTNode
        if (call.tokenType in qualifiedTypes) {
            call = significantChildren(source, call, accessTokens).lastOrNull() ?: return null
        }
        if (call.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        return lightChildren(source, call).firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST }
    }

    private class BangCollector : FirVisitorVoid() {
        val bangs = mutableListOf<FirCheckNotNullCall>()

        override fun visitElement(element: FirElement) {
            element.acceptChildren(this)
        }

        override fun visitCheckNotNullCall(checkNotNullCall: FirCheckNotNullCall) {
            bangs += checkNotNullCall
            visitElement(checkNotNullCall)
        }

        override fun visitAnonymousFunction(anonymousFunction: FirAnonymousFunction) {}

        override fun visitNamedFunction(namedFunction: FirNamedFunction) {}

        override fun visitRegularClass(regularClass: FirRegularClass) {}

        override fun visitAnonymousObject(anonymousObject: FirAnonymousObject) {}
    }

    // The bang when it is a subject of the call: written as `x!!` straight in the call's argument list, not
    // inside a nested argument list or lambda, and the operand's declared type is nullable.
    context(context: CheckerContext)
    private fun subjectBang(call: FirCheckNotNullCall, argumentList: LighterASTNode): Bang? {
        val source = call.source ?: return null
        val postfix = source.lighterASTNode
        if (postfix.tokenType != KtNodeTypes.POSTFIX_EXPRESSION || !isDirectArgument(source, postfix, argumentList)) {
            return null
        }
        if (!isDeclaredNullable(call.argument)) return null
        val operand = significantChildren(source, postfix).firstOrNull() ?: return null
        val parent = source.treeStructure.getParent(postfix)
        val isMemberReceiver = parent != null && parent.tokenType in qualifiedTypes &&
            significantChildren(source, parent, accessTokens).firstOrNull()?.let { sameNode(it, postfix) } == true
        return Bang(source, postfix, lightText(source, operand), isMemberReceiver)
    }

    private fun isDirectArgument(source: KtSourceElement, postfix: LighterASTNode, argumentList: LighterASTNode): Boolean {
        var node = postfix
        while (true) {
            val parent = source.treeStructure.getParent(node) ?: return false
            if (sameNode(parent, argumentList)) return true
            if (parent.tokenType in scopeBoundaries) return false
            node = parent
        }
    }

    // A platform type is not nullable here, and a stable smart cast to non-null makes the `!!` unnecessary.
    context(context: CheckerContext)
    private fun isDeclaredNullable(operand: FirExpression): Boolean {
        var expression = operand
        while (expression is FirSmartCastExpression && !expression.isStable) expression = expression.originalExpression
        return expression.resolvedType.lowerBoundIfFlexible().canBeNull(context.session)
    }

    // An earlier check in the same function: without a Kotlin contract the `!!` after it is the required idiom.
    // As in Go, the check is a call (`assertNotNull`, `requireNotNull` or `checkNotNull` on the operand, first or
    // after one message argument, `assertThat(x).isNotNull()` or `assertThat(x, notNullValue())`) that ends before
    // the assertion, and every block, lambda body or branch body around it also holds the assertion.
    context(context: CheckerContext)
    private fun isCheckedBefore(bang: Bang, call: KtSourceElement): Boolean {
        val operand = compact(bang.operandText)
        if (operand.isEmpty()) return false
        val scope = context.containingDeclarations.lastOrNull { it is FirNamedFunctionSymbol }?.source
            ?: containingFirFile()?.source ?: return false
        val callStart = qualifiedCall(call)?.let { lightSourceOf(it, call).startOffset } ?: call.startOffset
        val shift = scope.startOffset - scope.lighterASTNode.startOffset
        fun visit(node: LighterASTNode): Boolean {
            if (node.startOffset + shift >= callStart) return false
            if (node.tokenType == KtNodeTypes.CALL_EXPRESSION && node.endOffset + shift <= callStart &&
                isNonNullCheck(scope, node, operand) && reachesCall(scope, node, shift, callStart)
            ) {
                return true
            }
            return lightChildren(scope, node).any { visit(it) }
        }
        return visit(scope.lighterASTNode)
    }

    private fun compact(text: String): String = text.filterNot { isGoSpace(it) }

    private fun isNonNullCheck(source: KtSourceElement, call: LighterASTNode, operand: String): Boolean {
        val args = argumentTexts(source, call)
        return when (calleeName(source, call)) {
            "assertNotNull", "requireNotNull", "checkNotNull" -> args.getOrNull(0) == operand || args.getOrNull(1) == operand
            "assertThat" -> args.size == 2 && args[0] == operand && args[1] == "notNullValue()"
            "isNotNull" -> args.isEmpty() && truthSubject(source, call)?.let { argumentTexts(source, it) } == listOf(operand)
            else -> false
        }
    }

    // The `assertThat(x)` call that `.isNotNull()` is called on, written directly or as `Truth.assertThat(x)`.
    private fun truthSubject(source: KtSourceElement, call: LighterASTNode): LighterASTNode? {
        val parent = source.treeStructure.getParent(call) ?: return null
        if (parent.tokenType !in qualifiedTypes) return null
        val parts = significantChildren(source, parent, accessTokens)
        if (parts.size != 2 || !sameNode(parts[1], call)) return null
        var receiver = parts[0]
        if (receiver.tokenType in qualifiedTypes) receiver = significantChildren(source, receiver, accessTokens).lastOrNull() ?: return null
        return receiver.takeIf { it.tokenType == KtNodeTypes.CALL_EXPRESSION && calleeName(source, it) == "assertThat" }
    }

    private fun calleeName(source: KtSourceElement, call: LighterASTNode): String? =
        lightChildren(source, call).firstOrNull { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION }?.let { lightText(source, it) }

    // The compact text of each value argument, labels included.
    private fun argumentTexts(source: KtSourceElement, call: LighterASTNode): List<String> {
        val list = lightChildren(source, call).firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return emptyList()
        return lightChildren(source, list).filter { it.tokenType == KtNodeTypes.VALUE_ARGUMENT }.map { compact(lightText(source, it)) }
    }

    // Every block, lambda body and branch body between check and the scope also holds callStart.
    private fun reachesCall(scope: KtSourceElement, check: LighterASTNode, shift: Int, callStart: Int): Boolean {
        var node = scope.treeStructure.getParent(check)
        while (node != null && !sameNode(node, scope.lighterASTNode)) {
            if (node.tokenType in branchBodies && (node.startOffset + shift > callStart || node.endOffset + shift <= callStart)) {
                return false
            }
            node = scope.treeStructure.getParent(node)
        }
        return true
    }

    private val branchBodies = setOf(KtNodeTypes.BLOCK, KtNodeTypes.THEN, KtNodeTypes.ELSE, KtNodeTypes.BODY, KtNodeTypes.WHEN_ENTRY)

    context(context: CheckerContext)
    private fun containingFirFile(): FirFile? = context.containingElements.filterIsInstance<FirFile>().firstOrNull()

    private fun operatorAnchor(bang: Bang): KtSourceElement {
        val operator = lightChildren(bang.source, bang.postfix)
            .firstOrNull { it.tokenType == KtNodeTypes.OPERATION_REFERENCE }
        return operator?.let { lightSourceOf(it, bang.source) } ?: bang.source
    }

    context(context: CheckerContext)
    private fun message(bangs: List<Bang>): String {
        val usesTruth = containingFirFile()?.imports.orEmpty()
            .any { it.importedFqName?.asString()?.startsWith(TRUTH_PREFIX) == true }
        val scope = if (bangs.all { it.isMemberReceiver }) "assertion arguments" else "assertions"
        val checks = bangs.joinToString(" and ") {
            val operand = shortOperand(it.operandText.replace("!!", ""))
            if (usesTruth) "`assertThat($operand).isNotNull()`" else "`assertNotNull($operand)`"
        }
        return if (usesTruth) {
            "Avoid `!!` in $scope; use $checks first, then use a local val or smart-cast."
        } else {
            "Avoid `!!` in $scope; use $checks first and smart-cast."
        }
    }

    // Collapses whitespace to single spaces and cuts the text to 60 code points, as Go does for messages.
    private fun shortOperand(text: String): String {
        val collapsed = text.split(isGoSpaceDelimiter).filter { it.isNotEmpty() }.joinToString(" ")
        if (collapsed.codePointCount(0, collapsed.length) <= MAX_OPERAND_LENGTH) return collapsed
        return collapsed.substring(0, collapsed.offsetByCodePoints(0, MAX_OPERAND_LENGTH - 3)) + "..."
    }

    private fun sameNode(a: LighterASTNode, b: LighterASTNode): Boolean =
        a.startOffset == b.startOffset && a.endOffset == b.endOffset && a.tokenType == b.tokenType

    private val isGoSpaceDelimiter = Regex("[\\t\\n\\u000B\\u000C\\r \\u0085\\u00A0\\u1680\\u2000-\\u200A\\u2028\\u2029\\u202F\\u205F\\u3000]+")
}
