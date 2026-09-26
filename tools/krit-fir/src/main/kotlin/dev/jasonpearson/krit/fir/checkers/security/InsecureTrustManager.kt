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
import org.jetbrains.kotlin.fir.expressions.FirBooleanOperatorExpression
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStatement
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.expressions.impl.FirElseIfTrueCondition
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirBackingFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFieldSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirSyntheticPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

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
 * The body does nothing when it is empty (comments and `;` aside), or holds
 * only a bare or labeled `return` (Go's shapes), or only statements that
 * evaluate without inspecting a certificate ([isInert]):
 * - `Unit`, a literal, `return Unit`, an expression body `= Unit`;
 * - a plain read: a parameter, a local, `this`, or a property with neither a
 *   custom getter nor a delegate (reading one of those runs code that may
 *   validate);
 * - a safe call, an elvis, a smart cast, `!!`, an `is` check, or `&&` / `||`
 *   over such values;
 * - a null check (`x == null`, `requireNotNull(x)`, `checkNotNull(x)`,
 *   `require(x != null)`, `check(x != null)`): it rejects no real chain;
 * - an `if` / `when` with such conditions whose branches do nothing, and a
 *   `try` whose blocks do nothing;
 * - a do-nothing stdlib call on such values whose lambdas are themselves
 *   inert: the scope functions (`run`, `let`, `also`, `apply`, `with`),
 *   `forEach`, `forEachIndexed`, `onEach`, `onEachIndexed`, `map`,
 *   `mapIndexed`, `repeat`, `synchronized`, and the views `orEmpty`,
 *   `asList`, `asSequence`, `asIterable`, `toList`.
 * Any other call, including a stdlib one such as `require(pinned)` or a call
 * that hands the chain to a verifier, counts as validation (delegation).
 * Go reports an expression body whose first brace block is an empty lambda
 * (`= chain.forEach { }`, `= chain?.let { } ?: Unit`, `= synchronized(this) { }`,
 * `= repeat(n) { }`), so those do-nothing stdlib bodies are Go's too.
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
 *   reports an expression body whose call hands the chain to a verifier with
 *   an empty lambda (`= verify(chain) { }`), and a function whose parameter
 *   default is an empty lambda (`onFail: () -> Unit = { }`) whatever its body
 *   does. Neither is reported here.
 * - Recall: Go skips a class whose text contains ` by ` anywhere (a delegated
 *   supertype, a `by lazy` property, a comment), so it misses the empty check
 *   methods such a class declares; those are reported here.
 * - Recall: Go misses a trust manager that is a named `object`, a subclass of
 *   `X509ExtendedTrustManager` or of a project base class, one reached through
 *   an import alias or type alias, and one in a file that neither imports nor
 *   mentions the two javax names. Those are reported here.
 * - Recall: Go only accepts an empty body, a bare `return`, or an expression
 *   body with an empty lambda, so it misses the other do-nothing bodies listed
 *   above (`= Unit`, `{ Unit }`, `{ ; }`, `{ return Unit }`,
 *   `{ return@checkServerTrusted }`, `{ run { } }`, `{ chain?.let { } }`,
 *   `{ chain?.forEach { } }`, `{ requireNotNull(chain).also { } }`,
 *   `{ if (chain == null) return }`, `= chain.forEachIndexed { _, _ -> }`).
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

    private val kotlin = FqName("kotlin")
    private val collections = FqName("kotlin.collections")
    private val sequences = FqName("kotlin.sequences")

    private fun ids(pkg: FqName, vararg names: String) = names.map { CallableId(pkg, Name.identifier(it)) }

    // Stdlib calls that do nothing but run their lambda or return a view of
    // their receiver, and null checks, which reject no real chain.
    private val inertCalls: Set<CallableId> = (
        ids(kotlin, "run", "let", "also", "apply", "with", "repeat", "synchronized", "requireNotNull", "checkNotNull") +
            ids(collections, "forEach", "forEachIndexed", "onEach", "onEachIndexed", "map", "mapIndexed") +
            ids(collections, "orEmpty", "asList", "asSequence", "asIterable", "toList") +
            ids(sequences, "forEach", "forEachIndexed", "onEach", "onEachIndexed", "map", "mapIndexed") +
            ids(sequences, "orEmpty", "asIterable", "toList")
        ).toSet()

    // `require` / `check`: inert only when the condition is a null check; any
    // other condition may validate.
    private val conditionCalls: Set<CallableId> = ids(kotlin, "require", "check").toSet()

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
        is FirReturnExpression -> isInert(statement.result)
        is FirExpression -> isInert(statement)
        else -> false
    }

    // A value whose evaluation inspects no certificate and runs no code that
    // could: the implicit Unit of a bare `return` or an empty lambda, `Unit`,
    // a literal, a plain read, a null check, and the do-nothing stdlib calls.
    private fun isInert(expression: FirExpression): Boolean {
        if (expression.source?.kind is KtFakeSourceElementKind.ImplicitUnit) return true
        return when (expression) {
            is FirLiteralExpression -> true
            is FirResolvedQualifier -> expression.classId == unitClassId
            is FirThisReceiverExpression, is FirCheckedSafeCallSubject -> true
            is FirSmartCastExpression -> isInert(expression.originalExpression)
            is FirWrappedArgumentExpression -> isInert(expression.expression)
            is FirPropertyAccessExpression ->
                isPlainVariable(expression) && expression.explicitReceiver?.let(::isInert) != false
            is FirSafeCallExpression ->
                isInert(expression.receiver) && (expression.selector as? FirExpression)?.let(::isInert) == true
            is FirElvisExpression -> isInert(expression.lhs) && isInert(expression.rhs)
            is FirCheckNotNullCall -> expression.arguments.all(::isInert)
            is FirEqualityOperatorCall -> isNullCheck(expression)
            is FirTypeOperatorCall ->
                expression.operation in typeChecks && expression.arguments.all(::isInert)
            is FirBooleanOperatorExpression -> isInert(expression.leftOperand) && isInert(expression.rightOperand)
            is FirElseIfTrueCondition -> true
            // An `if` / `when` whose conditions are inert and whose branches do
            // nothing, and a `try` whose blocks all do nothing.
            is FirWhenExpression ->
                expression.subjectVariable?.initializer?.let(::isInert) != false &&
                    expression.branches.all { isInert(it.condition) && isTrivialBlock(it.result) }
            is FirTryExpression ->
                isTrivialBlock(expression.tryBlock) &&
                    expression.catches.all { isTrivialBlock(it.block) } &&
                    expression.finallyBlock?.let(::isTrivialBlock) != false
            is FirFunctionCall -> isInertCall(expression)
            else -> false
        }
    }

    private val typeChecks = setOf(FirOperation.IS, FirOperation.NOT_IS)

    // A read that runs no code: a value parameter (including a lambda's), a
    // local variable, a field, or a property with no custom getter and no
    // delegate. A Java getter behind a synthetic property runs code.
    private fun isPlainVariable(access: FirPropertyAccessExpression): Boolean =
        when (val symbol = access.calleeReference.toResolvedCallableSymbol()) {
            is FirValueParameterSymbol, is FirFieldSymbol, is FirBackingFieldSymbol -> true
            is FirSyntheticPropertySymbol -> false
            is FirPropertySymbol -> !symbol.hasDelegate && symbol.getterSymbol?.isDefault != false
            else -> false
        }

    // `x == null`, `x != null`, `x === null`, `x !== null` over an inert `x`.
    private fun isNullCheck(call: FirEqualityOperatorCall): Boolean {
        if (call.operation !in nullCheckOperations) return false
        val (left, right) = call.arguments.takeIf { it.size == 2 } ?: return false
        return (isNullLiteral(left) && isInert(right)) || (isNullLiteral(right) && isInert(left))
    }

    private val nullCheckOperations =
        setOf(FirOperation.EQ, FirOperation.NOT_EQ, FirOperation.IDENTITY, FirOperation.NOT_IDENTITY)

    private fun isNullLiteral(expression: FirExpression): Boolean =
        (expression as? FirLiteralExpression)?.kind == ConstantValueKind.Null

    // A do-nothing stdlib call on an inert receiver, with inert arguments and
    // lambdas that do nothing. A computed receiver or argument
    // (`validate(chain).let { }`) may validate, and so may any callee off the
    // list, such as a verifier that receives the chain.
    private fun isInertCall(call: FirFunctionCall): Boolean {
        val callableId = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
        val isCondition = callableId in conditionCalls
        if (!isCondition && callableId !in inertCalls) return false
        if (call.explicitReceiver?.let(::isInert) == false) return false
        return call.arguments.withIndex().all { (index, argument) ->
            val value = (argument as? FirWrappedArgumentExpression)?.expression ?: argument
            when {
                value is FirAnonymousFunctionExpression -> value.anonymousFunction.body?.let(::isTrivialBlock) == true
                isCondition && index == 0 -> value is FirEqualityOperatorCall && isNullCheck(value)
                else -> isInert(value)
            }
        }
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
