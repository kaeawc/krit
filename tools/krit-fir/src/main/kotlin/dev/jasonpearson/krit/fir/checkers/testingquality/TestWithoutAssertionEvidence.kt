package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.checkers.bodyIn
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.goNameGlobMatches
import dev.jasonpearson.krit.fir.support.hasErrorType
import dev.jasonpearson.krit.fir.support.isOrInheritsFrom
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCall
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.providers.symbolProvider
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.fir.expressions.FirOperation

/**
 * Whether a test body proves an assertion. TestWithoutAssertion is a confirm-only FIR rule: a Go finding stands only
 * where this checker reports it too, so "no finding" here removes Go's. Only proof may therefore suppress the report;
 * whatever FIR cannot see into (a helper in another file, module or jar, a constructor, a property read, a helper
 * cycle or one past the visit limits) is not proof, and the report keeps Go's verdict, which Go reached with its own
 * project-wide helper index (#1671).
 */
internal class TestWithoutAssertionEvidence(
    private val file: FirFile,
    private val session: FirSession,
    private val patterns: List<String>,
) {
    private val active = HashSet<FirNamedFunctionSymbol>()
    private val memo = HashMap<Pair<FirNamedFunctionSymbol, Int>, Boolean>()
    private var visitedHelpers = 0

    fun preventsFinding(body: FirBlock, depth: Int, helper: Boolean): Boolean {
        val visitor = EvidenceVisitor(this, depth, helper, patterns)
        body.accept(visitor)
        return visitor.found
    }

    /**
     * A member of a custom Truth subject or AssertJ assert (`subject.matchesValue(1)`) is an assertion: its class
     * extends the library's base. Supertypes are declaration facts, so this holds on a slice compile too.
     */
    fun isCustomAssertionSubjectCall(symbol: FirCallableSymbol<*>): Boolean {
        val owner = symbol.callableId?.classId ?: return false
        val classSymbol = session.symbolProvider.getClassLikeSymbolByClassId(owner) ?: return false
        return assertionSubjectBases.any { isOrInheritsFrom(classSymbol, it, session, lookupInterfaces = false) }
    }

    @OptIn(SymbolInternals::class)
    fun helperPreventsFinding(symbol: FirNamedFunctionSymbol, depth: Int): Boolean {
        if (symbol.callableId.asSingleFqName().asString() in TestWithoutAssertionCalls.nonAssertingCalls) return false
        // A cycle's back edge adds nothing; whatever else the helpers on the cycle do still counts.
        if (symbol in active) return false
        // A helper of another module, or one compiled to a jar, has no body to read: no proof.
        if (symbol.moduleData != file.moduleData) return false
        return resolveHelper(symbol, depth)
    }

    @OptIn(SymbolInternals::class)
    private fun resolveHelper(symbol: FirNamedFunctionSymbol, depth: Int): Boolean {
        // Only a body in the checked file is read (slice compiles stay sound); another file's is no proof.
        val body = symbol.bodyIn(file.symbol, session) ?: return false
        val key = symbol to depth
        val cached = memo[key]
        if (cached != null) return cached
        return inspectHelper(symbol, body, key, depth)
    }

    @OptIn(SymbolInternals::class)
    private fun inspectHelper(
        symbol: FirNamedFunctionSymbol,
        body: FirBlock,
        key: Pair<FirNamedFunctionSymbol, Int>,
        depth: Int,
    ): Boolean {
        // Past the visit budget or depth, or into a generated helper or another test, the body is not read: no proof.
        if (exceededHelperVisitLimit()) return false
        val function = symbol.fir
        val eligibleSource = hasEligibleSource(function)
        val maximumDepthReached = isAtMaximumHelperDepth(depth)
        if (!eligibleSource || maximumDepthReached) return false
        active += symbol
        val prevents = preventsFinding(body, depth + 1, helper = true)
        active -= symbol
        memo[key] = prevents
        return prevents
    }

    private fun exceededHelperVisitLimit(): Boolean = ++visitedHelpers > MAX_VISITED_HELPERS

    private fun hasEligibleSource(function: FirNamedFunction): Boolean =
        function.source?.kind is KtRealSourceElementKind && !TestWithoutAssertionGoSupport.isTest(function)

    private fun isAtMaximumHelperDepth(depth: Int): Boolean = depth >= MAX_HELPER_DEPTH
}

