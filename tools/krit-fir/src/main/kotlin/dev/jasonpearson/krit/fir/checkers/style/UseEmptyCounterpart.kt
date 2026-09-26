package dev.jasonpearson.krit.fir.checkers.style

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirArgumentList
import org.jetbrains.kotlin.fir.expressions.FirCollectionLiteral
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a stdlib collection factory called with no elements (`listOf()`,
// `listOfNotNull()`, `setOf()`, `mapOf()`, `arrayOf()`, `sequenceOf()`): the
// empty counterpart (`emptyList()`, `emptySet()`, ...) says the same thing
// directly.
//
// Mirrors the Go UseEmptyCounterpart rule, which matches the written shape: a
// call whose callee is a plain identifier spelled like one of the factories,
// with a parenthesized argument list that holds no value argument. FIR decides
// on resolution instead, with the Go message and the finding on the callee's
// line:
// - the call must resolve to the stdlib factory, so a same-package, local or
//   member function named `listOf` is not flagged (Go reports any callee with
//   that name);
// - the call must pass no element at all, so a trailing lambda, which Kotlin
//   passes as the single vararg element (`listOf() { 1 }` is a one-element
//   list), is not flagged (Go only counts the arguments inside the
//   parentheses);
// - a qualified (`kotlin.collections.listOf()`), backticked or import-aliased
//   call to the factory is flagged (Go only matches a plain identifier with
//   the factory's spelling). The message names the factory, not the alias.
// Inside an annotation argument (and an annotation parameter's default) K2
// turns `arrayOf()` into a collection literal, not a function call, so
// [AnnotationArrayOf] reports those the way Go does: `emptyArray()` is an
// accepted annotation argument too.
// The golden data pins both directions.
internal object UseEmptyCounterpart : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseEmptyCounterpart"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UseEmptyCounterpart)
        override val collectionLiteralCheckers = setOf(AnnotationArrayOf)
    }

    private val COLLECTIONS = FqName("kotlin.collections")
    private val KOTLIN = FqName("kotlin")
    private val SEQUENCES = FqName("kotlin.sequences")
    private val ARRAY_OF = Name.identifier("arrayOf")

    private val counterparts: Map<CallableId, String> = mapOf(
        CallableId(COLLECTIONS, Name.identifier("listOf")) to "emptyList",
        CallableId(COLLECTIONS, Name.identifier("listOfNotNull")) to "emptyList",
        CallableId(COLLECTIONS, Name.identifier("setOf")) to "emptySet",
        CallableId(COLLECTIONS, Name.identifier("mapOf")) to "emptyMap",
        CallableId(KOTLIN, ARRAY_OF) to "emptyArray",
        CallableId(SEQUENCES, Name.identifier("sequenceOf")) to "emptySequence",
    )

    private fun message(factory: Name, replacement: String) =
        "Use '$replacement()' instead of '${factory.asString()}()'."

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        val replacement = counterparts[callableId] ?: return
        if (passesElement(expression.argumentList)) return
        val source = expression.calleeReference.source ?: expression.source ?: return
        report(source, message(callableId.callableName, replacement))
    }

    // Whether the call passes any element: a non-empty vararg, a trailing
    // lambda, or any other argument.
    private fun passesElement(arguments: FirArgumentList): Boolean = arguments.arguments.any {
        it !is FirVarargArgumentsExpression || it.arguments.isNotEmpty()
    }

    // `arrayOf()` in an annotation argument. K2 resolves the call and then
    // replaces it with a collection literal that keeps the call's source, so
    // the literal no longer names its function. Only the stdlib array
    // factories (`arrayOf`, `emptyArray`, `intArrayOf`, ...) are allowed there,
    // and `[]` has a collection-literal source, so the callee's written name,
    // mapped through an import alias, tells which factory it was.
    private object AnnotationArrayOf : FirExpressionChecker<FirCollectionLiteral>(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirCollectionLiteral) {
            if (passesElement(expression.argumentList)) return
            val source = expression.source ?: return
            val callee = calleeNode(source, source.lighterASTNode) ?: return
            if (factoryName(lightText(source, callee).removeSurrounding("`")) != ARRAY_OF) return
            report(lightSourceOf(callee, source), message(ARRAY_OF, "emptyArray"))
        }

        // The callee REFERENCE_EXPRESSION of a call source: the call itself,
        // or the selector of a qualified call (`kotlin.arrayOf()`).
        private fun calleeNode(source: KtSourceElement, node: LighterASTNode): LighterASTNode? = when (node.tokenType) {
            KtNodeTypes.CALL_EXPRESSION ->
                significantChildren(source, node).firstOrNull()?.takeIf { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION }
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION ->
                significantChildren(source, node).lastOrNull()?.let { calleeNode(source, it) }
            else -> null
        }

        // The name of the function a written callee name stands for: the
        // imported name when the file imports it under that alias.
        @OptIn(SymbolInternals::class)
        context(context: CheckerContext)
        private fun factoryName(written: String): Name {
            val imports = context.containingFileSymbol?.fir?.imports.orEmpty()
            val aliased = imports.firstOrNull { it.aliasName?.asString() == written }
            return aliased?.importedFqName?.shortName() ?: Name.identifier(written)
        }
    }
}
