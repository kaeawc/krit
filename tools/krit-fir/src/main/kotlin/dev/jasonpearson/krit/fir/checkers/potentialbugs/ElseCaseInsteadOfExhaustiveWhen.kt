package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirWhenExpressionChecker
import org.jetbrains.kotlin.fir.expressions.ExhaustivenessStatus
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.impl.FirElseIfTrueCondition
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isNothing
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Flags a `when` on a sealed, enum or Boolean subject whose `else` branch is redundant: the other
 * branches already cover every case, which is the compiler's own exhaustiveness verdict
 * (`RedundantlyExhaustive`). An `else` that never completes (`else -> error(...)`) is a deliberate
 * guard against future cases and is not flagged, nor is a subject type listed in
 * `ignoredSubjectTypes`.
 *
 * Go decides coverage by matching branch type names against the variants its resolver lists, so it
 * misses subjects it cannot type and branches written through aliases, qualified names or nested
 * sealed hierarchies the compiler understands. The compiler's verdict also counts a nullable
 * subject as covered only when a `null` branch exists, as Go does.
 */
internal object ElseCaseInsteadOfExhaustiveWhen : FirWhenExpressionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ElseCaseInsteadOfExhaustiveWhen"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val whenExpressionCheckers = setOf(ElseCaseInsteadOfExhaustiveWhen)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirWhenExpression) {
        if (expression.exhaustivenessStatus !is ExhaustivenessStatus.RedundantlyExhaustive) return
        val subject = expression.subjectVariable ?: return
        val elseBranch = expression.branches.firstOrNull { it.condition is FirElseIfTrueCondition } ?: return
        if (elseBranch.result.resolvedType.isNothing) return
        val type = subject.returnTypeRef.coneType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType
            ?: return
        val classId = type.lookupTag.classId
        val ignored = (config()["ignoredSubjectTypes"] as? List<*>)?.mapNotNull { (it as? String)?.trim() }.orEmpty()
        if (classId.asFqNameString() in ignored) return
        val symbol = type.lookupTag.toClassSymbol(context.session)
        val kind = when {
            classId == StandardClassIds.Boolean -> "boolean"
            symbol?.classKind == ClassKind.ENUM_CLASS -> "enum class"
            symbol?.resolvedStatus?.modality == Modality.SEALED -> "sealed class"
            else -> return
        }
        report(expression.source ?: return, "When expression with $kind subject should not contain an `else` case.")
    }
}
