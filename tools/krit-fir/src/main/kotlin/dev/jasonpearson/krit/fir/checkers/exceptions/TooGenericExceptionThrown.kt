package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCatch
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go TooGenericExceptionThrown rule: a `throw` of a newly
 * constructed `Exception`, `RuntimeException`, `Error`, or `Throwable`
 * (`exceptionNames`), reported on the `throw` line with Go's message.
 *
 * The thrown value counts when it, or a value it can evaluate to (an
 * `if`/`when` branch, either side of an elvis, a `try` or `catch` block, a cast
 * or `!!` operand), is a constructor call of one of the listed classes, through
 * a type alias or an import alias too. `kotlin.Exception`, `kotlin.Error` and
 * `kotlin.RuntimeException` are aliases of the `java.lang` classes, and
 * `kotlin.Throwable` is the Kotlin class. A call of a function named like the
 * class it returns (`fun Error(code: Int): Error`) counts too: Go reads the
 * call's name. The class must be one of those four; a configured name outside
 * them counts for the class of that name in `java.lang` or `kotlin` (the
 * implicitly imported packages, which Go's resolver leaves unresolved and so
 * reports). A project class with a listed name is not a generic exception.
 *
 * Exemptions mirrored from Go: test files and `.gradle.kts` scripts, and a
 * constructor that passes the parameter of the nearest enclosing `catch`
 * directly as an argument (`throw RuntimeException("context", e)`), which
 * wraps the caught exception instead of hiding it.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`TooGenericExceptionThrown*.kt`):
 * - Go's dispatch node is any jump expression, so it also reports
 *   `return Exception(...)`, which throws nothing. Not reported here.
 * - Go reads only the first call inside the `throw` (`throw if (c)
 *   IllegalStateException() else Exception()` names IllegalStateException),
 *   the name as written (an import alias or type alias hides the class), and
 *   resolves an explicit `import java.lang.Exception` to `kotlin.Exception`,
 *   which is not in its table; it also skips every throw of a name the file
 *   declares a class for anywhere, even a nested class the throw does not
 *   resolve to. Each of those throws a generic exception, so each is reported
 *   here.
 * - Go matches the caught exception by name, so a lambda parameter shadowing
 *   the catch parameter exempts the throw there; here the argument must be
 *   the catch parameter itself.
 */
internal object TooGenericExceptionThrown :
    FirExpressionChecker<FirThrowExpression>(MppCheckerKind.Common), FirRule {
    override val ruleId = "TooGenericExceptionThrown"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val throwExpressionCheckers = setOf(TooGenericExceptionThrown)
    }

    private val defaultNames = listOf("Exception", "Throwable", "Error", "RuntimeException")

    private val javaLang = FqName("java.lang")
    private val kotlinPackage = FqName("kotlin")

    private val genericClassIds = setOf(
        ClassId(javaLang, Name.identifier("Exception")),
        ClassId(javaLang, Name.identifier("RuntimeException")),
        ClassId(javaLang, Name.identifier("Error")),
        ClassId(javaLang, Name.identifier("Throwable")),
        ClassId(kotlinPackage, Name.identifier("Throwable")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirThrowExpression) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (isInTestFile()) return
        if (containingScanPath()?.endsWith(".gradle.kts") == true) return

        val names = exceptionNames()
        val caught = (context.containingElements.lastOrNull { it is FirCatch } as? FirCatch)?.parameter?.symbol

        val candidates = mutableListOf<FirExpression>()
        collectResults(expression.exception, candidates, depth = 0)
        for (candidate in candidates) {
            val call = candidate as? FirFunctionCall ?: continue
            val name = genericClassName(call, names) ?: continue
            if (caught != null && passesCaught(call, caught)) continue
            report(throwKeyword(source), "Too-generic exception type '$name' thrown.")
            return
        }
    }

    context(context: CheckerContext)
    private fun exceptionNames(): Set<String> {
        val configured = (config()["exceptionNames"] as? List<*>)?.mapNotNull { it as? String }.orEmpty()
        return configured.ifEmpty { defaultNames }.toSet()
    }

    // The thrown expression and every value it can evaluate to, in source
    // order.
    private fun collectResults(expression: FirExpression, into: MutableList<FirExpression>, depth: Int) {
        into += expression
        if (depth > MAX_DEPTH) return
        when (expression) {
            is FirWhenExpression -> expression.branches.forEach { collectResults(it.result, into, depth + 1) }
            is FirElvisExpression -> {
                collectResults(expression.lhs, into, depth + 1)
                collectResults(expression.rhs, into, depth + 1)
            }
            is FirTryExpression -> {
                collectResults(expression.tryBlock, into, depth + 1)
                expression.catches.forEach { collectResults(it.block, into, depth + 1) }
            }
            is FirBlock -> (expression.statements.lastOrNull() as? FirExpression)?.let {
                collectResults(it, into, depth + 1)
            }
            is FirTypeOperatorCall -> if (expression.operation == FirOperation.AS || expression.operation == FirOperation.SAFE_AS) {
                expression.argumentList.arguments.singleOrNull()?.let { collectResults(it, into, depth + 1) }
            }
            is FirCheckNotNullCall -> expression.argumentList.arguments.singleOrNull()?.let {
                collectResults(it, into, depth + 1)
            }
            is FirSmartCastExpression -> collectResults(expression.originalExpression, into, depth + 1)
            else -> Unit
        }
    }

    // The simple name of the generic exception class [call] constructs, or
    // null. The class id comes from the call's type's lookup tag and is only
    // compared, never resolved, so a local class is safe.
    context(context: CheckerContext)
    private fun genericClassName(call: FirFunctionCall, names: Set<String>): String? {
        val callee = call.calleeReference.toResolvedCallableSymbol() ?: return null
        val type = call.resolvedType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType
            ?: return null
        val classId = type.lookupTag.classId
        if (classId.isLocal || classId.isNestedClass) return null
        val name = classId.shortClassName.asString()
        if (callee !is FirConstructorSymbol && callee.callableId?.callableName?.asString() != name) return null
        if (name !in names) return null
        if (classId in genericClassIds) return name
        return name.takeIf { classId.packageFqName == javaLang || classId.packageFqName == kotlinPackage }
    }

    // Whether [call] passes the catch parameter itself as an argument.
    private fun passesCaught(call: FirFunctionCall, caught: FirBasedSymbol<*>): Boolean =
        call.argumentList.arguments.any { argument ->
            var value = argument
            if (value is FirWrappedArgumentExpression) value = value.expression
            if (value is FirSmartCastExpression) value = value.originalExpression
            val access = value as? FirPropertyAccessExpression ?: return@any false
            access.explicitReceiver == null && access.calleeReference.toResolvedVariableSymbol() == caught
        }

    // Go reports at the throw node's start: the `throw` keyword.
    private fun throwKeyword(source: KtSourceElement): KtSourceElement {
        val keyword = lightChildren(source, source.lighterASTNode).firstOrNull { it.tokenType == KtTokens.THROW_KEYWORD }
            ?: return source
        return lightSourceOf(keyword, source)
    }

    private const val MAX_DEPTH = 16
}
