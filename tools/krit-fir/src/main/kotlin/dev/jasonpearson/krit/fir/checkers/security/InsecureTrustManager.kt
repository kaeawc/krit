package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.firstModifierAnchor
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go InsecureTrustManager rule: a trust manager's
 * `checkServerTrusted` or `checkClientTrusted` whose body does nothing, so the
 * trust manager accepts every certificate chain. Reported once per function on
 * its first line (its modifier list, else `fun`), with Go's message.
 *
 * The function is a `checkServerTrusted` / `checkClientTrusted` with a body
 * that a class, object, interface, companion, enum entry, local class, or
 * anonymous object declares directly as a member, where that declaring class
 * is a trust manager: `javax.net.ssl.TrustManager` is in its supertype
 * closure (so `X509TrustManager`, `X509ExtendedTrustManager`, project base
 * classes, type aliases, and import aliases of them all count). Like Go, the
 * parameters are not inspected: an overload with other parameters declared on
 * a trust manager counts too.
 *
 * The body does nothing when it is empty (comments aside), or holds only a
 * bare `return` (Go's shapes), or only statements that evaluate to `Unit`
 * without side effects: `Unit`, `return Unit`, an expression body `= Unit`,
 * and a stdlib scope function (`run`, `let`, `also`, `apply`, `with`) on no
 * written receiver or on a plain read (`chain`, `this`), directly or through a
 * safe call, whose lambda does nothing. Go reports the expression bodies
 * `= run { }`, `= chain.let { }`, and `= with(chain) { }` because its brace
 * scan finds the empty lambda.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`InsecureTrustManager*.kt`):
 * - Precision: Go takes any function with the name whose nearest enclosing
 *   class declaration or object expression mentions the word `TrustManager` or
 *   `X509TrustManager` anywhere in its text, in a file that imports or
 *   mentions `javax.net.ssl.TrustManager` / `javax.net.ssl.X509TrustManager`.
 *   So it reports a local function with the name, a function of a companion or
 *   nested object inside a trust manager, and a member of a class that only
 *   holds a trust manager in a property. None of those is a trust manager's
 *   check method, so none is reported here.
 * - Precision: Go reads the first brace block after the function's name, so it
 *   reports an expression body whose call passes an empty lambda
 *   (`= validator.check(chain) { }`), which does validate. Not reported here.
 * - Recall: Go skips a class whose text contains ` by ` anywhere (a delegated
 *   supertype, a `by lazy` property, a comment), so it misses the empty check
 *   methods such a class declares; those are reported here.
 * - Recall: Go misses a trust manager that is a named `object`, a subclass of
 *   `X509ExtendedTrustManager` or of a project base class, one reached through
 *   an import alias or type alias, and one in a file that neither imports nor
 *   mentions the two javax names. Those are reported here.
 * - Recall: Go only accepts an empty body or a bare `return`, so it misses the
 *   other do-nothing bodies listed above (`= Unit`, `{ Unit }`,
 *   `{ return Unit }`, `{ run { } }`, `{ chain?.let { } }`).
 */
internal object InsecureTrustManager : FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "InsecureTrustManager"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(InsecureTrustManager)
    }

    private const val MESSAGE =
        "Trust manager check method accepts certificates without validation. " +
            "Perform certificate validation or remove the trust-all manager."

    private val checkNames = setOf(Name.identifier("checkServerTrusted"), Name.identifier("checkClientTrusted"))
    private val trustManager = ClassId(FqName("javax.net.ssl"), Name.identifier("TrustManager"))
    private val unitClassId = ClassId(FqName("kotlin"), Name.identifier("Unit"))
    private val scopeFunctions = listOf("run", "let", "also", "apply", "with")
        .map { CallableId(FqName("kotlin"), Name.identifier(it)) }
        .toSet()

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        if (declaration.name !in checkNames) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val body = declaration.body ?: return
        // The declaring class must be the nearest container: a local function
        // with the name, even inside a trust manager, is not its check method.
        val owner = context.containingDeclarations.lastOrNull { it != declaration.symbol } as? FirClassSymbol<*> ?: return
        if (!isTrustManager(owner, context.session)) return
        if (!isTrivialBlock(body)) return
        report(firstLine(source), MESSAGE)
    }

    // The class itself or its supertypes, read from the class's own lookup
    // tags, which are bound to local and anonymous classes, so no class id is
    // resolved from a symbol that may be local.
    private fun isTrustManager(symbol: FirClassSymbol<*>, session: FirSession): Boolean =
        symbol.classId == trustManager ||
            lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
                .any { it.classId == trustManager }

    private fun isTrivialBlock(block: FirBlock): Boolean = block.statements.all(::isNoOp)

    private fun isNoOp(statement: FirStatement): Boolean = when (statement) {
        is FirReturnExpression -> isNoOpValue(statement.result)
        is FirExpression -> isNoOpValue(statement)
        else -> false
    }

    // A value that does nothing: the implicit Unit of a bare `return` or an
    // empty lambda, the `Unit` object, or a scope function around such a body.
    private fun isNoOpValue(expression: FirExpression): Boolean {
        if (expression.source?.kind is KtFakeSourceElementKind.ImplicitUnit) return true
        return when (expression) {
            is FirResolvedQualifier -> expression.classId == unitClassId
            is FirFunctionCall -> isTrivialScopeCall(expression)
            is FirSafeCallExpression ->
                isPlainRead(expression.receiver) &&
                    (expression.selector as? FirFunctionCall)?.let(::isTrivialScopeCall) == true
            else -> false
        }
    }

    // A stdlib scope function (`run`, `let`, `also`, `apply`, `with`) whose
    // lambda does nothing, on no written receiver (the implicit `this`) or on
    // a plain read such as `chain` or `this`. A computed receiver
    // (`validate(chain).let { }`) may validate.
    private fun isTrivialScopeCall(call: FirFunctionCall): Boolean {
        val symbol = call.calleeReference.toResolvedCallableSymbol() ?: return false
        if (symbol.callableId !in scopeFunctions) return false
        val arguments = call.arguments
        val lambda = arguments.lastOrNull() as? FirAnonymousFunctionExpression ?: return false
        val subjects = listOfNotNull(call.explicitReceiver) + arguments.dropLast(1)
        if (!subjects.all(::isPlainRead)) return false
        val body = lambda.anonymousFunction.body ?: return false
        return isTrivialBlock(body)
    }

    private fun isPlainRead(expression: FirExpression): Boolean = when (expression) {
        is FirThisReceiverExpression, is FirCheckedSafeCallSubject -> true
        is FirSmartCastExpression -> isPlainRead(expression.originalExpression)
        is FirPropertyAccessExpression -> expression.explicitReceiver?.let(::isPlainRead) ?: true
        else -> false
    }

    // Go reports the function_declaration's first line: the first entry of its
    // modifier list (an annotation or a modifier keyword) when it has one, else
    // the `fun` keyword. A preceding KDoc is not part of Go's node.
    private fun firstLine(source: KtSourceElement): KtSourceElement {
        firstModifierAnchor(source)?.let { return it }
        val keyword = lightChildren(source, source.lighterASTNode)
            .firstOrNull { it.tokenType == KtTokens.FUN_KEYWORD } ?: return source
        return lightSourceOf(keyword, source)
    }
}
