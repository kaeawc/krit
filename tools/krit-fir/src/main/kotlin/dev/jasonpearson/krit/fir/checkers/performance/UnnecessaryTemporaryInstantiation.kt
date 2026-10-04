package dev.jasonpearson.krit.fir.checkers.performance

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags `toString()` called directly on a JDK boxed-primitive conversion,
// `Integer.valueOf(x).toString()` or `java.lang.Integer.parseInt(s).toString()`:
// the temporary wrapper (or parsed value) exists only to be turned back into a
// String.
//
// Like the Go rule:
// - the conversion is `valueOf`, `parseInt`, `parseLong`, `parseFloat`, or
//   `parseDouble` of `java.lang.Integer`, `Long`, `Short`, `Byte`, `Float`,
//   `Double`, `Boolean`, or `Character`, with at least one argument (a radix
//   or a String argument counts too);
// - the `toString` call is any call named toString whose explicit receiver is
//   that conversion (`toString(radix)` and a safe call `?.toString()` count);
// - `toString()` on anything else (`x.toString()`, `Integer.valueOf(x)` stored
//   in a variable first, an implicit `with(Integer.valueOf(x)) { toString() }`
//   receiver) is not reported;
// - the finding sits on the first line of the whole call chain.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: the conversion is identified by resolution, so an import alias
//   (`import java.lang.Integer as JInt`), a typealias of the wrapper, a
//   statically imported `valueOf(x)`/`parseInt(s)`, and a parenthesized
//   conversion `(Integer.valueOf(x)).toString()` are reported. Go needs the
//   literal wrapper name as the last qualifier segment of the call and the
//   call itself as the direct receiver of toString.
// - Precision: a same-package class or object named like a wrapper
//   (`object Integer { fun valueOf(x: Int): Int }`) is not java.lang.Integer
//   and its call instantiates no wrapper, so it is not reported. Go matches
//   the qualifier text alone.
internal object UnnecessaryTemporaryInstantiation : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnnecessaryTemporaryInstantiation"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UnnecessaryTemporaryInstantiation)
    }

    private const val MESSAGE =
        "Unnecessary temporary instantiation. Use the type's toString() or conversion method directly."

    private val toStringName = Name.identifier("toString")

    private val javaLang = FqName("java.lang")
    private val wrapperClassIds: Set<ClassId> =
        listOf("Integer", "Long", "Short", "Byte", "Float", "Double", "Boolean", "Character")
            .mapTo(HashSet()) { ClassId(javaLang, Name.identifier(it)) }

    private val conversionNames: Set<Name> =
        listOf("valueOf", "parseInt", "parseLong", "parseFloat", "parseDouble")
            .mapTo(HashSet()) { Name.identifier(it) }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.calleeReference.name != toStringName) return
        val receiver = when (val explicit = expression.explicitReceiver) {
            is FirCheckedSafeCallSubject -> explicit.originalReceiverRef.value
            else -> explicit
        }
        val conversion = receiver as? FirFunctionCall ?: return
        if (conversion.argumentList.arguments.isEmpty()) return
        val symbol = conversion.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        val callableId = symbol.callableId
        if (callableId.callableName !in conversionNames) return
        if (callableId.classId !in wrapperClassIds) return
        report(expression.source, MESSAGE)
    }
}
