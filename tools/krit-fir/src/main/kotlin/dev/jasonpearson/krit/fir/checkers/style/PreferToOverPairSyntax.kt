package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.name.ClassId

// Flags a call to the kotlin.Pair constructor, `Pair(a, b)`: `a to b` says the
// same thing with the infix function.
//
// Mirrors the Go PreferToOverPairSyntax rule and detekt's rule of the same name,
// which resolves the call and requires the constructor of kotlin.Pair. The
// message lists the written arguments joined by " to ", as detekt does. FIR
// decides on resolution instead of Go's name, import and declaration checks:
// - a call that resolves to another class, function or member named `Pair` is
//   not flagged (Go skips file-local and imported lookalikes, but reports a
//   member of an implicit receiver spelled `Pair`);
// - a call through a typealias to kotlin.Pair is not flagged, like detekt (its
//   constructor is a typealias constructor, not the class constructor), and Go
//   does not match its name either;
// - a call through an import alias of kotlin.Pair (`import kotlin.Pair as P`)
//   is flagged, like detekt.
internal object PreferToOverPairSyntax : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "PreferToOverPairSyntax"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(PreferToOverPairSyntax)
    }

    private val PAIR = ClassId.fromString("kotlin/Pair")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    @OptIn(SymbolInternals::class)
    override fun check(expression: FirFunctionCall) {
        val constructor = expression.calleeReference.toResolvedConstructorSymbol() ?: return
        if (constructor.callableId.classId != PAIR) return
        val source = expression.source ?: return
        // A typealias constructor reports the class constructor's callable id too, so the
        // written callee has to name the class itself or an import alias of it.
        val written = writtenCallee(source) ?: return
        if (!namesPair(written, context)) return
        val arguments = writtenArguments(source) ?: return
        if (arguments.size != 2) return
        val anchor = expression.calleeReference.source ?: source
        report(
            anchor,
            "Pair is created by using the pair constructor. This can replaced by `${arguments.joinToString(" to ")}`.",
        )
    }

    // The CALL_EXPRESSION node: the source itself, or the selector of a qualified call.
    private fun callNode(source: org.jetbrains.kotlin.KtSourceElement) =
        source.lighterASTNode.let { node ->
            if (node.tokenType == KtNodeTypes.CALL_EXPRESSION) node
            else significantChildren(source, node).lastOrNull()?.takeIf { it.tokenType == KtNodeTypes.CALL_EXPRESSION }
        }

    // The callee reference text and its qualifier: `Pair`, or `kotlin.Pair` for a qualified call.
    private fun writtenCallee(source: org.jetbrains.kotlin.KtSourceElement): String? {
        val call = callNode(source) ?: return null
        val name = significantChildren(source, call).firstOrNull { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION }
            ?: return null
        val simple = lightText(source, name).removeSurrounding("`")
        val node = source.lighterASTNode
        if (node.tokenType == KtNodeTypes.CALL_EXPRESSION) return simple
        val receiver = significantChildren(source, node).firstOrNull() ?: return null
        return lightText(source, receiver) + "." + simple
    }

    private fun namesPair(written: String, context: CheckerContext): Boolean = when (written) {
        "Pair", "kotlin.Pair" -> true
        else -> {
            // An unqualified name other than Pair is an import alias or a typealias.
            !written.contains('.') && importAliasTargetsPair(written, context)
        }
    }

    @OptIn(SymbolInternals::class)
    private fun importAliasTargetsPair(name: String, context: CheckerContext): Boolean {
        val imports = context.containingFileSymbol?.fir?.imports.orEmpty()
        return imports.any { it.aliasName?.asString() == name && it.importedFqName?.asString() == "kotlin.Pair" }
    }

    // The value arguments as written, with a trailing lambda last.
    private fun writtenArguments(source: org.jetbrains.kotlin.KtSourceElement): List<String>? {
        val call = callNode(source) ?: return null
        val result = mutableListOf<String>()
        for (child in lightChildren(source, call)) {
            when (child.tokenType) {
                KtNodeTypes.VALUE_ARGUMENT_LIST ->
                    significantChildren(source, child)
                        .filter { it.tokenType == KtNodeTypes.VALUE_ARGUMENT }
                        .mapTo(result) { lightText(source, it) }
                KtNodeTypes.LAMBDA_ARGUMENT -> result.add(lightText(source, child))
            }
        }
        return result
    }
}
