package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.containingClassLookupTag
import org.jetbrains.kotlin.fir.declarations.FirDeclarationOrigin
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.references.toResolvedPropertySymbol
import org.jetbrains.kotlin.fir.resolve.providers.firProvider
import org.jetbrains.kotlin.fir.resolve.providers.getContainingFile
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.isMarkedNullable
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.unwrapFakeOverrides

/**
 * FIR `FirExpressionChecker<FirSmartCastExpression>` that records the
 * SMART-CAST-refined type (and nullability) of a stable reference at its
 * source position.
 *
 * Without this checker the oracle only saw the *declared* type of a
 * variable/property reference: [OracleQualifiedAccessChecker] visits the
 * inner [`FirPropertyAccessExpression`], whose `resolvedType` is the
 * declaration type (`Any?`), not the data-flow-refined type. FIR models a
 * smart cast as a [`FirSmartCastExpression`] that *wraps* that access, so
 * after `if (o == null) return` the use site is a smart-cast expression of
 * type `Any` (non-null). Go-side rules (CastNullableToNonNullableType, the
 * redundant-null-safety family) ask the oracle "is this reference nullable
 * here?" — they need the smart-cast answer, not the declaration answer.
 *
 * Dedup: the wrapping [`FirSmartCastExpression`] and its inner access map
 * to the same `"line:col"` key (the wrapper is source-transparent). The
 * entry is recorded with [`OracleCollector.addSmartCastExpression`], which
 * replaces a declared-type entry already at that key and is never displaced
 * by a later one, so the smart-cast type wins regardless of FIR traversal
 * order. Call sites stay authoritative because a smart cast wraps a stable
 * reference, not a call.
 *
 * A stable smart cast also records the other source files declaring the
 * properties it relies on ([declaringFiles]). The oracle compiles the whole
 * repo as one module, where a public property of another Gradle module is
 * stable; a per-module build rejects that smart cast, so the collector
 * attaches those files to the verdicts the smart cast decides and the caller
 * can drop them.
 *
 * Self-gates on `OracleCollectorRegistry.current() != null` like the other
 * oracle checkers.
 */
internal object OracleSmartCastChecker :
    FirExpressionChecker<FirSmartCastExpression>(MppCheckerKind.Common) {

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirSmartCastExpression) {
        val collector = OracleCollectorRegistry.current() ?: return
        val source = expression.source ?: return
        val filePath = context.containingFileSymbol?.fir?.sourceFile?.path ?: return
        val offsets = collector.offsetsFor(filePath) ?: return

        val startOffset = source.startOffset
        val (line, col) = offsets.lineColAt(startOffset)
        val key = "$line:$col"

        val files = if (expression.isStable) {
            (declaringFiles(expression.originalExpression, context.session, collector) - filePath).also {
                collector.addSmartCastDeclarations(filePath, startOffset, source.endOffset, it)
            }
        } else {
            emptySet()
        }

        val resolvedType = runCatching { expression.resolvedType }.getOrNull() ?: return
        val typeFqn = resolvedType.renderFqn()
        if (typeFqn.isBlank()) return

        collector.addSmartCastExpression(
            filePath,
            key,
            ExpressionPayload(
                type = typeFqn,
                nullable = resolvedType.isMarkedNullable,
                startByte = offsets.byteOffsetAt(startOffset),
                endByte = offsets.byteOffsetAt(source.endOffset),
                callTarget = null,
                callTargetResolved = false,
                callTargetSuspend = false,
                annotations = emptyList(),
                // The refined type holds only where those properties are stable: in a per-module build
                // a property of another module is not, so the caller may treat the type as unknown.
                smartCastDeclarations = files.toList(),
            ),
        )
    }

    /**
     * Source files a local `val` relies on when its inferred type comes from a smart-cast initializer
     * (`val p = a.profile` after `a.profile != null` has type `Profile`, not `Profile?`). A later `p?.x` is then
     * redundant only because of that smart cast, so the verdict needs the same stability.
     */
    @OptIn(SymbolInternals::class)
    fun localInitializerDeclarations(
        access: FirPropertyAccessExpression,
        session: FirSession,
        collector: OracleCollector,
        depth: Int = 0,
    ): Set<String> {
        val symbol = access.calleeReference.toResolvedPropertySymbol() ?: return emptySet()
        if (depth > MAX_LOCAL_DEPTH || !symbol.isLocal || !symbol.isVal) return emptySet()
        // An explicit type states the nullability itself, so no smart cast decides it.
        if (symbol.fir.returnTypeRef.source?.kind !is KtFakeSourceElementKind) return emptySet()
        val initializer = symbol.fir.initializer ?: return emptySet()
        val files = LinkedHashSet<String>()
        initializer.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                when (element) {
                    is FirAnonymousFunctionExpression -> return
                    is FirSmartCastExpression -> if (element.isStable) files += declaringFiles(element.originalExpression, session, collector)
                    is FirPropertyAccessExpression ->
                        if (element.calleeReference.toResolvedPropertySymbol()?.isLocal == true) {
                            files += localInitializerDeclarations(element, session, collector, depth + 1)
                        }
                }
                element.acceptChildren(this)
            }
        })
        return files
    }

    /**
     * Source files declaring the non-local properties along the receiver chain of [expression] (`a.b.c` relies on
     * `b` and `c`), or [UNKNOWN_DECLARATION] for one whose file cannot be found. Every link must be stable for the smart cast, so a property of another module anywhere in the
     * chain makes it impossible in a per-module build.
     */
    internal fun declaringFiles(expression: FirExpression, session: FirSession, collector: OracleCollector): Set<String> {
        val files = LinkedHashSet<String>()
        var current: FirExpression? = expression
        while (current != null) {
            current = when (current) {
                is FirSmartCastExpression -> current.originalExpression
                is FirPropertyAccessExpression -> {
                    val symbol = current.calleeReference.toResolvedPropertySymbol()?.unwrapFakeOverrides()
                    if (symbol is FirPropertySymbol && !symbol.isLocal) files += declaringFile(symbol, session, collector) ?: UNKNOWN_DECLARATION
                    current.explicitReceiver
                }
                else -> null
            }
        }
        return files
    }

    @OptIn(SymbolInternals::class)
    private fun declaringFile(symbol: FirPropertySymbol, session: FirSession, collector: OracleCollector): String? =
        if (symbol.origin == FirDeclarationOrigin.Source) {
            runCatching { session.firProvider.getContainingFile(symbol)?.sourceFile?.path }.getOrNull()
        } else {
            // A slice compile reads the module's other files as binaries; a library property has no module file.
            symbol.containingClassLookupTag()?.classId?.let { collector.binaryClassFile?.invoke(it.asFqNameString()) }
        }

    /**
     * Stands for a property whose declaring file cannot be found (a binary class missing from the build outputs). The
     * caller cannot tell which module owns it, so it treats the smart cast as unproven.
     */
    const val UNKNOWN_DECLARATION = "<unknown>"

    private const val MAX_LOCAL_DEPTH = 3
}
