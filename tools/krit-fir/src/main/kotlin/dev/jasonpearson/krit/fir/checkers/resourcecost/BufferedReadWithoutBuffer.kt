package dev.jasonpearson.krit.fir.checkers.resourcecost

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.security.assignedLocalValues
import dev.jasonpearson.krit.fir.checkers.security.isLocalVar
import dev.jasonpearson.krit.fir.checkers.security.reassignmentsAll
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.containingClassLookupTag
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirSuperReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go BufferedReadWithoutBuffer rule: `read(...)` on an input
 * stream whose bytes come straight from a `java.io.FileInputStream`, with no
 * `BufferedInputStream` in between. Reported on the call, like Go.
 *
 * Mirrored from Go:
 * - the call is named `read` and has an explicit receiver (a bare `read()`,
 *   `this.read()` or `super.read()` is not reported, as Go finds no receiver
 *   evidence there);
 * - the receiver is a FileInputStream constructor call (`FileInputStream(p)
 *   .read()`), a local initialized with one (`val s: InputStream =
 *   FileInputStream(p)`), or an expression that wraps one (`DataInputStream(
 *   FileInputStream(p)).read()`, `(FileInputStream(p) as InputStream).read()`);
 * - a `.buffered()` call or a `BufferedInputStream` in the stream chain makes
 *   the read buffered.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: the read must be a member `read` of a `java.io.InputStream`
 *   subtype, and the FileInputStream must be the source of the receiver
 *   stream. Go accepts any call named `read` whose receiver text contains a
 *   call named `FileInputStream`, so it also reports a Reader
 *   (`FileInputStream(p).reader().read()`, `.bufferedReader().read()`,
 *   `InputStreamReader(FileInputStream(p)).read()`), a FileChannel
 *   (`FileInputStream(p).channel.read(buffer)`), an in-memory stream built
 *   from the file's bytes (`ByteArrayInputStream(FileInputStream(p)
 *   .readBytes()).read()`), an extension function named `read`, a class of
 *   another package named FileInputStream, and a local var reassigned to a
 *   buffered stream. None of those reads a FileInputStream unbuffered.
 * - Recall: the receiver's resolved type proves a FileInputStream where Go
 *   has no source evidence: a parameter or property typed FileInputStream (or
 *   a subclass), a lambda parameter (`FileInputStream(p).use { it.read() }`),
 *   `File.inputStream()`, a smart cast, an import or type alias, a local
 *   initialized with a wrapper (`val s = DataInputStream(FileInputStream(p))`),
 *   and a FileInputStream receiver whose chain calls `buffered()` on a side
 *   branch (`FileInputStream(p).also { it.buffered() }.read()`).
 */
internal object BufferedReadWithoutBuffer : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "BufferedReadWithoutBuffer"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(BufferedReadWithoutBuffer)
    }

    private const val MESSAGE =
        "FileInputStream.read() without BufferedInputStream; wrap in .buffered() for efficient reads."

    private val READ = Name.identifier("read")
    private val INPUT_STREAM = streamType("java/io/InputStream")
    private val FILE_INPUT_STREAM = streamType("java/io/FileInputStream")
    private val BUFFERED_INPUT_STREAM = streamType("java/io/BufferedInputStream")

    // Bounds the recursion through wrappers and local initializers.
    private const val MAX_DEPTH = 32

    private fun streamType(id: String): ConeKotlinType =
        ClassId.fromString(id).constructClassLikeType(emptyArray(), isMarkedNullable = true)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (callee.name != READ) return
        // A member read of the stream, not an extension named read.
        if (callee.receiverParameterSymbol != null || callee.containingClassLookupTag() == null) return
        val receiver = expression.explicitReceiver ?: return
        if (receiver is FirThisReceiverExpression || receiver is FirSuperReceiverExpression) return
        if (!isSubtype(receiver.resolvedType, INPUT_STREAM)) return
        if (!Flow { assignedLocalValues() }.readsFileStream(receiver)) return
        report(expression.source, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isSubtype(type: ConeKotlinType, of: ConeKotlinType): Boolean = type.isSubtypeOf(of, context.session)

    // One receiver's evaluation: tracks the local variables being resolved so
    // a cycle through initializers ends, and treats a read of a local var
    // inside a value assigned to it (`s = DataInputStream(s)`) as its earlier
    // value, a FileInputStream by induction when its other values are.
    private class Flow(assigned: () -> Map<FirBasedSymbol<*>, List<FirExpression>>?) {
        private val assignedOnce by lazy(LazyThreadSafetyMode.NONE, assigned)
        private val visiting = HashSet<FirPropertySymbol>()
        private val reassigning = HashSet<FirPropertySymbol>()
        private var depth = 0

        // True when [expression] is an input stream reading a FileInputStream
        // without a BufferedInputStream: a FileInputStream (by its resolved
        // type), a stream built from one (a constructor or call that takes it
        // as its receiver or an argument), a local initialized with one, or a
        // branch of an if/when/elvis that is one.
        context(context: CheckerContext)
        fun readsFileStream(expression: FirExpression): Boolean {
            if (depth >= MAX_DEPTH) return false
            depth++
            try {
                val type = expression.resolvedType
                if (isSubtype(type, FILE_INPUT_STREAM)) return true
                if (isSubtype(type, BUFFERED_INPUT_STREAM)) return false
                return when (expression) {
                    is FirWrappedArgumentExpression -> readsFileStream(expression.expression)
                    is FirSmartCastExpression -> readsFileStream(expression.originalExpression)
                    is FirCheckedSafeCallSubject -> readsFileStream(expression.originalReceiverRef.value)
                    is FirCheckNotNullCall -> expression.argumentList.arguments.any { readsFileStream(it) }
                    is FirTypeOperatorCall ->
                        (expression.operation == FirOperation.AS || expression.operation == FirOperation.SAFE_AS) &&
                            expression.argumentList.arguments.any { readsFileStream(it) }
                    else -> isInputStream(type) && readsFileStreamValue(expression)
                }
            } finally {
                depth--
            }
        }

        context(context: CheckerContext)
        private fun isInputStream(type: ConeKotlinType): Boolean = isSubtype(type, INPUT_STREAM)

        context(context: CheckerContext)
        private fun readsFileStreamValue(expression: FirExpression): Boolean = when (expression) {
            is FirFunctionCall -> {
                val arguments = expression.argumentList.arguments.flatMap {
                    if (it is FirVarargArgumentsExpression) it.arguments else listOf(it)
                }
                listOfNotNull(expression.explicitReceiver).plus(arguments).any { readsFileStream(it) }
            }
            is FirPropertyAccessExpression -> readsFileStreamLocal(expression)
            is FirWhenExpression -> expression.branches.any { readsFileStreamResult(it.result) }
            is FirElvisExpression -> readsFileStream(expression.lhs) || readsFileStream(expression.rhs)
            else -> false
        }

        context(context: CheckerContext)
        private fun readsFileStreamResult(block: FirBlock): Boolean {
            val last = block.statements.lastOrNull() as? FirExpression ?: return false
            return readsFileStream(last)
        }

        // A local val initialized with a FileInputStream source; a local var
        // only when every value assigned to it is one too.
        context(context: CheckerContext)
        private fun readsFileStreamLocal(access: FirPropertyAccessExpression): Boolean {
            if (access.explicitReceiver != null) return false
            val symbol = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return false
            if (!symbol.isLocal) return false
            if (symbol in reassigning) return true
            if (!visiting.add(symbol)) return false
            try {
                val initializer = symbol.resolvedInitializer ?: return false
                if (!readsFileStream(initializer)) return false
                if (!isLocalVar(symbol)) return true
                reassigning += symbol
                try {
                    return reassignmentsAll(symbol, assignedOnce) { readsFileStream(it) }
                } finally {
                    reassigning -= symbol
                }
            } finally {
                visiting -= symbol
            }
        }
    }
}
