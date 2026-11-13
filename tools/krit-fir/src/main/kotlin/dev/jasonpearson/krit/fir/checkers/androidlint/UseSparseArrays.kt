package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinTypeProjection
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags an `android.util.SparseArray` constructor call whose only written type
// argument is Int or Boolean, where SparseIntArray or SparseBooleanArray avoids
// boxing the values. This is what Android Lint 32.4.1 reports under
// UseSparseArrays (JavaPerformanceDetector.checkSparseArray); it no longer
// reports a HashMap with an Int key.
//
// Like the Go rule:
// - the type argument must be written (`SparseArray<Int>()`), and a nullable
//   `Int?` counts as Int, as in Lint. `val d: SparseArray<Int> = SparseArray()`
//   infers its argument and is not reported;
// - a constructor reference, a subclass, and `SparseArrayCompat` are not
//   reported;
// - the finding sits on the first line of the call expression.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: the argument and the constructed class are resolved, so a typealias
//   for Int, Boolean, or SparseArray and an import alias are reported. Go reads
//   the written names only.
// - Precision: a SparseArray from another package, or an argument only named
//   Int or Boolean, is not android.util.SparseArray<kotlin.Int> and is not
//   reported. Go matches the written names and the imports.
internal object UseSparseArrays : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UseSparseArrays"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UseSparseArrays)
    }

    private val sparseArrayClassId = ClassId(FqName("android.util"), Name.identifier("SparseArray"))

    private val intClassIds = setOf(
        ClassId(FqName("kotlin"), Name.identifier("Int")),
        ClassId(FqName("java.lang"), Name.identifier("Integer")),
    )
    private val booleanClassIds = setOf(
        ClassId(FqName("kotlin"), Name.identifier("Boolean")),
        ClassId(FqName("java.lang"), Name.identifier("Boolean")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.calleeReference.toResolvedCallableSymbol() !is FirConstructorSymbol) return
        // The compiler fills in inferred type arguments too; only a written one counts.
        val written = expression.typeArguments.singleOrNull() ?: return
        if (written.source?.kind !is KtRealSourceElementKind) return
        val constructed = expression.resolvedType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType
            ?: return
        if (constructed.lookupTag.classId != sparseArrayClassId) return
        val argument = (constructed.typeArguments.singleOrNull() as? ConeKotlinTypeProjection)?.type ?: return
        val argumentClass = (argument.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType)
            ?.lookupTag?.classId
        val suggestion = when (argumentClass) {
            in intClassIds -> "SparseIntArray"
            in booleanClassIds -> "SparseBooleanArray"
            else -> return
        }
        report(expression.source, "Use `$suggestion(...)` instead for better performance")
    }
}
