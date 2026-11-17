package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.nestedClassIdOf
import dev.jasonpearson.krit.fir.support.qualifiedCall
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.Name

/**
 * Flags `acquire()` with no timeout on a `PowerManager.WakeLock`, as Android Lint's WakelockTimeout does: a call
 * written `acquire` with no arguments and an explicit receiver (`wl.acquire()`, `wl?.acquire()`, a chain on
 * `newWakeLock(...)`). The finding sits on the first line of the call expression, receiver included, where Go reports
 * it, with Go's message.
 *
 * Go types the receiver from the oracle, from a declared type it matches by name through the file's imports, or from
 * an earlier `newWakeLock` initializer in the function. This checker reads the receiver's resolved type instead: it
 * must be `android.os.PowerManager.WakeLock`.
 *
 * Like every Android Lint rule in Go, it skips test files (a test that verifies `acquire()` on a mock requests no
 * wake lock).
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Recall: a receiver Go cannot type is still a WakeLock where the compiler knows it (a property or call result of
 *   another file, `!!`, a smart cast, a type alias or an import alias of the class, an inferred local), and so is an
 *   implicit receiver (`with(lock) { acquire() }`, an extension on WakeLock), which Go skips for lack of a written one.
 * - Precision: a variable that Go's `newWakeLock` fallback matches by name only (a shadowing local of another type
 *   declared after a WakeLock of the same name) is not a WakeLock, so it is not reported.
 */
internal object WakelockTimeout : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WakelockTimeout"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(WakelockTimeout)
    }

    private const val MESSAGE =
        "Provide a timeout when requesting a wakelock with `PowerManager.Wakelock.acquire(long timeout)`. " +
            "This will ensure the OS will cleanup any wakelocks that last longer than you intend, and will save " +
            "your user's battery."

    private val acquire = Name.identifier("acquire")
    private val wakeLock = nestedClassIdOf("android.os", "PowerManager.WakeLock")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.calleeReference.name != acquire || expression.argumentList.arguments.isNotEmpty()) return
        if (isInTestFile()) return
        val receiver = expression.explicitReceiver ?: expression.dispatchReceiver
            ?: expression.extensionReceiver ?: return
        if (receiver.resolvedType.expandedClassType()?.lookupTag?.classId != wakeLock) return
        val source = expression.source ?: return
        report(qualifiedCall(source)?.let { lightSourceOf(it, source) } ?: source, MESSAGE)
    }
}
