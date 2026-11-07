package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirResolvedImport
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedBaseSymbol
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.providers.symbolProvider
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.coneType

/**
 * Checkers that record which operands and statements take their type from an error type, so
 * [OracleCollector] can drop the compiler verdicts that rest on it. A class from a module whose build output is
 * missing becomes an error type, and the compiler treats that as non-null, as a subtype of everything and as
 * `Nothing`: a "useless" safe call, elvis, cast or comparison on it, or code "unreachable" after it, is a guess.
 *
 * Each checker self-gates on `OracleCollectorRegistry.current() != null` like the other oracle checkers.
 */

/** Records [operand] on the active collector when its type derives from an error type. */
@OptIn(SymbolInternals::class)
context(context: CheckerContext)
private fun recordOperand(operand: FirExpression?, force: Boolean = false) {
    val collector = OracleCollectorRegistry.current() ?: return
    if (operand == null || !(force || ErrorInvolvement.involvesError(operand) || restsOnShadowedContract(collector, operand))) return
    val source = operand.source ?: return
    val filePath = context.containingFileSymbol?.fir?.sourceFile?.path ?: return
    collector.addErrorTainted(filePath, source.startOffset, source.endOffset)
}

internal object OracleSafeCallTaintChecker : FirExpressionChecker<FirSafeCallExpression>(MppCheckerKind.Common) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirSafeCallExpression) {
        if (OracleCollectorRegistry.current() == null) return
        recordOperand(expression.receiver)
    }
}

internal object OracleCheckNotNullTaintChecker : FirExpressionChecker<FirCheckNotNullCall>(MppCheckerKind.Common) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirCheckNotNullCall) {
        if (OracleCollectorRegistry.current() == null) return
        recordOperand(expression.argumentList.arguments.firstOrNull())
    }
}

internal object OracleElvisTaintChecker : FirExpressionChecker<FirElvisExpression>(MppCheckerKind.Common) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirElvisExpression) {
        if (OracleCollectorRegistry.current() == null) return
        recordOperand(expression.lhs)
    }
}

internal object OracleTypeOperatorTaintChecker : FirExpressionChecker<FirTypeOperatorCall>(MppCheckerKind.Common) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirTypeOperatorCall) {
        if (OracleCollectorRegistry.current() == null) return
        // A cast to a type the compilation cannot see is as unreliable as a cast of one.
        val targetIsError = runCatching { expression.conversionTypeRef.coneType }.getOrNull() is ConeErrorType
        recordOperand(expression.argumentList.arguments.firstOrNull(), force = targetIsError)
    }
}

internal object OracleEqualityTaintChecker : FirExpressionChecker<FirEqualityOperatorCall>(MppCheckerKind.Common) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirEqualityOperatorCall) {
        if (OracleCollectorRegistry.current() == null) return
        expression.argumentList.arguments.forEach { recordOperand(it) }
    }
}

/** Records the statements of a block whose type derives from an error type: the compiler treats them as `Nothing`. */
internal object OracleBlockTaintChecker : FirExpressionChecker<FirBlock>(MppCheckerKind.Common) {
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirBlock) {
        val collector = OracleCollectorRegistry.current() ?: return
        val filePath = context.containingFileSymbol?.fir?.sourceFile?.path ?: return
        for (statement in expression.statements) {
            val involved = ErrorInvolvement.errorExpressionOf(statement) ?: continue
            val source = involved.source ?: continue
            collector.addErrorTaintedStatement(filePath, source.startOffset, source.endOffset)
        }
    }
}

/** Whether a smart cast on [operand] (or on a safe-call receiver inside it) came from a call recorded as shadowed. */
private fun restsOnShadowedContract(collector: OracleCollector, operand: FirExpression): Boolean = when (operand) {
    is FirSmartCastExpression ->
        referencedSymbol(operand.originalExpression)?.let(collector::isShadowedContractSymbol) == true ||
            restsOnShadowedContract(collector, operand.originalExpression)
    is FirSafeCallExpression -> restsOnShadowedContract(collector, operand.receiver)
    is FirCheckedSafeCallSubject -> restsOnShadowedContract(collector, operand.originalReceiverRef.value)
    else -> false
}

private fun referencedSymbol(expression: FirExpression): Any? =
    (expression as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedBaseSymbol()

/**
 * Records the variables passed to a call whose callee an unresolved import or an unresolved supertype may have
 * shadowed. A test helper such as `assertNotNull` imported from a module whose build output is missing leaves the
 * name free, and the compiler binds `kotlin.test.assertNotNull`, whose contract then smart casts the argument.
 */
internal object OracleShadowedCallTaintChecker : FirExpressionChecker<FirFunctionCall>(MppCheckerKind.Common) {
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val collector = OracleCollectorRegistry.current() ?: return
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        // A member or local callee cannot be an import's target; only a top-level function can take its place.
        val id = callee.callableId ?: return
        if (id.classId != null || id.isLocal) return
        val name = id.callableName
        if (!nameMayBeShadowed(name)) return
        val candidates = expression.argumentList.arguments + listOfNotNull(expression.extensionReceiver, expression.explicitReceiver)
        for (candidate in candidates) {
            val argument = (candidate as? FirWrappedArgumentExpression)?.expression ?: candidate
            referencedSymbol(argument)?.let(collector::addShadowedContractSymbol)
        }
    }

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun nameMayBeShadowed(name: Name): Boolean =
        hasUnresolvedImportOf(name) || context.containingDeclarations.any { (it.fir as? FirClass)?.let(::hasErrorSupertype) == true }

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun hasUnresolvedImportOf(name: Name): Boolean {
        val file = context.containingFileSymbol?.fir ?: return false
        val session = context.session
        return file.imports.any { import ->
            if (import.isAllUnder) return@any false
            val fqName = import.importedFqName ?: return@any false
            val alias = import.aliasName
            if ((alias ?: fqName.shortName()) != name) return@any false
            val resolved = import as? FirResolvedImport ?: return@any true
            val parent = resolved.resolvedParentClassId
            if (parent != null) return@any session.symbolProvider.getClassLikeSymbolByClassId(parent) == null
            val pkg = resolved.packageFqName
            val target = fqName.shortName()
            session.symbolProvider.getTopLevelCallableSymbols(pkg, target).isEmpty() &&
                session.symbolProvider.getClassLikeSymbolByClassId(ClassId(pkg, target)) == null
        }
    }

    private fun hasErrorSupertype(declaration: FirClass): Boolean =
        declaration.superTypeRefs.any { runCatching { it.coneType }.getOrNull() is ConeErrorType }
}