private class EvidenceVisitor(
    private val evidence: TestWithoutAssertionEvidence,
    private val depth: Int,
    private val helper: Boolean,
    private val patterns: List<String>,
) : FirVisitorVoid() {
    var found = false
        private set
    private var conditional = 0

    override fun visitElement(element: FirElement) {
        if (found) return
        if (basicExpressionEvidence(element) || propertyAccessEvidence(element) ||
            hasThrowEvidence(element, helper, conditional)
        ) {
            found = true
            return
        }
        if (element is FirFunctionCall) {
            handleFunctionCall(element)
            return
        }
        visitChildrenConditionally(element)
    }

    // An assignment, a constructor call and a property read are not assertions; their operands are still visited.
    private fun basicExpressionEvidence(element: FirElement): Boolean = when {
        hasErrorType(element) -> true
        isAssertCastEvidence(element) -> true
        isOtherCallEvidence(element) -> true
        else -> false
    }

    private fun propertyAccessEvidence(element: FirElement): Boolean =
        element is FirPropertyAccessExpression && element.calleeReference.toResolvedCallableSymbol() == null

    private fun handleFunctionCall(element: FirFunctionCall) {
        val symbol = element.calleeReference.toResolvedCallableSymbol()
        if (symbol == null) {
            found = true
            return
        }
        if (callEvidence(symbol) || helperEvidence(symbol)) {
            found = true
            return
        }
        element.acceptChildren(this)
    }

    private fun callEvidence(symbol: FirCallableSymbol<*>): Boolean {
        val id = symbol.callableId ?: return false
        val libraryAssertion = TestWithoutAssertionCalls.libraryAssertion(id)
        val builtinCheck = TestWithoutAssertionCalls.builtinCheck(id)
        val conditionalError = TestWithoutAssertionCalls.builtinError(id) && (!helper || conditional > 0)
        val configuredPattern = patterns.any { goNameGlobMatches(it.trim(), id.callableName.asString()) }
        return libraryAssertion || builtinCheck || conditionalError || configuredPattern
    }

    private fun helperEvidence(symbol: FirCallableSymbol<*>): Boolean =
        evidence.isCustomAssertionSubjectCall(symbol) ||
            symbol is FirNamedFunctionSymbol && evidence.helperPreventsFinding(symbol, depth)

    private fun visitChildrenConditionally(element: FirElement) {
        val condition = isConditionalElement(element)
        if (condition) conditional++
        element.acceptChildren(this)
        if (condition) conditional--
    }

}

private fun isConditionalElement(element: FirElement): Boolean =
    element is FirWhenExpression || element is FirCatch || element is FirElvisExpression

private fun isAssertCastEvidence(element: FirElement): Boolean =
    element is FirTypeOperatorCall && element.operation == FirOperation.AS

private fun isOtherCallEvidence(element: FirElement): Boolean =
    element is FirCall && element !is FirFunctionCall && element !is FirTypeOperatorCall

private fun hasErrorType(element: FirElement): Boolean =
    element is FirExpression && element.hasErrorType()

private fun hasThrowEvidence(element: FirElement, helper: Boolean, conditional: Int): Boolean =
    element is FirThrowExpression && (!helper || conditional > 0)

private val assertionSubjectBases = listOf(
    classIdOf("com.google.common.truth", "Subject"),
    classIdOf("org.assertj.core.api", "AbstractAssert"),
)

private const val MAX_VISITED_HELPERS = 256
private const val MAX_HELPER_DEPTH = 4
