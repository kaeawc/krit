package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirQualifiedAccessExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Flags a use of a member of the deprecated `android.util.FloatMath`
 * (`FloatMath.sqrt(x)`), which kotlin.math or java.lang.Math replaces.
 *
 * The Go rule reports every navigation expression whose receiver is the bare
 * identifier `FloatMath` (`FloatMath.sqrt(x)`, also split over lines), on the
 * receiver's line. (`FloatMath?.sqrt(x)` does not compile: a Java class has
 * no companion object to use as a value, so Go stays authoritative for it.)
 * This checker reports every call or callable reference that resolves to a
 * member of `android.util.FloatMath`, on its explicit receiver (the
 * `FloatMath` qualifier, so the same line as Go), or on the callee name when
 * there is no explicit receiver. Like Go, it does not report the import
 * directive, a class literal, or a type reference.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: Go matches the receiver's text, so it also reports a project
 *   class or object named `FloatMath`, another class imported under the alias
 *   `FloatMath` (`import java.lang.Math as FloatMath`), and a property, local
 *   variable, or lambda parameter named `FloatMath`. None of them is the
 *   deprecated platform class, so FIR does not report them
 *   (FloatMathLookalike.kt, FloatMathLookalikeAlias.kt).
 * - Recall: resolution sees uses Go's text match misses: a fully qualified
 *   receiver (`android.util.FloatMath.sqrt(x)`), an import alias
 *   (`import android.util.FloatMath as FM`), a typealias, a static import of
 *   a member (`import android.util.FloatMath.sqrt`), and a callable reference
 *   (`FloatMath::sqrt`). Each one still uses FloatMath (FloatMathRecall.kt).
 */
internal object FloatMath : FirQualifiedAccessExpressionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "FloatMath"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val qualifiedAccessExpressionCheckers = setOf(FloatMath)
    }

    private const val MESSAGE = "FloatMath is deprecated. Use kotlin.math or java.lang.Math instead."
    private val floatMath = ClassId(FqName("android.util"), Name.identifier("FloatMath"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirQualifiedAccessExpression) {
        val symbol = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (symbol.callableId?.classId != floatMath) return
        report(expression.explicitReceiver?.source ?: expression.calleeReference.source ?: expression.source, MESSAGE)
    }
}
