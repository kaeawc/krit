package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.qualifiedCall
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.Name

// Flags a call to `java.lang.System.gc`, `System.runFinalization`, `Runtime.gc` or `Runtime.runFinalization`.
//
// Mirrors the Go ExplicitGarbageCollectionCall rule's Kotlin path: the finding is reported on the line where the whole
// call starts (the qualifier included) with Go's message, and test files are skipped (a test may call gc() to force
// finalization or to measure leaks); the test classification comes from the check request.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: the call must resolve to one of the JDK functions above. Go compares the call's text with
//   `System.gc()` and `Runtime.getRuntime().gc()`, so a project object or class named `System` or `Runtime` that
//   declares its own `gc()` is reported.
// - Recall: an import alias of the JDK class, a type alias, a `Runtime` held in a variable and a call on an implicit
//   `Runtime` receiver still run the collector; Go only sees the literal spellings.
internal object ExplicitGarbageCollectionCall : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ExplicitGarbageCollectionCall"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ExplicitGarbageCollectionCall)
    }

    private const val MESSAGE =
        "Do not call garbage collector explicitly. It is rarely necessary and can degrade performance."

    private val collectors = listOf("System", "Runtime").flatMap { owner ->
        listOf("gc", "runFinalization").map { CallableId(classIdOf("java.lang", owner), Name.identifier(it)) }
    }.toSet()

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callableId = expression.calleeReference.toResolvedCallableSymbol()?.callableId ?: return
        if (callableId !in collectors || isInTestFile()) return
        val source = expression.source ?: return
        report(qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source, MESSAGE)
    }
}
