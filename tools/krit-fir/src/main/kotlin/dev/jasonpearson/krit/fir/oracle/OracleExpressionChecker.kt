package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.resolvedType

/**
 * FIR `FirFunctionCallChecker` that projects each visited
 * [FirFunctionCall] into an [ExpressionPayload] and pushes it onto the
 * active [OracleCollector], if any. Mirrors krit-types' per-call
 * projection: only callable resolution is exposed (no expression type
 * or nullability), since the Go-side rules consuming this data only
 * read `callTarget`, `callTargetSuspend`, and annotations.
 *
 * The checker gates itself on `OracleCollectorRegistry.current() != null`
 * so plugin-only compilations (the diagnostic `check()` command) pay
 * only a thread-local probe per call.
 */
internal object OracleExpressionChecker : FirFunctionCallChecker(MppCheckerKind.Common) {

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val collector = OracleCollectorRegistry.current() ?: return
        val source = expression.source ?: return
        val filePath = context.containingFileSymbol?.fir?.sourceFile?.path ?: return
        val offsets = collector.offsetsFor(filePath) ?: return

        val startOffset = source.startOffset
        val endOffset = source.endOffset
        val (line, col) = offsets.lineColAt(startOffset)
        val key = "$line:$col"
        if (collector.hasExpression(filePath, key)) return

        val callee = expression.calleeReference.toResolvedCallableSymbol()
        val lexicalFallback = expression.calleeReference.name.asString()

        var callTarget: String = lexicalFallback
        var callTargetResolved = false
        var callTargetSuspend = false
        var annotations: List<String> = emptyList()
        if (callee != null) {
            val fqn = callee.callableId?.asSingleFqName()?.asString().orEmpty()
            if (fqn.isNotEmpty()) {
                callTarget = fqn
                callTargetResolved = true
                callTargetSuspend = callee.isSuspend
                annotations = callee.annotations.mapNotNull {
                    it.toAnnotationClassId(context.session)?.asSingleFqName()?.asString()
                }
            }
        }

        if (callTarget.isEmpty()) return

        // The call's `resolvedType` is the type the rule sees through
        // `Resolver.expressionType(call)`. We render the same
        // classId-FQN format krit-types' AnalysisApiResolver returns
        // so a rule jar shipped against either backend sees identical
        // strings. Pre-2.6 the oracle pass left this empty because no
        // Go-side rule consumed it; now the FIR-backed Resolver does.
        val resolvedType = runCatching { expression.resolvedType }.getOrNull()
        val typeFqn = resolvedType?.renderFqn().orEmpty()
        val typeNullable = resolvedType?.isMarkedNullable == true
        val payload = ExpressionPayload(
            type = typeFqn,
            nullable = typeNullable,
            startByte = offsets.byteOffsetAt(startOffset),
            endByte = offsets.byteOffsetAt(endOffset),
            callTarget = callTarget,
            callTargetResolved = callTargetResolved,
            callTargetSuspend = callTargetSuspend,
            annotations = annotations,
        )
        collector.addExpression(filePath, key, payload)
        recordLambdaSuspendArguments(collector, filePath, offsets, expression, callee)
    }

    /**
     * Inspect the argument list and, for each lambda passed positionally,
     * record whether the matching value parameter declares a suspend
     * functional type. Mirrors krit-types' "is this lambda passed to a
     * `suspend (...) -> R` parameter" heuristic but does the work at
     * the call site, where FIR's resolved argument mapping carries
     * accurate parameter types — checking the lambda's own
     * `FirAnonymousFunction.status.isSuspend` doesn't work because FIR
     * doesn't propagate the suspend conversion onto the lambda
     * declaration in 2.3.21.
     *
     * Non-call-site lambdas (e.g. `val f: suspend () -> Unit = { ... }`)
     * aren't visited here; the resolver's "missing key → false" default
     * matches krit-types' "unresolved → conservative false" contract.
     */
    @OptIn(SymbolInternals::class)
    private fun recordLambdaSuspendArguments(
        collector: OracleCollector,
        filePath: String,
        offsets: FileOffsetTable,
        call: FirFunctionCall,
        callee: org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol<*>?,
    ) {
        if (callee !is org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol) return
        val paramTypes = callee.fir.valueParameters
        val args = call.argumentList.arguments
        // Walk argument-position-for-position. Lambdas appear as
        // FirAnonymousFunctionExpression — wrappers around the lambda's
        // FirAnonymousFunction declaration. Matching by index is
        // approximate (named args + defaults can shift positions), but
        // works for the dominant block-builder shape rules care about.
        val n = minOf(args.size, paramTypes.size)
        for (i in 0 until n) {
            val arg = args[i] as? FirAnonymousFunctionExpression ?: continue
            val lambdaSource = arg.anonymousFunction.source ?: continue
            val (line, col) = offsets.lineColAt(lambdaSource.startOffset)
            val paramTypeRef = paramTypes[i].returnTypeRef as? FirResolvedTypeRef ?: continue
            val paramCone = paramTypeRef.coneType as? ConeClassLikeType ?: continue
            val classId = paramCone.lookupTag.classId.asFqNameString()
            val isSuspendFunctional = classId.startsWith("kotlin.coroutines.SuspendFunction") ||
                classId.startsWith("kotlin.coroutines.KSuspendFunction")
            collector.addLambdaSuspend(filePath, "$line:$col", isSuspendFunctional)
        }
    }
}
